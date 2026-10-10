#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
experiment_dir=$(cd "$script_dir/.." && pwd)

rates=${RATES:-10/s,100/s,500/s,1000/s,2000/s}
workloads=${WORKLOAD_PATHS:-/io?delay=50,/downstream?delay=50}
warmup_duration=${WARMUP_DURATION:-2s}
duration=${DURATION:-5s}
request_timeout=${REQUEST_TIMEOUT:-3s}
connections=${CONNECTIONS:-100}
repetitions=${REPETITIONS:-1}
result_root=${RESULT_ROOT:-$experiment_dir/results/raw}
downstream_url=${DOWNSTREAM_URL:-http://downstream:8080/io}
downstream_timeout_ms=${DOWNSTREAM_TIMEOUT_MS:-500}
downstream_timeout="${downstream_timeout_ms}ms"

IFS=',' read -r -a rate_values <<< "$rates"
IFS=',' read -r -a workload_values <<< "$workloads"

for repetition in $(seq 1 "$repetitions"); do
  for workload_path in "${workload_values[@]}"; do
    for rate in "${rate_values[@]}"; do
      safe_workload=$(echo "$workload_path" | tr '/?.=&' '_____')
      safe_rate=$(echo "$rate" | tr '/.' '__')
      run_id="$(date -u +%Y%m%dT%H%M%SZ)-${safe_workload}-${safe_rate}-rep${repetition}"
      for service in java go; do
        echo "running service=$service workload=$workload_path rate=$rate repetition=$repetition"
        RATE="$rate" \
        WORKLOAD_PATH="$workload_path" \
        DOWNSTREAM_URL="$downstream_url" \
        DOWNSTREAM_TIMEOUT_MS="$downstream_timeout_ms" \
        WARMUP_DURATION="$warmup_duration" \
        DURATION="$duration" \
        REQUEST_TIMEOUT="$request_timeout" \
        CONNECTIONS="$connections" \
        RESULT_ROOT="$result_root" \
        RUN_ID="$run_id" \
          "$script_dir/benchmark_io.sh" "$service"

        service_dir="$result_root/$run_id/$service"
        python3 "$experiment_dir/analysis/summarize_run.py" \
          --run-dir "$service_dir" \
          --output "$service_dir/summary.json"
      done
    done
  done
done
