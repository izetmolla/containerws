// Package linuxuser manages local Linux accounts and groups for the panel.
package linuxuser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"
)

// CommonGroups are suggested supplementary groups for the UI.
// Missing entries from this list are created with groupadd when assigned.
var CommonGroups = []string{
	"sudo", "wheel", "docker", "adm", "netdev", "www-data", "video", "audio",
	"plugdev", "users", "staff", "lxd", "systemd-journal",
}

var commonGroupSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(CommonGroups))
	for _, g := range CommonGroups {
		m[g] = struct{}{}
	}
	return m
}()


// CommonShells are typical login shells offered in the UI.
var CommonShells = []string{
	"/bin/bash",
	"/bin/sh",
	"/bin/zsh",
	"/usr/bin/fish",
	"/usr/sbin/nologin",
	"/bin/false",
}

// Account describes a local Linux user.
type Account struct {
	Username     string   `json:"username"`
	UID          string   `json:"uid"`
	GID          string   `json:"gid"`
	Name         string   `json:"name"` // GECOS
	HomeDir      string   `json:"home_dir"`
	Shell        string   `json:"shell"`
	Groups       []string `json:"groups"`
	PrimaryGroup string   `json:"primary_group"`
	Exists       bool     `json:"exists"`
	Locked       bool     `json:"locked,omitempty"`
}

// CreateOptions for useradd.
type CreateOptions struct {
	Username            string
	Password            string
	FullName            string
	HomeDir             string
	Shell               string
	PrimaryGroup        string
	Groups              []string // supplementary
	UID                 int      // 0 = auto
	CreateHome          bool
	SystemAccount       bool
	NoUserGroup         bool
	ExpireDays          int // -1 = none
	ForcePasswordChange bool
}

// UpdateOptions for usermod / related tools.
type UpdateOptions struct {
	FullName     *string
	HomeDir      *string
	Shell        *string
	PrimaryGroup *string
	Groups       *[]string // replace supplementary groups
	AppendGroups []string  // usermod -aG
	Password     *string
	Lock         *bool
	MoveHome     bool
}

// Lookup returns Linux account details if the user exists.
func Lookup(username string) (*Account, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return &Account{Exists: false}, nil
	}
	u, err := user.Lookup(username)
	if err != nil {
		return &Account{Username: username, Exists: false}, nil
	}
	groups, primary := userGroups(username, u.Gid)
	locked, _ := isLocked(username)
	return &Account{
		Username:     u.Username,
		UID:          u.Uid,
		GID:          u.Gid,
		Name:         u.Name,
		HomeDir:      u.HomeDir,
		Shell:        lookupShell(username),
		Groups:       groups,
		PrimaryGroup: primary,
		Exists:       true,
		Locked:       locked,
	}, nil
}

// Create runs useradd (+ optional chpasswd / groups / passwd expire).
func Create(opts CreateOptions) (*Account, error) {
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if !validUsername(username) {
		return nil, fmt.Errorf("invalid linux username %q", username)
	}
	if _, err := user.Lookup(username); err == nil {
		return nil, fmt.Errorf("linux user %q already exists", username)
	}

	args := []string{}
	if opts.CreateHome {
		args = append(args, "-m")
	} else {
		args = append(args, "-M")
	}
	if opts.SystemAccount {
		args = append(args, "-r")
	}
	shell := strings.TrimSpace(opts.Shell)
	if shell == "" {
		shell = "/bin/bash"
	}
	args = append(args, "-s", shell)
	if home := strings.TrimSpace(opts.HomeDir); home != "" {
		args = append(args, "-d", home)
	}
	if name := strings.TrimSpace(opts.FullName); name != "" {
		args = append(args, "-c", name)
	}
	if opts.UID > 0 {
		args = append(args, "-u", strconv.Itoa(opts.UID))
	}
	if g := strings.TrimSpace(opts.PrimaryGroup); g != "" {
		args = append(args, "-g", g)
	}
	if opts.NoUserGroup {
		args = append(args, "-N")
	}
	suppl := uniqueNonEmpty(opts.Groups)
	if err := EnsureGroups(suppl); err != nil {
		return nil, err
	}
	if len(suppl) > 0 {
		args = append(args, "-G", strings.Join(suppl, ","))
	}
	args = append(args, username)

	if out, err := exec.Command("useradd", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("useradd: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	if pwd := strings.TrimSpace(opts.Password); pwd != "" {
		if err := SetPassword(username, pwd); err != nil {
			return nil, err
		}
	}
	if opts.ForcePasswordChange {
		_ = exec.Command("passwd", "-e", username).Run()
	}

	return Lookup(username)
}

// Update modifies an existing Linux account.
func Update(username string, opts UpdateOptions) (*Account, error) {
	username = strings.TrimSpace(username)
	if _, err := user.Lookup(username); err != nil {
		return nil, fmt.Errorf("linux user %q not found", username)
	}

	args := []string{}
	if opts.FullName != nil {
		args = append(args, "-c", strings.TrimSpace(*opts.FullName))
	}
	if opts.Shell != nil {
		args = append(args, "-s", strings.TrimSpace(*opts.Shell))
	}
	if opts.HomeDir != nil {
		args = append(args, "-d", strings.TrimSpace(*opts.HomeDir))
		if opts.MoveHome {
			args = append(args, "-m")
		}
	}
	if opts.PrimaryGroup != nil {
		args = append(args, "-g", strings.TrimSpace(*opts.PrimaryGroup))
	}
	if opts.Groups != nil {
		groups := uniqueNonEmpty(*opts.Groups)
		if err := EnsureGroups(groups); err != nil {
			return nil, err
		}
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if len(args) > 0 {
		args = append(args, username)
		if out, err := exec.Command("usermod", args...).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("usermod: %w (%s)", err, strings.TrimSpace(string(out)))
		}
	}
	appendGroups := uniqueNonEmpty(opts.AppendGroups)
	if err := EnsureGroups(appendGroups); err != nil {
		return nil, err
	}
	for _, g := range appendGroups {
		if out, err := exec.Command("usermod", "-aG", g, username).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("usermod -aG %s: %w (%s)", g, err, strings.TrimSpace(string(out)))
		}
	}
	if opts.Password != nil && strings.TrimSpace(*opts.Password) != "" {
		if err := SetPassword(username, *opts.Password); err != nil {
			return nil, err
		}
	}
	if opts.Lock != nil {
		if *opts.Lock {
			_ = exec.Command("usermod", "-L", username).Run()
		} else {
			_ = exec.Command("usermod", "-U", username).Run()
		}
	}
	return Lookup(username)
}

// Delete removes a Linux user (optionally purge home).
func Delete(username string, removeHome bool) error {
	username = strings.TrimSpace(username)
	if username == "" || username == "root" {
		return fmt.Errorf("refusing to delete %q", username)
	}
	args := []string{}
	if removeHome {
		args = append(args, "-r")
	}
	args = append(args, username)
	if out, err := exec.Command("userdel", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("userdel: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetPassword sets the Linux login password via chpasswd.
func SetPassword(username, password string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return fmt.Errorf("username and password required")
	}
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chpasswd: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetGroups replaces supplementary groups (keeps primary).
func SetGroups(username string, groups []string) error {
	_, err := Update(username, UpdateOptions{Groups: &groups})
	return err
}

// ListGroups returns local group names from /etc/group (sorted).
func ListGroups() ([]string, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return append([]string(nil), CommonGroups...), nil
	}
	defer f.Close()
	seen := map[string]struct{}{}
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, _, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	sort.Strings(out)
	return out, nil
}

// FormGroups returns groups for the users UI select: suggested common groups
// first (including ones that can be auto-created), then remaining system groups.
func FormGroups() []string {
	system, err := ListGroups()
	if err != nil || len(system) == 0 {
		return append([]string(nil), CommonGroups...)
	}
	sysSet := make(map[string]struct{}, len(system))
	for _, g := range system {
		sysSet[g] = struct{}{}
	}

	seen := map[string]struct{}{}
	var out []string
	for _, g := range CommonGroups {
		if _, ok := seen[g]; ok {
			continue
		}
		// Include if it exists on the host OR we are allowed to create it.
		if _, exists := sysSet[g]; exists || isEnsureableGroup(g) {
			seen[g] = struct{}{}
			out = append(out, g)
		}
	}
	for _, g := range system {
		if _, ok := seen[g]; ok {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	return out
}

// GroupExists reports whether name is present in /etc/group.
func GroupExists(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	_, err := user.LookupGroup(name)
	return err == nil
}

func isEnsureableGroup(name string) bool {
	_, ok := commonGroupSet[strings.TrimSpace(name)]
	return ok
}

// EnsureGroup creates the group with groupadd when missing.
// Only CommonGroups may be auto-created; unknown missing groups return an error.
func EnsureGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if GroupExists(name) {
		return nil
	}
	if !isEnsureableGroup(name) {
		return fmt.Errorf("group %q does not exist on this system", name)
	}
	if out, err := exec.Command("groupadd", name).CombinedOutput(); err != nil {
		// Race: another process may have created it.
		if GroupExists(name) {
			return nil
		}
		return fmt.Errorf("groupadd %s: %w (%s)", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureGroups ensures every group exists (creating CommonGroups when needed).
func EnsureGroups(groups []string) error {
	for _, g := range uniqueNonEmpty(groups) {
		if err := EnsureGroup(g); err != nil {
			return err
		}
	}
	return nil
}

func userGroups(username, gid string) (all []string, primary string) {
	if g, err := user.LookupGroupId(gid); err == nil {
		primary = g.Name
	}
	// Prefer `id -nG` for accurate membership.
	out, err := exec.Command("id", "-nG", username).Output()
	if err == nil {
		for g := range strings.FieldsSeq(string(out)) {
			all = append(all, g)
		}
		return all, primary
	}
	if primary != "" {
		all = []string{primary}
	}
	return all, primary
}

// ListLoginAccounts returns local accounts suitable for panel sync:
// root and users with UID >= 1000 that have a real login shell.
func ListLoginAccounts() ([]Account, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	var out []Account
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		name := parts[0]
		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		shell := parts[6]
		if !isSyncableLoginUser(name, uid, shell) {
			continue
		}
		acc, err := Lookup(name)
		if err != nil || acc == nil || !acc.Exists {
			continue
		}
		out = append(out, *acc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

func isSyncableLoginUser(name string, uid int, shell string) bool {
	name = strings.TrimSpace(name)
	if name == "" || !validUsername(name) {
		return false
	}
	if _, skip := syncSkipUsers[name]; skip {
		return false
	}
	shell = strings.TrimSpace(shell)
	switch shell {
	case "/usr/sbin/nologin", "/sbin/nologin", "/bin/false", "/usr/bin/nologin":
		return false
	}
	if name == "root" {
		return true
	}
	return uid >= 1000
}

// syncSkipUsers are never imported into the panel, even if UID looks "normal".
var syncSkipUsers = map[string]struct{}{
	"nobody": {}, "nfsnobody": {}, "nogroup": {},
	"sync": {}, "shutdown": {}, "halt": {}, "operator": {},
}

func lookupShell(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	prefix := username + ":"
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			return parts[6]
		}
	}
	return ""
}

func isLocked(username string) (bool, error) {
	f, err := os.Open("/etc/shadow")
	if err != nil {
		return false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), ":", 3)
		if len(parts) < 2 || parts[0] != username {
			continue
		}
		h := parts[1]
		return h == "!" || h == "*" || strings.HasPrefix(h, "!") || strings.HasPrefix(h, "*"), nil
	}
	return false, nil
}

func validUsername(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || r == '_') {
				return false
			}
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
