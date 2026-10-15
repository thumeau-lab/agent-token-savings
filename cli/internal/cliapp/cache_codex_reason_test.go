package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCacheJSONCodexSkipReasonCitesUpstreamInstability verifies that when a
// ~/.codex home directory is present (so codex is "detected"), `ats cache`
// explains exactly why it still skips it instead of a generic "not parsed
// yet" placeholder: ~/.codex/sessions token_count events are not reliably
// emitted for interactive sessions upstream (openai/codex#9660), so ats has
// no stable schema to parse yet.
func TestCacheJSONCodexSkipReasonCitesUpstreamInstability(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent   string `json:"agent"`
			Skipped bool   `json:"skipped"`
			Reason  string `json:"reason,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}

	var codex *struct {
		Agent   string `json:"agent"`
		Skipped bool   `json:"skipped"`
		Reason  string `json:"reason,omitempty"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "codex" {
			codex = &payload.Agents[i]
		}
	}
	if codex == nil {
		t.Fatalf("no codex entry in %+v", payload.Agents)
	}
	if !codex.Skipped {
		t.Fatalf("codex.Skipped = false, want true (no stable token_count schema to parse yet)")
	}
	want := "openai/codex#9660"
	if !bytes.Contains([]byte(codex.Reason), []byte(want)) {
		t.Fatalf("codex.Reason = %q, want it to cite %q (exactly why, not a vague placeholder)", codex.Reason, want)
	}
}
