package linuxauth

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/md5_crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	"github.com/openwall/yescrypt-go"
)

var errUnsupportedHash = errors.New("unsupported shadow hash scheme")

func verifyHash(stored, password string) (bool, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" || password == "" {
		return false, nil
	}

	switch {
	case strings.HasPrefix(stored, "$y$"):
		out, err := yescrypt.Hash([]byte(password), []byte(stored))
		if err != nil {
			return false, err
		}
		if subtle.ConstantTimeCompare(out, []byte(stored)) == 1 {
			return true, nil
		}
		return false, nil

	case strings.HasPrefix(stored, "$6$"):
		return verifyGehirn(crypt.SHA512.New(), stored, password)

	case strings.HasPrefix(stored, "$5$"):
		return verifyGehirn(crypt.SHA256.New(), stored, password)

	case strings.HasPrefix(stored, "$1$"):
		return verifyGehirn(crypt.MD5.New(), stored, password)

	default:
		return false, errUnsupportedHash
	}
}

func verifyGehirn(c crypt.Crypter, stored, password string) (bool, error) {
	err := c.Verify(stored, []byte(password))
	if err != nil {
		return false, nil
	}
	return true, nil
}
