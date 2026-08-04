# minimal-v2-zero-default-max

A resource whose `default_max_expr` evaluates to **exactly 0**, and an ability
that costs 1 of it.

This exists for one boundary in `buildResources`: `if v > 0`. A max of 0 is not
a usable fixture max, so conformance falls back to `fixtureResourceFallbackMax`
and the smoke actor can afford the ability. Under `v >= 0` the actor would be
built with a max of 0, could not pay, and the smoke pass would fail — so `Run`
succeeding here is the assertion.

Nothing about this ruleset is otherwise interesting; it is `minimal-v2` with
`default_max_expr: "0"` and `poke` made limited.
