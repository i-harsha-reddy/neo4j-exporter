// Drop+recreate so neo4j_index_population_percent emits a non-NaN value during
// the population window. e2e calls this via `run.sh --once index_churn` for
// determinism.

DROP INDEX account_balance IF EXISTS;
CREATE RANGE INDEX account_balance FOR (a:Account) ON (a.balance);
