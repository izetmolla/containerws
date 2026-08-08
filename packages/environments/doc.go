// Package environments manages OS environment variables stored in the core database.
//
// Like [github.com/izetmolla/uetapp/packages/resources], it loads rows at boot,
// syncs them into the process environment, watches for changes across pods,
// and exposes CRUD helpers that notify other instances automatically.
package environments
