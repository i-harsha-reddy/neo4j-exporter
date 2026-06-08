package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildTLSConfig converts a *TLSConfig into a *tls.Config suitable for
// http.Transport or the Neo4j driver's connection. Returns nil if the
// input is nil (caller defaults to no TLS).
func BuildTLSConfig(t *TLSConfig) (*tls.Config, error) {
	if t == nil {
		return nil, nil
	}
	out := &tls.Config{
		ServerName:         t.ServerName,
		InsecureSkipVerify: t.InsecureSkipVerify, //nolint:gosec // gated by config
		MinVersion:         tls.VersionTLS12,
	}
	if t.CAFile != "" {
		raw, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file %q: %w", t.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("ca_file %q contains no PEM certificates", t.CAFile)
		}
		out.RootCAs = pool
	}
	if (t.CertFile == "") != (t.KeyFile == "") {
		return nil, fmt.Errorf("cert_file and key_file must both be set or both empty")
	}
	if t.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		out.Certificates = []tls.Certificate{cert}
	}
	return out, nil
}
