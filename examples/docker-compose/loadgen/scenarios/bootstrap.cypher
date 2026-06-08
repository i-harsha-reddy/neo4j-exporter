// Idempotent demo bootstrap. CE-only constraint syntax (UNIQUE only).
//
// After this runs:
//   neo4j_constraints_count{type="UNIQUENESS"} == 3
//   neo4j_indexes_count                       >= 6   (3 implicit from constraints + 3 explicit)
//   neo4j_meta_node_count                     ~= 55000
//   neo4j_meta_relationship_count             ~= 50000

CREATE CONSTRAINT account_id_unique IF NOT EXISTS
FOR (a:Account) REQUIRE a.id IS UNIQUE;

CREATE CONSTRAINT account_email_unique IF NOT EXISTS
FOR (a:Account) REQUIRE a.email IS UNIQUE;

CREATE CONSTRAINT person_id_name_unique IF NOT EXISTS
FOR (p:Person) REQUIRE (p.id, p.name) IS UNIQUE;

CREATE RANGE INDEX account_balance IF NOT EXISTS
FOR (a:Account) ON (a.balance);

CREATE TEXT INDEX account_name IF NOT EXISTS
FOR (a:Account) ON (a.name);

CREATE POINT INDEX account_location IF NOT EXISTS
FOR (a:Account) ON (a.location);

CALL apoc.periodic.iterate(
  'UNWIND range(1, 50000) AS i RETURN i',
  'MERGE (a:Account {id: i})
     ON CREATE SET a.email    = "user" + i + "@example.com",
                   a.name     = "Account-" + i,
                   a.balance  = rand() * 1000000,
                   a.location = point({x: rand()*180 - 90, y: rand()*90 - 45}),
                   a.created  = timestamp()',
  {batchSize: 5000, parallel: false}
) YIELD batches, total;

CALL apoc.periodic.iterate(
  'UNWIND range(1, 5000) AS i RETURN i',
  'MERGE (p:Person {id: i, name: "Person-" + i})
     ON CREATE SET p.created = timestamp()',
  {batchSize: 1000, parallel: false}
) YIELD batches, total;

// 10 hot-spot account ids the contention scenario will hammer.
MATCH (a:Account) WHERE a.id <= 10 SET a:HotSpot;

CALL apoc.periodic.iterate(
  'UNWIND range(1, 50000) AS i RETURN i',
  'MATCH (p1:Person {id: ((i-1) % 5000) + 1}),
         (p2:Person {id: (i % 5000) + 1})
   WHERE p1 <> p2
   MERGE (p1)-[r:KNOWS]->(p2)
     ON CREATE SET r.weight = rand(), r.created = timestamp()',
  {batchSize: 2000, parallel: false}
) YIELD batches, total;
