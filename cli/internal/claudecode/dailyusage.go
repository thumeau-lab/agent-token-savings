package claudecode

import (
	"path/filepath"
	"sort"
)

// DailyRow is one calendar day's aggregated token totals across every
// Claude Code session that had activity that day, for the Cache panel's
// per-day history graph. Built by DailyUsage from the exact same parsed
// rows Aggregate uses (ParseSessionFile): no second parser, just a
// different grouping of the same data.
type DailyRow struct {
	Day    string
	Create int64
	Read   int64
	Input  int64
	Output int64
}

// DailyUsage walks root/projects/*/*.jsonl (same glob as Aggregate),
// parses each file with ParseSessionFile, and buckets every row by its
// calendar day (the first 10 characters of its RFC3339 timestamp, e.g.
// "2026-10-11T10:00:00Z" -> "2026-10-11"). A row with no timestamp (some
// older transcripts omit it) is skipped: there is no day to bucket it
// under. Returned sorted by day, oldest first.
func DailyUsage(root string) ([]DailyRow, error) {
	paths, err := filepath.Glob(filepath.Join(root, "projects", "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}

	buckets := map[string]*DailyRow{}
	for _, p := range paths {
		rows, err := ParseSessionFile(p)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if len(row.TS) < 10 {
				continue
			}
			day := row.TS[:10]
			b, ok := buckets[day]
			if !ok {
				b = &DailyRow{Day: day}
				buckets[day] = b
			}
			b.Create += row.Create
			b.Read += row.Read
			b.Input += row.Input
			b.Output += row.Output
		}
	}

	out := make([]DailyRow, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out, nil
}
