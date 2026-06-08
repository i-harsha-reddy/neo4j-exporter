package collector

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DatabasesCollector reads SHOW DATABASES from the system database. CE
// installs always have exactly two: `system` and one user database.
type DatabasesCollector struct{}

func (DatabasesCollector) Name() string { return "databases" }

// canonical statuses we expect to see in SHOW DATABASES; we emit one row
// per (database, status) so dashboards can chart "is db X online?".
var dbStatuses = []string{"online", "offline", "starting", "stopping", "store copying", "unknown"}

func (DatabasesCollector) Collect(ctx context.Context, p *ProbeContext) error {
	rows, err := p.Entry.ExecReadSystem(ctx, "SHOW DATABASES YIELD name, currentStatus, default, access, lastCommittedTxn, creationTime", nil)
	if err != nil {
		return err
	}

	statusGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_database_status",
		Help: "1 if the database is in the labeled status, else 0.",
	}, []string{"database", "status"})
	defaultGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_database_default",
		Help: "1 if the database is the default user database.",
	}, []string{"database"})
	accessGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_database_access_info",
		Help: "Database access mode; value is always 1.",
	}, []string{"database", "access"})
	lastTxnGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_database_last_committed_txid",
		Help: "Last committed transaction ID for the database.",
	}, []string{"database"})
	creationGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_database_creation_timestamp_seconds",
		Help: "Database creation time as Unix seconds.",
	}, []string{"database"})

	for _, m := range []prometheus.Collector{statusGauge, defaultGauge, accessGauge, lastTxnGauge, creationGauge} {
		MustRegister(p.Registry, m)
	}

	for _, r := range rows {
		name := asString(r["name"])
		if name == "" {
			continue
		}
		actual := strings.ToLower(asString(r["currentStatus"]))
		for _, s := range dbStatuses {
			v := 0.0
			if s == actual {
				v = 1
			}
			statusGauge.WithLabelValues(name, s).Set(v)
		}
		// Catch-all if Neo4j returns a status we don't have in our list.
		known := false
		for _, s := range dbStatuses {
			if s == actual {
				known = true
				break
			}
		}
		if !known && actual != "" {
			statusGauge.WithLabelValues(name, actual).Set(1)
		}

		if asFloat64(r["default"]) > 0 {
			defaultGauge.WithLabelValues(name).Set(1)
		} else {
			defaultGauge.WithLabelValues(name).Set(0)
		}
		if access := asString(r["access"]); access != "" {
			accessGauge.WithLabelValues(name, access).Set(1)
		}
		lastTxnGauge.WithLabelValues(name).Set(asFloat64(r["lastCommittedTxn"]))

		switch t := r["creationTime"].(type) {
		case time.Time:
			creationGauge.WithLabelValues(name).Set(float64(t.Unix()))
		default:
			// Driver may return a *Time pointer or DateTime; try float coerce as fallback.
			if f := asFloat64(r["creationTime"]); f > 0 {
				creationGauge.WithLabelValues(name).Set(f)
			}
		}
	}
	return nil
}
