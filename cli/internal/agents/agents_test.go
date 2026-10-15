package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRootsJoinsDotDirsUnderHome(t *testing.T) {
	r, err := DefaultRoots("/home/x")
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	want := Roots{
		ClaudeHome:  filepath.Join("/home/x", ".claude"),
		CodexHome:   filepath.Join("/home/x", ".codex"),
		CopilotHome: filepath.Join("/home/x", ".copilot"),
		GeminiHome:  filepath.Join("/home/x", ".gemini"),
		HermesHome:  filepath.Join("/home/x", ".hermes"),
	}
	if r != want {
		t.Fatalf("DefaultRoots(/home/x) = %+v, want %+v", r, want)
	}
}

// TestDefaultRootsHonorsCodexHomeEnvOverride verifies $CODEX_HOME takes
// priority over ~/.codex, matching the card's "$CODEX_HOME or
// ~/.codex/sessions" resolution.
func TestDefaultRootsHonorsCodexHomeEnvOverride(t *testing.T) {
	t.Setenv("CODEX_HOME", "/custom/codex-home")
	r, err := DefaultRoots("/home/x")
	if err != nil {
		t.Fatalf("DefaultRoots: %v", err)
	}
	if r.CodexHome != "/custom/codex-home" {
		t.Fatalf("CodexHome = %q, want /custom/codex-home", r.CodexHome)
	}
}

func TestCopilotHasDataFalseWhenNoSessionStoreDB(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	if r.CopilotHasData() {
		t.Fatal("CopilotHasData() = true on an empty home, want false")
	}
}

func TestCopilotHasDataTrueWhenSessionStoreDBExists(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	if err := os.MkdirAll(r.CopilotHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.CopilotHome, "session-store.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.CopilotHasData() {
		t.Fatal("CopilotHasData() = false after writing session-store.db, want true")
	}
}

func TestCodexSessionFilesExistFalseWhenOnlyHomeDirPresent(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	if err := os.MkdirAll(r.CodexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if r.CodexSessionFilesExist() {
		t.Fatal("CodexSessionFilesExist() = true with only an empty home dir, want false")
	}
}

func TestCodexSessionFilesExistTrueWithARolloutFile(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	dir := filepath.Join(r.CodexHome, "sessions", "2026", "10", "14")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-a.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.CodexSessionFilesExist() {
		t.Fatal("CodexSessionFilesExist() = false after writing a rollout file, want true")
	}
}

func TestCodexSessionFilesExistTrueWithAnArchivedFile(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	dir := filepath.Join(r.CodexHome, "archived_sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.CodexSessionFilesExist() {
		t.Fatal("CodexSessionFilesExist() = false after writing an archived session, want true")
	}
}

func TestClaudeHasDataFalseWhenNoTranscripts(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	if r.ClaudeHasData() {
		t.Fatal("ClaudeHasData() = true on an empty home, want false")
	}
}

func TestClaudeHasDataTrueWhenATranscriptExists(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	dir := filepath.Join(r.ClaudeHome, "projects", "p1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.ClaudeHasData() {
		t.Fatal("ClaudeHasData() = false after writing a transcript, want true")
	}
}

func TestHermesHasDataTrueWhenStateDBExists(t *testing.T) {
	home := t.TempDir()
	r, _ := DefaultRoots(home)
	if err := os.MkdirAll(r.HermesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.HermesHome, "state.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !r.HermesHasData() {
		t.Fatal("HermesHasData() = false after writing state.db, want true")
	}
}
