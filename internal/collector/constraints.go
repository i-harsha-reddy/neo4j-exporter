package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

type ConstraintsCollector struct{}

func (ConstraintsCollector) Name() string { return "constraints" }

func (ConstraintsCollector) Collect(ctx context.Context, p *ProbeContext) error {
	rows, err := p.Entry.ExecRead(ctx, "", "SHOW CONSTRAINTS YIELD type, entityType", nil)
	if err != nil {
		return err
	}

	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_constraints_count",
		Help: "Number of constraints by type and entity type.",
	}, []string{"database", "type", "entity_type"})
	MustRegister(p.Registry, gauge)

	database := p.Target.Bolt.Database
	if database == "" {
		database = "neo4j"
	}
	type aggKey struct{ typ, entity string }
	agg := make(map[aggKey]int)
	for _, r := range rows {
		agg[aggKey{asString(r["type"]), asString(r["entityType"])}]++
	}
	for k, v := range agg {
		gauge.WithLabelValues(database, k.typ, k.entity).Set(float64(v))
	}
	return nil
}
