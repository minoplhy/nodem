package sqlite

import (
	"database/sql"
	"time"
)

var timeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

// parseSQLiteTime attempts to parse a timestamp string from SQLite in various formats.
func parseSQLiteTime(s string) (time.Time, error) {
	for _, layout := range timeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Parse(time.RFC3339, s)
}

// scanNullTime parses an sql.NullString into a *time.Time.
func scanNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := parseSQLiteTime(ns.String)
	if err != nil {
		return nil
	}
	return &t
}
