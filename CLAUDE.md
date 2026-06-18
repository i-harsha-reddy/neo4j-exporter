# CLAUDE.md

Project memory for Claude Code working in this repo. Read this first.

## What this is

A **Prometheus exporter for Neo4j Community Edition 5.x / 2025.x**, written in Go. Single binary, [multi-target probe pattern](https://prometheus.io/docs/guides/multi-target-exporter/) (same shape as `blackbox_exporter` / `snmp_exporter`).

CE has no native Prometheus endpoint and no `neo4j.metrics:*` JMX MBean tree (both Enterprise-only). This exporter fills that gap by combining Cypher `SHOW` commands + APOC procedures (auto-detected) + JVM via Jolokia.

Approved design plan: `/Users/harshareddy/.claude/plans/lets-create-an-exporter-vectorized-shamir.md`.

**Status — verified end-to-end** (2026-05-08): `bash scripts/e2e.sh` runs the full docker-compose pipeline against **two identical Neo4j instances** (`neo4j-a:7687`, `neo4j-b:7687` — both `neo4j:5-community` + APOC Core + APOC Extended + `jolokia-agent-jvm 2.1.1`) plus the exporter + `prom/prometheus:v2.55.0` + `grafana/grafana:11.3.0` + a multi-scenario loadgen. The 17-step e2e asserts per-instance: `neo4j_up==1`, every always-on AND APOC AND Jolokia collector succeeded, `apoc_extended_available==1`, version is 5.x community, database online, JVM/store/kernel/ids present, `neo4j_constraints_count{type="UNIQUENESS"} > 0` (loadgen bootstrap), `/slow-queries` returns rows, `transactions_rolled_back_count > 0` (loadgen rollbacks scenario), index churn round-trips ONLINE. Then fleet-level: Prometheus `count(neo4j_up==1)==2`, per-instance neo4j_up==1, `neo4j-slow-queries` job up for both. All assertions are hard — no soft warnings.

## Architecture in one diagram

```
Prometheus ─/probe?target=&module=─► neo4j-exporter (:9412)
                                          │
                  ┌───────────────────────┼──────────────────┐
                  ▼                       ▼                  ▼
              Bolt 7687             Jolokia 8778      (Bolt + Jolokia
            (Cypher SHOW +          (java.lang:*       for next target)
             apoc.monitor.*)         MBeans)
```

The demo compose stack runs **two** Neo4j instances (`neo4j-a` on 7687/7474, `neo4j-b` on 7688/7475). The exporter probes both; Prometheus relabels `__address__` → `__param_target` → `instance`. Probe URLs:

```
curl 'http://localhost:9412/probe?target=neo4j-a:7687&module=default'
curl 'http://localhost:9412/probe?target=neo4j-b:7687&module=default'
```

Each `/probe` request gets a fresh `prometheus.NewRegistry()`. Collectors run in parallel (`errgroup`) with per-collector timeouts. Failures don't abort the probe — they surface as `neo4j_collector_success{collector="..."}=0`.

## Build & run

```bash
make build           # → ./neo4j-exporter
make test            # go test -race ./...
make lint            # golangci-lint (skipped if not installed)
make run             # uses config/neo4j-exporter.minimal.yml against localhost
make e2e             # bash scripts/e2e.sh (full docker-compose, ~5–6 min)
make docker          # build container image

# Helm chart (charts/neo4j-exporter/)
make chart-lint      # helm lint against ci/{minimal,full}-values.yaml
make chart-template  # render to dist/chart-render/{minimal,full}.yaml
make chart-test      # chart-lint + chart-template + kubeconform (if installed)
make chart-package   # → dist/neo4j-exporter-<version>.tgz
```

Direct invocation:
```bash
./neo4j-exporter --config.file=config/neo4j-exporter.minimal.yml --log.level=debug
curl 'http://localhost:9412/probe?target=localhost:7687&module=default'
curl 'http://localhost:9412/slow-queries?target=localhost:7687'
```

## Module path & Go version

- Module: `github.com/i-harsha-reddy/neo4j-exporter` (change in `go.mod`, `Makefile`, `Dockerfile`, `Dockerfile.release`, `.goreleaser.yml` if you fork).
- Go: `1.22+` (developed against `1.26.3` via Homebrew).
- Driver: `github.com/neo4j/neo4j-go-driver/v5` (v5 is current stable; v6 not pinned).

## Repository layout

```
cmd/neo4j-exporter/main.go     # kingpin flags, http.Server, signals, collector registration
internal/
  config/                       # YAML schema + secret/file interpolation + atomic reload
  probe/                        # /probe handler + orchestrator + /slow-queries
  collector/                    # one file per subsystem; all implement collector.Collector
  neo4jclient/                  # driver pool keyed by (uri, auth-hash) + APOC capability cache
  jolokiaclient/                # tiny HTTP→JMX bulk-read client
  version/                      # ldflags-injected build info
config/                         # *user-facing* example YAMLs (not Go)
dashboards/grafana/             # 6 dashboards: overview (fleet triage), transactions, storage, jvm, slow-queries, schema
docs/
  CONFIGURATION, METRICS, JOLOKIA, COMPATIBILITY, DEMO-STACK
examples/docker-compose/        # 2-instance demo stack (neo4j-a + neo4j-b + apoc + jolokia + exporter + prom + grafana + loadgen)
  loadgen/run.sh                # bash scheduler dispatching scenarios to N instances
  loadgen/scenarios/*.cypher    # bootstrap, hot_writes, contention, slow_traversals, index_churn, rollbacks, apoc_batch, schema_diversity
examples/kubernetes/            # Helm scenario values (basic/ha/existing-secret/network-policy) + Probe/ServiceMonitor (chart doesn't bundle CRDs)
charts/neo4j-exporter/          # production-grade Helm chart, published to oci://ghcr.io/i-harsha-reddy/charts
scripts/e2e.sh                  # CI-runnable end-to-end test (17 hard-fail assertions)
.github/workflows/              # ci, e2e, release, chart-test, chart-release
.claude/commands/               # slash commands for this project (see below)
.claude/skills/                 # project-specific skills
```

## Conventions specific to this repo

These are non-obvious rules that govern most edits. Violate them with reason, not by reflex.

### Metric naming

`neo4j_<subsystem>_<metric>_<unit>`. Subsystem corresponds to the collector file (`server`, `bolt`, `databases`, `transactions`, `indexes`, `constraints`, `kernel`, `store`, `ids`, `meta`, `jvm`). Always include the unit (`_seconds`, `_bytes`, `_total` for monotonic counters).

The exporter does **not** stamp `instance` itself — Prometheus relabel adds it. Don't add an `instance` label inside any collector.

### Cypher-side aggregation, not Go-side

`SHOW TRANSACTIONS YIELD ... RETURN database, status, count(*), max(elapsedTime), sum(...)` keeps the wire payload at O(databases × statuses) ≈ 12 rows. Never pull per-tx rows into Go for aggregation — that's how a busy DB will stall the probe. The only place row-level data flows is `/slow-queries`, bounded by `LIMIT $top`.

### `up` is set by the bolt collector ONLY

`neo4j_up=1` iff the bolt collector's `VerifyConnectivity` and `RETURN 1` both succeeded. APOC, Jolokia, or any other collector failing does NOT flip `up`. This matches operator intuition: dashboards turn red on `up==0` for "Neo4j is unreachable", not "APOC misconfigured". Per-collector status lives in `neo4j_collector_success{collector="..."}`.

### APOC auto-detect

`apoc.monitor.*` procedures are in **APOC Extended** (separate plugin). The pool runs `SHOW PROCEDURES YIELD name WHERE name IN [...]` once per target on first contact, caches the result, and the orchestrator filters collectors marked `auto` accordingly. Reload calls `InvalidateCapabilities()`.

### PII guard on slow queries

`/slow-queries` always emits `query_hash` (sha1[:8]). It emits `query_preview` (truncated to 200 chars, whitespace-collapsed) ONLY when the target's module config has `slow_query.expose_query_text: true`. Default is `false`. Never invert this default — Cypher query text often embeds PII as literals.

### Two Dockerfiles, two paradigms

The repo ships **two** Dockerfiles with the same runtime stage but different build models:

- `Dockerfile` — multi-stage **build-from-source** (`golang:1.26-alpine` builder → `gcr.io/distroless/static-debian12:nonroot` runtime). Used by `make docker` and any path that builds the image from a full source tree. Honors `--build-arg VERSION/REVISION/BRANCH/BUILD_USER/BUILD_DATE` so the in-image binary self-reports correct ldflags.
- `Dockerfile.release` — **wrap-prebuilt-binary** (`gcr.io/distroless/static-debian12:nonroot` only; ~10 lines). Used by `.goreleaser.yml`'s `dockers:` blocks, which build the binary externally with goreleaser-computed ldflags and stage **only** the binary + this Dockerfile into a minimal build context. Attempting to use the main `Dockerfile` here fails (`COPY go.mod go.sum`: `/go.sum` not found in build context).

If you change the runtime layer (USER, EXPOSE, ENTRYPOINT, CMD, base image), change **both**. They share nothing else by design.

### Secret-file mode validator

`checkSecretFileMode` (`internal/config/config.go`) accepts modes `0400`, `0440`, `0600`, `0640` — all four. The group-read variants (`0440`, `0640`) are required because kubelet's `fsGroup` mechanism OR's group-read into Secret-mounted file modes when fsGroup is set (which is the canonical K8s nonroot pattern). A `defaultMode: 0400` in a Pod spec becomes `0440` once the kubelet projects the Secret. The validator rejects any "other" perms (world-readable hazard) and any execute bits. Don't tighten this back to `0400`/`0600` only — the chart was found broken on first live install because of that.

### Cardinality safeguards

- `indexes` collector caps the `name` label at 500 distinct names (`<truncated>` after that).
- `apoc_meta` caps the `label` and `relationship_type` labels at 200 distinct values each.
- CE has 2 databases (`system` + user DB), so the `database` label is naturally bounded.
- Never add per-query-id labels to a `/probe` time-series metric. That goes on `/slow-queries` only.

### Per-request registry on `/probe`

The handler builds a fresh `prometheus.NewRegistry()` per request. Collectors register with that registry. Cross-probe metric leakage is impossible by construction. Self-metrics (`go_*`, `process_*`, `neo4j_exporter_*`) live on a separate exporter-side registry served at `/metrics`.

### Failure semantics

Each collector returns `error`. The orchestrator records `neo4j_collector_success{collector}=0/1` and `neo4j_collector_duration_seconds{collector}` for every collector that ran. A collector returning a Go error does NOT cause the probe to return non-200. Only outright driver-pool failures do that (and even then we still emit `up=0`).

### No bundled Prometheus alert/recording rules

The repo ships **no** `alerts/`, `recording.yml`, runbook index, or `rules-test` CI — they were intentionally removed. The exporter emits raw metrics; operators bring their own alerting/recording rules. **Consequence for dashboards:** Grafana panels that used to query `neo4j:`-namespaced recording rules now **inline the equivalent raw-metric PromQL** (e.g. `neo4j:heap_used_ratio` → `neo4j_jvm_heap_used_bytes / neo4j_jvm_heap_max_bytes`, `neo4j:tx_commit_rate:5m` → `rate(neo4j_transactions_committed_count[5m])`). Don't reintroduce `neo4j:`-prefixed metric names into the dashboards — they have no producer now and the panels would go blank. If you re-add a recording-rule layer, update the dashboards in the same change.

### What is NOT in this exporter

These are Enterprise-only at the source. The exporter does NOT synthesize them — see `docs/COMPATIBILITY.md`:
- Page cache hit/fault rate (per-tx page hits/faults are NOT a substitute)
- Query execution by runtime (interpreted/slotted/pipelined)
- Checkpoint duration / frequency
- Cluster role / replication lag
- Bolt connection accept/close counters
- Per-database CPU
- Per-pool memory tracking

If a future task asks to add any of these, push back: explain that they require Enterprise.

## Adding a new collector

Follow the `add-collector` skill in `.claude/skills/add-collector/SKILL.md`. Short version:

1. New file under `internal/collector/<name>.go` with a struct implementing `Collector`.
2. Register it in `cmd/neo4j-exporter/main.go` `registerCollectors()`.
3. Add the toggle to known collectors in `internal/config/config.go` `knownCollector()`.
4. Add the toggle to `config/neo4j-exporter.example.yml`, `examples/docker-compose/neo4j-exporter/config.yml`, AND `charts/neo4j-exporter/values.yaml` (`config.modules.default.collectors`). All three drift independently if you forget any.
5. Add the metric(s) to `docs/METRICS.md`.
6. Optionally: panel(s) in a Grafana dashboard.

## Helm chart

Production-grade chart at `charts/neo4j-exporter/`. Verified via `helm lint` and `helm template` against `ci/{minimal,full}-values.yaml`. Non-obvious bits:

- **Single source of truth for the exporter image tag.** `image.tag` defaults to `.Chart.AppVersion`; bumping appVersion in `Chart.yaml` is what propagates a binary version. Don't hard-code tags in `values.yaml`.
- **Secret precedence.** `auth.existingSecret` wins; if empty, the chart renders a `Secret` from `auth.password` / `auth.jolokiaPassword`. Validation: empty BOTH triggers a `fail` with a clear message; partial triggers a `required` on the missing field. Do NOT relax these — silently rendering an empty Secret was the alternative.
- **Volume mount paths are stable, Secret keys are configurable.** Inside the pod, the files are always `/etc/neo4j-exporter/secrets/{password,jolokia-password}` — that's what the `password_file:` paths in `config.default_target` reference. The user picks the Secret keys (`auth.passwordKey`, `auth.jolokiaPasswordKey`); the deployment's `secret.items[].key→path` mapping translates between them.
- **Config reloads via pod restart.** `checksum/config` and (when chart-managed) `checksum/secret` annotations on the pod template trigger rolling restarts when the rendered ConfigMap/Secret content changes. The exporter also supports SIGHUP/`POST /-/reload` but the chart prefers the rollout.
- **PDB auto-enable.** Renders if `replicaCount > 1` OR `autoscaling.enabled`. Set `podDisruptionBudget.enabled: false` to force-disable. A single-replica Deployment with a `minAvailable: 1` PDB is a node-drain footgun; the chart guards against this.
- **No CRDs are bundled.** No Probe/ServiceMonitor/PrometheusRule/dashboard ConfigMaps. Operators who run prometheus-operator should apply `examples/kubernetes/servicemonitor.yaml` separately and adjust the `prober.url` to match their release name. This was a deliberate scope decision — bundling CRDs creates a hard dependency on operator CRDs being installed cluster-wide.
- **Publishing.** OCI to `oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter` on `v*`/`chart-v*` tags via `.github/workflows/chart-release.yml`. Cosign keyless signing of the OCI artifact. No GitHub Pages chart repo by design.
- **Chart-touching CI.** `.github/workflows/chart-test.yml` runs `helm lint`, `kubeconform`, and `ct install` on a kind cluster on PRs that touch `charts/**`.

## Slash commands available in this repo

See `.claude/commands/`:
- `/check` — vet + race tests + build
- `/probe` — issue an ad-hoc `/probe` to a running exporter
- `/compose-up`, `/compose-down` — bring the demo stack up/down
- `/e2e` — run `scripts/e2e.sh` end-to-end
- `/release-snapshot` — `goreleaser` snapshot build (no publish)

## Reference patterns (when in doubt, look at these)

- [`prometheus/blackbox_exporter`](https://github.com/prometheus/blackbox_exporter) — multi-target probe handler shape.
- [`prometheus/snmp_exporter`](https://github.com/prometheus/snmp_exporter) — module + auth split.
- [`prometheus-community/postgres_exporter`](https://github.com/prometheus-community/postgres_exporter) — collector toggles, queries.yaml pattern (intentionally NOT used here).

## Gotchas worth remembering

- **Prometheus instant-query JSON shape.** `/api/v1/query` returns each sample as `"value":[<ts>,"<value>"]` where the timestamp is a **raw number** (not a quoted string). Any test or polling script that greps the response must use a pattern like `'"value":\[[0-9.]+,"1"\]'` (extended regex). The earlier e2e script used a quoted-timestamp pattern and silently never matched, hiding the fact that the data was there.
- **`docker compose up --wait` returns when containers are HEALTHY, not when Prometheus has scraped anything.** All targets show `health=unknown` for ~5–10s after compose finishes. Tests that verify Prometheus state (target health, queryable samples) must poll, not single-shot. The same race applies one layer lower: container-healthy ≠ Neo4j ready for Cypher queries. On slower CI runners, neo4j-b's `transactions` collector has been observed to still be racing the bolt connection at the exact moment the e2e starts its first probe assertion. `scripts/e2e.sh` therefore runs an adaptive readiness wait (max 120 s, polls every 3 s) between `compose up --wait` and step 1, requiring every always-on collector on **both** instances to report `neo4j_collector_success=1` before proceeding. If you ever shorten that wait, re-test in GHA, not just locally.
- **`SHOW TRANSACTIONS` returns `elapsedTime`/`cpuTime`/`waitTime`/`idleTime` as Cypher Durations**, which the v5 Go driver surfaces as **`neo4j.Duration`** (alias of `dbtype.Duration`: `Months/Days/Seconds int64`, `Nanos int`) — **NOT** Go's `time.Duration`. Both `asFloat64` (`internal/collector/coerce.go`) and `durSeconds` (`internal/probe/slowqueries.go`) carry an explicit `case neo4j.Duration` that converts to total seconds (`float64(Months)*2629746 + Days*86400 + Seconds + Nanos/1e9`); `coerce_test.go` and `slowqueries_test.go` pin this. **History (2026-06-16):** before that branch existed both helpers fell through to `return 0`, silently zeroing every elapsed-based metric — `neo4j_transactions_longest_active_seconds`, `_active_{cpu,wait,idle}_seconds`, and all `neo4j_slow_query_{elapsed,cpu,wait}_seconds`. This is the bug that made the Slow Queries dashboard look "always empty" (the in-flight count, max-elapsed, and `>10s` backlog panels all key off elapsed). e2e never caught it because step 11 only asserts `neo4j_slow_query_info` rows exist, never their values — when extending, assert a numeric elapsed > 0, not just row presence. Keep these branches; do NOT assume `time.Duration`.
- **APOC-monitor outputs are mixed types, and the shape drifts across APOC versions.** `kernelStartTime` / `storeCreationDate` from `apoc.monitor.kernel` are **not** stable: older APOC returned ms-since-epoch numbers, but current APOC Extended (verified on Neo4j 5.26.26) returns formatted date **strings** like `"2026-06-08 09:48:04"` (no timezone — the Neo4j JVM defaults to UTC in containers). `asUnixSeconds` (`internal/collector/apoc_kernel.go`) handles all three forms: `time.Time`, numeric ms-since-epoch (divides by 1000 when > `1e10`), and the date-string layouts (parsed as UTC). If these two metrics ever read 0 again, re-check the live procedure output first — APOC likely changed the field type again. `apoc_kernel_test.go` pins this behavior. (`apoc.monitor.store` legitimately reports `nodeStoreSize`/`relStoreSize` as 0 before the first checkpoint — that's Neo4j, not a bug.)
- **Per-tx page hits/faults ≠ global page cache hit ratio.** Repeat: do NOT compute a fake hit ratio from `neo4j_transactions_active_page_hits` / `_page_faults`. They are summed across currently-active transactions only. The dashboards label them clearly; a future PR adding a `cache_hit_ratio` derived metric should be rejected.
- **Cypher refuses to compare Durations with `<,<=,>,>=`** (returns NULL because month-bearing durations have indeterminate length). Use `.nanoseconds` (total) or `.milliseconds` (total) accessors and divide. Existing example: `WHERE toFloat(elapsedTime.nanoseconds) / 1000000000.0 >= $min` in `internal/probe/slowqueries.go`. Don't revert that to `WHERE elapsedTime >= duration({seconds: $min})` — `/slow-queries` will silently return zero rows.
- **`kernel_info.store_id` value contains literal `{...}` braces** (it's a Java struct.toString()). When grepping metrics output, use `\{.*\}` not `\{[^}]*\}` for label-block matching on this metric. The e2e has a comment marking the assertion that hit this.
- **Cross-instance slow-query joins** must include `instance` in the `on()` clause: `_elapsed_seconds * on(instance, transaction_id) group_left(...) _info`. Two instances number transactions independently, so the same `transaction_id` value can appear on both. Without `instance`, PromQL returns "duplicate series for the match group".
- **APOC capability cache is per-target, populated on first contact, never TTL'd.** If `SHOW PROCEDURES` runs before APOC Extended finishes loading, capabilities are cached `false` forever (until reload). The exporter's compose health-check waits 60s `start_period` to avoid this. If you change healthcheck timing, re-test the e2e.
- **Timestamp metrics are seconds; Grafana `dateTimeFromNow`/`dateTimeAsIso` units want milliseconds.** Every `neo4j_*_timestamp_seconds` metric is emitted in Unix **seconds** (correct Prometheus convention). Grafana's date-time field units read the raw number as **milliseconds**, so a panel that points a seconds value at `dateTimeFromNow` renders ~1970 ("56 years ago"). Any dashboard panel displaying a `*_timestamp_seconds` metric as a date MUST multiply the query by `1000` (e.g. `neo4j_jvm_start_timestamp_seconds{...} * 1000`). This applies to the "JVM started" stat (`neo4j-jvm.json`), the kernel-identity table's "kernel started"/"store created" columns (`neo4j-storage.json`), and "Coldest indexes (last read)" (`neo4j-schema.json`). Don't fix this by changing the metric to milliseconds — the `_seconds` suffix and unit are correct; the `* 1000` belongs in the dashboard query.
