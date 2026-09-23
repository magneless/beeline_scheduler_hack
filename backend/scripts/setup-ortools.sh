#!/usr/bin/env bash
set -euo pipefail
[[ "$(uname -s)" == Linux ]] || { echo 'OR-Tools runtime script supports Linux only' >&2; exit 1; }
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cache_dir="$root_dir/.cache/ortools"
version="v9.15-go1.26.5"; release_version="9.15.6826"
mkdir -p "$cache_dir"
case "$(uname -m)" in
 x86_64|amd64) asset="or-tools_x86_64_AlmaLinux-8.10_go_v${release_version}.tar.gz"; sha="647020233b9a81703a818c0077c4295fc8ecbf6ebd895d21965863eeef30e130";;
 aarch64|arm64) asset="or-tools_aarch64_AlmaLinux-8.10_go_v${release_version}.tar.gz"; sha="8221b76fcd7d5c3165b18e3f96a26e18a17e531d58b2f3b95c00bddbcdb38e3d";;
 *) echo "unsupported Linux architecture: $(uname -m)" >&2; exit 1;;
esac
archive="$cache_dir/$asset"; url="https://github.com/AirspaceTechnologies/or-tools/releases/download/$version/$asset"
stamp="$cache_dir/.installed-$version-$asset"
if [[ -f "$stamp" && -f "$cache_dir/lib/libgoortools.so" && -f "$cache_dir/lib/libortools.so.$release_version" ]]; then exit 0; fi
part="$archive.part"
curl -fsSL --retry 3 --output "$part" "$url"
printf '%s  %s\n' "$sha" "$part" | sha256sum --check --status
mv "$part" "$archive"
tmp="$cache_dir/lib.part.$$"; rm -rf "$tmp"; mkdir -p "$tmp"
tar -xzf "$archive" -C "$tmp" --strip-components=1
[[ -f "$tmp/libgoortools.so" && -f "$tmp/libortools.so.$release_version" ]]
rm -rf "$cache_dir/lib"; mv "$tmp" "$cache_dir/lib"; touch "$stamp"
printf 'OR-Tools %s installed in %s\n' "$version" "$cache_dir/lib"
