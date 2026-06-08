# Configuration reference

The exporter loads a YAML file from `--config.file` (default `/etc/neo4j-exporter/config.yml`). Reload via `SIGHUP` or `POST /-/reload`. Failed reloads keep the previous config and return the parse error in the HTTP body.

The full schema is documented inline in [`config/neo4j-exporter.example.yml`](../config/neo4j-exporter.example.yml). This page covers the rules and conventions that aren't visible in the schema.

## Top-level

```yaml
global:
  default_module: default          # used when /probe is called without &module=
  scrape_timeout: 10s              # absolute cap on a probe's wall time
  collect_concurrency: 8
  driver:
    max_connection_pool_size: 4
    max_connection_lifetime: 1h
    connection_acquisition_timeout: 5s

modules:                           # see "Modules" below
  ...

targets:                           # see "Targets" below
  ...

default_target:                    # fallback for SD-discovered targets not listed
  ...
```

## Modules

A module is a named bundle of (collectors enabled / disabled / auto-detected) plus per-collector timeouts. Targets pick a module via `?module=...` in the probe URL.

```yaml
modules:
  default:
    timeout: 8s                    # the whole probe must finish under this
    collectors:
      server:        true          # always-on per-collector toggle
      bolt:          true
      databases:     true
      transactions:  true
      indexes:       true
      constraints:   true
      apoc_kernel:   auto          # tri-state: true | false | auto
      apoc_store:    auto
      apoc_tx:       auto
      apoc_ids:      auto
      apoc_meta:     auto
      jolokia:       true
      settings:      false         # opt-in (slow on cold caches)
    slow_query:
      enabled: true
      min_seconds: 5               # threshold for /slow-queries
      top: 10
      expose_query_text: false     # PII guard. Hash always emitted.
      timeout: 5s
    per_collector_timeout:
      transactions: 2s
      apoc_store: 3s
      jolokia: 2s
```

### `auto` mode for APOC collectors

`auto` means: *enable if the procedure exists on the target*. The exporter probes capabilities once per Neo4j instance at first contact (and again on reload). If `apoc.monitor.kernel` exists, the `apoc_kernel` collector runs; if not, it sits silently disabled. This is the right default for fleets where APOC tiers vary across instances.

### Suggested module set

The example config ships three modules:

- `default` — everything; ~15s scrape interval is appropriate.
- `readonly` — server + bolt + databases. Fast (≤3s) for high-frequency liveness checks.
- `jvm_only` — server + jolokia. Useful when JVM metrics need a different scrape interval than the rest.

Prometheus selects a module per scrape job:

```yaml
- job_name: neo4j-readonly
  metrics_path: /probe
  params: { module: [readonly] }
  scrape_interval: 5s
  ...
- job_name: neo4j-default
  metrics_path: /probe
  params: { module: [default] }
  scrape_interval: 30s
  ...
```

## Targets

```yaml
targets:
  - name: prod-primary
    address: neo4j-prod-1:7687
    module: default
    bolt:
      scheme: neo4j+s              # see schemes table below
      username: monitor
      password_file: /etc/neo4j-exporter/secrets/prod-primary.password
      database: neo4j              # default DB; affects auth-scoped queries only
      tls:
        ca_file: /etc/neo4j-exporter/tls/ca.pem
        server_name: neo4j-prod-1.internal
    jolokia:
      url: https://neo4j-prod-1:8778/jolokia/
      auth:
        username: jolokia
        password_file: /etc/neo4j-exporter/secrets/jolokia.password
      tls:
        ca_file: /etc/neo4j-exporter/tls/jolokia-ca.pem
```

### Bolt schemes

| Scheme | Encryption | Cert verification |
|---|---|---|
| `bolt`, `neo4j` | none | n/a |
| `bolt+s`, `neo4j+s` | TLS | system trust store (cert chain must be valid) |
| `bolt+ssc`, `neo4j+ssc` | TLS | accepts self-signed (unsafe for public networks) |

Use `neo4j` (routing-aware) for any production deployment; `bolt` exists for direct single-instance connections and is fine for CE.

### Custom CA via `tls.ca_file` for Bolt

For Bolt the exporter currently relies on the URI scheme + system trust store. Custom CA bundles for Bolt require `neo4j.WithCustomTrustStrategy`, which is on the v1.1 roadmap. The `tls` block under `bolt:` is parsed for forward compatibility but only `ca_file` for **Jolokia** is wired in v1.

## Authentication

Exactly one of:
- `bolt.password` — inline (NOT recommended)
- `bolt.password_file` — path on disk; mode must be `0400`, `0440`, `0600`, or `0640` (override with `--config.allow-insecure-secret-files`). Group-read modes are allowed because kubelet's `fsGroup` mechanism OR's group-read into Secret-mounted file modes (`0400` → `0440`) so a nonroot container can read them.
- `${VAR}` env interpolation in `bolt.password`
- `${file:/path/...}` interpolation anywhere in the YAML

Or set `bolt.auth: none` (rejected for non-localhost addresses).

Secrets are re-read on each `SIGHUP` / `POST /-/reload` so rotated credentials take effect without restart.

## Default target (SD fallback)

`default_target` matches probes whose address isn't in `targets`. Useful with `kubernetes_sd_configs` / `consul_sd_configs` / `file_sd_configs`:

```yaml
default_target:
  module: default
  bolt:
    scheme: neo4j
    username: ${NEO4J_DEFAULT_USER}
    password: ${NEO4J_DEFAULT_PASSWORD}
```

## Reload semantics

- `SIGHUP` and `POST /-/reload` are equivalent.
- Reload is **atomic**: parse → validate → swap. A failing reload leaves the previous config active and returns the parse error in the HTTP response body (or in `neo4j-exporter.log` for SIGHUP).
- After reload, the driver pool reconciles: any `(uri, auth-hash)` pair removed or whose auth changed is closed after a 30s grace window so in-flight probes drain cleanly.
- A `neo4j_exporter_config_last_reload_success_timestamp_seconds` metric is exposed on `/metrics`.

## Self-metrics (`/metrics`, separate from `/probe`)

| Metric | Type |
|---|---|
| `neo4j_exporter_build_info{version,revision,branch,goversion}` | gauge=1 |
| `neo4j_exporter_config_last_reload_success_timestamp_seconds` | gauge |
| `neo4j_exporter_probes_total{module,outcome=success\|error}` | counter |
| `neo4j_exporter_probe_duration_seconds_bucket{module}` | histogram |
| `neo4j_exporter_probe_inflight{module}` | gauge |
| `neo4j_exporter_driver_pool_size` | gauge |
| `neo4j_jolokia_attribute_errors_total{mbean,attribute}` | counter |
| `go_*`, `process_*` | from `client_golang` defaults |
