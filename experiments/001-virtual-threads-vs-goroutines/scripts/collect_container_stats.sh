#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 CONTAINER_ID OUTPUT_JSONL" >&2
  exit 2
fi

container_id=$1
output_path=$2

mkdir -p "$(dirname "$output_path")"
: > "$output_path"

while true; do
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  stats=$(docker stats --no-stream --format '{{json .}}' "$container_id" 2>/dev/null || true)

  if [[ -n "$stats" ]]; then
    printf '{"timestamp":"%s","stats":%s}\n' "$timestamp" "$stats" >> "$output_path"
  elif ! docker inspect "$container_id" >/dev/null 2>&1; then
    exit 0
  fi

  sleep 1
done
