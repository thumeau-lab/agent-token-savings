# ats demo on fixture data (no real usage data, no secrets). Regenerated on
# every tag by .github/workflows/release.yml. Run locally with:
#   vhs cli/demo.tape
Output docs/img/ats-demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1000
Set Height 560
Set Padding 20
Set TypingSpeed 40ms

Env HOME /tmp/ats-demo-home
Env NO_COLOR ""

Hide
Type "mkdir -p /tmp/ats-demo-home/.claude/projects/demo"
Enter
Type "printf '%s\n' '{\"message\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":5,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":30000}}}' '{\"message\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":5,\"cache_read_input_tokens\":30000,\"cache_creation_input_tokens\":900}}}' > /tmp/ats-demo-home/.claude/projects/demo/a.jsonl"
Enter
Show

Type "ats cache"
Enter
Sleep 2s

Type "ats prefix"
Enter
Sleep 2s
