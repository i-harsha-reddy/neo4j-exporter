// Package config defines the YAML schema for neo4j-exporter and loads it
// with secret interpolation, validation, and atomic reload.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level YAML schema.
type Config struct {
	Global        Global             `yaml:"global"`
	Modules       map[string]Module  `yaml:"modules"`
	Targets       []Target           `yaml:"targets"`
	DefaultTarget *Target            `yaml:"default_target,omitempty"`

	// path is set by Load so Reload knows where to read from.
	path string
}

type Global struct {
	DefaultModule       string        `yaml:"default_module"`
	ScrapeTimeout       time.Duration `yaml:"scrape_timeout"`
	CollectConcurrency  int           `yaml:"collect_concurrency"`
	Driver              DriverConfig  `yaml:"driver"`
}

type DriverConfig struct {
	MaxConnectionPoolSize         int           `yaml:"max_connection_pool_size"`
	MaxConnectionLifetime         time.Duration `yaml:"max_connection_lifetime"`
	ConnectionAcquisitionTimeout  time.Duration `yaml:"connection_acquisition_timeout"`
}

type Module struct {
	Timeout             time.Duration             `yaml:"timeout"`
	Collectors          CollectorToggles          `yaml:"collectors"`
	SlowQuery           SlowQueryConfig           `yaml:"slow_query"`
	PerCollectorTimeout map[string]time.Duration  `yaml:"per_collector_timeout"`
}

// CollectorToggles is a per-collector tri-state (off/on/auto).
//
// YAML accepts: true | false | "auto" | "true" | "false" | "on" | "off".
type CollectorToggles map[string]EnableMode

type EnableMode int

const (
	Disabled EnableMode = iota
	Enabled
	Auto
)

func (m EnableMode) String() string {
	switch m {
	case Enabled:
		return "true"
	case Auto:
		return "auto"
	default:
		return "false"
	}
}

func (m *EnableMode) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		switch strings.ToLower(value.Value) {
		case "true", "yes", "on", "1", "enabled":
			*m = Enabled
		case "false", "no", "off", "0", "disabled", "":
			*m = Disabled
		case "auto":
			*m = Auto
		default:
			return fmt.Errorf("invalid collector toggle %q (want true|false|auto)", value.Value)
		}
		return nil
	default:
		return fmt.Errorf("collector toggle must be scalar")
	}
}

func (m EnableMode) MarshalYAML() (interface{}, error) { return m.String(), nil }

type SlowQueryConfig struct {
	Enabled          bool          `yaml:"enabled"`
	MinSeconds       float64       `yaml:"min_seconds"`
	Top              int           `yaml:"top"`
	ExposeQueryText  bool          `yaml:"expose_query_text"`
	Timeout          time.Duration `yaml:"timeout"`
}

// Target describes a single Neo4j instance the exporter can probe.
type Target struct {
	Name    string         `yaml:"name"`
	Address string         `yaml:"address"`
	Module  string         `yaml:"module"`
	Bolt    BoltConfig     `yaml:"bolt"`
	Jolokia *JolokiaConfig `yaml:"jolokia,omitempty"`
}

type BoltConfig struct {
	Scheme       string     `yaml:"scheme"`        // neo4j, bolt, neo4j+s, bolt+s, neo4j+ssc, bolt+ssc
	Username     string     `yaml:"username"`
	Password     string     `yaml:"password,omitempty"`
	PasswordFile string     `yaml:"password_file,omitempty"`
	Database     string     `yaml:"database,omitempty"`
	Auth         string     `yaml:"auth,omitempty"`         // "none" to disable auth
	TLS          *TLSConfig `yaml:"tls,omitempty"`
}

type JolokiaConfig struct {
	URL  string         `yaml:"url"`
	Auth *JolokiaAuth   `yaml:"auth,omitempty"`
	TLS  *TLSConfig     `yaml:"tls,omitempty"`
}

type JolokiaAuth struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password,omitempty"`
	PasswordFile string `yaml:"password_file,omitempty"`
}

type TLSConfig struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	ServerName         string `yaml:"server_name,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
}

// ----- Load / Reload -----

// LoadFile reads, interpolates, validates, and returns a Config.
func LoadFile(path string, allowInsecureSecretFiles bool) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	expanded, err := expandSecrets(string(raw))
	if err != nil {
		return nil, fmt.Errorf("expand secrets in %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	c.path = path
	c.applyDefaults()
	if err := c.validate(allowInsecureSecretFiles); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Global.DefaultModule == "" {
		c.Global.DefaultModule = "default"
	}
	if c.Global.ScrapeTimeout == 0 {
		c.Global.ScrapeTimeout = 10 * time.Second
	}
	if c.Global.CollectConcurrency == 0 {
		c.Global.CollectConcurrency = 8
	}
	if c.Global.Driver.MaxConnectionPoolSize == 0 {
		c.Global.Driver.MaxConnectionPoolSize = 4
	}
	if c.Global.Driver.MaxConnectionLifetime == 0 {
		c.Global.Driver.MaxConnectionLifetime = time.Hour
	}
	if c.Global.Driver.ConnectionAcquisitionTimeout == 0 {
		c.Global.Driver.ConnectionAcquisitionTimeout = 5 * time.Second
	}
	for name, m := range c.Modules {
		if m.Timeout == 0 {
			m.Timeout = c.Global.ScrapeTimeout
		}
		if m.SlowQuery.Top == 0 {
			m.SlowQuery.Top = 10
		}
		if m.SlowQuery.MinSeconds == 0 {
			m.SlowQuery.MinSeconds = 5
		}
		if m.SlowQuery.Timeout == 0 {
			m.SlowQuery.Timeout = 5 * time.Second
		}
		c.Modules[name] = m
	}
	for i := range c.Targets {
		applyTargetDefaults(&c.Targets[i])
	}
	if c.DefaultTarget != nil {
		applyTargetDefaults(c.DefaultTarget)
	}
}

func applyTargetDefaults(t *Target) {
	if t.Bolt.Scheme == "" {
		t.Bolt.Scheme = "neo4j"
	}
	if t.Bolt.Database == "" {
		t.Bolt.Database = "neo4j"
	}
	if !strings.Contains(t.Address, ":") {
		t.Address += ":7687"
	}
}

func (c *Config) validate(allowInsecureSecretFiles bool) error {
	if len(c.Modules) == 0 {
		return fmt.Errorf("at least one module must be defined")
	}
	if _, ok := c.Modules[c.Global.DefaultModule]; !ok {
		return fmt.Errorf("default_module %q is not defined in modules", c.Global.DefaultModule)
	}
	for name, m := range c.Modules {
		for collector := range m.Collectors {
			if !knownCollector(collector) {
				return fmt.Errorf("module %q references unknown collector %q", name, collector)
			}
		}
		for collector := range m.PerCollectorTimeout {
			if !knownCollector(collector) {
				return fmt.Errorf("module %q has per_collector_timeout for unknown collector %q", name, collector)
			}
		}
	}
	seen := make(map[string]struct{}, len(c.Targets))
	for i, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("target[%d]: name is required", i)
		}
		if _, dup := seen[t.Name]; dup {
			return fmt.Errorf("target %q is defined more than once", t.Name)
		}
		seen[t.Name] = struct{}{}
		if t.Address == "" {
			return fmt.Errorf("target %q: address is required", t.Name)
		}
		if t.Module == "" {
			t.Module = c.Global.DefaultModule
			c.Targets[i].Module = t.Module
		}
		if _, ok := c.Modules[t.Module]; !ok {
			return fmt.Errorf("target %q references undefined module %q", t.Name, t.Module)
		}
		if err := validateBolt(t.Address, t.Bolt, allowInsecureSecretFiles); err != nil {
			return fmt.Errorf("target %q: %w", t.Name, err)
		}
		if t.Jolokia != nil {
			if err := validateJolokia(t.Jolokia, allowInsecureSecretFiles); err != nil {
				return fmt.Errorf("target %q jolokia: %w", t.Name, err)
			}
		}
	}
	if c.DefaultTarget != nil {
		if err := validateBolt("default_target", c.DefaultTarget.Bolt, allowInsecureSecretFiles); err != nil {
			return fmt.Errorf("default_target: %w", err)
		}
	}
	return nil
}

// FindTarget locates a target by address (host:port) or by name. For SD-style
// probes where the listed targets don't include the scraped address, falls
// back to default_target if defined.
func (c *Config) FindTarget(query string) (Target, bool) {
	for _, t := range c.Targets {
		if t.Name == query {
			return t, true
		}
	}
	address := query
	if !strings.Contains(address, ":") {
		address += ":7687"
	}
	for _, t := range c.Targets {
		if t.Address == address {
			return t, true
		}
	}
	if c.DefaultTarget != nil {
		t := *c.DefaultTarget
		t.Address = address
		if t.Module == "" {
			t.Module = c.Global.DefaultModule
		}
		applyTargetDefaults(&t)
		return t, true
	}
	return Target{}, false
}

// Module returns a module by name, or false if not found.
func (c *Config) Module(name string) (Module, bool) {
	if name == "" {
		name = c.Global.DefaultModule
	}
	m, ok := c.Modules[name]
	return m, ok
}

// Path returns the file path the config was loaded from (used by Reload).
func (c *Config) Path() string { return c.path }

// ----- Manager: atomic reload -----

// Manager owns the active *Config and supports atomic reload via SIGHUP or HTTP.
type Manager struct {
	current                  atomic.Pointer[Config]
	allowInsecureSecretFiles bool

	mu sync.Mutex // serializes reloads
}

func NewManager(initial *Config, allowInsecureSecretFiles bool) *Manager {
	m := &Manager{allowInsecureSecretFiles: allowInsecureSecretFiles}
	m.current.Store(initial)
	return m
}

// Get returns the current config (reads lock-free).
func (m *Manager) Get() *Config { return m.current.Load() }

// Reload reloads from the original file path and atomically swaps on success.
// On failure the previous config remains active and the error is returned.
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.current.Load()
	if cur == nil || cur.path == "" {
		return fmt.Errorf("config has no path; cannot reload")
	}
	next, err := LoadFile(cur.path, m.allowInsecureSecretFiles)
	if err != nil {
		return err
	}
	m.current.Store(next)
	return nil
}

// ----- helpers -----

func knownCollector(name string) bool {
	switch name {
	case "server", "bolt", "databases", "transactions", "indexes", "constraints",
		"apoc_kernel", "apoc_store", "apoc_tx", "apoc_ids", "apoc_meta",
		"jolokia", "settings":
		return true
	}
	return false
}

func validateBolt(label string, b BoltConfig, allowInsecureSecretFiles bool) error {
	switch b.Scheme {
	case "neo4j", "bolt", "neo4j+s", "bolt+s", "neo4j+ssc", "bolt+ssc":
	default:
		return fmt.Errorf("invalid bolt.scheme %q", b.Scheme)
	}
	if b.Auth == "none" {
		// Allowed only for localhost-style addresses; refuse for non-local.
		if !isLocalhostAddr(label) {
			return fmt.Errorf("bolt.auth=none is only permitted for localhost; got %q", label)
		}
		return nil
	}
	if b.Username == "" {
		return fmt.Errorf("bolt.username is required (or set bolt.auth: none for localhost)")
	}
	if b.Password == "" && b.PasswordFile == "" {
		return fmt.Errorf("exactly one of bolt.password or bolt.password_file is required")
	}
	if b.Password != "" && b.PasswordFile != "" {
		return fmt.Errorf("only one of bolt.password or bolt.password_file may be set")
	}
	if b.PasswordFile != "" {
		if err := checkSecretFileMode(b.PasswordFile, allowInsecureSecretFiles); err != nil {
			return err
		}
	}
	return nil
}

func validateJolokia(j *JolokiaConfig, allowInsecureSecretFiles bool) error {
	if j.URL == "" {
		return fmt.Errorf("jolokia.url is required")
	}
	if j.Auth != nil {
		if j.Auth.Username == "" {
			return fmt.Errorf("jolokia.auth.username is required when auth is set")
		}
		if j.Auth.Password == "" && j.Auth.PasswordFile == "" {
			return fmt.Errorf("jolokia.auth requires password or password_file")
		}
		if j.Auth.PasswordFile != "" {
			if err := checkSecretFileMode(j.Auth.PasswordFile, allowInsecureSecretFiles); err != nil {
				return err
			}
		}
	}
	return nil
}

func isLocalhostAddr(addr string) bool {
	if addr == "default_target" {
		return true
	}
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "":
		return true
	}
	return false
}

func checkSecretFileMode(path string, allowInsecure bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", path, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}
	mode := st.Mode().Perm()
	// Allowed: 0400, 0440, 0600, 0640. Group-read is allowed because kubelet's
	// fsGroup OR's group-read into the mode of Secret-mounted files (0400 -> 0440,
	// 0600 -> 0640) so a nonroot container can read them. World-readable bits
	// (any "other" perm) and execute bits are rejected.
	if mode&^os.FileMode(0o640) == 0 && mode&0o400 != 0 {
		return nil
	}
	if allowInsecure {
		return nil
	}
	return fmt.Errorf("secret file %q has insecure mode %o (want 0400, 0440, 0600, or 0640); pass --config.allow-insecure-secret-files to override", path, mode)
}
