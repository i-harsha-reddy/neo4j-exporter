# Compatibility matrix

What this exporter can and cannot expose, by Neo4j edition and APOC tier. Empty Grafana panels destroy operator trust — this doc is the authoritative explanation for what's intentionally absent.

> **Verified end-to-end (2026-05-08)** against `neo4j:5-community` with APOC Core + Extended and Jolokia 2.1.1. Every metric in the "exposes" table below was observed flowing during the e2e run.

## Editions

| Capability | CE 5.x / 2025.x | EE |
|---|---|---|
| Native Prometheus endpoint (`server.metrics.prometheus.enabled`) | ❌ | ✅ |
| Neo4j-specific JMX MBean tree (`neo4j.metrics:*`) | ❌ | ✅ |
| CSV metric files (`server.metrics.csv.enabled`) | ❌ | ✅ |
| Standard JVM MBeans (`java.lang:*`) | ✅ | ✅ |
| `SHOW TRANSACTIONS / DATABASES / INDEXES / CONSTRAINTS` | ✅ | ✅ |
| APOC Core (`apoc.meta.stats`, etc.) | ✅ | ✅ |
| APOC Extended (`apoc.monitor.*`) | ✅ (separate plugin) | ✅ |
| Multi-database | ❌ (1 user DB + system) | ✅ |
| Causal Clustering | ❌ | ✅ |

## Metrics this exporter exposes (full list)

| Metric | Source | Requires APOC Extended | Requires Jolokia |
|---|---|---|---|
| `neo4j_up` | Bolt ping | – | – |
| `neo4j_scrape_duration_seconds` | wall-clock | – | – |
| `neo4j_collector_success`, `_duration_seconds` | per-collector | – | – |
| `neo4j_neo4j_version_info` | `dbms.components()` | – | – |
| `neo4j_apoc_version_info` | `apoc.version()` | APOC Core | – |
| `neo4j_apoc_extended_available` | startup probe | – | – |
| `neo4j_bolt_handshake_duration_seconds` | driver | – | – |
| `neo4j_bolt_query_round_trip_seconds` | `RETURN 1` | – | – |
| `neo4j_database_status`, `_default`, `_access_info`, `_last_committed_txid`, `_creation_timestamp_seconds` | `SHOW DATABASES` | – | – |
| `neo4j_transactions_active`, `_longest_active_seconds`, `_active_cpu_seconds`, `_active_wait_seconds`, `_active_idle_seconds`, `_active_allocated_direct_bytes`, `_active_estimated_heap_bytes`, `_active_page_hits`, `_active_page_faults`, `_active_lock_count` | `SHOW TRANSACTIONS` (server-side aggregate) | – | – |
| `neo4j_transactions_committed_count`, `_rolled_back_count`, `_peak_concurrent`, `_currently_open`, `_opened_count`, `_last_id` | `apoc.monitor.tx` | ✅ | – |
| `neo4j_indexes_count`, `_index_population_percent`, `_index_read_count`, `_index_last_read_timestamp_seconds` | `SHOW INDEXES` | – | – |
| `neo4j_constraints_count` | `SHOW CONSTRAINTS` | – | – |
| `neo4j_kernel_info`, `_read_only`, `_start_timestamp_seconds`, `_store_creation_timestamp_seconds`, `_store_log_version` | `apoc.monitor.kernel` | ✅ | – |
| `neo4j_store_{node,relationship,property,string,array,log,total}_bytes` | `apoc.monitor.store` | ✅ | – |
| `neo4j_ids_in_use_total{kind}` | `apoc.monitor.ids` | ✅ | – |
| `neo4j_meta_{node,relationship,label,relationship_type,property_key}_count`, `_label_node_count`, `_relationship_type_count_by_type` | `apoc.meta.stats` | – (Core) | – |
| `neo4j_jvm_heap_*`, `_nonheap_*`, `_threads_*`, `_classes_*`, `_cpu_*`, `_open_fds`, `_max_fds`, `_load_average_1m`, `_uptime_seconds`, `_start_timestamp_seconds`, `_gc_*` | `java.lang:*` MBeans | – | ✅ |
| `neo4j_slow_query_info`, `_elapsed_seconds`, `_cpu_seconds`, `_wait_seconds`, `_page_faults`, `_allocated_bytes` | `SHOW TRANSACTIONS` (top-N at `/slow-queries`) | – | – |

## Metrics intentionally NOT in this exporter

These are Enterprise-only at the source. The exporter does **not** synthesize them, even when there are partial proxies, because faking a metric (e.g., a "page cache hit ratio" computed from per-tx page hits) would be misleading.

- **Page cache hit/fault rate** — needs `neo4j.dbms.page_cache.*` (Enterprise). The exporter's `neo4j_transactions_active_page_hits` / `_page_faults` are sums **across currently-active transactions only** — not a global cache hit ratio. Use them as a coarse "is the workload cache-bound right now?" signal, not as a replacement.
- **Query execution by runtime** (interpreted/slotted/pipelined breakdown) — Enterprise (`neo4j.dbms.query.execution.*`).
- **Checkpoint duration / frequency** — Enterprise (`neo4j.dbms.check_point.*`).
- **Cluster role transitions, replication lag** — Enterprise (`neo4j.dbms.cluster.*`); CE has no clustering.
- **Bolt connection accept/close counters** — Enterprise (`neo4j.dbms.bolt.*`).
- **Per-database CPU consumption** — Enterprise.
- **Per-pool memory tracking** — Enterprise (`neo4j.dbms.memory.pool.*`).

If you need these, run Neo4j Enterprise and use its native Prometheus endpoint — or, equivalently, run this exporter alongside it (the metric names are deliberately disjoint).
