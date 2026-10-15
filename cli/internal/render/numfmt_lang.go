package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// FormatCountLang is FormatCount, localized: French uses a decimal comma
// ("10,4 M") instead of a period ("10.4 M"). FormatCount never emits a
// thousands grouping comma below the K/M/B scale, so a plain character
// swap is safe and does not collide with any other separator.
func FormatCountLang(n int64, lang i18n.Lang) string {
	s := FormatCount(n)
	if lang != i18n.FR {
		return s
	}
	return strings.Replace(s, ".", ",", 1)
}

// FormatUSDLang is FormatUSD, localized: French writes the amount with a
// decimal comma and the "$" sign trailing after a space ("110,79 $"),
// matching docs/measurements.md and README.fr.md's own number style.
// Card-requested "narrow grouping not required": no thousands separator.
func FormatUSDLang(v float64, lang i18n.Lang) string {
	if lang != i18n.FR {
		return FormatUSD(v)
	}
	neg := v < 0
	if neg {
		v = -v
	}
	v = math.Round(v*100) / 100
	whole := int64(v)
	cents := int64(math.Round((v - float64(whole)) * 100))
	if cents == 100 {
		whole++
		cents = 0
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s%d,%02d $", sign, whole, cents)
}

// FormatPercentLang renders v (already a 0-100 percentage) with decimals
// digits after the point, localized: French uses a decimal comma and a
// space before the "%" sign ("97,1 %"), English keeps "97.1%".
func FormatPercentLang(v float64, lang i18n.Lang, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if lang != i18n.FR {
		return s + "%"
	}
	return strings.Replace(s, ".", ",", 1) + " %"
}

// FormatShortDateLang renders an ISO "YYYY-MM-DD" day string (the shape
// hermesusage.DailyRow.Day / claudecode.DailyRow.Day already use) as a
// short, localized date: day/month for French ("07/10"), month/day for
// English ("10/07") -- each locale's usual numeric date order, so the
// Cache panel's per-day sparkline can show its first/last day without
// spelling out month names (which would need their own translation
// table). day is returned unchanged if it does not parse as that exact
// layout (should not happen: every caller passes an already-validated
// DailyRow.Day).
func FormatShortDateLang(day string, lang i18n.Lang) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	if lang == i18n.FR {
		return t.Format("02/01")
	}
	return t.Format("01/02")
}
