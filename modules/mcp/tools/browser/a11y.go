package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// a11ySnapshotJS builds a compact accessibility-oriented tree with stable refs (e1, e2, …).
// Refs are stored on window.__cwsRefs for subsequent ref= actions.
const a11ySnapshotJS = `(() => {
  const opts = arguments[0] || {};
  const interactiveOnly = opts.interactive_only !== false;
  const maxDepth = typeof opts.max_depth === 'number' ? opts.max_depth : 12;
  const maxNodes = typeof opts.max_nodes === 'number' ? opts.max_nodes : 200;

  const norm = (s) => (s || '').replace(/\s+/g, ' ').trim();
  const isVisible = (el) => {
    if (!el || el.nodeType !== 1) return false;
    const st = window.getComputedStyle(el);
    if (!st || st.display === 'none' || st.visibility === 'hidden' || Number(st.opacity) === 0) return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
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
      if (type === 'hidden') return '';
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
    if (tag === 'ul' || tag === 'ol') return 'list';
    if (tag === 'li') return 'listitem';
    if (el.isContentEditable) return 'textbox';
    return '';
  };

  const accName = (el) => {
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
    if (wrap && wrap !== el) return norm(wrap.textContent);
    const ph = el.getAttribute('placeholder');
    if (ph) return norm(ph);
    const title = el.getAttribute('title');
    if (title) return norm(title);
    const alt = el.getAttribute('alt');
    if (alt) return norm(alt);
    const role = implicitRole(el);
    if (role === 'textbox' || role === 'combobox' || role === 'checkbox' || role === 'radio') {
      return '';
    }
    if (role === 'heading' || role === 'button' || role === 'link' || role === 'option' || role === 'menuitem') {
      return norm(el.innerText || el.textContent).slice(0, 80);
    }
    return '';
  };

  const interesting = (el) => {
    const role = implicitRole(el);
    if (!role) {
      if (!interactiveOnly && /^(P|DIV|SPAN|SECTION|ARTICLE)$/.test(el.tagName) && norm(el.innerText).length && norm(el.innerText).length < 100) {
        return 'text';
      }
      return '';
    }
    if (interactiveOnly) {
      const interactive = new Set([
        'button','link','textbox','checkbox','radio','combobox','listbox','option',
        'menuitem','tab','switch','searchbox','slider','spinbutton'
      ]);
      if (!interactive.has(role) && role !== 'heading') return '';
    }
    return role;
  };

  window.__cwsRefs = Object.create(null);
  let counter = 0;
  const lines = [];
  let nodeCount = 0;

  const walk = (el, depth) => {
    if (!el || el.nodeType !== 1 || depth > maxDepth || nodeCount >= maxNodes) return;
    const tag = el.tagName.toLowerCase();
    if (['script','style','noscript','svg','path','meta','link','head'].includes(tag)) return;
    if (!isVisible(el)) return;

    const role = interesting(el);
    if (role) {
      const ref = 'e' + (++counter);
      window.__cwsRefs[ref] = el;
      el.setAttribute('data-cws-ref', ref);
      nodeCount++;

      let name = accName(el);
      const states = [];
      if (el.disabled || el.getAttribute('aria-disabled') === 'true') states.push('disabled');
      if (el.checked || el.getAttribute('aria-checked') === 'true') states.push('checked');
      if (el.getAttribute('aria-expanded') === 'true') states.push('expanded');
      if (el.getAttribute('aria-expanded') === 'false') states.push('collapsed');
      if (el.getAttribute('aria-selected') === 'true') states.push('selected');
      if (el.getAttribute('aria-pressed') === 'true') states.push('pressed');
      if (document.activeElement === el) states.push('focused');

      let value = '';
      if (role === 'textbox' || role === 'searchbox' || role === 'combobox' || role === 'spinbutton') {
        value = norm(el.value != null ? String(el.value) : '');
      }
      if (role === 'heading') {
        const lvl = tag.match(/^h([1-6])$/);
        if (lvl) states.push('level ' + lvl[1]);
      }

      const indent = '  '.repeat(Math.min(depth, 8));
      let line = indent + '- ' + role + ' [ref=' + ref + ']';
      if (name) line += ' "' + name.replace(/"/g, '\\"') + '"';
      if (value) line += ' value="' + value.replace(/"/g, '\\"').slice(0, 60) + '"';
      if (states.length) line += ' (' + states.join(', ') + ')';
      lines.push(line);
    }

    const children = el.children || [];
    for (let i = 0; i < children.length; i++) {
      walk(children[i], depth + (role ? 1 : 0));
      if (nodeCount >= maxNodes) break;
    }
  };

  walk(document.body, 0);
  return {
    tree: lines.join('\n'),
    refs: counter,
    truncated: nodeCount >= maxNodes
  };
})()`

type A11ySnapshotInput struct {
	InteractiveOnly *bool `json:"interactive_only,omitempty" jsonschema:"default true; only buttons/links/inputs/headings (smaller)"`
	MaxChars        int   `json:"max_chars,omitempty" jsonschema:"max tree chars (default 4000)"`
	MaxNodes        int   `json:"max_nodes,omitempty" jsonschema:"max numbered refs (default 200)"`
}

type A11ySnapshotOutput struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Tree      string `json:"tree"`
	Refs      int    `json:"refs"`
	Truncated bool   `json:"truncated,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

func (c *Controller) A11ySnapshotTool(ctx context.Context, _ *mcp.CallToolRequest, input A11ySnapshotInput) (*mcp.CallToolResult, any, error) {
	maxChars := input.MaxChars
	if maxChars <= 0 {
		maxChars = 4000
	}
	maxNodes := input.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 200
	}
	interactiveOnly := true
	if input.InteractiveOnly != nil {
		interactiveOnly = *input.InteractiveOnly
	}

	var url, title string
	var result map[string]any
	if err := c.withTab(ctx, 30*time.Second, func(runCtx context.Context) error {
		opts, _ := json.Marshal(map[string]any{
			"interactive_only": interactiveOnly,
			"max_nodes":        maxNodes,
		})
		expr := strings.Replace(a11ySnapshotJS, "arguments[0] || {}", string(opts), 1)
		return chromedp.Run(runCtx,
			chromedp.Location(&url),
			chromedp.Title(&title),
			chromedp.Evaluate(expr, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
				return p.WithReturnByValue(true)
			}),
		)
	}); err != nil {
		return nil, nil, err
	}

	tree, _ := result["tree"].(string)
	refs := toInt(result["refs"])
	truncated, _ := result["truncated"].(bool)
	tree = truncateStr(tree, maxChars)

	out := A11ySnapshotOutput{
		URL:       url,
		Title:     title,
		Tree:      tree,
		Refs:      refs,
		Truncated: truncated,
		Hint:      "Act with selector ref=eN (from snapshot). Prefer browser_assert / browser_find over full text snapshots.",
	}
	return nil, out, nil
}

type FindInput struct {
	Role           string `json:"role,omitempty" jsonschema:"ARIA/implicit role e.g. button, link, textbox"`
	Name           string `json:"name,omitempty" jsonschema:"accessible name / label substring"`
	Text           string `json:"text,omitempty" jsonschema:"visible text substring (any element)"`
	Selector       string `json:"selector,omitempty" jsonschema:"CSS, text, role, label, placeholder, or ref selector"`
	Exact          bool   `json:"exact,omitempty" jsonschema:"require exact name/text match"`
	Limit          int    `json:"limit,omitempty" jsonschema:"max matches (default 10)"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type FindMatch struct {
	Ref      string `json:"ref,omitempty"`
	Role     string `json:"role,omitempty"`
	Name     string `json:"name,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Selector string `json:"selector_hint,omitempty"`
}

func (c *Controller) FindTool(ctx context.Context, _ *mcp.CallToolRequest, input FindInput) (*mcp.CallToolResult, any, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	query, _ := json.Marshal(map[string]any{
		"role":     strings.TrimSpace(input.Role),
		"name":     strings.TrimSpace(input.Name),
		"text":     strings.TrimSpace(input.Text),
		"selector": strings.TrimSpace(input.Selector),
		"exact":    input.Exact,
		"limit":    limit,
	})

	expr := fmt.Sprintf(`(() => {
  const q = %s;
  const norm = (s) => (s || '').replace(/\s+/g, ' ').trim();
  const matchName = (got, want) => {
    if (!want) return true;
    const a = norm(got).toLowerCase();
    const b = norm(want).toLowerCase();
    return q.exact ? a === b : a.includes(b);
  };
  const isVisible = (el) => {
    if (!el || !(el instanceof Element)) return false;
    const st = window.getComputedStyle(el);
    if (!st || st.display === 'none' || st.visibility === 'hidden') return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
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
    if (/^h[1-6]$/.test(tag)) return 'heading';
    if (el.isContentEditable) return 'textbox';
    return '';
  };
  const accName = (el) => {
    const aria = el.getAttribute('aria-label');
    if (aria) return norm(aria);
    if (el.id) {
      try {
        const lab = document.querySelector('label[for="' + CSS.escape(el.id) + '"]');
        if (lab) return norm(lab.textContent);
      } catch (e) {}
    }
    const ph = el.getAttribute('placeholder');
    if (ph) return norm(ph);
    return norm(el.innerText || el.textContent).slice(0, 80);
  };
  const cssHint = (el) => {
    if (el.id) return '#' + CSS.escape(el.id);
    const name = el.getAttribute('name');
    if (name) return el.tagName.toLowerCase() + '[name="' + name + '"]';
    const testid = el.getAttribute('data-testid');
    if (testid) return '[data-testid="' + testid + '"]';
    return el.tagName.toLowerCase();
  };

  // Ensure ref store exists and assign missing refs for matches.
  if (!window.__cwsRefs) window.__cwsRefs = Object.create(null);
  const ensureRef = (el) => {
    const existing = el.getAttribute('data-cws-ref');
    if (existing && window.__cwsRefs[existing] === el) return existing;
    let n = Object.keys(window.__cwsRefs).length + 1;
    let ref = 'e' + n;
    while (window.__cwsRefs[ref]) { n++; ref = 'e' + n; }
    window.__cwsRefs[ref] = el;
    el.setAttribute('data-cws-ref', ref);
    return ref;
  };

  let candidates = [];
  if (q.selector) {
    const sel = q.selector;
    const lower = sel.toLowerCase();
    if (lower.startsWith('ref=')) {
      const id = sel.slice(4).trim();
      const el = window.__cwsRefs && window.__cwsRefs[id];
      if (el) candidates = [el];
    } else if (lower.startsWith('role=')) {
      // handled via role/name below after parse — fall through using DOM scan with role from selector
      const m = sel.match(/^role=([a-z0-9_-]+)(?:\[name=(?:"([^"]*)"|'([^']*)'|([^\]]+))\])?$/i);
      if (m) {
        q.role = m[1];
        q.name = m[2] || m[3] || (m[4] || '').trim() || q.name;
      }
    } else if (lower.startsWith('label=')) {
      q.name = sel.slice(6).trim();
      q.role = q.role || '';
    } else if (lower.startsWith('placeholder=')) {
      const want = sel.slice(12).trim().toLowerCase();
      candidates = Array.from(document.querySelectorAll('input[placeholder],textarea[placeholder]')).filter(el => {
        const ph = (el.getAttribute('placeholder') || '').toLowerCase();
        return q.exact ? ph === want : ph.includes(want);
      });
    } else if (lower.startsWith('text=')) {
      q.text = sel.slice(5).trim();
    } else if (sel.startsWith('//') || lower.startsWith('xpath=')) {
      const xp = lower.startsWith('xpath=') ? sel.slice(6) : sel;
      const snap = document.evaluate(xp, document, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null);
      for (let i = 0; i < snap.snapshotLength; i++) candidates.push(snap.snapshotItem(i));
    } else {
      candidates = Array.from(document.querySelectorAll(sel));
    }
  }

  if (!candidates.length) {
    candidates = Array.from(document.querySelectorAll('a,button,input,textarea,select,[role],[contenteditable="true"],h1,h2,h3,h4,h5,h6'));
  }

  const out = [];
  for (const el of candidates) {
    if (!isVisible(el)) continue;
    const role = implicitRole(el);
    const name = accName(el);
    if (q.role && role !== q.role.toLowerCase()) continue;
    if (q.name && !matchName(name, q.name)) continue;
    if (q.text && !matchName(el.innerText || el.textContent, q.text)) continue;
    out.push({
      ref: ensureRef(el),
      role: role,
      name: name,
      tag: el.tagName.toLowerCase(),
      selector_hint: cssHint(el)
    });
    if (out.length >= q.limit) break;
  }
  return { matches: out, count: out.length };
})()`, string(query))

	var result map[string]any
	if err := c.withTab(ctx, defaultTimeout(input.TimeoutSeconds), func(runCtx context.Context) error {
		return chromedp.Run(runCtx, chromedp.Evaluate(expr, &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithReturnByValue(true)
		}))
	}); err != nil {
		return nil, nil, err
	}

	rawMatches, _ := result["matches"].([]any)
	matches := make([]FindMatch, 0, len(rawMatches))
	for _, m := range rawMatches {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		matches = append(matches, FindMatch{
			Ref:      asString(mm["ref"]),
			Role:     asString(mm["role"]),
			Name:     asString(mm["name"]),
			Tag:      asString(mm["tag"]),
			Selector: asString(mm["selector_hint"]),
		})
	}
	return nil, map[string]any{"matches": matches, "count": len(matches)}, nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(s)
	}
}
