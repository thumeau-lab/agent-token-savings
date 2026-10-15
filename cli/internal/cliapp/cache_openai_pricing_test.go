package cliapp

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// writeCodexFixturePricedModel writes one rollout file with a
// turn_context line setting the model to "gpt-5-codex" (a model present
// in openAIPriceUSDPerMTok) before a single token_count event, so the
// window's whole usage is attributable to one priced model.
func writeCodexFixturePricedModel(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "15")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	turn := `{"type":"turn_context","payload":{"model":"gpt-5-codex"}}`
	usage := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10000,"cached_input_tokens":2000,"output_tokens":500,"cache_write_input_tokens":0,"reasoning_output_tokens":0,"total_tokens":10500}}}}`
	content := turn + "\n" + usage + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-priced.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCacheJSONCodexPricedModelComputesUSD is the acceptance test for
// card point 2: when the exact model active for the window
// ("gpt-5-codex") is in openAIPriceUSDPerMTok, `ats cache` must compute
// usd/usd_share instead of leaving CostUnknown set. Figures below are
// hand-computed from openAIPriceUSDPerMTok["gpt-5-codex"] (input
// $1.25/MTok, cached input $0.125/MTok, output $10.00/MTok, no
// cache-write charge):
//
//	input:      (10000-2000) tok * 1.25 / 1e6 = 0.01
//	cache_read: 2000 tok         * 0.125 / 1e6 = 0.00025
//	output:     500 tok          * 10.00 / 1e6 = 0.005
//	cache_write: 0 (OpenAI charges nothing for cache writes)
func TestCacheJSONCodexPricedModelComputesUSD(t *testing.T) {
	home := t.TempDir()
	writeCodexFixturePricedModel(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent       string             `json:"agent"`
			CostUnknown bool               `json:"cost_unknown"`
			USD         map[string]float64 `json:"usd"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (codex only)", payload.Agents)
	}
	codex := payload.Agents[0]
	if codex.CostUnknown {
		t.Fatalf("codex.CostUnknown = true, want false (gpt-5-codex has a pricing table entry)")
	}
	const eps = 1e-9
	want := map[string]float64{"input": 0.01, "cache_read": 0.00025, "output": 0.005, "cache_write": 0}
	for k, v := range want {
		if got := codex.USD[k]; math.Abs(got-v) > eps {
			t.Fatalf("usd[%q] = %v, want %v (usd=%+v)", k, got, v, codex.USD)
		}
	}
}

// writeCopilotFixtureSingleOpenAIModel writes a session-store.db with
// every event on a single OpenAI model ("gpt-5"), unlike
// writeCopilotFixture's deliberate model switch: this is the "whole
// window priced" case for card point 3.
func writeCopilotFixtureSingleOpenAIModel(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".copilot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "session-store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE assistant_usage_events (
		session_id TEXT, model TEXT, input_tokens INTEGER, output_tokens INTEGER,
		cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER, created_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO assistant_usage_events
		(session_id, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"s1", "gpt-5", 1000, 50, 200, 0, 10, "2026-10-14T11:00:00.000Z"); err != nil {
		t.Fatal(err)
	}
}

// TestCacheJSONCopilotPricedModelHasUSDNote is the acceptance test for
// card point 3: when every token in the window belongs to a priced
// OpenAI model, Copilot gets a usd/usd_share figure plus a USDNote
// explaining (like Claude Code's note.claude_usd) that it is an
// API-equivalent cost at list price, not a real per-token charge --
// Copilot itself is billed by subscription, not per token. Figures are
// hand-computed from openAIPriceUSDPerMTok["gpt-5"] (input $1.25/MTok,
// cached input $0.125/MTok, output $10.00/MTok):
//
//	input:      (1000-200) tok * 1.25 / 1e6 = 0.001
//	cache_read: 200 tok        * 0.125 / 1e6 = 0.000025
//	output:     50 tok         * 10.00 / 1e6 = 0.0005
func TestCacheJSONCopilotPricedModelHasUSDNote(t *testing.T) {
	home := t.TempDir()
	writeCopilotFixtureSingleOpenAIModel(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "copilot", "--json", "--days", "30"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent       string             `json:"agent"`
			CostUnknown bool               `json:"cost_unknown"`
			USD         map[string]float64 `json:"usd"`
			USDNote     string             `json:"usd_note"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (copilot only)", payload.Agents)
	}
	copilot := payload.Agents[0]
	if copilot.CostUnknown {
		t.Fatalf("copilot.CostUnknown = true, want false (gpt-5 has a pricing table entry)")
	}
	if copilot.USDNote == "" {
		t.Fatalf("copilot.USDNote is empty, want a note that this is not billed per token on a subscription")
	}
	if !strings.Contains(strings.ToLower(copilot.USDNote), "subscription") {
		t.Fatalf("usd_note = %q, want it to mention a subscription plan is not billed per token", copilot.USDNote)
	}
	const eps = 1e-9
	want := map[string]float64{"input": 0.001, "cache_read": 0.000025, "output": 0.0005, "cache_write": 0}
	for k, v := range want {
		if got := copilot.USD[k]; math.Abs(got-v) > eps {
			t.Fatalf("usd[%q] = %v, want %v (usd=%+v)", k, got, v, copilot.USD)
		}
	}
}

// writeCodexFixtureOneModelWithCacheWrite writes one rollout file with a
// single turn_context (model) and one token_count event that includes a
// non-zero cache_write_input_tokens, isolating the cache-write multiplier
// case from the rest of the usd computation.
func writeCodexFixtureOneModelWithCacheWrite(t *testing.T, home, model string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "16")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	turn := `{"type":"turn_context","payload":{"model":"` + model + `"}}`
	usage := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10000,"cached_input_tokens":2000,"output_tokens":500,"cache_write_input_tokens":1000,"reasoning_output_tokens":0,"total_tokens":10500}}}}`
	content := turn + "\n" + usage + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-cachewrite.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCacheJSONCodexCacheWriteMultiplierAppliesWhenPageStatesIt is the
// acceptance test for card point 2, the stated case: gpt-5.6-luna's price
// page says "Cache writes are billed at 1.25x the uncached input token
// rate", so 1000 cache-write tokens must be billed at Input (0.20 $/MTok)
// times 1.25, not left at 0.
//
//	input:       (10000-2000) tok * 0.20 / 1e6 = 0.0016
//	cache_read:  2000 tok          * 0.02 / 1e6 = 0.00004
//	output:      500 tok           * 1.20 / 1e6 = 0.0006
//	cache_write: 1000 tok * (0.20 * 1.25) / 1e6  = 0.00025
func TestCacheJSONCodexCacheWriteMultiplierAppliesWhenPageStatesIt(t *testing.T) {
	home := t.TempDir()
	writeCodexFixtureOneModelWithCacheWrite(t, home, "gpt-5.6-luna")

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []struct {
			Agent       string             `json:"agent"`
			CostUnknown bool               `json:"cost_unknown"`
			USD         map[string]float64 `json:"usd"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (codex only)", payload.Agents)
	}
	codex := payload.Agents[0]
	if codex.CostUnknown {
		t.Fatalf("codex.CostUnknown = true, want false (gpt-5.6-luna has a pricing table entry)")
	}
	const eps = 1e-9
	want := map[string]float64{"input": 0.0016, "cache_read": 0.00004, "output": 0.0006, "cache_write": 0.00025}
	for k, v := range want {
		if got := codex.USD[k]; math.Abs(got-v) > eps {
			t.Fatalf("usd[%q] = %v, want %v (usd=%+v)", k, got, v, codex.USD)
		}
	}
}

// TestCacheJSONCodexCacheWriteMultiplierZeroWhenPageIsSilent is the
// acceptance test for card point 2, the unstated case: gpt-5.1-codex-max's
// price page says nothing about cache writes, so the same 1000 cache-write
// tokens must be billed at 0, not guessed at some multiplier.
//
//	input:       (10000-2000) tok * 1.25 / 1e6 = 0.01
//	cache_read:  2000 tok          * 0.125 / 1e6 = 0.00025
//	output:      500 tok           * 10.00 / 1e6 = 0.005
//	cache_write: 0 (page states nothing about cache writes)
func TestCacheJSONCodexCacheWriteMultiplierZeroWhenPageIsSilent(t *testing.T) {
	home := t.TempDir()
	writeCodexFixtureOneModelWithCacheWrite(t, home, "gpt-5.1-codex-max")

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []struct {
			Agent       string             `json:"agent"`
			CostUnknown bool               `json:"cost_unknown"`
			USD         map[string]float64 `json:"usd"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (codex only)", payload.Agents)
	}
	codex := payload.Agents[0]
	if codex.CostUnknown {
		t.Fatalf("codex.CostUnknown = true, want false (gpt-5.1-codex-max has a pricing table entry)")
	}
	const eps = 1e-9
	want := map[string]float64{"input": 0.01, "cache_read": 0.00025, "output": 0.005, "cache_write": 0}
	for k, v := range want {
		if got := codex.USD[k]; math.Abs(got-v) > eps {
			t.Fatalf("usd[%q] = %v, want %v (usd=%+v)", k, got, v, codex.USD)
		}
	}
}

// writeCodexFixtureModelMix writes one rollout file per (model, usage)
// pair, each with its own turn_context switch, the card's acceptance
// fixture for point 4 (a codex rollout mixing several models).
type codexFixtureTurn struct {
	model                                  string
	input, cachedInput, cacheWrite, output int64
}

func writeCodexFixtureModelMix(t *testing.T, home string, turns []codexFixtureTurn) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "16")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, tn := range turns {
		turn := `{"type":"turn_context","payload":{"model":"` + tn.model + `"}}`
		usage := fmt.Sprintf(`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"cache_write_input_tokens":%d,"reasoning_output_tokens":0,"total_tokens":%d}}}}`,
			tn.input, tn.cachedInput, tn.output, tn.cacheWrite, tn.input+tn.output)
		content := turn + "\n" + usage + "\n"
		name := filepath.Join(dir, fmt.Sprintf("rollout-mix-%c.jsonl", rune('a'+i)))
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCacheJSONCodexMixedModelsComputesUSD is the acceptance test for card
// point 4: a codex rollout mixing gpt-5.1-codex-max and gpt-5.6-luna (one
// with no cache-write multiplier, one with 1.25x) must still sum to the
// hand-computed total.
//
//	gpt-5.1-codex-max: input  (1000-200) tok * 1.25 / 1e6 = 0.001
//	                    cache_read 200 tok    * 0.125 / 1e6 = 0.000025
//	                    output 30 tok          * 10.00 / 1e6 = 0.0003
//	                    cache_write 50 tok * (1.25*0) / 1e6 = 0
//	gpt-5.6-luna:       input  (500-100) tok  * 0.20 / 1e6 = 0.00008
//	                    cache_read 100 tok    * 0.02 / 1e6 = 0.000002
//	                    output 15 tok          * 1.20 / 1e6 = 0.000018
//	                    cache_write 20 tok * (0.20*1.25) / 1e6 = 0.000005
//	totals:             input=0.00108 cache_read=0.000027 output=0.000318 cache_write=0.000005
func TestCacheJSONCodexMixedModelsComputesUSD(t *testing.T) {
	home := t.TempDir()
	writeCodexFixtureModelMix(t, home, []codexFixtureTurn{
		{model: "gpt-5.1-codex-max", input: 1000, cachedInput: 200, cacheWrite: 50, output: 30},
		{model: "gpt-5.6-luna", input: 500, cachedInput: 100, cacheWrite: 20, output: 15},
	})

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []struct {
			Agent       string             `json:"agent"`
			CostUnknown bool               `json:"cost_unknown"`
			USD         map[string]float64 `json:"usd"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (codex only)", payload.Agents)
	}
	codex := payload.Agents[0]
	if codex.CostUnknown {
		t.Fatalf("codex.CostUnknown = true, want false (both models have a pricing table entry)")
	}
	const eps = 1e-9
	want := map[string]float64{"input": 0.00108, "cache_read": 0.000027, "output": 0.000318, "cache_write": 0.000005}
	for k, v := range want {
		if got := codex.USD[k]; math.Abs(got-v) > eps {
			t.Fatalf("usd[%q] = %v, want %v (usd=%+v)", k, got, v, codex.USD)
		}
	}
}

// TestCacheJSONCodexUnknownModelAmongKnownKeepsCostUnknown is the
// acceptance test for card point 4's other half: mixing a known model
// (gpt-5.1-codex-max) with a model that has no pricing table entry at all
// must keep CostUnknown true for the whole window -- pricing only part of
// it would silently under-report the real cost.
func TestCacheJSONCodexUnknownModelAmongKnownKeepsCostUnknown(t *testing.T) {
	home := t.TempDir()
	writeCodexFixtureModelMix(t, home, []codexFixtureTurn{
		{model: "gpt-5.1-codex-max", input: 1000, cachedInput: 200, cacheWrite: 50, output: 30},
		{model: "gpt-no-such-model", input: 500, cachedInput: 100, cacheWrite: 20, output: 15},
	})

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []struct {
			Agent       string `json:"agent"`
			CostUnknown bool   `json:"cost_unknown"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (codex only)", payload.Agents)
	}
	if !payload.Agents[0].CostUnknown {
		t.Fatalf("codex.CostUnknown = false, want true (gpt-no-such-model has no pricing table entry)")
	}
}
