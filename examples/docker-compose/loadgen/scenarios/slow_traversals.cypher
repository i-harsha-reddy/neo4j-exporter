// Deterministically slow via apoc.util.sleep. Tuned long (first ~14s, second ~6s)
// so each statement stays in-flight across the /slow-queries scrape interval —
// the snapshot only shows queries running *right now* — and the first crosses
// 10s elapsed to exercise the dashboard's "backlog (>10s)" panel.
// Each statement runs as a separate transaction in cypher-shell.

MATCH (n) WITH count(n) AS total
CALL apoc.util.sleep(14000)
RETURN total;

MATCH (a:Account), (p:Person)
WITH a, p LIMIT 1000000
WITH count(*) AS pair_count
CALL apoc.util.sleep(6000)
RETURN pair_count;
