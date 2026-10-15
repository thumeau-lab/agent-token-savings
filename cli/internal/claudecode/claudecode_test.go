package claudecode

import (
	"os"
	"path/filepath"
	"testing"
)

func writeJSONL(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseSessionFileDedupsStreamingEvents(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}`,
		// streaming duplicate: must be ignored, same as the Python fixture in tests/test_tools.py
		`{"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}`,
		`{"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":30000,"cache_creation_input_tokens":900}}}`,
		`not json`,
	})

	rows, err := ParseSessionFile(path)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	var createSum int64
	for _, r := range rows {
		createSum += r.Create
	}
	if createSum != 30900 {
		t.Fatalf("createSum = %d, want 30900", createSum)
	}
}
