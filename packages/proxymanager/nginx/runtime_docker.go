package nginx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"github.com/izetmolla/containerws/packages/proxymanager/dockerrun"
	"gorm.io/gorm"
)

// ApplyDocker ensures an nginx container is running with generated config mounts.
func ApplyDocker(ctx context.Context, db *gorm.DB, settings *models.ProxySettings, configDir string) error {
	env, err := resolveDockerEnv(db, settings.DockerEnvironmentID)
	if err != nil {
		return err
	}
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}

	name := settings.NginxContainerName
	if name == "" {
		name = "cws-proxy-nginx"
	}
	img := settings.NginxImage
	if img == "" {
		img = "nginx:alpine"
	}

	if err := ensureImage(ctx, cli, img); err != nil {
		return err
	}

	_ = removeContainerByName(ctx, cli, name)

	listenHTTP, listenHTTPS := settings.NginxContainerListenPorts()
	plan, err := dockerrun.BuildPlan(settings, fmt.Sprintf("%d", listenHTTP), fmt.Sprintf("%d", listenHTTPS))
	if err != nil {
		return err
	}

	cfg := &container.Config{
		Image:        img,
		ExposedPorts: plan.ExposedPorts,
		Labels: map[string]string{
			"containerws.proxymanager": "nginx",
		},
	}
	host := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: configDir + "/nginx.conf",
				Target: "/etc/nginx/nginx.conf",
			},
			{
				Type:   mount.TypeBind,
				Source: configDir + "/conf.d",
				Target: "/etc/nginx/conf.d",
			},
			{
				Type:   mount.TypeBind,
				Source: configDir + "/ssl",
				Target: "/etc/nginx/ssl",
			},
		},
	}
	plan.ApplyToHostConfig(host)

	resp, err := cli.ContainerCreate(ctx, cfg, host, plan.NetworkingConfig, nil, name)
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	if err := waitContainerRunning(ctx, cli, resp.ID, 20*time.Second); err != nil {
		return err
	}
	if err := nginxConfigTest(ctx, cli, resp.ID); err != nil {
		return err
	}
	return nil
}

func waitContainerRunning(ctx context.Context, cli *client.Client, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		insp, err := cli.ContainerInspect(ctx, id)
		if err != nil {
			return fmt.Errorf("inspect container: %w", err)
		}
		if insp.State != nil && insp.State.Running {
			return nil
		}
		if insp.State != nil && !insp.State.Running && insp.State.Status == "exited" {
			msg := strings.TrimSpace(insp.State.Error)
			if msg == "" {
				msg = fmt.Sprintf("exit code %d", insp.State.ExitCode)
			}
			return fmt.Errorf("container exited before ready: %s", msg)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for container to become ready")
}

func nginxConfigTest(ctx context.Context, cli *client.Client, id string) error {
	execID, err := cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          []string{"nginx", "-t"},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return fmt.Errorf("nginx -t: %w", err)
	}
	hijack, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("nginx -t attach: %w", err)
	}
	defer hijack.Close()
	out, _ := io.ReadAll(io.LimitReader(hijack.Reader, 64*1024))
	inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("nginx -t inspect: %w", err)
	}
	if inspect.ExitCode != 0 {
		return fmt.Errorf("nginx -t failed: %s", strings.TrimSpace(demuxDockerLike(out)))
	}
	return nil
}

func demuxDockerLike(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var b strings.Builder
	i := 0
	for i+8 <= len(raw) {
		stream := raw[i]
		if stream > 2 {
			return string(raw)
		}
		size := int(raw[i+4])<<24 | int(raw[i+5])<<16 | int(raw[i+6])<<8 | int(raw[i+7])
		i += 8
		if size < 0 || i+size > len(raw) {
			return string(raw)
		}
		b.Write(raw[i : i+size])
		i += size
	}
	if b.Len() == 0 {
		return string(raw)
	}
	return b.String()
}

func resolveDockerEnv(db *gorm.DB, environmentID string) (*models.DockerEnvironment, error) {
	if db == nil {
		return nil, nil
	}
	id := strings.TrimSpace(environmentID)
	if id != "" {
		var env models.DockerEnvironment
		if err := db.Where("id = ?", id).First(&env).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("docker environment not found")
			}
			return nil, err
		}
		if env.IsDisabled {
			return nil, fmt.Errorf("docker environment is disabled")
		}
		return &env, nil
	}
	var env models.DockerEnvironment
	err := db.Where("is_default = ? AND is_disabled = ?", true, false).First(&env).Error
	if err == nil {
		return &env, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, nil
}

func ensureImage(ctx context.Context, cli *client.Client, img string) error {
	_, _, err := cli.ImageInspectWithRaw(ctx, img)
	if err == nil {
		return nil
	}
	reader, err := cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull %s: %w", img, err)
	}
	defer reader.Close()
	buf := make([]byte, 4096)
	for {
		_, rerr := reader.Read(buf)
		if rerr != nil {
			break
		}
	}
	return nil
}

func removeContainerByName(ctx context.Context, cli *client.Client, name string) error {
	f := filters.NewArgs()
	f.Add("name", name)
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return err
	}
	for _, c := range list {
		match := false
		for _, n := range c.Names {
			if strings.TrimPrefix(n, "/") == name {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		_ = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
	}
	return nil
}
