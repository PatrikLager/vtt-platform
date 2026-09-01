#!/usr/bin/env bash
# The breaking gate must report-without-failing before release and fail after.
# Patrik's amendment (ADR-007, 2026-08-30): additive-only binds from the first
# release others can run, not before.
set -euo pipefail
cd "$(dirname "$0")/.."

fail() { echo "check_breaking_marker_test: $1" >&2; exit 1; }

[ -f contract/RELEASED ] && fail "contract/RELEASED exists; this test manages it and will not clobber yours"

# Pre-release: must exit 0 even when buf would object.
if ! task check:breaking >/tmp/cbm-pre.log 2>&1; then
  fail "pre-release run must not fail; see /tmp/cbm-pre.log"
fi
grep -q "pre-release" /tmp/cbm-pre.log || fail "pre-release run must SAY it is reporting rather than enforcing"

# Released: must enforce. Restore the marker's absence whatever happens.
trap 'rm -f contract/RELEASED' EXIT
: > contract/RELEASED
task check:breaking >/tmp/cbm-post.log 2>&1 || true
grep -q "pre-release" /tmp/cbm-post.log && fail "released run must not claim to be pre-release"

echo "ok  check:breaking honours contract/RELEASED in both directions"
