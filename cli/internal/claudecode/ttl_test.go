package claudecode

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTTLSession writes a session file with one usage record per
// (timestamp, create) pair, used by the TTL gap-estimate tests below.
// Timestamps are passed as absolute time.Time so tests can express gaps
// precisely and stay independent of wall-clock drift for the
// days-window tests.
func writeTTLSession(t *testing.T, root, proj, name string, calls []struct {
	TS     time.Time
	Create int64
}) {
	t.Helper()
	dir := filepath.Join(root, "projects", proj)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, c := range calls {
		content += `{"timestamp":"` + c.TS.Format(time.RFC3339) + `","message":{"usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":` +
			itoa(c.Create) + `}}}` + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func call(ts time.Time, create int64) struct {
	TS     time.Time
	Create int64
} {
	return struct {
		TS     time.Time
		Create int64
	}{ts, create}
}

// TestEstimateTTLBucketsCallsByGapNotBySessionMaxGap is the regression
// test for the self-contradictory ttl claude-code headline bug: Claude
// Code sessions are fully interactive, so there is no machine/human
// session split (ttl_auto_estimate.py's method does not transfer, see
// the package doc comment); EstimateTTL must bucket every call by the
// gap preceding it -- first call, <=5min (hit under either TTL), (5min,
// 1h] (hit only under 1h), >1h (miss under either TTL) -- not by one
// max-gap-per-session flag.
func TestEstimateTTLBucketsCallsByGapNotBySessionMaxGap(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeTTLSession(t, root, "p", "mixed.jsonl", []struct {
		TS     time.Time
		Create int64
	}{
		call(now.Add(-2*time.Hour), 1000),              // first call: always a write
		call(now.Add(-2*time.Hour+1*time.Minute), 2000), // gap 1min <=5min: short
		call(now.Add(-2*time.Hour+21*time.Minute), 3000), // gap 20min, in (5m,1h]: mid
		call(now.Add(-30*time.Minute), 4000),            // gap ~51min... wait keep simple below
	})

	res, err := EstimateTTL(root, 30, now)
	if err != nil {
		t.Fatalf("EstimateTTL: %v", err)
	}
	if res.Sessions != 1 {
		t.Fatalf("Sessions = %d, want 1", res.Sessions)
	}
	if res.Calls != 4 {
		t.Fatalf("Calls = %d, want 4", res.Calls)
	}
	if res.FirstCallWriteTokens != 1000 {
		t.Fatalf("FirstCallWriteTokens = %d, want 1000", res.FirstCallWriteTokens)
	}
	if res.ShortGapWriteTokens != 2000 || res.ShortGapCalls != 1 {
		t.Fatalf("ShortGap = tokens=%d calls=%d, want tokens=2000 calls=1", res.ShortGapWriteTokens, res.ShortGapCalls)
	}
	if res.MidGapWriteTokens != 3000 || res.MidGapCalls != 1 {
		t.Fatalf("MidGap = tokens=%d calls=%d, want tokens=3000 calls=1", res.MidGapWriteTokens, res.MidGapCalls)
	}
	if res.LongGapWriteTokens != 4000 || res.LongGapCalls != 1 {
		t.Fatalf("LongGap = tokens=%d calls=%d, want tokens=4000 calls=1", res.LongGapWriteTokens, res.LongGapCalls)
	}
}

// TestEstimateTTLSavingRatioPositiveWhenMidGapDominates verifies the
// direct 5m-vs-1h cost comparison: when most of the write volume sits in
// the (5min,1h] "avoidable" bucket, switching to a 1h TTL is cheaper
// (SavingRatio > 0) even though every pricier-per-token 1h write still
// costs more per token -- fewer total writes wins.
func TestEstimateTTLSavingRatioPositiveWhenMidGapDominates(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeTTLSession(t, root, "p", "midgap.jsonl", []struct {
		TS     time.Time
		Create int64
	}{
		call(now.Add(-40*time.Minute), 1000), // first call
		call(now.Add(-20*time.Minute), 9000),  // gap 20min: mid-gap, avoidable under 1h
	})

	res, err := EstimateTTL(root, 30, now)
	if err != nil {
		t.Fatalf("EstimateTTL: %v", err)
	}
	// Cost5m = (1000+9000)*1.25 = 12500; Cost1h = (1000+0)*2.0 = 2000.
	if got, want := res.Cost5m(), 12500.0; got != want {
		t.Fatalf("Cost5m() = %v, want %v", got, want)
	}
	if got, want := res.Cost1h(), 2000.0; got != want {
		t.Fatalf("Cost1h() = %v, want %v", got, want)
	}
	if res.SavingRatio() <= 0 {
		t.Fatalf("SavingRatio() = %v, want > 0 (1h TTL is cheaper here)", res.SavingRatio())
	}
}

// TestEstimateTTLSavingRatioNonPositiveWhenLongGapDominates verifies the
// other direction: when the write volume sits in the >1h "unavoidable"
// bucket, a write happens under either TTL, so paying the pricier 1h
// multiplier on it is pure loss -- SavingRatio must not be positive.
func TestEstimateTTLSavingRatioNonPositiveWhenLongGapDominates(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeTTLSession(t, root, "p", "longgap.jsonl", []struct {
		TS     time.Time
		Create int64
	}{
		call(now.Add(-3*time.Hour), 1000), // first call
		call(now.Add(-1*time.Hour), 9000),  // gap 2h: long-gap, unavoidable either way
	})

	res, err := EstimateTTL(root, 30, now)
	if err != nil {
		t.Fatalf("EstimateTTL: %v", err)
	}
	// Cost5m = (1000+9000)*1.25 = 12500; Cost1h = (1000+9000)*2.0 = 20000: more expensive.
	if res.SavingRatio() > 0 {
		t.Fatalf("SavingRatio() = %v, want <= 0 (1h TTL is NOT cheaper when gaps are already >1h)", res.SavingRatio())
	}
}

// TestEstimateTTLFiltersSessionsOutsideDaysWindow verifies the look-back
// window (ats ttl --days N, default 7) is actually applied to Claude
// Code's per-call gap method, not silently ignored: a session whose last
// activity is well outside the window must not count.
func TestEstimateTTLFiltersSessionsOutsideDaysWindow(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeTTLSession(t, root, "p", "recent.jsonl", []struct {
		TS     time.Time
		Create int64
	}{call(now.Add(-1*time.Hour), 1000)})
	writeTTLSession(t, root, "p", "old.jsonl", []struct {
		TS     time.Time
		Create int64
	}{call(now.AddDate(0, 0, -100), 5000)})

	res, err := EstimateTTL(root, 7, now)
	if err != nil {
		t.Fatalf("EstimateTTL: %v", err)
	}
	if res.Sessions != 1 {
		t.Fatalf("Sessions = %d, want 1 (old.jsonl is 100 days old, outside a 7-day window)", res.Sessions)
	}
	if res.FirstCallWriteTokens != 1000 {
		t.Fatalf("FirstCallWriteTokens = %d, want 1000 (only recent.jsonl counted)", res.FirstCallWriteTokens)
	}
}
