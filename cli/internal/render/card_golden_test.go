package render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files in testdata/")

// demoCard is the fixture used by the golden snapshot tests: one agent's
// full card (verdict, 2-column stat grid, bars, worst-cache-breaks table),
// exercising every element RenderCard supports.
func demoCard() Card {
	return Card{
		Title:   "claude-code",
		Verdict: "Cache writes are 67% of the bill: 2 sessions rebuilt a warm cache.",
		Stats: []Stat{
			{Label: "Sessions", Value: "12"},
			{Label: "Calls", Value: "340"},
			{Label: "Cache write", Value: "9.87 M"},
			{Label: "Cache read", Value: "64.7 M"},
			{Label: "Input", Value: "1.20 M"},
			{Label: "Output", Value: "230 K"},
			{Label: "Hit rate", Value: "86.7%"},
			{Label: "Bill", Value: "$12.40"},
		},
		Bars: []BarStat{
			{Label: "hit rate", Frac: 0.867, Annotation: "86.7%"},
			{Label: "write $", Frac: 0.67, Annotation: "67%"},
		},
		Table: &CardTable{
			Title:   "worst cache breaks",
			Headers: []string{"session", "time", "tokens rewritten"},
			Rows:    [][]string{{"two.jsonl", "2026-10-11T10:00:00Z", "50,000"}},
		},
	}
}

func goldenPath(name string) string {
	return filepath.Join("testdata", name)
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run `go test ./... -run TestCardGolden -update` once to create it)", path, err)
	}
	if got != string(want) {
		t.Fatalf("rendering does not match golden %s.\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

// TestCardGoldenWidth80 and TestCardGoldenWidth120 pin RenderCard's exact
// output at the two terminal widths the card asks for, with a forced
// color profile (Dark: true, NoColor: false, IsTTY: true) so the test is
// deterministic regardless of the environment it runs in.
func TestCardGoldenWidth80(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	checkGolden(t, "card_width80.golden", r.RenderCard(demoCard(), 80))
}

func TestCardGoldenWidth120(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	checkGolden(t, "card_width120.golden", r.RenderCard(demoCard(), 120))
}

// TestCardGoldenLightBackground pins the light-background half of the
// adaptive color pairs, so a regression collapsing Dark:true/false to the
// same colors is caught.
func TestCardGoldenLightBackground(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: false})
	checkGolden(t, "card_width80_light.golden", r.RenderCard(demoCard(), 80))
}
