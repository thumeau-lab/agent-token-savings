// Package fixer finds settings that need adjustment (per docs/levers.md) and
// applies them safely: dry run by default, a timestamped .bak before any
// write, a re-read after, and a refusal if the file changed since it was
// read. ats never proposes disabling prompt caching.
package fixer

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// HermesCacheTTLKey is the one YAML key ats currently knows how to fix,
// verified against docs/levers.md section 3 ("prompt_caching.cache_ttl").
// ats has no YAML library (stdlib + Charm only): it edits this single,
// well-known line by exact text match instead of parsing/re-emitting YAML,
// so it never touches anything else in the file.
const HermesCacheTTLKey = "prompt_caching.cache_ttl"

// RecommendedCacheTTL is the only value ats ever proposes for this key: it
// adapts automatically (1h for human sessions, 5m for machine ones) and is
// never a way to disable caching. See tools/ttl_auto_estimate.py.
const RecommendedCacheTTL = "auto"

// Finding is one proposed change: a file, the key found at OldValue, and the
// NewValue ats recommends. snapshot/oldLine/newLine carry what ApplyFix
// needs to safely locate and replace the exact text later.
type Finding struct {
	Agent       string
	File        string
	Key         string
	OldValue    string
	NewValue    string
	Description string

	snapshot []byte
	oldLine  string
	newLine  string
}

var cacheTTLLineRE = regexp.MustCompile(`(?m)^(\s*cache_ttl:\s*)([^\s#]+)(.*)$`)

// readCacheTTLLine reads a Hermes config.yaml and finds the literal
// "cache_ttl:" line, if any. m is the regexp submatch (nil if the file has
// no such line): m[1] is the leading "  cache_ttl:" plus whitespace, m[2]
// is the current value, m[3] is any trailing comment/whitespace. Both
// DetectHermesCacheTTLFinding and CurrentCacheTTL read through this single
// function so there is exactly one parser for the key.
func readCacheTTLLine(configPath string) (raw []byte, m []string, err error) {
	raw, err = os.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	m = cacheTTLLineRE.FindStringSubmatch(string(raw))
	return raw, m, nil
}

// CurrentCacheTTL reads the current prompt_caching.cache_ttl value from a
// Hermes config.yaml, regardless of whether it already matches
// RecommendedCacheTTL. found is false when the file has no cache_ttl line
// (ats never invents structure it cannot see).
func CurrentCacheTTL(configPath string) (value string, found bool, err error) {
	_, m, err := readCacheTTLLine(configPath)
	if err != nil {
		return "", false, err
	}
	if m == nil {
		return "", false, nil
	}
	return m[2], true, nil
}

// DetectHermesCacheTTLFinding reads a Hermes config.yaml and reports whether
// prompt_caching.cache_ttl is set to something other than "auto". Only looks
// at the literal "cache_ttl:" line; a missing key is left alone (ats never
// invents structure in a file it cannot fully parse).
func DetectHermesCacheTTLFinding(configPath string) (Finding, bool, error) {
	raw, m, err := readCacheTTLLine(configPath)
	if err != nil {
		return Finding{}, false, err
	}
	if m == nil {
		return Finding{}, false, nil
	}
	current := m[2]
	if current == RecommendedCacheTTL {
		return Finding{}, false, nil
	}
	oldLine := m[0]
	newLine := m[1] + RecommendedCacheTTL + m[3]
	return Finding{
		Agent:    "hermes",
		File:     configPath,
		Key:      HermesCacheTTLKey,
		OldValue: current,
		NewValue: RecommendedCacheTTL,
		Description: fmt.Sprintf(
			"%s is %q; %q adapts the TTL to human vs machine sessions (see docs/levers.md, tools/ttl_auto_estimate.py)",
			HermesCacheTTLKey, current, RecommendedCacheTTL),
		snapshot: raw,
		oldLine:  oldLine,
		newLine:  newLine,
	}, true, nil
}

// ApplyFix applies a Finding. With write=false (the default) it only
// reports what it would do. With write=true it backs up the file to
// "<file>.bak-<unix-timestamp>", writes the single changed line, then
// re-reads the file to confirm the new value landed. It refuses if the
// file's content differs from what Finding captured when it was detected.
func ApplyFix(f Finding, write bool) (applied bool, backupPath string, err error) {
	if !write {
		return false, "", nil
	}

	current, err := os.ReadFile(f.File)
	if err != nil {
		return false, "", err
	}
	if sha256.Sum256(current) != sha256.Sum256(f.snapshot) {
		return false, "", fmt.Errorf("refusing to write %s: file changed since it was read", f.File)
	}

	fi, err := os.Stat(f.File)
	if err != nil {
		return false, "", err
	}
	backupPath = fmt.Sprintf("%s.bak-%d", f.File, time.Now().Unix())
	if err := os.WriteFile(backupPath, current, fi.Mode()); err != nil {
		return false, "", err
	}

	newContent := strings.Replace(string(current), f.oldLine, f.newLine, 1)
	if err := os.WriteFile(f.File, []byte(newContent), fi.Mode()); err != nil {
		return false, backupPath, err
	}

	readBack, err := os.ReadFile(f.File)
	if err != nil {
		return false, backupPath, err
	}
	if !strings.Contains(string(readBack), f.newLine) {
		return false, backupPath, fmt.Errorf("wrote %s but re-reading it does not show the new value", f.File)
	}
	return true, backupPath, nil
}
