//nolint:revive
package version

var (
	// Version is the current Container Workspace version (set via -ldflags at build).
	Version = "(untracked)"
	// CommitSHA is the short git commit sha (set via -ldflags at build).
	CommitSHA = "(unknown)"
)
