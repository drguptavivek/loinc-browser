#!/usr/bin/env bash
# Run every single-line `curl` example in docs/USE_CASES.md against a running server and fail
# on any non-2xx status. Unix-socket examples are skipped when the socket file is absent.
# Usage: scripts/check-use-cases.sh [base-url]   (default http://localhost:9005)
set -uo pipefail
cd "$(dirname "$0")/.."
BASE="${1:-http://localhost:9005}"
BASE="${BASE%/}"
fail=0
while IFS= read -r line; do
  if [[ "$line" == *"--unix-socket "* ]]; then
    sock="$(sed -E 's/.*--unix-socket ([^ ]+).*/\1/' <<<"$line")"
    [[ -S "$sock" ]] || { echo "SKIP (no socket $sock) $line"; continue; }
  fi
  cmd="${line//http:\/\/localhost:9005/$BASE}"
  # -f: non-2xx is an error; -w prints the status for the report.
  status="$(bash -c "${cmd/curl /curl -sS -f -o /dev/null -w '%{http_code}' }" 2>&1)"
  if [[ $? -eq 0 ]]; then echo "ok   $status $line"; else echo "FAIL $status $line"; fail=1; fi
done < <(grep -E '^curl ' docs/USE_CASES.md)
exit "$fail"
