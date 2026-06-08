#!/usr/bin/env bash
# End-to-end smoke test for neo4j-exporter against the 2-instance docker-compose
# stack. Hard assertions only — no soft warnings.
#
# Usage: scripts/e2e.sh
# Requires: docker, docker compose, curl

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( dirname "$SCRIPT_DIR" )"
COMPOSE_DIR="$ROOT_DIR/examples/docker-compose"
EXPORTER="http://localhost:9412"
PROM="http://localhost:9090"

INSTANCES=("neo4j-a:7687" "neo4j-b:7687")
METRIC_FILES=()
WARMUP_S="${E2E_WARMUP_S:-75}"

cleanup() {
  echo "--- compose down ---"
  for f in "${METRIC_FILES[@]:-}"; do [[ -n "$f" && -f "$f" ]] && rm -f "$f"; done
  ( cd "$COMPOSE_DIR" && docker compose down -v --remove-orphans ) || true
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
ok()   { echo "  ✓ $*"; }

probe_metrics() {
  local target="$1" out
  out="$(mktemp)"
  curl -fsS "${EXPORTER}/probe?target=${target}&module=default" -o "$out" \
    || { echo "probe failed for $target"; cat "$out" >&2 || true; rm -f "$out"; return 1; }
  echo "$out"
}

refresh_metrics() {
  local i
  for i in "${!INSTANCES[@]}"; do
    [[ -n "${METRIC_FILES[$i]:-}" && -f "${METRIC_FILES[$i]}" ]] && rm -f "${METRIC_FILES[$i]}"
    METRIC_FILES[$i]=$(probe_metrics "${INSTANCES[$i]}")
  done
}

# ---------- bring up ----------
echo "--- compose up (~90s for two Neo4j instances + plugins) ---"
( cd "$COMPOSE_DIR" && docker compose up -d --wait )

# ---------- wait for exporter probe to stabilize on BOTH instances ----------
# `compose --wait` returns when containers are healthy, but that only means the
# exporter HTTP port is reachable. The always-on collectors (especially the
# Cypher SHOW TRANSACTIONS one) may still need a few seconds to succeed against
# a freshly-started Neo4j, particularly on slower CI runners.
echo "--- waiting for always-on collectors to stabilize on both instances ---"
READY_DEADLINE=$(( $(date +%s) + 120 ))
while :; do
  all_ok=1
  for inst in "${INSTANCES[@]}"; do
    out=$(curl -fsS "${EXPORTER}/probe?target=${inst}&module=default" 2>/dev/null) || { all_ok=0; break; }
    for c in bolt transactions databases indexes constraints server; do
      if ! grep -qE "neo4j_collector_success\{collector=\"$c\"\}\s+1" <<<"$out"; then
        all_ok=0
        break 2
      fi
    done
  done
  if [[ $all_ok -eq 1 ]]; then
    break
  fi
  if (( $(date +%s) >= READY_DEADLINE )); then
    fail "always-on collectors did not stabilize on both instances within 120s"
  fi
  sleep 3
done
ok "always-on collectors stable on both instances"

# ---------- 1. HTTP UI sanity ----------
echo "--- 1. Neo4j HTTP UIs ---"
curl -fsS http://localhost:7474 >/dev/null || fail "neo4j-a 7474 unreachable"
ok "neo4j-a HTTP UI 7474"
curl -fsS http://localhost:7475 >/dev/null || fail "neo4j-b 7475 unreachable"
ok "neo4j-b HTTP UI 7475"

# ---------- 2. probe each instance ----------
echo "--- 2. Probing each instance ---"
refresh_metrics
for i in "${!INSTANCES[@]}"; do
  ok "[${INSTANCES[$i]}] /probe returned 200, $(wc -l <"${METRIC_FILES[$i]}") lines"
done

# ---------- 3-9. per-instance metric assertions ----------
echo "--- 3-9. Per-instance metric assertions ---"
for i in "${!INSTANCES[@]}"; do
  I="${INSTANCES[$i]}"
  M="${METRIC_FILES[$i]}"

  grep -qE '^neo4j_up\s+1\s*$' "$M" || fail "[$I] neo4j_up != 1"
  ok "[$I] 3. neo4j_up==1"

  for c in bolt transactions databases indexes constraints server; do
    grep -qE "neo4j_collector_success\{collector=\"$c\"\}\s+1" "$M" \
      || fail "[$I] always-on collector '$c' did not succeed"
  done
  ok "[$I] 4. always-on collectors succeeded"

  for c in apoc_kernel apoc_store apoc_tx apoc_ids apoc_meta jolokia; do
    grep -qE "neo4j_collector_success\{collector=\"$c\"\}\s+1" "$M" \
      || fail "[$I] capability-gated collector '$c' did not succeed"
  done
  ok "[$I] 5. APOC + jolokia collectors succeeded"

  grep -qE '^neo4j_apoc_extended_available\s+1\s*$' "$M" \
    || fail "[$I] neo4j_apoc_extended_available != 1"
  ok "[$I] 6. apoc_extended_available==1"

  grep -qE 'neo4j_neo4j_version_info\{.*edition="community".*version="5\..+"' "$M" \
    || fail "[$I] version_info not 5.x community"
  ok "[$I] 7. version 5.x community"

  grep -qE 'neo4j_database_status\{database="neo4j",status="online"\}\s+1' "$M" \
    || fail "[$I] database 'neo4j' not online"
  ok "[$I] 8. database neo4j online"

  for m in neo4j_jvm_heap_used_bytes neo4j_store_total_bytes; do
    grep -qE "^${m}\s+[1-9]" "$M" || fail "[$I] $m missing or zero"
  done
  # NB: store_id label value contains literal '{...}' braces, so .* (not [^}]*).
  grep -qE '^neo4j_kernel_info\{.*\} 1$' "$M" \
    || fail "[$I] neo4j_kernel_info missing"
  # ids{kind=node} is 0 on fresh empty user DB; just verify metric is emitted.
  grep -qE 'neo4j_ids_in_use_total\{kind="node"\}\s+[0-9]' "$M" \
    || fail "[$I] neo4j_ids_in_use_total{kind=node} missing"
  grep -qE '^neo4j_jvm_gc_count_total\{[^}]*\}' "$M" \
    || fail "[$I] neo4j_jvm_gc_count_total missing"
  ok "[$I] 9. JVM/store/kernel/ids present"
done

# ---------- warmup so bootstrap completes + slow_traversals + rollbacks fire ≥ once ----------
echo "--- Warmup ${WARMUP_S}s for bootstrap to finish and scenarios to fire ---"
sleep "$WARMUP_S"

refresh_metrics

# ---------- 10. constraints (bootstrap should have finished by now) ----------
echo "--- 10. Constraints non-zero (loadgen bootstrap should have created 3 UNIQUENESS) ---"
for i in "${!INSTANCES[@]}"; do
  I="${INSTANCES[$i]}"
  count=$(grep -E 'neo4j_constraints_count\{[^}]*type="UNIQUENESS"[^}]*\}' "${METRIC_FILES[$i]}" \
    | awk '{print $2}' | sort -nr | head -1)
  count=${count%%.*}
  [[ -n "$count" && "$count" -gt 0 ]] || fail "[$I] no UNIQUENESS constraints (bootstrap did not finish?)"
  ok "[$I] UNIQUENESS constraints=$count"
done

# ---------- 11. slow queries present (trigger via --once + poll while in-flight) ----------
echo "--- 11. /slow-queries returns rows (triggered via run.sh --once slow_traversals) ---"
SLOW_FOUND=()
for i in "${!INSTANCES[@]}"; do
  SLOW_FOUND[$i]=0
  ( cd "$COMPOSE_DIR" && docker compose exec -T loadgen bash /loadgen/run.sh --once slow_traversals "${INSTANCES[$i]}" >/dev/null 2>&1 ) &
done
# slow_traversals takes ~5s per instance; poll /slow-queries while in-flight
for n in $(seq 1 10); do
  all_found=1
  for i in "${!INSTANCES[@]}"; do
    if [[ "${SLOW_FOUND[$i]}" == "1" ]]; then continue; fi
    sq=$(curl -fsS "${EXPORTER}/slow-queries?target=${INSTANCES[$i]}&top=10" 2>/dev/null || true)
    if echo "$sq" | grep -qE '^neo4j_slow_query_info\{'; then
      ok "[${INSTANCES[$i]}] /slow-queries has rows"
      SLOW_FOUND[$i]=1
    else
      all_found=0
    fi
  done
  [[ $all_found -eq 1 ]] && break
  sleep 1
done
wait
for i in "${!INSTANCES[@]}"; do
  [[ "${SLOW_FOUND[$i]}" == "1" ]] || fail "[${INSTANCES[$i]}] no slow_query_info after polling"
done

# ---------- 12. rollbacks > 0 ----------
echo "--- 12. transactions_rolled_back_count > 0 (sum across instances) ---"
total_rollbacks=0
for i in "${!INSTANCES[@]}"; do
  v=$(grep -E '^neo4j_transactions_rolled_back_count\{database="neo4j"\}' "${METRIC_FILES[$i]}" \
    | awk '{print $2}' | head -1)
  v_int=${v%%.*}
  total_rollbacks=$((total_rollbacks + ${v_int:-0}))
done
[[ $total_rollbacks -gt 0 ]] || fail "rolled_back total = 0 across instances (rollbacks scenario silent?)"
ok "rolled_back total=$total_rollbacks"

# ---------- 13. index churn round-trip (via run.sh --once) ----------
# Population window for a 50k-row index is sub-second on modern hardware;
# rather than chase that race, exercise the round-trip and verify the index is
# back in ONLINE state with the indexes collector picking it up.
echo "--- 13. Index churn round-trip (DROP + CREATE via --once) ---"
for I in "${INSTANCES[@]}"; do
  if ! ( cd "$COMPOSE_DIR" && docker compose exec -T loadgen bash /loadgen/run.sh --once index_churn "$I" 2>&1 ); then
    fail "[$I] index_churn --once failed"
  fi
done
sleep 3
refresh_metrics
for i in "${!INSTANCES[@]}"; do
  I="${INSTANCES[$i]}"
  count=$(grep -E 'neo4j_indexes_count\{[^}]*state="ONLINE"[^}]*\}' "${METRIC_FILES[$i]}" \
    | awk '{print $2}' | sort -nr | head -1)
  count=${count%%.*}
  [[ -n "$count" && "$count" -gt 0 ]] || fail "[$I] no ONLINE indexes after index_churn"
  ok "[$I] ONLINE indexes=$count after index_churn"
done

# ---------- 14. cardinality cap canary ----------
echo "--- 14. No <truncated> labels (cardinality cap canary) ---"
for i in "${!INSTANCES[@]}"; do
  ! grep -q '<truncated>' "${METRIC_FILES[$i]}" \
    || fail "[${INSTANCES[$i]}] cardinality cap leaked — saw <truncated> in metrics output"
done
ok "no <truncated> labels"

# ---------- 15. Prometheus has both targets up ----------
echo "--- 15. Prometheus: count(neo4j_up==1) == 2 ---"
prom_ok=0
for n in $(seq 1 30); do
  result=$(curl -fsS "${PROM}/api/v1/query?query=count(neo4j_up==1)" 2>/dev/null || true)
  if echo "$result" | grep -qE '"value":\[[0-9.]+,"2"\]'; then prom_ok=1; break; fi
  sleep 2
done
[[ $prom_ok -eq 1 ]] || { echo "Last response: ${result:-<empty>}"; fail "count(neo4j_up==1) != 2 within 60s"; }
ok "Prometheus count(neo4j_up==1)==2"

# ---------- 16. per-instance presence in Prometheus ----------
echo "--- 16. Prometheus: per-instance neo4j_up==1 ---"
for I in "${INSTANCES[@]}"; do
  q=$(printf 'neo4j_up{instance="%s"}' "$I" | python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.stdin.read()))' 2>/dev/null \
    || python -c 'import sys,urllib; print(urllib.quote(sys.stdin.read()))')
  result=$(curl -fsS "${PROM}/api/v1/query?query=${q}")
  echo "$result" | grep -qE '"value":\[[0-9.]+,"1"\]' \
    || { echo "$result" >&2; fail "[$I] neo4j_up not 1 in Prometheus"; }
  ok "[$I] Prometheus neo4j_up==1"
done

# ---------- 17. /slow-queries job up for both ----------
echo "--- 17. Prometheus targets: neo4j-slow-queries job up for both instances ---"
slow_targets=$(curl -fsS "${PROM}/api/v1/targets?scrapePool=neo4j-slow-queries" || true)
slow_up_count=$(echo "$slow_targets" | grep -o '"health":"up"' | wc -l | tr -d ' ')
[[ "$slow_up_count" == "2" ]] \
  || fail "neo4j-slow-queries job has $slow_up_count up targets, expected 2"
ok "neo4j-slow-queries: 2/2 targets up"

echo
echo "ALL CHECKS PASSED ✓"
echo "Visit http://localhost:3000 to view dashboards (anonymous Admin)."
