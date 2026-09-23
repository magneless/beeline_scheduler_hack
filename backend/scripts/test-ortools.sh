#!/usr/bin/env bash
set -euo pipefail
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
"$root_dir/scripts/setup-ortools.sh"
go_bin=${GO_BIN:-}
if [[ -z "$go_bin" ]]; then
 if command -v go >/dev/null 2>&1 && [[ "$(go version | awk '{print $3}')" == go1.27* ]]; then go_bin=$(command -v go)
 elif [[ -x /usr/local/go/bin/go ]]; then go_bin=/usr/local/go/bin/go
 else go_bin=$(command -v go || true); fi
fi
[[ -n "$go_bin" && -x "$go_bin" ]] || { echo 'Go executable not found' >&2; exit 1; }
lib_dir="$root_dir/.cache/ortools/lib"; export LD_LIBRARY_PATH="$lib_dir${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"; export CGO_LDFLAGS="-L$lib_dir -Wl,-rpath,$lib_dir${CGO_LDFLAGS:+ $CGO_LDFLAGS}"
cd "$root_dir"; exec "$go_bin" test -tags ortools ./... "$@"
