// Deterministically slow via apoc.util.sleep; min duration ~3s.
// Each statement runs as a separate transaction in cypher-shell.

MATCH (n) WITH count(n) AS total
CALL apoc.util.sleep(3000)
RETURN total;

MATCH (a:Account), (p:Person)
WITH a, p LIMIT 1000000
WITH count(*) AS pair_count
CALL apoc.util.sleep(2000)
RETURN pair_count;
