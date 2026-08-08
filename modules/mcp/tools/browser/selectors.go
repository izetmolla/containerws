package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// target describes how to locate an element. CSS/XPath use chromedp queries;
// ref/role/label/placeholder use injected page JS (refs from browser_a11y_snapshot).
type target struct {
	Kind     string // css | xpath | ref | role | label | placeholder
	Value    string
	Name     string // optional accessible name for role=
	Exact    bool
	Nth      int // 0-based; -1 means first match (default)
	By       chromedp.QueryOption
	Selector string // chromedp selector when Kind is css|xpath
}

var roleNameRe = regexp.MustCompile(`(?i)^role=([a-z0-9_-]+)(?:\[name=(?:"([^"]*)"|'([^']*)'|([^\]]+))\])?$`)

func parseTarget(raw string) (target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return target{}, fmt.Errorf("selector is required")
	}
	lower := strings.ToLower(raw)

	switch {
	case strings.HasPrefix(lower, "ref="):
		ref := strings.TrimSpace(raw[len("ref="):])
		if ref == "" {
			return target{}, fmt.Errorf("ref= requires an id from browser_a11y_snapshot (e.g. ref=e3)")
		}
		return target{Kind: "ref", Value: ref, Nth: -1}, nil

	case strings.HasPrefix(lower, "role="):
		m := roleNameRe.FindStringSubmatch(raw)
		if m == nil {
			return target{}, fmt.Errorf("invalid role selector %q (use role=button or role=button[name=\"Sign in\"])", raw)
		}
		name := m[2]
		if name == "" {
			name = m[3]
		}
		if name == "" {
			name = strings.TrimSpace(m[4])
		}
		return target{Kind: "role", Value: strings.ToLower(m[1]), Name: name, Nth: -1}, nil

	case strings.HasPrefix(lower, "label="):
		return target{Kind: "label", Value: strings.TrimSpace(raw[len("label="):]), Nth: -1}, nil

	case strings.HasPrefix(lower, "placeholder="):
		return target{Kind: "placeholder", Value: strings.TrimSpace(raw[len("placeholder="):]), Nth: -1}, nil

	case strings.HasPrefix(lower, "xpath="):
		sel := strings.TrimSpace(raw[len("xpath="):])
		return target{Kind: "xpath", Selector: sel, By: chromedp.BySearch, Nth: -1}, nil

	case strings.HasPrefix(lower, "text="):
		text := strings.TrimSpace(raw[len("text="):])
		sel := fmt.Sprintf(`//*[contains(normalize-space(.), %q)]`, text)
		return target{Kind: "xpath", Selector: sel, By: chromedp.BySearch, Value: text, Nth: -1}, nil

	case strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "(//"):
		return target{Kind: "xpath", Selector: raw, By: chromedp.BySearch, Nth: -1}, nil

	default:
		return target{Kind: "css", Selector: raw, By: chromedp.ByQuery, Nth: -1}, nil
	}
}

// parseSelector keeps the chromedp CSS/XPath path used by legacy wait helpers.
func parseSelector(raw string) (string, chromedp.QueryOption, error) {
	t, err := parseTarget(raw)
	if err != nil {
		return "", nil, err
	}
	switch t.Kind {
	case "css", "xpath":
		return t.Selector, t.By, nil
	default:
		return "", nil, fmt.Errorf("selector %q is not supported by browser_wait — use CSS, text=, or xpath=; for role/ref use browser_wait_for or browser_find", raw)
	}
}

func (t target) needsJS() bool {
	switch t.Kind {
	case "ref", "role", "label", "placeholder":
		return true
	default:
		return false
	}
}

type locateOpts struct {
	Exact bool
	Nth   int // -1 = first
}

func chooseNth(optNth, targetNth int) int {
	if optNth >= 0 {
		return optNth
	}
	if targetNth >= 0 {
		return targetNth
	}
	return -1
}

// locateElementJS returns an IIFE that resolves to a DOM element or throws.
func locateElementJS(t target, opts locateOpts) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"kind":  t.Kind,
		"value": t.Value,
		"name":  t.Name,
		"exact": opts.Exact || t.Exact,
		"nth":   chooseNth(opts.Nth, t.Nth),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`(%s)(%s)`, locateElementFn, string(payload)), nil
}

// locateElementFn is a JS function (q) => Element that finds a target.
const locateElementFn = `function(q) {
  const exact = !!q.exact;
  const nth = (typeof q.nth === 'number' && q.nth >= 0) ? q.nth : 0;
  const norm = (s) => (s || '').replace(/\s+/g, ' ').trim();
  const matchName = (got, want) => {
    if (!want) return true;
    const a = norm(got).toLowerCase();
    const b = norm(want).toLowerCase();
    return exact ? a === b : a.includes(b);
  };
  const isVisible = (el) => {
    if (!el || !(el instanceof Element)) return false;
    const st = window.getComputedStyle(el);
    if (!st || st.display === 'none' || st.visibility === 'hidden' || Number(st.opacity) === 0) return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const accName = (el) => {
    if (!el) return '';
    const aria = el.getAttribute('aria-label');
    if (aria) return norm(aria);
    const labelled = el.getAttribute('aria-labelledby');
    if (labelled) {
      const parts = labelled.split(/\s+/).map(id => {
        const n = document.getElementById(id);
        return n ? norm(n.textContent) : '';
      }).filter(Boolean);
      if (parts.length) return parts.join(' ');
    }
    if (el.id) {
      try {
        const lab = document.querySelector('label[for="' + CSS.escape(el.id) + '"]');
        if (lab) return norm(lab.textContent);
      } catch (e) {}
    }
    const wrap = el.closest && el.closest('label');
    if (wrap) return norm(wrap.textContent);
    const ph = el.getAttribute('placeholder');
    if (ph) return norm(ph);
    const title = el.getAttribute('title');
    if (title) return norm(title);
    const alt = el.getAttribute('alt');
    if (alt) return norm(alt);
    const val = el.getAttribute('value');
    if (val && (el.tagName === 'INPUT' || el.tagName === 'BUTTON')) return norm(val);
    return norm(el.textContent).slice(0, 120);
  };
  const implicitRole = (el) => {
    const explicit = (el.getAttribute('role') || '').toLowerCase();
    if (explicit) return explicit;
    const tag = el.tagName.toLowerCase();
    const type = (el.getAttribute('type') || '').toLowerCase();
    if (tag === 'a' && el.hasAttribute('href')) return 'link';
    if (tag === 'button') return 'button';
    if (tag === 'input') {
      if (['button','submit','reset','image'].includes(type)) return 'button';
      if (type === 'checkbox') return 'checkbox';
      if (type === 'radio') return 'radio';
      return 'textbox';
    }
    if (tag === 'textarea') return 'textbox';
    if (tag === 'select') return 'combobox';
    if (tag === 'option') return 'option';
    if (tag === 'img') return 'img';
    if (/^h[1-6]$/.test(tag)) return 'heading';
    if (tag === 'nav') return 'navigation';
    if (tag === 'main') return 'main';
    if (tag === 'header') return 'banner';
    if (tag === 'footer') return 'contentinfo';
    if (tag === 'form') return 'form';
    if (el.isContentEditable) return 'textbox';
    return '';
  };

  let matches = [];
  if (q.kind === 'ref') {
    const store = window.__cwsRefs;
    const el = store && store[q.value];
    if (!el || !document.contains(el)) {
      throw new Error('ref ' + q.value + ' not found — call browser_a11y_snapshot again');
    }
    matches = [el];
  } else if (q.kind === 'role') {
    const wantRole = (q.value || '').toLowerCase();
    matches = Array.from(document.querySelectorAll('*')).filter(el => {
      if (!isVisible(el)) return false;
      if (implicitRole(el) !== wantRole) return false;
      return matchName(accName(el), q.name || '');
    });
  } else if (q.kind === 'label') {
    const seen = new Set();
    const add = (el) => { if (el && isVisible(el) && !seen.has(el)) { seen.add(el); matches.push(el); } };
    for (const lab of document.querySelectorAll('label')) {
      if (!matchName(lab.textContent, q.value)) continue;
      let el = null;
      const fr = lab.getAttribute('for');
      if (fr) el = document.getElementById(fr);
      if (!el) el = lab.querySelector('input,textarea,select,[contenteditable="true"]');
      add(el);
    }
    for (const el of document.querySelectorAll('input,textarea,select,button,[role],[contenteditable="true"]')) {
      if (matchName(accName(el), q.value)) add(el);
    }
  } else if (q.kind === 'placeholder') {
    matches = Array.from(document.querySelectorAll('input[placeholder],textarea[placeholder]')).filter(el => {
      if (!isVisible(el)) return false;
      return matchName(el.getAttribute('placeholder'), q.value);
    });
  } else {
    throw new Error('unsupported locate kind: ' + q.kind);
  }

  if (!matches.length) {
    throw new Error('no match for ' + q.kind + '=' + (q.value || '') + (q.name ? '[name=' + q.name + ']' : ''));
  }
  if (nth >= matches.length) {
    throw new Error('nth=' + nth + ' out of range (' + matches.length + ' matches)');
  }
  const el = matches[nth];
  el.scrollIntoView({ block: 'center', inline: 'nearest' });
  return el;
}`

func runLocateAction(ctx context.Context, t target, opts locateOpts, action, value string) error {
	locate, err := locateElementJS(t, opts)
	if err != nil {
		return err
	}
	op, err := json.Marshal(map[string]string{"action": action, "value": value})
	if err != nil {
		return err
	}
	expr := fmt.Sprintf(`(() => {
  const el = %s;
  const op = %s;
  if (!el) throw new Error('element missing');
  const tag = (el.tagName || '').toLowerCase();
  switch (op.action) {
    case 'click':
      el.focus();
      el.click();
      return true;
    case 'hover': {
      const r = el.getBoundingClientRect();
      const x = r.left + r.width / 2;
      const y = r.top + r.height / 2;
      for (const type of ['pointerover','pointerenter','mouseover','mouseenter','mousemove']) {
        el.dispatchEvent(new MouseEvent(type, { bubbles: true, clientX: x, clientY: y, view: window }));
      }
      return true;
    }
    case 'fill': {
      el.focus();
      if (tag === 'select') {
        el.value = op.value;
        el.dispatchEvent(new Event('input', { bubbles: true }));
        el.dispatchEvent(new Event('change', { bubbles: true }));
        return true;
      }
      if (tag === 'input' || tag === 'textarea') {
        const proto = tag === 'input' ? window.HTMLInputElement.prototype : window.HTMLTextAreaElement.prototype;
        const desc = Object.getOwnPropertyDescriptor(proto, 'value');
        if (desc && desc.set) desc.set.call(el, '');
        else el.value = '';
        el.dispatchEvent(new Event('input', { bubbles: true }));
        if (desc && desc.set) desc.set.call(el, op.value);
        else el.value = op.value;
        el.dispatchEvent(new Event('input', { bubbles: true }));
        el.dispatchEvent(new Event('change', { bubbles: true }));
        return true;
      }
      if (el.isContentEditable) {
        el.textContent = op.value;
        el.dispatchEvent(new Event('input', { bubbles: true }));
        return true;
      }
      throw new Error('cannot fill element <' + tag + '>');
    }
    case 'select': {
      if (tag !== 'select') throw new Error('select requires a <select> element');
      const want = (op.value || '').trim();
      let found = false;
      for (const opt of Array.from(el.options)) {
        if (opt.value === want || (opt.text || '').trim() === want || opt.label === want) {
          el.value = opt.value;
          found = true;
          break;
        }
      }
      if (!found) {
        for (const opt of Array.from(el.options)) {
          if (((opt.text || '') + ' ' + (opt.label || '')).toLowerCase().includes(want.toLowerCase())) {
            el.value = opt.value;
            found = true;
            break;
          }
        }
      }
      if (!found) throw new Error('option not found: ' + want);
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    }
    case 'scroll':
      el.scrollIntoView({ block: 'center', inline: 'nearest' });
      return true;
    default:
      throw new Error('unknown action ' + op.action);
  }
})()`, locate, string(op))

	var ok bool
	return chromedp.Run(ctx, chromedp.Evaluate(expr, &ok, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithReturnByValue(true).WithAwaitPromise(true)
	}))
}

func clickTarget(ctx context.Context, t target, opts locateOpts) error {
	if !t.needsJS() {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(t.Selector, t.By),
			chromedp.Click(t.Selector, t.By, chromedp.NodeVisible),
		)
	}
	return runLocateAction(ctx, t, opts, "click", "")
}

func fillTarget(ctx context.Context, t target, value string, opts locateOpts) error {
	if !t.needsJS() {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(t.Selector, t.By),
			chromedp.Clear(t.Selector, t.By),
			chromedp.SendKeys(t.Selector, value, t.By),
		)
	}
	return runLocateAction(ctx, t, opts, "fill", value)
}

func hoverTarget(ctx context.Context, t target, opts locateOpts) error {
	if !t.needsJS() {
		// Resolve via temporary ref-less JS using CSS/XPath query inside the page.
		selJSON, _ := json.Marshal(t.Selector)
		kind := t.Kind
		expr := fmt.Sprintf(`(() => {
  let el = null;
  const sel = %s;
  if (%q === 'xpath') {
    const snap = document.evaluate(sel, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
    el = snap.singleNodeValue;
  } else {
    el = document.querySelector(sel);
  }
  if (!el) throw new Error('hover target not found');
  el.scrollIntoView({ block: 'center', inline: 'nearest' });
  const r = el.getBoundingClientRect();
  const x = r.left + r.width / 2;
  const y = r.top + r.height / 2;
  for (const type of ['pointerover','pointerenter','mouseover','mouseenter','mousemove']) {
    el.dispatchEvent(new MouseEvent(type, { bubbles: true, clientX: x, clientY: y, view: window }));
  }
  return true;
})()`, string(selJSON), kind)
		var ok bool
		return chromedp.Run(ctx,
			chromedp.WaitVisible(t.Selector, t.By),
			chromedp.Evaluate(expr, &ok),
		)
	}
	return runLocateAction(ctx, t, opts, "hover", "")
}

func selectTarget(ctx context.Context, t target, value string, opts locateOpts) error {
	if !t.needsJS() {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(t.Selector, t.By),
			chromedp.SetValue(t.Selector, value, t.By),
		)
	}
	return runLocateAction(ctx, t, opts, "select", value)
}

func scrollTarget(ctx context.Context, t target, opts locateOpts) error {
	if !t.needsJS() {
		return chromedp.Run(ctx,
			chromedp.ScrollIntoView(t.Selector, t.By),
		)
	}
	return runLocateAction(ctx, t, opts, "scroll", "")
}

func truncateStr(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
