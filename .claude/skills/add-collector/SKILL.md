---
name: add-collector
description: Use when the user wants to add a new metric collector to neo4j-exporter — for example, exposing a new Cypher SHOW result, an APOC procedure, or a Jolokia MBean. Walks through the full set of touch points (collector file, registration, config, docs, dashboard, alert) so nothing is missed.
---

# Adding a new collector

A collector is a Go type that implements `collector.Collector` and emits metrics for one subsystem of Neo4j. This skill covers the complete change set required to land a new collector cleanly.

## Decide first

Before writing code, the user should be clear on:

1. **Source** — where does the data come from? Bolt + Cypher (`SHOW <something>` or a procedure call), an APOC procedure, or a Jolokia MBean? If it's Cypher, can it be aggregated server-side?
2. **APOC tier** — if APOC, is it Core or Extended? Extended needs `auto`-mode handling.
3. **Cardinality** — what labels? Bound them. CE has 2 DBs so `database` is fine. Per-name labels need a cap.
4. **Naming** — `neo4j_<subsystem>_<metric>_<unit>`. Always include the unit.
5. **Edition gating** — does the source actually exist in CE? If it's an Enterprise-only metric (page cache, runtime breakdown, checkpoints, cluster, per-DB CPU, per-pool memory, Bolt accept/close counters), STOP. Push back — those need Enterprise. See `docs/COMPATIBILITY.md`.

## The complete touch list

For a new collector named `<name>` (e.g. `pagecache`, `checkpoints`, `meta_v2`):

### 1. `internal/collector/<name>.go`

```go
package collector

import (
    "context"

    "github.com/prometheus/client_golang/prometheus"
)

type <Name>Collector struct{}

func (<Name>Collector) Name() string { return "<name>" }

func (<Name>Collector) Collect(ctx context.Context, p *ProbeContext) error {
    // Gate on capability if APOC Extended:
    // if !p.Caps.HasAPOCMonitor<X> { return nil }

    rows, err := p.Entry.ExecRead(ctx, "", "<CYPHER>", nil)
    if err != nil {
        return err
    }

    g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "neo4j_<subsystem>_<metric>_<unit>",
        Help: "...",
    }, []string{"database", /* etc */})
    MustRegister(p.Registry, g)

    for _, r := range rows {
        // Use asString / asFloat64 / asUnixSeconds from coerce.go
        g.WithLabelValues(/* ... */).Set(asFloat64(r["..."]))
    }
    return nil
}
```

**Aggregation rules** (see `transactions.go` for the canonical example):
- If the query CAN return many rows (transactions, indexes), aggregate in **Cypher** (`count(*)`, `sum`, `max`) so the wire payload is bounded.
- Use `coalesce(<col>, 0)` (or `duration({seconds:0})` for durations) to defend against NULLs.

### 2. Register in `cmd/neo4j-exporter/main.go`

Add to `registerCollectors()`:

```go
"<name>": collector.<Name>Collector{},
```

### 3. Allow in config — `internal/config/config.go` `knownCollector()`

Add `"<name>"` to the switch list. Without this, the loader rejects the toggle as unknown.

### 4. Update example configs

- `config/neo4j-exporter.example.yml` — add the collector under each module's `collectors:` map with a sensible default (`true` if always-on, `auto` if capability-gated).
- `config/neo4j-exporter.minimal.yml` — only include if it's part of the happy path.
- `examples/docker-compose/neo4j-exporter/config.yml` — keep in sync so the demo shows it.

### 5. Document in `docs/METRICS.md`

Add one row per metric to the table. Use the `[ext]` / `[core]` / `[jolokia]` tags consistently.

### 6. Update `docs/COMPATIBILITY.md` if the matrix changes

If the collector exposes something whose availability differs across editions or APOC tiers, add it to the relevant table.

### 7. (Optional) Grafana panel

Pick the right dashboard:
- `dashboards/grafana/neo4j-overview.json` — fleet-wide
- `dashboards/grafana/neo4j-transactions.json` — txn-related
- `dashboards/grafana/neo4j-storage.json` — store/index/schema
- `dashboards/grafana/neo4j-jvm.json` — JVM only

Keep panel ids unique within the dashboard. Use `${datasource}` and `${instance}` template variables. If the panel could be empty in CE-without-APOC-Extended, add a description noting that.

### 8. Verify

```bash
go vet ./...
go test -race -count=1 ./...
go build ./cmd/neo4j-exporter
```

Then run the demo stack and confirm the new metric appears:

```bash
# /compose-up to start the stack, then:
curl -fsS 'http://localhost:9412/probe?target=neo4j:7687&module=default' | grep -E '^neo4j_<subsystem>_'
```

If the metric is empty in the demo, check capability gating (`p.Caps.Has...`) and the orchestrator's `selectCollectors` filter.

## Anti-patterns to avoid

- Pulling per-row data into Go for aggregation. Aggregate in Cypher.
- Adding `instance` as an explicit label. Prometheus relabel adds it.
- Per-query-id labels on `/probe` time-series. That belongs on `/slow-queries`.
- Naming `*_total` for non-monotonic counters. APOC counters reset on restart — name them `*_count` (gauge).
- Faking an Enterprise-only metric. Document its absence in `COMPATIBILITY.md` instead.
- Adding new dependencies without `go mod tidy` and without checking license compatibility (Apache-2.0).
