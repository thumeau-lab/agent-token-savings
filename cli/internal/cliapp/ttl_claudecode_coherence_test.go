package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeClaudeTTLFixture writes one Claude Code session transcript whose
// usage rows carry the given timestamps/create-token pairs, for the ttl
// coherence tests below.
func writeClaudeTTLFixture(t *testing.T, home, name string, calls []struct {
	TS     time.Time
	Create int64
}) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, c := range calls {
		content += `{"timestamp":"` + c.TS.Format(time.RFC3339) +
			`","message":{"usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":` +
			jsonInt(c.Create) + `}}}` + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsonInt(n int64) string {
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

type claudeTTLPayload struct {
	Agents []struct {
		Agent              string  `json:"agent"`
		Skipped             bool    `json:"skipped"`
		MachineSessions    int64   `json:"machine_sessions,omitempty"`
		GapSessions        int64   `json:"gap_sessions,omitempty"`
		MidGapCalls        int64   `json:"mid_gap_calls,omitempty"`
		LongGapCalls       int64   `json:"long_gap_calls,omitempty"`
		EstimatedSaving    float64 `json:"estimated_saving_ratio,omitempty"`
		Recommendation     string  `json:"recommendation,omitempty"`
		Headline           string  `json:"headline,omitempty"`
		MethodNote         string  `json:"method_note,omitempty"`
	} `json:"agents"`
}

func runClaudeTTL(t *testing.T, home string) claudeTTLPayload {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run([]string{"ttl", "--json", "claude-code"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload claudeTTLPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Skipped {
		t.Fatalf("claude-code ttl = %+v, want one non-skipped entry", payload.Agents)
	}
	return payload
}

// TestTTLClaudeCodeMethodNoteSaysTheMachineSessionMethodDoesNotApply is
// the regression test for "re-derive... what is a machine session for
// Claude Code (likely none: interactive)": the result must say, in
// plain words, that ttl_auto_estimate.py's machine-session split does
// not apply to this agent instead of silently reusing it.
func TestTTLClaudeCodeMethodNoteSaysTheMachineSessionMethodDoesNotApply(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	writeClaudeTTLFixture(t, home, "a.jsonl", []struct {
		TS     time.Time
		Create int64
	}{{now.Add(-1 * time.Hour), 1000}})

	payload := runClaudeTTL(t, home)
	note := payload.Agents[0].MethodNote
	if note == "" {
		t.Fatalf("method_note is empty, want an explicit note that the machine-session method does not apply")
	}
	if !strings.Contains(note, "interactive") {
		t.Fatalf("method_note = %q, want it to explain sessions are interactive (no machine-session split)", note)
	}
}

// TestTTLClaudeCodeHeadlineRecommendsSwitchingWhenMidGapDominates is the
// core regression test for the self-contradictory headline bug: when
// most of the write volume sits in the (5min,1h] avoidable bucket, the
// headline must claim a positive saving AND recommend switching to a 1h
// TTL -- never both a positive "saves X%" headline and a "no session
// idles" recommendation at the same time (the exact contradiction the
// card reported).
func TestTTLClaudeCodeHeadlineRecommendsSwitchingWhenMidGapDominates(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	writeClaudeTTLFixture(t, home, "a.jsonl", []struct {
		TS     time.Time
		Create int64
	}{
		{now.Add(-40 * time.Minute), 1000},
		{now.Add(-20 * time.Minute), 9000}, // gap 20min: mid-gap, avoidable under 1h
	})

	payload := runClaudeTTL(t, home)
	a := payload.Agents[0]
	if a.EstimatedSaving <= 0 {
		t.Fatalf("estimated_saving_ratio = %v, want > 0 (mid-gap dominates)", a.EstimatedSaving)
	}
	if a.MidGapCalls != 1 {
		t.Fatalf("mid_gap_calls = %d, want 1", a.MidGapCalls)
	}
	for _, text := range []string{a.Headline, a.Recommendation} {
		if strings.Contains(text, "no session goes idle") {
			t.Fatalf("text %q claims no session idles, contradicting mid_gap_calls=%d and a positive saving", text, a.MidGapCalls)
		}
	}
	if !strings.Contains(a.Headline, "1h") {
		t.Fatalf("headline = %q, want it to mention switching to a 1h TTL", a.Headline)
	}
}

// TestTTLClaudeCodeHeadlineRecommendsKeepingDefaultWhenLongGapDominates
// is the other direction: when the write volume sits in the >1h
// unavoidable bucket, switching to 1h costs MORE (pricier multiplier on
// a write that happens regardless), so the headline/recommendation must
// NOT claim a saving and must say to keep the default 5m TTL.
func TestTTLClaudeCodeHeadlineRecommendsKeepingDefaultWhenLongGapDominates(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	writeClaudeTTLFixture(t, home, "a.jsonl", []struct {
		TS     time.Time
		Create int64
	}{
		{now.Add(-3 * time.Hour), 1000},
		{now.Add(-1 * time.Hour), 9000}, // gap 2h: long-gap, unavoidable either way
	})

	payload := runClaudeTTL(t, home)
	a := payload.Agents[0]
	if a.EstimatedSaving > 0 {
		t.Fatalf("estimated_saving_ratio = %v, want <= 0 (long-gap dominates, 1h is not cheaper)", a.EstimatedSaving)
	}
	if a.LongGapCalls != 1 {
		t.Fatalf("long_gap_calls = %d, want 1", a.LongGapCalls)
	}
	if !strings.Contains(strings.ToLower(a.Headline+a.Recommendation), "default") &&
		!strings.Contains(strings.ToLower(a.Headline+a.Recommendation), "5m") {
		t.Fatalf("headline/recommendation = %q / %q, want a mention of keeping the default 5m TTL", a.Headline, a.Recommendation)
	}
}
