# Kubernetes examples

The Helm chart at [`../../charts/neo4j-exporter/`](../../charts/neo4j-exporter/) is the supported install path. This folder holds:

- **[`values/`](./values/)** — copy-paste starting points for common deployment scenarios.
- **[`servicemonitor.yaml`](./servicemonitor.yaml)** — Prometheus Operator `Probe` + `ServiceMonitor`. The chart deliberately doesn't bundle these CRDs (it would force a hard dependency on prometheus-operator being installed); apply them separately if you use kube-prometheus-stack.

## Quickstart

```bash
helm install neo4j-exporter oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter \
  --version 0.1.0 \
  --namespace monitoring --create-namespace \
  -f examples/kubernetes/values/values-basic.yaml
```

## Scenario values

| File | When to use |
|---|---|
| [`values/values-basic.yaml`](./values/values-basic.yaml) | Smoke test or demo. Single replica, inline credentials. |
| [`values/values-existing-secret.yaml`](./values/values-existing-secret.yaml) | Production. Reads credentials from a pre-created Secret managed by your secrets pipeline. |
| [`values/values-ha.yaml`](./values/values-ha.yaml) | Multi-replica with podAntiAffinity, zone spreading, HPA. PDB auto-enables. Combine with a credentials values file. |
| [`values/values-network-policy.yaml`](./values/values-network-policy.yaml) | Locked-down ingress (Prometheus only) and egress (DNS + Bolt + Jolokia). Combine with a credentials values file. |

Combine multiple values files by passing each with `-f`. Helm merges right-to-left:

```bash
helm install neo4j-exporter oci://ghcr.io/i-harsha-reddy/charts/neo4j-exporter \
  --version 0.1.0 \
  --namespace monitoring \
  -f examples/kubernetes/values/values-existing-secret.yaml \
  -f examples/kubernetes/values/values-ha.yaml \
  -f examples/kubernetes/values/values-network-policy.yaml
```

## Wiring Prometheus Operator

After installing the chart, apply [`servicemonitor.yaml`](./servicemonitor.yaml). It contains:

- A **`Probe`** that points Prometheus at the exporter's `/probe` endpoint and lists Neo4j fleet targets (one entry per Neo4j instance).
- A **`ServiceMonitor`** for the exporter's own `/metrics` (build info, probe count, driver pool size).

Edit `prober.url` to match your release name and namespace. With `helm install neo4j-exporter ... -n monitoring` the Service URL is `neo4j-exporter.monitoring.svc:9412`. With a different release name (e.g. `helm install ne ...`) it becomes `ne-neo4j-exporter.monitoring.svc:9412`.

## Helm-free / kustomize users

The chart can produce a static manifest bundle:

```bash
helm template neo4j-exporter charts/neo4j-exporter \
  -f examples/kubernetes/values/values-basic.yaml \
  --namespace monitoring \
  > k8s.yaml
kubectl apply -f k8s.yaml
```

That gives you the same output as a `helm install` without runtime dependency on Helm — apply with `kubectl`, vendor it into kustomize, or commit it to a GitOps repo.

## Full values reference

[`charts/neo4j-exporter/values.yaml`](../../charts/neo4j-exporter/values.yaml) is the authoritative reference; every value is documented inline. The chart's [README](../../charts/neo4j-exporter/README.md) has installation patterns, secret strategies, and a compatibility matrix.
