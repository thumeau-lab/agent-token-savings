package render

import (
	"strings"
	"testing"
)

func TestRenderCardIncludesTitleAndVerdict(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{
		Title:   "claude-code",
		Verdict: "Cache writes are 67% of the bill: 2 sessions rebuilt a warm cache.",
		Stats: []Stat{
			{Label: "Sessions", Value: "12"},
			{Label: "Calls", Value: "340"},
		},
	}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "claude-code") {
		t.Fatalf("card missing title, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Cache writes are 67%") {
		t.Fatalf("card missing verdict, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Sessions") || !strings.Contains(stripped, "12") {
		t.Fatalf("card missing stat grid entry, got:\n%s", stripped)
	}
}

func TestRenderCardIncludesBarsAndTable(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{
		Title: "claude-code",
		Bars: []BarStat{
			{Label: "hit rate", Frac: 0.867, Annotation: "86.7%"},
		},
		Table: &CardTable{
			Title:   "worst cache breaks",
			Headers: []string{"session", "time", "tokens rewritten"},
			Rows:    [][]string{{"two.jsonl", "2026-10-11T10:00:00Z", "50,000"}},
		},
	}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "86.7%") {
		t.Fatalf("card missing bar annotation, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "worst cache breaks") || !strings.Contains(stripped, "two.jsonl") {
		t.Fatalf("card missing table, got:\n%s", stripped)
	}
}

// stripANSI removes every ANSI CSI escape sequence from s, for assertions
// on the visible text content of colored output.
func stripANSI(s string) string {
	var out strings.Builder
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:i])
		s = s[i+2:]
		j := strings.IndexAny(s, "mK")
		if j < 0 {
			break
		}
		s = s[j+1:]
	}
	return out.String()
}

// TestRenderCardIncludesSubtitle verifies the card prints an optional
// subtitle line (e.g. "last 7 days") right under the title, so cache and
// ttl can show the look-back window they used.
func TestRenderCardIncludesSubtitle(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{Title: "claude-code", Subtitle: "last 7 days", Verdict: "All good."}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "last 7 days") {
		t.Fatalf("card missing subtitle, got:\n%s", stripped)
	}
}

// TestRenderCardStatGridSwitchesToOneColumnWhenTwoColumnsDontFit is the
// regression test for the TTL panel overflow bug (reviewer-reported,
// LANG=fr_FR.UTF-8, 170x46): a card with a long French stat label (e.g.
// "Ecritures ecart moyen (5-60 min)") at a narrow card width must not
// wrap a stat row onto a second line (which also broke the right
// border) -- the grid must fall back to one column per row instead.
func TestRenderCardStatGridSwitchesToOneColumnWhenTwoColumnsDontFit(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{
		Title: "claude-code",
		Stats: []Stat{
			{Label: "Sessions", Value: "150"},
			{Label: "Appels", Value: "594"},
			{Label: "Écritures 1er appel", Value: "1.20 M"},
			{Label: "Écritures écart moyen (5-60 min)", Value: "711 K"},
			{Label: "Écritures écart long (>1 h)", Value: "50.0 K"},
		},
	}
	// 84: the leftWidth the TUI's wide layout passes to the TTL panel at
	// 170 total columns (170/2 - 1), the exact width that reproduced the
	// bug.
	out := r.RenderCard(card, 84)
	stripped := stripANSI(out)
	lines := strings.Split(stripped, "\n")

	for _, s := range card.Stats {
		found := false
		for _, line := range lines {
			if strings.Contains(line, s.Label) {
				if !strings.Contains(line, s.Value) {
					t.Errorf("label %q is on a line without its value %q (stat row wrapped):\n%s", s.Label, s.Value, stripped)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("label %q not found in the rendered card:\n%s", s.Label, stripped)
		}
	}
}

// TestRenderCardStatGridAlwaysSeparatesLabelFromValue is the regression
// test for statCell gluing a label directly onto its value with no
// space when the label's rune length exactly equals the column's
// labelWidth (labelStyle.Width() then has no padding left to add): a
// stat whose label is as long as the card's longest label must still
// read as "label value", never "labelvalue".
func TestRenderCardStatGridAlwaysSeparatesLabelFromValue(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{
		Title: "hermes",
		Stats: []Stat{
			// The label is exactly as long as the card's longest label
			// (labelWidth), and the value is exactly as long as the
			// card's longest value (valueWidth): both columns pad to
			// exactly their own content length, leaving zero spaces on
			// either side.
			{Label: "tokens intégrés", Value: "19,3 K tok"},
			{Label: "x", Value: "1"},
		},
	}
	out := r.RenderCard(card, 80)
	stripped := stripANSI(out)
	if strings.Contains(stripped, "intégrés19,3 K tok") {
		t.Fatalf("label glued directly onto its value with no separator:\n%s", stripped)
	}
}

// TestRenderCardBarLabelColumnFitsLongestLabelWithoutWrapping is the
// regression test for the "CLAUDE.md (u" truncation / mid-row wrap bug:
// a bar whose label is longer than the old fixed 12-char column must
// render on one line, in full, not truncated and not wrapped onto a
// second line.
func TestRenderCardBarLabelColumnFitsLongestLabelWithoutWrapping(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	card := Card{
		Title: "hermes",
		Bars: []BarStat{
			{Label: "CLAUDE.md (user)", Frac: 0.5, Annotation: "1.2 KB"},
			{Label: "MCP servers (config.yaml)", Frac: 0.2, Annotation: "300 B"},
		},
	}
	out := r.RenderCard(card, 120)
	stripped := stripANSI(out)
	lines := strings.Split(stripped, "\n")
	for _, label := range []string{"CLAUDE.md (user)", "MCP servers (config.yaml)"} {
		found := false
		for _, line := range lines {
			if strings.Contains(line, label) {
				found = true
			}
		}
		if !found {
			t.Errorf("label %q not found whole on a single line, got:\n%s", label, stripped)
		}
	}
}
