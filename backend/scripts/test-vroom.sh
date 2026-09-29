#!/usr/bin/env bash
set -euo pipefail
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
go_bin=${GO_BIN:-go}
vroom_bin=${VROOM_BIN:-vroom}
command -v "$vroom_bin" >/dev/null 2>&1 || {
    echo 'VROOM executable not found; set VROOM_BIN or use docker compose --profile test run --rm --build backend-test' >&2
    exit 1
}
"$vroom_bin" --version
cd "$root_dir"
exec "$go_bin" test -tags vroom "$@" ./...
