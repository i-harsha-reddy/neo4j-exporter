# Security policy

## Reporting a vulnerability

**Please do not file public issues for security vulnerabilities.**

Email **mailme.sha97@gmail.com** with subject line `SECURITY: neo4j-exporter`. Include:

- a description of the issue,
- the affected version (`neo4j_exporter_build_info`),
- reproduction steps or a proof-of-concept,
- whether you've notified anyone else.

We aim to acknowledge within 3 business days and provide a remediation plan within 14 days for confirmed issues. Critical issues (active exploitation, RCE, secrets exfiltration) are prioritized.

If you'd like to coordinate a public disclosure timeline, mention it in your initial report.

## Supported versions

We support the most recent minor release line. Patch releases for security issues are made against the supported line; older versions receive fixes only at the maintainers' discretion.

| Version | Supported |
|---|---|
| 0.1.x | ✅ |

## Threat model

This is a Prometheus exporter — it talks to Neo4j (Bolt), Jolokia (HTTP→JMX), and Prometheus (HTTP scrape). Credentials are passed at startup via config files; nothing flows back to the exporter from Prometheus.

In scope:

- Authentication and authorization to Bolt and Jolokia (the exporter is a client).
- Credential exposure: the exporter does not log credentials and validates secret-file modes (`0400`/`0440`/`0600`/`0640`); `--config.allow-insecure-secret-files` overrides this only with explicit operator action.
- Cypher injection: the exporter runs only fixed `SHOW`/APOC procedure queries with no user-supplied parameters. The `/probe?target=...` parameter is treated as a Bolt URI, not a Cypher fragment.
- PII in slow-query text: `slow_query.expose_query_text` defaults `false`. The default `query_hash` (sha1[:8]) is not reversible to the original query.
- Cardinality DoS: collectors cap label cardinality (indexes: 500; apoc_meta label/rel_type: 200 each) so a misconfigured Neo4j cannot blow up Prometheus.
- Bolt/Jolokia connection pool: per-target driver pool with bounded size; `/probe` requests use per-collector timeouts to prevent a slow target from holding connections.

Out of scope:

- Hardening of the Neo4j or Jolokia agents themselves — see [`docs/JOLOKIA.md`](docs/JOLOKIA.md) for our recommended Jolokia hardening (TLS + basic auth + restrictive `policy.xml`).
- The Prometheus and Grafana dashboards' own security model.
- Compromise of the GHCR registry (`ghcr.io`) — we sign **both** the container image and the Helm chart artifact with cosign keyless OIDC. Verify with:

  ```bash
  # Helm chart
  cosign verify ghcr.io/i-harsha-reddy/charts/neo4j-exporter:<tag> \
    --certificate-identity-regexp 'github.com/i-harsha-reddy/neo4j-exporter' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com

  # Container image
  cosign verify ghcr.io/i-harsha-reddy/neo4j-exporter:<tag> \
    --certificate-identity-regexp 'github.com/i-harsha-reddy/neo4j-exporter' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com
  ```

## Hardening checklist for operators

- Use a least-privilege Neo4j user (read-only on the system database is sufficient — no user database access required).
- Use [`auth.existingSecret`](charts/neo4j-exporter/values.yaml) instead of inline credentials in chart values.
- Apply [`examples/kubernetes/values/values-network-policy.yaml`](examples/kubernetes/values/values-network-policy.yaml) (or equivalent) so only Prometheus can reach the exporter, and the exporter's egress is limited to DNS + Bolt + Jolokia.
- Run with the chart's default `securityContext` (read-only rootfs, nonroot, dropped capabilities, RuntimeDefault seccomp). Don't relax.
- Pin the chart version (`--version 0.1.0`) and verify the cosign signature in CI before deployment.
- Set Jolokia in production per `docs/JOLOKIA.md`. The `examples/docker-compose` setup is plaintext + no auth and is **demo-only**.
- Treat `/slow-queries` as a privileged endpoint. Default exposes only `query_hash`; flipping `expose_query_text: true` requires accepting that Cypher literals may include PII.

## Acknowledgments

Reported issues that lead to fixes are credited in `CHANGELOG.md` (with permission of the reporter).
