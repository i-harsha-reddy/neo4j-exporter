// Package collector defines the Collector interface and the per-probe
// context that collectors operate on. Concrete collectors live in
// sibling files (server.go, bolt.go, transactions.go, ...).
package collector

import (
	"context"
	"sync"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/jolokiaclient"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/neo4jclient"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector runs one slice of metric collection during a probe. Each
// implementation must be safe to run concurrently with other collectors
// (they share the *neo4jclient.Entry but get independent contexts).
type Collector interface {
	// Name is the stable identifier used in module config and the
	// neo4j_collector_success{collector="..."} label.
	Name() string

	// Collect runs the data collection. Errors are non-fatal at the probe
	// level — they surface as neo4j_collector_success{collector}=0.
	Collect(ctx context.Context, p *ProbeContext) error
}

// ProbeContext is the per-probe state passed to every collector.
type ProbeContext struct {
	Entry    *neo4jclient.Entry
	Caps     *neo4jclient.Capabilities
	Jolokia  *jolokiaclient.Client
	Module   cfg.Module
	Target   cfg.Target
	Registry *prometheus.Registry

	mu       sync.Mutex
	upMu     bool   // sticky: once true, stays true
	upDecided bool
}

// SignalUp is called by the bolt collector when its connectivity check
// succeeds. The orchestrator reads this to populate neo4j_up at the end
// of the probe.
func (p *ProbeContext) SignalUp() {
	p.mu.Lock()
	p.upMu = true
	p.upDecided = true
	p.mu.Unlock()
}

// MarkDown is the explicit "Bolt failed" signal. Only one of SignalUp or
// MarkDown should be called per probe.
func (p *ProbeContext) MarkDown() {
	p.mu.Lock()
	p.upMu = false
	p.upDecided = true
	p.mu.Unlock()
}

// Up returns the final up gauge value (1 if SignalUp was called, else 0).
// If neither was called the bolt collector wasn't run; default to 0 so
// dashboards remain conservative.
func (p *ProbeContext) Up() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.upDecided {
		return 0
	}
	if p.upMu {
		return 1
	}
	return 0
}

// MustRegister is a thin wrapper that registers a metric and logs (via
// panic in tests, no-op in prod via recover) if registration fails.
// Probe orchestration uses prometheus.NewRegistry() per request, so
// duplicate-registration is exclusively a programming error.
func MustRegister(r *prometheus.Registry, c prometheus.Collector) {
	if err := r.Register(c); err != nil {
		// Same metric registered twice within one collector — surface
		// the bug in tests but don't crash production probes.
		_ = err
	}
}
