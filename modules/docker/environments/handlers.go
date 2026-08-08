package environments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/dockerclient"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts /api/docker/environments.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	_ = EnsureDefaultLocal(appClients.DB())
	api.Get("/", cc.ListAPI)
	api.Post("/", cc.CreateAPI)
	api.Get("/:id", cc.GetAPI)
	api.Put("/:id", cc.UpdateAPI)
	api.Delete("/:id", cc.DeleteAPI)
	api.Post("/:id/activate", cc.ActivateAPI)
	api.Post("/:id/test", cc.TestAPI)
}

// EnsureDefaultLocal seeds a unix:// socket environment when the table is empty
// and a local Docker socket (or DOCKER_HOST unix) is present.
// When a previously seeded Local unix env was marked disabled because the socket
// was missing at first boot (common on production before Softwares installs Docker),
// clear is_disabled once the socket appears so the sticky "disabled" state recovers.
func EnsureDefaultLocal(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var count int64
	if err := db.Model(&models.DockerEnvironment{}).Count(&count).Error; err != nil {
		return err
	}
	sock := dockerclient.SockPath()
	sockOK := false
	if _, err := os.Stat(sock); err == nil {
		sockOK = true
	}
	if count > 0 {
		return reconcileDisabledLocalUnix(db, sock, sockOK)
	}
	if !sockOK {
		// Still seed a local entry so the UI has something to edit; mark disabled if sock missing.
		env := models.DockerEnvironment{
			Name:        "Local",
			Description: seededLocalDisabledDesc,
			ConnType:    models.DockerConnUnix,
			SocketPath:  sock,
			IsDefault:   true,
			IsDisabled:  true,
		}
		env.Normalize()
		return db.Create(&env).Error
	}
	env := models.DockerEnvironment{
		Name:        "Local",
		Description: "Local Docker Engine via " + sock,
		ConnType:    models.DockerConnUnix,
		SocketPath:  sock,
		IsDefault:   true,
		IsDisabled:  false,
	}
	env.Normalize()
	return db.Create(&env).Error
}

const seededLocalDisabledDesc = "Local Docker Engine (unix socket)"

// reconcileDisabledLocalUnix re-enables the seeded Local unix environment when the
// Docker socket becomes available after first-boot seeding marked it disabled.
// Only recovers the auto-seeded sticky state (matching seed description), not a
// later manual disable.
func reconcileDisabledLocalUnix(db *gorm.DB, sock string, sockOK bool) error {
	if !sockOK {
		return nil
	}
	var env models.DockerEnvironment
	err := db.Where(
		"conn_type = ? AND is_disabled = ? AND is_default = ? AND description = ?",
		models.DockerConnUnix, true, true, seededLocalDisabledDesc,
	).Order("created_at asc").First(&env).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	path := strings.TrimSpace(env.SocketPath)
	if path == "" {
		path = sock
	}
	if path != sock {
		if _, err := os.Stat(path); err != nil {
			return nil
		}
	}
	updates := map[string]any{
		"is_disabled": false,
		"description": "Local Docker Engine via " + path,
	}
	if strings.TrimSpace(env.SocketPath) == "" {
		updates["socket_path"] = sock
	}
	return db.Model(&env).Updates(updates).Error
}

// Resolve returns the environment for handlers: query environment_id, else default, else nil (local sock).
func Resolve(db *gorm.DB, environmentID string) (*models.DockerEnvironment, error) {
	_ = EnsureDefaultLocal(db)
	id := strings.TrimSpace(environmentID)
	if id != "" {
		var env models.DockerEnvironment
		if err := db.Where("id = ?", id).First(&env).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fiber.NewError(fiber.StatusNotFound, "docker environment not found")
			}
			return nil, err
		}
		if env.IsDisabled {
			return nil, fiber.NewError(fiber.StatusBadRequest, "docker environment is disabled")
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
	err = db.Where("is_disabled = ?", false).Order("created_at asc").First(&env).Error
	if err == nil {
		return &env, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

// ClientFromQuery resolves ?environment_id= (or default) and returns an Engine client.
func ClientFromQuery(db *gorm.DB, environmentID string) (*models.DockerEnvironment, *client.Client, error) {
	env, err := Resolve(db, environmentID)
	if err != nil {
		return nil, nil, err
	}
	cli, err := dockerclient.ClientFor(env)
	if err != nil {
		return env, nil, err
	}
	return env, cli, nil
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	_ = EnsureDefaultLocal(db)
	var rows []models.DockerEnvironment
	if err := db.WithContext(c.Context()).Order("is_default desc, name asc").Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	out := make([]fiber.Map, 0, len(rows))
	for i := range rows {
		out = append(out, publicEnv(&rows[i], false))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var env models.DockerEnvironment
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&env).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": publicEnv(&env, true)}))
}

type upsertBody struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	ConnType        string `json:"conn_type"`
	SocketPath      string `json:"socket_path"`
	SSHHost         string `json:"ssh_host"`
	SSHPort         int    `json:"ssh_port"`
	SSHUser         string `json:"ssh_user"`
	SSHPrivateKey   string `json:"ssh_private_key"`
	SSHPassphrase   string `json:"ssh_passphrase"`
	SSHRemoteSocket string `json:"ssh_remote_socket"`
	TCPHost         string `json:"tcp_host"`
	TCPPort         int    `json:"tcp_port"`
	TLSCACert       string `json:"tls_ca_cert"`
	TLSCert         string `json:"tls_cert"`
	TLSKey          string `json:"tls_key"`
	TLSSkipVerify   bool   `json:"tls_skip_verify"`
	IsDefault       *bool  `json:"is_default"`
	IsDisabled      *bool  `json:"is_disabled"`
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	var body upsertBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	env := models.DockerEnvironment{}
	if err := applyUpsert(&env, body, true); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	ctx := c.Context()
	if env.IsDefault {
		_ = db.WithContext(ctx).Model(&models.DockerEnvironment{}).Where("is_default = ?", true).Update("is_default", false).Error
	}
	if err := db.WithContext(ctx).Create(&env).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	dockerclient.Reset()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    publicEnv(&env, true),
		"message": "Environment created",
	}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	var env models.DockerEnvironment
	if err := db.WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&env).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	var body upsertBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	if err := applyUpsert(&env, body, false); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("VALIDATION_ERROR"))
	}
	ctx := c.Context()
	if env.IsDefault {
		_ = db.WithContext(ctx).Model(&models.DockerEnvironment{}).
			Where("is_default = ? AND id <> ?", true, env.ID).
			Update("is_default", false).Error
	}
	if err := db.WithContext(ctx).Save(&env).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	dockerclient.Reset()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    publicEnv(&env, true),
		"message": "Environment updated",
	}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	id := c.Params("id")
	var env models.DockerEnvironment
	if err := db.WithContext(c.Context()).Where("id = ?", id).First(&env).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := db.WithContext(c.Context()).Delete(&env).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if env.IsDefault {
		var next models.DockerEnvironment
		if err := db.Where("is_disabled = ?", false).Order("created_at asc").First(&next).Error; err == nil {
			_ = db.Model(&next).Update("is_default", true).Error
		}
	}
	dockerclient.Reset()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "Environment deleted",
	}))
}

func (cc *controller) ActivateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	ctx := c.Context()
	var env models.DockerEnvironment
	if err := db.WithContext(ctx).Where("id = ?", c.Params("id")).First(&env).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if env.IsDisabled {
		return r.Api(c, r.WithError(errors.New("cannot activate a disabled environment")), r.WithStatus(fiber.StatusBadRequest))
	}
	_ = db.WithContext(ctx).Model(&models.DockerEnvironment{}).Where("is_default = ?", true).Update("is_default", false).Error
	_ = db.WithContext(ctx).Model(&env).Update("is_default", true).Error
	env.IsDefault = true
	dockerclient.Reset()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    publicEnv(&env, false),
		"message": "Environment activated",
	}))
}

func (cc *controller) TestAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var env models.DockerEnvironment
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", c.Params("id")).First(&env).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.Api(c, r.WithError(errors.New("not found")), r.WithStatus(fiber.StatusNotFound))
		}
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	err := dockerclient.PingEnv(ctx, &env)
	out := fiber.Map{
		"ok":       err == nil,
		"host_url": env.HostURL,
		"conn_type": env.ConnType,
	}
	if err != nil {
		out["error"] = err.Error()
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out, "message": "Connection failed"}))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out, "message": "Connection OK"}))
}

func applyUpsert(env *models.DockerEnvironment, body upsertBody, creating bool) error {
	name := strings.TrimSpace(body.Name)
	if creating && name == "" {
		return errors.New("name is required")
	}
	if name != "" {
		env.Name = name
	}
	if body.Description != "" || creating {
		env.Description = strings.TrimSpace(body.Description)
	}
	conn := strings.ToLower(strings.TrimSpace(body.ConnType))
	if creating && conn == "" {
		conn = models.DockerConnUnix
	}
	if conn != "" {
		switch conn {
		case models.DockerConnUnix, models.DockerConnSSH, models.DockerConnTLS:
			env.ConnType = conn
		default:
			return fmt.Errorf("conn_type must be unix, ssh, or tls")
		}
	}
	if body.SocketPath != "" || (creating && env.ConnType == models.DockerConnUnix) {
		env.SocketPath = strings.TrimSpace(body.SocketPath)
		if env.SocketPath == "" {
			env.SocketPath = dockerclient.SockPath()
		}
	}
	if body.SSHHost != "" || env.ConnType == models.DockerConnSSH {
		if body.SSHHost != "" {
			env.SSHHost = strings.TrimSpace(body.SSHHost)
		}
		if body.SSHPort > 0 {
			env.SSHPort = body.SSHPort
		}
		if body.SSHUser != "" {
			env.SSHUser = strings.TrimSpace(body.SSHUser)
		}
		if body.SSHPrivateKey != "" {
			env.SSHPrivateKey = body.SSHPrivateKey
		}
		if body.SSHPassphrase != "" {
			env.SSHPassphrase = body.SSHPassphrase
		}
		if body.SSHRemoteSocket != "" {
			env.SSHRemoteSocket = strings.TrimSpace(body.SSHRemoteSocket)
		}
	}
	if body.TCPHost != "" || env.ConnType == models.DockerConnTLS {
		if body.TCPHost != "" {
			env.TCPHost = strings.TrimSpace(body.TCPHost)
		}
		if body.TCPPort > 0 {
			env.TCPPort = body.TCPPort
		}
		if body.TLSCACert != "" {
			env.TLSCACert = body.TLSCACert
		}
		if body.TLSCert != "" {
			env.TLSCert = body.TLSCert
		}
		if body.TLSKey != "" {
			env.TLSKey = body.TLSKey
		}
		env.TLSSkipVerify = body.TLSSkipVerify
	}
	if body.IsDefault != nil {
		env.IsDefault = *body.IsDefault
	} else if creating {
		env.IsDefault = false
	}
	if body.IsDisabled != nil {
		env.IsDisabled = *body.IsDisabled
	}

	switch env.ConnType {
	case models.DockerConnSSH:
		if strings.TrimSpace(env.SSHHost) == "" {
			return errors.New("ssh_host is required")
		}
		if creating && strings.TrimSpace(env.SSHPrivateKey) == "" {
			return errors.New("ssh_private_key is required")
		}
	case models.DockerConnTLS:
		if strings.TrimSpace(env.TCPHost) == "" {
			return errors.New("tcp_host is required")
		}
	}
	env.Normalize()
	return nil
}

func publicEnv(env *models.DockerEnvironment, includeSecrets bool) fiber.Map {
	m := fiber.Map{
		"id":                env.ID,
		"name":              env.Name,
		"description":       env.Description,
		"conn_type":         env.ConnType,
		"host_url":          env.HostURL,
		"socket_path":       env.SocketPath,
		"ssh_host":          env.SSHHost,
		"ssh_port":          env.SSHPort,
		"ssh_user":          env.SSHUser,
		"ssh_remote_socket": env.SSHRemoteSocket,
		"tcp_host":          env.TCPHost,
		"tcp_port":          env.TCPPort,
		"tls_skip_verify":   env.TLSSkipVerify,
		"is_default":        env.IsDefault,
		"is_disabled":       env.IsDisabled,
		"created_at":        env.CreatedAt,
		"updated_at":        env.UpdatedAt,
		"has_ssh_key":       strings.TrimSpace(env.SSHPrivateKey) != "",
		"has_tls_ca":        strings.TrimSpace(env.TLSCACert) != "",
		"has_tls_cert":      strings.TrimSpace(env.TLSCert) != "",
		"has_tls_key":       strings.TrimSpace(env.TLSKey) != "",
	}
	if includeSecrets {
		m["ssh_private_key"] = env.SSHPrivateKey
		m["ssh_passphrase"] = env.SSHPassphrase
		m["tls_ca_cert"] = env.TLSCACert
		m["tls_cert"] = env.TLSCert
		m["tls_key"] = env.TLSKey
	}
	return m
}
