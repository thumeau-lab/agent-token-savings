package i18n

import "testing"

func getenvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDetectLangFlagWins(t *testing.T) {
	got := DetectLang("fr", getenvFrom(map[string]string{"ATS_LANG": "en", "LANG": "en_US.UTF-8"}))
	if got != FR {
		t.Fatalf("DetectLang = %q, want fr (flag must win)", got)
	}
}

func TestDetectLangEnvVarWinsOverLocale(t *testing.T) {
	got := DetectLang("", getenvFrom(map[string]string{"ATS_LANG": "fr", "LANG": "en_US.UTF-8"}))
	if got != FR {
		t.Fatalf("DetectLang = %q, want fr (ATS_LANG must win over LANG)", got)
	}
}

func TestDetectLangFallsBackToLocaleStartingWithFr(t *testing.T) {
	got := DetectLang("", getenvFrom(map[string]string{"LANG": "fr_FR.UTF-8"}))
	if got != FR {
		t.Fatalf("DetectLang = %q, want fr (LANG starts with fr)", got)
	}
}

func TestDetectLangLCAllBeatsLang(t *testing.T) {
	got := DetectLang("", getenvFrom(map[string]string{"LC_ALL": "fr_FR.UTF-8", "LANG": "en_US.UTF-8"}))
	if got != FR {
		t.Fatalf("DetectLang = %q, want fr (LC_ALL must win over LANG)", got)
	}
}

func TestDetectLangDefaultsToEnglish(t *testing.T) {
	got := DetectLang("", getenvFrom(map[string]string{}))
	if got != EN {
		t.Fatalf("DetectLang = %q, want en (empty environment)", got)
	}
}

func TestDetectLangNonFrenchLocaleIsEnglish(t *testing.T) {
	got := DetectLang("", getenvFrom(map[string]string{"LANG": "de_DE.UTF-8"}))
	if got != EN {
		t.Fatalf("DetectLang = %q, want en (German locale)", got)
	}
}
