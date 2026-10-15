package cliapp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// tuiTab is one of the five tabs the interactive app arranges the
// existing summary/cache/prefix/ttl/fix renderers and data collectors
// under.
type tuiTab int

const (
	tabDashboard tuiTab = iota
	tabCache
	tabPrefix
	tabTTL
	tabFix
	tabCount
)

func (t tuiTab) String() string {
	switch t {
	case tabDashboard:
		return "Dashboard"
	case tabCache:
		return "Cache"
	case tabPrefix:
		return "Prefix"
	case tabTTL:
		return "TTL"
	case tabFix:
		return "Fix"
	default:
		return ""
	}
}

// catalogKey is the i18n catalog key for this tab's localized label.
func (t tuiTab) catalogKey() string {
	switch t {
	case tabDashboard:
		return "tab.dashboard"
	case tabCache:
		return "tab.cache"
	case tabPrefix:
		return "tab.prefix"
	case tabTTL:
		return "tab.ttl"
	case tabFix:
		return "tab.fix"
	default:
		return ""
	}
}

// keyDigit is the btop-style shortcut digit shown next to this tab's
// name in the header, and the digit handleKey listens for to jump
// straight to it. Dashboard is the new default screen and keeps the
// first digit (1); Cache/Prefix/TTL/Fix shift to 2-5 so the digits read
// in the same visual order as the tab bar itself (card: "digits in
// visual order").
func (t tuiTab) keyDigit() int {
	switch t {
	case tabDashboard:
		return 1
	case tabCache:
		return 2
	case tabPrefix:
		return 3
	case tabTTL:
		return 4
	case tabFix:
		return 5
	default:
		return 0
	}
}

// wideLayoutMinWidth is the terminal width at or above which the TUI
// shows several panels at once (Cache+TTL stacked left, Prefix right)
// instead of one panel per tab. The Fix tab always stays a separate
// tab/modal regardless of width.
const wideLayoutMinWidth = 140

// windowDays lists the look-back windows `d` cycles through, in order.
var windowDays = []int{7, 30, 1}

// loadedMsg carries the result of one async data load: every agent's
// cache/prefix/ttl card data (collected through the exact same
// collectCache/collectPrefix/collectTTL functions `ats cache`/`ats
// prefix`/`ats ttl` use, so the TUI has no second computation path) plus
// the current `ats fix` finding, if any, and the per-day history used by
// the Cache panel's sparkline (dailyHermes/dailyClaude, one of which
// applies depending on the current agent).
type loadedMsg struct {
	days            int
	cacheResults    []AgentCacheResult
	prefixResults   []AgentPrefixResult
	ttlResults      []AgentTTLResult
	fixFinding      fixer.Finding
	fixHasFinding   bool
	dailyHermes     []hermesusage.DailyRow
	dailyClaude     []claudecode.DailyRow
	dailyHermesLong []hermesusage.DailyRow
	hermesCompare   comparisonResult
	claudeCompare   comparisonResult
	loadedAt        time.Time
}

// tuiKeyMap documents every key the interactive app responds to, for the
// footer's short/full help (charm.land/bubbles/v2/help).
type tuiKeyMap struct {
	SwitchTab   key.Binding
	SwitchAgent key.Binding
	Period      key.Binding
	ToggleScope key.Binding
	Reload      key.Binding
	Scroll      key.Binding
	Confirm     key.Binding
	Lang        key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func newTUIKeyMap(lang i18n.Lang) tuiKeyMap {
	return tuiKeyMap{
		SwitchTab:   key.NewBinding(key.WithKeys("tab", "shift+tab", "1", "2", "3", "4", "5"), key.WithHelp("tab/1-5", i18n.T(lang, "key.switch_tab"))),
		SwitchAgent: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", i18n.T(lang, "key.switch_agent"))),
		Period:      key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("\u2190/\u2192", i18n.T(lang, "key.period"))),
		ToggleScope: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", i18n.T(lang, "key.toggle_scope"))),
		Reload:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", i18n.T(lang, "key.reload"))),
		Scroll:      key.NewBinding(key.WithKeys("up", "down", "pgup", "pgdown"), key.WithHelp("up/down/pgup/pgdn", i18n.T(lang, "key.scroll"))),
		Confirm:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", i18n.T(lang, "key.confirm"))),
		Lang:        key.NewBinding(key.WithKeys("L"), key.WithHelp("L", i18n.T(lang, "key.lang"))),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", i18n.T(lang, "key.help"))),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", i18n.T(lang, "key.quit"))),
	}
}

// ShortHelp orders the footer's keys in the exact sequence the card
// specifies: "q quitter <-/-> periode p agent 1-5 vues ? aide L
// langue". bubbles/help's ShortHelpView stops adding items (replacing
// the rest with an ellipsis) once they stop fitting the terminal
// width, in the order given here.
func (k tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Period, k.SwitchAgent, k.SwitchTab, k.Help, k.Lang}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.SwitchTab, k.SwitchAgent},
		{k.Period, k.ToggleScope, k.Reload, k.Scroll},
		{k.Confirm, k.Lang, k.Help, k.Quit},
	}
}

// tuiModel is the Bubble Tea v2 model for interactive `ats`. It arranges
// the existing cache/prefix/ttl/fix renderers and data collectors: this
// file only adds layout, navigation and the fix confirm flow on top of
// them, never a second computation path.
type tuiModel struct {
	env   Env
	roots agents.Roots
	lang  i18n.Lang
	now   func() time.Time

	width, height int
	tab           tuiTab
	dayIdx        int

	detected  []agents.Kind  // agents.All, filtered to only those with usable local data
	skipped   []skippedAgent // the rest of agents.All, each with why it was excluded
	agentIdx  int
	allAgents bool // Dashboard hero scope: true = every detected agent combined, false = the selected one only ('a' toggles it)

	keys     tuiKeyMap
	help     help.Model
	fullHelp bool
	spin     spinner.Model
	vp       viewport.Model

	loading bool

	cacheResults  []AgentCacheResult
	prefixResults []AgentPrefixResult
	ttlResults    []AgentTTLResult
	dailyHermes   []hermesusage.DailyRow
	dailyClaude   []claudecode.DailyRow
	// dailyHermesLong is Hermes' daily history over a fixed, wider
	// lookback than the selected window (dashboardHeatmapLookbackDays):
	// only the Activite heatmap needs it (card: "read further back than
	// the window for this panel only"). Claude Code needs no
	// counterpart: claudecode.DailyUsage already returns every day in
	// history regardless of window (see dailyClaude above).
	dailyHermesLong []hermesusage.DailyRow
	hermesCompare   comparisonResult
	claudeCompare   comparisonResult
	lastLoaded      time.Time

	fixHasFinding bool
	fixFinding    fixer.Finding
	fixResultLine string
	confirming    bool
	confirmForm   *huh.Form

	quitting bool
}

// newTUIModel builds the initial model. Real data is loaded
// asynchronously (see Init/loadCmd); newTUIModel itself only detects
// which agents are present (cheap os.Stat/Glob calls, not a real load) so
// the agent selector can be populated before the first paint.
func newTUIModel(env Env, roots agents.Roots) tuiModel {
	lang := resolveLang(env)
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := tuiModel{
		env:       env,
		roots:     roots,
		lang:      lang,
		now:       func() time.Time { return resolveNow(env) },
		tab:       tabDashboard,
		allAgents: true,
		detected:  detectedAgents(roots),
		skipped:   skippedAgentsFor(roots, lang),
		keys:      newTUIKeyMap(lang),
		help:      help.New(),
		spin:      sp,
		vp:        viewport.New(),
		loading:   true,
	}
	m.applyHelpStyles()
	m.vp.SetWidth(cardWidth(env))
	m.vp.SetHeight(20)
	m.vp.SetContent(i18n.T(lang, "tui.loading"))
	return m
}

// applyHelpStyles sets help.Model's key/description colors: the key
// itself in the renderer's accent (btop style: the shortcut stands out),
// the description in the primary label color, both zeroed out when this
// renderer would not emit ANSI (NO_COLOR/non-TTY), same as before.
func (m *tuiModel) applyHelpStyles() {
	if !m.renderer().Colored() {
		m.help.Styles = help.Styles{}
		return
	}
	r := m.renderer()
	accent, label, dim := r.ChromeColors()
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)
	descStyle := lipgloss.NewStyle().Foreground(label)
	sepStyle := lipgloss.NewStyle().Foreground(dim)
	m.help.Styles = help.Styles{
		ShortKey:       keyStyle,
		ShortDesc:      descStyle,
		ShortSeparator: sepStyle,
		Ellipsis:       sepStyle,
		FullKey:        keyStyle,
		FullDesc:       descStyle,
		FullSeparator:  sepStyle,
	}
}

// skippedAgent is one agents.All member the Cache/Prefix/TTL agent
// selector excludes because it has no usable local data: either nothing
// detected at all, or -- for Codex and Gemini CLI -- a structural "not
// parsed yet" limitation every collector (collectCache/collectPrefix/
// collectTTL) already applies unconditionally. Shown as one dim
// "skipped: ..." line under the selector; Reason (the same text
// collectCache would give it) is shown in full, wrapped, in the "?" help.
type skippedAgent struct {
	Kind   agents.Kind
	Reason string
}

// detectedAgents lists the agents the Cache/Prefix/TTL selector cycles
// through with left/right: only those with actual usable local data.
// Codex and Gemini CLI are never included here even when their home
// directory is present, because collectCache/collectPrefix/collectTTL
// always mark them Skipped (no stable schema to parse yet) -- a home
// directory on its own is not "usable data" (see skippedAgentsFor).
func detectedAgents(roots agents.Roots) []agents.Kind {
	var out []agents.Kind
	if roots.ClaudeHasData() {
		out = append(out, agents.ClaudeCode)
	}
	if roots.CodexSessionFilesExist() {
		out = append(out, agents.Codex)
	}
	if roots.CopilotHasData() {
		out = append(out, agents.Copilot)
	}
	if roots.HermesHasData() {
		out = append(out, agents.Hermes)
	}
	return out
}

// skippedAgentsFor lists every agents.All member not returned by
// detectedAgents, each paired with the reason collectCache would give it
// (i18n keys skip.codex/skip.gemini, or "no local usage data found" when
// the home directory itself is absent), in agents.All order.
func skippedAgentsFor(roots agents.Roots, lang i18n.Lang) []skippedAgent {
	var out []skippedAgent
	for _, k := range agents.All {
		switch k {
		case agents.ClaudeCode:
			if !roots.ClaudeHasData() {
				out = append(out, skippedAgent{k, i18n.T(lang, "skip.no_local_data")})
			}
		case agents.Hermes:
			if !roots.HermesHasData() {
				out = append(out, skippedAgent{k, i18n.T(lang, "skip.no_local_data")})
			}
		case agents.Codex:
			if roots.CodexSessionFilesExist() {
				continue
			}
			if roots.CodexHasData() {
				out = append(out, skippedAgent{k, i18n.T(lang, codexSkipKey)})
			} else {
				out = append(out, skippedAgent{k, i18n.T(lang, "skip.no_local_data")})
			}
		case agents.Copilot:
			if !roots.CopilotHasData() {
				out = append(out, skippedAgent{k, i18n.T(lang, "skip.no_local_data")})
			}
		case agents.GeminiCLI:
			if roots.GeminiHasData() {
				out = append(out, skippedAgent{k, i18n.T(lang, geminiSkipKey)})
			} else {
				out = append(out, skippedAgent{k, i18n.T(lang, "skip.no_local_data")})
			}
		}
	}
	return out
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.loadCmd())
}

// loadCmd re-collects every tab's data through the same functions the
// non-interactive subcommands use, for the current window (m.dayIdx),
// plus the per-day history (hermesusage.QueryDaily / claudecode.DailyUsage
// -- the same query/parser collectCache already uses, called again here
// only for the day-level grouping the Cache panel's sparkline needs; see
// those functions for the single source of truth they share with `ats
// cache`) used by the Cache panel's sparkline.
func (m tuiModel) loadCmd() tea.Cmd {
	roots := m.roots
	env := m.env
	lang := m.lang
	days := windowDays[m.dayIdx]
	now := m.now
	return func() tea.Msg {
		nowVal := now()
		cache := collectCache(roots, "", days, lang, nowVal)
		prefix := collectPrefix(roots, tuiCwd(env), "", lang)
		ttl := collectTTL(roots, "", days, lang, nowVal)
		configPath := filepath.Join(roots.HermesHome, "config.yaml")
		finding, hasFinding, err := fixer.DetectHermesCacheTTLFinding(configPath)
		if err != nil {
			hasFinding = false
		}
		var dailyHermes []hermesusage.DailyRow
		var dailyHermesLong []hermesusage.DailyRow
		var hermesCompare comparisonResult
		if roots.HermesHasData() {
			dailyHermes, _ = hermesusage.QueryDaily(filepath.Join(roots.HermesHome, "state.db"), days)
			// Previous-period comparison: the same QueryDaily reader,
			// just asked for twice the window and split by day cutoff
			// in memory (no new parser, see splitHermesWindow).
			if rows2x, err := hermesusage.QueryDaily(filepath.Join(roots.HermesHome, "state.db"), 2*days); err == nil {
				cur, prev := splitHermesWindow(rows2x, days, nowVal)
				hermesCompare = hermesComparison(cur, prev)
			}
			// The Activite heatmap always wants dashboardHeatmapLookbackDays
			// of history regardless of the selected window (card: "read
			// further back than the window for this panel only").
			dailyHermesLong, _ = hermesusage.QueryDaily(filepath.Join(roots.HermesHome, "state.db"), dashboardHeatmapLookbackDays)
		}
		var dailyClaude []claudecode.DailyRow
		var claudeCompare comparisonResult
		if roots.ClaudeHasData() {
			dailyClaude, _ = claudecode.DailyUsage(roots.ClaudeHome)
			cur, prev := splitClaudeWindow(dailyClaude, days, nowVal)
			claudeCompare = claudeComparison(cur, prev)
		}
		return loadedMsg{
			days: days, cacheResults: cache, prefixResults: prefix, ttlResults: ttl,
			fixFinding: finding, fixHasFinding: hasFinding,
			dailyHermes: dailyHermes, dailyClaude: dailyClaude,
			dailyHermesLong: dailyHermesLong,
			hermesCompare:   hermesCompare, claudeCompare: claudeCompare, loadedAt: nowVal,
		}
	}
}

func tuiCwd(env Env) string {
	if env.WorkDir != "" {
		return env.WorkDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// While a fix confirm is open, every message not about the outer
	// window size goes to the embedded huh.Form: huh's own internal
	// messages (nextFieldMsg, updateFieldMsg...) are not tea.KeyPressMsg,
	// so routing only key presses here would strand the form mid-submit.
	if m.confirming {
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width, m.height = wsm.Width, wsm.Height
			m.help.SetWidth(m.width)
			m.vp.SetWidth(m.bodyWidth())
			m.vp.SetHeight(m.bodyHeight())
			return m, nil
		}
		return m.updateConfirm(msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(m.width)
		m.vp.SetWidth(m.bodyWidth())
		m.vp.SetHeight(m.bodyHeight())
		m.refreshBody()
		return m, nil
	case loadedMsg:
		m.loading = false
		m.cacheResults = msg.cacheResults
		m.prefixResults = msg.prefixResults
		m.ttlResults = msg.ttlResults
		m.fixFinding = msg.fixFinding
		m.fixHasFinding = msg.fixHasFinding
		m.fixResultLine = ""
		m.dailyHermes = msg.dailyHermes
		m.dailyClaude = msg.dailyClaude
		m.dailyHermesLong = msg.dailyHermesLong
		m.hermesCompare = msg.hermesCompare
		m.claudeCompare = msg.claudeCompare
		m.lastLoaded = msg.loadedAt
		m.refreshBody()
		return m, nil
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m tuiModel) bodyWidth() int {
	w := m.width
	if w <= 0 {
		w = cardWidth(m.env)
	}
	return w
}

func (m tuiModel) bodyHeight() int {
	h := m.height - m.headerHeight() - m.footerHeight()
	if h < 3 {
		h = 3
	}
	return h
}

// headerHeight/footerHeight report the real number of rows renderHeader/
// renderFooter currently produce (not a fixed budget): bodyHeight derives
// the viewport's height from these so the header (tabs + agent/window
// line + optional skip line) and footer (short or full help, which grows
// from 1 to several rows) are always both fully on screen, however tall
// either one happens to be right now.
func (m tuiModel) headerHeight() int {
	return strings.Count(m.renderHeader(), "\n") + 1
}

func (m tuiModel) footerHeight() int {
	return strings.Count(m.renderFooter(), "\n") + 1
}

// wide reports whether the terminal is wide enough to show several
// panels at once (Cache+TTL stacked left, Prefix right) instead of one
// panel per tab.
func (m tuiModel) wide() bool {
	return m.bodyWidth() >= wideLayoutMinWidth
}

func (m tuiModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.fullHelp = !m.fullHelp
		m.refreshBody()
		return m, nil
	case key.Matches(msg, m.keys.Lang):
		if m.lang == i18n.FR {
			m.lang = i18n.EN
		} else {
			m.lang = i18n.FR
		}
		m.env.Lang = m.lang
		m.keys = newTUIKeyMap(m.lang)
		m.applyHelpStyles()
		m.loading = true
		m.vp.SetContent(i18n.T(m.lang, "tui.loading"))
		return m, tea.Batch(m.spin.Tick, m.loadCmd())
	case msg.String() == "tab":
		m.tab = (m.tab + 1) % tabCount
		m.refreshBody()
		return m, nil
	case msg.String() == "shift+tab":
		m.tab = (m.tab - 1 + tabCount) % tabCount
		m.refreshBody()
		return m, nil
	case msg.String() == "1":
		m.tab = tabDashboard
		m.refreshBody()
		return m, nil
	case msg.String() == "2":
		m.tab = tabCache
		m.refreshBody()
		return m, nil
	case msg.String() == "3":
		m.tab = tabPrefix
		m.refreshBody()
		return m, nil
	case msg.String() == "4":
		m.tab = tabTTL
		m.refreshBody()
		return m, nil
	case msg.String() == "5":
		m.tab = tabFix
		m.refreshBody()
		return m, nil
	case msg.String() == "p":
		m.cycleAgent(1)
		return m, nil
	case msg.String() == "a":
		m.allAgents = !m.allAgents
		m.refreshBody()
		return m, nil
	case msg.String() == "left":
		m.dayIdx = (m.dayIdx - 1 + len(windowDays)) % len(windowDays)
		m.loading = true
		m.vp.SetContent(i18n.T(m.lang, "tui.loading"))
		return m, tea.Batch(m.spin.Tick, m.loadCmd())
	case msg.String() == "right":
		m.dayIdx = (m.dayIdx + 1) % len(windowDays)
		m.loading = true
		m.vp.SetContent(i18n.T(m.lang, "tui.loading"))
		return m, tea.Batch(m.spin.Tick, m.loadCmd())
	case key.Matches(msg, m.keys.Reload):
		m.loading = true
		m.vp.SetContent(i18n.T(m.lang, "tui.loading"))
		return m, tea.Batch(m.spin.Tick, m.loadCmd())
	case msg.String() == "up":
		m.vp.ScrollUp(1)
		return m, nil
	case msg.String() == "down":
		m.vp.ScrollDown(1)
		return m, nil
	case msg.String() == "pgup":
		m.vp.PageUp()
		return m, nil
	case msg.String() == "pgdown":
		m.vp.PageDown()
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		if m.tab == tabFix && m.fixHasFinding && m.fixResultLine == "" {
			return m.startConfirm()
		}
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) cycleAgent(dir int) {
	if len(m.detected) == 0 {
		return
	}
	m.agentIdx = (m.agentIdx + dir + len(m.detected)) % len(m.detected)
	m.refreshBody()
}

// startConfirm opens the huh v2 confirm prompt (NewConfirm, default
// "No") for the one finding `ats fix` would apply, embedded as a
// tea.Model exactly as charmbracelet/huh's Bubble Tea integration
// documents (huh.Form is a tea.Model).
func (m tuiModel) startConfirm() (tea.Model, tea.Cmd) {
	approved := false
	confirm := huh.NewConfirm().
		Key("approve").
		Title(i18n.T(m.lang, "confirm.title", m.fixFinding.Key, m.fixFinding.OldValue, m.fixFinding.NewValue)).
		Description(m.fixFinding.Description).
		Affirmative(i18n.T(m.lang, "confirm.affirmative")).
		Negative(i18n.T(m.lang, "confirm.negative")).
		Value(&approved)
	m.confirmForm = huh.NewForm(huh.NewGroup(confirm)).WithShowHelp(true)
	m.confirming = true
	return m, m.confirmForm.Init()
}

func (m tuiModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := m.confirmForm.Update(msg)
	if f, ok := newModel.(*huh.Form); ok {
		m.confirmForm = f
	}
	switch m.confirmForm.State {
	case huh.StateCompleted:
		approved := m.confirmForm.GetBool("approve")
		m.finishConfirm(approved)
		return m, nil
	case huh.StateAborted:
		m.finishConfirm(false)
		return m, nil
	}
	return m, cmd
}

// finishConfirm applies the finding (approved=true) through the exact
// same fixer.ApplyFix call `ats fix --write` uses -- backup first, never
// disables caching -- or records that the user skipped it.
func (m *tuiModel) finishConfirm(approved bool) {
	m.confirming = false
	m.confirmForm = nil
	if !approved {
		m.fixResultLine = i18n.T(m.lang, "fix.not_applied")
		m.refreshBody()
		return
	}
	applied, backup, err := fixer.ApplyFix(m.fixFinding, true)
	switch {
	case err != nil:
		m.fixResultLine = i18n.T(m.lang, "fix.error", err)
	case !applied:
		m.fixResultLine = i18n.T(m.lang, "fix.was_not_applied")
	default:
		m.fixResultLine = i18n.T(m.lang, "fix.applied", m.fixFinding.Key, m.fixFinding.OldValue, m.fixFinding.NewValue, backup)
	}
	m.refreshBody()
}

// refreshBody rebuilds the viewport content for the current tab/agent
// from already-loaded data: no recomputation, just re-arranging it (see
// package doc comment).
func (m *tuiModel) refreshBody() {
	m.vp.SetWidth(m.bodyWidth())
	m.vp.SetHeight(m.bodyHeight())
	m.vp.SetContent(m.bodyContent())
	m.vp.GotoTop()
}

func (m tuiModel) renderer() render.Renderer {
	return render.New(render.Options{NoColor: m.env.NoColor, IsTTY: m.env.IsTTY, Dark: m.env.Dark, ForceASCII: m.env.ForceASCII})
}

func (m tuiModel) bodyContent() string {
	if m.loading {
		return m.spin.View() + " " + i18n.T(m.lang, "tui.loading")
	}
	if m.tab == tabFix {
		return m.fixBody()
	}
	if m.tab == tabDashboard {
		return m.dashboardBody(m.bodyWidth(), m.bodyHeight())
	}
	if m.wide() {
		return m.widePanelsBody()
	}
	switch m.tab {
	case tabCache:
		return m.cacheBody(m.bodyWidth())
	case tabPrefix:
		return m.prefixBody(m.bodyWidth())
	case tabTTL:
		return m.ttlBody(m.bodyWidth())
	default:
		return ""
	}
}

// widePanelsBody lays out Cache and TTL stacked in a left column and
// Prefix in a right column, so a wide terminal (>= wideLayoutMinWidth)
// shows several panels at once instead of one panel per tab (the Fix
// tab is handled separately in bodyContent and never joins this layout).
func (m tuiModel) widePanelsBody() string {
	leftWidth := m.bodyWidth()/2 - 1
	rightWidth := m.bodyWidth() - leftWidth - 1
	left := lipgloss.JoinVertical(lipgloss.Left, m.cacheBody(leftWidth), "", m.ttlBody(leftWidth))
	right := m.prefixBody(rightWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m tuiModel) currentAgent() (agents.Kind, bool) {
	if len(m.detected) == 0 {
		return "", false
	}
	return m.detected[m.agentIdx], true
}

func (m tuiModel) cacheBody(width int) string {
	kind, ok := m.currentAgent()
	if !ok {
		return i18n.T(m.lang, "tui.no_agents_locally")
	}
	r := m.renderer()
	for _, a := range m.cacheResults {
		if a.Agent != string(kind) {
			continue
		}
		if a.Skipped {
			return r.Muted(i18n.T(m.lang, "tui.skipped_prefix", a.Agent) + "\n\n" + a.Reason)
		}
		if r.Colored() {
			card := cacheCard(a, m.lang)
			card.Graph = m.cacheGraph(kind)
			return r.RenderCard(card, width)
		}
		var sb strings.Builder
		renderCacheText(&sb, m.env, []AgentCacheResult{a})
		return sb.String()
	}
	return i18n.T(m.lang, "tui.no_data", string(kind))
}

// cacheGraph builds the Cache panel's per-day history block: two
// sparklines (hit rate, API-equivalent cost) over the already-loaded
// per-day rows for kind, min/max shown on the right of each sparkline,
// and the first/last day (short, localized) under the graph -- or ""
// when there is less than 2 days of history to draw a line through.
func (m tuiModel) cacheGraph(kind agents.Kind) string {
	var hitRates, costs []float64
	var days []string
	switch kind {
	case agents.Hermes:
		for _, row := range m.dailyHermes {
			s := hermesusage.Summarize([]hermesusage.DailyRow{row})
			hitRates = append(hitRates, s.HitRate*100)
			costs = append(costs, s.USD["input"]+s.USD["output"]+s.USD["cache_read"]+s.USD["cache_write"])
			days = append(days, row.Day)
		}
	case agents.ClaudeCode:
		for _, row := range m.dailyClaude {
			denom := row.Create + row.Read
			hitRate := 0.0
			if denom > 0 {
				hitRate = float64(row.Read) / float64(denom) * 100
			}
			usd, _ := usdSplit(row.Input, row.Output, row.Read, row.Create)
			hitRates = append(hitRates, hitRate)
			costs = append(costs, usd["input"]+usd["output"]+usd["cache_read"]+usd["cache_write"])
			days = append(days, row.Day)
		}
	}
	if len(hitRates) < 2 {
		return ""
	}
	width := 24
	hitLine := render.Sparkline(hitRates, width)
	costLine := render.Sparkline(costs, width)
	hitMin, hitMax := minMax(hitRates)
	costMin, costMax := minMax(costs)
	line1 := fmt.Sprintf("%s %s  %s %s  %s %s", i18n.T(m.lang, "stat.hit_rate"), hitLine,
		i18n.T(m.lang, "graph.min"), render.FormatPercentLang(hitMin, m.lang, 1),
		i18n.T(m.lang, "graph.max"), render.FormatPercentLang(hitMax, m.lang, 1))
	line2 := fmt.Sprintf("%s %s  %s %s  %s %s", i18n.T(m.lang, "stat.api_equiv_cost"), costLine,
		i18n.T(m.lang, "graph.min"), render.FormatUSDLang(costMin, m.lang),
		i18n.T(m.lang, "graph.max"), render.FormatUSDLang(costMax, m.lang))
	dateLine := graphDateLine(days[0], days[len(days)-1], m.lang, line1, line2)
	return line1 + "\n" + line2 + "\n" + dateLine
}

// minMax returns the smallest and largest value in vs.
func minMax(vs []float64) (min, max float64) {
	min, max = vs[0], vs[0]
	for _, v := range vs {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// graphDateLine renders the first/last day (short, localized) under the
// graph: first day left-aligned, last day right-aligned, padded to the
// width of the wider of the two sparkline lines above it so it reads as
// one aligned block instead of a ragged line.
func graphDateLine(firstDay, lastDay string, lang i18n.Lang, line1, line2 string) string {
	first := render.FormatShortDateLang(firstDay, lang)
	last := render.FormatShortDateLang(lastDay, lang)
	total := lipgloss.Width(line1)
	if w := lipgloss.Width(line2); w > total {
		total = w
	}
	gap := total - len([]rune(first)) - len([]rune(last))
	if gap < 1 {
		gap = 1
	}
	return first + strings.Repeat(" ", gap) + last
}

func (m tuiModel) prefixBody(width int) string {
	kind, ok := m.currentAgent()
	if !ok {
		return i18n.T(m.lang, "tui.no_agents_locally")
	}
	r := m.renderer()
	for _, a := range m.prefixResults {
		if a.Agent != string(kind) {
			continue
		}
		if a.Skipped {
			return r.Muted(i18n.T(m.lang, "tui.skipped_prefix", a.Agent) + "\n\n" + a.Reason)
		}
		if r.Colored() {
			return r.RenderCard(prefixCard(a, m.lang), width)
		}
		var sb strings.Builder
		renderPrefixText(&sb, m.env, []AgentPrefixResult{a})
		return sb.String()
	}
	return i18n.T(m.lang, "tui.no_data", string(kind))
}

func (m tuiModel) ttlBody(width int) string {
	kind, ok := m.currentAgent()
	if !ok {
		return i18n.T(m.lang, "tui.no_agents_locally")
	}
	r := m.renderer()
	for _, a := range m.ttlResults {
		if a.Agent != string(kind) {
			continue
		}
		if a.Skipped {
			return r.Muted(i18n.T(m.lang, "tui.skipped_prefix", a.Agent) + "\n\n" + a.Reason)
		}
		if r.Colored() {
			return r.RenderCard(ttlCard(a, m.lang), width)
		}
		var sb strings.Builder
		renderTTLText(&sb, m.env, []AgentTTLResult{a})
		return sb.String()
	}
	return i18n.T(m.lang, "tui.no_data", string(kind))
}

func (m tuiModel) fixBody() string {
	if m.confirming {
		return m.confirmForm.View()
	}
	if m.fixResultLine != "" {
		return m.fixResultLine
	}
	if !m.fixHasFinding {
		return i18n.T(m.lang, "fix.nothing_to_fix")
	}
	f := m.fixFinding
	return i18n.T(m.lang, "fix.found_header_tui", f.Agent, f.Description, f.Key, f.OldValue, f.Key, f.NewValue, f.File)
}

func (m tuiModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	var sb strings.Builder
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(m.vp.View())
	sb.WriteString("\n")
	sb.WriteString(m.renderFooter())
	return tea.NewView(sb.String())
}

func (m tuiModel) renderHeader() string {
	r := m.renderer()
	accent, _, _ := r.ChromeColors()
	var tabsLine strings.Builder
	for t := tuiTab(0); t < tabCount; t++ {
		name := i18n.T(m.lang, t.catalogKey())
		label := name
		if t == m.tab {
			if r.Colored() {
				label = lipgloss.NewStyle().Bold(true).Foreground(accent).Render("[" + name + "]")
			} else {
				label = "[" + name + "]"
			}
		}
		// The btop-style key digit sits right next to the name, but
		// outside the active-tab brackets: "3 [TTL]", not "[3 TTL]".
		label = fmt.Sprintf("%d %s", t.keyDigit(), label)
		tabsLine.WriteString(label)
		if t < tabCount-1 {
			tabsLine.WriteString("  ")
		}
	}

	agentLabel := i18n.T(m.lang, "tui.no_agents")
	if kind, ok := m.currentAgent(); ok {
		if m.tab == tabDashboard && m.allAgents {
			agentLabel = i18n.T(m.lang, "tui.agent_label_all")
		} else {
			agentLabel = i18n.T(m.lang, "tui.agent_label", kind, m.agentIdx+1, len(m.detected))
		}
	}
	days := windowDays[m.dayIdx]
	title := i18n.T(m.lang, "tui.header_title", Version)
	second := fmt.Sprintf("%s    %s    %s", title, agentLabel, i18n.T(m.lang, "tui.window", days))
	if fresh := m.freshness(); fresh != "" {
		second += "    " + fresh
	}

	header := tabsLine.String() + "\n" + second
	// Once the Dashboard tab has loaded, its "Mesure" panel already
	// lists every ignored agent and why (dashboardData.IgnoredNote):
	// repeating it here would just be the same note twice. Before the
	// first load completes the Dashboard body is still the loading
	// spinner, so the header note is the only place a user sees it.
	suppressed := m.tab == tabDashboard && !m.lastLoaded.IsZero()
	if len(m.skipped) > 0 && !suppressed {
		header += "\n" + r.Muted(i18n.T(m.lang, "tui.skipped_prefix", m.skippedNames()))
	}
	return header
}

// freshness renders "updated Ns/Nm ago" from m.lastLoaded, or "" before
// the first load has completed (there is nothing to report yet).
func (m tuiModel) freshness() string {
	if m.lastLoaded.IsZero() {
		return ""
	}
	elapsed := m.now().Sub(m.lastLoaded)
	var ago string
	switch {
	case elapsed < time.Minute:
		ago = fmt.Sprintf("%ds", int(elapsed.Seconds()))
	default:
		ago = fmt.Sprintf("%dm", int(elapsed.Minutes()))
	}
	return i18n.T(m.lang, "tui.updated_ago", ago)
}

// skippedNames joins m.skipped's kinds, in order, for the header's dim
// one-line note ("skipped: codex, gemini-cli").
func (m tuiModel) skippedNames() string {
	names := make([]string, len(m.skipped))
	for i, s := range m.skipped {
		names[i] = string(s.Kind)
	}
	return strings.Join(names, ", ")
}

func (m tuiModel) renderFooter() string {
	if !m.fullHelp {
		return m.help.ShortHelpView(m.keys.ShortHelp())
	}
	view := m.help.FullHelpView(m.keys.FullHelp())
	width := m.width
	if width <= 0 {
		width = cardWidth(m.env)
	}
	var sb strings.Builder
	sb.WriteString(view)
	// The Dashboard tab's "Efficacite" panel score is a formula, not a
	// raw measurement: document it in full help (card: "documented in
	// `?` help"), the same place skipped-agent reasons already live.
	if m.tab == tabDashboard {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Width(width).Render(i18n.T(m.lang, "dashboard.efficiency_help")))
	}
	for _, s := range m.skipped {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s: %s", s.Kind, s.Reason)))
	}
	return sb.String()
}

// ttyColorProfile picks the color profile runTUI forces on the
// underlying tea.Program (card point 2: "color in tmux"), honoring the
// exact same IsTTY/NoColor decision render.Renderer.Colored() already
// makes elsewhere, instead of letting bubbletea infer it purely from
// its own isatty check on p.output -- which can disagree with the
// IsTTY this process already established once in main.go (e.g. under
// tmux/ssh pty forwarding), silently stripping every color the
// renderer embedded. NoColor or a non-TTY both collapse to NoTTY
// (colorprofile.Writer's Profile<=NoTTY strips every ANSI escape, not
// just colors -- "NO_COLOR=1 -> none" per the card). environ is the
// real process environment (TERM, COLORTERM, TMUX...), passed
// separately from env so tests can force it without real env vars.
func ttyColorProfile(env Env, environ []string) colorprofile.Profile {
	if !env.IsTTY || env.NoColor {
		return colorprofile.NoTTY
	}
	return colorprofile.Env(environ)
}

// runTUI starts the interactive app: launched only when `ats` is run bare
// (no args) with stdin and stdout both a TTY (see Run in cliapp.go).
func runTUI(stdout, stderr io.Writer, env Env) int {
	roots, err := agents.DefaultRoots(env.HomeDir)
	if err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	profile := ttyColorProfile(env, os.Environ())
	p := tea.NewProgram(newTUIModel(env, roots), tea.WithColorProfile(profile))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	return 0
}
