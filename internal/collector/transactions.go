package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// TransactionsCollector aggregates SHOW TRANSACTIONS server-side. Even
// with thousands of in-flight txns the wire payload stays O(databases ×
// statuses) ≈ 12 rows.
//
// The query also exposes per-tx page hits/faults, allocated bytes, locks,
// and CPU/wait/idle time — these are valuable proxies for behavior that
// is otherwise Enterprise-only (page cache hit rate, per-DB CPU). We sum
// these across active transactions; dashboards should label them
// "currently-active sum" so users don't mistake them for global counters.
type TransactionsCollector struct{}

func (TransactionsCollector) Name() string { return "transactions" }

const aggregatedTransactionsQuery = `
SHOW TRANSACTIONS YIELD
  database, status, elapsedTime, cpuTime, waitTime, idleTime,
  allocatedDirectBytes, estimatedUsedHeapMemory, pageHits, pageFaults,
  activeLockCount
RETURN
  database,
  status,
  count(*)                                          AS active,
  max(elapsedTime)                                  AS longest,
  sum(coalesce(cpuTime, duration({seconds:0})))     AS cpu_total,
  sum(coalesce(waitTime, duration({seconds:0})))    AS wait_total,
  sum(coalesce(idleTime, duration({seconds:0})))    AS idle_total,
  sum(coalesce(allocatedDirectBytes, 0))            AS direct_bytes,
  sum(coalesce(estimatedUsedHeapMemory, 0))         AS heap_bytes,
  sum(coalesce(pageHits, 0))                        AS page_hits,
  sum(coalesce(pageFaults, 0))                      AS page_faults,
  sum(coalesce(activeLockCount, 0))                 AS locks
`

func (TransactionsCollector) Collect(ctx context.Context, p *ProbeContext) error {
	rows, err := p.Entry.ExecRead(ctx, "", aggregatedTransactionsQuery, nil)
	if err != nil {
		return err
	}

	active := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active",
		Help: "Number of currently active transactions, by database and status.",
	}, []string{"database", "status"})
	longest := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_longest_active_seconds",
		Help: "Elapsed time of the oldest currently-active transaction.",
	}, []string{"database"})
	cpu := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_cpu_seconds",
		Help: "Sum of cpuTime across currently-active transactions (snapshot).",
	}, []string{"database"})
	wait := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_wait_seconds",
		Help: "Sum of waitTime (lock acquisition) across currently-active transactions.",
	}, []string{"database"})
	idle := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_idle_seconds",
		Help: "Sum of idleTime across currently-active transactions.",
	}, []string{"database"})
	direct := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_allocated_direct_bytes",
		Help: "Sum of allocatedDirectBytes across currently-active transactions.",
	}, []string{"database"})
	heap := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_estimated_heap_bytes",
		Help: "Sum of estimatedUsedHeapMemory across currently-active transactions.",
	}, []string{"database"})
	pageHits := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_page_hits",
		Help: "Sum of pageHits across currently-active transactions (per-tx counter, not global).",
	}, []string{"database"})
	pageFaults := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_page_faults",
		Help: "Sum of pageFaults across currently-active transactions (per-tx counter, not global).",
	}, []string{"database"})
	locks := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_transactions_active_lock_count",
		Help: "Sum of activeLockCount across currently-active transactions.",
	}, []string{"database"})

	for _, m := range []prometheus.Collector{
		active, longest, cpu, wait, idle, direct, heap, pageHits, pageFaults, locks,
	} {
		MustRegister(p.Registry, m)
	}

	// Track per-database aggregates derived from per-(db,status) rows.
	type dbAgg struct {
		longest                            float64
		cpu, wait, idle                     float64
		direct, heap                        float64
		pageHits, pageFaults                 float64
		locks                                float64
	}
	perDB := map[string]*dbAgg{}

	for _, r := range rows {
		db := asString(r["database"])
		status := asString(r["status"])
		if db == "" {
			continue
		}
		active.WithLabelValues(db, status).Set(asFloat64(r["active"]))

		ag := perDB[db]
		if ag == nil {
			ag = &dbAgg{}
			perDB[db] = ag
		}
		if l := asFloat64(r["longest"]); l > ag.longest {
			ag.longest = l
		}
		ag.cpu += asFloat64(r["cpu_total"])
		ag.wait += asFloat64(r["wait_total"])
		ag.idle += asFloat64(r["idle_total"])
		ag.direct += asFloat64(r["direct_bytes"])
		ag.heap += asFloat64(r["heap_bytes"])
		ag.pageHits += asFloat64(r["page_hits"])
		ag.pageFaults += asFloat64(r["page_faults"])
		ag.locks += asFloat64(r["locks"])
	}
	for db, ag := range perDB {
		// elapsedTime / cpuTime / etc. come back as a Cypher Duration which the
		// v5 driver surfaces as neo4j.Duration; asFloat64 converts to seconds.
		longest.WithLabelValues(db).Set(ag.longest)
		cpu.WithLabelValues(db).Set(ag.cpu)
		wait.WithLabelValues(db).Set(ag.wait)
		idle.WithLabelValues(db).Set(ag.idle)
		direct.WithLabelValues(db).Set(ag.direct)
		heap.WithLabelValues(db).Set(ag.heap)
		pageHits.WithLabelValues(db).Set(ag.pageHits)
		pageFaults.WithLabelValues(db).Set(ag.pageFaults)
		locks.WithLabelValues(db).Set(ag.locks)
	}
	return nil
}
