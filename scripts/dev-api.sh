#!/usr/bin/env bash
set -euo pipefail

DB="${DB:-./data/loinc.sqlite}"
ADDR="${ADDR:-:9005}"
POLL_INTERVAL="${POLL_INTERVAL:-1}"

pid=""
last_signature=""

cleanup() {
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

signature() {
  {
    find cmd internal web -type f \( -name '*.go' -o -name 'assets.go' \) -print0
    printf '%s\0' go.mod go.sum
  } | xargs -0 stat -f '%N:%m' 2>/dev/null | sort | shasum
}

start_server() {
  cleanup
  echo "Starting Go API on ${ADDR} with DB=${DB}"
  go run ./cmd/loinc-browser serve --db "${DB}" --addr "${ADDR}" &
  pid="$!"
}

last_signature="$(signature)"
start_server

while true; do
  sleep "${POLL_INTERVAL}"
  next_signature="$(signature)"
  if [[ "${next_signature}" != "${last_signature}" ]]; then
    last_signature="${next_signature}"
    echo "Go source changed; restarting API..."
    start_server
  fi
done
