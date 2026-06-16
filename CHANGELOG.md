# Changelog

All notable changes to this project are documented here. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] — 2026-06-16

Bug-fix release addressing user-reported dashboard issues — including a latent metric-coercion bug that silently zeroed every elapsed-time metric.

### Fixed

- **Cypher `Duration` metrics read `0`.** `SHOW TRANSACTIONS` yields `elapsedTime` / `cpuTime` / `waitTime` / `idleTime` as Cypher Durations, which the v5 Go driver surfaces as `neo4j.Duration` — **not** Go's `time.Duration`. Both coercion helpers (`asFloat64` in `internal/collector/coerce.go`, `durSeconds` in `internal/probe/slowqueries.go`) fell through to `0`, silently zeroing `neo4j_transactions_longest_active_seconds`, `neo4j_transactions_active_{cpu,wait,idle}_seconds`, and every `neo4j_slow_query_{elapsed,cpu,wait}_seconds`. Added explicit `neo4j.Duration` → seconds conversion with regression tests (`coerce_test.go`, `slowqueries_test.go`). This was the root cause of the Slow Queries dashboard appearing empty — its in-flight count, max-elapsed, and `>10s` backlog panels all key off elapsed time.
- **Overview → "Probe duration per instance" panel was empty.** The panel and the per-instance triage table hard-coded `job="neo4j"` instead of the dashboard's `$job` template variable, so they returned nothing for any Prometheus job name other than literally `neo4j`. Both now use `job=~"$job"`.
- **Dashboards rendered raw, unformatted large numbers.** Added Grafana units to ~30 count / rate panels across all six dashboards — `short` (SI suffixes, e.g. `8.5 Mil`, `655 K`) for node / relationship / ID / txid / page-fault / thread counts, `ops` for rate panels. Byte panels were already correct and are unchanged.

### Changed

- **Slow Queries dashboard.** Renders the point-in-time snapshot as points / bars so single samples are visible, adds a "live snapshot — empty is healthy" banner explaining that Neo4j CE cannot replay finished queries, and applies the unit fixes above.
- **Demo stack realism.** Lowered the `neo4j-slow-queries` Prometheus scrape interval (30s → 15s) and retuned the `slow_traversals` loadgen scenario (longer queries, 5s cadence) so a slow query is reliably in-flight at scrape time and crosses the 10s mark that drives the backlog panel.

## [0.1.0] — 2026-06-08

Initial release. Single-binary Prometheus exporter for **Neo4j Community Edition 5.x / 2025.x**, verified end-to-end against `neo4j:5-community` + APOC Core + APOC Extended + Jolokia 2.1.1 + Prometheus v2.55 + Grafana 11.3 via `bash scripts/e2e.sh` (17 hard-fail assertions across two Neo4j instances plus fleet-level Prometheus checks).

### Added

#### Exporter

- **Multi-target probe-pattern exporter.** Single binary, one exporter scrapes a fleet of Neo4j targets — same shape as `blackbox_exporter` / `snmp_exporter`. Endpoints: `/probe`, `/metrics`, `/slow-queries`, `/health`, `/ready`, `/-/reload`. Per-request fresh `prometheus.Registry`; collectors run in parallel under `errgroup` with per-collector timeouts.
- **12 collectors.** `server`, `bolt`, `databases`, `transactions`, `indexes`, `constraints`, `apoc_kernel`, `apoc_store`, `apoc_tx`, `apoc_ids`, `apoc_meta`, `jolokia`. APOC Extended is auto-detected per target on first contact and cached; reload invalidates the cache. `neo4j_up` is set by the bolt collector only — other collector failures do not flip `up`.
- **JVM via Jolokia.** Heap, non-heap, GC, threads, classes, uptime, file descriptors, system load — all read via Jolokia bulk MBean queries against `java.lang:*`. Lives behind opt-in basic-auth + TLS guidance in `docs/JOLOKIA.md`.
- **Slow-query snapshot.** `/slow-queries` returns top-N currently-running queries with `query_hash` (SHA-1[:8]) by default; full query text exposure is opt-in via `slow_query.expose_query_text: true` (defaults `false` for PII safety). Cardinality-capped at the request level via `top` and `min_seconds`.

#### Observability artifacts

- **6 Grafana dashboards** under `dashboards/grafana/`: `neo4j-overview` (fleet triage with per-instance click-through), `neo4j-transactions` (CPU/wait/idle decomposition + APOC accounting + slow-query callout), `neo4j-storage` (bytes + growth forecast via `predict_linear` + kernel identity + ID growth), `neo4j-jvm` (heap + non-heap + GC + classes + lifecycle), `neo4j-slow-queries` (snapshot table + distributions + hotspots + backlog), `neo4j-schema` (indexes by state + usage analytics + schema scale + ID forecast + constraints). Dashboards query raw exporter metrics directly — no Prometheus recording rules required. Time-based panels multiply `*_timestamp_seconds` metrics by `1000` for Grafana's `dateTimeFromNow` unit.

#### Demo stack

- **2-instance docker-compose stack** at `examples/docker-compose/`: `neo4j-a:7687` + `neo4j-b:7687` (both `neo4j:5-community` + APOC Core + Extended + Jolokia 2.1.1) plus exporter + Prometheus v2.55 + Grafana 11.3 + multi-scenario loadgen. Exercises the multi-target probe pipeline (`__address__` → `__param_target` → `instance` relabel) end-to-end.
- **8-scenario loadgen** (`bootstrap`, `hot_writes`, `contention`, `slow_traversals`, `index_churn`, `rollbacks`, `apoc_batch`, `schema_diversity`) with a bash scheduler and `--once` subcommand for deterministic e2e. Produces 3 UNIQUENESS constraints, 6 indexes, slow queries via `apoc.util.sleep`, deliberate UNIQUE-violation rollbacks, GC pressure via `apoc.periodic.iterate`, and label / relationship-type diversity.
- **17-step e2e smoke test** at `scripts/e2e.sh` — adaptive readiness wait, per-instance collector success, version + database-online, `transactions_rolled_back_count > 0`, `/slow-queries` returns rows, index-churn round-trip, no cardinality leakage, Prometheus fleet-level checks. All assertions are hard-fail; no soft warnings.

#### Packaging & distribution

- **Helm chart** at `charts/neo4j-exporter/`, published to `oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter`. Production-grade: ServiceAccount, PodDisruptionBudget (auto-enables for HA), NetworkPolicy (opt-in), HPA (opt-in), readiness / liveness probes, secret via `existingSecret` or inline, checksum-driven config rollout, `helm test` connectivity probe.
- **Container image** at `ghcr.io/i-harsha-reddy/neo4j-exporter` — multi-arch (amd64 + arm64), distroless static base, nonroot user. Both image and chart are cosign keyless-OIDC signed. Verify with `cosign verify ghcr.io/i-harsha-reddy/charts/neo4j-exporter:<tag> --certificate-identity-regexp 'github.com/i-harsha-reddy/neo4j-exporter' --certificate-oidc-issuer https://token.actions.githubusercontent.com`.
- **Two Dockerfiles** with the same runtime stage: `Dockerfile` (multi-stage build-from-source, used by `make docker`) and `Dockerfile.release` (wrap-prebuilt-binary, used by goreleaser's `dockers:` blocks). See `CLAUDE.md` for when to edit which.

#### Continuous integration

- `ci.yml` — vet, race tests, build, `golangci-lint` v2.12.2, `govulncheck`.
- `e2e.yml` — full compose-stack smoke test on every PR + push to main.
- `chart-test.yml` — `helm lint` + `kubeconform` + chart-testing on a kind cluster, on `charts/**` changes.
- `release.yml` — goreleaser-driven binary archives (darwin/linux × amd64/arm64) + multi-arch docker images + GitHub Release on `v*` tags.
- `chart-release.yml` — `helm push` to GHCR + cosign sign + chart `.tgz` attached to the GitHub Release on `v*` and `chart-v*` tags.

#### Project hygiene

- `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1).
- `.github/ISSUE_TEMPLATE/` — YAML Issue Forms for bug reports and feature requests, plus `config.yml` that disables blank issues and routes security reports to email.
- `.github/PULL_REQUEST_TEMPLATE.md` — auto-populates the PR checklist from `CONTRIBUTING.md`.

### Security notes

- Secret-file mode validator accepts `0400`, `0440`, `0600`, `0640`. The group-read modes are required for the canonical K8s nonroot pattern (kubelet's `fsGroup` OR's group-read into Secret-mounted file modes).
- Cardinality safeguards: `indexes` collector caps the `name` label at 500 distinct values; `apoc_meta` caps `label` / `relationship_type` at 200 each. CE has two databases (`system` + user DB), so the `database` label is naturally bounded.
- PII guard: `/slow-queries` always emits `query_hash`; query-text exposure is opt-in via `slow_query.expose_query_text: true`.
- The exporter does **not** synthesize Enterprise-only metrics (page cache hit ratio, query runtime breakdown, checkpoints, cluster, per-database CPU). See [`docs/COMPATIBILITY.md`](docs/COMPATIBILITY.md).

[0.1.1]: https://github.com/i-harsha-reddy/neo4j-exporter/releases/tag/0.1.1
[0.1.0]: https://github.com/i-harsha-reddy/neo4j-exporter/releases/tag/0.1.0
