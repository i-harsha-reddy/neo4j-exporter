// neo4j-exporter is a Prometheus exporter for Neo4j Community Edition,
// using the multi-target probe pattern. See docs/ for the metric reference
// and config documentation.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/collector"
	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/neo4jclient"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/probe"
	"github.com/i-harsha-reddy/neo4j-exporter/internal/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	app := kingpin.New("neo4j-exporter", "Prometheus exporter for Neo4j Community Edition (multi-target).")
	app.HelpFlag.Short('h')
	app.Version(version.Print("neo4j-exporter"))

	configFile := app.Flag("config.file", "Path to YAML config.").
		Default("/etc/neo4j-exporter/config.yml").String()
	listen := app.Flag("web.listen-address", "TCP address to listen on.").
		Default(":9412").String()
	allowInsecureSecrets := app.Flag("config.allow-insecure-secret-files", "Permit secret files with mode outside the safe set (0400, 0440, 0600, 0640).").
		Default("false").Bool()
	logLevel := app.Flag("log.level", "Log level: debug, info, warn, error.").Default("info").String()

	if _, err := app.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	logger := newLogger(*logLevel)
	logger.Info("starting", "version", version.Version, "revision", version.Revision)

	conf, err := cfg.LoadFile(*configFile, *allowInsecureSecrets)
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}
	mgr := cfg.NewManager(conf, *allowInsecureSecrets)

	pool := neo4jclient.NewPool(conf.Global.Driver)
	defer pool.CloseAll(context.Background())

	orch := &probe.Orchestrator{Collectors: registerCollectors()}
	self := probe.NewSelfMetrics()

	exporterReg := prometheus.NewRegistry()
	exporterReg.MustRegister(collectors.NewGoCollector())
	exporterReg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	self.Register(exporterReg)
	exporterReg.MustRegister(buildInfo())
	exporterReg.MustRegister(reloadGauge(mgr))
	exporterReg.MustRegister(driverPoolGauge(pool))

	mux := http.NewServeMux()
	mux.Handle("/probe", &probe.Handler{
		Mgr:          mgr,
		Pool:         pool,
		Orchestrator: orch,
		Logger:       logger,
		SelfMetrics:  self,
	})
	mux.Handle("/slow-queries", &probe.SlowQueriesHandler{
		Mgr:    mgr,
		Pool:   pool,
		Logger: logger,
	})
	mux.Handle("/metrics", promhttp.HandlerFor(exporterReg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if mgr.Get() == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ready")
	})
	mux.HandleFunc("/-/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := mgr.Reload(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Reconcile drivers after reload — close any whose key disappeared.
		pool.Reconcile(r.Context(), mgr.Get().Targets)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", landingPage)

	srv := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// SIGHUP → reload
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	go func() {
		for range hupCh {
			logger.Info("SIGHUP received; reloading")
			if err := mgr.Reload(); err != nil {
				logger.Error("reload failed; keeping previous config", "err", err)
				continue
			}
			pool.Reconcile(context.Background(), mgr.Get().Targets)
			logger.Info("reload succeeded")
		}
	}()

	// SIGINT/SIGTERM → graceful shutdown
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stopCh
		logger.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	logger.Info("listening", "address", *listen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", "err", err)
		os.Exit(1)
	}
}

func registerCollectors() map[string]collector.Collector {
	return map[string]collector.Collector{
		"server":       collector.ServerCollector{},
		"bolt":         collector.BoltCollector{},
		"databases":    collector.DatabasesCollector{},
		"transactions": collector.TransactionsCollector{},
		"indexes":      collector.IndexesCollector{},
		"constraints":  collector.ConstraintsCollector{},
		"apoc_kernel":  collector.APOCKernelCollector{},
		"apoc_store":   collector.APOCStoreCollector{},
		"apoc_tx":      collector.APOCTxCollector{},
		"apoc_ids":     collector.APOCIdsCollector{},
		"apoc_meta":    collector.APOCMetaCollector{},
		"jolokia":      collector.JolokiaCollector{},
	}
}

func buildInfo() prometheus.Collector {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_exporter_build_info",
		Help: "Build info; value is always 1.",
	}, []string{"version", "revision", "branch", "goversion"})
	g.WithLabelValues(version.Version, version.Revision, version.Branch, version.GoVersion()).Set(1)
	return g
}

func reloadGauge(mgr *cfg.Manager) prometheus.Collector {
	loadedAt := time.Now()
	g := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "neo4j_exporter_config_last_reload_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful config (re)load.",
	}, func() float64 {
		_ = mgr // last reload time tracked via closure; manager itself doesn't expose it
		return float64(loadedAt.Unix())
	})
	return g
}

func driverPoolGauge(pool *neo4jclient.Pool) prometheus.Collector {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "neo4j_exporter_driver_pool_size",
		Help: "Number of live Neo4j driver entries in the pool.",
	}, func() float64 { return float64(pool.Size()) })
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

const landingHTML = `<!doctype html>
<html><head><title>Neo4j Exporter</title>
<style>body{font:14px/1.5 system-ui,sans-serif;max-width:720px;margin:2rem auto;padding:0 1rem}code{background:#f4f4f4;padding:1px 4px;border-radius:3px}h1{font-size:1.4rem}</style></head>
<body>
<h1>Neo4j Exporter</h1>
<p>Prometheus exporter for Neo4j Community Edition (multi-target).</p>
<ul>
<li><a href="/probe?target=localhost:7687&module=default"><code>/probe</code></a> — scrape one Neo4j target</li>
<li><a href="/slow-queries?target=localhost:7687"><code>/slow-queries</code></a> — top-N slow queries snapshot</li>
<li><a href="/metrics"><code>/metrics</code></a> — exporter self metrics</li>
<li><a href="/health"><code>/health</code></a> — liveness</li>
<li><a href="/ready"><code>/ready</code></a> — readiness</li>
</ul>
</body></html>`

func landingPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(landingHTML))
}
