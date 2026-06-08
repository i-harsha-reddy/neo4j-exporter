// Cycles through 20 labels and 20 rel-types over time so apoc.meta.stats
// returns a varied schema. apoc.create.node/relationship lets us pass the
// label/type as a runtime variable.

WITH "Tag"   + toString((timestamp() / 1000) % 20 + 1) AS tag,
     "REL_T" + toString((timestamp() / 1000) % 20 + 1) AS rt
CALL apoc.create.node([tag], {created: timestamp(), iter: timestamp() / 1000}) YIELD node AS n1
WITH n1, rt
CALL apoc.create.node(["Generic"], {created: timestamp()}) YIELD node AS n2
CALL apoc.create.relationship(n1, rt, {created: timestamp()}, n2) YIELD rel
RETURN count(*) AS created;
