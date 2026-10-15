// Package agents detects which AI coding agents have local, offline-readable
// usage data on this machine (claude-code, codex, gemini-cli, hermes).
package agents

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Kind identifies a supported agent.
type Kind string

const (
	ClaudeCode Kind = "claude-code"
	Codex      Kind = "codex"
	Copilot    Kind = "copilot"
	GeminiCLI  Kind = "gemini-cli"
	Hermes     Kind = "hermes"
)

// All lists every agent ats knows about, in the order `ats` (bare) reports them.
var All = []Kind{ClaudeCode, Codex, Copilot, GeminiCLI, Hermes}

// Roots holds the per-agent config/data directory, all derived from one home
// directory so tests can point everything at a fixture without touching the
// real filesystem or environment.
type Roots struct {
	ClaudeHome  string
	CodexHome   string
	CopilotHome string
	GeminiHome  string
	HermesHome  string
}

// DefaultRoots builds the standard "~/.<agent>" layout under home. Pass ""
// to use the real os.UserHomeDir(). CodexHome honors $CODEX_HOME when set
// (the card: "$CODEX_HOME or ~/.codex/sessions"), matching codex-cli's own
// resolution; every other agent has no such override.
func DefaultRoots(home string) (Roots, error) {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return Roots{}, err
		}
		home = h
	}
	codexHome := filepath.Join(home, ".codex")
	if v := os.Getenv("CODEX_HOME"); v != "" {
		codexHome = v
	}
	return Roots{
		ClaudeHome:  filepath.Join(home, ".claude"),
		CodexHome:   codexHome,
		CopilotHome: filepath.Join(home, ".copilot"),
		GeminiHome:  filepath.Join(home, ".gemini"),
		HermesHome:  filepath.Join(home, ".hermes"),
	}, nil
}

// ClaudeHasData reports whether at least one Claude Code session transcript
// is present under ClaudeHome/projects/*/*.jsonl.
func (r Roots) ClaudeHasData() bool {
	matches, _ := filepath.Glob(filepath.Join(r.ClaudeHome, "projects", "*", "*.jsonl"))
	return len(matches) > 0
}

// HermesHasData reports whether a Hermes state.db is present (root or any
// profile's own db).
func (r Roots) HermesHasData() bool {
	if _, err := os.Stat(filepath.Join(r.HermesHome, "state.db")); err == nil {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(r.HermesHome, "profiles", "*", "state.db"))
	return len(matches) > 0
}

// CodexHasData reports whether a Codex CLI home directory exists. A
// present-but-empty home (no session files at all) still reports true
// here: CodexSessionFilesExist is the check for actual usable data.
func (r Roots) CodexHasData() bool {
	_, err := os.Stat(r.CodexHome)
	return err == nil
}

// CodexSessionFilesExist reports whether at least one rollout session
// file exists under CodexHome (sessions/*/*/*/rollout-*.jsonl, or any
// *.jsonl under archived_sessions). Cheap glob/stat only, no JSON
// parsing -- mirrors ClaudeHasData's detection-only contract so the TUI's
// agent selector stays instant (see cliapp's detectedAgents): the actual
// parse happens later, in codexcli.Aggregate.
func (r Roots) CodexSessionFilesExist() bool {
	matches, _ := filepath.Glob(filepath.Join(r.CodexHome, "sessions", "*", "*", "*", "rollout-*.jsonl"))
	if len(matches) > 0 {
		return true
	}
	found := false
	_ = filepath.WalkDir(filepath.Join(r.CodexHome, "archived_sessions"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // archived_sessions absent: nothing to walk, not an error.
		}
		if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
			found = true
		}
		return nil
	})
	return found
}

// CopilotHasData reports whether a Copilot CLI session-store.db is present.
func (r Roots) CopilotHasData() bool {
	_, err := os.Stat(filepath.Join(r.CopilotHome, "session-store.db"))
	return err == nil
}

// GeminiHasData reports whether a Gemini CLI home directory exists. Same
// caveat as CodexHasData: detection only, no parsing yet.
func (r Roots) GeminiHasData() bool {
	_, err := os.Stat(r.GeminiHome)
	return err == nil
}
