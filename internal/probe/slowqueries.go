package probe

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/neo4jclient"
)

// SlowQueriesHandler serves /slow-queries?target=...&top=10&min_seconds=5
// as either OpenMetrics info family (default) or plain text.
//
// Per-query labels would explode cardinality if put on /probe time-series
// metrics, so this lives on its own endpoint and is best scraped at a
// different (typically slower) interval.
type SlowQueriesHandler struct {
	Mgr    *cfg.Manager
	Pool   *neo4jclient.Pool
	Logger *slog.Logger
}

func (h *SlowQueriesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("target")
	if target == "" {
		http.Error(w, "target query parameter is required", http.StatusBadRequest)
		return
	}
	conf := h.Mgr.Get()
	tgt, ok := conf.FindTarget(target)
	if !ok {
		http.Error(w, "target not found in config", http.StatusNotFound)
		return
	}
	module, _ := conf.Module(tgt.Module)
	if !module.SlowQuery.Enabled {
		http.Error(w, "slow_query is disabled for this target's module", http.StatusForbidden)
		return
	}

	top := atoiOr(q.Get("top"), module.SlowQuery.Top)
	if top <= 0 || top > 1000 {
		top = 10
	}
	minSeconds := atofOr(q.Get("min_seconds"), module.SlowQuery.MinSeconds)
	if minSeconds < 0 {
		minSeconds = 0
	}

	timeout := module.SlowQuery.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	entry, err := h.Pool.Get(tgt)
	if err != nil {
		http.Error(w, "driver: "+err.Error(), http.StatusBadGateway)
		return
	}

	// elapsedTime is a Duration; we compare in Cypher using duration arithmetic.
	// Cypher refuses to compare Durations with <,<=,>,>= (returns NULL because
	// month-bearing durations have indeterminate length). Compare on the total
	// nanoseconds accessor instead, which is always defined.
	rows, err := entry.ExecRead(ctx, "", `
SHOW TRANSACTIONS YIELD database, transactionId, currentQueryId, currentQuery, status,
  elapsedTime, cpuTime, waitTime, idleTime, allocatedDirectBytes, estimatedUsedHeapMemory,
  pageHits, pageFaults, currentQueryAllocatedBytes, username, clientAddress
WHERE toFloat(elapsedTime.nanoseconds) / 1000000000.0 >= $min
RETURN database, transactionId, currentQueryId, currentQuery, status,
  elapsedTime, cpuTime, waitTime, idleTime, allocatedDirectBytes, estimatedUsedHeapMemory,
  pageHits, pageFaults, currentQueryAllocatedBytes, username, clientAddress
ORDER BY elapsedTime DESC
LIMIT $top
`, map[string]any{"min": minSeconds, "top": top})
	if err != nil {
		http.Error(w, "query: "+err.Error(), http.StatusBadGateway)
		return
	}

	format := q.Get("format")
	if format == "" {
		format = "openmetrics_info"
	}
	switch format {
	case "text":
		writeSlowText(w, rows)
	case "openmetrics_info":
		w.Header().Set("Content-Type", "application/openmetrics-text; version=1.0.0; charset=utf-8")
		writeSlowOpenMetrics(w, rows, module.SlowQuery.ExposeQueryText)
	default:
		http.Error(w, "unsupported format (use text|openmetrics_info)", http.StatusBadRequest)
	}
}

func writeSlowText(w http.ResponseWriter, rows []neo4jclient.Row) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%-30s %-15s %-12s %-10s %s\n", "transaction_id", "user", "elapsed_s", "status", "query (truncated)")
	for _, r := range rows {
		txid := strOf(r["transactionId"])
		user := strOf(r["username"])
		elapsed := durSeconds(r["elapsedTime"])
		status := strOf(r["status"])
		query := truncateForLog(strOf(r["currentQuery"]), 80)
		_, _ = fmt.Fprintf(w, "%-30s %-15s %-12.2f %-10s %s\n", txid, user, elapsed, status, query)
	}
}

func writeSlowOpenMetrics(w http.ResponseWriter, rows []neo4jclient.Row, exposeText bool) {
	_, _ = fmt.Fprintln(w, "# HELP neo4j_slow_query_info Long-running query snapshot. Refreshed each scrape.")
	_, _ = fmt.Fprintln(w, "# TYPE neo4j_slow_query_info info")
	for _, r := range rows {
		hash := queryHash(strOf(r["currentQuery"]))
		labels := []labelKV{
			{"database", strOf(r["database"])},
			{"transaction_id", strOf(r["transactionId"])},
			{"username", strOf(r["username"])},
			{"client_address", strOf(r["clientAddress"])},
			{"status", strOf(r["status"])},
			{"query_hash", hash},
		}
		if exposeText {
			labels = append(labels, labelKV{"query_preview", openMetricsEscape(truncateForLog(strOf(r["currentQuery"]), 200))})
		}
		_, _ = fmt.Fprintf(w, "neo4j_slow_query_info{%s} 1\n", joinLabels(labels))
	}

	_, _ = fmt.Fprintln(w, "# HELP neo4j_slow_query_elapsed_seconds Wall-clock elapsed seconds.")
	_, _ = fmt.Fprintln(w, "# TYPE neo4j_slow_query_elapsed_seconds gauge")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "neo4j_slow_query_elapsed_seconds{transaction_id=%q} %g\n",
			strOf(r["transactionId"]), durSeconds(r["elapsedTime"]))
	}
	_, _ = fmt.Fprintln(w, "# HELP neo4j_slow_query_cpu_seconds CPU seconds consumed.")
	_, _ = fmt.Fprintln(w, "# TYPE neo4j_slow_query_cpu_seconds gauge")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "neo4j_slow_query_cpu_seconds{transaction_id=%q} %g\n",
			strOf(r["transactionId"]), durSeconds(r["cpuTime"]))
	}
	_, _ = fmt.Fprintln(w, "# HELP neo4j_slow_query_wait_seconds Time spent waiting for locks.")
	_, _ = fmt.Fprintln(w, "# TYPE neo4j_slow_query_wait_seconds gauge")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "neo4j_slow_query_wait_seconds{transaction_id=%q} %g\n",
			strOf(r["transactionId"]), durSeconds(r["waitTime"]))
	}
	_, _ = fmt.Fprintln(w, "# HELP neo4j_slow_query_page_faults Page faults caused by this transaction.")
	_, _ = fmt.Fprintln(w, "# TYPE neo4j_slow_query_page_faults gauge")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "neo4j_slow_query_page_faults{transaction_id=%q} %v\n",
			strOf(r["transactionId"]), numOf(r["pageFaults"]))
	}
	_, _ = fmt.Fprintln(w, "# HELP neo4j_slow_query_allocated_bytes Heap bytes allocated by current query.")
	_, _ = fmt.Fprintln(w, "# TYPE neo4j_slow_query_allocated_bytes gauge")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "neo4j_slow_query_allocated_bytes{transaction_id=%q} %v\n",
			strOf(r["transactionId"]), numOf(r["currentQueryAllocatedBytes"]))
	}
	_, _ = fmt.Fprintln(w, "# EOF")
}

type labelKV struct{ k, v string }

func joinLabels(ls []labelKV) string {
	var sb strings.Builder
	first := true
	for _, l := range ls {
		if l.v == "" {
			continue
		}
		if !first {
			sb.WriteByte(',')
		}
		first = false
		sb.WriteString(l.k)
		sb.WriteByte('=')
		sb.WriteByte('"')
		sb.WriteString(openMetricsEscape(l.v))
		sb.WriteByte('"')
	}
	return sb.String()
}

func openMetricsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func queryHash(q string) string {
	if q == "" {
		return ""
	}
	sum := sha1.Sum([]byte(q))
	return hex.EncodeToString(sum[:])[:8]
}

func truncateForLog(s string, n int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func strOf(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func numOf(v any) string {
	if v == nil {
		return "0"
	}
	switch x := v.(type) {
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

func durSeconds(v any) float64 {
	if v == nil {
		return 0
	}
	if d, ok := v.(time.Duration); ok {
		return d.Seconds()
	}
	// Driver returns Cypher duration as a custom type; fall back to numeric coerce.
	switch x := v.(type) {
	case int64:
		return float64(x) / 1000.0
	case float64:
		if x > 1e9 {
			return x / 1e9
		}
		return x / 1000.0
	}
	return 0
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func atofOr(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
