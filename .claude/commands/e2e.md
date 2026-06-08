---
description: Run the end-to-end smoke test (compose-up → assert metrics → compose-down).
---

Run `scripts/e2e.sh` — the full end-to-end test that the CI workflow uses.

```bash
bash scripts/e2e.sh
```

What it does:
1. `docker compose up -d --wait` in `examples/docker-compose/`
2. Asserts the exporter responds at `:9412/probe`
3. Asserts the curated set of required metrics is present
4. Asserts `neo4j_up == 1`
5. Asserts `/slow-queries` returns 200
6. Asserts Prometheus has healthy targets
7. `docker compose down -v` (via trap on EXIT)

Pre-flight checks before running:
- Docker daemon running? (`docker info >/dev/null 2>&1`)
- Port 9412, 9090, 3000, 7474, 7687 free? (else compose binds will fail)

If the script fails, surface the precise step that failed and the relevant compose logs:
```bash
( cd examples/docker-compose && docker compose logs --no-color --tail 50 )
```

Total runtime: ~90 seconds on a warm Docker cache, ~3 minutes on first run (image + APOC Extended download).
