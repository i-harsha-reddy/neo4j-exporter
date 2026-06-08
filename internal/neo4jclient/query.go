package neo4jclient

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Row is a Cypher result row as a flat map. Easier to consume in collectors
// than the driver's Record type.
type Row map[string]any

// ExecRead runs a read query and returns all records as []Row. Uses the
// driver-level ExecuteQuery for clean session management.
func (e *Entry) ExecRead(ctx context.Context, database, cypher string, params map[string]any) ([]Row, error) {
	if database == "" {
		database = e.Bolt.Database
		if database == "" {
			database = "neo4j"
		}
	}
	res, err := neo4j.ExecuteQuery(ctx, e.Driver, cypher, params,
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithReadersRouting(),
		neo4j.ExecuteQueryWithDatabase(database),
	)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", trim(cypher, 80), err)
	}
	out := make([]Row, 0, len(res.Records))
	for _, rec := range res.Records {
		row := make(Row, len(rec.Keys))
		for i, k := range rec.Keys {
			row[k] = rec.Values[i]
		}
		out = append(out, row)
	}
	return out, nil
}

// ExecReadSystem is a convenience wrapper that runs against the system
// database (used for SHOW DATABASES).
func (e *Entry) ExecReadSystem(ctx context.Context, cypher string, params map[string]any) ([]Row, error) {
	return e.ExecRead(ctx, "system", cypher, params)
}

// VerifyConnectivity confirms the driver can establish a session. The
// timing of this call is what we expose as neo4j_bolt_handshake_duration_seconds.
func (e *Entry) VerifyConnectivity(ctx context.Context) error {
	return e.Driver.VerifyConnectivity(ctx)
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
