package neo4jclient

import (
	"context"
	"sync"
	"time"

	cfg "github.com/i-harsha-reddy/neo4j-exporter/internal/config"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Capabilities is what we know about a target's Neo4j build and APOC
// installation. Cached per pool entry; reset on driver swap.
type Capabilities struct {
	Neo4jVersion          string
	Neo4jEdition          string
	APOCAvailable         bool
	APOCVersion           string
	APOCExtendedAvailable bool
	HasAPOCMonitorKernel  bool
	HasAPOCMonitorStore   bool
	HasAPOCMonitorTx      bool
	HasAPOCMonitorIds     bool
	HasAPOCMetaStats      bool

	probedAt time.Time
}

// Entry is one live Neo4j driver plus capability cache.
type Entry struct {
	Driver  neo4j.DriverWithContext
	Address string
	Bolt    cfg.BoltConfig

	mu   sync.Mutex
	caps *Capabilities
}

// Pool keeps one driver per (address, auth-hash). Drivers persist across
// probes; reload differences close stale drivers after a grace window.
type Pool struct {
	mu      sync.Mutex
	entries map[string]*Entry // key = authHash(address, bolt)
	global  cfg.DriverConfig

	// pending close-on-reload entries; closed after Grace.
	stale []staleEntry
	Grace time.Duration
}

type staleEntry struct {
	entry      *Entry
	deadline   time.Time
}

func NewPool(global cfg.DriverConfig) *Pool {
	return &Pool{
		entries: make(map[string]*Entry),
		global:  global,
		Grace:   30 * time.Second,
	}
}

// Get returns a driver entry for the given target, creating it on first use.
func (p *Pool) Get(t cfg.Target) (*Entry, error) {
	key, err := authHash(t.Address, t.Bolt)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.entries[key]; ok {
		return e, nil
	}
	d, err := NewDriver(t.Address, t.Bolt, p.global)
	if err != nil {
		return nil, err
	}
	e := &Entry{Driver: d, Address: t.Address, Bolt: t.Bolt}
	p.entries[key] = e
	return e, nil
}

// Reconcile compares the active config with the pool. Entries whose key
// disappeared are moved to the stale list and closed after Grace.
func (p *Pool) Reconcile(ctx context.Context, targets []cfg.Target) {
	keep := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		k, err := authHash(t.Address, t.Bolt)
		if err == nil {
			keep[k] = struct{}{}
		}
	}
	now := time.Now()
	p.mu.Lock()
	for k, e := range p.entries {
		if _, ok := keep[k]; !ok {
			p.stale = append(p.stale, staleEntry{entry: e, deadline: now.Add(p.Grace)})
			delete(p.entries, k)
		}
	}
	due := p.stale[:0]
	for _, s := range p.stale {
		if now.After(s.deadline) {
			_ = s.entry.Driver.Close(ctx)
		} else {
			due = append(due, s)
		}
	}
	p.stale = due
	p.mu.Unlock()
}

// CloseAll closes every driver. Called at shutdown.
func (p *Pool) CloseAll(ctx context.Context) {
	p.mu.Lock()
	entries := p.entries
	stale := p.stale
	p.entries = make(map[string]*Entry)
	p.stale = nil
	p.mu.Unlock()
	for _, e := range entries {
		_ = e.Driver.Close(ctx)
	}
	for _, s := range stale {
		_ = s.entry.Driver.Close(ctx)
	}
}

// Size returns the number of live driver entries.
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}
