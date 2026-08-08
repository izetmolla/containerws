// Package linuxauth validates credentials against local Linux accounts.
//
// Authentication reads password hashes from /etc/shadow and verifies them with
// pure-Go crypt implementations (no CGO). The process must be able to read
// /etc/shadow (typically root). When shadow is unreadable (e.g. local non-root
// development), Authenticate returns (false, nil) so callers can fall back to
// database password checks.
package linuxauth

import (
	"bufio"
	"errors"
	"os"
	"os/user"
	"strings"
)

const shadowPath = "/etc/shadow"

// UserExists reports whether username is a local account (via os/user.Lookup).
func UserExists(username string) bool {
	username = strings.TrimSpace(username)
	if username == "" || strings.Contains(username, ":") {
		return false
	}
	_, err := user.Lookup(username)
	return err == nil
}

// Authenticate checks username/password against /etc/shadow.
// Returns (false, nil) when the user is missing, the password is wrong,
// the account is locked, the hash scheme is unsupported, or shadow is
// unreadable. Returns a non-nil error only for unexpected I/O failures
// while shadow is otherwise accessible.
func Authenticate(username, password string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return false, nil
	}
	if !UserExists(username) {
		return false, nil
	}

	hash, err := readShadowHash(username)
	if err != nil {
		if errors.Is(err, errShadowUnavailable) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return false, nil
		}
		if errors.Is(err, errUserNotInShadow) || errors.Is(err, errAccountLocked) {
			return false, nil
		}
		return false, err
	}

	ok, err := verifyHash(hash, password)
	if err != nil {
		// Unsupported scheme or verify failure → treat as non-match.
		return false, nil
	}
	return ok, nil
}

// LinuxLoginName returns a candidate local username for a sign-in identifier.
// If id has no "@", it is returned as-is. If it looks like email, the local
// part is returned (e.g. root@localhost → root).
func LinuxLoginName(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if i := strings.IndexByte(id, '@'); i > 0 {
		return id[:i]
	}
	return id
}

var (
	errShadowUnavailable = errors.New("shadow unavailable")
	errUserNotInShadow   = errors.New("user not in shadow")
	errAccountLocked     = errors.New("account locked")
)

func readShadowHash(username string) (string, error) {
	f, err := os.Open(shadowPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return "", errShadowUnavailable
		}
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 2 || parts[0] != username {
			continue
		}
		hash := parts[1]
		if hash == "" || hash == "*" || hash == "!" || strings.HasPrefix(hash, "!") || strings.HasPrefix(hash, "*") {
			return "", errAccountLocked
		}
		return hash, nil
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", errUserNotInShadow
}
