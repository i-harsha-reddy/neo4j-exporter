package probe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/collector"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/neo4jclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler serves /probe?target=...&module=... using a fresh registry per
// request so per-target metrics never leak between probes.
type Handler struct {
	Mgr          *cfg.Manager
	Pool         *neo4jclient.Pool
	Orchestrator *Orchestrator
	Logger       *slog.Logger

	// SelfMetrics is registered on the exporter's own /metrics. It is
	// observed at the end of every probe.
	SelfMetrics *SelfMetrics
}

// SelfMetrics is the Prometheus collector for exporter-side aggregates.
type SelfMetrics struct {
	ProbesTotal    *prometheus.CounterVec
	ProbeDuration  *prometheus.HistogramVec
	ProbeInflight  *prometheus.GaugeVec
	JolokiaErrors  *prometheus.CounterVec
}

func NewSelfMetrics() *SelfMetrics {
	return &SelfMetrics{
		ProbesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "neo4j_exporter_probes_total",
			Help: "Total number of probes by module and outcome.",
		}, []string{"module", "outcome"}),
		ProbeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "neo4j_exporter_probe_duration_seconds",
			Help:    "Histogram of probe wall durations by module.",
			Buckets: prometheus.DefBuckets,
		}, []string{"module"}),
		ProbeInflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "neo4j_exporter_probe_inflight",
			Help: "Number of in-flight probes by module.",
		}, []string{"module"}),
		JolokiaErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "neo4j_jolokia_attribute_errors_total",
			Help: "Per-attribute Jolokia errors observed during probes.",
		}, []string{"mbean", "attribute"}),
	}
}

func (s *SelfMetrics) Register(r prometheus.Registerer) {
	r.MustRegister(s.ProbesTotal, s.ProbeDuration, s.ProbeInflight, s.JolokiaErrors)
}

// ServeHTTP handles /probe?target=...&module=...
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	moduleName := r.URL.Query().Get("module")
	if target == "" {
		http.Error(w, "target query parameter is required", http.StatusBadRequest)
		return
	}

	conf := h.Mgr.Get()
	tgt, ok := conf.FindTarget(target)
	if !ok {
		http.Error(w, fmt.Sprintf("target %q not in config and no default_target defined", target), http.StatusNotFound)
		return
	}
	if moduleName == "" {
		moduleName = tgt.Module
	}
	if moduleName == "" {
		moduleName = conf.Global.DefaultModule
	}
	module, ok := conf.Module(moduleName)
	if !ok {
		http.Error(w, fmt.Sprintf("module %q not defined", moduleName), http.StatusNotFound)
		return
	}

	// Honor Prometheus' scrape timeout header if present.
	timeout := module.Timeout
	if timeout == 0 {
		timeout = conf.Global.ScrapeTimeout
	}
	if h := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); h != "" {
		if v, err := strconv.ParseFloat(h, 64); err == nil {
			pt := time.Duration(v * float64(time.Second))
			// Leave 250ms headroom so Prometheus doesn't time out before us.
			pt -= 250 * time.Millisecond
			if pt > 0 && pt < timeout {
				timeout = pt
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	registry := prometheus.NewRegistry()

	upGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "neo4j_up",
		Help: "1 if the Neo4j Bolt port responded successfully, else 0.",
	})
	durGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "neo4j_scrape_duration_seconds",
		Help: "Total wall duration of the probe.",
	})
	collector.MustRegister(registry, upGauge)
	collector.MustRegister(registry, durGauge)

	if h.SelfMetrics != nil {
		h.SelfMetrics.ProbeInflight.WithLabelValues(moduleName).Inc()
		defer h.SelfMetrics.ProbeInflight.WithLabelValues(moduleName).Dec()
	}

	res, err := h.Orchestrator.Run(ctx, h.Pool, tgt, module, registry)
	if res != nil {
		upGauge.Set(res.Up)
		durGauge.Set(res.Duration().Seconds())
		if h.SelfMetrics != nil {
			h.SelfMetrics.ProbeDuration.WithLabelValues(moduleName).Observe(res.Duration().Seconds())
		}
	}
	outcome := "success"
	if err != nil || (res != nil && res.Up == 0) {
		outcome = "error"
	}
	if h.SelfMetrics != nil {
		h.SelfMetrics.ProbesTotal.WithLabelValues(moduleName, outcome).Inc()
	}

	if h.Logger != nil && (err != nil || (res != nil && len(res.Errors) > 0)) {
		attrs := []any{"target", target, "module", moduleName, "up", res.Up}
		if err != nil {
			attrs = append(attrs, "driver_error", err.Error())
		}
		for k, v := range res.Errors {
			attrs = append(attrs, "err."+k, v.Error())
		}
		h.Logger.Warn("probe completed with errors", attrs...)
	}

	promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      slogToStdLogger(h.Logger),
		ErrorHandling: promhttp.ContinueOnError,
	}).ServeHTTP(w, r)
}

// slogToStdLogger adapts slog.Logger to promhttp's stdlib logger expectation.
func slogToStdLogger(l *slog.Logger) promhttp.Logger {
	if l == nil {
		return nil
	}
	return slogStdLog{l: l}
}

type slogStdLog struct{ l *slog.Logger }

func (s slogStdLog) Println(v ...interface{}) {
	s.l.Warn(fmt.Sprint(v...))
}
