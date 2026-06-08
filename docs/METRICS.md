# Metrics reference

Generated from collector source files. All metrics labelled `[ext]` require **APOC Extended**; metrics labelled `[core]` require **APOC Core**; metrics labelled `[jolokia]` require Jolokia. Everything else works on a vanilla Neo4j 5.x Community installation.

| Metric | Type | Labels | Description |
|---|---|---|---|
| `neo4j_up` | gauge | – | 1 if the Bolt port responded successfully, else 0. Set by the bolt collector. |
| `neo4j_scrape_duration_seconds` | gauge | – | Total wall duration of the probe. |
| `neo4j_collector_success` | gauge | `collector` | 1 if the named collector succeeded, else 0. |
| `neo4j_collector_duration_seconds` | gauge | `collector` | Duration of the named collector. |
| `neo4j_neo4j_version_info` | gauge=1 | `version`,`edition` | From `dbms.components()`. |
| `neo4j_apoc_version_info` | gauge=1 | `version` | `[core]` From `apoc.version()`. |
| `neo4j_apoc_extended_available` | gauge | – | 1 if `apoc.monitor.*` procedures are detected on the target. |
| `neo4j_bolt_handshake_duration_seconds` | gauge | – | Time to verify Bolt connectivity. |
| `neo4j_bolt_query_round_trip_seconds` | gauge | – | Time for `RETURN 1` over Bolt. |
| `neo4j_database_status` | gauge | `database`,`status` | 1 if the database is in the labeled status. |
| `neo4j_database_default` | gauge | `database` | 1 if the database is the default user database. |
| `neo4j_database_access_info` | gauge=1 | `database`,`access` | Access mode (read-write / read-only). |
| `neo4j_database_last_committed_txid` | gauge | `database` | Last committed txid (monotonic per DB). |
| `neo4j_database_creation_timestamp_seconds` | gauge | `database` | Database creation time, Unix seconds. |
| `neo4j_transactions_active` | gauge | `database`,`status` | Currently active transactions per (db, status). |
| `neo4j_transactions_longest_active_seconds` | gauge | `database` | Elapsed time of the oldest active transaction. |
| `neo4j_transactions_active_cpu_seconds` | gauge | `database` | Sum of cpuTime across active txns. |
| `neo4j_transactions_active_wait_seconds` | gauge | `database` | Sum of waitTime (lock acquisition). |
| `neo4j_transactions_active_idle_seconds` | gauge | `database` | Sum of idleTime. |
| `neo4j_transactions_active_allocated_direct_bytes` | gauge | `database` | Sum of direct (off-heap) bytes. |
| `neo4j_transactions_active_estimated_heap_bytes` | gauge | `database` | Sum of estimated heap bytes. |
| `neo4j_transactions_active_page_hits` | gauge | `database` | Sum of pageHits across active txns (per-tx counter, NOT global). |
| `neo4j_transactions_active_page_faults` | gauge | `database` | Sum of pageFaults across active txns (per-tx counter, NOT global). |
| `neo4j_transactions_active_lock_count` | gauge | `database` | Sum of activeLockCount. |
| `neo4j_transactions_committed_count` | gauge | `database` | `[ext]` Total committed (resets on restart). |
| `neo4j_transactions_rolled_back_count` | gauge | `database` | `[ext]` Total rolled back. |
| `neo4j_transactions_peak_concurrent` | gauge | `database` | `[ext]` Peak concurrent since last restart. |
| `neo4j_transactions_currently_open` | gauge | `database` | `[ext]` Currently open transactions per APOC. |
| `neo4j_transactions_opened_count` | gauge | `database` | `[ext]` Total transactions opened. |
| `neo4j_transactions_last_id` | gauge | `database` | `[ext]` Last transaction id. |
| `neo4j_indexes_count` | gauge | `database`,`state`,`type`,`entity_type` | Index count by state/type/entity. |
| `neo4j_index_population_percent` | gauge | `database`,`name`,`type` | Population progress (only for POPULATING). |
| `neo4j_index_read_count` | gauge | `database`,`name`,`type` | Cumulative reads per index. |
| `neo4j_index_last_read_timestamp_seconds` | gauge | `database`,`name`,`type` | Last read time. |
| `neo4j_constraints_count` | gauge | `database`,`type`,`entity_type` | Constraint count grouped. |
| `neo4j_kernel_info` | gauge=1 | `database_name`,`kernel_version`,`store_id` | `[ext]` Static kernel info. |
| `neo4j_kernel_read_only` | gauge | `database_name` | `[ext]` 1 if read-only. |
| `neo4j_kernel_start_timestamp_seconds` | gauge | `database_name` | `[ext]` Kernel start time. |
| `neo4j_kernel_store_creation_timestamp_seconds` | gauge | `database_name` | `[ext]` Store creation time. |
| `neo4j_kernel_store_log_version` | gauge | `database_name` | `[ext]` Store log version. |
| `neo4j_store_node_bytes` | gauge | – | `[ext]` Node store size. |
| `neo4j_store_relationship_bytes` | gauge | – | `[ext]` Relationship store size. |
| `neo4j_store_property_bytes` | gauge | – | `[ext]` Property store size. |
| `neo4j_store_string_bytes` | gauge | – | `[ext]` String store size. |
| `neo4j_store_array_bytes` | gauge | – | `[ext]` Array store size. |
| `neo4j_store_log_bytes` | gauge | – | `[ext]` Transaction log size. |
| `neo4j_store_total_bytes` | gauge | – | `[ext]` Total store size. |
| `neo4j_ids_in_use_total` | gauge | `kind` | `[ext]` IDs in use by kind. |
| `neo4j_meta_node_count` | gauge | `database` | `[core]` Total nodes. |
| `neo4j_meta_relationship_count` | gauge | `database` | `[core]` Total relationships. |
| `neo4j_meta_label_count` | gauge | `database` | `[core]` Distinct labels. |
| `neo4j_meta_relationship_type_count` | gauge | `database` | `[core]` Distinct rel types. |
| `neo4j_meta_property_key_count` | gauge | `database` | `[core]` Distinct property keys. |
| `neo4j_meta_label_node_count` | gauge | `database`,`label` | `[core]` Nodes per label (capped). |
| `neo4j_meta_relationship_type_count_by_type` | gauge | `database`,`relationship_type` | `[core]` Relationships per type (capped). |
| `neo4j_jvm_heap_used_bytes` | gauge | – | `[jolokia]` |
| `neo4j_jvm_heap_committed_bytes` | gauge | – | `[jolokia]` |
| `neo4j_jvm_heap_max_bytes` | gauge | – | `[jolokia]` |
| `neo4j_jvm_nonheap_used_bytes` | gauge | – | `[jolokia]` |
| `neo4j_jvm_nonheap_committed_bytes` | gauge | – | `[jolokia]` |
| `neo4j_jvm_nonheap_max_bytes` | gauge | – | `[jolokia]` (omitted if max=−1) |
| `neo4j_jvm_threads_current` | gauge | – | `[jolokia]` |
| `neo4j_jvm_threads_daemon` | gauge | – | `[jolokia]` |
| `neo4j_jvm_threads_peak` | gauge | – | `[jolokia]` |
| `neo4j_jvm_threads_started_total` | gauge | – | `[jolokia]` Monotonic counter (gauge for JVM-restart safety). |
| `neo4j_jvm_classes_loaded` | gauge | – | `[jolokia]` |
| `neo4j_jvm_classes_loaded_total` | gauge | – | `[jolokia]` |
| `neo4j_jvm_classes_unloaded_total` | gauge | – | `[jolokia]` |
| `neo4j_jvm_cpu_process_ratio` | gauge | – | `[jolokia]` 0–1 |
| `neo4j_jvm_cpu_system_ratio` | gauge | – | `[jolokia]` 0–1 |
| `neo4j_jvm_open_fds` | gauge | – | `[jolokia]` |
| `neo4j_jvm_max_fds` | gauge | – | `[jolokia]` |
| `neo4j_jvm_load_average_1m` | gauge | – | `[jolokia]` |
| `neo4j_jvm_uptime_seconds` | gauge | – | `[jolokia]` |
| `neo4j_jvm_start_timestamp_seconds` | gauge | – | `[jolokia]` |
| `neo4j_jvm_gc_count_total` | gauge | `gc` | `[jolokia]` Per-GC collection count. |
| `neo4j_jvm_gc_seconds_total` | gauge | `gc` | `[jolokia]` Per-GC time. |
| `neo4j_slow_query_info` | info | `database`,`transaction_id`,`username`,`status`,`query_hash`,`query_preview?` | At `/slow-queries`. |
| `neo4j_slow_query_elapsed_seconds` | gauge | `transaction_id` | At `/slow-queries`. |
| `neo4j_slow_query_cpu_seconds` | gauge | `transaction_id` | At `/slow-queries`. |
| `neo4j_slow_query_wait_seconds` | gauge | `transaction_id` | At `/slow-queries`. |
| `neo4j_slow_query_page_faults` | gauge | `transaction_id` | At `/slow-queries`. |
| `neo4j_slow_query_allocated_bytes` | gauge | `transaction_id` | At `/slow-queries`. |

## Dashboards

Six Grafana dashboards under [`dashboards/grafana/`](../dashboards/grafana). UIDs are stable.

| UID | Title | Best for |
|---|---|---|
| `neo4j-overview` | Neo4j — Overview | Fleet entry point. Triage table with click-through to per-instance dashboards. |
| `neo4j-transactions` | Neo4j — Transactions | Live txn snapshot, throughput, CPU/wait/idle decomposition, joined slow-query callout. |
| `neo4j-storage` | Neo4j — Storage | Bytes by component, growth rate + 7d projection, ID usage, kernel identity. |
| `neo4j-jvm` | Neo4j — JVM | Heap + non-heap, GC pause estimate, threads, classes, CPU, FDs, lifecycle. |
| `neo4j-slow-queries` | Neo4j — Slow Queries | `/slow-queries` snapshot table joined across elapsed/cpu/wait/page-faults/allocated. Hotspots by user and query_hash. |
| `neo4j-schema` | Neo4j — Schema & Capacity | Indexes by state, index usage analytics (read rate / coldest), constraints, `apoc.meta.stats` distributions, ID forecast. |

## Notes

- **`*_count` not `*_total`** for APOC-derived counters: APOC reports cumulative-since-last-restart values, not strict monotonic counters. Naming them `*_count` (gauge) instead of `*_total` (counter) avoids `rate()` going negative when Neo4j restarts.
- **Per-tx page hits/faults are NOT a cache hit ratio.** They are summed across currently-active transactions only. A global hit ratio requires Enterprise.
- **Slow-query metrics live at `/slow-queries`**, not `/probe`. They are exposed as an OpenMetrics info family per request and should be scraped at a separate, typically slower, interval. See [`docs/CONFIGURATION.md`](CONFIGURATION.md).
- **`*_timestamp_seconds` metrics are Unix seconds.** Grafana's `dateTimeFromNow` / `dateTimeAsIso` field units interpret the raw value as milliseconds, so any panel rendering one of these as a date multiplies the query by `1000`. Keep the metric in seconds (Prometheus convention); apply the `* 1000` in the dashboard query.
