# Enabling JVM metrics (Jolokia)

Neo4j Community Edition does NOT expose its own JMX MBean tree (`neo4j.metrics:*` is Enterprise). However the underlying JVM still exposes the standard `java.lang:*` MBeans, which give us heap, GC, threads, file descriptors, and CPU. We read them via [Jolokia](https://jolokia.org/), an HTTP→JMX bridge attached as a JVM agent.

This page covers production-grade Jolokia setup. **Demo-grade** setup (no auth, no TLS) is in [`examples/docker-compose/`](../examples/docker-compose/) and is NOT safe to expose outside a private network.

## Install the agent

Drop the Jolokia agent jar into Neo4j's plugins directory:

```bash
JOLOKIA_VERSION=2.1.1
sudo curl -fsSL -o /var/lib/neo4j/plugins/jolokia.jar \
  "https://repo1.maven.org/maven2/org/jolokia/jolokia-agent-jvm/${JOLOKIA_VERSION}/jolokia-agent-jvm-${JOLOKIA_VERSION}-javaagent.jar"
sudo chown neo4j:neo4j /var/lib/neo4j/plugins/jolokia.jar
sudo chmod 0644 /var/lib/neo4j/plugins/jolokia.jar
```

## Configure Neo4j to start with the agent

Edit `neo4j.conf`:

```properties
# Tell the JVM to load Jolokia at startup, pointing at a properties file we control.
server.jvm.additional=-javaagent:/var/lib/neo4j/plugins/jolokia.jar=config=/var/lib/neo4j/conf/jolokia.properties
```

Then create `/var/lib/neo4j/conf/jolokia.properties`:

```properties
# Bind only to the interface(s) the exporter uses. NEVER 0.0.0.0 in production
# unless your network policy already isolates the host.
host=127.0.0.1
port=8778

# HTTPS + basic auth + restrictive policy file. All three are required.
protocol=https
keystore=/var/lib/neo4j/conf/jolokia.jks
keystorePassword=${file:/var/lib/neo4j/conf/jolokia.kspw}

authMode=basic
user=jolokia
password=${file:/var/lib/neo4j/conf/jolokia.pw}

policyLocation=file:///var/lib/neo4j/conf/jolokia-policy.xml
discoveryEnabled=false
includeStackTrace=false
agentDescription=neo4j-${HOSTNAME}
```

## Lock down with `policy.xml` — non-negotiable

Without a policy file, anyone with the auth credential can use Jolokia to **execute arbitrary MBean operations** (e.g., `setLogLevel`, custom MBean exec). Restrict to read-only on the JVM tree only:

```xml
<?xml version="1.0" encoding="utf-8"?>
<restrict>
  <commands>
    <command>read</command>
    <command>list</command>
  </commands>
  <allow>
    <mbean>
      <name>java.lang:*</name>
    </mbean>
  </allow>
</restrict>
```

Save this as `/var/lib/neo4j/conf/jolokia-policy.xml`. Anything not in the `<allow>` list is denied.

## Verify

After restarting Neo4j:

```bash
curl -k -u jolokia:THE_PASSWORD https://127.0.0.1:8778/jolokia/version
# Expect: {"request":{"type":"version"},"value":{"agent":"2.1.1",...},"status":200,...}

curl -k -u jolokia:THE_PASSWORD -H 'Content-Type: application/json' \
  -d '[{"type":"read","mbean":"java.lang:type=Memory","attribute":"HeapMemoryUsage"}]' \
  https://127.0.0.1:8778/jolokia/
```

## Configure the exporter

In `config.yml` per target:

```yaml
jolokia:
  url: https://neo4j-prod-1:8778/jolokia/
  auth:
    username: jolokia
    password_file: /etc/neo4j-exporter/secrets/jolokia.password
  tls:
    ca_file: /etc/neo4j-exporter/tls/jolokia-ca.pem
    server_name: neo4j-prod-1.internal
```

`tls.ca_file` should be the CA that signed the keystore presented by Jolokia. For a self-signed setup pin the CA explicitly — do not use `insecure_skip_verify: true` in production.

## Alternative: prometheus/jmx_exporter as a Java agent

If you'd rather not run Jolokia, you can attach [prometheus/jmx_exporter](https://github.com/prometheus/jmx_exporter) as a separate Java agent. Trade-offs:

| | Jolokia (this exporter's primary path) | jmx_exporter (alternative) |
|---|---|---|
| Multi-target compatible | ✅ One exporter scrapes N JVMs | ❌ Each JVM exposes its own `/metrics` endpoint |
| Bulk request | ✅ One POST per probe, ~7 MBean families | n/a — exposes pre-formatted metrics |
| Auth/TLS shared with Bolt config | ✅ Yes | ❌ Separate Prometheus scrape job |
| Read-only by default | ❌ requires `policy.xml` | ✅ exposes only `/metrics` |
| Operational overhead | One javaagent + properties file | One javaagent + YAML config + extra Prometheus job |

If you choose jmx_exporter, set `jolokia.enabled: false` for those targets in this exporter's config and add a separate `scrape_configs` job. The metric names differ (`jvm_*` vs `neo4j_jvm_*`); add Prometheus recording rules to align them or switch the dashboard queries.
