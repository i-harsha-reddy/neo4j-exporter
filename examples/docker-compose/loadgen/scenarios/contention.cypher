// Hammer the 10 :HotSpot Account ids. run.sh fires this 4× concurrently per
// invocation; each writer holds the same locks so neo4j_transactions_active_wait_seconds
// and _active_lock_count move.

UNWIND range(1, 10) AS hot_id
MATCH (a:Account:HotSpot {id: hot_id})
SET a.balance      = a.balance + 1,
    a.last_writer  = "writer-" + toString(timestamp() % 1000),
    a.last_updated = timestamp();
