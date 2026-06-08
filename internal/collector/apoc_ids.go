package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// APOCIdsCollector reads ID-space usage via apoc.monitor.ids.
// Neo4j 5+ uses block format and IDs are not reused, so this is a
// rough indicator of historical scale rather than an exhaustion concern.
type APOCIdsCollector struct{}

func (APOCIdsCollector) Name() string { return "apoc_ids" }

func (APOCIdsCollector) Collect(ctx context.Context, p *ProbeContext) error {
	if !p.Caps.HasAPOCMonitorIds {
		return nil
	}
	rows, err := p.Entry.ExecRead(ctx, "", "CALL apoc.monitor.ids()", nil)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	r := rows[0]

	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_ids_in_use_total",
		Help: "Number of IDs in use by kind (per apoc.monitor.ids).",
	}, []string{"kind"})
	MustRegister(p.Registry, g)
	g.WithLabelValues("node").Set(asFloat64(r["nodeIds"]))
	g.WithLabelValues("relationship").Set(asFloat64(r["relIds"]))
	g.WithLabelValues("property").Set(asFloat64(r["propIds"]))
	g.WithLabelValues("relationship_type").Set(asFloat64(r["relTypeIds"]))
	return nil
}
