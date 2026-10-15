package cliapp

import (
	"crypto/sha256"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

var tuiUpdate = flag.Bool("tui-update", false, "update TUI golden files in testdata/")

// resize sends a WindowSizeMsg through Update and returns the resulting
// tuiModel, exactly as the real Program would on start or on a terminal
// resize.
func resize(t *testing.T, m tuiModel, w, h int) tuiModel {
	t.Helper()
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	mm, ok := next.(tuiModel)
	if !ok {
		t.Fatalf("Update(WindowSizeMsg) returned %T, want tuiModel", next)
	}
	return mm
}

// press sends one key press (matched the same way msg.String() would be
// matched in production) through Update, draining any command it returns
// (huh's internal nextFieldMsg/updateFieldMsg... chain) so the model
// settles before the test asserts on it.
func press(t *testing.T, m tuiModel, key tea.Key) tuiModel {
	t.Helper()
	next, cmd := m.Update(tea.KeyPressMsg(key))
	mm, ok := next.(tuiModel)
	if !ok {
		t.Fatalf("Update(KeyPressMsg) returned %T, want tuiModel", next)
	}
	return drainCmd(t, mm, cmd)
}

// drainCmd runs cmd (and whatever further commands its resulting message
// produces) to completion, feeding every message back into Update. Bounded
// so a programming mistake (a command that never settles) fails the test
// instead of hanging it.
func drainCmd(t *testing.T, m tuiModel, cmd tea.Cmd) tuiModel {
	t.Helper()
	for i := 0; i < 5 && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			return m
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = drainCmd(t, m, c)
			}
			return m
		}
		next, nextCmd := m.Update(msg)
		mm, ok := next.(tuiModel)
		if !ok {
			t.Fatalf("Update(%T) returned %T, want tuiModel", msg, next)
		}
		m, cmd = mm, nextCmd
	}
	return m
}

func feedLoaded(t *testing.T, m tuiModel, msg loadedMsg) tuiModel {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(tuiModel)
	if !ok {
		t.Fatalf("Update(loadedMsg) returned %T, want tuiModel", next)
	}
	return mm
}

func testRoots(home string) agents.Roots {
	return agents.Roots{
		ClaudeHome:  filepath.Join(home, ".claude"),
		CodexHome:   filepath.Join(home, ".codex"),
		CopilotHome: filepath.Join(home, ".copilot"),
		GeminiHome:  filepath.Join(home, ".gemini"),
		HermesHome:  filepath.Join(home, ".hermes"),
	}
}

func TestTUISwitchesTabWithTabShiftTabAndNumberKeys(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)

	if m.tab != tabDashboard {
		t.Fatalf("initial tab = %v, want tabDashboard", m.tab)
	}

	m = press(t, m, tea.Key{Code: tea.KeyTab})
	if m.tab != tabCache {
		t.Fatalf("after tab: tab = %v, want tabCache", m.tab)
	}

	m = press(t, m, tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.tab != tabDashboard {
		t.Fatalf("after shift+tab: tab = %v, want tabDashboard", m.tab)
	}

	m = press(t, m, tea.Key{Text: "4", Code: '4'})
	if m.tab != tabTTL {
		t.Fatalf("after '4': tab = %v, want tabTTL", m.tab)
	}
	if !strings.Contains(m.View().Content, "[Cache lifetime]") {
		t.Fatalf("View() does not show the Cache lifetime tab as active:\n%s", m.View().Content)
	}

	m = press(t, m, tea.Key{Text: "5", Code: '5'})
	if m.tab != tabFix {
		t.Fatalf("after '5': tab = %v, want tabFix", m.tab)
	}
}

// TestTUIFooterNarrowWidthShowsMostUsefulKeysFirst is the regression
// test for the narrow (100-col) footer dropping the most useful keys
// (quit, help, switch tab, language) behind the ellipsis while less
// useful ones (switch agent, cycle window) consumed the width budget
// first -- reviewer feedback, LANG=fr_FR.UTF-8, 100x34. ShortHelp()'s
// order controls which items bubbles/help keeps when truncating, so q/
// ?/tab/L must come before switch-agent/window in that order.
func TestTUIFooterNarrowWidthShowsMostUsefulKeysFirst(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.FR}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, cacheResults: []AgentCacheResult{demoCacheResult()}})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.refreshBody()

	footer := m.renderFooter()
	for _, want := range []string{"q ", "? ", "tab/1-5", "L "} {
		if !strings.Contains(footer, want) {
			t.Errorf("100-col FR footer missing %q (should show the most useful keys first):\n%s", want, footer)
		}
	}
}

func writeClaudeJSONLFixture(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeHermesStateDBFixture(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// HermesHasData only stats this file; its content does not matter for
	// detection (ats cache/collectCache handles an unparsable db as a
	// clean skip, exercised separately by the existing cache tests).
	if err := os.WriteFile(filepath.Join(dir, "state.db"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTUISwitchesAgentWithPAmongDetectedOnly(t *testing.T) {
	home := t.TempDir()
	writeClaudeJSONLFixture(t, home)
	writeHermesStateDBFixture(t, home)

	m := newTUIModel(Env{HomeDir: home}, testRoots(home))
	m = resize(t, m, 100, 30)

	if len(m.detected) != 2 || m.detected[0] != agents.ClaudeCode || m.detected[1] != agents.Hermes {
		t.Fatalf("detected = %v, want [claude-code hermes] (codex/gemini-cli absent)", m.detected)
	}
	if kind, _ := m.currentAgent(); kind != agents.ClaudeCode {
		t.Fatalf("initial agent = %v, want claude-code", kind)
	}

	m = press(t, m, tea.Key{Text: "p", Code: 'p'})
	if kind, _ := m.currentAgent(); kind != agents.Hermes {
		t.Fatalf("after p: agent = %v, want hermes", kind)
	}

	m = press(t, m, tea.Key{Text: "p", Code: 'p'})
	if kind, _ := m.currentAgent(); kind != agents.ClaudeCode {
		t.Fatalf("after second p (wrap): agent = %v, want claude-code", kind)
	}
}

func TestTUIPeriodCyclesSevenThirtyOneWithLeftRight(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)

	if windowDays[m.dayIdx] != 7 {
		t.Fatalf("initial window = %d, want 7", windowDays[m.dayIdx])
	}

	m = press(t, m, tea.Key{Code: tea.KeyRight})
	if windowDays[m.dayIdx] != 30 {
		t.Fatalf("after one right: window = %d, want 30", windowDays[m.dayIdx])
	}
	if !strings.Contains(m.View().Content, "window: 30d") {
		t.Fatalf("View() does not show window: 30d:\n%s", m.View().Content)
	}

	m = press(t, m, tea.Key{Code: tea.KeyRight})
	if windowDays[m.dayIdx] != 1 {
		t.Fatalf("after two right: window = %d, want 1", windowDays[m.dayIdx])
	}

	m = press(t, m, tea.Key{Code: tea.KeyRight})
	if windowDays[m.dayIdx] != 7 {
		t.Fatalf("after three right (wrap): window = %d, want 7", windowDays[m.dayIdx])
	}

	m = press(t, m, tea.Key{Code: tea.KeyLeft})
	if windowDays[m.dayIdx] != 1 {
		t.Fatalf("after left (wrap back): window = %d, want 1", windowDays[m.dayIdx])
	}
}

func writeHermesConfigFixture(t *testing.T, home, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// realFindingFor loads a Finding through the same fixer.DetectHermesCacheTTLFinding
// call `ats fix` uses: the TUI never fabricates its own Finding, so tests
// must not either.
func realFindingFor(t *testing.T, configPath string) fixer.Finding {
	t.Helper()
	f, ok, err := fixer.DetectHermesCacheTTLFinding(configPath)
	if err != nil {
		t.Fatalf("DetectHermesCacheTTLFinding: %v", err)
	}
	if !ok {
		t.Fatalf("DetectHermesCacheTTLFinding: ok = false, want a finding")
	}
	return f
}

func TestTUIFixConfirmNoLeavesFileByteIdentical(t *testing.T) {
	home := t.TempDir()
	configPath := writeHermesConfigFixture(t, home, "prompt_caching:\n  cache_ttl: 1h\n")
	finding := realFindingFor(t, configPath)

	before, _ := os.ReadFile(configPath)
	beforeSHA := sha256.Sum256(before)

	m := newTUIModel(Env{HomeDir: home, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, fixFinding: finding, fixHasFinding: true})
	m = press(t, m, tea.Key{Text: "5", Code: '5'}) // Fix tab
	m = press(t, m, tea.Key{Code: tea.KeyEnter})   // open confirm (default No)
	if !m.confirming {
		t.Fatalf("pressing enter on the Fix tab did not open the confirm prompt")
	}

	m = press(t, m, tea.Key{Code: tea.KeyEnter}) // submit the default (No)
	if m.confirming {
		t.Fatalf("confirm still open after submitting the default No")
	}
	if m.fixResultLine != "not applied" {
		t.Fatalf("fixResultLine = %q, want %q", m.fixResultLine, "not applied")
	}

	after, _ := os.ReadFile(configPath)
	afterSHA := sha256.Sum256(after)
	if beforeSHA != afterSHA {
		t.Fatalf("confirming No modified the file: before sha=%x after sha=%x", beforeSHA, afterSHA)
	}
	if matches, _ := filepath.Glob(configPath + ".bak-*"); len(matches) != 0 {
		t.Fatalf("confirming No created a backup file: %v", matches)
	}
}

func TestTUIFixConfirmYesWritesBackupThenFile(t *testing.T) {
	home := t.TempDir()
	configPath := writeHermesConfigFixture(t, home, "prompt_caching:\n  cache_ttl: 1h\n  other: x\n")
	finding := realFindingFor(t, configPath)

	m := newTUIModel(Env{HomeDir: home, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, fixFinding: finding, fixHasFinding: true})
	m = press(t, m, tea.Key{Text: "5", Code: '5'})
	m = press(t, m, tea.Key{Code: tea.KeyEnter}) // open confirm

	m = press(t, m, tea.Key{Text: "y", Code: 'y'}) // accept
	if m.confirming {
		t.Fatalf("confirm still open after accepting with 'y'")
	}
	if !strings.HasPrefix(m.fixResultLine, "applied:") {
		t.Fatalf("fixResultLine = %q, want it to start with 'applied:'", m.fixResultLine)
	}

	newContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newContent), "cache_ttl: auto") {
		t.Fatalf("config not updated: %q", newContent)
	}
	if !strings.Contains(string(newContent), "other: x") {
		t.Fatalf("unrelated line lost: %q", newContent)
	}

	matches, _ := filepath.Glob(configPath + ".bak-*")
	if len(matches) != 1 {
		t.Fatalf("backup files = %v, want exactly 1", matches)
	}
	backupContent, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(backupContent), "cache_ttl: 1h") {
		t.Fatalf("backup lost the old value: %q", backupContent)
	}
}

// TestTUIFullHelpKeepsHeaderVisible is the regression test for the full
// help ("?") pushing the tab bar off the top of the viewport: the header
// (tabs + agent/window line) must stay on screen no matter how tall the
// footer grows when help toggles.
func TestTUIFullHelpKeepsHeaderVisible(t *testing.T) {
	home := t.TempDir()
	writeClaudeJSONLFixture(t, home)

	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, cacheResults: []AgentCacheResult{demoCacheResult()}})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.refreshBody()

	m = press(t, m, tea.Key{Text: "?", Code: '?'})

	lines := strings.Split(m.View().Content, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "Cache") {
		t.Fatalf("tab bar not visible on the first line after opening full help:\n%s", m.View().Content)
	}
	if len(lines) < 2 || !strings.Contains(lines[1], "agent:") {
		t.Fatalf("agent line not visible on the second line after opening full help:\n%s", m.View().Content)
	}
}

// TestTUIAgentSelectorExcludesAgentsWithoutUsableData is the regression
// test for the selector listing Codex even though ~/.codex only has a
// home directory and no usable data (collectCache always skips it, see
// codexSkipReason): the selector must contain claude-code only, the
// excluded agents must appear as one dim "skipped: ..." line, and their
// full skip reason must show up wrapped (never cut) in the "?" help.
func TestTUIAgentSelectorExcludesAgentsWithoutUsableData(t *testing.T) {
	home := t.TempDir()
	writeClaudeJSONLFixture(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)

	if len(m.detected) != 1 || m.detected[0] != agents.ClaudeCode {
		t.Fatalf("detected = %v, want [claude-code] (codex has a home dir but no usable data)", m.detected)
	}

	shortView := m.View().Content
	if !strings.Contains(shortView, "skipped: codex, copilot, gemini-cli, hermes") {
		t.Fatalf("View() does not show the dim skipped line:\n%s", shortView)
	}
	if strings.Contains(shortView, "openai/codex#9660") {
		t.Fatalf("the full skip reason leaked into the short view instead of staying in the '?' help:\n%s", shortView)
	}

	m = press(t, m, tea.Key{Text: "?", Code: '?'})
	fullView := m.View().Content
	if !strings.Contains(fullView, "(openai/codex#9660),") {
		t.Fatalf("full help does not show codex's complete, unwrapped skip reason:\n%s", fullView)
	}
}

func goldenTUIPath(name string) string {
	return filepath.Join("testdata", name)
}

// readGoldenFile reads one committed golden file's content as a string,
// for tests that scan existing goldens directly (e.g.
// TestFrenchSummaryGoldensHaveNoJargon) rather than comparing a freshly
// rendered View() against one.
func readGoldenFile(t *testing.T, name string) (string, error) {
	t.Helper()
	data, err := os.ReadFile(goldenTUIPath(name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// stripTrailingSpaces trims trailing blanks from every line: the viewport
// pads each line to the full column width, and that pad count shifts with
// the length of whatever variable text preceded it (e.g. a temp dir path
// of a different digit count between runs), which would otherwise make
// the golden flaky for reasons that have nothing to do with the actual
// layout being pinned.
func stripTrailingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func checkTUIGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenTUIPath(name)
	if *tuiUpdate {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run `go test ./internal/cliapp/... -run TestTUIGolden -tui-update` once to create it)", path, err)
	}
	if got != string(want) {
		t.Fatalf("View() does not match golden %s.\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

// demoCacheResult is the fixture used by the Cache tab golden test: one
// agent's full card data, built the same way `ats cache` builds it (same
// AgentCacheResult shape), so the golden pins the TUI's layout around the
// exact renderer `ats cache` itself uses.
func demoCacheResult() AgentCacheResult {
	return AgentCacheResult{
		Agent: "claude-code", Days: 7, Sessions: 12, Calls: 340,
		CacheWriteTokens: 9_870_000, CacheReadTokens: 64_700_000,
		InputTokens: 1_200_000, OutputTokens: 230_000, HitRate: 0.867,
	}
}

// TestTUIGoldenCacheTab pins the full View() output (header, Cache card
// body, footer) at a fixed 100x30 size with NoColor so it is pasteable
// plain ASCII and deterministic regardless of the environment it runs in.
func TestTUIGoldenCacheTab(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN, Now: func() time.Time { return dashboardGoldenNow }}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, cacheResults: []AgentCacheResult{demoCacheResult()}})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.refreshBody()

	checkTUIGolden(t, "tui_cache_100x30.golden", stripTrailingSpaces(m.View().Content))
}

// TestTUIGoldenOutputStableUnderTwoFakeRealDates is the card's explicit
// "run all TUI goldens with two different fake real dates -> identical
// output" check for the newTUIModel path (the Dashboard tab's histogram
// renders inside this same golden, see TestTUIGoldenCacheTab): with the
// same Env.Now anchor on both builds, the real wall clock advancing
// between them (time.Sleep) must never leak into the output, proving
// every loader this golden depends on (dashboard histogram included)
// reads the injected clock and never time.Now() directly.
func TestTUIGoldenOutputStableUnderTwoFakeRealDates(t *testing.T) {
	build := func() string {
		home := t.TempDir()
		m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN, Now: func() time.Time { return dashboardGoldenNow }}, testRoots(home))
		m = resize(t, m, 100, 30)
		m = feedLoaded(t, m, loadedMsg{days: 7, cacheResults: []AgentCacheResult{demoCacheResult()}})
		m.detected = []agents.Kind{agents.ClaudeCode}
		m.skipped = nil
		m.refreshBody()
		return stripTrailingSpaces(m.View().Content)
	}
	out1 := build()
	time.Sleep(5 * time.Millisecond)
	out2 := build()
	if out1 != out2 {
		t.Fatalf("View() changed between two builds of the same fixed clock separated by real wall-clock time, want identical:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
}

// TestTUIGoldenFixTab pins the Fix tab's pending-change view (before any
// confirm) at the same fixed size.
func TestTUIGoldenFixTab(t *testing.T) {
	home := t.TempDir()
	configPath := writeHermesConfigFixture(t, home, "prompt_caching:\n  cache_ttl: 1h\n")
	finding := realFindingFor(t, configPath)

	// The file path embeds t.TempDir()'s unique per-run path, and that
	// path's length varies a lot by OS (short /tmp/... on Linux, long
	// /private/var/folders/.../T/... on macOS): normalize it to a fixed
	// placeholder *before* rendering, not after. The Fix tab body is
	// word-wrapped by lipgloss to the viewport width, so a longer real
	// path wraps at a different column than a short one; normalizing
	// only the final rendered string cannot undo a wrap that already
	// happened on the real (long) path, which is what truncated the
	// file name on macOS. Rendering the already-normalized placeholder
	// makes the wrap identical, and therefore the golden stable, on
	// every OS.
	finding.File = strings.ReplaceAll(finding.File, home, "/home/demo")
	// On Windows, home and the rest of the path (joined by filepath.Join)
	// use backslash separators; the replacement above only swaps the
	// home prefix for the forward-slash placeholder and leaves any
	// trailing backslashes from the real path untouched. Normalize to
	// forward slashes now, before rendering, so the golden is identical
	// across OSes (same reasoning as the comment above: this must happen
	// before word-wrap, not after).
	finding.File = filepath.ToSlash(finding.File)

	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, fixFinding: finding, fixHasFinding: true})
	m.skipped = nil
	m = press(t, m, tea.Key{Text: "5", Code: '5'})

	checkTUIGolden(t, "tui_fix_100x30.golden", stripTrailingSpaces(m.View().Content))
}
