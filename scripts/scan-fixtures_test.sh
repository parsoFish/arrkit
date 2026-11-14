#!/usr/bin/env bash
# scan-fixtures_test.sh: self-check for scan-fixtures.sh. Each case builds a
# throwaway tree, runs the scanner on it and checks the exit code and output.
# Exit 0 when every case passes, 1 otherwise.
set -uo pipefail

SCAN="$(cd "$(dirname "$0")" && pwd)/scan-fixtures.sh"
WORK="$(mktemp -d)"
trap 'chmod -R u+rwX "$WORK"; rm -rf "$WORK"' EXIT
FAILS=0

# check NAME WANT_EXIT WANT_OUTPUT [ARG...]: run the scanner with ARGs and
# compare its exit code, and its combined output when WANT_OUTPUT is set.
check() {
  local name="$1" want_exit="$2" want_out="$3" out rc=0
  shift 3
  out="$("$SCAN" "$@" 2>&1)" || rc=$?
  if [ "$rc" -ne "$want_exit" ]; then
    echo "FAIL $name: exit $rc, want $want_exit; output: $out"
    FAILS=$((FAILS + 1))
  elif [ -n "$want_out" ] && [[ "$out" != *"$want_out"* ]]; then
    echo "FAIL $name: output lacks \"$want_out\": $out"
    FAILS=$((FAILS + 1))
  else
    echo "ok   $name"
  fi
}

# fixture NAME CONTENT: create $WORK/NAME/pkg/testdata/f.json holding CONTENT
# and print the tree's root.
fixture() {
  local root="$WORK/$1"
  mkdir -p "$root/pkg/testdata"
  printf '%s\n' "$2" >"$root/pkg/testdata/f.json"
  printf '%s' "$root"
}

# Built at run time so no hex run of the scanned lengths sits in this file.
HEX40="$(printf 'ab%.0s' {1..20})"
HEX32="$(printf 'cd%.0s' {1..16})"

check clean 0 "1 fixture files clean" "$(fixture clean '{"downloadId": "0000000000000000000000000000000000000001", "ip": "10.0.0.2", "email": "user1@example.com", "title": "Example.Show.S01E01.1080p-GROUP", "path": "/tv/Example Show/Season 01", "root": "/movies/#recycle"}')"
check infohash 1 "infohash: " "$(fixture infohash "{\"downloadId\": \"$HEX40\"}")"
check hex32 1 "hex32: " "$(fixture hex32 "{\"apiKey\": \"$HEX32\"}")"
check private-ip 1 "private-ip: " "$(fixture private-ip '{"host": "192.168.1.20"}')"
check email 1 "email: " "$(fixture email '{"email": "someone@mail.invalid"}')"
check home-path 1 "home-path: " "$(fixture home-path '{"path": "/home/someone/media"}')"
check release-name 1 "release-name: " "$(fixture release-name '{"title": "Some.Show.S01E01.1080p.WEB.h264-CREW"}')"
check fansub-release 1 "fansub-release: " "$(fixture fansub-release '{"title": "[SubGroup] Some Show - 01 (1080p)"}')"
check library-path 1 "library-path: " "$(fixture library-path '{"path": "/movies/Some Movie (2020)"}')"
check hit-count 1 "2 hit(s)" "$(fixture hit-count '{"a": "192.168.1.20", "b": "/tv/Some Show"}')"

mkdir -p "$WORK/empty/pkg"
check no-fixtures 2 "no files under" "$WORK/empty"
check not-a-directory 2 "not a directory" "$WORK/missing"
check too-many-args 2 "usage:" "$WORK/empty" "$WORK/empty"

if [ "$(id -u)" -ne 0 ]; then
  unreadable="$(fixture unreadable '{}')"
  chmod 000 "$unreadable/pkg/testdata/f.json"
  check unreadable-file 2 "grep failed" "$unreadable"
else
  echo "skip unreadable-file (root can read any file)"
fi

if [ "$FAILS" -ne 0 ]; then
  echo "scan-fixtures_test: $FAILS case(s) failed"
  exit 1
fi
echo "scan-fixtures_test: all cases passed"
