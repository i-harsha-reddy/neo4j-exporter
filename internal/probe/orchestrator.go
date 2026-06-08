// Package probe implements the multi-target probe HTTP handler and its
// orchestrator. One probe = one /probe?target=...&module=... request.
package probe

import (
	"context"
	"sync"
	"time"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/collector"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/jolokiaclient"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/neo4jclient"
	"github.com/prometheus/client_golang/prometheus"
)

// Orchestrator runs all enabled collectors for one probe in parallel,
// each with its own per-collector timeout. Partial failures are recorded
// but do not abort the probe.
type Orchestrator struct {
	Collectors map[string]collector.Collector
}

// Run executes the orchestrator. Returns when all collectors finish or
// the parent context is canceled. The supplied registry receives all
// metrics including the per-orchestrator `neo4j_collector_*` gauges.
//
// Caller is expected to register `neo4j_up` and `neo4j_scrape_duration_seconds`
// gauges on the same registry after Run returns.
func (o *Orchestrator) Run(
	ctx context.Context,
	pool *neo4jclient.Pool,
	target cfg.Target,
	module cfg.Module,
	registry *prometheus.Registry,
) (*ProbeResult, error) {
	result := &ProbeResult{Started: time.Now()}

	successGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_collector_success",
		Help: "1 if the named collector succeeded for this probe, else 0.",
	}, []string{"collector"})
	durGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_collector_duration_seconds",
		Help: "Duration of the named collector for this probe.",
	}, []string{"collector"})
	collector.MustRegister(registry, successGauge)
	collector.MustRegister(registry, durGauge)

	entry, err := pool.Get(target)
	if err != nil {
		// Couldn't even build a driver — entire probe fails.
		successGauge.WithLabelValues("driver").Set(0)
		result.DriverError = err
		return result, err
	}

	// Probe APOC capabilities once per probe (cached on the entry across
	// probes). This dictates which APOC collectors run.
	capCtx, capCancel := context.WithTimeout(ctx, 5*time.Second)
	caps, _ := entry.Capabilities(capCtx)
	capCancel()
	if caps == nil {
		caps = &neo4jclient.Capabilities{}
	}

	var jclient *jolokiaclient.Client
	if target.Jolokia != nil {
		jto := module.PerCollectorTimeout["jolokia"]
		if jto == 0 {
			jto = 3 * time.Second
		}
		jc, err := jolokiaclient.New(target.Jolokia, jto)
		if err == nil {
			jclient = jc
		}
	}

	pctx := &collector.ProbeContext{
		Entry:    entry,
		Caps:     caps,
		Jolokia:  jclient,
		Module:   module,
		Target:   target,
		Registry: registry,
	}

	enabled := o.selectCollectors(module, caps, target)
	var wg sync.WaitGroup
	for _, c := range enabled {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			to := module.PerCollectorTimeout[c.Name()]
			if to == 0 {
				to = module.Timeout
				if to == 0 {
					to = 5 * time.Second
				}
			}
			cctx, cancel := context.WithTimeout(ctx, to)
			defer cancel()

			start := time.Now()
			err := c.Collect(cctx, pctx)
			elapsed := time.Since(start).Seconds()

			durGauge.WithLabelValues(c.Name()).Set(elapsed)
			if err != nil {
				successGauge.WithLabelValues(c.Name()).Set(0)
				result.recordError(c.Name(), err)
			} else {
				successGauge.WithLabelValues(c.Name()).Set(1)
			}
		}()
	}
	wg.Wait()

	result.Up = pctx.Up()
	result.Finished = time.Now()
	return result, nil
}

// selectCollectors filters the registered collectors by:
//   1. The module's `collectors:` toggle (true / false / auto)
//   2. For `auto` toggles, the actual capabilities probed at startup
//   3. For `jolokia`, whether the target has a jolokia block configured
func (o *Orchestrator) selectCollectors(m cfg.Module, caps *neo4jclient.Capabilities, t cfg.Target) []collector.Collector {
	out := make([]collector.Collector, 0, len(o.Collectors))
	for name, c := range o.Collectors {
		mode, set := m.Collectors[name]
		if !set || mode == cfg.Disabled {
			continue
		}
		switch name {
		case "apoc_kernel":
			if mode == cfg.Auto && !caps.HasAPOCMonitorKernel {
				continue
			}
		case "apoc_store":
			if mode == cfg.Auto && !caps.HasAPOCMonitorStore {
				continue
			}
		case "apoc_tx":
			if mode == cfg.Auto && !caps.HasAPOCMonitorTx {
				continue
			}
		case "apoc_ids":
			if mode == cfg.Auto && !caps.HasAPOCMonitorIds {
				continue
			}
		case "apoc_meta":
			if mode == cfg.Auto && !caps.HasAPOCMetaStats {
				continue
			}
		case "jolokia":
			if t.Jolokia == nil {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// ProbeResult is returned to the caller for self-metric recording.
type ProbeResult struct {
	Started, Finished time.Time
	Up                float64
	DriverError       error
	Errors            map[string]error // collector → error
}

func (r *ProbeResult) recordError(name string, err error) {
	if r.Errors == nil {
		r.Errors = make(map[string]error, 4)
	}
	r.Errors[name] = err
}

func (r *ProbeResult) Duration() time.Duration { return r.Finished.Sub(r.Started) }
