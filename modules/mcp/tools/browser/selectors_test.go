package browser

import "testing"

func TestParseTarget(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		val  string
		name string
	}{
		{"#login", "css", "", ""},
		{"text=Sign in", "xpath", "Sign in", ""},
		{"xpath=//button", "xpath", "", ""},
		{"//button[@type='submit']", "xpath", "", ""},
		{"ref=e12", "ref", "e12", ""},
		{`role=button[name="Sign in"]`, "role", "button", "Sign in"},
		{"role=textbox", "role", "textbox", ""},
		{"label=Email", "label", "Email", ""},
		{"placeholder=Search", "placeholder", "Search", ""},
	}
	for _, tc := range cases {
		got, err := parseTarget(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got.Kind != tc.kind {
			t.Fatalf("%q: kind=%q want %q", tc.in, got.Kind, tc.kind)
		}
		if tc.val != "" && got.Value != tc.val && got.Selector == "" {
			if got.Value != tc.val {
				t.Fatalf("%q: value=%q want %q", tc.in, got.Value, tc.val)
			}
		}
		if tc.kind == "ref" && got.Value != tc.val {
			t.Fatalf("%q: ref value=%q want %q", tc.in, got.Value, tc.val)
		}
		if tc.kind == "role" {
			if got.Value != tc.val || got.Name != tc.name {
				t.Fatalf("%q: role=%q name=%q want %q/%q", tc.in, got.Value, got.Name, tc.val, tc.name)
			}
		}
		if tc.kind == "label" || tc.kind == "placeholder" {
			if got.Value != tc.val {
				t.Fatalf("%q: value=%q want %q", tc.in, got.Value, tc.val)
			}
		}
	}
}

func TestParseTargetEmpty(t *testing.T) {
	if _, err := parseTarget("  "); err == nil {
		t.Fatal("expected error")
	}
}
