# neo4j-exporter

Prometheus exporter for **Neo4j Community Edition 5.x / 2025.x**, designed for fleet operation.

Neo4j Community Edition has no native Prometheus endpoint and no `neo4j.metrics:*` JMX MBean tree — both are Enterprise-only. This exporter fills that gap by combining:

- Cypher `SHOW` commands (transactions, databases, indexes, constraints) — universal
- APOC monitor procedures (`apoc.monitor.kernel/store/tx/ids` from APOC Extended; `apoc.meta.stats` from APOC Core) — auto-detected at startup, gracefully degraded
- JVM metrics via [Jolokia](https://jolokia.org/) (HTTP→JMX bridge for `java.lang:*` MBeans)

It uses the [multi-target probe pattern](https://prometheus.io/docs/guides/multi-target-exporter/) (same shape as `blackbox_exporter` / `snmp_exporter`): a single exporter binary scrapes a fleet of Neo4j instances.

**Status:** v1 verified end-to-end against **2 identical Neo4j instances** (`neo4j:5-community` + APOC Core + Extended + Jolokia 2.1.1) plus Prometheus v2.55 + Grafana 11.3 via `bash scripts/e2e.sh` (17 hard-fail assertions). Multi-target probe pipeline proven; every collector reports green; `/slow-queries` returns rows.

## Quickstart

```bash
# spin up the 2-instance demo stack (neo4j-a + neo4j-b + apoc + jolokia + exporter + prometheus + grafana + loadgen)
cd examples/docker-compose
docker compose up -d --wait
open http://localhost:3000  # Grafana — 6 provisioned dashboards
open http://localhost:9090  # Prometheus — scrapes both instances
curl 'http://localhost:9412/probe?target=neo4j-a:7687&module=default'
curl 'http://localhost:9412/probe?target=neo4j-b:7687&module=default'
```

See [`docs/DEMO-STACK.md`](docs/DEMO-STACK.md) for topology, loadgen scenario menu, and env-var knobs.

Or run the exporter directly:

```bash
go build ./cmd/neo4j-exporter
./neo4j-exporter --config.file=config/neo4j-exporter.minimal.yml --log.level=info
curl 'http://localhost:9412/probe?target=localhost:7687&module=default'
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: neo4j
    metrics_path: /probe
    params: { module: [default] }
    static_configs:
      - targets: [neo4j-prod-1:7687, neo4j-prod-2:7687]
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: neo4j-exporter:9412
```

### Install — Docker

A multi-arch (amd64 + arm64) distroless image is published at `ghcr.io/i-harsha-reddy/neo4j-exporter`:

```bash
docker pull ghcr.io/i-harsha-reddy/neo4j-exporter:0.1.0

# Optional: verify the cosign keyless OIDC signature
cosign verify ghcr.io/i-harsha-reddy/neo4j-exporter:0.1.0 \
  --certificate-identity-regexp 'github.com/i-harsha-reddy/neo4j-exporter' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

# Run, mounting a config (see config/neo4j-exporter.example.yml for the schema)
docker run -d --name neo4j-exporter -p 9412:9412 \
  -v "$PWD/config.yml:/etc/neo4j-exporter/config.yml:ro" \
  ghcr.io/i-harsha-reddy/neo4j-exporter:0.1.0
```

### Install on Kubernetes (Helm)

A production-grade Helm chart is published at `oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter`:

```bash
helm install neo4j-exporter oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter \
  --version 0.1.0 \
  --namespace monitoring --create-namespace \
  --set auth.password=YOUR_BOLT_PASSWORD \
  --set auth.jolokiaPassword=YOUR_JOLOKIA_PASSWORD
```

For production, prefer `auth.existingSecret` over inline passwords. See [`charts/neo4j-exporter/README.md`](charts/neo4j-exporter/README.md) for the full values reference, target configuration, NetworkPolicy and HPA opt-ins, and how to wire Prometheus / Prometheus Operator.

## What it monitors

| Subsystem | Source | Notes |
|---|---|---|
| Liveness, version, latency | Bolt ping | Always available |
| Database state | `SHOW DATABASES` | status, default, last committed txid |
| Transactions (live) | `SHOW TRANSACTIONS` (server-side aggregated) | active by status, longest age, page hits/faults, allocated bytes, lock counts |
| Transactions (counters) | `apoc.monitor.tx` | committed, rolled back, peak, currently open — APOC Extended |
| Store sizes | `apoc.monitor.store` | bytes per component (node, rel, prop, string, array, log) — APOC Extended |
| Kernel info | `apoc.monitor.kernel` | version, store id, read-only flag — APOC Extended |
| ID usage | `apoc.monitor.ids` | node/rel/property/rel-type IDs — APOC Extended |
| Schema counts | `apoc.meta.stats` | node/rel counts per label — APOC Core fallback |
| Indexes | `SHOW INDEXES` | count by state/type, population %, read count |
| Constraints | `SHOW CONSTRAINTS` | count by type/entity |
| JVM | Jolokia / `java.lang:*` MBeans | heap, GC, threads, FDs, CPU |
| Slow queries | `SHOW TRANSACTIONS … WHERE elapsedTime>$min` | separate `/slow-queries` info endpoint |

See [`docs/METRICS.md`](docs/METRICS.md) for the full metric reference and [`docs/COMPATIBILITY.md`](docs/COMPATIBILITY.md) for what is **not** available in CE (page cache hit rate, query runtime breakdown, checkpoints, cluster — Enterprise-only).

## Bundled

- [`dashboards/grafana/`](dashboards/grafana) — six dashboards: overview (fleet triage), transactions, storage, JVM, slow-queries, schema & capacity
- [`examples/docker-compose/`](examples/docker-compose) — end-to-end stack with 2 Neo4j instances + multi-scenario loadgen
- [`charts/neo4j-exporter/`](charts/neo4j-exporter) — production-grade Helm chart (OCI-published)
- [`examples/kubernetes/`](examples/kubernetes) — Helm scenario values + Probe/ServiceMonitor for prometheus-operator

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — project memory for Claude Code (architecture, conventions, build commands, gotchas)
- [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) — full YAML reference, modules, targets, reload semantics
- [`docs/METRICS.md`](docs/METRICS.md) — every metric with type, labels, source, plus dashboards reference
- [`docs/JOLOKIA.md`](docs/JOLOKIA.md) — production-grade Jolokia setup (TLS + auth + `policy.xml`)
- [`docs/COMPATIBILITY.md`](docs/COMPATIBILITY.md) — CE vs EE feature matrix; what is intentionally absent
- [`docs/DEMO-STACK.md`](docs/DEMO-STACK.md) — topology, scenario menu, env-var knobs for the 2-instance demo

## Development

```bash
make build           # compile to ./neo4j-exporter
make test            # go test -race ./...
make lint            # golangci-lint (skipped if not installed)
make run             # run against config/neo4j-exporter.minimal.yml
make e2e             # full docker-compose smoke test (~5–6 min, 17 assertions)
make docker          # build the container image
```

If you're using Claude Code, this repo ships project-specific helpers under [`.claude/`](.claude):

| Slash command | What it does |
|---|---|
| `/check` | vet + race tests + build (preflight before commit) |
| `/probe <target>` | issue an ad-hoc `/probe` to a running exporter |
| `/compose-up` / `/compose-down` | bring the demo stack up / down |
| `/e2e` | run `scripts/e2e.sh` end-to-end |
| `/release-snapshot` | `goreleaser` snapshot build (no publish) |

| Skill | Use when |
|---|---|
| [`add-collector`](.claude/skills/add-collector/SKILL.md) | adding a new metric collector — covers all touch points (Go file, registration, config, docs, dashboard) |

See [`CLAUDE.md`](CLAUDE.md) for repo conventions (metric naming, Cypher-side aggregation, `up`-from-bolt-only, PII guard, cardinality caps) before making changes.

## Contributing

Pull requests welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for setup, conventions, and the PR checklist. Security-sensitive issues: see [`SECURITY.md`](SECURITY.md), do not file public issues. Release notes live in [`CHANGELOG.md`](CHANGELOG.md).

## License

[Apache-2.0](LICENSE)
