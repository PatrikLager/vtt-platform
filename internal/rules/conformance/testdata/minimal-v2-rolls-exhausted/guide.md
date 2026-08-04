# minimal-v2-rolls-exhausted

A golden that records **no rolls** for an ability that rolls.

This exists for `tableRoller.Roll`'s exhaustion guard, `if r.i >= len(r.steps)`.
A golden with too few recorded steps is a broken fixture, not a platform bug, so
the roller returns a zero-valued roll and the mismatch surfaces as an ordinary
conformance failure naming the golden. Here the zero roll is RECORDED as the
expected outcome (total 3 vs def 10, a miss), so `Run` succeeds.

Under `r.i > len(r.steps)` the guard misses the exact-exhaustion case by one and
indexes past the end of the slice, panicking. `Run` succeeding here is what
separates the two.
