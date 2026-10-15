package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestTTLJSONReportsClaudeCodeFromSessionGaps verifies `ats ttl claude-code`
// is wired to claudecode.EstimateTTL (session call-gap method, same
// threshold/ratio as tools/ttl_auto_estimate.py) instead of the old
// blanket "not applicable" skip.
func TestTTLJSONReportsClaudeCodeFromSessionGaps(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-10-11T10:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000}}}
{"timestamp":"2026-10-11T10:01:00Z","message":{"usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":1000,"cache_creation_input_tokens":2000}}}
`
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"ttl", "--json", "claude-code"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent              string `json:"agent"`
			Skipped            bool   `json:"skipped"`
			MachineSessions    int64  `json:"machine_sessions,omitempty"`
			MachineWriteTokens int64  `json:"machine_write_tokens,omitempty"`
			Recommendation     string `json:"recommendation,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Skipped {
		t.Fatalf("claude-code ttl = %+v, want one non-skipped entry", payload.Agents)
	}
	a := payload.Agents[0]
	if a.MachineSessions != 1 || a.MachineWriteTokens != 3000 {
		t.Fatalf("claude-code ttl = %+v, want sessions=1 write_tokens=3000", a)
	}
	if a.Recommendation == "" {
		t.Fatalf("claude-code ttl recommendation is empty")
	}
}
