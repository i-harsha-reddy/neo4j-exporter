package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// APOCStoreCollector reads byte-size breakdown of the store via
// apoc.monitor.store. The store totals are dbms-wide (not per-database)
// — APOC does not partition them. We therefore omit the database label.
type APOCStoreCollector struct{}

func (APOCStoreCollector) Name() string { return "apoc_store" }

func (APOCStoreCollector) Collect(ctx context.Context, p *ProbeContext) error {
	if !p.Caps.HasAPOCMonitorStore {
		return nil
	}
	rows, err := p.Entry.ExecRead(ctx, "", "CALL apoc.monitor.store()", nil)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	r := rows[0]

	pairs := []struct {
		metric, help, key string
	}{
		{"neo4j_store_node_bytes", "Size of the node store in bytes.", "nodeStoreSize"},
		{"neo4j_store_relationship_bytes", "Size of the relationship store in bytes.", "relStoreSize"},
		{"neo4j_store_property_bytes", "Size of the property store in bytes.", "propStoreSize"},
		{"neo4j_store_string_bytes", "Size of the string store in bytes.", "stringStoreSize"},
		{"neo4j_store_array_bytes", "Size of the array store in bytes.", "arrayStoreSize"},
		{"neo4j_store_log_bytes", "Size of the transaction log files in bytes.", "logSize"},
		{"neo4j_store_total_bytes", "Total store size in bytes.", "totalStoreSize"},
	}
	for _, pair := range pairs {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: pair.metric, Help: pair.help})
		MustRegister(p.Registry, g)
		g.Set(asFloat64(r[pair.key]))
	}
	return nil
}
