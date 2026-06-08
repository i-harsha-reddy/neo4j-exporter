# Demo Stack

End-to-end docker-compose stack used by `make compose-up`, `make e2e`, and the
GitHub Actions e2e workflow. Lives at `examples/docker-compose/`.

## Topology

```
                         ┌─────────────┐
                         │  prometheus │  9090 → host
                         └──────┬──────┘
                                │ /probe?target=neo4j-{a,b}:7687
                         ┌──────▼──────┐
                         │   exporter  │  9412 → host
                         └──────┬──────┘
                                │
              ┌─────────────────┼─────────────────┐
              │                 │                 │
        ┌─────▼─────┐    ┌──────▼─────┐    ┌──────▼─────┐
        │  neo4j-a  │    │  neo4j-b   │    │  loadgen   │
        │ 7687/7474 │    │ 7688/7475  │    │  scheduler │
        │  + APOC + │    │  + APOC +  │    └────────────┘
        │  Jolokia  │    │  Jolokia   │
        └───────────┘    └────────────┘

                         ┌─────────────┐
                         │   grafana   │  3000 → host
                         └─────────────┘
```

Two **identical** Neo4j Community 5.x instances, both with APOC Core + APOC
Extended + Jolokia. The point is to validate the multi-target probe pipeline
(blackbox-shaped relabel: `__address__` → `__param_target` → `instance`), not
to test version diversity.

## Host ports

| Service | Port | Path / purpose |
|---|---|---|
| `neo4j-a` HTTP UI | 7474 | `http://localhost:7474` |
| `neo4j-a` Bolt | 7687 | `bolt://neo4j/dev_password_change_me@localhost:7687` |
| `neo4j-b` HTTP UI | 7475 | `http://localhost:7475` |
| `neo4j-b` Bolt | 7688 | `bolt://neo4j/dev_password_change_me@localhost:7688` |
| `neo4j-exporter` | 9412 | `http://localhost:9412/metrics`, `/probe?target=...`, `/slow-queries?target=...` |
| `prometheus` | 9090 | `http://localhost:9090` |
| `grafana` | 3000 | `http://localhost:3000` (anonymous Admin) |

Jolokia (8778) intentionally NOT published — exporter reaches it inside the network at `neo4j-{a,b}:8778`.

## Loadgen

See [`examples/docker-compose/loadgen/README.md`](../examples/docker-compose/loadgen/README.md) for the full scenario list and env-var knobs. In short, after `bootstrap.cypher` runs once per instance, the scheduler keeps the following metrics fresh:

- `neo4j_constraints_count{type="UNIQUENESS"}` — 3 per instance
- `neo4j_indexes_count` — 6 per instance, with periodic `state="POPULATING"` from `index_churn`
- `neo4j_transactions_committed_count`, `_rolled_back_count`, `_active_lock_count`, `_active_wait_seconds`
- `neo4j_jvm_gc_count_total` (driven by `apoc_batch`)
- `/slow-queries` always populated (driven by `slow_traversals`)
- `neo4j_meta_label_node_count`, `_relationship_type_count_by_type` (driven by `schema_diversity`)

## Resource footprint

- 2× Neo4j (1 GB heap + 512 MB pagecache + ~500 MB JVM overhead per instance) → ~4 GB
- exporter ~50 MB, prometheus ~150 MB, grafana ~250 MB, loadgen ~80 MB
- **Steady state ~5 GB resident**, well under the 7 GB GitHub-runner cap.

For low-RAM laptops, set `LOADGEN_DISABLE=apoc_batch,schema_diversity` in the loadgen service block to skip the heaviest scenarios.

## Running

```sh
make compose-up      # → http://localhost:3000
make compose-down    # cleanup
make e2e             # full assertion suite (~5 min)

# Probe the exporter directly:
curl 'http://localhost:9412/probe?target=neo4j-a:7687&module=default'
curl 'http://localhost:9412/probe?target=neo4j-b:7687&module=default'
curl 'http://localhost:9412/slow-queries?target=neo4j-a:7687&top=5'
```

## Note for users coming from the single-instance stack

The probe URL changed: `target=neo4j:7687` is no longer valid. Use `target=neo4j-a:7687` or `target=neo4j-b:7687`.
