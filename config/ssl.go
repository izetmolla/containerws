package config

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/izetmolla/containerws/packages/environments"
	"github.com/spf13/viper"
)

const (
	DefaultHTTPPort = "9000"
	DefaultSSLDir   = "/config/containerws/ssl"
	DevSSLDir       = "./tmp/ssl"
	sslCertFileName = "cert.pem"
	sslKeyFileName  = "key.pem"
)

// SSLPaths holds on-disk certificate locations.
type SSLPaths struct {
	Dir      string
	CertFile string
	KeyFile  string
}

// ResolveSSLDir returns the SSL directory from SSL_DIR, or a env-based default.
func ResolveSSLDir() string {
	if dir := viper.GetString("SSL_DIR"); dir != "" {
		return dir
	}
	if viper.GetString("ENV") != "development" {
		return DefaultSSLDir
	}
	return DevSSLDir
}

// ResolveTLSCertificate returns a tls.Certificate when HTTPS is enabled.
// Preference: SERVER_SSL_* PEMs from environments/env → existing files → generate
// self-signed and persist PEMs into os_environments (SERVER_SSL_CERTIFICATE / SERVER_SSL_KEY).
func ResolveTLSCertificate(ctx context.Context, cfg *ServerConfig, envMgr *environments.Environments) (tls.Certificate, error) {
	if cfg == nil {
		return tls.Certificate{}, fmt.Errorf("server config is nil")
	}
	if !cfg.ENABLE_HTTPS {
		return tls.Certificate{}, fmt.Errorf("https is disabled")
	}

	certPEM := cfg.SSLCertPEM
	keyPEM := cfg.SSLKeyPEM

	if certPEM != "" && keyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("load SERVER_SSL_* PEMs: %w", err)
		}
		log.Printf("using TLS certificate from environments (SERVER_SSL_CERTIFICATE)")
		return cert, nil
	}

	paths := SSLPaths{
		Dir:      cfg.SSL_DIR,
		CertFile: filepath.Join(cfg.SSL_DIR, sslCertFileName),
		KeyFile:  filepath.Join(cfg.SSL_DIR, sslKeyFileName),
	}
	if err := os.MkdirAll(paths.Dir, 0o755); err != nil {
		return tls.Certificate{}, fmt.Errorf("create ssl dir: %w", err)
	}

	certOK := fileExists(paths.CertFile)
	keyOK := fileExists(paths.KeyFile)

	switch {
	case certOK && keyOK:
		certPEMBytes, err := os.ReadFile(paths.CertFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("read cert: %w", err)
		}
		keyPEMBytes, err := os.ReadFile(paths.KeyFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("read key: %w", err)
		}
		cert, err := tls.X509KeyPair(certPEMBytes, keyPEMBytes)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("existing ssl files are invalid: %w", err)
		}
		if err := persistSSLPEMs(ctx, envMgr, string(certPEMBytes), string(keyPEMBytes)); err != nil {
			log.Printf("warn: failed to sync ssl files into environments: %v", err)
		} else {
			cfg.SSLCertPEM = string(certPEMBytes)
			cfg.SSLKeyPEM = string(keyPEMBytes)
		}
		log.Printf("using existing TLS certificate: %s", paths.CertFile)
		return cert, nil
	case certOK || keyOK:
		missing := paths.KeyFile
		if !certOK {
			missing = paths.CertFile
		}
		return tls.Certificate{}, fmt.Errorf("incomplete ssl pair: found one of cert/key but missing %s", missing)
	}

	certPEMBytes, keyPEMBytes, err := generateSelfSignedPEM()
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := writeFileExclusive(paths.CertFile, certPEMBytes, 0o644); err != nil {
		return tls.Certificate{}, fmt.Errorf("write cert: %w", err)
	}
	if err := writeFileExclusive(paths.KeyFile, keyPEMBytes, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("write key: %w", err)
	}

	cert, err := tls.X509KeyPair(certPEMBytes, keyPEMBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load generated cert: %w", err)
	}
	if err := persistSSLPEMs(ctx, envMgr, string(certPEMBytes), string(keyPEMBytes)); err != nil {
		return tls.Certificate{}, fmt.Errorf("persist ssl to environments: %w", err)
	}
	cfg.SSLCertPEM = string(certPEMBytes)
	cfg.SSLKeyPEM = string(keyPEMBytes)
	log.Printf("generated self-signed TLS certificate: %s (stored in environments)", paths.CertFile)
	return cert, nil
}

func persistSSLPEMs(ctx context.Context, envMgr *environments.Environments, certPEM, keyPEM string) error {
	if envMgr == nil {
		return nil
	}
	if err := envMgr.SetCore(ctx, "SERVER_SSL_CERTIFICATE", certPEM); err != nil {
		return err
	}
	if err := envMgr.SetCore(ctx, "SERVER_SSL_KEY", keyPEM); err != nil {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func writeFileExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func generateSelfSignedPEM() (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate tls key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Container Workspace"},
			CommonName:   "cws",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "cws", "containerws"},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	return certPEM, keyPEM, nil
}
