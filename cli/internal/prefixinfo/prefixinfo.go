// Package prefixinfo estimates what makes up an agent's cached prompt
// prefix, from files readable offline: memory files (CLAUDE.md, AGENTS.md),
// the skills and agents index (name+description of each skill/subagent,
// the part actually injected per turn), and configured MCP servers.
//
// This is a static, local estimate, not a live measurement of the model's
// actual prompt: built-in tool schemas and the base system prompt are
// baked into each agent's binary and cannot be sized without a live call
// (see bench/claude_code_levers.py and docs/levers.md for the measured
// reference numbers). That part is reported separately via
// Report.BuiltinTokens/BuiltinSource -- never silently as 0. Every ats
// output says "estimated" and the byte/4 token conversion explicitly.
package prefixinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
)

// Component is one named contributor to the cached prefix, sized in bytes.
type Component struct {
	Name  string
	Bytes int64
}

// Report is the full prefix composition breakdown for one agent.
type Report struct {
	Agent      string
	Components []Component
	TotalBytes int64

	// MCPServers is every distinct MCP server name found across every
	// source AnalyzeClaudeCode checks (project .mcp.json, project and
	// user .claude/settings.json, and the global ~/.claude.json --
	// both its top-level mcpServers and every project's own mcpServers
	// block), sorted and deduplicated. Names only: never the server's
	// command/args/env values. Empty (not nil) when none are
	// configured, so the card can say "0 MCP servers configured"
	// explicitly instead of treating absence as unmeasured.
	MCPServers []string

	// BuiltinTokens is the agent's base system prompt + built-in tool
	// schemas: baked into the binary, not sizeable from local files. It
	// is measured once, by a live bench script, and cited via
	// BuiltinSource -- never left at 0 when the agent has one.
	BuiltinTokens int64
	BuiltinSource string
}

// EstimatedTokens is TotalBytes / 4, the rough token estimate the card asks
// ats to use and to label explicitly in its output. It does NOT include
// BuiltinTokens: that part is reported as its own separate line (see the
// package doc comment), since it comes from a different, non-local source.
func (r Report) EstimatedTokens() int64 {
	return r.TotalBytes / 4
}

func (r *Report) add(name string, path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return
	}
	r.Components = append(r.Components, Component{Name: name, Bytes: fi.Size()})
	r.TotalBytes += fi.Size()
}

// ClaudeCodeDefaultPrefixTokens and ClaudeCodeNoToolsPrefixTokens are
// the cold-call prefix sizes measured by bench/claude_code_levers.py
// (see docs/levers.md section 2, "Claude Code (CLI 2.1.220)" table):
// default vs `--tools ""` (no built-in tools). The difference is what a
// local file scan can never see: the built-in tool schemas. Exported so
// cliapp's prefixVerdict can build its own localized citation sentence
// instead of embedding claudeCodeBuiltinSourceComment's English prose
// (the --json path keeps using that comment verbatim, since --json
// always stays English regardless of UI language).
const (
	ClaudeCodeDefaultPrefixTokens  = 28_362
	ClaudeCodeNoToolsPrefixTokens  = 9_072
	claudeCodeBuiltinToolsTokens   = ClaudeCodeDefaultPrefixTokens - ClaudeCodeNoToolsPrefixTokens
	claudeCodeBuiltinSourceComment = `bench/claude_code_levers.py, docs/levers.md: default prefix 28,362 tok vs 9,072 tok with --tools "" (tool schemas = 68% of the prefix)`
)

// AnalyzeClaudeCode builds the prefix composition for Claude Code: the
// project and user CLAUDE.md/AGENTS.md memory files found from cwd, the
// skills and agents index (name+description) from both user and project
// scope, every configured MCP server found in .mcp.json / ~/.claude.json
// (top-level AND every project's own mcpServers block) / settings.json,
// and the repo's measured built-in (not locally measurable) tool schema
// + system prompt figure.
func AnalyzeClaudeCode(roots agents.Roots, cwd string) (Report, error) {
	rep := Report{Agent: string(agents.ClaudeCode), MCPServers: []string{}}
	rep.add("CLAUDE.md", filepath.Join(cwd, "CLAUDE.md"))
	rep.add("AGENTS.md", filepath.Join(cwd, "AGENTS.md"))
	rep.add("CLAUDE.md (user)", filepath.Join(roots.ClaudeHome, "CLAUDE.md"))

	addSkillsIndex(&rep, "skills index",
		filepath.Join(roots.ClaudeHome, "skills"),
		filepath.Join(cwd, ".claude", "skills"))
	addSkillsIndex(&rep, "agents index",
		filepath.Join(roots.ClaudeHome, "agents"),
		filepath.Join(cwd, ".claude", "agents"))

	homeDir := filepath.Dir(roots.ClaudeHome) // roots.ClaudeHome is <home>/.claude
	addMCPServers(&rep, filepath.Join(cwd, ".mcp.json"))
	addMCPServers(&rep, filepath.Join(cwd, ".claude", "settings.json"))
	addMCPServers(&rep, filepath.Join(roots.ClaudeHome, "settings.json"))
	addClaudeJSON(&rep, filepath.Join(homeDir, ".claude.json"))
	rep.MCPServers = dedupSorted(rep.MCPServers)

	rep.BuiltinTokens = claudeCodeBuiltinToolsTokens
	rep.BuiltinSource = claudeCodeBuiltinSourceComment

	return rep, nil
}

// hermesMemoryFiles are Hermes' per-install memory/identity files: name
// (the Component shown on the card) and path relative to HermesHome.
// MEMORY.md and USER.md are persistent-memory files the agent writes
// through its memory tool under $HERMES_HOME/memories/ (see
// hermes-agent.nousresearch.com/docs/user-guide/features/memory);
// SOUL.md is a project/identity context file read directly from
// $HERMES_HOME, not from memories/ (docs/levers.md already measures it
// there).
var hermesMemoryFiles = []struct{ name, rel string }{
	{"MEMORY.md", filepath.Join("memories", "MEMORY.md")},
	{"USER.md", filepath.Join("memories", "USER.md")},
	{"SOUL.md", "SOUL.md"},
}

// hermesConfigBlockKeys are the top-level config.yaml keys ats knows how
// to size as their own component instead of lumping them into the rest of
// the file. "mcp_servers" is confirmed (references/native-mcp.md); ats has
// no YAML parser and no confirmed single key for "toolsets" (enabled
// per-platform, see references/configuration.md), so that stays folded
// into the generic "toolsets (config.yaml)" remainder below instead of
// being invented.
var hermesConfigBlockKeys = []string{"mcp_servers"}

// AnalyzeHermes builds the prefix composition for Hermes Agent: its
// memory/identity files, the skills index, and config.yaml split into its
// mcp_servers block (confirmed key, references/native-mcp.md) versus the
// rest of the file (settings and per-platform toolsets).
func AnalyzeHermes(roots agents.Roots) (Report, error) {
	rep := Report{Agent: string(agents.Hermes)}
	for _, f := range hermesMemoryFiles {
		rep.add(f.name, filepath.Join(roots.HermesHome, f.rel))
	}
	addSkillsIndex(&rep, "skills index", filepath.Join(roots.HermesHome, "skills"))
	addHermesConfig(&rep, filepath.Join(roots.HermesHome, "config.yaml"))
	return rep, nil
}

// addSkillsIndex sums the "name: ...\ndescription: ...\n" frontmatter lines
// of every <dir>/*/SKILL.md or <dir>/*.md under each given dir: that's the
// part actually injected into the system prompt per skill/agent
// (docs/levers.md "Skills index"), not the full body. Multiple dirs (user
// + project scope) are merged into one component.
func addSkillsIndex(rep *Report, componentName string, dirs ...string) {
	var total int64
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*", "SKILL.md"))
		flat, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		matches = append(matches, flat...)
		for _, m := range matches {
			raw, err := os.ReadFile(m)
			if err != nil {
				continue
			}
			total += int64(len(frontmatterIndexLines(string(raw))))
		}
	}
	if total == 0 {
		return
	}
	rep.Components = append(rep.Components, Component{Name: componentName, Bytes: total})
	rep.TotalBytes += total
}

// frontmatterIndexLines extracts the "name:" and "description:" lines from a
// skill's YAML frontmatter (between the two "---" markers).
func frontmatterIndexLines(skillMD string) string {
	lines := strings.Split(skillMD, "\n")
	inFrontmatter := false
	var out strings.Builder
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if i == 0 {
				inFrontmatter = true
				continue
			}
			if inFrontmatter {
				break
			}
			continue
		}
		if !inFrontmatter {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") || strings.HasPrefix(trimmed, "description:") {
			out.WriteString(trimmed)
			out.WriteString("\n")
		}
	}
	return out.String()
}

// addMCPServers reads a .mcp.json (or ~/.claude/settings.json, same
// {"mcpServers": {...}} shape -- confirmed identical across these files)
// and adds one component per server, sized as its own serialized JSON
// entry (a proxy for the tool schemas that server would register -- the
// real wire size needs a live call, see the package doc comment), plus
// its name (never its command/args/env values) to rep.MCPServers.
func addMCPServers(rep *Report, mcpConfigPath string) {
	raw, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		return
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return
	}
	addServerComponents(rep, cfg.MCPServers)
}

// addClaudeJSON reads the global ~/.claude.json: its top-level
// "mcpServers" (same shape as addMCPServers) AND every project's own
// "mcpServers" block under its "projects": {<path>: {...}} map, since
// Claude Code also lets a server be configured per-project there instead
// of in a project-local .mcp.json.
func addClaudeJSON(rep *Report, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Projects   map[string]struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return
	}
	addServerComponents(rep, cfg.MCPServers)
	projectPaths := make([]string, 0, len(cfg.Projects))
	for p := range cfg.Projects {
		projectPaths = append(projectPaths, p)
	}
	sort.Strings(projectPaths)
	for _, p := range projectPaths {
		addServerComponents(rep, cfg.Projects[p].MCPServers)
	}
}

// addServerComponents adds one "mcp: <name>" byte-sized Component per
// server in servers (sorted for determinism) and appends each name
// (names only) to rep.MCPServers.
func addServerComponents(rep *Report, servers map[string]json.RawMessage) {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := int64(len(servers[name]))
		rep.Components = append(rep.Components, Component{Name: "mcp: " + name, Bytes: n})
		rep.TotalBytes += n
		rep.MCPServers = append(rep.MCPServers, name)
	}
}

// dedupSorted returns names deduplicated and sorted, so a server
// configured in more than one source (e.g. both .mcp.json and
// ~/.claude.json) is only counted once in the "N MCP servers configured"
// summary.
func dedupSorted(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// yamlBlockRE matches a bare top-level-or-nested "key:" line (no inline
// scalar value), used to find where a named YAML block starts.
var yamlBlockRE = regexp.MustCompile(`^(\s*)([A-Za-z0-9_-]+):\s*(#.*)?$`)

// yamlBlockBytes extracts every block in raw whose key matches keyName
// (e.g. "mcp_servers:"), where a block is the key line plus every
// following line that is blank or more indented than the key. Returns the
// combined byte size of all such blocks (ats has no YAML parser; this is
// a line-based proxy, same spirit as fixer.go's single-line edits).
func yamlBlockBytes(raw []byte, keyName string) int64 {
	lines := strings.Split(string(raw), "\n")
	var total int64
	i := 0
	for i < len(lines) {
		m := yamlBlockRE.FindStringSubmatch(lines[i])
		if m == nil || m[2] != keyName {
			i++
			continue
		}
		baseIndent := len(m[1])
		blockLen := len(lines[i]) + 1 // +1 for the newline consumed by Split
		j := i + 1
		for j < len(lines) {
			line := lines[j]
			trimmed := strings.TrimLeft(line, " \t")
			indent := len(line) - len(trimmed)
			if trimmed != "" && indent <= baseIndent {
				break
			}
			blockLen += len(line) + 1
			j++
		}
		total += int64(blockLen)
		i = j
	}
	return total
}

// hermesConfigBlockNames maps a hermesConfigBlockKeys entry to the
// display name shown for its component: the distinguishing word first
// ("MCP servers (config.yaml)"), not "config.yaml (mcp_servers)" --
// the latter made two config.yaml rows collide under the label column's
// old fixed truncation width (both share the "config.yaml " prefix).
var hermesConfigBlockNames = map[string]string{
	"mcp_servers": "MCP servers (config.yaml)",
}

// addHermesConfig splits config.yaml into its mcp_servers block (confirmed
// key) and everything else (settings, per-platform toolsets -- no single
// confirmed YAML key, see hermesConfigBlockKeys), as two components that
// sum back to the whole file (no double-counting).
func addHermesConfig(rep *Report, configPath string) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var blocksTotal int64
	for _, key := range hermesConfigBlockKeys {
		n := yamlBlockBytes(raw, key)
		if n <= 0 {
			continue
		}
		rep.Components = append(rep.Components, Component{Name: hermesConfigBlockNames[key], Bytes: n})
		rep.TotalBytes += n
		blocksTotal += n
	}
	rest := int64(len(raw)) - blocksTotal
	if rest > 0 {
		rep.Components = append(rep.Components, Component{Name: "toolsets (config.yaml)", Bytes: rest})
		rep.TotalBytes += rest
	}
}
