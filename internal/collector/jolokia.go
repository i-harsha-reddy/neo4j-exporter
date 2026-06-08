package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/i-harsha-reddy/neo4j-exporter/internal/jolokiaclient"
	"github.com/prometheus/client_golang/prometheus"
)

// JolokiaCollector emits JVM-level metrics by issuing one bulk read to
// the configured Jolokia agent. Standard java.lang:* MBeans are exposed
// by every JVM regardless of Neo4j edition, so this works for CE.
type JolokiaCollector struct{}

func (JolokiaCollector) Name() string { return "jolokia" }

// jolokiaReadIndex matches the order of the bulk request below.
const (
	idxHeap = iota
	idxNonHeap
	idxThreading
	idxClassLoading
	idxOS
	idxRuntime
	idxGC
)

var jolokiaBulk = []jolokiaclient.ReadRequest{
	jolokiaclient.ReadAll("java.lang:type=Memory"), // returns HeapMemoryUsage + NonHeapMemoryUsage
	{Type: "read", MBean: "java.lang:type=Memory", Attribute: "NonHeapMemoryUsage"},
	jolokiaclient.ReadMulti("java.lang:type=Threading", "ThreadCount", "DaemonThreadCount", "PeakThreadCount", "TotalStartedThreadCount"),
	jolokiaclient.ReadMulti("java.lang:type=ClassLoading", "LoadedClassCount", "TotalLoadedClassCount", "UnloadedClassCount"),
	jolokiaclient.ReadMulti("java.lang:type=OperatingSystem",
		"ProcessCpuLoad", "SystemCpuLoad", "OpenFileDescriptorCount", "MaxFileDescriptorCount", "SystemLoadAverage"),
	jolokiaclient.ReadMulti("java.lang:type=Runtime", "Uptime", "StartTime"),
	jolokiaclient.ReadMulti("java.lang:type=GarbageCollector,name=*", "CollectionCount", "CollectionTime"),
}

func (JolokiaCollector) Collect(ctx context.Context, p *ProbeContext) error {
	if p.Jolokia == nil {
		return fmt.Errorf("jolokia client not configured")
	}
	resp, err := p.Jolokia.Bulk(ctx, jolokiaBulk)
	if err != nil {
		return err
	}
	if len(resp) < len(jolokiaBulk) {
		return fmt.Errorf("jolokia bulk: expected %d responses, got %d", len(jolokiaBulk), len(resp))
	}

	// Heap (CompositeData with init/used/committed/max nested under HeapMemoryUsage)
	if resp[idxHeap].OK() {
		emitMemory(p.Registry, resp[idxHeap].Value, "HeapMemoryUsage", "neo4j_jvm_heap")
	}
	if resp[idxNonHeap].OK() {
		emitMemoryFlat(p.Registry, resp[idxNonHeap].Value, "neo4j_jvm_nonheap")
	}

	// Threading
	if resp[idxThreading].OK() {
		m, _ := resp[idxThreading].Value.(map[string]any)
		setGauge(p.Registry, "neo4j_jvm_threads_current", "Current thread count.", asFloat64(m["ThreadCount"]))
		setGauge(p.Registry, "neo4j_jvm_threads_daemon", "Current daemon thread count.", asFloat64(m["DaemonThreadCount"]))
		setGauge(p.Registry, "neo4j_jvm_threads_peak", "Peak thread count since JVM start.", asFloat64(m["PeakThreadCount"]))
		setGauge(p.Registry, "neo4j_jvm_threads_started_total", "Total threads started since JVM start.", asFloat64(m["TotalStartedThreadCount"]))
	}

	// ClassLoading
	if resp[idxClassLoading].OK() {
		m, _ := resp[idxClassLoading].Value.(map[string]any)
		setGauge(p.Registry, "neo4j_jvm_classes_loaded", "Currently loaded class count.", asFloat64(m["LoadedClassCount"]))
		setGauge(p.Registry, "neo4j_jvm_classes_loaded_total", "Total classes loaded since JVM start.", asFloat64(m["TotalLoadedClassCount"]))
		setGauge(p.Registry, "neo4j_jvm_classes_unloaded_total", "Total classes unloaded since JVM start.", asFloat64(m["UnloadedClassCount"]))
	}

	// OperatingSystem (CPU, FDs, load)
	if resp[idxOS].OK() {
		m, _ := resp[idxOS].Value.(map[string]any)
		setGauge(p.Registry, "neo4j_jvm_cpu_process_ratio", "Process CPU load (0-1).", asFloat64(m["ProcessCpuLoad"]))
		setGauge(p.Registry, "neo4j_jvm_cpu_system_ratio", "System CPU load (0-1).", asFloat64(m["SystemCpuLoad"]))
		setGauge(p.Registry, "neo4j_jvm_open_fds", "Open file descriptors.", asFloat64(m["OpenFileDescriptorCount"]))
		setGauge(p.Registry, "neo4j_jvm_max_fds", "Maximum file descriptors permitted.", asFloat64(m["MaxFileDescriptorCount"]))
		if la := asFloat64(m["SystemLoadAverage"]); la >= 0 {
			setGauge(p.Registry, "neo4j_jvm_load_average_1m", "1-minute system load average.", la)
		}
	}

	// Runtime (uptime + start time, both ms)
	if resp[idxRuntime].OK() {
		m, _ := resp[idxRuntime].Value.(map[string]any)
		setGauge(p.Registry, "neo4j_jvm_uptime_seconds", "JVM uptime in seconds.", asFloat64(m["Uptime"])/1000)
		setGauge(p.Registry, "neo4j_jvm_start_timestamp_seconds", "JVM start time as Unix seconds.", asFloat64(m["StartTime"])/1000)
	}

	// GarbageCollector wildcard — value is map keyed by full MBean name.
	if resp[idxGC].OK() {
		emitGC(p.Registry, resp[idxGC].Value)
	}
	return nil
}

func emitMemory(r *prometheus.Registry, v any, key, prefix string) {
	outer, _ := v.(map[string]any)
	mem, _ := outer[key].(map[string]any)
	if mem == nil {
		// Some Jolokia builds return composite data already flattened.
		mem = outer
	}
	emitMemoryFlat(r, mem, prefix)
}

func emitMemoryFlat(r *prometheus.Registry, v any, prefix string) {
	mem, _ := v.(map[string]any)
	if mem == nil {
		return
	}
	setGauge(r, prefix+"_used_bytes", "JVM "+prefix+" memory used.", asFloat64(mem["used"]))
	setGauge(r, prefix+"_committed_bytes", "JVM "+prefix+" memory committed.", asFloat64(mem["committed"]))
	if max := asFloat64(mem["max"]); max >= 0 {
		setGauge(r, prefix+"_max_bytes", "JVM "+prefix+" memory max.", max)
	}
}

func emitGC(r *prometheus.Registry, v any) {
	mp, _ := v.(map[string]any)
	if mp == nil {
		return
	}
	count := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_jvm_gc_count_total",
		Help: "Total GC collection count by GC name.",
	}, []string{"gc"})
	dur := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_jvm_gc_seconds_total",
		Help: "Total GC time in seconds by GC name.",
	}, []string{"gc"})
	MustRegister(r, count)
	MustRegister(r, dur)

	for mbeanName, attrs := range mp {
		gcName := parseGCName(mbeanName)
		ax, _ := attrs.(map[string]any)
		count.WithLabelValues(gcName).Set(asFloat64(ax["CollectionCount"]))
		dur.WithLabelValues(gcName).Set(asFloat64(ax["CollectionTime"]) / 1000.0)
	}
}

// parseGCName extracts the `name=` clause from a JMX MBean ObjectName and
// normalizes it to snake_case for the `gc` label.
//
//	java.lang:type=GarbageCollector,name=G1 Young Generation → g1_young_generation
func parseGCName(mbean string) string {
	idx := strings.Index(mbean, "name=")
	if idx < 0 {
		return mbean
	}
	name := mbean[idx+len("name="):]
	if comma := strings.IndexByte(name, ','); comma >= 0 {
		name = name[:comma]
	}
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

func setGauge(r *prometheus.Registry, name, help string, val float64) {
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
	MustRegister(r, g)
	g.Set(val)
}
