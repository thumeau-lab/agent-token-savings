package render

import (
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

func TestFormatCountLangFrenchUsesDecimalComma(t *testing.T) {
	got := FormatCountLang(10_400_000, i18n.FR)
	if got != "10,4 M" {
		t.Fatalf("FormatCountLang(10_400_000, fr) = %q, want %q", got, "10,4 M")
	}
}

func TestFormatCountLangEnglishUnchanged(t *testing.T) {
	got := FormatCountLang(9_870_000, i18n.EN)
	if got != "9.87 M" {
		t.Fatalf("FormatCountLang(9_870_000, en) = %q, want %q", got, "9.87 M")
	}
}

// TestFormatCountLangFrenchUsesMdForBillionSuffix pins card point 7:
// French never writes the English "B" (billion) scale letter -- it
// must read "Md" (milliard). K and M stay unchanged in both locales.
func TestFormatCountLangFrenchUsesMdForBillionSuffix(t *testing.T) {
	got := FormatCountLang(1_860_000_000, i18n.FR)
	if got != "1,86 Md" {
		t.Fatalf("FormatCountLang(1_860_000_000, fr) = %q, want %q", got, "1,86 Md")
	}
}

// TestFormatCountLangEnglishKeepsBForBillion: English is unaffected by
// the French "Md" substitution.
func TestFormatCountLangEnglishKeepsBForBillion(t *testing.T) {
	got := FormatCountLang(1_860_000_000, i18n.EN)
	if got != "1.86 B" {
		t.Fatalf("FormatCountLang(1_860_000_000, en) = %q, want %q", got, "1.86 B")
	}
}

func TestFormatUSDLangFrenchPutsSignAfterAmount(t *testing.T) {
	got := FormatUSDLang(110.79, i18n.FR)
	if got != "110,79 $" {
		t.Fatalf("FormatUSDLang(110.79, fr) = %q, want %q", got, "110,79 $")
	}
}

func TestFormatUSDLangEnglishUnchanged(t *testing.T) {
	got := FormatUSDLang(12.40, i18n.EN)
	if got != "$12.40" {
		t.Fatalf("FormatUSDLang(12.40, en) = %q, want %q", got, "$12.40")
	}
}

func TestFormatPercentLangFrenchUsesCommaAndSpaceBeforePercent(t *testing.T) {
	got := FormatPercentLang(97.1, i18n.FR, 1)
	if got != "97,1 %" {
		t.Fatalf("FormatPercentLang(97.1, fr, 1) = %q, want %q", got, "97,1 %")
	}
}

func TestFormatPercentLangEnglishUnchanged(t *testing.T) {
	got := FormatPercentLang(86.7, i18n.EN, 1)
	if got != "86.7%" {
		t.Fatalf("FormatPercentLang(86.7, en, 1) = %q, want %q", got, "86.7%")
	}
}

func TestFormatShortDateLangFrenchIsDayMonth(t *testing.T) {
	got := FormatShortDateLang("2026-10-07", i18n.FR)
	if got != "07/10" {
		t.Fatalf("FormatShortDateLang(2026-10-07, fr) = %q, want %q", got, "07/10")
	}
}

func TestFormatShortDateLangEnglishIsMonthDay(t *testing.T) {
	got := FormatShortDateLang("2026-10-07", i18n.EN)
	if got != "10/07" {
		t.Fatalf("FormatShortDateLang(2026-10-07, en) = %q, want %q", got, "10/07")
	}
}

func TestFormatShortDateLangReturnsInputUnchangedWhenUnparsable(t *testing.T) {
	got := FormatShortDateLang("not-a-date", i18n.FR)
	if got != "not-a-date" {
		t.Fatalf("FormatShortDateLang(not-a-date, fr) = %q, want input unchanged", got)
	}
}
