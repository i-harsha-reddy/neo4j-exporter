package collector

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// APOCKernelCollector calls apoc.monitor.kernel (APOC Extended only).
// Yields: readOnly, kernelVersion, storeId, kernelStartTime, databaseName,
// storeLogVersion, storeCreationDate.
type APOCKernelCollector struct{}

func (APOCKernelCollector) Name() string { return "apoc_kernel" }

func (APOCKernelCollector) Collect(ctx context.Context, p *ProbeContext) error {
	if !p.Caps.HasAPOCMonitorKernel {
		return nil // auto-disabled; orchestrator should already filter, defensive guard
	}
	rows, err := p.Entry.ExecRead(ctx, "", "CALL apoc.monitor.kernel()", nil)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	r := rows[0]

	info := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_kernel_info",
		Help: "Static kernel info from apoc.monitor.kernel; value is always 1.",
	}, []string{"database_name", "kernel_version", "store_id"})
	readOnly := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_kernel_read_only",
		Help: "1 if the kernel is read-only.",
	}, []string{"database_name"})
	startTime := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_kernel_start_timestamp_seconds",
		Help: "Kernel start time as Unix seconds.",
	}, []string{"database_name"})
	creation := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_kernel_store_creation_timestamp_seconds",
		Help: "Store creation time as Unix seconds.",
	}, []string{"database_name"})
	logVersion := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_kernel_store_log_version",
		Help: "Current store log version.",
	}, []string{"database_name"})
	for _, m := range []prometheus.Collector{info, readOnly, startTime, creation, logVersion} {
		MustRegister(p.Registry, m)
	}

	dbName := asString(r["databaseName"])
	if dbName == "" {
		dbName = p.Target.Bolt.Database
	}
	info.WithLabelValues(dbName, asString(r["kernelVersion"]), asString(r["storeId"])).Set(1)
	readOnlyVal := 0.0
	if asFloat64(r["readOnly"]) > 0 {
		readOnlyVal = 1
	}
	readOnly.WithLabelValues(dbName).Set(readOnlyVal)

	startTime.WithLabelValues(dbName).Set(asUnixSeconds(r["kernelStartTime"]))
	creation.WithLabelValues(dbName).Set(asUnixSeconds(r["storeCreationDate"]))
	logVersion.WithLabelValues(dbName).Set(asFloat64(r["storeLogVersion"]))
	return nil
}

// asUnixSeconds tolerates the several shapes APOC / the driver use for kernel
// timestamps across versions: a Neo4j time.Time, a numeric ms-since-epoch, or a
// formatted date string ("2006-01-02 15:04:05") — which is what apoc.monitor.kernel
// emits on Neo4j 5.26.x with current APOC Extended. Unparseable input yields 0.
func asUnixSeconds(v any) float64 {
	switch x := v.(type) {
	case time.Time:
		return float64(x.Unix())
	case nil:
		return 0
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		// APOC formats these with no timezone; the Neo4j JVM defaults to UTC in
		// containers, so parse as UTC. Several layouts are tried for resilience.
		for _, layout := range []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			time.RFC3339,
		} {
			if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
				return float64(t.Unix())
			}
		}
		// Some APOC versions return the value as a numeric string; fall through
		// to the numeric path below.
	}
	f := asFloat64(v)
	// Older APOC returns kernelStartTime as ms since epoch.
	if f > 1e10 {
		return f / 1000
	}
	return f
}
