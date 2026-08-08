package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/docker/environments"
	"github.com/izetmolla/containerws/packages/composecli"
	"github.com/izetmolla/containerws/packages/dockerclient"
)

const defaultTemplatesURL = "https://raw.githubusercontent.com/portainer/templates/v3/templates.json"

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	list := api.Group("/list")
	list.Get("/", cc.ListAPI)

	single := api.Group("/single")
	single.Post("/deploy", cc.DeployAPI)
	single.Get("/:id", cc.GetAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(fmt.Errorf("%s", msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}

// ---- Portainer template types (v3) ----

type portainerCatalog struct {
	Version   string              `json:"version"`
	Templates []portainerTemplate `json:"templates"`
}

type portainerTemplate struct {
	ID            int                  `json:"id"`
	Type          int                  `json:"type"` // 1=container, 2=swarm stack, 3=compose stack
	Title         string               `json:"title"`
	Description   string               `json:"description"`
	Name          string               `json:"name,omitempty"`
	Logo          string               `json:"logo,omitempty"`
	Note          string               `json:"note,omitempty"`
	Platform      string               `json:"platform,omitempty"`
	Categories    []string             `json:"categories,omitempty"`
	Image         string               `json:"image,omitempty"`
	Registry      string               `json:"registry,omitempty"`
	Command       string               `json:"command,omitempty"`
	Hostname      string               `json:"hostname,omitempty"`
	Network       string               `json:"network,omitempty"`
	Privileged    bool                 `json:"privileged,omitempty"`
	Interactive   bool                 `json:"interactive,omitempty"`
	RestartPolicy string               `json:"restart_policy,omitempty"`
	Ports         []string             `json:"ports,omitempty"`
	Volumes       []portainerVolume    `json:"volumes,omitempty"`
	Env           []portainerEnv       `json:"env,omitempty"`
	Labels        []portainerLabel     `json:"labels,omitempty"`
	Repository    *portainerRepository `json:"repository,omitempty"`
}

type portainerVolume struct {
	Container string `json:"container"`
	Bind      string `json:"bind,omitempty"`
	Readonly  bool   `json:"readonly,omitempty"`
}

type portainerEnv struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Preset      bool   `json:"preset,omitempty"`
	Select      []struct {
		Text    string `json:"text"`
		Value   string `json:"value"`
		Default bool   `json:"default,omitempty"`
	} `json:"select,omitempty"`
}

type portainerLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type portainerRepository struct {
	URL       string `json:"url"`
	Stackfile string `json:"stackfile"`
}

var (
	catalogMu    sync.Mutex
	catalogCache *portainerCatalog
	catalogAt    time.Time
	catalogTTL   = 30 * time.Minute
)

func loadCatalog(force bool) (*portainerCatalog, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if !force && catalogCache != nil && time.Since(catalogAt) < catalogTTL {
		return catalogCache, nil
	}
	client := &http.Client{Timeout: 45 * time.Second}
	req, err := http.NewRequest(http.MethodGet, defaultTemplatesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "containerws")
	res, err := client.Do(req)
	if err != nil {
		if catalogCache != nil {
			return catalogCache, nil
		}
		return nil, fmt.Errorf("fetch portainer templates: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("fetch portainer templates: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var cat portainerCatalog
	if err := json.NewDecoder(res.Body).Decode(&cat); err != nil {
		return nil, fmt.Errorf("parse portainer templates: %w", err)
	}
	catalogCache = &cat
	catalogAt = time.Now()
	return catalogCache, nil
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	force := c.Query("refresh") == "1" || c.Query("refresh") == "true"
	cat, err := loadCatalog(force)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway), r.WithErrorCode("TEMPLATES_FETCH_FAILED"))
	}

	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	typeFilter := strings.TrimSpace(c.Query("type"))

	out := make([]fiber.Map, 0, len(cat.Templates))
	cats := map[string]int{}
	for _, t := range cat.Templates {
		for _, cname := range t.Categories {
			cats[cname]++
		}
		if typeFilter != "" {
			want := 0
			switch typeFilter {
			case "1", "container":
				want = 1
			case "2", "swarm":
				want = 2
			case "3", "stack", "compose":
				want = 3
			}
			if want != 0 && t.Type != want {
				continue
			}
		}
		if category != "" && category != "all" {
			ok := false
			for _, cname := range t.Categories {
				if strings.EqualFold(cname, category) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if q != "" {
			blob := strings.ToLower(t.Title + " " + t.Description + " " + strings.Join(t.Categories, " ") + " " + t.Image)
			if !strings.Contains(blob, q) {
				continue
			}
		}
		typeLabel := "container"
		switch t.Type {
		case 2:
			typeLabel = "swarm"
		case 3:
			typeLabel = "compose"
		}
		out = append(out, fiber.Map{
			"id":             t.ID,
			"type":           t.Type,
			"type_label":     typeLabel,
			"title":          t.Title,
			"description":    t.Description,
			"name":           t.Name,
			"logo":           t.Logo,
			"note":           t.Note,
			"platform":       t.Platform,
			"categories":     t.Categories,
			"image":          t.Image,
			"ports":          t.Ports,
			"env":            t.Env,
			"repository":     t.Repository,
			"interactive":    t.Interactive,
			"privileged":     t.Privileged,
			"restart_policy": t.RestartPolicy,
		})
	}

	catList := make([]fiber.Map, 0, len(cats))
	for name, count := range cats {
		catList = append(catList, fiber.Map{"name": name, "count": count})
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
		"meta": fiber.Map{
			"version":    cat.Version,
			"source":     defaultTemplatesURL,
			"total":      len(cat.Templates),
			"returned":   len(out),
			"categories": catList,
			"cached_at":  catalogAt.Format(time.RFC3339),
		},
	}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cat, err := loadCatalog(false)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}
	id := strings.TrimSpace(c.Params("id"))
	for _, t := range cat.Templates {
		if fmt.Sprintf("%d", t.ID) == id {
			return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": t}))
		}
	}
	return r.Api(c, r.WithError(fmt.Errorf("template %s not found", id)), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
}

type deployBody struct {
	TemplateID int               `json:"template_id"`
	Name       string            `json:"name"`
	Env        map[string]string `json:"env"`
}

func (cc *controller) DeployAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := environments.Resolve(cc.app.DB(), c.Query("environment_id"))
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if env == nil {
		return r.Api(c, r.WithError(fmt.Errorf("no docker environment selected")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body deployBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	cat, err := loadCatalog(false)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadGateway))
	}
	var tpl *portainerTemplate
	for i := range cat.Templates {
		if cat.Templates[i].ID == body.TemplateID {
			tpl = &cat.Templates[i]
			break
		}
	}
	if tpl == nil {
		return r.Api(c, r.WithError(fmt.Errorf("template %d not found", body.TemplateID)), r.WithStatus(fiber.StatusNotFound))
	}

	envMap := map[string]string{}
	for _, e := range tpl.Env {
		if e.Default != "" {
			envMap[e.Name] = e.Default
		}
		for _, opt := range e.Select {
			if opt.Default {
				envMap[e.Name] = opt.Value
			}
		}
	}
	maps.Copy(envMap, body.Env)

	switch tpl.Type {
	case 1:
		return cc.deployContainer(c, env, tpl, body.Name, envMap)
	case 2, 3:
		return cc.deployStack(c, env, tpl, body.Name, envMap)
	default:
		return r.Api(c, r.WithError(fmt.Errorf("unsupported template type %d", tpl.Type)), r.WithStatus(fiber.StatusBadRequest))
	}
}

func (cc *controller) deployContainer(c fiber.Ctx, env *models.DockerEnvironment, tpl *portainerTemplate, name string, envMap map[string]string) error {
	r := cc.app.Render()
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return cc.respondErr(c, err)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSpace(tpl.Name)
	}
	if name == "" {
		name = slugify(tpl.Title)
	}

	imageRef := strings.TrimSpace(tpl.Image)
	if imageRef == "" {
		return r.Api(c, r.WithError(fmt.Errorf("template has no image")), r.WithStatus(fiber.StatusBadRequest))
	}
	if reg := strings.TrimSpace(tpl.Registry); reg != "" && !strings.Contains(imageRef, "/") {
		imageRef = strings.TrimRight(reg, "/") + "/" + imageRef
	}

	envList := make([]string, 0, len(envMap))
	for k, v := range envMap {
		envList = append(envList, k+"="+v)
	}

	exposed, bindings, err := parsePorts(tpl.Ports)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	binds := make([]string, 0, len(tpl.Volumes))
	anonVolumes := map[string]struct{}{}
	for _, v := range tpl.Volumes {
		cont := strings.TrimSpace(v.Container)
		if cont == "" {
			continue
		}
		hostPath := strings.TrimSpace(v.Bind)
		if hostPath == "" {
			anonVolumes[cont] = struct{}{}
			continue
		}
		entry := hostPath + ":" + cont
		if v.Readonly {
			entry += ":ro"
		}
		binds = append(binds, entry)
	}

	labels := map[string]string{
		"com.containerws.template.id":    fmt.Sprintf("%d", tpl.ID),
		"com.containerws.template.title": tpl.Title,
	}
	for _, l := range tpl.Labels {
		if strings.TrimSpace(l.Name) != "" {
			labels[l.Name] = l.Value
		}
	}

	cfg := &container.Config{
		Image:        imageRef,
		Env:          envList,
		Labels:       labels,
		Hostname:     strings.TrimSpace(tpl.Hostname),
		ExposedPorts: exposed,
		Tty:          tpl.Interactive,
		OpenStdin:    tpl.Interactive,
		Volumes:      anonVolumes,
	}
	if cmd := strings.TrimSpace(tpl.Command); cmd != "" {
		cfg.Cmd = []string{"/bin/sh", "-c", cmd}
		if tpl.Interactive {
			cfg.Cmd = strings.Fields(cmd)
		}
	}

	host := &container.HostConfig{
		Binds:        binds,
		PortBindings: bindings,
		Privileged:   tpl.Privileged,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}
	if rp := strings.TrimSpace(tpl.RestartPolicy); rp != "" {
		host.RestartPolicy.Name = container.RestartPolicyMode(rp)
	}

	networking := &network.NetworkingConfig{}
	if netName := strings.TrimSpace(tpl.Network); netName != "" {
		networking.EndpointsConfig = map[string]*network.EndpointSettings{
			netName: {},
		}
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()

	// Pull image best-effort
	reader, pullErr := cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if pullErr == nil && reader != nil {
		_, _ = io.Copy(io.Discard, reader)
		_ = reader.Close()
	}

	resp, err := cli.ContainerCreate(ctx, cfg, host, networking, nil, name)
	if err != nil {
		return cc.respondErr(c, err)
	}
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"kind":         "container",
			"container_id": resp.ID,
			"name":         name,
			"image":        imageRef,
			"template_id":  tpl.ID,
		},
		"message": fmt.Sprintf("Deployed template %q as container %s", tpl.Title, name),
	}))
}

func (cc *controller) deployStack(c fiber.Ctx, env *models.DockerEnvironment, tpl *portainerTemplate, name string, envMap map[string]string) error {
	r := cc.app.Render()
	if tpl.Repository == nil || strings.TrimSpace(tpl.Repository.URL) == "" || strings.TrimSpace(tpl.Repository.Stackfile) == "" {
		return r.Api(c, r.WithError(fmt.Errorf("template has no repository stackfile")), r.WithStatus(fiber.StatusBadRequest))
	}

	rawURL, err := rawStackURL(tpl.Repository.URL, tpl.Repository.Stackfile)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	yamlBytes, err := httpGet(rawURL)
	if err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("download stackfile: %w", err)), r.WithStatus(fiber.StatusBadGateway), r.WithErrorCode("STACKFILE_FETCH_FAILED"))
	}
	yamlText := applyEnv(string(yamlBytes), envMap)

	name = strings.TrimSpace(name)
	if name == "" {
		name = slugify(tpl.Title)
	}
	name = slugify(name)

	tid := tpl.ID
	stack := models.DockerStack{
		Name:          name,
		EnvironmentID: env.ID,
		ComposeYAML:   yamlText,
		Status:        models.DockerStackStatusCreated,
		TemplateID:    &tid,
		TemplateTitle: tpl.Title,
	}
	var existing models.DockerStack
	if err := cc.app.DB().Where("environment_id = ? AND name = ?", env.ID, name).First(&existing).Error; err == nil {
		existing.ComposeYAML = yamlText
		existing.TemplateID = &tid
		existing.TemplateTitle = tpl.Title
		stack = existing
	} else {
		if err := cc.app.DB().Create(&stack).Error; err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
		}
	}

	if res := composecli.Up(c.Context(), env, stack.Name, stack.ComposeYAML); res != nil {
		stack.Status = models.DockerStackStatusError
		stack.Message = res.Error()
		_ = cc.app.DB().Save(&stack).Error
		return r.Api(c,
			r.WithError(fmt.Errorf("%s", res.Error())),
			r.WithStatus(fiber.StatusBadRequest),
			r.WithErrorCode("COMPOSE_ERROR"),
			r.WithErrorData(fiber.Map{
				"command":  res.Command,
				"stdout":   res.Stdout,
				"stderr":   res.Stderr,
				"stack_id": stack.ID,
			}),
		)
	}
	stack.Status = models.DockerStackStatusRunning
	stack.Message = ""
	_ = cc.app.DB().Save(&stack).Error

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"kind":        "stack",
			"stack_id":    stack.ID,
			"name":        stack.Name,
			"template_id": tpl.ID,
		},
		"message": fmt.Sprintf("Deployed template %q as stack %s", tpl.Title, stack.Name),
	}))
}

func rawStackURL(repoURL, stackfile string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	stackfile = strings.TrimLeft(strings.TrimSpace(stackfile), "/")
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid repository url: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("repository url must be github.com/owner/repo")
	}
	owner, repo := parts[0], parts[1]
	repo = strings.TrimSuffix(repo, ".git")
	ref := "master"
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, stackfile), nil
}

func httpGet(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "containerws")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d fetching %s: %s", res.StatusCode, rawURL, strings.TrimSpace(string(body)))
	}
	return body, nil
}

var envToken = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func applyEnv(yaml string, env map[string]string) string {
	return envToken.ReplaceAllStringFunc(yaml, func(m string) string {
		sub := envToken.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := env[sub[1]]; ok {
			return v
		}
		return m
	})
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "stack"
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func parsePorts(ports []string) (nat.PortSet, nat.PortMap, error) {
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, raw := range ports {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		proto := "tcp"
		parts := strings.Split(raw, "/")
		if len(parts) == 2 {
			proto = strings.ToLower(parts[1])
			raw = parts[0]
		}
		hp := strings.Split(raw, ":")
		var hostPort, containerPort string
		switch len(hp) {
		case 1:
			containerPort = hp[0]
		case 2:
			hostPort, containerPort = hp[0], hp[1]
		default:
			return nil, nil, fmt.Errorf("invalid port mapping: %s", raw)
		}
		p, err := nat.NewPort(proto, containerPort)
		if err != nil {
			return nil, nil, err
		}
		exposed[p] = struct{}{}
		if hostPort != "" {
			bindings[p] = append(bindings[p], nat.PortBinding{HostPort: hostPort})
		} else {
			bindings[p] = append(bindings[p], nat.PortBinding{})
		}
	}
	return exposed, bindings, nil
}
