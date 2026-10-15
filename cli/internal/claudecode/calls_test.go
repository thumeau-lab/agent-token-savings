package claudecode

import "testing"
import "time"

func TestAggregateCountsTotalAPICallsAcrossAllSessions(t *testing.T) {
	root := t.TempDir()
	writeSession(t, root, "proj-a", "one.jsonl", [][4]int64{
		{1, 5, 0, 30000},
		{1, 5, 30000, 900},
	})
	writeSession(t, root, "proj-b", "two.jsonl", [][4]int64{
		{1, 5, 0, 30000},
	})

	rep, err := Aggregate(root, 36600, time.Now())
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if rep.Calls != 3 {
		t.Fatalf("Calls = %d, want 3 (2 + 1 across both sessions)", rep.Calls)
	}
}
