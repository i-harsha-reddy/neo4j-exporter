---
description: Pre-commit preflight — go vet, race tests, build. Run before any commit or PR.
---

Run the full preflight in this order. Stop at the first failure and report it concisely (file:line + the actual error from compiler/test output).

```bash
go vet ./...
go test -race -count=1 ./...
go build -trimpath ./cmd/neo4j-exporter
```

If `golangci-lint` is on PATH, also run:
```bash
golangci-lint run --timeout 3m ./...
```

If `govulncheck` is installed, also run:
```bash
govulncheck ./...
```

Report at the end:
- ✅/❌ each step
- For failures: the actual error, the file:line, and a one-line proposed fix (don't edit yet — wait for the user to ack).
- For warnings (vet, lint): list them with file:line, ranked by severity.

Do NOT proceed to fix anything without the user confirming. The point of `/check` is the report.
