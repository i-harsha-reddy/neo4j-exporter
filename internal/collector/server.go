package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// ServerCollector emits version-info metrics that work across CE/EE
// regardless of APOC tier.
type ServerCollector struct{}

func (ServerCollector) Name() string { return "server" }

func (ServerCollector) Collect(ctx context.Context, p *ProbeContext) error {
	caps := p.Caps

	versionInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "neo4j_neo4j_version_info",
		Help: "Neo4j build info; value is always 1.",
	}, []string{"version", "edition"})

	apocAvail := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "neo4j_apoc_extended_available",
		Help: "1 if APOC Extended monitor procedures are detected on the target.",
	})

	MustRegister(p.Registry, versionInfo)
	MustRegister(p.Registry, apocAvail)

	if caps.Neo4jVersion != "" {
		edition := caps.Neo4jEdition
		if edition == "" {
			edition = "community"
		}
		versionInfo.WithLabelValues(caps.Neo4jVersion, edition).Set(1)
	}
	if caps.APOCExtendedAvailable {
		apocAvail.Set(1)
	}
	if caps.APOCAvailable {
		apocVersion := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "neo4j_apoc_version_info",
			Help: "APOC version; value is always 1. Absent if APOC is not installed.",
		}, []string{"version"})
		MustRegister(p.Registry, apocVersion)
		apocVersion.WithLabelValues(caps.APOCVersion).Set(1)
	}
	return nil
}
