package collector

import (
	"testing"
	"time"
)

func TestAsUnixSeconds(t *testing.T) {
	// apoc.monitor.kernel on Neo4j 5.26.x / current APOC Extended returns
	// kernelStartTime / storeCreationDate as "2006-01-02 15:04:05" strings.
	wantUTC := float64(time.Date(2026, 6, 8, 9, 48, 4, 0, time.UTC).Unix())

	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"apoc date string (space)", "2026-06-08 09:48:04", wantUTC},
		{"apoc date string trimmed", "  2026-06-08 09:48:04  ", wantUTC},
		{"iso date string (T)", "2026-06-08T09:48:04", wantUTC},
		{"rfc3339", "2026-06-08T09:48:04Z", wantUTC},
		{"ms-since-epoch float", 1780912084455.0, 1780912084.455},
		{"seconds float passthrough", 1780912084.0, 1780912084.0},
		{"numeric string (ms)", "1780912084455", 1780912084.455},
		{"time.Time", time.Date(2026, 6, 8, 9, 48, 4, 0, time.UTC), wantUTC},
		{"nil", nil, 0},
		{"empty string", "", 0},
		{"whitespace string", "   ", 0},
		{"unparseable string", "not-a-date", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := asUnixSeconds(c.in); got != c.want {
				t.Errorf("asUnixSeconds(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
