// Package i18n is the single user-facing string catalog for ats: every
// string shown to a human (TUI chrome, cards, verdicts, recommendations,
// help, errors, usage) goes through T(lang, key, args...) so English and
// French never drift out of two separate code paths. --json output is
// never translated: it stays English/machine format in every language
// (see cliapp's JSON encoders, which never call this package).
package i18n

import "strings"

// Lang is one of the two languages ats supports.
type Lang string

const (
	EN Lang = "en"
	FR Lang = "fr"
)

// normalizeLang maps a raw string (flag value, env var, locale) to a
// known Lang, or "" if it names neither.
func normalizeLang(s string) Lang {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fr":
		return FR
	case "en":
		return EN
	default:
		return ""
	}
}

// DetectLang picks the UI language with this precedence: the --lang flag
// value (already parsed by the caller) first, then the ATS_LANG env var,
// then the standard POSIX locale precedence (LC_ALL overrides
// LC_MESSAGES overrides LANG, first non-empty one wins) -- fr if that
// locale value starts with "fr" (case-insensitive), en otherwise. An
// empty environment defaults to en.
//
// getenv is injected (instead of calling os.Getenv directly) so tests
// can exercise every precedence case without mutating process-wide env
// vars, the same pattern render.LocaleIsUTF8 already uses.
func DetectLang(flagLang string, getenv func(string) string) Lang {
	if l := normalizeLang(flagLang); l != "" {
		return l
	}
	if l := normalizeLang(getenv("ATS_LANG")); l != "" {
		return l
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := getenv(key)
		if v == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(v), "fr") {
			return FR
		}
		return EN
	}
	return EN
}
