package collector

import (
	"strconv"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// asFloat64 coerces a Cypher result value to float64. Used by collectors
// that read numeric columns. Returns 0 for nil. Treats Neo4j Duration as
// total seconds (we never need finer than that).
func asFloat64(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case time.Duration:
		return x.Seconds()
	case neo4j.Duration:
		// Cypher Duration (elapsedTime/cpuTime/waitTime/idleTime from SHOW
		// TRANSACTIONS) — the v5 driver surfaces it as neo4j.Duration, NOT Go's
		// time.Duration. Convert to total seconds. Months never occur for the
		// sub-day timings we read; use Neo4j's average-month seconds for safety.
		return float64(x.Months)*2629746 + float64(x.Days)*86400 + float64(x.Seconds) + float64(x.Nanos)/1e9
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	}
	return 0
}

// asString coerces a value to a string for label use.
func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.(bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return strings.TrimSpace(fmtAny(v))
}

func fmtAny(v any) string {
	switch x := v.(type) {
	case fmt_Stringer:
		return x.String()
	default:
		return ""
	}
}

type fmt_Stringer interface{ String() string }
