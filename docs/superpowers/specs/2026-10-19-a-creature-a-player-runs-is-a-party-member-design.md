# A creature a player runs is granted as a party member, and the agent is told so

## The problem

The owner ruled on 2026-10-19 that a creature a player runs for a while,
summoned or possessed, is a party member while they run it, "with everything
that implies", and that it needs no kind of its own. It is granted
`ACTOR_KIND_PARTY_MEMBER`, and `ACTOR_KIND_NON_PARTY` or removed when that
ends. Today's code already gives such a creature everything a party member has:

- the fan-out writes it a log of its own (SPEC-021);
- `perceive` puts it on every log's roster and looks through its eyes
  (SPEC-016);
- the player may `set_viewpoint` on it (`MayWatch`), it may be their default
  (`defaultViewpoint`), and while they watch it the move gate asks its eyes
  (`judgedBy`, `canSee`; SPEC-015);
- granted another kind or removed, its log pauses (VTT-316, VTT-318).

The owner accepted two consequences with the ruling. A creature possessed a
second time resumes its old log. A creature added under an id an earlier actor
had continues that actor's log (VTT-318,
`TestANewActorUnderAnOldIdContinuesItsLog`). He also ruled that the kind stays
the grant's to set and never follows control: "det ska fungera som idag".

The agent is told the opposite. `tools/toolgen/main.go`, generated into
`cmd/vtt/tools.json`, describes `grant_actor_control`'s `kind` as
`ACTOR_KIND_NON_PARTY` "when you are handing out a monster, NPC or creature
for someone to operate", and `add_actor`'s `kind` says "a charmed monster is
still a monster". An agent that follows them grants a summoned wolf
`ACTOR_KIND_NON_PARTY`. Then the wolf has no log, `MayWatch` refuses a
`set_viewpoint` on it, and the move gate in `handleCommand` judges a move of
its token by the eyes of the character the player watches. A player who
watches nobody has every such move refused (VTT-370). The player runs a
creature they cannot look through.

Nothing tells the agent that an id is a log's name. `add_actor`'s `actorId`
says "unique in the campaign", and `engine.Apply` refuses an `ActorAdded`
only for an id the world holds now, so an id freed by a removal can be used
again, and the new actor continues the old one's log.

`contract/vtt/v1/events.proto`'s comment on `ActorKind` gives "a charmed
monster handed to a player was still a monster" as one of the two cases the
rule it replaced got backwards, and names "a familiar, a summon" as a
foreseeable third kind. Its argument, that control does not decide what an
actor is, still holds. The example and the third kind now say the opposite of
the ruling.

The sentences are older than this ruling, and two of them are older than the
one before it. `80dfa0e` wrote "a charmed monster is still a monster" and "a
familiar, a summon" on 2026-08-23. The same day the owner revised the
visibility ticket's §5.1 so that a charmed monster "becomes a player's to run
and then becomes a monster again" (`ActorControlGranted.kind`'s comment says
so), and `42f7af9` put the kind on the grant. That commit wrote the grant's
whole `kind` description: "for someone to operate" beside "grant again with
the other value when a monster is charmed into the party's service". Since
then the grant's description has said both, and `add_actor`'s has kept the
first draft's words. `TestTheGrantDeclaresTheActorsKind` holds the revised
case: a monster charmed into a player's hands ends a party member.

How MapTool solves this, per CLAUDE.md rule 9. `Token.Type` has two values,
`PC` and `NPC`, which the GM sets on any token at any time, and no third for a
summon. `ZoneViewModel.makePlayerView` gives a player a view through the
selected tokens they own that have sight. With none selected, it uses every
owned token with sight only when the server uses individual views
(`StartServerDialogPreferences.getUseIndividualViews`, off by default) or is
personal, and otherwise `Zone.getPlayerTokensWithSight`, the `PC` tokens
alone. So under MapTool's defaults a summoned creature is looked through as a
character once the GM makes it a `PC`: its standing is the GM's to set, which
is the outcome the ruling gives here through `ACTOR_KIND_PARTY_MEMBER` on a
grant. The union of views, sight by ownership and the filter in the client
are not borrowed (rule 9).

Not part of this ticket:

- the client's grant control in `client/src/view/dm.ts`, which already offers
  both kinds to a human DM;
- a test of whether an agent follows a description, which no gate here runs;
- what another character's log is told when a party member it does not see
  leaves the party or is removed. A viewer keeps the kind and the actor it
  last saw (SPEC-016), so a summoned creature that leaves out of a
  character's sight stays on that character's roster. The owner ruled on
  2026-10-19 to leave it so.

## Done looks like

1. A new test in `tools/toolgen/main_test.go` shows that
   `grant_actor_control`'s `kind` description tells the agent to grant
   `ACTOR_KIND_PARTY_MEMBER` for a creature a player is to run for a while,
   summoned, possessed or charmed into their service, and, when that ends, to
   grant it
   `ACTOR_KIND_NON_PARTY` and revoke the player's control, or remove the
   actor. It also says that revoking alone leaves the creature a party
   member, and it no longer offers `ACTOR_KIND_NON_PARTY` for a creature
   handed to someone to operate. It fails today.
2. A new test in `tools/toolgen/main_test.go` shows that `add_actor`'s `kind`
   description no longer calls a charmed monster still a monster and says
   that the grant makes a creature a player is to run a party member,
   whatever it was added as. It also shows that `add_actor`'s `actorId`
   description tells the agent to give each new creature an id no actor has
   had, since a reused id continues the earlier actor's log. It fails today.
3. `contract/gen/tools/tools.json` and `cmd/vtt/tools.json` are regenerated
   and `contract/testdata/expected_tools.json` matches them byte for byte.
   `TestToolsMatchGolden` and `TestRunOutputMatchesCommittedToolsJSON` pass,
   and `task check:drift` is green. The two sentences in
   `tools/toolgen/main_test.go` that the new text makes false, both in
   `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`, are
   rewritten.
4. `ActorKind`'s comment in `contract/vtt/v1/events.proto` no longer gives a
   creature handed to a player as one that stays a monster, no longer names
   a familiar or a summon as a third kind, and keeps its argument that
   control does not decide kind. `contract/gen/` is regenerated, and
   `task check:breaking` and `task check:drift` report nothing new.
5. A new test in `internal/gateway` walks the ruling's case on today's code.
   A player who controls a party member is granted a placed creature as
   `ACTOR_KIND_PARTY_MEMBER`.
   - Their `set_viewpoint` on it is accepted, and they are sent its log from
     its opening.
   - A move of its token onto a square it sees is accepted, and one onto a
     square it cannot see is refused.
   - The DM ends the run by granting it `ACTOR_KIND_NON_PARTY`, and their
     connection, which was watching it, moves back to their party member.
     Watching another character changes no kind: only the grant does.
   - When it is granted `ACTOR_KIND_PARTY_MEMBER` again, a `set_viewpoint` on
     it sends the same log, numbered on from where it paused, with no second
     opening.

   It PASSES on today's tree, by design. The ruling is that no code is needed,
   and this test is what shows it. Its deliberate breaks are the plan's.
6. SPEC-021 records the ruling, in the specification step.
7. `task check` is green on the last code commit.

## Rules this puts on the system

- The agent's description of `grant_actor_control`'s `kind` tells it to grant
  a creature a player is to run as a party member, and, when that ends, to
  grant it as non-party and revoke the player, or to remove it.
- The agent's description of `add_actor`'s `actorId` tells it never to reuse
  the id of an actor the campaign has had.

The first rule's run-end steps rely on two behaviours of today's fold that no
row stated: a grant restates the kind even for a participant who already
controls the actor, and a revocation leaves a party member a party member.
The reading review of this ticket found them, and they became VTT-391 and
VTT-392, held by `internal/engine`'s existing tests.

The walk in item 5 adds no rule. VTT-316, VTT-317, VTT-318, VTT-340,
VTT-343 and VTT-350 already state what it observes, and the test cites them.

## What it touches

1. the agent's tools: `tools/toolgen/main.go` and `tools/toolgen/main_test.go`,
   then the generated `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`,
   and `contract/testdata/expected_tools.json`, which no task names and is
   edited by hand to match;
2. the contract's comment: `contract/vtt/v1/events.proto`, then the
   generated `contract/gen/`. `a0c48c9` edited only comments in
   `events.proto`, and of the generated files it moved `events.pb.go` and
   `events_pb.ts` and no other; `cmd/vtt/webdist/` holds none of the
   comments' text;
3. a new test file in `internal/gateway`;
4. `docs/requirements.md`, `internal/engine/actor_control_test.go`, whose two
   tests carry VTT-391's and VTT-392's ids, and `docs/verification-debt.md`
   for the recipe of the descriptions' escape.

That is three components. The owner chose one ticket for this work on
2026-10-19, when it was larger.

## Specifications this moves

docs/specifications/021-each-party-member-keeps-its-own-log.md

## What could not be established

- Whether an agent ends a run by both calls the description names. Nothing
  refuses a creature left non-party in a player's control; that player moves
  it under their own character's eyes, as today.
