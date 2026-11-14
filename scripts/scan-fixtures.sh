#!/usr/bin/env bash
# scan-fixtures.sh: fail when a fixture under */testdata/* carries data the
# README's fixture rule forbids. Needs bash 4.4+ (mapfile -d) and GNU grep.
#
# Usage: scripts/scan-fixtures.sh [ROOT]   (ROOT defaults to .)
# Exit:  0 clean; 1 hits found (one "rule: file:line:match" line each, on
#        stdout); 2 usage error, no fixtures found, or a grep failure.
set -euo pipefail

usage() {
  echo "usage: $0 [ROOT]" >&2
  exit 2
}
[ "$#" -le 1 ] || usage
ROOT="${1:-.}"
[ -d "$ROOT" ] || { echo "scan-fixtures: not a directory: $ROOT" >&2; exit 2; }

LIST="$(mktemp)"
trap 'rm -f "$LIST"' EXIT
find "$ROOT" -name .git -prune -o -type f -path '*/testdata/*' -print0 >"$LIST"
mapfile -d '' FILES <"$LIST"
if [ "${#FILES[@]}" -eq 0 ]; then
  echo "scan-fixtures: no files under */testdata/ in $ROOT" >&2
  exit 2
fi

HITS=0

# rule NAME PATTERN [ALLOW]: report every match of the extended regex PATTERN
# in the fixtures, except matches that also match the extended regex ALLOW.
rule() {
  local name="$1" pattern="$2" allow="${3:-}" out rc=0 line match
  # -a: a NUL byte must not turn a file into "binary file matches" and hide hits.
  out="$(grep -aHnoE -e "$pattern" -- "${FILES[@]}")" || rc=$?
  if [ "$rc" -gt 1 ]; then
    echo "scan-fixtures: grep failed for rule $name (exit $rc)" >&2
    exit 2
  fi
  [ -n "$out" ] || return 0
  while IFS= read -r line; do
    match="${line#*:*:}"
    if [ -n "$allow" ] && [[ "$match" =~ $allow ]]; then
      continue
    fi
    printf '%s: %s\n' "$name" "$line"
    HITS=$((HITS + 1))
  done <<<"$out"
}

# Real torrent infohashes (v1 40-hex, v2 64-hex). Fakes are zero-padded.
rule infohash '\b([0-9A-Fa-f]{40}|[0-9A-Fa-f]{64})\b' '^0{30}'
# Real API keys and plex.direct certificate hashes (32-hex).
rule hex32 '\b[0-9A-Fa-f]{32}\b' '^0{24}'
# Private-network addresses other than the documented 10.0.0.x.
rule private-ip '\b(10\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])|192\.168)\.[0-9]{1,3}\.[0-9]{1,3}\b' '^10\.0\.0\.'
# Emails anywhere but the example domains.
rule email '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' '@example\.(com|net|org)$'
# Home directories.
rule home-path '/(home|Users)/[A-Za-z0-9._-]+'
# Scene release names: dotted name, resolution, -GROUP.
rule release-name '[A-Za-z0-9]+(\.[A-Za-z0-9]+)*\.(480p|576p|720p|1080p|2160p)(\.[A-Za-z0-9]+)*-[A-Za-z0-9]+' '^Example\.'
# Fansub release names: [Group] title ... resolution.
rule fansub-release '\[[^]"]+\] [^"]*(480p|576p|720p|1080p|2160p)'
# Library paths: a JSON string starting /tv/ or /movies/ with a title in it.
rule library-path '"/(tv|movies)/[^"/#][^"/]*' '^"/(tv|movies)/Example( |$)'

if [ "$HITS" -gt 0 ]; then
  echo "scan-fixtures: $HITS hit(s) in ${#FILES[@]} fixture files; scrub them (README, Fixture rule)" >&2
  exit 1
fi
echo "scan-fixtures: ${#FILES[@]} fixture files clean"
