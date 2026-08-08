package keys

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/linuxuser"
	"gorm.io/gorm"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesAPI mounts per-user SSH key routes under /users/single.
func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/:id/keys", cc.GetKeysAPI)
	api.Post("/:id/keys/authorized", cc.AddAuthorizedKeyAPI)
	api.Delete("/:id/keys/authorized/:index", cc.RemoveAuthorizedKeyAPI)
	api.Post("/:id/keys/identity", cc.GenerateIdentityAPI)
	api.Delete("/:id/keys/identity", cc.DeleteIdentityAPI)
	api.Post("/:id/keys/identity/authorize", cc.AuthorizeIdentityAPI)
	api.Get("/:id/keys/sessions", cc.ListSessionsAPI)
	api.Delete("/:id/keys/sessions/:sessionId", cc.KillSessionAPI)
}

type addAuthorizedBody struct {
	Key     string `json:"key"`
	Comment string `json:"comment"`
}

type generateIdentityBody struct {
	Type       string `json:"type"`
	Comment    string `json:"comment"`
	Passphrase string `json:"passphrase"`
	Overwrite  bool   `json:"overwrite"`
	Bits       int    `json:"bits"`
}

func (cc *controller) GetKeysAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	includePrivate := strings.EqualFold(c.Query("include_private"), "true") ||
		c.Query("include_private") == "1"
	st, err := linuxuser.SSHKeys(user.Username, includePrivate)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": st}))
}

func (cc *controller) AddAuthorizedKeyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body addAuthorizedBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}
	st, err := linuxuser.AddAuthorizedKey(user.Username, body.Key, body.Comment)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    st,
		"message": "Authorized key added",
	}))
}

func (cc *controller) RemoveAuthorizedKeyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	index, err := strconv.Atoi(strings.TrimSpace(c.Params("index")))
	if err != nil {
		return r.Api(c, r.WithError(errors.New("invalid key index")), r.WithStatus(fiber.StatusBadRequest))
	}
	st, err := linuxuser.RemoveAuthorizedKey(user.Username, index)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    st,
		"message": "Authorized key removed",
	}))
}

func (cc *controller) GenerateIdentityAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body generateIdentityBody
	_ = c.Bind().Body(&body)
	st, err := linuxuser.GenerateIdentity(user.Username, linuxuser.GenerateIdentityOptions{
		Type:       body.Type,
		Comment:    body.Comment,
		Passphrase: body.Passphrase,
		Overwrite:  body.Overwrite,
		Bits:       body.Bits,
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    st,
		"message": "SSH identity keypair generated",
	}))
}

func (cc *controller) DeleteIdentityAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	st, err := linuxuser.DeleteIdentity(user.Username)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    st,
		"message": "SSH identity keypair deleted",
	}))
}

func (cc *controller) AuthorizeIdentityAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	st, err := linuxuser.AuthorizeIdentityPublicKey(user.Username)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    st,
		"message": "Identity public key added to authorized_keys",
	}))
}

func (cc *controller) ListSessionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	sessions, err := linuxuser.ListSSHConnections(user.Username)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if sessions == nil {
		sessions = []linuxuser.SSHConnection{}
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": sessions,
	}))
}

func (cc *controller) KillSessionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	sessionID := strings.TrimSpace(c.Params("sessionId"))
	if sessionID == "" {
		return r.Api(c, r.WithError(errors.New("session id is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := linuxuser.KillSSHConnection(user.Username, sessionID); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	sessions, _ := linuxuser.ListSSHConnections(user.Username)
	if sessions == nil {
		sessions = []linuxuser.SSHConnection{}
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    sessions,
		"message": "SSH session terminated",
	}))
}

func (cc *controller) loadUser(c fiber.Ctx) (*models.User, error) {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}
	var user models.User
	if err := cc.app.DB().WithContext(c.Context()).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiber.NewError(fiber.StatusNotFound, "user not found")
		}
		return nil, err
	}
	if strings.TrimSpace(user.Username) == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "user has no linux username")
	}
	return &user, nil
}

func (cc *controller) respondLoadErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return r.Api(c, r.WithError(err), r.WithStatus(fe.Code), r.WithErrorCode("ERROR"))
	}
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
}
