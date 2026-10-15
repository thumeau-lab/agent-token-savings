package cliapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestTUIGoldenCacheTab100x30EN pins the English View() at 100x30
// (narrow: one panel per tab), the exact size/language combination the
// card asks for.
func TestTUIGoldenCacheTab100x30EN(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, cacheResults: []AgentCacheResult{demoCacheResult()}})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	checkTUIGolden(t, "tui_cache_100x30_en.golden", stripTrailingSpaces(m.View().Content))
}

// TestTUIGoldenCacheTab100x30FR pins the French View() at the same
// 100x30 size: the card's required golden pair (en/fr at this size).
func TestTUIGoldenCacheTab100x30FR(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.FR}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{days: 7, cacheResults: []AgentCacheResult{demoCacheResult()}})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	content := stripTrailingSpaces(m.View().Content)
	if strings.Contains(content, "Cache write") || strings.Contains(content, "sessions,") && strings.Contains(content, " calls over ") {
		t.Fatalf("French View() still contains English wording:\n%s", content)
	}
	checkTUIGolden(t, "tui_cache_100x30_fr.golden", content)
}

// TestTUIGoldenCacheTab160x45EN pins the English View() at 160x45 (wide:
// the multi-panel layout, Cache+TTL stacked left, Prefix right).
func TestTUIGoldenCacheTab160x45EN(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 160, 45)
	m = feedLoaded(t, m, loadedMsg{
		days:          7,
		cacheResults:  []AgentCacheResult{demoCacheResult()},
		prefixResults: []AgentPrefixResult{demoPrefixResult(i18n.EN)},
		ttlResults:    []AgentTTLResult{demoTTLResult(i18n.EN)},
	})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	if !m.wide() {
		t.Fatalf("160 columns should trigger the wide multi-panel layout")
	}
	checkTUIGolden(t, "tui_cache_160x45_en.golden", stripTrailingSpaces(m.View().Content))
}

// TestTUIGoldenCacheTab160x45FR is the 160x45 French counterpart.
func TestTUIGoldenCacheTab160x45FR(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.FR}, testRoots(home))
	m = resize(t, m, 160, 45)
	m = feedLoaded(t, m, loadedMsg{
		days:          7,
		cacheResults:  []AgentCacheResult{demoCacheResult()},
		prefixResults: []AgentPrefixResult{demoPrefixResult(i18n.FR)},
		ttlResults:    []AgentTTLResult{demoTTLResult(i18n.FR)},
	})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	checkTUIGolden(t, "tui_cache_160x45_fr.golden", stripTrailingSpaces(m.View().Content))
}

// demoPrefixResult returns demo prefix data for the TUI Prefix panel,
// with component names localized exactly like the real collectPrefix
// path does (fromReport calls localizedComponentName before an
// AgentPrefixResult is ever built): a raw, hardcoded English name here
// would make the French golden show an untranslated label that could
// never actually appear in production.
func demoPrefixResult(lang i18n.Lang) AgentPrefixResult {
	return AgentPrefixResult{
		Agent: "claude-code", TotalBytes: 12_400, EstimatedTokens: 3_100,
		TokenEstimate: "bytes / 4, a rough estimate, not a tokenizer count",
		Components: []ComponentJSON{
			{Name: "CLAUDE.md (user)", Bytes: 8_000},
			{Name: localizedComponentName("MCP servers (config.yaml)", lang), Bytes: 4_400},
		},
		BuiltinTokens: 9_000, BuiltinSource: "claude-code system prompt + tool schemas",
		MCPServerCount: 2, MCPServersNote: "2 MCP servers configured: context7, kanban",
	}
}

// demoTTLMethodNoteAndHeadline returns the demo TTL verdict text
// (MethodNote, Headline) localized for lang. These are hand-written,
// shortened stand-ins for the real sentences collectTTL/claudeCodeTTLResult
// build via i18n.T (ttl.go:88-99: the production MethodNote/Headline are
// already fully localized at the source, same as collectPrefix for
// component names) -- kept short here on purpose, unlike the verbose
// production template (which embeds a whole recommendation sentence),
// so the golden stays readable and does not reintroduce wrapping noise
// unrelated to the stat-grid bug this file actually pins. A hardcoded
// English sentence here regardless of lang would leak into the French
// golden (reviewer feedback), which is exactly what
// TestFrenchGoldensHaveNoKnownEnglishFragment now catches.
func demoTTLMethodNoteAndHeadline(lang i18n.Lang) (methodNote, headline string) {
	if lang == i18n.FR {
		return "Claude Code n'a pas de session machine/cron/sous-agent : tout y est interactif.",
			"Passer à 1 h de TTL économiserait environ 12,0 % du coût d'écriture."
	}
	return "Claude Code sessions are fully interactive -- there is no machine/cron/subagent session type.",
		"Switching to a 1h cache TTL would save an estimated 12.0% of cache-write cost."
}

func demoTTLResult(lang i18n.Lang) AgentTTLResult {
	methodNote, headline := demoTTLMethodNoteAndHeadline(lang)
	return AgentTTLResult{
		Agent: "claude-code", Days: 7,
		MachineSessions: 12, Calls: 340,
		FirstCallWriteTokens: 1_200_000, MidGapWriteTokens: 300_000, LongGapWriteTokens: 50_000,
		MethodNote: methodNote,
		Headline:   headline,
	}
}

// TestCatalogKeyHasNoEnglishLeftoverInFrench is a light smoke test: every
// catalog key's French entry must differ from its English entry (a
// French translation that is byte-identical to English almost always
// means someone forgot to translate it), except technical loanwords this
// repo's own README.fr.md already keeps as-is ("Cache").
func TestCatalogKeyHasNoEnglishLeftoverInFrench(t *testing.T) {
	allowedIdentical := map[string]bool{"tab.cache": true, "confirm.affirmative": true}
	for _, key := range []string{
		"tui.loading", "key.quit", "fix.not_applied", "verdict.hit_rate_only",
		"tab.dashboard", "tab.cache", "tab.prefix", "tab.ttl",
		"dashboard.panel_improve", "dashboard.nothing_to_fix", "dashboard.no_agents",
	} {
		en := i18n.T(i18n.EN, key)
		fr := i18n.T(i18n.FR, key)
		if en == fr && !allowedIdentical[key] {
			t.Errorf("catalog key %q has identical en/fr text %q, looks untranslated", key, en)
		}
	}
}

// knownEnglishLeftovers is the list of English fragments that have
// leaked into the French UI before (reviewer feedback): the Claude
// Code built-in citation prose, the prefixinfo component labels that
// bypassed the catalog, and renamed labels (old "Taux de hit"/"Tok
// natifs" wording). Shared by TestCatalogFrenchHasNoKnownEnglishFragment
// (checks the catalog directly) and
// TestFrenchGoldensHaveNoKnownEnglishFragment (checks every committed
// French golden file, so a regression in any rendered view -- not just
// the catalog -- fails the build).
var knownEnglishLeftovers = []string{
	"tool schemas",
	"skills index",
	"agents index",
	"MCP servers (config.yaml)",
	"toolsets (config.yaml)",
	"Taux de hit",
	"Tok natifs",
	"Switching to a 1h cache TTL",
	"fully interactive -- there is no machine/cron/subagent",
}

// TestCatalogFrenchHasNoKnownEnglishFragment extends the parity check:
// a French golden/render must never contain any of these known English
// fragments that have leaked into the French UI before (reviewer
// feedback) -- the Claude Code built-in citation prose, and the
// prefixinfo component labels that bypassed the catalog.
func TestCatalogFrenchHasNoKnownEnglishFragment(t *testing.T) {
	// Walk every catalog key via i18n.T and assert the French render of
	// known multi-arg templates never reproduces an English fragment.
	for _, leftover := range knownEnglishLeftovers {
		for _, key := range []string{
			"stat.hit_rate", "bar.hit_rate", "text.hit_rate_line", "verdict.hit_rate_only",
			"stat.builtin_tok", "component.skills_index", "component.agents_index",
			"component.mcp_servers_config", "component.toolsets_config",
			"prefix.claude_builtin_citation",
		} {
			if strings.Contains(i18n.T(i18n.FR, key), leftover) {
				t.Errorf("catalog key %q's French entry contains English fragment %q", key, leftover)
			}
		}
	}
}

// TestFrenchGoldensHaveNoKnownEnglishFragment walks every committed
// French TUI golden file in testdata/ (name matches *_fr.golden) and
// fails if any contains a known English leftover fragment. Unlike
// TestCatalogFrenchHasNoKnownEnglishFragment (which only checks the
// catalog's own i18n.T output), this scans the actual pinned View()
// bytes, so a leftover introduced by a test fixture or a render path
// that bypasses the catalog (e.g. a component name set directly on an
// AgentPrefixResult instead of going through collectPrefix) fails the
// build too.
func TestFrenchGoldensHaveNoKnownEnglishFragment(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("testdata", "*_fr.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no testdata/*_fr.golden files found -- glob pattern or testdata layout changed")
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, leftover := range knownEnglishLeftovers {
			if strings.Contains(text, leftover) {
				t.Errorf("%s still contains English leftover %q", path, leftover)
			}
		}
	}
}

// demoTTLResultWideBug reproduces the exact TTL stat-grid overflow bug
// (reviewer feedback, LANG=fr_FR.UTF-8, 170x46): Sessions=150 ("150"),
// Calls=594 ("594"), MidGapWriteTokens=711_000 ("711 K") -- the exact
// values the reviewer saw wrapped onto their own line with a broken
// right border.
func demoTTLResultWideBug(lang i18n.Lang) AgentTTLResult {
	methodNote, headline := demoTTLMethodNoteAndHeadline(lang)
	return AgentTTLResult{
		Agent: "claude-code", Days: 7,
		MachineSessions: 150, Calls: 594,
		FirstCallWriteTokens: 1_200_000, MidGapWriteTokens: 711_000, LongGapWriteTokens: 50_000,
		MethodNote: methodNote,
		Headline:   headline,
	}
}

// stripANSIGolden removes ANSI CSI escapes, for the label/value
// same-line assertions below (the pinned golden itself keeps the raw
// ANSI, like render's own card goldens).
func stripANSIGolden(s string) string {
	var out strings.Builder
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:i])
		s = s[i+2:]
		j := strings.IndexAny(s, "mK")
		if j < 0 {
			break
		}
		s = s[j+1:]
	}
	return out.String()
}

// TestTUIGoldenTTLWideColored170x46FR is the regression test for the
// TTL panel's stat grid wrapping a row (and breaking the right border)
// in the wide multi-panel layout, with real color (not NoColor) at
// 170x46 -- the exact size/language the reviewer drove in tmux.
func TestTUIGoldenTTLWideColored170x46FR(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: false, Dark: true, Lang: i18n.FR}, testRoots(home))
	m = resize(t, m, 170, 46)
	m = feedLoaded(t, m, loadedMsg{
		days:          7,
		cacheResults:  []AgentCacheResult{demoCacheResult()},
		prefixResults: []AgentPrefixResult{demoPrefixResult(i18n.FR)},
		ttlResults:    []AgentTTLResult{demoTTLResultWideBug(i18n.FR)},
	})
	m.detected = []agents.Kind{agents.ClaudeCode}
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	if !m.wide() {
		t.Fatalf("170 columns should trigger the wide multi-panel layout")
	}
	content := m.View().Content
	stripped := stripANSIGolden(content)
	lines := strings.Split(stripped, "\n")
	assertTogether := func(label, value string) {
		for _, line := range lines {
			if strings.Contains(line, value) {
				if !strings.Contains(line, label) {
					continue
				}
				return
			}
		}
		t.Errorf("no line has both label %q and value %q together (stat row wrapped or missing):\n%s", label, value, stripped)
	}
	assertTogether("Sessions", "150")
	assertTogether("Appels", "594")
	assertTogether("écart moyen (5-60 min)", "711 K")

	checkTUIGolden(t, "tui_ttl_170x46_fr.golden", stripTrailingSpaces(content))
}
