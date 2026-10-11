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
workload_path=${WORKLOAD_PATH:-/io?delay=50}
downstream_url=${DOWNSTREAM_URL:-http://downstream:8080/io}
downstream_timeout_ms=${DOWNSTREAM_TIMEOUT_MS:-500}
downstream_timeout="${downstream_timeout_ms}ms"
downstream_mode=${DOWNSTREAM_MODE:-success}
downstream_max_in_flight=${DOWNSTREAM_MAX_IN_FLIGHT:-0}
fanout_branches=${FANOUT_BRANCHES:-4}
fanout_max_in_flight=${FANOUT_MAX_IN_FLIGHT:-256}
fanout_max_branches_in_flight=${FANOUT_MAX_BRANCHES_IN_FLIGHT:-256}
if [[ "$service" == "java" ]]; then
  client_variant=${CLIENT_VARIANT:-${JAVA_HTTP_CLIENT_VARIANT:-sync}}
else
  client_variant=${CLIENT_VARIANT:-go-net-http}
fi
cpu_limit=${SUT_CPUS:-1.0}
result_root=${RESULT_ROOT:-$experiment_dir/results/raw}
run_id=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$(echo "$rate-$workload_path" | tr '/?.=&' '_____')}

target="http://${service}:8080${workload_path}"
compose_services=("$service")
if [[ "$workload_path" == /downstream* || "$workload_path" == /fanout* ]]; then
  compose_services+=(downstream)
fi

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

docker compose up -d "${compose_services[@]}" >/dev/null
container_id=$(docker compose ps -q "$service")
if [[ -z "$container_id" ]]; then
  echo "could not determine container ID for $service" >&2
  exit 1
fi

for required_service in "${compose_services[@]}"; do
  required_container_id=$(docker compose ps -q "$required_service")
  healthy=0
  for _ in $(seq 1 60); do
    health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$required_container_id" 2>/dev/null || true)
    if [[ "$health" == "healthy" ]]; then
      healthy=1
      break
    fi
    if [[ "$health" == "unhealthy" ]]; then
      docker compose logs "$required_service" >&2 || true
      exit 1
    fi
    sleep 1
  done
  if [[ "$healthy" -ne 1 ]]; then
    echo "timed out waiting for $required_service health" >&2
    docker compose logs "$required_service" >&2 || true
    exit 1
  fi
done

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
  --workload "$workload_path" \
  --target "$target" \
  --rate "$rate" \
  --warmup "$warmup_duration" \
  --duration "$duration" \
  --timeout "$request_timeout" \
  --connections "$connections" \
  --container-id "$container_id" \
  --loadgen-version "${loadgen_version:-unknown}" \
  --downstream-url "$downstream_url" \
  --downstream-timeout "$downstream_timeout" \
  --downstream-mode "$downstream_mode" \
  --downstream-max-in-flight "$downstream_max_in_flight" \
  --fanout-branches "$fanout_branches" \
  --fanout-max-in-flight "$fanout_max_in_flight" \
  --fanout-max-branches-in-flight "$fanout_max_branches_in_flight" \
  --client-variant "$client_variant" \
  --cpu-limit "$cpu_limit" \
  --started-at "$started_at" \
  --ended-at "$ended_at"

echo "wrote $run_dir"
