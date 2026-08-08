package linuxuser

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	IdentityED25519 = "id_ed25519"
	IdentityRSA     = "id_rsa"
)

// AuthorizedKey is one parsed line from ~/.ssh/authorized_keys.
type AuthorizedKey struct {
	Index       int      `json:"index"`
	Type        string   `json:"type"`
	Fingerprint string   `json:"fingerprint"`
	Comment     string   `json:"comment"`
	Options     []string `json:"options,omitempty"`
	Line        string   `json:"line"`
}

// IdentityKey describes the user's primary SSH identity keypair.
type IdentityKey struct {
	Exists       bool   `json:"exists"`
	Type         string `json:"type,omitempty"`
	PrivatePath  string `json:"private_path,omitempty"`
	PublicPath   string `json:"public_path,omitempty"`
	PublicKey    string `json:"public_key,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	Comment      string `json:"comment,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"` // only when explicitly requested
	HasPrivate   bool   `json:"has_private"`
	HasPublic    bool   `json:"has_public"`
}

// SSHKeysStatus is the panel payload for a Linux user's SSH keys.
type SSHKeysStatus struct {
	Username            string          `json:"username"`
	HomeDir             string          `json:"home_dir"`
	SSHDir              string          `json:"ssh_dir"`
	SSHDirExists        bool            `json:"ssh_dir_exists"`
	AuthorizedKeysPath  string          `json:"authorized_keys_path"`
	AuthorizedKeys      []AuthorizedKey `json:"authorized_keys"`
	AuthorizedKeysCount int             `json:"authorized_keys_count"`
	Identity            IdentityKey     `json:"identity"`
}

// SSHKeys returns authorized_keys + identity keypair status for username.
func SSHKeys(username string, includePrivate bool) (*SSHKeysStatus, error) {
	acc, err := Lookup(username)
	if err != nil {
		return nil, err
	}
	if acc == nil || !acc.Exists {
		return nil, fmt.Errorf("linux user %q does not exist", strings.TrimSpace(username))
	}
	home := strings.TrimSpace(acc.HomeDir)
	if home == "" {
		return nil, fmt.Errorf("linux user %q has no home directory", acc.Username)
	}

	sshDir := filepath.Join(home, ".ssh")
	authPath := filepath.Join(sshDir, "authorized_keys")
	st := &SSHKeysStatus{
		Username:           acc.Username,
		HomeDir:            home,
		SSHDir:             sshDir,
		SSHDirExists:       dirExists(sshDir),
		AuthorizedKeysPath: authPath,
		AuthorizedKeys:     []AuthorizedKey{},
	}

	keys, err := readAuthorizedKeys(authPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	st.AuthorizedKeys = keys
	st.AuthorizedKeysCount = len(keys)
	st.Identity = readIdentity(sshDir, includePrivate)
	return st, nil
}

// AddAuthorizedKey appends one OpenSSH public key line to authorized_keys.
// key may be a full line (with optional options/comment) or just the key blob line.
// optionalComment overrides the comment when the pasted line has none.
func AddAuthorizedKey(username, key, optionalComment string) (*SSHKeysStatus, error) {
	acc, err := requireAccount(username)
	if err != nil {
		return nil, err
	}
	line, err := normalizeAuthorizedKeyLine(key, optionalComment)
	if err != nil {
		return nil, err
	}

	sshDir, authPath, err := ensureSSHDir(acc)
	if err != nil {
		return nil, err
	}

	existing, err := readAuthorizedKeys(authPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	fp := fingerprintFromLine(line)
	for _, k := range existing {
		if k.Fingerprint != "" && fp != "" && k.Fingerprint == fp {
			return nil, fmt.Errorf("this public key is already authorized (%s)", fp)
		}
		if strings.TrimSpace(k.Line) == line {
			return nil, fmt.Errorf("this public key is already authorized")
		}
	}

	f, err := os.OpenFile(authPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open authorized_keys: %w", err)
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return nil, err
	}
	_ = f.Close()
	_ = os.Chmod(authPath, 0o600)
	_ = chownPath(authPath, acc)
	_ = chownPath(sshDir, acc)

	return SSHKeys(acc.Username, false)
}

// RemoveAuthorizedKey removes the authorized key at index (0-based).
func RemoveAuthorizedKey(username string, index int) (*SSHKeysStatus, error) {
	acc, err := requireAccount(username)
	if err != nil {
		return nil, err
	}
	authPath := filepath.Join(acc.HomeDir, ".ssh", "authorized_keys")
	keys, err := readAuthorizedKeys(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no authorized_keys file")
		}
		return nil, err
	}
	if index < 0 || index >= len(keys) {
		return nil, fmt.Errorf("invalid key index %d", index)
	}

	var kept []string
	for i, k := range keys {
		if i == index {
			continue
		}
		kept = append(kept, k.Line)
	}
	content := ""
	if len(kept) > 0 {
		content = strings.Join(kept, "\n") + "\n"
	}
	if err := os.WriteFile(authPath, []byte(content), 0o600); err != nil {
		return nil, err
	}
	_ = os.Chmod(authPath, 0o600)
	_ = chownPath(authPath, acc)

	return SSHKeys(acc.Username, false)
}

// GenerateIdentityOptions controls identity keypair creation.
type GenerateIdentityOptions struct {
	Type       string // ed25519 (default) or rsa
	Comment    string
	Passphrase string
	Overwrite  bool
	Bits       int // RSA only; default 4096
}

// GenerateIdentity creates ~/.ssh/id_ed25519 (or id_rsa) for the user.
// The returned status includes private_key once so the UI can offer copy/download.
func GenerateIdentity(username string, opts GenerateIdentityOptions) (*SSHKeysStatus, error) {
	acc, err := requireAccount(username)
	if err != nil {
		return nil, err
	}
	keyType := strings.ToLower(strings.TrimSpace(opts.Type))
	if keyType == "" {
		keyType = "ed25519"
	}
	var baseName string
	switch keyType {
	case "ed25519":
		baseName = IdentityED25519
	case "rsa":
		baseName = IdentityRSA
		if opts.Bits <= 0 {
			opts.Bits = 4096
		}
		if opts.Bits < 2048 {
			return nil, fmt.Errorf("rsa bits must be at least 2048")
		}
	default:
		return nil, fmt.Errorf("unsupported key type %q (use ed25519 or rsa)", opts.Type)
	}

	sshDir, _, err := ensureSSHDir(acc)
	if err != nil {
		return nil, err
	}
	privPath := filepath.Join(sshDir, baseName)
	pubPath := privPath + ".pub"
	if !opts.Overwrite {
		if fileExists(privPath) || fileExists(pubPath) {
			return nil, fmt.Errorf("identity key already exists at %s (pass overwrite=true to replace)", privPath)
		}
	}

	comment := strings.TrimSpace(opts.Comment)
	if comment == "" {
		comment = acc.Username + "@containerws"
	}

	var privBytes, pubBytes []byte
	switch keyType {
	case "ed25519":
		privBytes, pubBytes, err = generateED25519(comment, opts.Passphrase)
	case "rsa":
		privBytes, pubBytes, err = generateRSA(opts.Bits, comment, opts.Passphrase)
	}
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(privPath, privBytes, 0o600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}
	_ = os.Chmod(privPath, 0o600)
	_ = chownPath(privPath, acc)

	if err := os.WriteFile(pubPath, pubBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write public key: %w", err)
	}
	_ = os.Chmod(pubPath, 0o644)
	_ = chownPath(pubPath, acc)

	// Keep a single primary identity: remove the other algorithm's files.
	alt := IdentityRSA
	if baseName == IdentityRSA {
		alt = IdentityED25519
	}
	_ = os.Remove(filepath.Join(sshDir, alt))
	_ = os.Remove(filepath.Join(sshDir, alt+".pub"))
	_ = chownPath(sshDir, acc)

	st, err := SSHKeys(acc.Username, true)
	if err != nil {
		return nil, err
	}
	// Ensure generate response always carries the fresh private material.
	st.Identity.PrivateKey = string(privBytes)
	st.Identity.HasPrivate = true
	return st, nil
}

// DeleteIdentity removes the user's identity private/public key files.
func DeleteIdentity(username string) (*SSHKeysStatus, error) {
	acc, err := requireAccount(username)
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(acc.HomeDir, ".ssh")
	for _, name := range []string{IdentityED25519, IdentityRSA} {
		_ = os.Remove(filepath.Join(sshDir, name))
		_ = os.Remove(filepath.Join(sshDir, name+".pub"))
	}
	return SSHKeys(acc.Username, false)
}

// AuthorizeIdentityPublicKey appends the identity .pub into authorized_keys.
func AuthorizeIdentityPublicKey(username string) (*SSHKeysStatus, error) {
	acc, err := requireAccount(username)
	if err != nil {
		return nil, err
	}
	id := readIdentity(filepath.Join(acc.HomeDir, ".ssh"), false)
	if !id.Exists || strings.TrimSpace(id.PublicKey) == "" {
		return nil, fmt.Errorf("no identity public key to authorize — generate a keypair first")
	}
	return AddAuthorizedKey(acc.Username, id.PublicKey, "")
}

func requireAccount(username string) (*Account, error) {
	acc, err := Lookup(username)
	if err != nil {
		return nil, err
	}
	if acc == nil || !acc.Exists {
		return nil, fmt.Errorf("linux user %q does not exist — provision Linux first", strings.TrimSpace(username))
	}
	if strings.TrimSpace(acc.HomeDir) == "" {
		return nil, fmt.Errorf("linux user %q has no home directory", acc.Username)
	}
	return acc, nil
}

func ensureSSHDir(acc *Account) (sshDir, authPath string, err error) {
	sshDir = filepath.Join(acc.HomeDir, ".ssh")
	authPath = filepath.Join(sshDir, "authorized_keys")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create .ssh: %w", err)
	}
	_ = os.Chmod(sshDir, 0o700)
	_ = chownPath(sshDir, acc)
	return sshDir, authPath, nil
}

func readAuthorizedKeys(path string) ([]AuthorizedKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []AuthorizedKey
	idx := 0
	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(trimmed))
		entry := AuthorizedKey{
			Index:   idx,
			Comment: strings.TrimSpace(comment),
			Options: options,
			Line:    trimmed,
		}
		if err == nil && pub != nil {
			entry.Type = pub.Type()
			entry.Fingerprint = ssh.FingerprintSHA256(pub)
		} else {
			// Keep unparsable lines visible so admins can still remove them.
			fields := strings.Fields(trimmed)
			if len(fields) > 0 {
				entry.Type = fields[0]
			}
			entry.Comment = "(unrecognized key line)"
		}
		out = append(out, entry)
		idx++
	}
	return out, nil
}

func normalizeAuthorizedKeyLine(key, optionalComment string) (string, error) {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "\r\n", "\n")
	key = strings.ReplaceAll(key, "\r", "\n")
	// Allow pasting multi-line; take first non-empty non-comment line.
	var candidate string
	for line := range strings.SplitSeq(key, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		candidate = line
		break
	}
	if candidate == "" {
		return "", fmt.Errorf("public key is required")
	}

	pub, comment, options, _, err := ssh.ParseAuthorizedKey([]byte(candidate))
	if err != nil || pub == nil {
		return "", fmt.Errorf("invalid OpenSSH public key: %v", err)
	}
	if c := strings.TrimSpace(optionalComment); c != "" {
		comment = c
	}
	marshaled := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	// MarshalAuthorizedKey ends with newline and has no comment; rebuild line.
	line := marshaled
	if comment != "" {
		line = marshaled + " " + comment
	}
	if len(options) > 0 {
		line = strings.Join(options, ",") + " " + line
	}
	return strings.TrimSpace(line), nil
}

func fingerprintFromLine(line string) string {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil || pub == nil {
		return ""
	}
	return ssh.FingerprintSHA256(pub)
}

func readIdentity(sshDir string, includePrivate bool) IdentityKey {
	for _, name := range []string{IdentityED25519, IdentityRSA} {
		privPath := filepath.Join(sshDir, name)
		pubPath := privPath + ".pub"
		hasPriv := fileExists(privPath)
		hasPub := fileExists(pubPath)
		if !hasPriv && !hasPub {
			continue
		}
		id := IdentityKey{
			Exists:      true,
			PrivatePath: privPath,
			PublicPath:  pubPath,
			HasPrivate:  hasPriv,
			HasPublic:   hasPub,
		}
		if hasPub {
			raw, err := os.ReadFile(pubPath)
			if err == nil {
				line := strings.TrimSpace(string(raw))
				id.PublicKey = line
				pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
				if err == nil && pub != nil {
					id.Type = pub.Type()
					id.Fingerprint = ssh.FingerprintSHA256(pub)
					id.Comment = strings.TrimSpace(comment)
				}
			}
		} else if hasPriv {
			id.Type = guessTypeFromName(name)
		}
		if includePrivate && hasPriv {
			if raw, err := os.ReadFile(privPath); err == nil {
				id.PrivateKey = string(raw)
			}
		}
		return id
	}
	return IdentityKey{Exists: false}
}

func guessTypeFromName(name string) string {
	switch name {
	case IdentityED25519:
		return "ssh-ed25519"
	case IdentityRSA:
		return "ssh-rsa"
	default:
		return ""
	}
}

func generateED25519(comment, passphrase string) (privPEM, pubLine []byte, err error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, nil, err
	}
	block, err := marshalPrivate(priv, comment, passphrase)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(block), publicKeyLine(signer.PublicKey(), comment), nil
}

func generateRSA(bits int, comment, passphrase string) (privPEM, pubLine []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, nil, err
	}
	block, err := marshalPrivate(key, comment, passphrase)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(block), publicKeyLine(signer.PublicKey(), comment), nil
}

func marshalPrivate(key any, comment, passphrase string) (*pem.Block, error) {
	if strings.TrimSpace(passphrase) != "" {
		return ssh.MarshalPrivateKeyWithPassphrase(key, comment, []byte(passphrase))
	}
	return ssh.MarshalPrivateKey(key, comment)
}

func publicKeyLine(pub ssh.PublicKey, comment string) []byte {
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if comment != "" {
		line = line + " " + comment
	}
	return []byte(line + "\n")
}

func chownPath(path string, acc *Account) error {
	if acc == nil || path == "" {
		return nil
	}
	uid, err1 := strconv.Atoi(acc.UID)
	gid, err2 := strconv.Atoi(acc.GID)
	if err1 != nil || err2 != nil {
		if u, err := user.Lookup(acc.Username); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
			gid, _ = strconv.Atoi(u.Gid)
		} else {
			return nil
		}
	}
	return os.Chown(path, uid, gid)
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
