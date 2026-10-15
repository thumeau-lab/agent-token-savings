package render

import "testing"

// TestLocaleIsUTF8ChecksLCAllThenLCCTYPEThenLANG verifies the standard
// POSIX locale precedence (LC_ALL overrides LC_CTYPE overrides LANG), and
// that a "UTF-8" (any case, with or without the dash) substring anywhere
// in the winning variable counts as a UTF-8 locale.
func TestLocaleIsUTF8ChecksLCAllThenLCCTYPEThenLANG(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"all empty defaults to non-UTF-8 (POSIX/C)", map[string]string{}, false},
		{"LANG with UTF-8", map[string]string{"LANG": "en_US.UTF-8"}, true},
		{"LANG with lowercase utf8 (no dash)", map[string]string{"LANG": "en_US.utf8"}, true},
		{"LC_CTYPE wins over LANG", map[string]string{"LANG": "en_US.UTF-8", "LC_CTYPE": "POSIX"}, false},
		{"LC_ALL wins over LC_CTYPE and LANG", map[string]string{"LANG": "POSIX", "LC_CTYPE": "POSIX", "LC_ALL": "C.UTF-8"}, true},
		{"plain POSIX is not UTF-8", map[string]string{"LANG": "POSIX"}, false},
		{"plain C is not UTF-8", map[string]string{"LANG": "C"}, false},
	}
	for _, c := range cases {
		getenv := func(key string) string { return c.env[key] }
		if got := LocaleIsUTF8(getenv); got != c.want {
			t.Errorf("%s: LocaleIsUTF8() = %v, want %v", c.name, got, c.want)
		}
	}
}
