// Deliberate UNIQUE-constraint violation. cypher-shell exits non-zero with
// Neo.ClientError.Schema.ConstraintValidationFailed; run.sh expects and
// swallows that single failure mode so neo4j_transactions_rolled_back_count
// strictly grows.

CREATE (:Account {
  id: 1,
  email: "user1@example.com",
  name: "Duplicate",
  balance: 0,
  location: point({x: 0, y: 0}),
  created: timestamp()
});
