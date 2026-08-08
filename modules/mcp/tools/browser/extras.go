package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ObserveOutput struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

func (c *Controller) maybeObserve(ctx context.Context, observe bool) (*ObserveOutput, error) {
	if !observe {
		return nil, nil
	}
	url, title, err := c.currentMeta(ctx)
	if err != nil {
		return nil, err
	}
	return &ObserveOutput{URL: url, Title: title}, nil
}

type ExtractInput struct {
	Selector       string   `json:"selector,omitempty" jsonschema:"CSS selector for nodes (default body links/text blocks)"`
	Selectors      []string `json:"selectors,omitempty" jsonschema:"optional list of CSS selectors to extract"`
	Attribute      string   `json:"attribute,omitempty" jsonschema:"attribute to read (default text); href|src|value|innerText|..."`
	Limit          int      `json:"limit,omitempty" jsonschema:"max items (default 30)"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

func (c *Controller) ExtractTool(ctx context.Context, _ *mcp.CallToolRequest, input ExtractInput) (*mcp.CallToolResult, any, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}
	attr := strings.TrimSpace(input.Attribute)
	if attr == "" {
		attr = "text"
	}
	sels := input.Selectors
	if len(sels) == 0 && strings.TrimSpace(input.Selector) != "" {
		sels = []string{input.Selector}
	}
	if len(sels) == 0 {
		sels = []string{"a[href]", "h1", "h2", "h3", "button", "label", "input", "textarea"}
	}

	payload, _ := json.Marshal(map[string]any{
		"selectors": sels,
		"attribute": attr,
		"limit":     limit,
	})
	expr := fmt.Sprintf(`(() => {
  const q = %s;
  const norm = (s) => (s || '').replace(/\s+/g, ' ').trim();
  const read = (el, attr) => {
    if (attr === 'text' || attr === 'innerText') return norm(el.innerText || el.textContent);
    if (attr === 'value') return el.value != null ? String(el.value) : '';
    if (attr === 'html') return el.innerHTML;
    return el.getAttribute(attr) || '';
  };
  const items = [];
  for (const sel of q.selectors) {
    let nodes = [];
    try { nodes = Array.from(document.querySelectorAll(sel)); } catch (e) { continue; }
    for (const el of nodes) {
      const st = window.getComputedStyle(el);
      if (st && (st.display === 'none' || st.visibility === 'hidden')) continue;
      const value = read(el, q.attribute);
      if (!value) continue;
      items.push({
        selector: sel,
        tag: el.tagName.toLowerCase(),
        value: value.slice(0, 500),
        href: el.href || undefined,
        name: el.getAttribute('name') || undefined
      });
      if (items.length >= q.limit) break;
    }
    if (items.length >= q.limit) break;
  }
  return { items, count: items.length };
})()`, string(payload))

	var result any
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return chromedp.Run(runCtx, chromedp.Evaluate(expr, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithReturnByValue(true)
		}))
	}); err != nil {
		return nil, nil, err
	}
	return nil, result, nil
}

type FormField struct {
	Selector string `json:"selector" jsonschema:"CSS, ref, role, label, placeholder, or text selector"`
	Value    string `json:"value" jsonschema:"value to fill (or option text for selects)"`
}

type FillFormInput struct {
	Fields         []FormField `json:"fields" jsonschema:"ordered list of fields to fill"`
	Submit         string      `json:"submit,omitempty" jsonschema:"optional selector to click after fill"`
	TimeoutSeconds int         `json:"timeout_seconds,omitempty"`
	Observe        bool        `json:"observe,omitempty" jsonschema:"return url/title after fill"`
}

func (c *Controller) FillFormTool(ctx context.Context, _ *mcp.CallToolRequest, input FillFormInput) (*mcp.CallToolResult, any, error) {
	if len(input.Fields) == 0 {
		return nil, nil, fmt.Errorf("fields is required")
	}
	filled := make([]string, 0, len(input.Fields))
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		for _, f := range input.Fields {
			t, err := parseTarget(f.Selector)
			if err != nil {
				return err
			}
			// Prefer select action for <select> via fill which handles both.
			if err := fillTarget(runCtx, t, f.Value, locateOpts{Nth: -1}); err != nil {
				return fmt.Errorf("field %q: %w", f.Selector, err)
			}
			filled = append(filled, f.Selector)
		}
		if s := strings.TrimSpace(input.Submit); s != "" {
			t, err := parseTarget(s)
			if err != nil {
				return err
			}
			return clickTarget(runCtx, t, locateOpts{Nth: -1})
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	out := map[string]any{"filled": filled, "count": len(filled), "submitted": strings.TrimSpace(input.Submit) != ""}
	if obs, err := c.maybeObserve(ctx, input.Observe); err != nil {
		return nil, nil, err
	} else if obs != nil {
		out["url"] = obs.URL
		out["title"] = obs.Title
	}
	return nil, out, nil
}

type WaitForInput struct {
	Selector       string `json:"selector,omitempty" jsonschema:"CSS, text, or xpath selector to wait for"`
	State          string `json:"state,omitempty" jsonschema:"visible (default), present, hidden"`
	Text           string `json:"text,omitempty" jsonschema:"wait until this text appears in the page"`
	URLContains    string `json:"url_contains,omitempty" jsonschema:"wait until location.href contains this"`
	URLMatches     string `json:"url_matches,omitempty" jsonschema:"wait until location.href matches this regex"`
	Milliseconds   int    `json:"milliseconds,omitempty" jsonschema:"fixed sleep (prefer condition waits)"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) WaitForTool(ctx context.Context, _ *mcp.CallToolRequest, input WaitForInput) (*mcp.CallToolResult, any, error) {
	if input.Milliseconds > 0 &&
		strings.TrimSpace(input.Selector) == "" &&
		strings.TrimSpace(input.Text) == "" &&
		strings.TrimSpace(input.URLContains) == "" &&
		strings.TrimSpace(input.URLMatches) == "" {
		d := min(time.Duration(input.Milliseconds)*time.Millisecond, 60*time.Second)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(d):
		}
		return nil, map[string]any{"waited_ms": input.Milliseconds}, nil
	}

	timeout := defaultTimeout(input.TimeoutSeconds)
	deadline := time.Now().Add(timeout)

	if err := c.withTab(ctx, timeout+time.Second, func(runCtx context.Context) error {
		if sel := strings.TrimSpace(input.Selector); sel != "" {
			s, by, err := parseSelector(sel)
			if err != nil {
				return err
			}
			state := strings.ToLower(strings.TrimSpace(input.State))
			if state == "" {
				state = "visible"
			}
			switch state {
			case "visible":
				return chromedp.Run(runCtx, chromedp.WaitVisible(s, by))
			case "present", "attached":
				return chromedp.Run(runCtx, chromedp.WaitReady(s, by))
			case "hidden", "detached":
				return chromedp.Run(runCtx, chromedp.WaitNotPresent(s, by))
			default:
				return fmt.Errorf("unknown state %q", state)
			}
		}

		// Poll for text / URL conditions.
		for {
			if runCtx.Err() != nil {
				return runCtx.Err()
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("wait_for timed out")
			}
			var ok bool
			cond, _ := json.Marshal(map[string]string{
				"text":         input.Text,
				"url_contains": input.URLContains,
				"url_matches":  input.URLMatches,
			})
			expr := fmt.Sprintf(`(() => {
  const c = %s;
  if (c.url_contains && !location.href.includes(c.url_contains)) return false;
  if (c.url_matches) {
    try { if (!new RegExp(c.url_matches).test(location.href)) return false; } catch (e) { throw e; }
  }
  if (c.text) {
    const body = (document.body && (document.body.innerText || document.body.textContent)) || '';
    if (!body.toLowerCase().includes(c.text.toLowerCase())) return false;
  }
  return true;
})()`, string(cond))
			if err := chromedp.Run(runCtx, chromedp.Evaluate(expr, &ok)); err != nil {
				return err
			}
			if ok {
				return nil
			}
			select {
			case <-runCtx.Done():
				return runCtx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}); err != nil {
		return nil, nil, err
	}

	out := map[string]any{"ok": true}
	if input.Selector != "" {
		out["selector"] = input.Selector
		out["state"] = coalesce(input.State, "visible")
	}
	if input.Text != "" {
		out["text"] = input.Text
	}
	if input.URLContains != "" {
		out["url_contains"] = input.URLContains
	}
	if input.URLMatches != "" {
		out["url_matches"] = input.URLMatches
	}
	return nil, out, nil
}

type AssertInput struct {
	URLContains    string `json:"url_contains,omitempty"`
	URLMatches     string `json:"url_matches,omitempty"`
	TitleContains  string `json:"title_contains,omitempty"`
	TextContains   string `json:"text_contains,omitempty" jsonschema:"page body must contain this text"`
	Selector       string `json:"selector,omitempty" jsonschema:"CSS, text, or xpath selector that must be visible"`
	NotSelector    string `json:"not_selector,omitempty" jsonschema:"must NOT be present or visible"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) AssertTool(ctx context.Context, _ *mcp.CallToolRequest, input AssertInput) (*mcp.CallToolResult, any, error) {
	var url, title, bodyText string
	var selectorOK, notSelectorOK bool
	hasSelector := strings.TrimSpace(input.Selector) != ""
	hasNot := strings.TrimSpace(input.NotSelector) != ""

	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		actions := []chromedp.Action{
			chromedp.Location(&url),
			chromedp.Title(&title),
		}
		if strings.TrimSpace(input.TextContains) != "" {
			actions = append(actions, chromedp.Evaluate(`(() => (document.body && (document.body.innerText || document.body.textContent) || '').slice(0, 50000))()`, &bodyText))
		}
		if err := chromedp.Run(runCtx, actions...); err != nil {
			return err
		}
		if hasSelector {
			t, err := parseTarget(input.Selector)
			if err != nil {
				return err
			}
			ok, err := elementVisible(runCtx, t)
			if err != nil {
				return err
			}
			selectorOK = ok
		}
		if hasNot {
			t, err := parseTarget(input.NotSelector)
			if err != nil {
				return err
			}
			ok, err := elementPresent(runCtx, t)
			if err != nil {
				return err
			}
			notSelectorOK = !ok
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	failures := []string{}
	if s := input.URLContains; s != "" && !strings.Contains(url, s) {
		failures = append(failures, fmt.Sprintf("url_contains %q not in %q", s, url))
	}
	if s := input.URLMatches; s != "" {
		okExpr := false
		_ = c.withTab(ctx, 5*time.Second, func(runCtx context.Context) error {
			return chromedp.Run(runCtx, chromedp.Evaluate(fmt.Sprintf(`new RegExp(%q).test(location.href)`, s), &okExpr))
		})
		if !okExpr {
			failures = append(failures, fmt.Sprintf("url_matches %q failed for %q", s, url))
		}
	}
	if s := input.TitleContains; s != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(s)) {
		failures = append(failures, fmt.Sprintf("title_contains %q not in %q", s, title))
	}
	if s := input.TextContains; s != "" && !strings.Contains(strings.ToLower(bodyText), strings.ToLower(s)) {
		failures = append(failures, fmt.Sprintf("text_contains %q not found on page", s))
	}
	if hasSelector && !selectorOK {
		failures = append(failures, fmt.Sprintf("selector %q not visible", input.Selector))
	}
	if hasNot && !notSelectorOK {
		failures = append(failures, fmt.Sprintf("not_selector %q is still present", input.NotSelector))
	}
	if len(failures) == 0 &&
		input.URLContains == "" && input.URLMatches == "" && input.TitleContains == "" &&
		input.TextContains == "" && !hasSelector && !hasNot {
		return nil, nil, fmt.Errorf("assert requires at least one condition")
	}

	return nil, map[string]any{
		"ok":     len(failures) == 0,
		"url":    url,
		"title":  title,
		"failed": failures,
	}, nil
}

func elementVisible(ctx context.Context, t target) (bool, error) {
	if t.needsJS() {
		locate, err := locateElementJS(t, locateOpts{Nth: -1})
		if err != nil {
			return false, err
		}
		expr := fmt.Sprintf(`(() => { try { const el = %s; return !!el; } catch (e) { return false; } })()`, locate)
		var ok bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &ok)); err != nil {
			return false, err
		}
		return ok, nil
	}
	selJSON, _ := json.Marshal(t.Selector)
	var expr string
	if t.Kind == "xpath" {
		expr = fmt.Sprintf(`(() => {
  const snap = document.evaluate(%s, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
  const el = snap.singleNodeValue;
  if (!el || !(el instanceof Element)) return false;
  const st = getComputedStyle(el);
  if (st.display === 'none' || st.visibility === 'hidden') return false;
  const r = el.getBoundingClientRect();
  return r.width > 0 && r.height > 0;
})()`, string(selJSON))
	} else {
		expr = fmt.Sprintf(`(() => {
  const el = document.querySelector(%s);
  if (!el) return false;
  const st = getComputedStyle(el);
  if (st.display === 'none' || st.visibility === 'hidden') return false;
  const r = el.getBoundingClientRect();
  return r.width > 0 && r.height > 0;
})()`, string(selJSON))
	}
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &ok)); err != nil {
		return false, err
	}
	return ok, nil
}

func elementPresent(ctx context.Context, t target) (bool, error) {
	if t.needsJS() {
		return elementVisible(ctx, t)
	}
	selJSON, _ := json.Marshal(t.Selector)
	var expr string
	if t.Kind == "xpath" {
		expr = fmt.Sprintf(`document.evaluate(%s, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue != null`, string(selJSON))
	} else {
		expr = fmt.Sprintf(`!!document.querySelector(%s)`, string(selJSON))
	}
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &ok)); err != nil {
		return false, err
	}
	return ok, nil
}

type SelectInput struct {
	Selector       string `json:"selector" jsonschema:"CSS, ref, role, or label selector for a select element"`
	Value          string `json:"value" jsonschema:"option value, label, or visible text"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Observe        bool   `json:"observe,omitempty"`
}

func (c *Controller) SelectTool(ctx context.Context, _ *mcp.CallToolRequest, input SelectInput) (*mcp.CallToolResult, any, error) {
	t, err := parseTarget(input.Selector)
	if err != nil {
		return nil, nil, err
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return selectTarget(runCtx, t, input.Value, locateOpts{Nth: -1})
	}); err != nil {
		return nil, nil, err
	}
	out := map[string]any{"selected": input.Selector, "value": input.Value}
	if obs, err := c.maybeObserve(ctx, input.Observe); err != nil {
		return nil, nil, err
	} else if obs != nil {
		out["url"] = obs.URL
		out["title"] = obs.Title
	}
	return nil, out, nil
}

type HoverInput struct {
	Selector       string `json:"selector" jsonschema:"CSS, ref, role, text, or label selector"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) HoverTool(ctx context.Context, _ *mcp.CallToolRequest, input HoverInput) (*mcp.CallToolResult, any, error) {
	t, err := parseTarget(input.Selector)
	if err != nil {
		return nil, nil, err
	}
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return hoverTarget(runCtx, t, locateOpts{Nth: -1})
	}); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"hovered": input.Selector}, nil
}

type ScrollInput struct {
	Selector       string `json:"selector,omitempty" jsonschema:"scroll element into view; omit for page scroll"`
	X              int    `json:"x,omitempty" jsonschema:"horizontal page scroll delta"`
	Y              int    `json:"y,omitempty" jsonschema:"vertical page scroll delta (e.g. 800)"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (c *Controller) ScrollTool(ctx context.Context, _ *mcp.CallToolRequest, input ScrollInput) (*mcp.CallToolResult, any, error) {
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		if s := strings.TrimSpace(input.Selector); s != "" {
			t, err := parseTarget(s)
			if err != nil {
				return err
			}
			return scrollTarget(runCtx, t, locateOpts{Nth: -1})
		}
		expr := fmt.Sprintf(`window.scrollBy(%d, %d)`, input.X, input.Y)
		return chromedp.Run(runCtx, chromedp.Evaluate(expr, nil))
	}); err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"scrolled": true,
		"selector": input.Selector,
		"x":        input.X,
		"y":        input.Y,
	}, nil
}

func coalesce(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func resolveScreenshotPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = abs
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, nil
}
