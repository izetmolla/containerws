package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/goauth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gorm.io/gorm"
)

func init() {
	rootCmd.AddCommand(usersCmd)

	usersCmd.AddCommand(usersLsCmd)
	usersCmd.AddCommand(usersGetCmd)
	usersCmd.AddCommand(usersAddCmd)
	usersCmd.AddCommand(usersUpdateCmd)
	usersCmd.AddCommand(usersRmCmd)

	addUserFlags(usersAddCmd.Flags())
	usersAddCmd.Flags().String("password", "", "plaintext password (will be hashed)")
	_ = usersAddCmd.MarkFlagRequired("password")
	_ = usersAddCmd.MarkFlagRequired("email")

	addUserFlags(usersUpdateCmd.Flags())
	usersUpdateCmd.Flags().String("password", "", "new plaintext password (will be hashed)")
}

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Users management utility",
	Long:  `Manage Container Workspace users in the SQLite database.`,
	Args:  cobra.NoArgs,
}

var usersLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List users",
	Args:  cobra.NoArgs,
	RunE: withStore(func(cmd *cobra.Command, _ []string, appClients *config.AppClients) error {
		var users []models.User
		if err := appClients.DB().Order("created_at asc").Find(&users).Error; err != nil {
			return err
		}
		printUsers(users)
		return nil
	}),
}

var usersGetCmd = &cobra.Command{
	Use:   "get <id|email|username>",
	Short: "Show a single user",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		user, err := findUser(appClients.DB(), args[0])
		if err != nil {
			return err
		}
		printUsers([]models.User{*user})
		return nil
	}),
}

var usersAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a user",
	Args:  cobra.NoArgs,
	RunE: withStore(func(cmd *cobra.Command, _ []string, appClients *config.AppClients) error {
		user, err := userFromFlags(cmd.Flags(), true)
		if err != nil {
			return err
		}

		password, err := cmd.Flags().GetString("password")
		if err != nil {
			return err
		}
		hash, err := goauth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		user.Password = hash

		if err := appClients.DB().Create(user).Error; err != nil {
			return err
		}

		fmt.Printf("User created: %s (%s)\n", user.ID, displayName(user))
		printUsers([]models.User{*user})
		return nil
	}),
}

var usersUpdateCmd = &cobra.Command{
	Use:   "update <id|email|username>",
	Short: "Update a user",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		user, err := findUser(appClients.DB(), args[0])
		if err != nil {
			return err
		}

		if err := applyUserFlagUpdates(cmd.Flags(), user); err != nil {
			return err
		}

		if cmd.Flags().Changed("password") {
			password, err := cmd.Flags().GetString("password")
			if err != nil {
				return err
			}
			if strings.TrimSpace(password) == "" {
				return errors.New("password cannot be empty")
			}
			hash, err := goauth.HashPassword(password)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}
			user.Password = hash
		}

		if err := appClients.DB().Save(user).Error; err != nil {
			return err
		}

		fmt.Printf("User updated: %s (%s)\n", user.ID, displayName(user))
		printUsers([]models.User{*user})
		return nil
	}),
}

var usersRmCmd = &cobra.Command{
	Use:   "rm <id|email|username>",
	Short: "Soft-delete a user",
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		user, err := findUser(appClients.DB(), args[0])
		if err != nil {
			return err
		}
		if err := appClients.DB().Delete(user).Error; err != nil {
			return err
		}
		fmt.Printf("User deleted: %s (%s)\n", user.ID, displayName(user))
		return nil
	}),
}

func addUserFlags(flags *pflag.FlagSet) {
	flags.String("email", "", "email address")
	flags.String("username", "", "username (unique)")
	flags.String("firstname", "", "first name")
	flags.String("lastname", "", "last name")
	flags.String("organization-email", "", "organization email")
	flags.String("image", "", "avatar/image URL")
	flags.String("status", string(models.Active), "status: active|inactive|suspended|disabled|deleted|new|pending")
	flags.StringSlice("roles", nil, "roles (e.g. admin:rw,hr:r)")
	flags.Bool("confirmed", false, "mark email as confirmed")
}

func userFromFlags(flags *pflag.FlagSet, requireEmail bool) (*models.User, error) {
	email, _ := flags.GetString("email")
	email = strings.TrimSpace(email)
	if requireEmail && email == "" {
		return nil, errors.New("email is required")
	}

	username, _ := flags.GetString("username")
	firstname, _ := flags.GetString("firstname")
	lastname, _ := flags.GetString("lastname")
	orgEmail, _ := flags.GetString("organization-email")
	image, _ := flags.GetString("image")
	statusStr, _ := flags.GetString("status")
	roles, _ := flags.GetStringSlice("roles")
	confirmed, _ := flags.GetBool("confirmed")

	status, err := parseUserStatus(statusStr)
	if err != nil {
		return nil, err
	}

	return &models.User{
		Email:             email,
		Username:          strings.TrimSpace(username),
		FirstName:         strings.TrimSpace(firstname),
		LastName:          strings.TrimSpace(lastname),
		OrganizationEmail: strings.TrimSpace(orgEmail),
		Image:             strings.TrimSpace(image),
		Status:            status,
		Roles:             rolesToJSONB(roles),
		IsConfirmed:       confirmed,
	}, nil
}

func applyUserFlagUpdates(flags *pflag.FlagSet, user *models.User) error {
	if flags.Changed("email") {
		email, _ := flags.GetString("email")
		user.Email = strings.TrimSpace(email)
	}
	if flags.Changed("username") {
		username, _ := flags.GetString("username")
		user.Username = strings.TrimSpace(username)
	}
	if flags.Changed("firstname") {
		firstname, _ := flags.GetString("firstname")
		user.FirstName = strings.TrimSpace(firstname)
	}
	if flags.Changed("lastname") {
		lastname, _ := flags.GetString("lastname")
		user.LastName = strings.TrimSpace(lastname)
	}
	if flags.Changed("organization-email") {
		orgEmail, _ := flags.GetString("organization-email")
		user.OrganizationEmail = strings.TrimSpace(orgEmail)
	}
	if flags.Changed("image") {
		image, _ := flags.GetString("image")
		user.Image = strings.TrimSpace(image)
	}
	if flags.Changed("status") {
		statusStr, _ := flags.GetString("status")
		status, err := parseUserStatus(statusStr)
		if err != nil {
			return err
		}
		user.Status = status
	}
	if flags.Changed("roles") {
		roles, _ := flags.GetStringSlice("roles")
		user.Roles = rolesToJSONB(roles)
	}
	if flags.Changed("confirmed") {
		confirmed, _ := flags.GetBool("confirmed")
		user.IsConfirmed = confirmed
	}
	return nil
}

func parseUserStatus(s string) (models.UserStatus, error) {
	status := models.UserStatus(strings.ToLower(strings.TrimSpace(s)))
	switch status {
	case models.Active, models.Inactive, models.Suspended, models.Disabled,
		models.Deleted, models.New, models.Pending, "":
		if status == "" {
			return models.Active, nil
		}
		return status, nil
	default:
		return "", fmt.Errorf("invalid status %q (want active|inactive|suspended|disabled|deleted|new|pending)", s)
	}
}

func rolesToJSONB(roles []string) goauth.JSONBArray {
	out := make(goauth.JSONBArray, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		out = append(out, r)
	}
	if out == nil {
		return goauth.JSONBArray{}
	}
	return out
}

func formatRoles(roles goauth.JSONBArray) string {
	parts := goauth.FormatRoles(roles)
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

func findUser(db *gorm.DB, key string) (*models.User, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("empty user identifier")
	}

	var user models.User
	var q *gorm.DB
	if strings.Contains(key, "@") {
		q = db.Where("email = ? OR organization_email = ?", key, key)
	} else {
		q = db.Where("id = ? OR username = ? OR email = ?", key, key, key)
	}

	if err := q.First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found: %s", key)
		}
		return nil, err
	}
	return &user, nil
}

func displayName(u *models.User) string {
	switch {
	case u.Username != "":
		return u.Username
	case u.Email != "":
		return u.Email
	default:
		return u.ID
	}
}

func printUsers(users []models.User) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tUsername\tEmail\tName\tStatus\tConfirmed\tRoles\tCreated")

	for _, u := range users {
		name := strings.TrimSpace(u.FirstName + " " + u.LastName)
		if name == "" {
			name = "-"
		}
		username := u.Username
		if username == "" {
			username = "-"
		}
		email := u.Email
		if email == "" {
			email = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
			u.ID,
			username,
			email,
			name,
			u.Status,
			u.IsConfirmed,
			formatRoles(u.Roles),
			u.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	_ = w.Flush()
}
