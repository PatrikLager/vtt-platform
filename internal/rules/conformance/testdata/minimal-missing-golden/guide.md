# Minimal Missing Golden

A conformance unit-test fixture: two abilities (poke, prod) that both resolve
against the generated fixture actor, but only `poke` ships a golden. Used to
prove Run enforces spec §8's per-ability golden requirement (it must fail,
naming `prod`). Not a real game system.
