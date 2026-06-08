UNWIND range(1, 200) AS i
MATCH (a:Account {id: toInteger(rand() * 50000) + 1})
SET a.balance      = a.balance + (rand() * 100 - 50),
    a.last_updated = timestamp(),
    a.touch_count  = coalesce(a.touch_count, 0) + 1;
