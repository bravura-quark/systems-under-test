#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 || ("$1" != "java" && "$1" != "go") ]]; then
  echo "usage: $0 java|go" >&2
  exit 2
fi

service=$1
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
experiment_dir=$(cd "$script_dir/.." && pwd)
profile_seconds=${PROFILE_SECONDS:-5}
rate=${RATE:-100/s}
workload_path=${WORKLOAD_PATH:-/fanout?delay=50\&branches=4}
profile_root=${PROFILE_ROOT:-$(mktemp -d /tmp/sut-profiles.XXXXXX)}
request_timeout=${REQUEST_TIMEOUT:-3s}
connections=${CONNECTIONS:-100}
target="http://${service}:8080${workload_path}"

mkdir -p "$profile_root"
chmod a+rwx "$profile_root"

cleanup() {
  docker compose down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

if [[ "$service" == "java" ]]; then
  jfr_seconds=$((profile_seconds + 15))
  PROFILE_HOST_DIR="$profile_root" \
  JAVA_JFR_OPTIONS="-XX:StartFlightRecording=filename=/profiles/java.jfr,settings=profile,duration=${jfr_seconds}s" \
    docker compose up -d java downstream >/dev/null
  diagnostics_url=""
else
  PPROF_ADDR=:6060 docker compose up -d go downstream >/dev/null
  diagnostics_url="http://127.0.0.1:6060"
fi

for required_service in "$service" downstream; do
  container_id=$(docker compose ps -q "$required_service")
  healthy=0
  for _ in $(seq 1 60); do
    health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container_id" 2>/dev/null || true)
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
    exit 1
  fi
done

printf 'GET %s\n' "$target" |
  docker compose run --rm --no-deps -T loadgen attack \
    -rate="$rate" -duration=1s -timeout="$request_timeout" -connections="$connections" >/dev/null

run_load() {
  local output=$1
  printf 'GET %s\n' "$target" |
    docker compose run --rm --no-deps -T loadgen attack \
      -rate="$rate" -duration="${profile_seconds}s" -timeout="$request_timeout" \
      -connections="$connections" > "$output"
}

if [[ "$service" == "go" ]]; then
  run_load "$profile_root/go-cpu.attack" & load_pid=$!
  curl --fail --silent "$diagnostics_url/debug/pprof/profile?seconds=$profile_seconds" > "$profile_root/go-cpu.pprof"
  wait "$load_pid"

  run_load "$profile_root/go-trace.attack" & load_pid=$!
  curl --fail --silent "$diagnostics_url/debug/pprof/trace?seconds=$profile_seconds" > "$profile_root/go-trace.out"
  wait "$load_pid"
  curl --fail --silent "$diagnostics_url/debug/pprof/heap" > "$profile_root/go-heap.pprof"
  curl --fail --silent "$diagnostics_url/debug/pprof/goroutine?debug=1" > "$profile_root/go-goroutines.txt"
else
  run_load "$profile_root/java.attack"
fi

echo "profiling artifacts written to $profile_root"
