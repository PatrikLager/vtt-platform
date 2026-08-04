# placement-actor-id-empty

Copy of `testdata/valid` with exactly one field emptied, to pin the corresponding `must not be empty` rule in load.go. Those four error paths had NO test at all until 2026-08-04 — gremlins reported them NOT COVERED, a category check-mutation.py does not fail on, so nothing was ever going to say so.
