package prefixinfo

import (
	"path/filepath"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
)

// TestAnalyzeHermesSumsMemoryFiles verifies MEMORY.md and USER.md --
// Hermes' persistent-memory files, written by the agent through the
// memory tool under $HERMES_HOME/memories/ (see
// hermes-agent.nousresearch.com/docs/user-guide/features/memory) -- and
// SOUL.md -- the identity/persona context file, read directly from
// $HERMES_HOME -- each contribute their byte size, per the card's
// explicit list.
func TestAnalyzeHermesSumsMemoryFiles(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	writeFile(t, filepath.Join(roots.HermesHome, "memories", "MEMORY.md"), "memory content")
	writeFile(t, filepath.Join(roots.HermesHome, "memories", "USER.md"), "user content!!")
	writeFile(t, filepath.Join(roots.HermesHome, "SOUL.md"), "soul")

	rep, err := AnalyzeHermes(roots)
	if err != nil {
		t.Fatalf("AnalyzeHermes: %v", err)
	}
	want := map[string]int64{"MEMORY.md": 14, "USER.md": 14, "SOUL.md": 4}
	for _, c := range rep.Components {
		if wantBytes, ok := want[c.Name]; ok {
			if c.Bytes != wantBytes {
				t.Errorf("%s bytes = %d, want %d", c.Name, c.Bytes, wantBytes)
			}
			delete(want, c.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing components in %+v: %+v", rep.Components, want)
	}
}

// TestAnalyzeHermesSplitsConfigIntoMCPServersAndRest verifies config.yaml
// is split into its mcp_servers block (confirmed key,
// references/native-mcp.md) and the rest of the file, with no
// double-counting: the two components sum back to the file's total size.
func TestAnalyzeHermesSplitsConfigIntoMCPServersAndRest(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	config := "model:\n  default: opus\n" +
		"mcp_servers:\n" +
		"  time:\n" +
		"    command: \"uvx\"\n" +
		"    args: [\"mcp-server-time\"]\n" +
		"display:\n  skin: default\n"
	writeFile(t, filepath.Join(roots.HermesHome, "config.yaml"), config)

	rep, err := AnalyzeHermes(roots)
	if err != nil {
		t.Fatalf("AnalyzeHermes: %v", err)
	}
	var mcp, rest *Component
	for i := range rep.Components {
		switch rep.Components[i].Name {
		case "MCP servers (config.yaml)":
			mcp = &rep.Components[i]
		case "toolsets (config.yaml)":
			rest = &rep.Components[i]
		}
	}
	if mcp == nil || rest == nil {
		t.Fatalf("missing config.yaml components in %+v", rep.Components)
	}
	if mcp.Bytes+rest.Bytes != int64(len(config)) {
		t.Fatalf("mcp(%d) + rest(%d) = %d, want file size %d", mcp.Bytes, rest.Bytes, mcp.Bytes+rest.Bytes, len(config))
	}
	if mcp.Bytes <= 0 {
		t.Fatalf("mcp_servers block bytes = %d, want > 0", mcp.Bytes)
	}
}

// TestAnalyzeHermesSumsSkillsIndex verifies Hermes' skills index
// (name+description frontmatter under $HERMES_HOME/skills) is measured
// the same way as Claude Code's.
func TestAnalyzeHermesSumsSkillsIndex(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	skill := "---\nname: demo\ndescription: Demo skill.\n---\nBody not counted.\n"
	writeFile(t, filepath.Join(roots.HermesHome, "skills", "demo", "SKILL.md"), skill)

	rep, err := AnalyzeHermes(roots)
	if err != nil {
		t.Fatalf("AnalyzeHermes: %v", err)
	}
	var got *Component
	for i := range rep.Components {
		if rep.Components[i].Name == "skills index" {
			got = &rep.Components[i]
		}
	}
	if got == nil {
		t.Fatalf("no 'skills index' component in %+v", rep.Components)
	}
	want := int64(len("name: demo\ndescription: Demo skill.\n"))
	if got.Bytes != want {
		t.Fatalf("skills index bytes = %d, want %d", got.Bytes, want)
	}
}
