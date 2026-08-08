package traefik

import (
	"context"
	"errors"
	"fmt"
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

// ApplyDocker ensures a Traefik container with generated config mounts.
func ApplyDocker(ctx context.Context, db *gorm.DB, settings *models.ProxySettings, configDir string) error {
	env, err := resolveDockerEnv(db, settings.DockerEnvironmentID)
	if err != nil {
		return err
	}
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}

	name := settings.TraefikContainerName
	if name == "" {
		name = "cws-proxy-traefik"
	}
	img := settings.TraefikImage
	if img == "" {
		img = "traefik:v3.3"
	}
	if err := ensureImage(ctx, cli, img); err != nil {
		return err
	}
	_ = removeContainerByName(ctx, cli, name)

	httpPort := fmt.Sprintf("%d", settings.HTTPPort)
	httpsPort := fmt.Sprintf("%d", settings.HTTPSPort)
	plan, err := dockerrun.BuildPlan(settings, httpPort, httpsPort)
	if err != nil {
		return err
	}

	cfg := &container.Config{
		Image: img,
		Cmd: []string{
			"--configFile=/etc/traefik/traefik.yml",
		},
		ExposedPorts: plan.ExposedPorts,
		Labels: map[string]string{
			"containerws.proxymanager": "traefik",
		},
	}
	host := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: configDir + "/traefik.yml",
				Target: "/etc/traefik/traefik.yml",
			},
			{
				Type:   mount.TypeBind,
				Source: configDir + "/dynamic",
				Target: "/etc/traefik/dynamic",
			},
			{
				Type:   mount.TypeBind,
				Source: configDir + "/ssl",
				Target: "/etc/traefik/ssl",
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
	return waitContainerRunning(ctx, cli, resp.ID, 20*time.Second)
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
