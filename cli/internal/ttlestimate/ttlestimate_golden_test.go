package ttlestimate

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// TestFormatMatchesPythonOnFixture is the golden test required by the card:
// tools/ttl_auto_estimate.py and the Go port must report the same numbers
// (here: the exact same formatted text) for the same fixture state.db.
// ttl_auto_estimate.py hardcodes a 30-day window, so the fixture uses days=30.
func TestFormatMatchesPythonOnFixture(t *testing.T) {
	home := buildFixtureHome(t, []fixtureSession{
		{id: "human1", source: "cli", writeTokens: 999_000, gapSeconds: 1000},
		{id: "m-nogap", source: "cron", writeTokens: 100_000, gapSeconds: 60},
		{id: "m-gap", source: "subagent", writeTokens: 200_000, gapSeconds: 600},
	})

	res, err := Estimate(home, 30)
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}

	pyScript := filepath.Join(repoRoot(t), "tools", "ttl_auto_estimate.py")
	out, err := exec.Command("python3", pyScript, home).CombinedOutput()
	if err != nil {
		t.Fatalf("python3 %s %s: %v\n%s", pyScript, home, err, out)
	}

	// On Windows, Python's stdout is opened in text mode: print()'s "\n"
	// gets translated to "\r\n" on the way out, while Format() (Go) always
	// writes a literal "\n". Normalize that platform-specific translation
	// before comparing so the two formatters are compared on content, not
	// on an artifact of how Python buffers its own output.
	want := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got := res.Format(); got != want {
		t.Errorf("Format() mismatch:\n--- go ---\n%s--- python ---\n%s", got, want)
	}
}
