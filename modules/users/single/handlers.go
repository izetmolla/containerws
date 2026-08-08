package single

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/users/single/vnc"
	"github.com/izetmolla/containerws/modules/vncnovnc/install/adduser"
	"github.com/izetmolla/containerws/packages/linuxuser"
	"github.com/izetmolla/goauth"
)

type createUserBody struct {
	Username          string   `json:"username"`
	Email             string   `json:"email"`
	FirstName         string   `json:"first_name"`
	LastName          string   `json:"last_name"`
	OrganizationEmail string   `json:"organization_email"`
	Password          string   `json:"password"`
	Status            string   `json:"status"`
	Roles             []string `json:"roles"`
	IsConfirmed       bool     `json:"is_confirmed"`

	// Linux provisioning
	CreateLinux         bool     `json:"create_linux"`
	LinuxPassword       string   `json:"linux_password"`
	LinuxShell          string   `json:"linux_shell"`
	LinuxHome           string   `json:"linux_home"`
	LinuxGroups         []string `json:"linux_groups"`
	LinuxPrimaryGroup   string   `json:"linux_primary_group"`
	LinuxCreateHome     *bool    `json:"linux_create_home"`
	ForcePasswordChange bool     `json:"linux_force_password_change"`

	// VNC profile
	CreateVnc   bool   `json:"create_vnc"`
	VncPassword string `json:"vnc_password"`
	StartVnc    bool   `json:"start_vnc"`
}

type updateUserBody struct {
	Username          *string  `json:"username"`
	Email             *string  `json:"email"`
	FirstName         *string  `json:"first_name"`
	LastName          *string  `json:"last_name"`
	OrganizationEmail *string  `json:"organization_email"`
	Status            *string  `json:"status"`
	Roles             []string `json:"roles"`
	IsConfirmed       *bool    `json:"is_confirmed"`
	Image             *string  `json:"image"`
}

type passwordBody struct {
	Password string `json:"password"`
}

type linuxProvisionBody struct {
	Password            string   `json:"password"`
	Shell               string   `json:"shell"`
	HomeDir             string   `json:"home_dir"`
	Groups              []string `json:"groups"`
	PrimaryGroup        string   `json:"primary_group"`
	CreateHome          *bool    `json:"create_home"`
	ForcePasswordChange bool     `json:"force_password_change"`
	FullName            string   `json:"full_name"`
}

type linuxUpdateBody struct {
	FullName     *string  `json:"full_name"`
	Shell        *string  `json:"shell"`
	HomeDir      *string  `json:"home_dir"`
	PrimaryGroup *string  `json:"primary_group"`
	Groups       []string `json:"groups"`
	AppendGroups []string `json:"append_groups"`
	Password     *string  `json:"password"`
	Lock         *bool    `json:"lock"`
	MoveHome     bool     `json:"move_home"`
}

type groupsBody struct {
	Groups []string `json:"groups"`
}

func (cc *controller) CreateUserAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	var body createUserBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("INVALID_BODY"))
	}

	username := strings.TrimSpace(body.Username)
	email := strings.TrimSpace(body.Email)
	if username == "" && email == "" {
		return r.Api(c, r.WithError(errors.New("username or email is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	if strings.TrimSpace(body.Password) == "" {
		return r.Api(c, r.WithError(errors.New("password is required")), r.WithStatus(fiber.StatusBadRequest))
	}

	hash, err := goauth.HashPassword(body.Password)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	status := models.UserStatus(strings.ToLower(strings.TrimSpace(body.Status)))
	if status == "" {
		status = models.Active
	}

	user := models.User{
		Username:          username,
		Email:             email,
		FirstName:         strings.TrimSpace(body.FirstName),
		LastName:          strings.TrimSpace(body.LastName),
		OrganizationEmail: strings.TrimSpace(body.OrganizationEmail),
		Password:          hash,
		Status:            status,
		Roles:             toRoles(body.Roles),
		IsConfirmed:       body.IsConfirmed,
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	warnings := []string{}
	if body.CreateLinux {
		if username == "" {
			warnings = append(warnings, "skipped linux: username required")
		} else {
			createHome := true
			if body.LinuxCreateHome != nil {
				createHome = *body.LinuxCreateHome
			}
			linuxPass := strings.TrimSpace(body.LinuxPassword)
			if linuxPass == "" {
				linuxPass = body.Password
			}
			full := strings.TrimSpace(user.FirstName + " " + user.LastName)
			if _, err := linuxuser.Create(linuxuser.CreateOptions{
				Username:            username,
				Password:            linuxPass,
				FullName:            full,
				HomeDir:             body.LinuxHome,
				Shell:               body.LinuxShell,
				PrimaryGroup:        body.LinuxPrimaryGroup,
				Groups:              body.LinuxGroups,
				CreateHome:          createHome,
				ForcePasswordChange: body.ForcePasswordChange,
			}); err != nil {
				warnings = append(warnings, "linux: "+err.Error())
			}
		}
	}

	var vncSession *models.VncSession
	if body.CreateVnc {
		vncPass := strings.TrimSpace(body.VncPassword)
		if vncPass == "" {
			vncPass = body.Password
		}
		if len(vncPass) > 8 {
			vncPass = vncPass[:8]
		}
		session, w := vnc.EnsureSession(cc.app.DB(), &user, vncPass, body.StartVnc)
		if w != "" {
			warnings = append(warnings, w)
		}
		vncSession = session
	}

	payload := cc.userPayload(c, user)
	if vncSession != nil {
		payload["vnc_session"] = vnc.Payload(*vncSession, user.Username)
		payload["novnc_url"] = vncSession.ClientURL()
	}

	return r.Api(c, r.WithStatus(fiber.StatusCreated), r.WithData(fiber.Map{
		"data":     payload,
		"warnings": warnings,
		"message":  "User created",
	}))
}

func (cc *controller) GetUserAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": cc.userPayload(c, *user),
	}))
}

func (cc *controller) UpdateUserAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body updateUserBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}

	updates := map[string]any{}
	if body.Username != nil {
		updates["username"] = strings.TrimSpace(*body.Username)
	}
	if body.Email != nil {
		updates["email"] = strings.TrimSpace(*body.Email)
	}
	if body.FirstName != nil {
		updates["first_name"] = strings.TrimSpace(*body.FirstName)
	}
	if body.LastName != nil {
		updates["last_name"] = strings.TrimSpace(*body.LastName)
	}
	if body.OrganizationEmail != nil {
		updates["organization_email"] = strings.TrimSpace(*body.OrganizationEmail)
	}
	if body.Status != nil {
		updates["status"] = strings.ToLower(strings.TrimSpace(*body.Status))
	}
	if body.Roles != nil {
		updates["roles"] = toRoles(body.Roles)
	}
	if body.IsConfirmed != nil {
		updates["is_confirmed"] = *body.IsConfirmed
	}
	if body.Image != nil {
		updates["image"] = strings.TrimSpace(*body.Image)
	}
	if len(updates) == 0 {
		return r.Api(c, r.WithError(errors.New("no fields to update")), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := db.WithContext(ctx).Model(user).Updates(updates).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	_ = db.WithContext(ctx).First(user, "id = ?", user.ID)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cc.userPayload(c, *user),
		"message": "User updated",
	}))
}

func (cc *controller) DeleteUserAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}

	deleteLinux := c.Query("delete_linux") == "1" || c.Query("delete_linux") == "true"
	removeHome := c.Query("remove_home") == "1" || c.Query("remove_home") == "true"
	warnings := []string{}

	var session models.VncSession
	if err := db.WithContext(ctx).Where("user_id = ?", user.ID).First(&session).Error; err == nil {
		if user.Username != "" {
			_ = adduser.StopUserSession(user.Username, session.VncPort, session.NoVncPort)
			_ = adduser.RemovePortAssignment(user.Username)
		}
		_ = db.WithContext(ctx).Delete(&session).Error
	}

	if deleteLinux && user.Username != "" && user.Username != "root" {
		if err := linuxuser.Delete(user.Username, removeHome); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	if err := db.WithContext(ctx).Delete(user).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"message":  "User deleted",
		"warnings": warnings,
		"data":     fiber.Map{"id": user.ID},
	}))
}

func (cc *controller) SetPasswordAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body passwordBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if strings.TrimSpace(body.Password) == "" {
		return r.Api(c, r.WithError(errors.New("password is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	hash, err := goauth.HashPassword(body.Password)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	if err := db.WithContext(ctx).Model(user).Update("password", hash).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Panel password updated"}))
}

func (cc *controller) ProvisionLinuxAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if strings.TrimSpace(user.Username) == "" {
		return r.Api(c, r.WithError(errors.New("panel user needs a username for linux account")), r.WithStatus(fiber.StatusBadRequest))
	}
	var body linuxProvisionBody
	_ = c.Bind().Body(&body)
	createHome := true
	if body.CreateHome != nil {
		createHome = *body.CreateHome
	}
	full := strings.TrimSpace(body.FullName)
	if full == "" {
		full = strings.TrimSpace(user.FirstName + " " + user.LastName)
	}
	acc, err := linuxuser.Create(linuxuser.CreateOptions{
		Username:            user.Username,
		Password:            body.Password,
		FullName:            full,
		HomeDir:             body.HomeDir,
		Shell:               body.Shell,
		PrimaryGroup:        body.PrimaryGroup,
		Groups:              body.Groups,
		CreateHome:          createHome,
		ForcePasswordChange: body.ForcePasswordChange,
	})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError), r.WithErrorCode("LINUX_CREATE_FAILED"))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    acc,
		"message": "Linux user created",
	}))
}

func (cc *controller) UpdateLinuxAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	if user.Username == "" {
		return r.Api(c, r.WithError(errors.New("no username")), r.WithStatus(fiber.StatusBadRequest))
	}
	var body linuxUpdateBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	opts := linuxuser.UpdateOptions{
		FullName:     body.FullName,
		Shell:        body.Shell,
		HomeDir:      body.HomeDir,
		PrimaryGroup: body.PrimaryGroup,
		AppendGroups: body.AppendGroups,
		Password:     body.Password,
		Lock:         body.Lock,
		MoveHome:     body.MoveHome,
	}
	if body.Groups != nil {
		opts.Groups = &body.Groups
	}
	acc, err := linuxuser.Update(user.Username, opts)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    acc,
		"message": "Linux user updated",
	}))
}

func (cc *controller) DeleteLinuxAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	removeHome := c.Query("remove_home") == "1" || c.Query("remove_home") == "true"
	if err := linuxuser.Delete(user.Username, removeHome); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Linux user deleted"}))
}

func (cc *controller) SetLinuxPasswordAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body passwordBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := linuxuser.SetPassword(user.Username, body.Password); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"message": "Linux password updated"}))
}

func (cc *controller) SetLinuxGroupsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	var body groupsBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	if err := linuxuser.SetGroups(user.Username, body.Groups); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	acc, _ := linuxuser.Lookup(user.Username)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    acc,
		"message": "Groups updated",
	}))
}

func (cc *controller) LockLinuxAPI(c fiber.Ctx) error {
	return cc.setLinuxLock(c, true)
}

func (cc *controller) UnlockLinuxAPI(c fiber.Ctx) error {
	return cc.setLinuxLock(c, false)
}

func (cc *controller) setLinuxLock(c fiber.Ctx, lock bool) error {
	r := cc.app.Render()
	user, err := cc.loadUser(c)
	if err != nil {
		return cc.respondLoadErr(c, err)
	}
	acc, err := linuxuser.Update(user.Username, linuxuser.UpdateOptions{Lock: &lock})
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	msg := "Linux account unlocked"
	if lock {
		msg = "Linux account locked"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": acc, "message": msg}))
}
