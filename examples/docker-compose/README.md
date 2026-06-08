# docker-compose demo stack

End-to-end stack for trying out neo4j-exporter without installing anything except Docker.

## Tested versions

Verified end-to-end with `bash scripts/e2e.sh` on 2026-05-08:

| Component | Version |
|---|---|
| Neo4j | `neo4j:5-community` |
| APOC | Core + Extended (via `NEO4J_PLUGINS=["apoc","apoc-extended"]`) |
| Jolokia agent | `jolokia-agent-jvm 2.1.1` |
| Prometheus | `prom/prometheus:v2.55.0` |
| Grafana | `grafana/grafana:11.3.0` |
| Docker Engine | 29.x |
| Docker Compose | v2 (bundled with recent Docker Desktop) |

First-time `docker compose up` pulls ~2.5 GB of images (neo4j is ~983 MB on its own); subsequent runs reuse the cache.

## Services

| Service | Port (host) | Purpose |
|---|---|---|
| `jolokia-init` | – | one-shot: downloads the Jolokia agent jar into a shared volume |
| `neo4j-a` | 7474, 7687 | Neo4j 5 CE + APOC Core + APOC Extended + Jolokia agent loaded |
| `neo4j-b` | 7475, 7688 | identical second instance — validates the multi-target probe pipeline |
| `neo4j-exporter` | 9412 | the exporter, built from this repo's `Dockerfile` |
| `prometheus` | 9090 | scrapes `/probe` and `/slow-queries` for both instances |
| `grafana` | 3000 | provisions Prometheus datasource + dashboards |
| `loadgen` | – | cypher-shell scheduler dispatching scenarios to both instances; see [`loadgen/README.md`](loadgen/README.md) |

Note: Jolokia (port 8778) is **deliberately not** published to the host. The exporter and Neo4j share the compose network. Don't change this — Jolokia is a JMX gateway and should never be exposed publicly.

## Run

```bash
docker compose up -d --wait
```

First boot takes ~60s because Neo4j downloads APOC Extended from labs and the init container fetches the Jolokia jar.

Then:
- Grafana: http://localhost:3000 (anonymous Admin)
- Prometheus: http://localhost:9090
- Neo4j Browser (instance a): http://localhost:7474 (`neo4j` / `dev_password_change_me`)
- Neo4j Browser (instance b): http://localhost:7475 (`neo4j` / `dev_password_change_me`)
- Exporter probe: http://localhost:9412/probe?target=neo4j-a:7687&module=default (or `neo4j-b:7687`)
- Slow queries: http://localhost:9412/slow-queries?target=neo4j-a:7687

## Tear down

```bash
docker compose down -v --remove-orphans
```

`-v` removes the `jolokia-data` volume so the next run re-downloads the jar fresh.

## Customizing

- Demo password is `dev_password_change_me`. Change in `docker-compose.yml` (`NEO4J_AUTH` on both Neo4j services + `LOADGEN_PASSWORD` on loadgen) and in `neo4j-exporter/config.yml` (both targets).
- The exporter's config lives at `neo4j-exporter/config.yml`. `slow_query.expose_query_text: true` is enabled here for demo visibility — **do not copy this to production**.
- Demo Jolokia is plaintext + no auth (binds 0.0.0.0:8778 inside the network). Production deployments must use HTTPS + basic auth + restrictive `policy.xml` per [`docs/JOLOKIA.md`](../../docs/JOLOKIA.md).
