package render

import "strings"

// LocaleIsUTF8 reports whether the process' locale is UTF-8, following the
// standard POSIX precedence: LC_ALL overrides LC_CTYPE overrides LANG, the
// first non-empty one of those three wins. "UTF-8" is matched
// case-insensitively, with or without the dash (e.g. "en_US.UTF-8",
// "C.utf8"). An empty environment (no LANG/LC_ALL/LC_CTYPE set, the POSIX/C
// default) is treated as non-UTF-8, matching locale(1)'s own fallback.
//
// getenv is injected (instead of calling os.Getenv directly) so tests can
// exercise every precedence case without mutating process-wide env vars.
func LocaleIsUTF8(getenv func(string) string) bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := getenv(key)
		if v == "" {
			continue
		}
		v = strings.ToUpper(v)
		return strings.Contains(v, "UTF-8") || strings.Contains(v, "UTF8")
	}
	return false
}
