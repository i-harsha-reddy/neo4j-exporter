// Package jolokiaclient is a tiny Jolokia HTTP→JMX client tuned for
// bulk-read probes. We use Jolokia (https://jolokia.org/) because the
// Neo4j Go driver gives us no path to JVM-level MBeans, and standard
// JVM MBeans (java.lang:*) are exposed by every JVM regardless of edition.
package jolokiaclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
)

// Client is a Jolokia HTTP endpoint with credentials and TLS pre-resolved.
type Client struct {
	URL      string
	username string
	password string
	http     *http.Client
}

// New builds a Client. Returns nil, nil if jc is nil (collector should
// then disable itself silently).
func New(jc *cfg.JolokiaConfig, timeout time.Duration) (*Client, error) {
	if jc == nil {
		return nil, nil
	}
	if jc.URL == "" {
		return nil, fmt.Errorf("jolokia.url is required")
	}
	tlsCfg, err := cfg.BuildTLSConfig(jc.TLS)
	if err != nil {
		return nil, fmt.Errorf("jolokia tls: %w", err)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if tlsCfg != nil {
		tr.TLSClientConfig = tlsCfg
	}
	c := &Client{
		URL:  ensureTrailingSlash(jc.URL),
		http: &http.Client{Timeout: timeout, Transport: tr},
	}
	if jc.Auth != nil {
		c.username = jc.Auth.Username
		pw, err := cfg.ResolveJolokiaPassword(jc.Auth)
		if err != nil {
			return nil, err
		}
		c.password = pw
	}
	return c, nil
}

// Bulk POSTs a slice of read requests and returns the parallel slice of
// responses. The responses are in the same order as the requests.
//
// Per the Jolokia protocol each response has a `status` field; HTTP-level
// errors come from this method as `error`, while per-request MBean errors
// stay inside the response array (status != 200).
func (c *Client) Bulk(ctx context.Context, reqs []ReadRequest) ([]Response, error) {
	if c == nil {
		return nil, fmt.Errorf("jolokia client is nil")
	}
	body, err := json.Marshal(reqs)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MiB cap
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jolokia returned %d: %s", resp.StatusCode, trimUTF8(string(raw), 256))
	}
	// Jolokia returns either a single object or an array; bulk requests
	// always return an array. Accept both for robustness.
	var multi []Response
	if err := json.Unmarshal(raw, &multi); err == nil {
		return multi, nil
	}
	var single Response
	if err := json.Unmarshal(raw, &single); err == nil {
		return []Response{single}, nil
	}
	return nil, fmt.Errorf("decode jolokia response: %s", trimUTF8(string(raw), 256))
}

func ensureTrailingSlash(s string) string {
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}

func trimUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && (s[n]&0xC0) == 0x80 {
		n--
	}
	return s[:n] + "…"
}
