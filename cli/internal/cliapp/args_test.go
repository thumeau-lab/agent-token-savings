package cliapp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Regression for the review bug: Go's flag package stops parsing at the
// first non-flag argument, so documented invocations like
// `ats ttl hermes --json` or `ats cache claude-code --days 1` silently
// dropped the flags that come after the agent. Every command must accept
// flags before or after the single positional agent argument.

// TestTTLAcceptsJSONFlagAfterAgent is the `ats ttl hermes --json` case
// from the bug report: --json placed after the agent must still select
// JSON output, not styled text.
func TestTTLAcceptsJSONFlagAfterAgent(t *testing.T) {
	home := t.TempDir()
	writeHermesFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"ttl", "hermes", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON (flag after agent was dropped): %v\noutput: %s", err, out.String())
	}
	if payload.Command != "ttl" {
		t.Fatalf("command = %q, want ttl", payload.Command)
	}
}

// TestCacheAcceptsDaysFlagAfterAgent is the `ats cache claude-code --days 1`
// case from the bug report: --days placed after the agent must override
// the default 7-day window.
func TestCacheAcceptsDaysFlagAfterAgent(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "claude-code", "--days", "1", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Days int `json:"days"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if payload.Days != 1 {
		t.Fatalf("days = %d, want 1 (flag after agent was dropped)", payload.Days)
	}
}

// TestCacheAcceptsFlagsBeforeAgent is the already-working order (flags
// then the positional agent): must keep working once the shared helper
// re-parses tokens after the agent.
func TestCacheAcceptsFlagsBeforeAgent(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--days", "1", "--json", "claude-code"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Days   int `json:"days"`
		Agents []struct {
			Agent string `json:"agent"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if payload.Days != 1 {
		t.Fatalf("days = %d, want 1", payload.Days)
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Agent != "claude-code" {
		t.Fatalf("agents = %+v, want only claude-code", payload.Agents)
	}
}

// TestCacheUnknownFlagAfterAgentErrors verifies an unknown flag placed
// after the agent still errors with usage and exit code 2, instead of
// being silently swallowed as a second positional argument.
func TestCacheUnknownFlagAfterAgentErrors(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "claude-code", "--bogus"}, &out, &errOut, Env{HomeDir: home})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stdout: %s stderr: %s)", code, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "bogus") {
		t.Fatalf("stderr = %q, want it to mention the unknown flag", errOut.String())
	}
}

// TestCacheSecondPositionalAfterAgentErrors verifies a second bare
// positional argument after the agent (not a flag) still errors with
// usage and exit code 2.
func TestCacheSecondPositionalAfterAgentErrors(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "claude-code", "hermes"}, &out, &errOut, Env{HomeDir: home})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stdout: %s stderr: %s)", code, out.String(), errOut.String())
	}
}
