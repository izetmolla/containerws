package hostsetup

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DetectInit picks systemd / launchd / openrc / freebsd-rc / sysv / direct.
func DetectInit() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "freebsd":
		return "freebsd-rc"
	case "windows":
		return "direct"
	}

	if systemdUsable() {
		return "systemd"
	}
	if commandExists("rc-status") {
		if err := exec.Command("rc-status").Run(); err == nil {
			return "openrc"
		}
	}
	if commandExists("rc-update") {
		if dirExists("/run/openrc") || dirExists("/etc/runlevels") {
			return "openrc"
		}
	}
	if dirExists("/etc/init.d") && commandExists("update-rc.d") {
		pid1 := strings.TrimSpace(runOutput("ps", "-p", "1", "-o", "comm="))
		switch pid1 {
		case "init", "sysvinit", "busybox":
			return "sysv"
		}
	}
	return "direct"
}

func systemdUsable() bool {
	if !commandExists("systemctl") || !dirExists("/run/systemd/system") {
		return false
	}
	state := strings.TrimSpace(runOutput("systemctl", "is-system-running"))
	switch state {
	case "running", "degraded", "maintenance", "initializing", "starting":
		return true
	}
	pid1 := strings.TrimSpace(runOutput("ps", "-p", "1", "-o", "comm="))
	return pid1 == "systemd"
}

func installDaemon(initSys, bin string, opts Options) error {
	switch initSys {
	case "systemd":
		if err := writeFile(UnitPath, systemdUnit(bin, opts.Repo), 0o644); err != nil {
			return err
		}
		_ = exec.Command("systemctl", "daemon-reload").Run()
		if err := exec.Command("systemctl", "enable", ServiceName).Run(); err != nil {
			return fmt.Errorf("systemctl enable: %w", err)
		}
		if !opts.NoStart {
			if err := exec.Command("systemctl", "restart", ServiceName).Run(); err != nil {
				return fmt.Errorf("systemctl restart: %w", err)
			}
		}
	case "launchd":
		if err := writeFile(LaunchdPath, launchdPlist(bin), 0o644); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "bootout", "system/"+LaunchdLabel).Run()
		if err := exec.Command("launchctl", "bootstrap", "system", LaunchdPath).Run(); err != nil {
			return fmt.Errorf("launchctl bootstrap: %w", err)
		}
		if !opts.NoStart {
			_ = exec.Command("launchctl", "kickstart", "-k", "system/"+LaunchdLabel).Run()
			_ = exec.Command("launchctl", "enable", "system/"+LaunchdLabel).Run()
		}
	case "openrc", "sysv":
		if err := writeFile(OpenRCPath, openRCService(bin), 0o755); err != nil {
			return err
		}
		if initSys == "openrc" {
			_ = exec.Command("rc-update", "add", ServiceName, "default").Run()
			if !opts.NoStart {
				if err := exec.Command("rc-service", ServiceName, "restart").Run(); err != nil {
					_ = exec.Command("rc-service", ServiceName, "start").Run()
				}
			}
		} else {
			_ = exec.Command("update-rc.d", ServiceName, "defaults").Run()
			if !opts.NoStart {
				_ = exec.Command("service", ServiceName, "restart").Run()
				_ = exec.Command("service", ServiceName, "start").Run()
			}
		}
	case "freebsd-rc":
		if err := writeFile(FreeBSDRC, freebsdRC(bin), 0o755); err != nil {
			return err
		}
		ensureFreeBSDEnable()
		if !opts.NoStart {
			if err := exec.Command("service", ServiceName, "restart").Run(); err != nil {
				_ = exec.Command("service", ServiceName, "start").Run()
			}
		}
	default: // direct
		if err := writeDirectDaemonWrapper(bin); err != nil {
			return err
		}
		_ = installDirectCron()
		if !opts.NoStart {
			if err := exec.Command(DaemonWrap, "restart").Run(); err != nil {
				return fmt.Errorf("direct daemon start: %w", err)
			}
		}
	}
	return nil
}

func uninstall(initSys, bin string) error {
	switch initSys {
	case "systemd":
		_ = exec.Command("systemctl", "disable", "--now", ServiceName).Run()
		_ = os.Remove(UnitPath)
		_ = exec.Command("systemctl", "daemon-reload").Run()
	case "launchd":
		_ = exec.Command("launchctl", "bootout", "system/"+LaunchdLabel).Run()
		_ = os.Remove(LaunchdPath)
	case "openrc":
		_ = exec.Command("rc-service", ServiceName, "stop").Run()
		_ = exec.Command("rc-update", "del", ServiceName, "default").Run()
		_ = os.Remove(OpenRCPath)
	case "freebsd-rc":
		_ = exec.Command("service", ServiceName, "stop").Run()
		_ = os.Remove(FreeBSDRC)
	case "sysv":
		_ = exec.Command("service", ServiceName, "stop").Run()
		_ = exec.Command("update-rc.d", "-f", ServiceName, "remove").Run()
		_ = os.Remove(OpenRCPath)
	case "direct":
		if isExecutable(DaemonWrap) {
			_ = exec.Command(DaemonWrap, "stop").Run()
		}
		_ = os.Remove(CronFile)
		_ = os.Remove(PIDFile)
	}
	if isExecutable(DaemonWrap) {
		_ = exec.Command(DaemonWrap, "stop").Run()
	}
	_ = exec.Command("pkill", "-f", bin+" --start").Run()

	_ = os.Remove(CLIPath)
	_ = os.Remove(AliasPath)
	if brewBin := brewPrefixBin(); brewBin != "" {
		_ = os.Remove(filepath.Join(brewBin, CLIName))
	}
	_ = os.Remove(CronFile)
	_ = os.Remove(PIDFile)
	_ = os.Remove(DaemonWrap)
	// Keep InstallDir if brew owns the binary elsewhere; only remove wrapper leftovers.
	return nil
}

func verifyDaemonStarted(initSys string) bool {
	for i := 0; i < 10; i++ {
		if daemonIsRunning(initSys) {
			return true
		}
		client := &http.Client{Timeout: time.Second}
		if resp, err := client.Get("http://127.0.0.1:9000/"); err == nil {
			_ = resp.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func daemonIsRunning(initSys string) bool {
	switch initSys {
	case "systemd":
		return exec.Command("systemctl", "is-active", "--quiet", ServiceName).Run() == nil
	case "launchd":
		out := runOutput("launchctl", "print", "system/"+LaunchdLabel)
		return strings.Contains(out, "state = running")
	case "openrc":
		return exec.Command("rc-service", ServiceName, "status").Run() == nil
	case "freebsd-rc", "sysv":
		return exec.Command("service", ServiceName, "status").Run() == nil
	case "direct":
		return isExecutable(DaemonWrap) && exec.Command(DaemonWrap, "status").Run() == nil
	default:
		return false
	}
}

func systemdUnit(bin, repo string) string {
	return fmt.Sprintf(`[Unit]
Description=Container Workspace (cws)
Documentation=https://github.com/%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-%s
Environment=ENV=production
WorkingDirectory=/
ExecStart=%s --start
Restart=on-failure
RestartSec=3
KillMode=mixed
TimeoutStopSec=30
LimitNOFILE=1048576
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, repo, EnvFile, bin)
}

func launchdPlist(bin string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>--start</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>ENV</key>
    <string>production</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>/</string>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, LaunchdLabel, bin, LogOut, LogErr)
}

func openRCService(bin string) string {
	return fmt.Sprintf(`#!/sbin/openrc-run
description="Container Workspace (cws)"
command="%s"
command_args="--start"
command_background=yes
pidfile="/run/${RC_SVCNAME}.pid"
output_log="%s"
error_log="%s"

depend() {
  need net
  after firewall
}

start_pre() {
  checkpath --directory /var/log/containerws
  if [ -f "%s" ]; then
    set -a
    . "%s"
    set +a
  fi
  export ENV="${ENV:-production}"
}
`, bin, LogOut, LogErr, EnvFile, EnvFile)
}

func freebsdRC(bin string) string {
	return fmt.Sprintf(`#!/bin/sh
# PROVIDE: containerws
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="%s"
rcvar="${name}_enable"
command="%s"
command_args="--start"
pidfile="%s"
start_cmd="containerws_start"
stop_cmd="containerws_stop"
status_cmd="containerws_status"

load_rc_config $name
: ${containerws_enable:="NO"}

containerws_start() {
  export ENV=production
  if [ -f "%s" ]; then
    set -a
    . "%s"
    set +a
  fi
  /usr/sbin/daemon -p "${pidfile}" -f ${command} ${command_args}
}

containerws_stop() {
  if [ -f "${pidfile}" ]; then
    kill "$(cat "${pidfile}")" 2>/dev/null || true
    rm -f "${pidfile}"
  fi
}

containerws_status() {
  if [ -f "${pidfile}" ] && kill -0 "$(cat "${pidfile}")" 2>/dev/null; then
    echo "${name} is running as pid $(cat "${pidfile}")."
  else
    echo "${name} is not running."
    return 1
  fi
}

run_rc_command "$1"
`, ServiceName, bin, PIDFile, EnvFile, EnvFile)
}

func writeDirectDaemonWrapper(bin string) error {
	if err := os.MkdirAll(BinDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll("/var/log/containerws", 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`#!/usr/bin/env bash
# Container Workspace direct daemon helper (cws --start)
set -euo pipefail
BIN="%s"
PIDFILE="%s"
ENV_FILE="%s"
LOG_OUT="%s"
LOG_ERR="%s"

load_env() {
  export ENV="${ENV:-production}"
  if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
  fi
}

is_running() {
  [[ -f "$PIDFILE" ]] || return 1
  local pid
  pid="$(tr -d '[:space:]' <"$PIDFILE" 2>/dev/null || true)"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  return 0
}

cmd_start() {
  if is_running; then
    echo "containerws already running (pid $(cat "$PIDFILE"))"
    return 0
  fi
  load_env
  mkdir -p "$(dirname "$PIDFILE")" "$(dirname "$LOG_OUT")"
  rm -f "$PIDFILE"
  nohup "$BIN" --start >>"$LOG_OUT" 2>>"$LOG_ERR" &
  local pid=$!
  echo "$pid" >"$PIDFILE"
  sleep 1
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "error: cws --start exited immediately; see $LOG_ERR" >&2
    rm -f "$PIDFILE"
    return 1
  fi
  echo "started containerws (pid $pid)"
}

cmd_stop() {
  if ! [[ -f "$PIDFILE" ]]; then
    pkill -f "$BIN --start" 2>/dev/null || true
    echo "containerws not running"
    return 0
  fi
  local pid
  pid="$(tr -d '[:space:]' <"$PIDFILE")"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$PIDFILE"
  echo "stopped containerws"
}

cmd_status() {
  if is_running; then
    echo "containerws is running (pid $(cat "$PIDFILE"))"
    return 0
  fi
  echo "containerws is not running"
  return 1
}

cmd_restart() {
  cmd_stop || true
  cmd_start
}

case "${1:-}" in
  start) cmd_start ;;
  stop) cmd_stop ;;
  restart) cmd_restart ;;
  status) cmd_status ;;
  *)
    echo "usage: $0 {start|stop|restart|status}" >&2
    exit 1
    ;;
esac
`, bin, PIDFile, EnvFile, LogOut, LogErr)
	return writeFile(DaemonWrap, body, 0o755)
}

func installDirectCron() error {
	if !dirExists("/etc/cron.d") {
		return nil
	}
	body := fmt.Sprintf(`# Restart Container Workspace after reboot (direct daemon mode)
@reboot root %s start >/dev/null 2>&1
`, DaemonWrap)
	return writeFile(CronFile, body, 0o644)
}

func ensureFreeBSDEnable() {
	const line = `containerws_enable="YES"`
	b, err := os.ReadFile("/etc/rc.conf")
	if err != nil {
		_ = os.WriteFile("/etc/rc.conf", []byte(line+"\n"), 0o644)
		return
	}
	if strings.Contains(string(b), "containerws_enable=") {
		return
	}
	f, err := os.OpenFile("/etc/rc.conf", os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString("\n" + line + "\n")
}

func writeFile(path, body string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

func runOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out))
	}
	return strings.TrimSpace(string(out))
}
