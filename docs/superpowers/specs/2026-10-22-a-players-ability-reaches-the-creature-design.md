# A player's ability reaches the creature they aimed it at

## The problem

A player who uses an ability from the browser is always refused. The player
panel, `renderPlayerPanel`'s targets section in `client/src/view/player.ts`,
offers one button per token `targetableTokens` (`client/src/player.ts`)
returns, labels it with the token's actor's name, and on a click sends
`useAbility(actorId, armed.id, [t.ID])`: the TOKEN's id. The server reads
`UseAbility.target_ids` as ACTOR ids. `rules.Resolve`
(`internal/rules/resolve.go`) looks each one up in `st.Actors` and refuses one
it does not find with `rules: resolve: unknown target actor "<id>"`, and
SPEC-018's table already says `Resolve` refuses a target that names no folded
actor. Every token and actor the shipped adventures place has two different
ids (`tok-cutter` shows `act-cutter`, `tok-hollis` shows `hollis`), so no
ability a player aims through the panel ever resolves. It was found at the
table on 2026-10-22 (finding S-1 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`): Lera's Longsword
Strike at the sleeping goblin was answered `refused: rules: resolve: unknown
target actor "tok-sleeper"`, and
the DM, through `vtt mcp` and the actor id `gob-sleeper`, landed the same
strike at once. `client/src/view/player.ts` is the only place the client sends
`use_ability` (`grep -rn "useAbility(" client/src`), and the panel is drawn for
a DM in the browser too, so a DM aiming as an actor granted to them is
refused the same way.

Three things let it through. The contract does not say which id
`target_ids` holds: `UseAbility` in `contract/vtt/v1/commands.proto` and
`AbilityUsed` in `contract/vtt/v1/events.proto` carry no comment on the field,
and `use_ability`'s tool definition in `tools/toolgen/main.go` gives
`targetIds` no description. The client test that covers the button,
`a target button sends the ability at that token, then disarms` in
`client/test/spectator-view.test.ts`, gives the target token and its actor
different ids (`t2`, `a2`) and expects `targetIds: ["t2"]`, so it pins the
defect. And no browser e2e has a player use an ability: `client/e2e/` drives
moves, grants and joins, and the e2e server already runs with
`dnd45e-minimal` and the adventures (`client/e2e/setup.ts`).

## Done looks like

1. The client test of the target button sends `targetIds` holding the actor
   id of the token clicked, with a token and an actor whose ids differ, and
   it fails on today's tree.
2. A browser e2e in `client/e2e/` has a player use an ability on a creature
   through the player panel against the shipped binary, and observes that it
   resolved (no refusal, and the creature's hp falls or the miss is recorded);
   it fails on today's tree with `unknown target actor`.
3. The contract says `target_ids` names actors, on `UseAbility` and on
   `AbilityUsed`, and `contract/gen/tools/tools.json`'s `use_ability` gives
   `targetIds` a description saying the same, so `task generate:contract`
   leaves a tree `check:drift` accepts.
4. Still refused afterwards: a `use_ability` whose target names no actor, a
   token id included (`TestResolveUnknownTarget` stays green).
5. `docs/verification-debt.md` carries the recipe: the edit that puts the
   defect back, the gate that should have caught it, and why it did not.

## Rules this puts on the system

- The player panel aims an ability at the actor the clicked token shows,
  never at the token.
- `UseAbility.target_ids` and `AbilityUsed.target_ids` name actors.

## What it touches

In this order:

1. `contract/vtt/v1/commands.proto`, `contract/vtt/v1/events.proto`,
   `tools/toolgen/main.go`, `tools/toolgen/main_test.go`, the golden
   `contract/testdata/expected_tools.json`, and what `task generate:contract`
   writes from them under `contract/gen/` and into `cmd/vtt/tools.json`.
2. `internal/rules/resolve_test.go`, and `internal/rules/resolve_v2_test.go`,
   whose `TestResolveV2WardShiftMultiTarget` gains a citation line.
3. The contract's examples, `contract/testdata/use_ability_command.json` and
   `contract/testdata/ability_used_envelope.json`, which name tokens in these
   fields, and `client/test/commands.test.ts`, which reads them.
4. `client/src/view/player.ts`, `client/test/spectator-view.test.ts`, and the
   committed bundle `cmd/vtt/webdist/` from `task build:client`.
5. `client/e2e/table.spec.ts`, and `client/e2e/setup.ts` if the fixture needs
   it.
6. `docs/verification-debt.md`, `docs/requirements.md`.

## Specifications this moves

None.

## What could not be established

- Whether `task build:client` leaves `cmd/vtt/webdist/` byte-identical after
  a change to a generated TypeScript comment alone; the client change in
  step 4 rebuilds it either way.
- The server does not check that the player's character sees the target:
  `use_ability`'s authorization (`authorizeActorOwnership` in
  `internal/gateway/authz.go`) checks only that the player controls the
  caster, and `Resolve` the range. That is outside this ticket. It is
  finding A-10 in `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`,
  whose order of work gives it to the fifth ticket there, with A-6.
