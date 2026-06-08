---
description: Bring up the docker-compose demo stack (neo4j + apoc + jolokia + exporter + prometheus + grafana + loadgen).
---

Start the end-to-end demo stack in `examples/docker-compose/`.

Steps:

1. Verify Docker daemon is running:
   ```bash
   docker info >/dev/null 2>&1 || echo "Docker daemon not running. Start Docker Desktop first."
   ```
   If not running, stop and tell the user. Don't try to start Docker for them.

2. Bring up the stack:
   ```bash
   cd examples/docker-compose && docker compose up -d --wait
   ```

3. Wait for Neo4j healthcheck to pass (`docker compose --wait` does this; first boot takes ~60s because it pulls APOC Extended from labs).

4. Quickly verify each component:
   ```bash
   curl -fsS http://localhost:7474 >/dev/null && echo "neo4j UI ok"
   curl -fsS http://localhost:9412/health && echo
   curl -fsS http://localhost:9090/-/healthy && echo
   curl -fsS http://localhost:3000/api/health | head -1 && echo
   ```

5. Issue one probe to confirm metrics flow:
   ```bash
   curl -fsS 'http://localhost:9412/probe?target=neo4j:7687&module=default' | grep -E '^neo4j_up' | head -1
   ```

6. Print URLs the user can open:
   - Grafana (anonymous Admin): http://localhost:3000
   - Prometheus: http://localhost:9090
   - Neo4j Browser: http://localhost:7474 (neo4j / dev_password_change_me)
   - Exporter probe: http://localhost:9412/probe?target=neo4j:7687&module=default
   - Slow queries: http://localhost:9412/slow-queries?target=neo4j:7687

If any service is unhealthy, run `docker compose logs <service>` and surface the relevant lines.
