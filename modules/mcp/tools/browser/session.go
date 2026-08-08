package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/izetmolla/containerws/config"
)

var candidateBins = []string{
	"chrome-desktop",
	"chromium-desktop",
	"google-chrome-desktop",
	"google-chrome-stable",
	"google-chrome",
	"chromium",
	"chromium-browser",
}

// Controller owns a process-wide Chromium/Chrome DevTools session.
type Controller struct {
	app *config.AppClients

	mu       sync.Mutex
	allocCtx context.Context
	allocCancel context.CancelFunc
	tabCtx   context.Context
	tabCancel context.CancelFunc
	binary   string
	headless bool
	userData string
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

type DetectedBrowser struct {
	Found  bool   `json:"found"`
	Binary string `json:"binary,omitempty"`
	Source string `json:"source,omitempty"`
}

func DetectBrowser() DetectedBrowser {
	for _, name := range candidateBins {
		if p, err := exec.LookPath(name); err == nil {
			return DetectedBrowser{Found: true, Binary: p, Source: "PATH:" + name}
		}
	}
	for _, p := range []string{
		"/usr/local/bin/chrome-desktop",
		"/usr/local/bin/chromium-desktop",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return DetectedBrowser{Found: true, Binary: p, Source: "path"}
		}
	}
	return DetectedBrowser{Found: false}
}

func hasDisplay() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func (c *Controller) ensureSession(ctx context.Context, headless *bool, startURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tabCtx != nil {
		return nil
	}

	det := DetectBrowser()
	if !det.Found {
		return fmt.Errorf("chromium/chrome not found — install via Softwares catalog (Google Chrome) or apt, then retry")
	}

	useHeadless := !hasDisplay()
	if headless != nil {
		useHeadless = *headless
	}

	userData := filepath.Join(os.TempDir(), "cws-mcp-chrome-profile")
	_ = os.MkdirAll(userData, 0o755)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(det.Binary),
		chromedp.UserDataDir(userData),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("window-size", "1280,900"),
		chromedp.WSURLReadTimeout(60*time.Second),
	)
	if useHeadless {
		opts = append(opts, chromedp.Headless)
		opts = append(opts, chromedp.Flag("headless", "new"))
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)

	runCtx, cancel := context.WithTimeout(tabCtx, 60*time.Second)
	defer cancel()
	_ = ctx

	if err := chromedp.Run(runCtx, chromedp.Navigate(coalesceURL(startURL))); err != nil {
		tabCancel()
		allocCancel()
		return fmt.Errorf("failed to start browser (%s): %w", det.Binary, err)
	}

	c.allocCtx = allocCtx
	c.allocCancel = allocCancel
	c.tabCtx = tabCtx
	c.tabCancel = tabCancel
	c.binary = det.Binary
	c.headless = useHeadless
	c.userData = userData
	return nil
}

func coalesceURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "about:blank"
	}
	return u
}

func (c *Controller) withTab(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	c.mu.Lock()
	tab := c.tabCtx
	c.mu.Unlock()
	if tab == nil {
		return fmt.Errorf("browser is not open — call browser_open first")
	}
	runCtx, cancel := context.WithTimeout(tab, timeout)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	return fn(runCtx)
}

func (c *Controller) closeSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tabCancel != nil {
		c.tabCancel()
		c.tabCancel = nil
	}
	if c.allocCancel != nil {
		c.allocCancel()
		c.allocCancel = nil
	}
	c.tabCtx = nil
	c.allocCtx = nil
	c.binary = ""
}

func (c *Controller) sessionInfo() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	open := c.tabCtx != nil
	info := map[string]any{
		"open":     open,
		"headless": c.headless,
		"binary":   c.binary,
		"user_data": c.userData,
		"display":  os.Getenv("DISPLAY"),
	}
	return info
}

func defaultTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 30 * time.Second
	}
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}
