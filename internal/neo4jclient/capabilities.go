package neo4jclient

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Capabilities returns the cached capabilities for this entry, probing
// once on first call. Subsequent calls return the cached value.
//
// Probe queries:
//   1. CALL dbms.components() — Neo4j version + edition (always works)
//   2. RETURN apoc.version() — APOC version (errors if APOC absent; we swallow)
//   3. SHOW PROCEDURES YIELD name WHERE name STARTS WITH 'apoc.' AND
//      (name = 'apoc.monitor.kernel' OR name = 'apoc.monitor.store' OR
//       name = 'apoc.monitor.tx' OR name = 'apoc.monitor.ids' OR
//       name = 'apoc.meta.stats')
//      RETURN name
func (e *Entry) Capabilities(ctx context.Context) (*Capabilities, error) {
	e.mu.Lock()
	if e.caps != nil {
		c := e.caps
		e.mu.Unlock()
		return c, nil
	}
	e.mu.Unlock()

	caps := &Capabilities{probedAt: time.Now()}

	if rows, err := e.ExecRead(ctx, "system", "CALL dbms.components()", nil); err == nil {
		for _, r := range rows {
			caps.Neo4jVersion = firstStringFromList(r["versions"])
			caps.Neo4jEdition = asString(r["edition"])
			break
		}
	} else {
		return nil, fmt.Errorf("dbms.components: %w", err)
	}

	// APOC version — single-row scalar. Failure means APOC is not installed.
	if rows, err := e.ExecRead(ctx, "", "RETURN apoc.version() AS v", nil); err == nil && len(rows) > 0 {
		caps.APOCAvailable = true
		caps.APOCVersion = asString(rows[0]["v"])
	}

	// Procedure existence probe. We deliberately probe each name explicitly
	// rather than enumerating all apoc.* procedures (which can be slow on
	// large schemas). Errors here are non-fatal: SHOW PROCEDURES should
	// always work in 5.x; if it errors, Has* flags stay false and the
	// collectors degrade gracefully.
	if rows, err := e.ExecRead(ctx, "", `
SHOW PROCEDURES YIELD name
WHERE name IN ['apoc.monitor.kernel','apoc.monitor.store','apoc.monitor.tx',
               'apoc.monitor.ids','apoc.meta.stats']
RETURN name
`, nil); err == nil {
		for _, r := range rows {
			switch asString(r["name"]) {
			case "apoc.monitor.kernel":
				caps.HasAPOCMonitorKernel = true
			case "apoc.monitor.store":
				caps.HasAPOCMonitorStore = true
			case "apoc.monitor.tx":
				caps.HasAPOCMonitorTx = true
			case "apoc.monitor.ids":
				caps.HasAPOCMonitorIds = true
			case "apoc.meta.stats":
				caps.HasAPOCMetaStats = true
			}
		}
	}
	caps.APOCExtendedAvailable = caps.HasAPOCMonitorKernel || caps.HasAPOCMonitorStore ||
		caps.HasAPOCMonitorTx || caps.HasAPOCMonitorIds

	e.mu.Lock()
	e.caps = caps
	e.mu.Unlock()
	return caps, nil
}

// InvalidateCapabilities forces the next Capabilities() call to re-probe.
// Called on config reload to pick up newly-installed plugins.
func (e *Entry) InvalidateCapabilities() {
	e.mu.Lock()
	e.caps = nil
	e.mu.Unlock()
}

// ----- type coercion helpers -----

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func firstStringFromList(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case []any:
		if len(x) == 0 {
			return ""
		}
		return asString(x[0])
	case []string:
		if len(x) == 0 {
			return ""
		}
		return x[0]
	default:
		return asString(v)
	}
}
