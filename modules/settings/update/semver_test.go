package update

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.2.0", "1.1.9", 1},
		{"1.0.0", "v1.0.1", -1},
		{"(untracked)", "1.0.0", -1},
		{"2.0.0-rc1", "2.0.0", -1},
	}
	for _, tc := range cases {
		got := compareSemver(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("compareSemver(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
