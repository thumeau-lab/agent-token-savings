package cliapp

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func writeHermesFixture(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE sessions (id TEXT, source TEXT, started_at REAL, cache_write_tokens INT)`,
		`CREATE TABLE messages (session_id TEXT, timestamp REAL)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	now := float64(time.Now().Unix())
	if _, err := db.Exec(`INSERT INTO sessions VALUES (?,?,?,?)`, "m1", "cron", now, 100_000); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages VALUES (?,?)`, "m1", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages VALUES (?,?)`, "m1", now+60); err != nil {
		t.Fatal(err)
	}
}

type ttlHermesJSON struct {
	Agent          string  `json:"agent"`
	Skipped        bool    `json:"skipped"`
	CurrentValue   string  `json:"current_value,omitempty"`
	Recommendation string  `json:"recommendation,omitempty"`
	Headline       string  `json:"headline,omitempty"`
	EstimatedSaving float64 `json:"estimated_saving_ratio,omitempty"`
}

func runTTLJSONForHermes(t *testing.T, home string) ttlHermesJSON {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run([]string{"ttl", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []ttlHermesJSON `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	for _, a := range payload.Agents {
		if a.Agent == "hermes" {
			return a
		}
	}
	t.Fatalf("no hermes entry in %+v", payload.Agents)
	return ttlHermesJSON{}
}

// Regression for: ats ttl recommended switching to auto TTL even when
// ~/.hermes/config.yaml already has prompt_caching.cache_ttl: auto, while
// `ats fix` correctly said there was nothing to fix (ttl.go recommended
// without reading the current value). When already on auto, the headline
// must state that and must not tell the user to run `ats fix`.
func TestTTLJSONAlreadyAutoHasNoCallToAction(t *testing.T) {
	home := t.TempDir()
	writeHermesFixture(t, home)
	writeHermesConfig(t, home, "prompt_caching:\n  cache_ttl: auto\n")

	hermes := runTTLJSONForHermes(t, home)

	if hermes.CurrentValue != "auto" {
		t.Fatalf("current_value = %q, want %q", hermes.CurrentValue, "auto")
	}
	if !strings.Contains(hermes.Headline, "already") && !strings.Contains(hermes.Headline, "auto TTL is on") {
		t.Fatalf("headline = %q, want it to state auto TTL is already on", hermes.Headline)
	}
	if strings.Contains(hermes.Headline, "ats fix") || strings.Contains(hermes.Recommendation, "ats fix") {
		t.Fatalf("headline/recommendation still call `ats fix` to action when already auto: headline=%q recommendation=%q",
			hermes.Headline, hermes.Recommendation)
	}
}

// Regression companion: when cache_ttl is NOT auto, ats ttl must still
// recommend the fix (current behavior) and report the current value.
func TestTTLJSONRecommendsFixWhenCacheTTLNotAuto(t *testing.T) {
	home := t.TempDir()
	writeHermesFixture(t, home)
	writeHermesConfig(t, home, "prompt_caching:\n  cache_ttl: 1h\n")

	hermes := runTTLJSONForHermes(t, home)

	if hermes.CurrentValue != "1h" {
		t.Fatalf("current_value = %q, want %q", hermes.CurrentValue, "1h")
	}
	if !strings.Contains(hermes.Recommendation, "ats fix") {
		t.Fatalf("recommendation = %q, want it to mention `ats fix`", hermes.Recommendation)
	}
}

func TestTTLJSONReportsHermesAndSkipsOtherAgents(t *testing.T) {
	home := t.TempDir()
	writeHermesFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"ttl", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent              string `json:"agent"`
			Skipped            bool   `json:"skipped"`
			MachineSessions    int64  `json:"machine_sessions,omitempty"`
			MachineWriteTokens int64  `json:"machine_write_tokens,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	var hermes *struct {
		Agent              string `json:"agent"`
		Skipped            bool   `json:"skipped"`
		MachineSessions    int64  `json:"machine_sessions,omitempty"`
		MachineWriteTokens int64  `json:"machine_write_tokens,omitempty"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "hermes" {
			hermes = &payload.Agents[i]
		}
	}
	if hermes == nil {
		t.Fatalf("no hermes entry in %+v", payload.Agents)
	}
	if hermes.MachineSessions != 1 || hermes.MachineWriteTokens != 100_000 {
		t.Fatalf("hermes = %+v, want machine_sessions=1 machine_write_tokens=100000", hermes)
	}
}
