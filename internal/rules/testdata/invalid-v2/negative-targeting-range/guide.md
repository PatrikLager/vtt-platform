Invalid v2 fixture: a targeting atom declares a NEGATIVE range as a literal.

Distinct from invalid-v2/negative-int-binding, which trips compile.go's
int-BINDING guard. This one splices no param at all — the range is written
straight into the atom — so it reaches the targeting guard instead, which is
the one the TypeScript client's board leans on: client/src/view/player.ts has
no empty-state for its target list because a range >= 0 always includes the
acting token, and this rejection is what makes that true.
