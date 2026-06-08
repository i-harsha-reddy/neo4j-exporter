package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// APOCMetaCollector reads schema-level node/rel counts via apoc.meta.stats
// (APOC Core — works without the Extended plugin). It is the partial
// fallback for apoc.monitor.ids when only Core is installed.
//
// apoc.meta.stats returns a single row with:
//   labelCount, relTypeCount, propertyKeyCount, nodeCount, relCount,
//   labels (map<label,count>), relTypes (map<type,count>), stats (more detail)
type APOCMetaCollector struct {
	// MaxLabelCardinality caps the number of distinct labels we expose
	// as a label dimension. Schema-runaway protection; default 200.
	MaxLabelCardinality int
}

func (APOCMetaCollector) Name() string { return "apoc_meta" }

const defaultMetaCardinality = 200

func (c APOCMetaCollector) Collect(ctx context.Context, p *ProbeContext) error {
	if !p.Caps.HasAPOCMetaStats {
		return nil
	}
	rows, err := p.Entry.ExecRead(ctx, "", "CALL apoc.meta.stats()", nil)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	r := rows[0]
	database := p.Target.Bolt.Database
	if database == "" {
		database = "neo4j"
	}

	mk := func(name, help string, labels []string) *prometheus.GaugeVec {
		g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, labels)
		MustRegister(p.Registry, g)
		return g
	}

	mk("neo4j_meta_node_count", "Total nodes in the database (apoc.meta.stats).", []string{"database"}).
		WithLabelValues(database).Set(asFloat64(r["nodeCount"]))
	mk("neo4j_meta_relationship_count", "Total relationships in the database (apoc.meta.stats).", []string{"database"}).
		WithLabelValues(database).Set(asFloat64(r["relCount"]))
	mk("neo4j_meta_label_count", "Number of distinct labels (apoc.meta.stats).", []string{"database"}).
		WithLabelValues(database).Set(asFloat64(r["labelCount"]))
	mk("neo4j_meta_relationship_type_count", "Number of distinct relationship types.", []string{"database"}).
		WithLabelValues(database).Set(asFloat64(r["relTypeCount"]))
	mk("neo4j_meta_property_key_count", "Number of distinct property keys.", []string{"database"}).
		WithLabelValues(database).Set(asFloat64(r["propertyKeyCount"]))

	maxCard := c.MaxLabelCardinality
	if maxCard == 0 {
		maxCard = defaultMetaCardinality
	}

	if labels, ok := r["labels"].(map[string]any); ok {
		g := mk("neo4j_meta_label_node_count", "Number of nodes per label (capped at MaxLabelCardinality).", []string{"database", "label"})
		emitMapBounded(g, database, "label", labels, maxCard)
	}
	if rt, ok := r["relTypes"].(map[string]any); ok {
		g := mk("neo4j_meta_relationship_type_count_by_type", "Number of relationships per type (capped at MaxLabelCardinality).", []string{"database", "relationship_type"})
		emitMapBounded(g, database, "relationship_type", rt, maxCard)
	}
	return nil
}

func emitMapBounded(g *prometheus.GaugeVec, database, labelName string, m map[string]any, cap int) {
	count := 0
	for k, v := range m {
		if count >= cap {
			g.WithLabelValues(database, "<truncated>").Set(asFloat64(v))
			return
		}
		g.WithLabelValues(database, k).Set(asFloat64(v))
		count++
	}
	_ = labelName // kept for grep
}
