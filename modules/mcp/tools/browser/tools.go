package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/izetmolla/containerws/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func LoadTools(server *mcp.Server, app *config.AppClients) {
	c := NewController(app)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_status",
		Description: "Detect whether Chromium/Chrome is installed and whether an MCP browser session is open. " +
			"Call this before browser_open when unsure.",
	}, c.StatusTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_open",
		Description: "Open Chromium/Chrome (if installed) via DevTools and optionally navigate to a URL. " +
			"Reuses an existing session. headless defaults to false when DISPLAY is set, else true. " +
			"Container flags: --no-sandbox, --disable-dev-shm-usage.",
	}, c.OpenTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_navigate",
		Description: "Navigate the open browser tab to a URL. Requires browser_open first.",
	}, c.NavigateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_a11y_snapshot",
		Description: "PREFERRED observe tool: compact accessibility tree with stable refs (e1, e2, …). " +
			"Much smaller than browser_snapshot. Use refs in click/fill: selector=ref=e3. " +
			"Call again after navigation or DOM changes.",
	}, c.A11ySnapshotTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_find",
		Description: "Find elements by role/name/text/selector and return a small list of matches with refs. " +
			"Prefer this over dumping the whole page when looking for one control.",
	}, c.FindTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_click",
		Description: "Click an element. Selectors: CSS, xpath=, text=, ref=eN (from a11y snapshot), " +
			"role=button[name=\"Sign in\"], label=, placeholder=. Optional observe=true returns url/title.",
	}, c.ClickTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_fill",
		Description: "Clear and type into an input/textarea. Same selector DSL as browser_click " +
			"(ref=, role=, label=, placeholder=, CSS, text=).",
	}, c.FillTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_fill_form",
		Description: "Fill multiple fields in one call (and optionally click submit). " +
			"Prefer this over many browser_fill round-trips.",
	}, c.FillFormTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_type",
		Description: "Type text into the focused element or a selector without clearing first. Optional press_enter.",
	}, c.TypeTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_press",
		Description: "Press a keyboard key (Enter, Tab, Escape, ArrowDown, Control+a, etc.).",
	}, c.PressTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_select",
		Description: "Choose an option in a <select> by value or visible label. Supports ref=/role=/label= selectors.",
	}, c.SelectTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_hover",
		Description: "Hover/mouse-over an element (opens menus, tooltips). Same selector DSL as click.",
	}, c.HoverTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_scroll",
		Description: "Scroll an element into view (selector) or scroll the page by x/y pixels.",
	}, c.ScrollTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_wait",
		Description: "Wait until a CSS/text=/xpath= selector is visible/present/hidden, or wait milliseconds.",
	}, c.WaitTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_wait_for",
		Description: "Wait for URL, page text, or selector condition. Prefer this over fixed sleeps. " +
			"Supports url_contains, url_matches, text, selector+state.",
	}, c.WaitForTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_assert",
		Description: "Cheap verification: check url/title/text/selector without a full snapshot. " +
			"Returns {ok, url, title, failed}. Prefer over browser_snapshot for success checks.",
	}, c.AssertTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_extract",
		Description: "Extract structured values (text/href/attrs) matching CSS selectors into a small JSON list. " +
			"Prefer over snapshot when gathering data.",
	}, c.ExtractTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_snapshot",
		Description: "Full visible-text dump of the page (token-heavy). Prefer browser_a11y_snapshot for UI automation " +
			"and browser_extract / browser_assert for data or verification.",
	}, c.SnapshotTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "browser_screenshot",
		Description: "Capture PNG. Default: save to path (or /tmp) and return metadata only (low tokens). " +
			"Set return_image=true only when the model must see pixels.",
	}, c.ScreenshotTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_evaluate",
		Description: "Run a JavaScript expression in the page and return the JSON-serialized result.",
	}, c.EvaluateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "browser_close",
		Description: "Close the MCP-managed Chromium/Chrome session.",
	}, c.CloseTool)
}

type StatusOutput struct {
	Browser DetectedBrowser `json:"browser"`
	Session map[string]any  `json:"session"`
	Display bool            `json:"has_display"`
	Hint    string          `json:"hint,omitempty"`
}

func (c *Controller) StatusTool(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	_ = ctx
	det := DetectBrowser()
	out := StatusOutput{
		Browser: det,
		Session: c.sessionInfo(),
		Display: hasDisplay(),
	}
	if !det.Found {
		out.Hint = "Install Google Chrome from Softwares catalog, or ensure chromium/google-chrome is on PATH."
	}
	return nil, out, nil
}

type OpenInput struct {
	URL      string `json:"url,omitempty" jsonschema:"optional start URL (default about:blank)"`
	Headless *bool  `json:"headless,omitempty" jsonschema:"optional; default false when DISPLAY is set, else true"`
	Restart  bool   `json:"restart,omitempty" jsonschema:"when true, close any existing session and start fresh"`
}

type OpenOutput struct {
	Opened  bool            `json:"opened"`
	URL     string          `json:"url"`
	Title   string          `json:"title"`
	Browser DetectedBrowser `json:"browser"`
	Session map[string]any  `json:"session"`
}

func (c *Controller) OpenTool(ctx context.Context, _ *mcp.CallToolRequest, input OpenInput) (*mcp.CallToolResult, any, error) {
	if input.Restart {
		c.closeSession()
	}

	wasOpen := c.sessionInfo()["open"] == true
	if err := c.ensureSession(ctx, input.Headless, input.URL); err != nil {
		return nil, nil, err
	}
	if wasOpen {
		if u := strings.TrimSpace(input.URL); u != "" {
			if err := c.withTab(ctx, 60*time.Second, func(runCtx context.Context) error {
				return chromedp.Run(runCtx, chromedp.Navigate(u), chromedp.WaitReady("body"))
			}); err != nil {
				return nil, nil, err
			}
		}
	}

	url, title, err := c.currentMeta(ctx)
	if err != nil {
		return nil, nil, err
	}
	return nil, OpenOutput{
		Opened:  true,
		URL:     url,
		Title:   title,
		Browser: DetectBrowser(),
		Session: c.sessionInfo(),
	}, nil
}

type NavigateInput struct {
	URL            string `json:"url" jsonschema:"required URL to open"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Observe        bool   `json:"observe,omitempty" jsonschema:"included for consistency; navigate always returns url/title"`
}

func (c *Controller) NavigateTool(ctx context.Context, _ *mcp.CallToolRequest, input NavigateInput) (*mcp.CallToolResult, any, error) {
	u := strings.TrimSpace(input.URL)
	if u == "" {
		return nil, nil, fmt.Errorf("url is required")
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return chromedp.Run(runCtx, chromedp.Navigate(u), chromedp.WaitReady("body"))
	}); err != nil {
		return nil, nil, err
	}
	url, title, err := c.currentMeta(ctx)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"url": url, "title": title}, nil
}

type SelectorInput struct {
	Selector       string `json:"selector" jsonschema:"CSS, xpath, text, ref (e.g. e3), role with name, label, or placeholder"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Exact          bool   `json:"exact,omitempty" jsonschema:"exact name match for role/label"`
	Nth            int    `json:"nth,omitempty" jsonschema:"0-based match index when multiple (default 0)"`
	Observe        bool   `json:"observe,omitempty" jsonschema:"return url/title after action (cheap verify)"`
}

func (c *Controller) ClickTool(ctx context.Context, _ *mcp.CallToolRequest, input SelectorInput) (*mcp.CallToolResult, any, error) {
	t, err := parseTarget(input.Selector)
	if err != nil {
		return nil, nil, err
	}
	nth := input.Nth
	if nth < 0 {
		nth = -1
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return clickTarget(runCtx, t, locateOpts{Exact: input.Exact, Nth: nth})
	}); err != nil {
		return nil, nil, err
	}
	out := map[string]any{"clicked": input.Selector}
	if obs, err := c.maybeObserve(ctx, input.Observe); err != nil {
		return nil, nil, err
	} else if obs != nil {
		out["url"] = obs.URL
		out["title"] = obs.Title
	}
	return nil, out, nil
}

type FillInput struct {
	Selector       string `json:"selector" jsonschema:"CSS, ref, role, label, placeholder, or text selector"`
	Value          string `json:"value" jsonschema:"text to fill"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Exact          bool   `json:"exact,omitempty"`
	Nth            int    `json:"nth,omitempty"`
	Observe        bool   `json:"observe,omitempty"`
}

func (c *Controller) FillTool(ctx context.Context, _ *mcp.CallToolRequest, input FillInput) (*mcp.CallToolResult, any, error) {
	t, err := parseTarget(input.Selector)
	if err != nil {
		return nil, nil, err
	}
	nth := input.Nth
	if nth < 0 {
		nth = -1
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return fillTarget(runCtx, t, input.Value, locateOpts{Exact: input.Exact, Nth: nth})
	}); err != nil {
		return nil, nil, err
	}
	out := map[string]any{"filled": input.Selector, "length": len(input.Value)}
	if obs, err := c.maybeObserve(ctx, input.Observe); err != nil {
		return nil, nil, err
	} else if obs != nil {
		out["url"] = obs.URL
		out["title"] = obs.Title
	}
	return nil, out, nil
}

type TypeInput struct {
	Text           string `json:"text" jsonschema:"text to type"`
	Selector       string `json:"selector,omitempty" jsonschema:"optional selector to focus first"`
	PressEnter     bool   `json:"press_enter,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) TypeTool(ctx context.Context, _ *mcp.CallToolRequest, in TypeInput) (*mcp.CallToolResult, any, error) {
	if err := c.withTab(ctx, defaultTimeout(in.TimeoutSeconds), func(runCtx context.Context) error {
		if s := strings.TrimSpace(in.Selector); s != "" {
			t, err := parseTarget(s)
			if err != nil {
				return err
			}
			if t.needsJS() {
				locate, err := locateElementJS(t, locateOpts{Nth: -1})
				if err != nil {
					return err
				}
				focusExpr := fmt.Sprintf(`(() => { const el = %s; el.focus(); return true; })()`, locate)
				var ok bool
				if err := chromedp.Run(runCtx, chromedp.Evaluate(focusExpr, &ok)); err != nil {
					return err
				}
			} else {
				if err := chromedp.Run(runCtx,
					chromedp.WaitVisible(t.Selector, t.By),
					chromedp.Focus(t.Selector, t.By),
				); err != nil {
					return err
				}
			}
		}
		text := in.Text
		actions := []chromedp.Action{
			chromedp.ActionFunc(func(ctx context.Context) error {
				return input.InsertText(text).Do(ctx)
			}),
		}
		if in.PressEnter {
			actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
				return keyPress(ctx, "Enter")
			}))
		}
		return chromedp.Run(runCtx, actions...)
	}); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"typed": len(in.Text), "press_enter": in.PressEnter}, nil
}

type PressInput struct {
	Key            string `json:"key" jsonschema:"key name e.g. Enter, Tab, Escape, ArrowDown, Backspace"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) PressTool(ctx context.Context, _ *mcp.CallToolRequest, input PressInput) (*mcp.CallToolResult, any, error) {
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return nil, nil, fmt.Errorf("key is required")
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return chromedp.Run(runCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return keyPress(ctx, key)
		}))
	}); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"pressed": key}, nil
}

type WaitInput struct {
	Selector       string `json:"selector,omitempty" jsonschema:"element to wait for (CSS, xpath, or text)"`
	State          string `json:"state,omitempty" jsonschema:"visible (default), present, hidden"`
	Milliseconds   int    `json:"milliseconds,omitempty" jsonschema:"optional fixed sleep instead of selector"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) WaitTool(ctx context.Context, _ *mcp.CallToolRequest, input WaitInput) (*mcp.CallToolResult, any, error) {
	if input.Milliseconds > 0 {
		d := min(time.Duration(input.Milliseconds)*time.Millisecond, 60*time.Second)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(d):
		}
		return nil, map[string]any{"waited_ms": input.Milliseconds}, nil
	}
	sel, by, err := parseSelector(input.Selector)
	if err != nil {
		return nil, nil, err
	}
	state := strings.ToLower(strings.TrimSpace(input.State))
	if state == "" {
		state = "visible"
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		switch state {
		case "visible":
			return chromedp.Run(runCtx, chromedp.WaitVisible(sel, by))
		case "present", "attached":
			return chromedp.Run(runCtx, chromedp.WaitReady(sel, by))
		case "hidden", "detached":
			return chromedp.Run(runCtx, chromedp.WaitNotPresent(sel, by))
		default:
			return fmt.Errorf("unknown state %q (use visible|present|hidden)", state)
		}
	}); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"selector": input.Selector, "state": state}, nil
}

type SnapshotInput struct {
	IncludeHTML bool `json:"include_html,omitempty" jsonschema:"include truncated HTML body"`
	MaxChars    int  `json:"max_chars,omitempty" jsonschema:"max text chars (default 20000)"`
}

type SnapshotOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
	HTML  string `json:"html,omitempty"`
}

func (c *Controller) SnapshotTool(ctx context.Context, _ *mcp.CallToolRequest, input SnapshotInput) (*mcp.CallToolResult, any, error) {
	maxChars := input.MaxChars
	if maxChars <= 0 {
		maxChars = 20000
	}
	var url, title, text, html string
	if err := c.withTab(ctx, 30*time.Second, func(runCtx context.Context) error {
		actions := []chromedp.Action{
			chromedp.Location(&url),
			chromedp.Title(&title),
			chromedp.Evaluate(`(() => {
  const walk = (node, acc) => {
    if (!node) return acc;
    if (node.nodeType === Node.TEXT_NODE) {
      const t = (node.textContent || '').replace(/\s+/g, ' ').trim();
      if (t) acc.push(t);
      return acc;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return acc;
    const tag = (node.tagName || '').toLowerCase();
    if (['script','style','noscript','svg'].includes(tag)) return acc;
    const style = window.getComputedStyle(node);
    if (style && (style.display === 'none' || style.visibility === 'hidden')) return acc;
    for (const child of node.childNodes) walk(child, acc);
    return acc;
  };
  return walk(document.body, []).join('\n');
})()`, &text),
		}
		if input.IncludeHTML {
			actions = append(actions, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
		}
		return chromedp.Run(runCtx, actions...)
	}); err != nil {
		return nil, nil, err
	}
	text = truncateStr(text, maxChars)
	html = truncateStr(html, maxChars)
	return nil, SnapshotOutput{URL: url, Title: title, Text: text, HTML: html}, nil
}

type ScreenshotInput struct {
	Selector       string `json:"selector,omitempty" jsonschema:"optional element selector; full page viewport if empty"`
	Path           string `json:"path,omitempty" jsonschema:"save PNG to this path (default /tmp/cws-mcp-screenshot.png)"`
	ReturnImage    bool   `json:"return_image,omitempty" jsonschema:"if true, also return ImageContent (token-heavy); default false when path set, true when path empty for back-compat"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) ScreenshotTool(ctx context.Context, _ *mcp.CallToolRequest, input ScreenshotInput) (*mcp.CallToolResult, any, error) {
	var buf []byte
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		if s := strings.TrimSpace(input.Selector); s != "" {
			t, err := parseTarget(s)
			if err != nil {
				return err
			}
			if t.needsJS() {
				// Scroll target into view, then capture viewport (element screenshot needs CSS).
				if err := scrollTarget(runCtx, t, locateOpts{Nth: -1}); err != nil {
					return err
				}
				return chromedp.Run(runCtx, chromedp.FullScreenshot(&buf, 90))
			}
			return chromedp.Run(runCtx,
				chromedp.WaitVisible(t.Selector, t.By),
				chromedp.Screenshot(t.Selector, &buf, t.By),
			)
		}
		return chromedp.Run(runCtx, chromedp.FullScreenshot(&buf, 90))
	}); err != nil {
		return nil, nil, err
	}

	path := strings.TrimSpace(input.Path)
	returnImage := input.ReturnImage
	if path == "" && !input.ReturnImage {
		// Back-compat: no path → return image like before, but also write a default file.
		path = "/tmp/cws-mcp-screenshot.png"
		returnImage = true
	}
	if path == "" {
		path = "/tmp/cws-mcp-screenshot.png"
	}
	abs, err := resolveScreenshotPath(path)
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(abs, buf, 0o644); err != nil {
		return nil, nil, fmt.Errorf("write screenshot: %w", err)
	}

	meta := map[string]any{
		"bytes":     len(buf),
		"path":      abs,
		"selector":  input.Selector,
		"mime_type": "image/png",
	}
	metaJSON, _ := json.Marshal(meta)

	if !returnImage {
		return nil, meta, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: buf, MIMEType: "image/png"},
			&mcp.TextContent{Text: string(metaJSON)},
		},
	}, meta, nil
}

type EvaluateInput struct {
	Expression     string `json:"expression" jsonschema:"JavaScript expression to evaluate in the page"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) EvaluateTool(ctx context.Context, _ *mcp.CallToolRequest, in EvaluateInput) (*mcp.CallToolResult, any, error) {
	expr := strings.TrimSpace(in.Expression)
	if expr == "" {
		return nil, nil, fmt.Errorf("expression is required")
	}
	var result any
	if err := c.withTab(ctx, defaultTimeout(in.TimeoutSeconds), func(runCtx context.Context) error {
		return chromedp.Run(runCtx, chromedp.Evaluate(expr, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithReturnByValue(true).WithAwaitPromise(true)
		}))
	}); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"result": result}, nil
}

func (c *Controller) CloseTool(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	_ = ctx
	c.closeSession()
	return nil, map[string]any{"closed": true}, nil
}

func (c *Controller) currentMeta(ctx context.Context) (url, title string, err error) {
	err = c.withTab(ctx, 15*time.Second, func(runCtx context.Context) error {
		return chromedp.Run(runCtx, chromedp.Location(&url), chromedp.Title(&title))
	})
	return
}

func keyPress(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if strings.Contains(key, "+") {
		parts := strings.Split(key, "+")
		var mods input.Modifier
		var main string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			switch strings.ToLower(p) {
			case "ctrl", "control":
				mods |= input.ModifierCtrl
			case "alt":
				mods |= input.ModifierAlt
			case "shift":
				mods |= input.ModifierShift
			case "meta", "cmd", "command":
				mods |= input.ModifierMeta
			default:
				main = p
			}
		}
		if main == "" {
			return fmt.Errorf("invalid key chord %q", key)
		}
		k := keyDef(main)
		return chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				if err := input.DispatchKeyEvent(input.KeyDown).
					WithModifiers(mods).WithKey(k.Key).WithCode(k.Code).
					WithWindowsVirtualKeyCode(k.VK).Do(ctx); err != nil {
					return err
				}
				return input.DispatchKeyEvent(input.KeyUp).
					WithModifiers(mods).WithKey(k.Key).WithCode(k.Code).
					WithWindowsVirtualKeyCode(k.VK).Do(ctx)
			}),
		)
	}
	k := keyDef(key)
	return chromedp.Run(ctx, chromedp.KeyEvent(k.Key))
}

type keyInfo struct {
	Key  string
	Code string
	VK   int64
}

func keyDef(name string) keyInfo {
	switch strings.ToLower(name) {
	case "enter", "return":
		return keyInfo{Key: "Enter", Code: "Enter", VK: 13}
	case "tab":
		return keyInfo{Key: "Tab", Code: "Tab", VK: 9}
	case "escape", "esc":
		return keyInfo{Key: "Escape", Code: "Escape", VK: 27}
	case "backspace":
		return keyInfo{Key: "Backspace", Code: "Backspace", VK: 8}
	case "delete", "del":
		return keyInfo{Key: "Delete", Code: "Delete", VK: 46}
	case "arrowdown", "down":
		return keyInfo{Key: "ArrowDown", Code: "ArrowDown", VK: 40}
	case "arrowup", "up":
		return keyInfo{Key: "ArrowUp", Code: "ArrowUp", VK: 38}
	case "arrowleft", "left":
		return keyInfo{Key: "ArrowLeft", Code: "ArrowLeft", VK: 37}
	case "arrowright", "right":
		return keyInfo{Key: "ArrowRight", Code: "ArrowRight", VK: 39}
	case "space", " ":
		return keyInfo{Key: " ", Code: "Space", VK: 32}
	default:
		if len(name) == 1 {
			return keyInfo{Key: name, Code: "Key" + strings.ToUpper(name), VK: int64(strings.ToUpper(name)[0])}
		}
		return keyInfo{Key: name, Code: name}
	}
}
