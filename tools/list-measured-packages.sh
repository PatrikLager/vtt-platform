#!/usr/bin/env bash
# List every package that MUST appear in the coverage profile: one import path
# per line, packages with non-test Go files only.
#
# Why this exists (it was deleted once and had to come back):
# `go test -cover -args -test.gocoverdir=$D` only produces covdata from test
# binaries that actually RUN. A package with no _test.go files has no binary,
# so it emits nothing at all — where the older `-coverprofile` invocation
# listed it at 0%. Without this list such a package is invisible to the gate:
# it is not in `measured`, so the missing-floor fatal cannot see it, and not in
# `thresholds`, so the stale-key fatal cannot either. The two-direction set
# equality is between thresholds and what was MEASURED; it cannot detect
# something never measured at all.
#
# That is the worst possible blind spot for this gate: a package landing before
# its tests is precisely what ADR-009 rule 1 forbids and what the gate exists
# to catch when discipline slips.
#
# It lives in a file rather than inline in Taskfile.yml because Task renders
# double-brace sequences as its OWN Go template, so a `go list -f` format
# string never survives to reach go list. Piping `go list -json` through a
# filter avoids braces in the command line entirely.
set -euo pipefail

go list -json ./... | python3 -c '
import json, sys

EXCLUDE = (
    "github.com/PatrikLager/vtt-platform/contract/gen/",     # generated code
    "github.com/PatrikLager/vtt-platform/contract-spike/",   # frozen ADR-007 evidence
)

# go list -json emits a stream of concatenated objects, not an array.
decoder = json.JSONDecoder()
buf = sys.stdin.read()
idx = 0
while idx < len(buf):
    while idx < len(buf) and buf[idx].isspace():
        idx += 1
    if idx >= len(buf):
        break
    obj, idx = decoder.raw_decode(buf, idx)
    path = obj.get("ImportPath", "")
    # GoFiles excludes _test.go, so a test-only package (./contract holds just
    # roundtrip_test.go) is correctly omitted: it has no statements and never
    # appears in a profile.
    if obj.get("GoFiles") and not path.startswith(EXCLUDE):
        print(path)
'
