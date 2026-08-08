package update

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const serviceUnitName = "containerws"

// scheduleRestart replaces the current process with the newly installed binary
// (or asks systemd to restart the unit) shortly after the HTTP response is flushed.
func scheduleRestart(exePath string) {
	go func() {
		// Let the Apply API response reach the client first.
		time.Sleep(900 * time.Millisecond)

		// Ensure the new binary is durable on disk before we re-exec.
		syncFilesystem(exePath)

		if trySystemdRestart() {
			// systemctl restart will stop this process; give it a moment.
			time.Sleep(20 * time.Second)
			os.Exit(0)
		}

		argv := restartArgv(exePath)
		env := os.Environ()
		if err := syscall.Exec(exePath, argv, env); err != nil {
			log.Printf("settings/update: exec %s failed: %v — exiting for supervisor restart", exePath, err)
			os.Exit(0)
		}
	}()
}

func syncFilesystem(path string) {
	if f, err := os.Open(path); err == nil {
		_ = f.Sync()
		_ = f.Close()
	}
	dir := filepath.Dir(path)
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}

// trySystemdRestart restarts the containerws unit when we are running under systemd.
// This is more reliable than Exec for Type=simple services with EnvironmentFile.
func trySystemdRestart() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// Only when this process is clearly a systemd service unit. Avoids
	// restarting a host unit while developing under air / a foreground CLI.
	if strings.TrimSpace(os.Getenv("INVOCATION_ID")) == "" &&
		strings.TrimSpace(os.Getenv("SYSTEMD_EXEC_PID")) == "" {
		return false
	}
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return false
	}
	cmd := exec.Command(systemctl, "restart", serviceUnitName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("settings/update: systemctl restart %s failed to start: %v", serviceUnitName, err)
		return false
	}
	log.Printf("settings/update: requested systemctl restart %s", serviceUnitName)
	return true
}

// restartArgv rebuilds argv so the new process always runs the server (`--start`).
func restartArgv(exePath string) []string {
	args := append([]string(nil), os.Args...)
	if len(args) == 0 {
		return []string{exePath, "--start"}
	}
	args[0] = exePath
	hasStart := false
	for _, a := range args[1:] {
		if a == "--start" {
			hasStart = true
			break
		}
	}
	if !hasStart {
		args = append(args, "--start")
	}
	return args
}
