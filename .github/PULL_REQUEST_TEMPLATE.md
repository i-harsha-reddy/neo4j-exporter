<!--
Thanks for opening a PR. Before submitting, please confirm the relevant items below.
For security-sensitive changes, see SECURITY.md and email first.
-->

## Summary

<!-- One or two sentences. Focus on the *why*, not a play-by-play of files changed. -->

## Test plan

<!-- How did you verify this works? Specific commands, screenshots of dashboards,
     /probe diff before/after, etc. -->

## Checklist

- [ ] `make test` passes (no race warnings)
- [ ] `make lint` passes (golangci-lint v2.x)
- [ ] If you touched a collector / orchestrator / driver pool / demo compose / loadgen: `make e2e` passes
- [ ] If you touched the chart: `make chart-test` passes; `values.yaml` comments updated
- [ ] If you touched documented metrics: `docs/METRICS.md` updated
- [ ] Commit messages explain *why*, not *what*
- [ ] I've read `CLAUDE.md` (conventions specific to this repo) and any deviations are explained above

## Related issues

<!-- Closes #N / Refs #N / N/A -->
