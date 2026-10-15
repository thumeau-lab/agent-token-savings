package fixer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDetectHermesCacheTTLFindingWhenNotAuto(t *testing.T) {
	path := writeYAML(t, "prompt_caching:\n  cache_ttl: 1h\n  other: x\n")

	f, ok, err := DetectHermesCacheTTLFinding(path)
	if err != nil {
		t.Fatalf("DetectHermesCacheTTLFinding: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true (cache_ttl is 1h, not auto)")
	}
	if f.OldValue != "1h" || f.NewValue != "auto" {
		t.Fatalf("finding = %+v, want old=1h new=auto", f)
	}
	if f.Key != "prompt_caching.cache_ttl" {
		t.Fatalf("Key = %q, want prompt_caching.cache_ttl", f.Key)
	}
}

func TestDetectHermesCacheTTLFindingNoneWhenAlreadyAuto(t *testing.T) {
	path := writeYAML(t, "prompt_caching:\n  cache_ttl: auto\n")

	_, ok, err := DetectHermesCacheTTLFinding(path)
	if err != nil {
		t.Fatalf("DetectHermesCacheTTLFinding: %v", err)
	}
	if ok {
		t.Fatal("ok = true, want false: cache_ttl is already auto, nothing to fix")
	}
}

func TestApplyFixDryRunNeverTouchesTheFile(t *testing.T) {
	path := writeYAML(t, "prompt_caching:\n  cache_ttl: 1h\n")
	before, _ := os.ReadFile(path)

	f, ok, err := DetectHermesCacheTTLFinding(path)
	if err != nil || !ok {
		t.Fatalf("DetectHermesCacheTTLFinding: ok=%v err=%v", ok, err)
	}
	applied, backup, err := ApplyFix(f, false)
	if err != nil {
		t.Fatalf("ApplyFix(dry run): %v", err)
	}
	if applied {
		t.Fatal("applied = true on a dry run, want false")
	}
	if backup != "" {
		t.Fatalf("backup = %q on a dry run, want empty", backup)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatalf("dry run modified the file: before=%q after=%q", before, after)
	}
}

func TestApplyFixWriteBacksUpAndUpdatesTheKey(t *testing.T) {
	path := writeYAML(t, "prompt_caching:\n  cache_ttl: 1h\n  other: x\n")
	f, ok, err := DetectHermesCacheTTLFinding(path)
	if err != nil || !ok {
		t.Fatalf("DetectHermesCacheTTLFinding: ok=%v err=%v", ok, err)
	}

	applied, backup, err := ApplyFix(f, true)
	if err != nil {
		t.Fatalf("ApplyFix(write): %v", err)
	}
	if !applied {
		t.Fatal("applied = false, want true")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup file %s missing: %v", backup, err)
	}
	backupContent, _ := os.ReadFile(backup)
	if !strings.Contains(string(backupContent), "cache_ttl: 1h") {
		t.Fatalf("backup content lost the old value: %q", backupContent)
	}

	newContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newContent), "cache_ttl: auto") {
		t.Fatalf("config not updated: %q", newContent)
	}
	if !strings.Contains(string(newContent), "other: x") {
		t.Fatalf("unrelated line lost: %q", newContent)
	}
}

func TestApplyFixRefusesWhenFileChangedSinceRead(t *testing.T) {
	path := writeYAML(t, "prompt_caching:\n  cache_ttl: 1h\n")
	f, ok, err := DetectHermesCacheTTLFinding(path)
	if err != nil || !ok {
		t.Fatalf("DetectHermesCacheTTLFinding: ok=%v err=%v", ok, err)
	}
	// Someone else edits the file after ats read it, before ats writes.
	if err := os.WriteFile(path, []byte("prompt_caching:\n  cache_ttl: 1h\n  edited_by_someone_else: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	applied, _, err := ApplyFix(f, true)
	if err == nil {
		t.Fatal("ApplyFix did not refuse a file that changed since it was read")
	}
	if applied {
		t.Fatal("applied = true despite the refusal, want false")
	}
}

func TestRecommendedCacheTTLNeverDisablesCaching(t *testing.T) {
	for _, bad := range []string{"off", "disabled", "false", "none", "0"} {
		if RecommendedCacheTTL == bad {
			t.Fatalf("RecommendedCacheTTL = %q, which would disable caching", bad)
		}
	}
	if RecommendedCacheTTL != "auto" {
		t.Fatalf("RecommendedCacheTTL = %q, want %q", RecommendedCacheTTL, "auto")
	}
}
