package collector

import (
	"strconv"
	"strings"
	"time"
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
