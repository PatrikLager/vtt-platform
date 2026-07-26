Invalid v2 fixture (spec §4 execution-order clause; finding F3): the
`primer` atom is an unconditional ("always") outcome that PROVIDES key
"primer" (a +5 pool gain that runs in the effects phase), and `spender` is
a "hit"-branch outcome that CONSUMES "primer" (a pool drain that runs in the
earlier branch phase). The provides/consumes graph declares primer-before-
spender, and the topo sort honors it, but Resolve's fixed branch-then-
effects execution order runs spender BEFORE primer — the declared
dependency is silently inverted at runtime. buildAndSortDAG must reject this
unhonorable cross-phase edge at load, the same way it rejects a cycle.
