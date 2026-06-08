// High-churn allocation pattern. On the demo heap settings this reliably
// triggers G1 young-gen activity so neo4j_jvm_gc_count_total moves.

CALL apoc.periodic.iterate(
  'UNWIND range(1, 200000) AS i RETURN i',
  'CREATE (:Audit {ts: timestamp(), i: i, payload: "x" + toString(i)})',
  {batchSize: 5000, parallel: false}
) YIELD batches, total;

CALL apoc.periodic.iterate(
  'MATCH (a:Audit) WHERE a.ts < timestamp() - 300000 RETURN a',
  'DETACH DELETE a',
  {batchSize: 5000, parallel: false}
) YIELD batches, total;
