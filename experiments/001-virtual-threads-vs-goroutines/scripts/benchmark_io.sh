#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 || ("$1" != "java" && "$1" != "go") ]]; then
  echo "usage: $0 java|go" >&2
  exit 2
fi

service=$1
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
experiment_dir=$(cd "$script_dir/.." && pwd)

rate=${RATE:-10/s}
warmup_duration=${WARMUP_DURATION:-2s}
duration=${DURATION:-5s}
request_timeout=${REQUEST_TIMEOUT:-3s}
connections=${CONNECTIONS:-100}
result_root=${RESULT_ROOT:-$experiment_dir/results/raw}
run_id=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$(echo "$rate" | tr '/.' '__')}

case "$service" in
  java) target=http://java:8080/io?delay=50 ;;
  go) target=http://go:8080/io?delay=50 ;;
esac

run_dir="$result_root/$run_id/$service"
attack_path="$run_dir/attack.bin"
report_path="$run_dir/report.json"
stats_path="$run_dir/container-stats.jsonl"
metadata_path="$run_dir/run.json"

mkdir -p "$run_dir"

stats_pid=""
container_id=""
started_at=""

cleanup() {
  if [[ -n "$stats_pid" ]]; then
    kill "$stats_pid" 2>/dev/null || true
    wait "$stats_pid" 2>/dev/null || true
  fi
  docker compose down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker compose up -d "$service" >/dev/null
container_id=$(docker compose ps -q "$service")
if [[ -z "$container_id" ]]; then
  echo "could not determine container ID for $service" >&2
  exit 1
fi

healthy=0
for _ in $(seq 1 60); do
  health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container_id" 2>/dev/null || true)
  if [[ "$health" == "healthy" ]]; then
    healthy=1
    break
  fi
  if [[ "$health" == "unhealthy" ]]; then
    docker compose logs "$service" >&2 || true
    exit 1
  fi
  sleep 1
done
if [[ "$healthy" -ne 1 ]]; then
  echo "timed out waiting for $service health" >&2
  docker compose logs "$service" >&2 || true
  exit 1
fi

printf 'GET %s\n' "$target" |
  docker compose run --rm --no-deps -T loadgen attack \
    -rate="$rate" -duration="$warmup_duration" -timeout="$request_timeout" \
    -connections="$connections" >/dev/null

started_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
"$script_dir/collect_container_stats.sh" "$container_id" "$stats_path" &
stats_pid=$!

printf 'GET %s\n' "$target" |
  docker compose run --rm --no-deps -T loadgen attack \
    -rate="$rate" -duration="$duration" -timeout="$request_timeout" \
    -connections="$connections" > "$attack_path"

kill "$stats_pid" 2>/dev/null || true
wait "$stats_pid" 2>/dev/null || true
stats_pid=""

docker compose run --rm --no-deps -T loadgen report -type=json < "$attack_path" > "$report_path"
ended_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
loadgen_version=$(docker compose run --rm --no-deps -T loadgen -version | sed -n 's/^Version: //p')

python3 "$script_dir/write_run_metadata.py" \
  --output "$metadata_path" \
  --service "$service" \
  --target "$target" \
  --rate "$rate" \
  --warmup "$warmup_duration" \
  --duration "$duration" \
  --timeout "$request_timeout" \
  --connections "$connections" \
  --container-id "$container_id" \
  --loadgen-version "${loadgen_version:-unknown}" \
  --started-at "$started_at" \
  --ended-at "$ended_at"

echo "wrote $run_dir"
