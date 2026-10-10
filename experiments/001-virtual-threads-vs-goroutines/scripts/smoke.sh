#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXPERIMENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ $# -lt 1 || $# -gt 2 || ( "$1" != "java" && "$1" != "go" ) ]]; then
  echo "usage: $0 java|go [success|error|slow|capacity]" >&2
  exit 2
fi

service="$1"
scenario="${2:-success}"
case "$scenario" in
  success|error|slow) max_in_flight=0 ;;
  capacity) max_in_flight="${DOWNSTREAM_MAX_IN_FLIGHT:-1}" ;;
  *) echo "unsupported scenario: $scenario" >&2; exit 2 ;;
esac
case "$service" in
  java) port=8081 ;;
  go) port=8082 ;;
esac

cd "$EXPERIMENT_DIR"
cleanup() {
  docker compose down --remove-orphans >/dev/null
}
trap cleanup EXIT

DOWNSTREAM_MODE="$scenario" DOWNSTREAM_MAX_IN_FLIGHT="$max_in_flight" docker compose up -d "$service" downstream
for required_service in "$service" downstream; do
  container_id="$(docker compose ps -q "$required_service")"
  for _ in {1..30}; do
    health="$(docker inspect -f '{{.State.Health.Status}}' "$container_id" 2>/dev/null || true)"
    if [[ "$health" == "healthy" ]]; then
      break
    fi
    if [[ "$health" == "unhealthy" ]]; then
      docker compose logs "$required_service" >&2
      exit 1
    fi
    sleep 1
  done
  if [[ "$health" != "healthy" ]]; then
    docker compose logs "$required_service" >&2
    echo "$required_service did not become healthy" >&2
    exit 1
  fi
done

python3 "$SCRIPT_DIR/smoke.py" "http://127.0.0.1:${port}" "$scenario"
