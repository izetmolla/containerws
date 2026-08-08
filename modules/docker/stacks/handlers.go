package stacks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/docker/environments"
	"github.com/izetmolla/containerws/packages/composecli"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"gorm.io/gorm"
)

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
	single.Post("/", cc.CreateAPI)
	single.Post("/validate", cc.ValidateAPI)
	single.Get("/:id", cc.GetAPI)
	single.Put("/:id", cc.UpdateAPI)
	single.Delete("/:id", cc.RemoveAPI)
	single.Post("/:id/deploy", cc.DeployAPI)
	single.Post("/:id/stop", cc.StopAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(fmt.Errorf("%s", msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}

func (cc *controller) respondComposeErr(c fiber.Ctx, res *composecli.Result) error {
	r := cc.app.Render()
	msg := res.Error()
	return r.Api(c,
		r.WithError(fmt.Errorf("%s", msg)),
		r.WithStatus(fiber.StatusBadRequest),
		r.WithErrorCode("COMPOSE_ERROR"),
		r.WithErrorData(fiber.Map{
			"command": res.Command,
			"stdout":  res.Stdout,
			"stderr":  res.Stderr,
		}),
	)
}

type stackRow struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	EnvironmentID   string `json:"environment_id"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
	TemplateID      *int   `json:"template_id,omitempty"`
	TemplateTitle   string `json:"template_title,omitempty"`
	ContainerCount  int    `json:"container_count"`
	RunningCount    int    `json:"running_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	ComposePreview  string `json:"compose_preview,omitempty"`
}

type stackDetail struct {
	stackRow
	ComposeYAML string              `json:"compose_yaml"`
	EnvFile     string              `json:"env_file"`
	Containers  []stackContainerRow `json:"containers"`
}

type stackContainerPort struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type stackContainerRow struct {
	ID        string               `json:"id"`
	ShortID   string               `json:"short_id"`
	Name      string               `json:"name"`
	State     string               `json:"state"`
	Status    string               `json:"status"`
	Image     string               `json:"image"`
	Service   string               `json:"service,omitempty"`
	Created   int64                `json:"created"`
	IPAddress string               `json:"ip_address,omitempty"`
	Ports     []stackContainerPort `json:"ports"`
}

func (cc *controller) resolveEnv(c fiber.Ctx) (*models.DockerEnvironment, error) {
	return environments.Resolve(cc.app.DB(), c.Query("environment_id"))
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	envID := ""
	if env != nil {
		envID = env.ID
	}

	var stacks []models.DockerStack
	q := cc.app.DB().Order("updated_at desc")
	if envID != "" {
		q = q.Where("environment_id = ?", envID)
	}
	if err := q.Find(&stacks).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	counts := map[string][2]int{}
	cli, err := dockerclient.ClientFor(env)
	if err == nil {
		ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
		defer cancel()
		list, listErr := cli.ContainerList(ctx, container.ListOptions{All: true})
		if listErr == nil {
			for _, it := range list {
				proj := it.Labels["com.docker.compose.project"]
				if proj == "" {
					continue
				}
				pair := counts[proj]
				pair[0]++
				if strings.EqualFold(it.State, "running") {
					pair[1]++
				}
				counts[proj] = pair
			}
		}
	}

	rows := make([]stackRow, 0, len(stacks))
	for _, s := range stacks {
		pair := counts[strings.ToLower(s.Name)]
		preview := s.ComposeYAML
		if len(preview) > 240 {
			preview = preview[:240] + "…"
		}
		rows = append(rows, stackRow{
			ID:             s.ID,
			Name:           s.Name,
			EnvironmentID:  s.EnvironmentID,
			Status:         s.Status,
			Message:        s.Message,
			TemplateID:     s.TemplateID,
			TemplateTitle:  s.TemplateTitle,
			ContainerCount: pair[0],
			RunningCount:   pair[1],
			CreatedAt:      s.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      s.UpdatedAt.Format(time.RFC3339),
			ComposePreview: preview,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type upsertBody struct {
	Name          string `json:"name"`
	ComposeYAML   string `json:"compose_yaml"`
	EnvFile       string `json:"env_file"`
	Deploy        *bool  `json:"deploy"`
	Pull          bool   `json:"pull"`
	Prune         *bool  `json:"prune"`
	TemplateID    *int   `json:"template_id"`
	TemplateTitle string `json:"template_title"`
}

type validateBody struct {
	Name        string `json:"name"`
	ComposeYAML string `json:"compose_yaml"`
	EnvFile     string `json:"env_file"`
}

func (cc *controller) ValidateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	var body validateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	yaml := strings.TrimSpace(body.ComposeYAML)
	if yaml == "" {
		return r.Api(c, r.WithError(fmt.Errorf("compose YAML is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	project := composecliSanitize(body.Name)
	if project == "" {
		project = "validate"
	}

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if res := composecli.Config(ctx, env, project, yaml, composecli.RunOptions{EnvFile: body.EnvFile}); res != nil {
		return cc.respondComposeErr(c, res)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"valid": true},
		"message": "Compose file is valid",
	}))
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if env == nil {
		return r.Api(c, r.WithError(fmt.Errorf("no docker environment selected")), r.WithStatus(fiber.StatusBadRequest))
	}

	var body upsertBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	name := composecliSanitize(body.Name)
	yaml := strings.TrimSpace(body.ComposeYAML)
	if name == "" {
		return r.Api(c, r.WithError(fmt.Errorf("name is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	if yaml == "" {
		return r.Api(c, r.WithError(fmt.Errorf("compose YAML is required")), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}

	ctxValidate, cancelValidate := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancelValidate()
	envOpt := composecli.RunOptions{EnvFile: body.EnvFile}
	if res := composecli.Config(ctxValidate, env, name, yaml, envOpt); res != nil {
		return cc.respondComposeErr(c, res)
	}

	var existing models.DockerStack
	err = cc.app.DB().Where("environment_id = ? AND name = ?", env.ID, name).First(&existing).Error
	if err == nil {
		return r.Api(c, r.WithError(fmt.Errorf("a stack named %q already exists in this environment", name)), r.WithStatus(fiber.StatusConflict), r.WithErrorCode("STACK_EXISTS"))
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	stack := models.DockerStack{
		Name:          name,
		EnvironmentID: env.ID,
		ComposeYAML:   yaml,
		EnvFile:       body.EnvFile,
		Status:        models.DockerStackStatusCreated,
		TemplateID:    body.TemplateID,
		TemplateTitle: strings.TrimSpace(body.TemplateTitle),
	}
	if err := cc.app.DB().Create(&stack).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	doDeploy := body.Deploy == nil || *body.Deploy
	if doDeploy {
		upOpt := composecli.RunOptions{
			EnvFile:         body.EnvFile,
			Pull:            body.Pull,
			NoRemoveOrphans: body.Prune != nil && !*body.Prune,
		}
		if res := composecli.Up(c.Context(), env, stack.Name, stack.ComposeYAML, upOpt); res != nil {
			stack.Status = models.DockerStackStatusError
			stack.Message = res.Error()
			_ = cc.app.DB().Save(&stack).Error
			return cc.respondComposeErr(c, res)
		}
		stack.Status = models.DockerStackStatusRunning
		stack.Message = ""
		_ = cc.app.DB().Save(&stack).Error
	}

	detail, err := cc.buildDetail(c.Context(), env, &stack)
	if err != nil {
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
			"data":    stack,
			"message": "Stack created",
		}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    detail,
		"message": "Stack created",
	}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	stack, err := cc.loadStack(c.Params("id"), env)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}
	detail, err := cc.buildDetail(c.Context(), env, stack)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": detail}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	stack, err := cc.loadStack(c.Params("id"), env)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}

	var body upsertBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	if yaml := strings.TrimSpace(body.ComposeYAML); yaml != "" {
		ctxValidate, cancelValidate := context.WithTimeout(c.Context(), 30*time.Second)
		defer cancelValidate()
		project := stack.Name
		if name := composecliSanitize(body.Name); name != "" {
			project = name
		}
		envOpt := composecli.RunOptions{EnvFile: body.EnvFile}
		if body.EnvFile == "" {
			envOpt.EnvFile = stack.EnvFile
		}
		if res := composecli.Config(ctxValidate, env, project, yaml, envOpt); res != nil {
			return cc.respondComposeErr(c, res)
		}
		stack.ComposeYAML = yaml
	}
	// Always accept env_file from body (including clearing it).
	stack.EnvFile = body.EnvFile
	if name := composecliSanitize(body.Name); name != "" && name != stack.Name {
		var clash models.DockerStack
		err = cc.app.DB().Where("environment_id = ? AND name = ? AND id <> ?", stack.EnvironmentID, name, stack.ID).First(&clash).Error
		if err == nil {
			return r.Api(c, r.WithError(fmt.Errorf("a stack named %q already exists", name)), r.WithStatus(fiber.StatusConflict))
		}
		stack.Name = name
	}
	if err := cc.app.DB().Save(stack).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	doDeploy := body.Deploy == nil || *body.Deploy
	if doDeploy {
		upOpt := composecli.RunOptions{
			EnvFile:         stack.EnvFile,
			Pull:            body.Pull,
			NoRemoveOrphans: body.Prune != nil && !*body.Prune,
		}
		if res := composecli.Up(c.Context(), env, stack.Name, stack.ComposeYAML, upOpt); res != nil {
			stack.Status = models.DockerStackStatusError
			stack.Message = res.Error()
			_ = cc.app.DB().Save(stack).Error
			return cc.respondComposeErr(c, res)
		}
		stack.Status = models.DockerStackStatusRunning
		stack.Message = ""
		_ = cc.app.DB().Save(stack).Error
	}

	detail, _ := cc.buildDetail(c.Context(), env, stack)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    detail,
		"message": "Stack updated",
	}))
}

func (cc *controller) DeployAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	stack, err := cc.loadStack(c.Params("id"), env)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}
	if res := composecli.Up(c.Context(), env, stack.Name, stack.ComposeYAML, composecli.RunOptions{EnvFile: stack.EnvFile}); res != nil {
		stack.Status = models.DockerStackStatusError
		stack.Message = res.Error()
		_ = cc.app.DB().Save(stack).Error
		return cc.respondComposeErr(c, res)
	}
	stack.Status = models.DockerStackStatusRunning
	stack.Message = ""
	_ = cc.app.DB().Save(stack).Error
	detail, _ := cc.buildDetail(c.Context(), env, stack)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    detail,
		"message": "Stack deployed",
	}))
}

func (cc *controller) StopAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	stack, err := cc.loadStack(c.Params("id"), env)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}
	if res := composecli.Down(c.Context(), env, stack.Name, stack.ComposeYAML, composecli.RunOptions{EnvFile: stack.EnvFile}); res != nil {
		stack.Status = models.DockerStackStatusError
		stack.Message = res.Error()
		_ = cc.app.DB().Save(stack).Error
		return cc.respondComposeErr(c, res)
	}
	stack.Status = models.DockerStackStatusStopped
	stack.Message = ""
	_ = cc.app.DB().Save(stack).Error
	detail, _ := cc.buildDetail(c.Context(), env, stack)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    detail,
		"message": "Stack stopped",
	}))
}

func (cc *controller) RemoveAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	env, err := cc.resolveEnv(c)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	stack, err := cc.loadStack(c.Params("id"), env)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusNotFound), r.WithErrorCode("NOT_FOUND"))
	}

	stack.Status = models.DockerStackStatusRemoving
	_ = cc.app.DB().Save(stack).Error

	if res := composecli.Down(c.Context(), env, stack.Name, stack.ComposeYAML, composecli.RunOptions{EnvFile: stack.EnvFile}); res != nil {
		// Still remove DB record when force=1
		if c.Query("force") != "1" && c.Query("force") != "true" {
			stack.Status = models.DockerStackStatusError
			stack.Message = res.Error()
			_ = cc.app.DB().Save(stack).Error
			return cc.respondComposeErr(c, res)
		}
	}
	if err := cc.app.DB().Delete(stack).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"id": stack.ID},
		"message": "Stack removed",
	}))
}

func (cc *controller) loadStack(id string, env *models.DockerEnvironment) (*models.DockerStack, error) {
	id = strings.TrimSpace(id)
	var stack models.DockerStack
	q := cc.app.DB().Where("id = ?", id)
	if env != nil {
		q = q.Where("environment_id = ?", env.ID)
	}
	if err := q.First(&stack).Error; err != nil {
		return nil, fmt.Errorf("stack not found")
	}
	return &stack, nil
}

func (cc *controller) buildDetail(ctx context.Context, env *models.DockerEnvironment, stack *models.DockerStack) (*stackDetail, error) {
	detail := &stackDetail{
		stackRow: stackRow{
			ID:            stack.ID,
			Name:          stack.Name,
			EnvironmentID: stack.EnvironmentID,
			Status:        stack.Status,
			Message:       stack.Message,
			TemplateID:    stack.TemplateID,
			TemplateTitle: stack.TemplateTitle,
			CreatedAt:     stack.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     stack.UpdatedAt.Format(time.RFC3339),
		},
		ComposeYAML: stack.ComposeYAML,
		EnvFile:     stack.EnvFile,
		Containers:  []stackContainerRow{},
	}
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return detail, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	f := filters.NewArgs()
	f.Add("label", "com.docker.compose.project="+strings.ToLower(stack.Name))
	list, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return detail, nil
	}
	running := 0
	for _, it := range list {
		name := ""
		if len(it.Names) > 0 {
			name = strings.TrimPrefix(it.Names[0], "/")
		}
		if strings.EqualFold(it.State, "running") {
			running++
		}
		short := it.ID
		if len(short) > 12 {
			short = short[:12]
		}
		ports := make([]stackContainerPort, 0, len(it.Ports))
		for _, p := range it.Ports {
			ports = append(ports, stackContainerPort{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}
		ip := ""
		if it.NetworkSettings != nil {
			for _, nw := range it.NetworkSettings.Networks {
				if nw == nil {
					continue
				}
				if strings.TrimSpace(nw.IPAddress) != "" {
					ip = nw.IPAddress
					break
				}
			}
		}
		detail.Containers = append(detail.Containers, stackContainerRow{
			ID:        it.ID,
			ShortID:   short,
			Name:      name,
			State:     it.State,
			Status:    it.Status,
			Image:     it.Image,
			Service:   it.Labels["com.docker.compose.service"],
			Created:   it.Created,
			IPAddress: ip,
			Ports:     ports,
		})
	}
	detail.ContainerCount = len(list)
	detail.RunningCount = running
	return detail, nil
}

func composecliSanitize(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}
