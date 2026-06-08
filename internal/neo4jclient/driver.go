// Package neo4jclient wraps the Neo4j Go driver with a per-target pool,
// APOC capability auto-detection, and read-query helpers tailored for the
// exporter's collectors.
package neo4jclient

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	driverconf "github.com/neo4j/neo4j-go-driver/v5/neo4j/config"
)

// NewDriver builds a Neo4j driver for a single target.
//
// TLS handling: encryption is selected by the URI scheme (neo4j+s/bolt+s,
// neo4j+ssc/bolt+ssc). For custom root CAs we fall back to system trust —
// custom-CA + per-target trust requires neo4j.WithCustomTrustStrategy and
// is left to v1.1 (the JolokiaConfig.TLS path covers the equivalent for
// the JVM-metrics side, where it matters more).
func NewDriver(address string, b cfg.BoltConfig, g cfg.DriverConfig) (neo4j.DriverWithContext, error) {
	uri := b.Scheme + "://" + address
	pw, err := cfg.ResolveBoltPassword(b)
	if err != nil {
		return nil, err
	}
	var token neo4j.AuthToken
	if b.Auth == "none" {
		token = neo4j.NoAuth()
	} else {
		token = neo4j.BasicAuth(b.Username, pw, "")
	}
	return neo4j.NewDriverWithContext(uri, token, func(c *driverconf.Config) {
		if g.MaxConnectionPoolSize > 0 {
			c.MaxConnectionPoolSize = g.MaxConnectionPoolSize
		}
		if g.MaxConnectionLifetime > 0 {
			c.MaxConnectionLifetime = g.MaxConnectionLifetime
		}
		if g.ConnectionAcquisitionTimeout > 0 {
			c.ConnectionAcquisitionTimeout = g.ConnectionAcquisitionTimeout
		}
		c.UserAgent = "neo4j-exporter/1.0"
	})
}

// authHash returns a stable digest of the connection identity. Two targets
// with the same (uri, username, password) share a driver entry in the pool;
// when any of them changes on reload, the pool key changes and the old
// driver is closed after a grace window.
func authHash(address string, b cfg.BoltConfig) (string, error) {
	pw, err := cfg.ResolveBoltPassword(b)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s|%s", b.Scheme, address, b.Username, pw, b.Auth)
	return hex.EncodeToString(h.Sum(nil)), nil
}
