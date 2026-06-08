# neo4j-exporter

Helm chart for [neo4j-exporter](https://github.com/i-harsha-reddy/neo4j-exporter) — a Prometheus exporter for Neo4j Community Edition 5.x / 2025.x. The exporter combines Cypher `SHOW` commands, APOC monitor procedures, and JVM-via-Jolokia into a single binary that scrapes a fleet of Neo4j instances using the [multi-target probe pattern](https://prometheus.io/docs/guides/multi-target-exporter/).

## Install

From OCI registry (once the chart is published):

```bash
helm install neo4j-exporter oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter \
  --version 0.1.0 \
  --namespace monitoring --create-namespace \
  --set auth.password=YOUR_BOLT_PASSWORD \
  --set auth.jolokiaPassword=YOUR_JOLOKIA_PASSWORD
```

From a local checkout:

```bash
helm install neo4j-exporter ./charts/neo4j-exporter \
  --namespace monitoring --create-namespace \
  -f my-values.yaml
```

## Credentials

You must supply Bolt and Jolokia passwords in one of two ways.

**1. Pre-create a Secret** (recommended for production):

```bash
kubectl --namespace monitoring create secret generic neo4j-creds \
  --from-literal=password=YOUR_BOLT_PASSWORD \
  --from-literal=jolokia-password=YOUR_JOLOKIA_PASSWORD
```

```yaml
auth:
  existingSecret: neo4j-creds
  passwordKey: password           # default
  jolokiaPasswordKey: jolokia-password  # default
  username: monitor
```

**2. Inline in values** (fine for demos, not for production):

```yaml
auth:
  username: monitor
  password: hunter2
  jolokiaPassword: jolokia-secret
```

## Targets

The chart's default `config.default_target` lets the exporter accept any `target=host:port` query parameter and reuse the same credentials. This is the right setup for service-discovery: Prometheus discovers Neo4j pods, hands the address as `target`, and the exporter does the rest.

For static target lists, set `config.targets`:

```yaml
config:
  targets:
    - name: prod-1
      address: neo4j-prod-1.databases.svc:7687
      module: default
    - name: prod-2
      address: neo4j-prod-2.databases.svc:7687
      module: default
```

## Wiring Prometheus

This chart deliberately does not bundle a `Probe` or `ServiceMonitor` CRD. Apply your own from the upstream [`examples/kubernetes/servicemonitor.yaml`](https://github.com/i-harsha-reddy/neo4j-exporter/blob/main/examples/kubernetes/servicemonitor.yaml), or use a native Prometheus scrape config (see the `helm install` post-install notes).

## Production knobs

| Setting | Default | Effect |
|---|---|---|
| `replicaCount` | `1` | Stateless; HA is fine. |
| `podDisruptionBudget.enabled` | `null` | Auto-enabled when `replicaCount > 1` or autoscaling is on. Set explicitly to override. |
| `autoscaling.enabled` | `false` | HPA on CPU and/or memory. |
| `networkPolicy.enabled` | `false` | Restricts ingress to the named selectors and egress to declared rules. |
| `affinity` / `topologySpreadConstraints` | `{}` / `[]` | Spread replicas across hosts/zones. |
| `serviceAccount.create` | `true` | The exporter does not call the K8s API; the SA exists only for namespacing. |

## Values reference

See [`values.yaml`](./values.yaml) — every value is annotated with `# --` doc comments and the file is the authoritative reference. Highlights:

- `image.*` — repository, tag (defaults to `.Chart.AppVersion`), pullPolicy, pullSecrets.
- `resources` — sane defaults for a steady-state CE fleet (50m CPU req, 64Mi mem req, 500m / 256Mi limits). Scale up if you target many Neo4j hosts or run with very short scrape intervals.
- `probes.{liveness,readiness}` — paths are pinned to `/health` and `/ready`; durations are tunable.
- `securityContext` / `podSecurityContext` — defaults match the distroless image (UID 65532, read-only rootfs, no caps, RuntimeDefault seccomp).
- `config` — the full exporter config, rendered into a ConfigMap. Schema mirrors [`internal/config/config.go`](https://github.com/i-harsha-reddy/neo4j-exporter/blob/main/internal/config/config.go).
- `extraArgs` / `extraEnv` / `extraVolumes` / `extraVolumeMounts` — escape hatches for things not first-class in values.

## Smoke test

After install:

```bash
helm test neo4j-exporter --namespace monitoring
```

The bundled test pod hits `/ready` and asserts that `neo4j_exporter_build_info` is exposed on `/metrics`. It does NOT require a reachable Neo4j to pass — it verifies the exporter itself is healthy.

## Compatibility

| Chart | Exporter | Helm | Kubernetes |
|---|---|---|---|
| `0.1.x` | `0.1.x` | `>=3.8` (incl. Helm 4) | `>=1.25` |

`kubeVersion` requires 1.25+ because the chart uses `seccompProfile.type: RuntimeDefault` directly on the pod (graduated to GA in 1.25), `policy/v1` PodDisruptionBudget, and `autoscaling/v2` HPA.

## License

Apache-2.0. See [LICENSE](https://github.com/i-harsha-reddy/neo4j-exporter/blob/main/LICENSE) in the source repository.
