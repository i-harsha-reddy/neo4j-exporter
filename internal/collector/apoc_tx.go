package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// APOCTxCollector reads transaction counters via apoc.monitor.tx.
// These are historical counters that complement TransactionsCollector's
// live snapshot.
type APOCTxCollector struct{}

func (APOCTxCollector) Name() string { return "apoc_tx" }

func (APOCTxCollector) Collect(ctx context.Context, p *ProbeContext) error {
	if !p.Caps.HasAPOCMonitorTx {
		return nil
	}
	rows, err := p.Entry.ExecRead(ctx, "", "CALL apoc.monitor.tx()", nil)
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

	mk := func(name, help string) *prometheus.GaugeVec {
		g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, []string{"database"})
		MustRegister(p.Registry, g)
		return g
	}

	// We expose these as gauges (not counters) because we cannot enforce
	// monotonicity client-side; if Neo4j restarts, totalTx resets.
	mk("neo4j_transactions_committed_count", "Total committed transactions reported by apoc.monitor.tx (resets on restart).").
		WithLabelValues(database).Set(asFloat64(r["totalTx"]))
	mk("neo4j_transactions_rolled_back_count", "Total rolled-back transactions (resets on restart).").
		WithLabelValues(database).Set(asFloat64(r["rolledBackTx"]))
	mk("neo4j_transactions_peak_concurrent", "Peak concurrent transactions since last restart.").
		WithLabelValues(database).Set(asFloat64(r["peakTx"]))
	mk("neo4j_transactions_currently_open", "Transactions currently open per apoc.monitor.tx.").
		WithLabelValues(database).Set(asFloat64(r["currentOpenedTx"]))
	mk("neo4j_transactions_opened_count", "Total transactions opened (resets on restart).").
		WithLabelValues(database).Set(asFloat64(r["totalOpenedTx"]))
	mk("neo4j_transactions_last_id", "Last transaction ID assigned by Neo4j.").
		WithLabelValues(database).Set(asFloat64(r["lastTxId"]))
	return nil
}
