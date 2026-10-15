package i18n

import "testing"

func TestTRendersEnglishByKey(t *testing.T) {
	got := T(EN, "tui.loading")
	if got != "loading..." {
		t.Fatalf("T(EN, tui.loading) = %q, want %q", got, "loading...")
	}
}

func TestTRendersFrenchByKey(t *testing.T) {
	got := T(FR, "tui.loading")
	if got != "chargement..." {
		t.Fatalf("T(FR, tui.loading) = %q, want %q", got, "chargement...")
	}
}

func TestTFormatsArgsWithSprintf(t *testing.T) {
	got := T(EN, "tui.window", 30)
	if got != "window: 30d" {
		t.Fatalf("T(EN, tui.window, 30) = %q, want %q", got, "window: 30d")
	}
}

// TestCatalogParity is the acceptance test named by the card: it fails if
// any catalog key exists in one language and not the other, so an entry
// added for only one language is caught immediately instead of silently
// falling back forever.
func TestCatalogParity(t *testing.T) {
	for key, entry := range catalog {
		for _, lang := range []Lang{EN, FR} {
			v, ok := entry[lang]
			if !ok || v == "" {
				t.Errorf("catalog key %q has no %s translation", key, lang)
			}
		}
	}
}

// TestCatalogHasNoEmptyKey guards against a key typo registering an
// accidental "" entry.
func TestCatalogHasNoEmptyKey(t *testing.T) {
	if _, ok := catalog[""]; ok {
		t.Fatalf("catalog has an empty-string key")
	}
}
