# resolution-key-consumed-at-targeting

`strike-roll` provides `strike` from a **resolution** contribution
(`phaseResolution`); `reach`, which carries the composition's single
**targeting** contribution, consumes it (`phaseTargeting`). The provider runs
after the consumer, so the declared dependency cannot be honoured.

Sibling of `cross-phase-edge-inverted`, which inverts the edge with an
`always`-outcome provider. This one inverts it from the *resolution* side, and
that is the difference: `producePhaseForKey` returns `phaseResolution` **only**
via its `c.Kind == "resolution" && c.Key == key` early return. Break either half
of that condition and the function falls through to the outcome loop, which for
this atom finds nothing and yields `phaseTargeting` — so `pp > cp` becomes
`0 > 0`, the rejection stops firing, and an unhonourable composition loads
clean. Both mutants on that line survived every other fixture.

No cycle: `strike-roll` consumes nothing, so the edge runs one way only.
