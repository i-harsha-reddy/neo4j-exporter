---
description: Stop the docker-compose demo stack and remove its volumes.
---

Tear down the demo stack in `examples/docker-compose/`.

```bash
cd examples/docker-compose && docker compose down -v --remove-orphans
```

The `-v` flag removes the `jolokia-data` volume (which holds the downloaded Jolokia jar) so the next `compose-up` re-downloads it. If the user wants to KEEP the volume across restarts (faster restart), drop the `-v` and tell them. Default is to clean fully.

After teardown, confirm with `docker ps -a --filter name=neo4j-exporter-demo-` showing no containers, and `docker volume ls --filter name=docker-compose_jolokia-data` showing nothing.
