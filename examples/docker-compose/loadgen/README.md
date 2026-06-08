# Demo Loadgen

Cypher-shell + bash. Runs continuous traffic across N Neo4j instances so the
exporter's dashboards have real signal during a `make compose-up` session.

## Scenarios

| File | Cadence | Produces |
|---|---|---|
| `bootstrap.cypher` | once at startup | 3 UNIQUENESS constraints, 6 indexes (3 implicit + 3 explicit), 50k Account, 5k Person, 50k KNOWS rels, 10 `:HotSpot` ids |
| `hot_writes.cypher` | `LOADGEN_RATE_HOT_WRITES` (default 4s) | continuous trickle of `MATCH ... SET balance = ...` against random accounts |
| `contention.cypher` | `LOADGEN_CONTENTION_S` (default 5s), 4× concurrent | hammers the 10 `:HotSpot` accounts → `neo4j_transactions_active_wait_seconds` and `_active_lock_count` move |
| `slow_traversals.cypher` | `LOADGEN_SLOW_INTERVAL_S` (default 30s) | variable-length `KNOWS*1..6` path + capped cartesian → `/slow-queries` populated |
| `index_churn.cypher` | `LOADGEN_INDEX_CHURN_S` (default 90s) | drops + recreates `account_balance` → `neo4j_index_population_percent` emits during the population window |
| `rollbacks.cypher` | `LOADGEN_ROLLBACK_S` (default 20s) | `CREATE` violating `account_id_unique` → `neo4j_transactions_rolled_back_count` strictly grows |
| `apoc_batch.cypher` | `LOADGEN_APOC_BATCH_S` (default 60s) | `apoc.periodic.iterate` 200k creates + sweep delete → `neo4j_jvm_gc_count_total` moves |
| `schema_diversity.cypher` | `LOADGEN_SCHEMA_DIVERSITY_S` (default 300s), first instance only | rotates through `Tag1..Tag20` labels and `REL_T1..REL_T20` types via `apoc.create.node`/`apoc.create.relationship` |

All scenarios are idempotent (`MERGE` / `IF EXISTS` / `IF NOT EXISTS` / `ON CREATE`)
and use only CE-compatible constraint syntax (`IS UNIQUE`, including composite).

## Env-var knobs

| Var | Default | Effect |
|---|---|---|
| `LOADGEN_TARGETS` | `a=neo4j-a:7687,b=neo4j-b:7687` | comma-separated `name=host:port` pairs |
| `LOADGEN_USERNAME` | `neo4j` | shared bolt username |
| `LOADGEN_PASSWORD` | `dev_password_change_me` | shared bolt password |
| `LOADGEN_RATE_HOT_WRITES` | `4` | hot_writes cadence in seconds |
| `LOADGEN_CONTENTION_S` | `5` | contention cadence in seconds |
| `LOADGEN_SLOW_INTERVAL_S` | `30` | slow-traversal cadence |
| `LOADGEN_INDEX_CHURN_S` | `90` | index drop+recreate cadence |
| `LOADGEN_APOC_BATCH_S` | `60` | apoc.periodic.iterate cadence |
| `LOADGEN_ROLLBACK_S` | `20` | constraint-violation cadence |
| `LOADGEN_SCHEMA_DIVERSITY_S` | `300` | schema-rotation cadence |
| `LOADGEN_DISABLE` | `` | comma-sep scenario names to skip — useful on low-RAM laptops, e.g. `apoc_batch,schema_diversity` |
| `LOADGEN_LOG` | `info` | `quiet` / `info` / `debug` |

## Single-shot mode

`scripts/e2e.sh` uses this to make the index-population assertion deterministic:

```sh
docker compose -f examples/docker-compose/docker-compose.yml \
  exec loadgen bash run.sh --once index_churn neo4j-a:7687
```

Accepts either the target name (`a`) or the address (`neo4j-a:7687`).
