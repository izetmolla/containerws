//go:build !linux

package identity

// withCredentials is a no-op identity switch on non-Linux builds.
func withCredentials(uid, gid uint32, fn func() error) error {
	_ = uid
	_ = gid
	if fn == nil {
		return nil
	}
	return fn()
}
