package update

import (
	"strconv"
	"strings"
)

// normalizeVersion strips a leading "v"/"V" for comparison.
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}

// compareSemver returns -1 if a<b, 0 if equal, 1 if a>b.
// Handles dotted numeric versions; non-numeric suffixes are compared lexicographically.
func compareSemver(a, b string) int {
	a = normalizeVersion(a)
	b = normalizeVersion(b)
	if a == b {
		return 0
	}
	if a == "" || a == "(untracked)" {
		return -1
	}
	if b == "" || b == "(untracked)" {
		return 1
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		// Split numeric prefix from suffix (e.g. 1-rc.1 → 1 + -rc)
		an, asuf := splitNumSuffix(av)
		bn, bsuf := splitNumSuffix(bv)
		if an != bn {
			if an < bn {
				return -1
			}
			return 1
		}
		if asuf != bsuf {
			if asuf == "" {
				return 1 // 1.0 > 1.0-rc
			}
			if bsuf == "" {
				return -1
			}
			if asuf < bsuf {
				return -1
			}
			return 1
		}
	}
	return 0
}

func splitNumSuffix(s string) (num int64, suffix string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, s
	}
	n, _ := strconv.ParseInt(s[:i], 10, 64)
	return n, s[i:]
}

func isNewer(candidate, current string) bool {
	return compareSemver(candidate, current) > 0
}
