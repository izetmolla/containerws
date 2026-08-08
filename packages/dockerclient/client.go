package dockerclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/izetmolla/containerws/models"
	"golang.org/x/crypto/ssh"
)

var (
	mu       sync.Mutex
	shared   *client.Client
	sharedID string // environment id used for shared
)

// SockPath returns the Docker unix socket path (DOCKER_HOST unix://… or default).
func SockPath() string {
	if v := strings.TrimSpace(os.Getenv("DOCKER_HOST")); strings.HasPrefix(v, "unix://") {
		return strings.TrimPrefix(v, "unix://")
	}
	return "/var/run/docker.sock"
}

// Client returns a shared Docker Engine API client for the default/local socket.
// Prefer ClientFor when an environment is selected.
func Client() (*client.Client, error) {
	return ClientFor(nil)
}

// ClientFor builds (or reuses) a client for the given environment.
// nil env → local unix socket (SockPath).
func ClientFor(env *models.DockerEnvironment) (*client.Client, error) {
	id := ""
	if env != nil {
		id = env.ID
	}
	mu.Lock()
	defer mu.Unlock()
	if shared != nil && sharedID == id {
		return shared, nil
	}
	if shared != nil {
		_ = shared.Close()
		shared = nil
		sharedID = ""
	}
	cli, err := newClientFor(env)
	if err != nil {
		return nil, err
	}
	shared = cli
	sharedID = id
	return shared, nil
}

// Reset drops the shared client (e.g. after Engine restart or env switch).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	if shared != nil {
		_ = shared.Close()
		shared = nil
		sharedID = ""
	}
}

func newClientFor(env *models.DockerEnvironment) (*client.Client, error) {
	if env == nil || env.ConnType == "" || env.ConnType == models.DockerConnUnix {
		sock := SockPath()
		if env != nil && strings.TrimSpace(env.SocketPath) != "" {
			sock = strings.TrimSpace(env.SocketPath)
		}
		return newUnixClient(sock)
	}
	switch env.ConnType {
	case models.DockerConnTLS:
		return newTLSClient(env)
	case models.DockerConnSSH:
		return newSSHClient(env)
	default:
		return nil, fmt.Errorf("unsupported docker connection type: %s", env.ConnType)
	}
}

func newUnixClient(sock string) (*client.Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
		client.WithHost("unix://" + sock),
		client.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", sock)
				},
			},
			Timeout: 60 * time.Second,
		}),
	}
	return client.NewClientWithOpts(opts...)
}

func newTLSClient(env *models.DockerEnvironment) (*client.Client, error) {
	host := strings.TrimSpace(env.HostURL)
	if host == "" {
		port := env.TCPPort
		if port <= 0 {
			port = 2376
		}
		host = fmt.Sprintf("tcp://%s:%d", strings.TrimSpace(env.TCPHost), port)
	}
	tlsCfg, err := buildTLSConfig(env)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
		Timeout: 60 * time.Second,
	}
	return client.NewClientWithOpts(
		client.WithAPIVersionNegotiation(),
		client.WithHost(host),
		client.WithHTTPClient(httpClient),
	)
}

func buildTLSConfig(env *models.DockerEnvironment) (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: env.TLSSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}
	if ca := strings.TrimSpace(env.TLSCACert); ca != "" && !env.TLSSkipVerify {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("invalid TLS CA certificate PEM")
		}
		cfg.RootCAs = pool
	}
	certPEM := strings.TrimSpace(env.TLSCert)
	keyPEM := strings.TrimSpace(env.TLSKey)
	if certPEM != "" && keyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return nil, fmt.Errorf("tls client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

func newSSHClient(env *models.DockerEnvironment) (*client.Client, error) {
	remoteSock := strings.TrimSpace(env.SSHRemoteSocket)
	if remoteSock == "" {
		remoteSock = "/var/run/docker.sock"
	}
	dialer, err := sshUnixDialer(env, remoteSock)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer(ctx)
			},
		},
		Timeout: 60 * time.Second,
	}
	// Host is a placeholder; DialContext tunnels to the remote unix socket.
	return client.NewClientWithOpts(
		client.WithAPIVersionNegotiation(),
		client.WithHost("unix://"+remoteSock),
		client.WithHTTPClient(httpClient),
		client.WithDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer(ctx)
		}),
	)
}

func sshUnixDialer(env *models.DockerEnvironment, remoteSock string) (func(context.Context) (net.Conn, error), error) {
	host := strings.TrimSpace(env.SSHHost)
	if host == "" {
		return nil, errors.New("ssh_host is required")
	}
	port := env.SSHPort
	if port <= 0 {
		port = 22
	}
	user := strings.TrimSpace(env.SSHUser)
	if user == "" {
		user = "root"
	}
	key := strings.TrimSpace(env.SSHPrivateKey)
	if key == "" {
		return nil, errors.New("ssh_private_key is required for SSH environments")
	}
	signer, err := parseSSHPrivateKey(key, strings.TrimSpace(env.SSHPassphrase))
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // user-managed remote endpoints
		Timeout:         20 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	return func(ctx context.Context) (net.Conn, error) {
		d := net.Dialer{Timeout: 20 * time.Second}
		tcpConn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
		}
		sshConn, chans, reqs, err := ssh.NewClientConn(tcpConn, addr, cfg)
		if err != nil {
			_ = tcpConn.Close()
			return nil, fmt.Errorf("ssh handshake: %w", err)
		}
		sshClient := ssh.NewClient(sshConn, chans, reqs)
		unixConn, err := sshClient.Dial("unix", remoteSock)
		if err != nil {
			_ = sshClient.Close()
			return nil, fmt.Errorf("ssh remote socket %s: %w", remoteSock, err)
		}
		return &sshTunnelConn{Conn: unixConn, ssh: sshClient}, nil
	}, nil
}

type sshTunnelConn struct {
	net.Conn
	ssh *ssh.Client
}

func (c *sshTunnelConn) Close() error {
	err := c.Conn.Close()
	_ = c.ssh.Close()
	return err
}

func parseSSHPrivateKey(pemBytes, passphrase string) (ssh.Signer, error) {
	if passphrase == "" {
		signer, err := ssh.ParsePrivateKey([]byte(pemBytes))
		if err != nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase([]byte(pemBytes), []byte(passphrase))
	if err != nil {
		// fallback if block encrypted differently
		block, _ := pem.Decode([]byte(pemBytes))
		if block == nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		return nil, fmt.Errorf("parse ssh private key with passphrase: %w", err)
	}
	return signer, nil
}

// Ping checks Engine reachability for the default/shared client.
func Ping(ctx context.Context) error {
	cli, err := Client()
	if err != nil {
		return err
	}
	_, err = cli.Ping(ctx)
	if err != nil {
		Reset()
		return err
	}
	return nil
}

// PingEnv pings a specific environment without replacing the shared client permanently on success path for other envs.
func PingEnv(ctx context.Context, env *models.DockerEnvironment) error {
	cli, err := newClientFor(env)
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.Ping(ctx)
	return err
}

// IsReachable is a quick boolean ping of the shared client.
func IsReachable(ctx context.Context) bool {
	c, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	return Ping(c) == nil
}

// ErrUnavailable is returned when the Engine socket/API is down.
var ErrUnavailable = errors.New("docker engine unavailable")

// MapError maps Docker client errors to HTTP-ish messages.
func MapError(err error) (int, string) {
	if err == nil {
		return 200, ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "no such container"),
		strings.Contains(lower, "no such image"),
		strings.Contains(lower, "no such network"),
		strings.Contains(lower, "no such volume"):
		return 404, msg
	case strings.Contains(lower, "conflict"),
		strings.Contains(lower, "already in use"),
		strings.Contains(lower, "is already"):
		return 409, msg
	case strings.Contains(lower, "cannot connect"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such file"),
		strings.Contains(lower, "is the docker daemon running"),
		strings.Contains(lower, "ssh"):
		return 503, "Docker Engine is not reachable — check the selected environment"
	default:
		return 500, msg
	}
}
