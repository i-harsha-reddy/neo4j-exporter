# Contributing

Thanks for considering a contribution! This project is small and opinionated; the goal of this guide is to keep the contribution loop fast.

## Code of conduct

Be respectful. Stick to the technical merits of an idea. We follow the spirit of the [Contributor Covenant](https://www.contributor-covenant.org/).

## Development setup

```bash
git clone https://github.com/i-harsha-reddy/neo4j-exporter
cd neo4j-exporter
make build
```

Required toolchain:

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.22+ | builds the exporter |
| Docker | any recent | container image, e2e |
| Helm | 3.8+ (incl. 4.x) | chart work |
| kind | any recent | live chart testing |
| `golangci-lint` | v2.x (CI pins **v2.12.2**) | static analysis. `make lint` no-ops locally if absent; CI is authoritative. Install via `brew install golangci-lint` or `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2` |
| `kubeconform` | optional | chart schema validation |
| `goreleaser` | optional (only for snapshot builds) | `goreleaser release --snapshot --clean` produces the same artifact set CI publishes on tag. Uses `Dockerfile.release` for the docker step |

The exporter has no Go-side cgo dependencies; it builds and runs on Linux, macOS, and Windows.

## Build, test, lint

```bash
make build           # → ./neo4j-exporter
make test            # go test -race ./... (no live DB needed)
make lint            # golangci-lint (skipped if not installed)
make e2e             # full docker-compose E2E (~5–6 min, 17 assertions, requires Docker)
make chart-lint      # helm lint against ci/{minimal,full}-values.yaml
make chart-template  # render templates to dist/chart-render/
make chart-test      # chart-lint + chart-template + kubeconform
make chart-package   # → dist/neo4j-exporter-<version>.tgz
```

`make e2e` runs `scripts/e2e.sh`, which spins up the **2-instance demo stack** (`neo4j-a` + `neo4j-b` — both Neo4j CE + APOC Core + APOC Extended + Jolokia, plus exporter + Prometheus + Grafana + multi-scenario loadgen) and runs 17 hard-fail assertions including per-instance collector success, joined slow-query rows, deliberate rollbacks, index-churn round-trip, and Prometheus fleet-level checks. Use it before any change that touches collectors, the orchestrator, the driver pool, or the demo compose / loadgen.

## Repo conventions

These are all in [`CLAUDE.md`](CLAUDE.md) — read that file before non-trivial work. The short version:

- **Metric naming.** `neo4j_<subsystem>_<metric>_<unit>`. Always include the unit (`_seconds`, `_bytes`, `_total` for monotonic counters). The exporter does not stamp `instance` itself — Prometheus relabel does.
- **Cypher-side aggregation.** Use `RETURN database, status, count(*), max(...), sum(...)` shape; never pull per-tx rows into Go.
- **`up` is set by the bolt collector only.** APOC, Jolokia, or any other collector failing does not flip `up`.
- **PII guard on slow queries.** `expose_query_text` defaults `false`. Don't invert.
- **Cardinality safeguards.** `indexes` caps `name` at 500; `apoc_meta` caps `label`/`relationship_type` at 200 each.

## Adding a new collector

Walkthrough in [`.claude/skills/add-collector/SKILL.md`](.claude/skills/add-collector/SKILL.md). Minimum touch set:

1. New file under `internal/collector/<name>.go` implementing `collector.Collector`.
2. Register it in `cmd/neo4j-exporter/main.go` `registerCollectors()`.
3. Add the toggle to `internal/config/config.go` `knownCollector()`.
4. Add the toggle to all three places it lives: `config/neo4j-exporter.example.yml`, `examples/docker-compose/neo4j-exporter/config.yml`, and `charts/neo4j-exporter/values.yaml` (`config.modules.default.collectors`). They drift independently if you forget any.
5. Document the metric(s) in `docs/METRICS.md`.
6. Optional but encouraged: Grafana panel(s) on the appropriate dashboard.

## Pull request checklist

Before opening a PR:

- [ ] `make test` (passes, no race warnings)
- [ ] `make lint` (clean, or you've explained the warning)
- [ ] If you touched a collector or the demo stack: `make e2e` passes
- [ ] If you touched the chart: `make chart-test` passes; values.yaml comments updated
- [ ] If you touched documented metrics: `docs/METRICS.md` updated
- [ ] Commit messages explain *why*, not *what*

PRs are reviewed for: correctness, fit with the conventions above, test coverage on non-trivial logic, and explicit "why" reasoning when going against a stated convention.

## Reporting bugs

Please include:

- exporter version (`/metrics` exposes `neo4j_exporter_build_info`)
- Neo4j version (`SHOW DATABASES` or the exporter's `neo4j_neo4j_version_info`)
- APOC tier (`neo4j_apoc_extended_available`)
- Whether you're scraping via `helm install` or raw manifests
- Output of `curl -s 'http://<exporter>:9412/probe?target=<neo4j>:7687'` (truncate / sanitize as needed)

Security-sensitive issues: see [`SECURITY.md`](SECURITY.md), do not file public issues.

## License

By contributing, you agree your contributions are licensed under [Apache-2.0](LICENSE).
