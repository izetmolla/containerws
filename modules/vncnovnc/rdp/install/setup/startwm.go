package setup

import (
	"os"
	"strings"
)

const startwmPath = "/etc/xrdp/startwm.sh"

const preferredStartwm = `#!/bin/sh
if [ -r /etc/default/locale ]; then
  . /etc/default/locale
  export LANG LANGUAGE
fi
unset DBUS_SESSION_BUS_ADDRESS
unset SESSION_MANAGER

# Same desktop as the user's VNC profile when no RDP session type is chosen
if [ -x "$HOME/.xsession" ]; then
  exec "$HOME/.xsession"
fi
if [ -r "$HOME/.xsession" ]; then
  . "$HOME/.xsession"
  exit
fi

if command -v startxfce4 >/dev/null 2>&1; then
  exec startxfce4
fi
if command -v xfce4-session >/dev/null 2>&1; then
  exec xfce4-session
fi
test -x /etc/X11/Xsession && exec /etc/X11/Xsession
exec /bin/sh /etc/X11/xinit/xinitrc
`

// EnsureStartwmPrefersUserSession rewrites /etc/xrdp/startwm.sh so RDP
// logins use ~/.xsession (matching the VNC desktop_session) when present.
func EnsureStartwmPrefersUserSession() error {
	if _, err := os.Stat("/etc/xrdp"); err != nil {
		return nil
	}
	existing, _ := os.ReadFile(startwmPath)
	if strings.Contains(string(existing), `exec "$HOME/.xsession"`) {
		return nil
	}
	return os.WriteFile(startwmPath, []byte(preferredStartwm), 0o755)
}
