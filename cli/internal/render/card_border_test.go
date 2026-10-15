package render

import (
	"image/color"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
)

// lipglossColorSGR renders a single probe character in c and extracts its
// SGR escape code (the part between "\x1b[" and "m"), so a test can
// assert a later render uses the exact same color without hardcoding an
// ANSI code of its own.
func lipglossColorSGR(c color.Color) string {
	probe := lipgloss.NewStyle().Foreground(c).Render("X")
	i := strings.Index(probe, "\x1b[")
	if i < 0 {
		return ""
	}
	rest := probe[i+2:]
	j := strings.Index(rest, "m")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// TestRenderCardEmbedsTitleInTopBorder is the btop-style requirement: the
// panel title lives inside the top border ("╭─ Cache ─────╮"), not as a
// separate line inside the body.
func TestRenderCardEmbedsTitleInTopBorder(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{Title: "claude-code", Verdict: "All good."}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	lines := strings.Split(stripped, "\n")
	if len(lines) == 0 {
		t.Fatalf("RenderCard produced no lines")
	}
	top := lines[0]
	if !strings.Contains(top, "claude-code") {
		t.Fatalf("top border line does not embed the title, got: %q", top)
	}
	if !strings.HasPrefix(top, "\u256d") || !strings.HasSuffix(top, "\u256e") {
		t.Fatalf("top border line is not a rounded-corner border (╭...╮), got: %q", top)
	}
}

// TestRenderCardKeyShortcutShownNextToTitle verifies the btop-style key
// digit (e.g. "1") shows up right next to the panel title in the border
// when Card.Key is set.
func TestRenderCardKeyShortcutShownNextToTitle(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{Key: "1", Title: "Cache"}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	top := strings.Split(stripped, "\n")[0]
	if !strings.Contains(top, "1 Cache") {
		t.Fatalf("top border does not show the key next to the title, got: %q", top)
	}
}

// TestRenderCardLegendShowsOneSwatchPerBucket verifies the cost-share
// legend (write/read/input/output) renders when the card carries one.
func TestRenderCardLegendShowsOneSwatchPerBucket(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{
		Title: "claude-code",
		Legend: []LegendItem{
			{Label: "write", Color: r.CostBucketColor("cache_write")},
			{Label: "read", Color: r.CostBucketColor("cache_read")},
			{Label: "input", Color: r.CostBucketColor("input")},
			{Label: "output", Color: r.CostBucketColor("output")},
		},
	}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	for _, want := range []string{"write", "read", "input", "output"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("legend missing %q, got:\n%s", want, stripped)
		}
	}
}

// TestRenderCardGraphBlockIsIncluded verifies an optional pre-rendered
// Graph block (the Cache panel's per-day history sparkline) shows up in
// the card body.
func TestRenderCardGraphBlockIsIncluded(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{Title: "claude-code", Graph: "Hit rate \u2581\u2582\u2587\u2588"}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "Hit rate") || !strings.Contains(stripped, "\u2581\u2582\u2587\u2588") {
		t.Fatalf("card missing Graph block, got:\n%s", stripped)
	}
}

// TestRenderCardBarUsesGivenColorNotAccent verifies a BarStat with an
// explicit Color renders using that color's SGR sequence rather than the
// renderer's single accent.
func TestRenderCardBarUsesGivenColorNotAccent(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	bad := r.SemanticColor(SemanticBad)
	card := Card{
		Title: "claude-code",
		Bars:  []BarStat{{Label: "hit rate", Frac: 0.1, Annotation: "10%", Color: bad}},
	}
	out := r.RenderCard(card, 80)
	wantCode := lipglossColorSGR(bad)
	if !strings.Contains(out, wantCode) {
		t.Fatalf("bar does not use the given Color's SGR code %q, got:\n%s", wantCode, out)
	}
}
