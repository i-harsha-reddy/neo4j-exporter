package probe

import (
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestDurSeconds(t *testing.T) {
	const eps = 1e-9
	tests := []struct {
		name string
		in   any
		want float64
	}{
		// /slow-queries returns the raw elapsedTime/cpuTime/waitTime Cypher
		// Durations, surfaced by the v5 driver as neo4j.Duration. Without an
		// explicit case these coerced to 0, zeroing every elapsed-based panel.
		{"neo4j.Duration seconds+nanos", neo4j.Duration{Seconds: 12, Nanos: 865000000}, 12.865},
		{"neo4j.Duration sub-second", neo4j.Duration{Nanos: 66000000}, 0.066},
		{"neo4j.Duration with days", neo4j.Duration{Days: 1, Seconds: 1}, 86401},
		{"neo4j.Duration zero", neo4j.Duration{}, 0},
		{"go time.Duration", 2 * time.Second, 2},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durSeconds(tt.in)
			if d := got - tt.want; d > eps || d < -eps {
				t.Fatalf("durSeconds(%#v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
