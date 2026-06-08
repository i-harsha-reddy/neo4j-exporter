package collector

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// IndexesCollector reads SHOW INDEXES. Cardinality is bounded by the
// number of named indexes on the schema; we cap per-name series at 500
// (above which the `name` label is replaced with "<truncated>").
type IndexesCollector struct {
	NameLabelMaxCardinality int // 0 ⇒ default 500
}

func (IndexesCollector) Name() string { return "indexes" }

const defaultIndexNameCardinality = 500

func (c IndexesCollector) Collect(ctx context.Context, p *ProbeContext) error {
	rows, err := p.Entry.ExecRead(ctx, "",
		`SHOW INDEXES YIELD name, type, entityType, state, populationPercent, readCount, lastRead, owningConstraint`, nil)
	if err != nil {
		return err
	}

	count := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_indexes_count",
		Help: "Number of indexes by state, type, and entity type.",
	}, []string{"database", "state", "type", "entity_type"})
	pop := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_index_population_percent",
		Help: "Population progress per index (only emitted while POPULATING).",
	}, []string{"database", "name", "type"})
	reads := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_index_read_count",
		Help: "Cumulative read count per index.",
	}, []string{"database", "name", "type"})
	lastRead := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_index_last_read_timestamp_seconds",
		Help: "Last read timestamp for the index, as Unix seconds.",
	}, []string{"database", "name", "type"})

	for _, m := range []prometheus.Collector{count, pop, reads, lastRead} {
		MustRegister(p.Registry, m)
	}

	cap := c.NameLabelMaxCardinality
	if cap == 0 {
		cap = defaultIndexNameCardinality
	}
	database := p.Target.Bolt.Database
	if database == "" {
		database = "neo4j"
	}

	type aggKey struct{ state, typ, entity string }
	aggregate := make(map[aggKey]int)

	for i, r := range rows {
		state := asString(r["state"])
		typ := asString(r["type"])
		entity := asString(r["entityType"])
		aggregate[aggKey{state, typ, entity}]++

		name := asString(r["name"])
		if i >= cap {
			name = "<truncated>"
		}
		if state == "POPULATING" {
			pop.WithLabelValues(database, name, typ).Set(asFloat64(r["populationPercent"]))
		}
		reads.WithLabelValues(database, name, typ).Set(asFloat64(r["readCount"]))
		switch t := r["lastRead"].(type) {
		case time.Time:
			if !t.IsZero() {
				lastRead.WithLabelValues(database, name, typ).Set(float64(t.Unix()))
			}
		}
	}
	for k, v := range aggregate {
		count.WithLabelValues(database, k.state, k.typ, k.entity).Set(float64(v))
	}
	return nil
}
