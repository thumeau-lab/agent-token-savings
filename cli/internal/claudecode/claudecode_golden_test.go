package claudecode

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// repoRoot resolves the repo root (two levels above cli/) from this test
// file's own path, so the golden test works regardless of the invocation cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// this file: <root>/cli/internal/claudecode/claudecode_golden_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// pythonNumber extracts the first integer-looking number (commas stripped)
// following `label` on its own line in s.
func pythonNumber(t *testing.T, s, label string) int64 {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(label) + `\s*:\s*([0-9,]+)`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("label %q not found in python output:\n%s", label, s)
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64)
	if err != nil {
		t.Fatalf("parsing number for %q: %v", label, err)
	}
	return n
}

func pythonRatio(t *testing.T, s, label string) (hit, total int64) {
	t.Helper()
	re := regexp.MustCompile(regexp.QuoteMeta(label) + `\s*:\s*([0-9]+)/([0-9]+)`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("ratio label %q not found in python output:\n%s", label, s)
	}
	h, _ := strconv.ParseInt(m[1], 10, 64)
	tt, _ := strconv.ParseInt(m[2], 10, 64)
	return h, tt
}

// writeSession writes one Claude Code session jsonl file under root/projects/<proj>/<name>.jsonl.
// Every line carries a "timestamp" of the real current moment: callers
// that do not care about the days window pass a wide enough window
// (e.g. Aggregate(root, 36600, time.Now())) so these fixtures never
// depend on which actual calendar day the test happens to run.
func writeSession(t *testing.T, root, proj, name string, usages [][4]int64) {
	t.Helper()
	dir := filepath.Join(root, "projects", proj)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	var sb strings.Builder
	for _, u := range usages {
		sb.WriteString(`{"timestamp":"`)
		sb.WriteString(ts)
		sb.WriteString(`","message":{"usage":{"input_tokens":`)
		sb.WriteString(strconv.FormatInt(u[0], 10))
		sb.WriteString(`,"output_tokens":`)
		sb.WriteString(strconv.FormatInt(u[1], 10))
		sb.WriteString(`,"cache_read_input_tokens":`)
		sb.WriteString(strconv.FormatInt(u[2], 10))
		sb.WriteString(`,"cache_creation_input_tokens":`)
		sb.WriteString(strconv.FormatInt(u[3], 10))
		sb.WriteString("}}}\n")
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAggregateMatchesPythonOnFixture is the golden test required by the
// card: tools/claude_cache_ratio.py and the Go port must report the same
// numbers for the same fixture input.
func TestAggregateMatchesPythonOnFixture(t *testing.T) {
	root := t.TempDir()
	// session 1: cold start, one small write, then a read -> healthy.
	writeSession(t, root, "proj-a", "one.jsonl", [][4]int64{
		{1, 5, 0, 30000},
		{1, 5, 30000, 900},
	})
	// session 2: warm, then a massive rebuild with read=0 -> the anomaly
	// tools/claude_cache_ratio.py calls a "resume" signature.
	writeSession(t, root, "proj-b", "two.jsonl", [][4]int64{
		{1, 5, 0, 30000},
		{1, 5, 30000, 900},
		{2, 6, 0, 50000},
	})

	rep, err := Aggregate(root, 36600, time.Now())
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}

	pyScript := filepath.Join(repoRoot(t), "tools", "claude_cache_ratio.py")
	out, err := exec.Command("python3", pyScript, root).CombinedOutput()
	if err != nil {
		t.Fatalf("python3 %s %s: %v\n%s", pyScript, root, err, out)
	}
	pyOut := string(out)

	if got, want := pythonNumber(t, pyOut, "sessions with usage"), int64(rep.Sessions); got != want {
		t.Errorf("sessions: python=%d go=%d", got, want)
	}
	if got, want := pythonNumber(t, pyOut, "cache_creation total"), rep.Totals.Create; got != want {
		t.Errorf("cache_creation total: python=%d go=%d", got, want)
	}
	if got, want := pythonNumber(t, pyOut, "cache_read total"), rep.Totals.Read; got != want {
		t.Errorf("cache_read total: python=%d go=%d", got, want)
	}
	if got, want := pythonNumber(t, pyOut, "uncached input"), rep.Totals.Input; got != want {
		t.Errorf("uncached input: python=%d go=%d", got, want)
	}
	if got, want := pythonNumber(t, pyOut, "output"), rep.Totals.Output; got != want {
		t.Errorf("output: python=%d go=%d", got, want)
	}
	pyWarm, pyWarmTotal := pythonRatio(t, pyOut, "warm rebuilds (abnormal)")
	if pyWarm != int64(len(rep.WarmRebuilds)) || pyWarmTotal != int64(rep.Sessions) {
		t.Errorf("warm rebuilds: python=%d/%d go=%d/%d", pyWarm, pyWarmTotal, len(rep.WarmRebuilds), rep.Sessions)
	}
	pyCold, _ := pythonRatio(t, pyOut, "cold first call (normal)")
	if pyCold != int64(rep.FirstCold) {
		t.Errorf("first cold: python=%d go=%d", pyCold, rep.FirstCold)
	}
}
