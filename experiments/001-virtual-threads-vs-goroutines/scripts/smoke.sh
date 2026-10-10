#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXPERIMENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ $# -ne 1 || ( "$1" != "java" && "$1" != "go" ) ]]; then
  echo "usage: $0 java|go" >&2
  exit 2
fi

service="$1"
case "$service" in
  java) port=8081 ;;
  go) port=8082 ;;
esac

cd "$EXPERIMENT_DIR"
cleanup() {
  docker compose down --remove-orphans >/dev/null
}
trap cleanup EXIT

docker compose up -d "$service"
container_id="$(docker compose ps -q "$service")"
for _ in {1..30}; do
  health="$(docker inspect -f '{{.State.Health.Status}}' "$container_id" 2>/dev/null || true)"
  if [[ "$health" == "healthy" ]]; then
    python3 "$SCRIPT_DIR/smoke.py" "http://127.0.0.1:${port}"
    exit 0
  fi
  if [[ "$health" == "unhealthy" ]]; then
    docker compose logs "$service" >&2
    exit 1
  fi
  sleep 1
done

docker compose logs "$service" >&2
echo "service did not become healthy" >&2
exit 1
