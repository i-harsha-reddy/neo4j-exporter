#!/usr/bin/env bash
# loadgen runner — schedules cypher scenarios against multiple Neo4j targets.
#
#   run.sh                              # daemon: dispatch scenarios on a schedule
#   run.sh --once SCENARIO INSTANCE     # one-shot for e2e determinism
#
# Env vars (defaults shown):
#   LOADGEN_TARGETS=a=neo4j-a:7687,b=neo4j-b:7687
#   LOADGEN_USERNAME=neo4j
#   LOADGEN_PASSWORD=dev_password_change_me
#   LOADGEN_RATE_HOT_WRITES=4   LOADGEN_CONTENTION_S=5
#   LOADGEN_SLOW_INTERVAL_S=30  LOADGEN_INDEX_CHURN_S=90
#   LOADGEN_APOC_BATCH_S=60     LOADGEN_ROLLBACK_S=20
#   LOADGEN_SCHEMA_DIVERSITY_S=300
#   LOADGEN_DISABLE=            (comma-separated scenario names)
#   LOADGEN_LOG=info            (quiet | info | debug)

set -uo pipefail

LOADGEN_TARGETS="${LOADGEN_TARGETS:-a=neo4j-a:7687,b=neo4j-b:7687}"
LOADGEN_USERNAME="${LOADGEN_USERNAME:-neo4j}"
LOADGEN_PASSWORD="${LOADGEN_PASSWORD:-dev_password_change_me}"
LOADGEN_RATE_HOT_WRITES="${LOADGEN_RATE_HOT_WRITES:-4}"
LOADGEN_CONTENTION_S="${LOADGEN_CONTENTION_S:-5}"
LOADGEN_SLOW_INTERVAL_S="${LOADGEN_SLOW_INTERVAL_S:-30}"
LOADGEN_INDEX_CHURN_S="${LOADGEN_INDEX_CHURN_S:-90}"
LOADGEN_APOC_BATCH_S="${LOADGEN_APOC_BATCH_S:-60}"
LOADGEN_ROLLBACK_S="${LOADGEN_ROLLBACK_S:-20}"
LOADGEN_SCHEMA_DIVERSITY_S="${LOADGEN_SCHEMA_DIVERSITY_S:-300}"
LOADGEN_DISABLE="${LOADGEN_DISABLE:-}"
LOADGEN_LOG="${LOADGEN_LOG:-info}"

SCENARIOS_DIR="$(cd "$(dirname "$0")" && pwd)/scenarios"

log() {
  local level="$1"; shift
  case "$LOADGEN_LOG/$level" in
    quiet/*)    return 0 ;;
    info/debug) return 0 ;;
  esac
  printf '[%s] %s\n' "$(date -u +%FT%TZ)" "$*"
}

is_disabled() {
  case ",$LOADGEN_DISABLE," in
    *,"$1",*) return 0 ;;
    *)        return 1 ;;
  esac
}

NAMES=()
HOSTS=()
parse_targets() {
  local IFS=','
  for pair in $LOADGEN_TARGETS; do
    [[ -z "$pair" ]] && continue
    NAMES+=("${pair%%=*}")
    HOSTS+=("${pair#*=}")
  done
}

run_cypher() {
  local target="$1" file="$2"
  cypher-shell \
    --address "bolt://$target" \
    --username "$LOADGEN_USERNAME" \
    --password "$LOADGEN_PASSWORD" \
    --format plain \
    --non-interactive \
    -f "$SCENARIOS_DIR/$file"
}

fire_scenario() {
  local scenario="$1" name="$2" host="$3"
  local file="$scenario.cypher"
  if [[ ! -f "$SCENARIOS_DIR/$file" ]]; then
    log error "[$name] $scenario: file not found at $SCENARIOS_DIR/$file"
    return 1
  fi
  case "$scenario" in
    contention)
      for w in 1 2 3 4; do
        run_cypher "$host" "$file" >/dev/null 2>&1 &
      done
      wait
      log info "[$name] $scenario ok (4 concurrent)"
      ;;
    rollbacks)
      # Expected to exit non-zero on Neo.ClientError.Schema.ConstraintValidationFailed.
      if run_cypher "$host" "$file" >/dev/null 2>&1; then
        log info "[$name] $scenario unexpectedly succeeded"
      else
        log info "[$name] $scenario rolled back"
      fi
      ;;
    *)
      local start_ms end_ms
      start_ms=$(date +%s%3N 2>/dev/null || date +%s)
      if run_cypher "$host" "$file" >/dev/null 2>&1; then
        end_ms=$(date +%s%3N 2>/dev/null || date +%s)
        log info "[$name] $scenario ok ($((end_ms - start_ms))ms)"
      else
        log info "[$name] $scenario FAILED"
      fi
      ;;
  esac
}

wait_for_target() {
  local name="$1" host="$2"
  local tries=120
  while ! cypher-shell \
    --address "bolt://$host" \
    --username "$LOADGEN_USERNAME" \
    --password "$LOADGEN_PASSWORD" \
    --non-interactive 'RETURN 1' >/dev/null 2>&1; do
    tries=$((tries - 1))
    if [[ $tries -le 0 ]]; then
      log error "[$name] never became ready"
      return 1
    fi
    sleep 2
  done
  log info "[$name] ready"
}

dispatch_loop() {
  local scenario="$1" interval="$2"
  shift 2
  local indices=("$@")
  while true; do
    for idx in "${indices[@]}"; do
      fire_scenario "$scenario" "${NAMES[$idx]}" "${HOSTS[$idx]}"
    done
    sleep "$interval"
  done
}

main() {
  parse_targets
  if [[ ${#NAMES[@]} -eq 0 ]]; then
    log error "no targets parsed from LOADGEN_TARGETS=$LOADGEN_TARGETS"
    exit 1
  fi

  log info "starting loadgen with ${#NAMES[@]} targets: ${NAMES[*]}"

  for i in "${!NAMES[@]}"; do
    wait_for_target "${NAMES[$i]}" "${HOSTS[$i]}" || exit 1
  done

  if ! is_disabled bootstrap; then
    for i in "${!NAMES[@]}"; do
      log info "[${NAMES[$i]}] bootstrap starting (~30s, parallel)"
      fire_scenario bootstrap "${NAMES[$i]}" "${HOSTS[$i]}" &
    done
    wait
    log info "all bootstraps complete"
  fi

  is_disabled hot_writes        || dispatch_loop hot_writes        "$LOADGEN_RATE_HOT_WRITES"     "${!NAMES[@]}" &
  is_disabled contention        || dispatch_loop contention        "$LOADGEN_CONTENTION_S"        "${!NAMES[@]}" &
  is_disabled slow_traversals   || dispatch_loop slow_traversals   "$LOADGEN_SLOW_INTERVAL_S"     "${!NAMES[@]}" &
  is_disabled index_churn       || dispatch_loop index_churn       "$LOADGEN_INDEX_CHURN_S"       "${!NAMES[@]}" &
  is_disabled apoc_batch        || dispatch_loop apoc_batch        "$LOADGEN_APOC_BATCH_S"        "${!NAMES[@]}" &
  is_disabled rollbacks         || dispatch_loop rollbacks         "$LOADGEN_ROLLBACK_S"          "${!NAMES[@]}" &
  is_disabled schema_diversity  || dispatch_loop schema_diversity  "$LOADGEN_SCHEMA_DIVERSITY_S"  0 &

  trap 'log info "shutting down"; kill 0' TERM INT
  wait
}

once() {
  local scenario="${1:-}"
  local target_query="${2:-}"
  if [[ -z "$scenario" || -z "$target_query" ]]; then
    echo "usage: run.sh --once SCENARIO INSTANCE_NAME_OR_ADDRESS" >&2
    exit 2
  fi
  parse_targets
  for i in "${!NAMES[@]}"; do
    if [[ "${NAMES[$i]}" == "$target_query" || "${HOSTS[$i]}" == "$target_query" ]]; then
      wait_for_target "${NAMES[$i]}" "${HOSTS[$i]}" || exit 1
      fire_scenario "$scenario" "${NAMES[$i]}" "${HOSTS[$i]}"
      return $?
    fi
  done
  echo "no target matched: $target_query (have names: ${NAMES[*]} hosts: ${HOSTS[*]})" >&2
  exit 2
}

case "${1:-}" in
  --once) shift; once "$@" ;;
  --help|-h)
    sed -n '1,/^$/p' "$0" | sed 's/^# \{0,1\}//'
    ;;
  *) main ;;
esac
