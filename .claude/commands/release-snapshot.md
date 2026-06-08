---
description: Build a goreleaser snapshot (binaries + Docker images, no publish). Use to verify release config without tagging.
---

Run a goreleaser snapshot to verify the release config without creating a tag or pushing anything.

Pre-flight:
- `which goreleaser` — install via `brew install goreleaser` if missing.
- `git status --porcelain` — warn if working tree is dirty (snapshots are version-stamped from git state).

Run:
```bash
goreleaser release --snapshot --clean --skip=publish
```

This will:
- Build binaries for `linux/{amd64,arm64}` and `darwin/{amd64,arm64}` into `dist/`
- Build multi-arch Docker images locally (no push since `--skip=publish`)
- Generate a `dist/checksums.txt`
- Generate `dist/CHANGELOG.md`

Verify the artifacts:
```bash
ls -lh dist/
file dist/neo4j-exporter_*_linux_amd64/neo4j-exporter
docker images | grep neo4j-exporter
```

If `goreleaser` complains about a missing tag, suggest creating one with `git tag v0.0.1-rc1` (the user decides; do not auto-tag).
