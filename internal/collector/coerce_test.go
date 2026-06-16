package collector

import (
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestAsFloat64Duration(t *testing.T) {
	const eps = 1e-9
	tests := []struct {
		name string
		in   any
		want float64
	}{
		// SHOW TRANSACTIONS YIELD elapsedTime/cpuTime/... come back as a Cypher
		// Duration, which the v5 driver surfaces as neo4j.Duration (NOT Go's
		// time.Duration). These pin the seconds conversion.
		{"neo4j.Duration seconds+nanos", neo4j.Duration{Seconds: 12, Nanos: 865000000}, 12.865},
		{"neo4j.Duration whole seconds", neo4j.Duration{Seconds: 5}, 5},
		{"neo4j.Duration sub-second", neo4j.Duration{Nanos: 66000000}, 0.066},
		{"neo4j.Duration with days", neo4j.Duration{Days: 1, Seconds: 1}, 86401},
		{"neo4j.Duration zero", neo4j.Duration{}, 0},
		// Other types the helper already handled.
		{"go time.Duration", 1500 * time.Millisecond, 1.5},
		{"int64", int64(7), 7},
		{"float64", 3.5, 3.5},
		{"string", "2.25", 2.25},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asFloat64(tt.in)
			if d := got - tt.want; d > eps || d < -eps {
				t.Fatalf("asFloat64(%#v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
