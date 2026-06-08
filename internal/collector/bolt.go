package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// BoltCollector measures Bolt connectivity and round-trip latency. It is
// also the SOURCE of the neo4j_up gauge — orchestrator reads ProbeContext.Up()
// after all collectors complete, and bolt is responsible for calling
// SignalUp / MarkDown.
type BoltCollector struct{}

func (BoltCollector) Name() string { return "bolt" }

func (BoltCollector) Collect(ctx context.Context, p *ProbeContext) error {
	handshake := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "neo4j_bolt_handshake_duration_seconds",
		Help: "Time to verify Bolt connectivity (driver.VerifyConnectivity).",
	})
	rtt := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "neo4j_bolt_query_round_trip_seconds",
		Help: "Time to execute `RETURN 1` over Bolt (excludes handshake).",
	})
	MustRegister(p.Registry, handshake)
	MustRegister(p.Registry, rtt)

	t0 := time.Now()
	if err := p.Entry.VerifyConnectivity(ctx); err != nil {
		p.MarkDown()
		return err
	}
	handshake.Set(time.Since(t0).Seconds())

	t1 := time.Now()
	if _, err := p.Entry.ExecRead(ctx, "", "RETURN 1 AS x", nil); err != nil {
		p.MarkDown()
		return err
	}
	rtt.Set(time.Since(t1).Seconds())

	p.SignalUp()
	return nil
}
