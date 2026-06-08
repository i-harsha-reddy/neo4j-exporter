---
description: Issue an ad-hoc /probe against a running exporter. Usage — /probe <target> [module]
---

The user wants to issue an ad-hoc probe against a running `neo4j-exporter`.

Arguments: `$ARGUMENTS` — typically `<target>` or `<target> <module>`. If empty, default to `localhost:7687 default`.

Steps:

1. Determine `EXPORTER_URL` (default `http://localhost:9412`). If `$EXPORTER_URL` env is set, use that.

2. Curl the probe and pipe through the metric grouper:
   ```bash
   curl -fsS "$EXPORTER_URL/probe?target=<TARGET>&module=<MODULE>" | grep -v '^#' | sort
   ```

3. Show the user:
   - Was `neo4j_up == 1`? (extract the line)
   - Which collectors succeeded vs failed (`neo4j_collector_success` lines)
   - For each failed collector, hint at likely cause (network → bolt port reachability; auth → credentials; capability flag → APOC plugin; jolokia → agent not loaded)
   - Counts of major metric families (transactions, indexes, store_*, jvm_*)

4. If the exporter is not running on `localhost:9412`, check whether the user wants you to start it via `make run` (with the minimal config). Don't auto-start without confirmation.

Keep the report tight — under 200 words unless something is genuinely broken.
