# A player's or a spectator's connection streams one character's log

## The problem

A player's or a spectator's connection is fed the world log and sent a
filter's verdict on it. `serve` in `internal/gateway/server.go` builds a
`seat` (`newSeat`, `internal/gateway/seat.go`). For every role but the DM
and the agent (`projected`), the seat holds a `Projector`
(`internal/gateway/project.go`) and subscribes to the world from sequence 0
(`subscribeFrom`), whatever cursor the client asked for. `receive` folds the
whole prefix received so far with `campaign.FoldPrefix`, and `Project` folds
every frame it has sent, so each event costs two folds per connection.
`pastResume` drops output at or below the cursor, and a perch is applied by
`reperch` as frames of sequence 0. So every world event, the GM's secrets
included, reaches each player's connection, and only the `Projector`'s
verdict stands between it and the socket.

Meanwhile each party member's experience is already a log of its own,
written when the event is appended (SPEC-021). Since the first half of this
ticket's pair
(`docs/superpowers/specs/2026-10-16-a-character-log-has-its-own-subscription-design.md`)
it can be followed from its own cursor (`Campaign.SubscribeCharacter`). No
connection reads or follows one: `grep -rn
'CharacterLog(\|ReadCharacter\|SubscribeCharacter' --include='*.go' . | grep
-v _test.go` prints only `internal/campaign` and `internal/store`. The
fan-out's decision therefore runs once per party member and envelope, on top
of the gateway's projection (SPEC-021's cost consequence).

Three more facts follow from the seat:
- `set_viewpoint` is the spectator's alone. `commandRoles` in
  `internal/gateway/authz.go` holds its one cell, and `MayPerch` in
  `viewpoint.go` refuses every other role. A player who controls two party
  members sees through both at once, because `eyesFor` gives a player
  `perceive.Eyes{Controller: p.ID}`, every actor they control.
- The move gate in `handleCommand` asks `canSee(viewerFor(p), …)` with the
  same union.
- The harness proves a denial broadcast nothing by ordering:
  `leakedSince` (`internal/harness/engine.go`) and `leakedBelow`
  (`internal/harness/soak.go`) report any participant who received an
  envelope whose sequence is below the next accepted command's world
  sequence. That holds only while every participant's stream carries world
  sequences.

The owner ruled on 2026-10-15 and 2026-10-16:
- one active character at a time, switched by clicking its token, and the
  union goes;
- no viewpoint means no events;
- a switch is announced by a new server frame and followed by the new log
  from its start;
- the move gate uses the active character's eyes;
- the projection is deleted in the ticket that replaces it, with its
  goldens, and with no dual path.

On 2026-10-17 he added these rulings:
- a player's default viewpoint is their first party member by id, for a
  connection that has not chosen one;
- a reconnect that names the log it was reading opens on that log, while its
  participant may still watch it;
- the DM and the agent keep the world, and a DM who wants a character's view
  opens a spectator link and perches;
- a controlled creature that is not a party member gets a log of its own,
  under a new kind of grant, in a ticket after this one. Until then such a
  creature is nobody's viewpoint, and a player who controls only such a
  creature watches nobody and moves nothing.

How MapTool solves this, per CLAUDE.md rule 9.
`ZoneViewModel.makePlayerView` builds a player's view from the tokens they
have selected that they own and that have sight. With none selected, it
falls back to every token they own with sight (with individual views on),
which is the union. Every client holds the whole campaign, and the view is a
client-side filter. The GM's "Show As Player" (`LocalPlayer.getEffectiveRole`)
is the same filter over what the GM already holds.

We borrow one thing: the view follows the token clicked. We do not borrow
the union fallback, which the owner struck, or the distribution model, which
this platform refuses (rule 9). "Show As Player" was checked and not
borrowed, by the 2026-10-17 ruling.

## Done looks like

1. **A player's connection carries their character's log and nothing
   else.** A new gateway test, `TestAPlayersConnectionIsItsCharactersLog`.
   A player who controls one party member connects. After the opening frames
   (item 4), they are sent a `ViewpointChanged` naming that member, then
   exactly what `Campaign.CharacterLog` holds for it, in order and
   `proto.Equal`, then each later entry as it is written. They are sent no
   world event the log does not hold. On `main` it fails: the stream is the
   projection, in world sequences.
2. **The default, and nobody.** A new gateway test,
   `TestAPlayerWatchesTheirFirstPartyMemberByID`. A player who controls two
   party members connects watching the one with the lower id. A player who
   controls none, and a spectator who has not perched, are sent the opening
   frames and presence and no event. A player who has none and is granted a
   first party member while connected starts watching it without
   reconnecting.
3. **A switch.** New gateway tests:
   - `TestAPlayerSwitchesToAnotherOfTheirOwn`. A player's `set_viewpoint`
     naming another party member they control is answered ok, then
     `ViewpointChanged` naming it, then that log from its first entry. No
     entry of the old log follows.
   - `TestAPlayerMayWatchOnlyTheirOwnPartyMembers`. A `set_viewpoint`
     naming an actor they do not control, a non-party actor they control, an
     absent actor, or no actor is refused. The first three get one string.
   - A spectator's perch does the same through a party member's log.
   - The DM and the agent are still refused (`TestAuthorizeTableAllCommandsAllRoles`).
4. **What a connection opens with.** `CatchUpHead` is still the first frame,
   and for a seat that watches a log it carries that log's catch-up head.
   `PresenceSnapshot` follows. `ViewpointChanged` precedes the log's first
   entry on every connection of a player or a spectator, and on a seat that
   watches nobody it names no actor. The `server_test.go` tests that read
   `CatchUpHead` then `PresenceSnapshot` on a player's or a spectator's
   socket pass. `TestEverySeatCanReachTheCatchUpHeadItIsGiven` holds for
   every role.
5. **Losing the right to watch.** A new gateway test,
   `TestAConnectionStopsWatchingAnActorItMayNoLongerWatch`. The DM revokes
   the watched party member's control from the player, makes it a non-party
   actor, or removes it. The player's connection is then sent no further
   entry of that log, and it switches to their first remaining party member,
   or to nobody. A spectator whose shoulder stops being a party member
   switches to nobody.
6. **A reconnect opens on the log it names.** A new gateway test,
   `TestACursorIsHonouredOnlyInTheLogItWasReadFrom`:
   - A player who switched to their second party member and reconnects
     naming that log at cursor N opens on it, and is sent its entries after
     N only.
   - A reconnect that names a log the player may no longer watch opens on
     the default and is sent it from its first entry. So is one that names
     no log, and one that names a party member the player never controlled.
   - A spectator's connection whose `log` names an actor that is not a party
     member opens watching nobody.
7. **The move gate uses the active character's eyes.** A new gateway test,
   `TestAPlayersMoveIsJudgedByTheCharacterTheyWatch`. A player controls two
   party members in rooms that cannot see each other and watches the first.
   A move of the second's token to a square only the second sees is refused
   with the unseen-square string. After a switch to the second, the same
   move is accepted. On `main` it is accepted at once, through the union.
8. **The projection is gone from the gateway.**
   - `grep -rn 'Projector\|pastResume\|subscribeFrom\|reperch\|perchSequence'
     internal/gateway cmd` prints nothing.
   - `scenarios/goldens/*/projections/` and
     `internal/gateway/testdata/history-fold/` no longer exist, nor does any
     test that read them (`client/test/projection-parity.test.ts` among them).
   - `internal/perceive`'s tests still pass unchanged.
9. **The DM's and the agent's connections are unchanged.**
   `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`, or its renamed
   successor, passes. So do `internal/gateway/server_test.go`'s DM
   tests and `cmd/vtt`'s state dump, events tail and MCP end-to-end tests run
   as the DM or the agent.
10. **The scenarios still prove their denials.** These pass, with every
    participant of a scenario connected:
    - `TestScenarioLibraryRunsSelfContained`,
    - `TestEveryScenarioFileIsActuallyRun`,
    - `TestThreeRoleExitScenarioOverLiveServeSubprocess`,
    - `TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario`,
    - `TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts`.

    A deliberate break that makes a denied command append anything still
    turns one of them red.
11. **The gate holds it.** `task check` is green, whole:
    - no floor in `tools/coverage-thresholds.txt` is lowered;
    - `internal/gateway` is in the Go mutation gate with no unadjudicated
      survivor;
    - `check:breaking` accepts the new frame as additive.

## Rules this puts on the system

- A player's or a spectator's connection is sent the entries of the one
  character log it watches and no world event.
- A player's or a spectator's connection that names a log its participant
  may watch opens on it.
- A player watches their first party member by id, or nobody when they
  control none, at a connect that names no log they may watch, and whenever
  they watch nobody or an actor they may no longer watch.
- A connection that watches nobody is sent no event.
- A switch of what an open connection watches is announced by one
  `ViewpointChanged`, followed by the new log from its first entry.
- A player may watch only a party member they control. A spectator may
  watch any party member, or nobody. The DM and the agent watch the world.
- A refusal of a viewpoint does not say whether the actor exists, whether it
  is a party member, or whether the player controls it.
- No entry of a log is sent on a connection after the connection may no
  longer watch that log's actor.
- A player's or a spectator's connection is sent only entries after its
  cursor when it opens on the log it names.
- A player's move is refused for a square the character they watch cannot
  see.

Existing rules this moves. The sort after sign-off decides each row, and
these are the classes it will find:
- **Withdrawn and succeeded by a rule above.** A rule that held for the
  projected seat alone: VTT-177, VTT-178, VTT-179, VTT-186, VTT-187 and
  VTT-191, and VTT-155, whose "they cannot see" meant the union.
- **Rules that still hold, re-evidenced.** Their tests move or are
  rewritten: VTT-032, VTT-086, VTT-156 to VTT-160, VTT-176, VTT-180 to
  VTT-184, VTT-188 and VTT-306.
- **SPEC-016 rows whose evidence names a deleted gateway test.** These are
  re-pointed at `internal/perceive` or `internal/campaign` tests that hold
  the same rule, or withdrawn where only the gateway's projection held it.

## What it touches

1. `contract/vtt/v1/commands.proto`: a `ViewpointChanged` arm on
   `ServerFrame`, then `task generate:contract`, `contract/gen` and
   `contract/README.md` or `contract/testdata` as `check:drift` requires.
2. `internal/gateway`:
   - `server.go` (`serve`, `answerCommand`, `handleSetViewpoint`,
     `handleCommand`'s move gate), `seat.go`, `viewpoint.go` (`MayPerch`),
     `authz.go` (`commandRoles`, `playerRules`);
   - `project.go` deleted, with `export_test.go`'s projector helpers and the
     projection's test files: `project_test.go`, `project_internal_test.go`,
     `project_property_test.go`, `keystone_test.go`, `history_fold_test.go`
     and `testdata/history-fold/`, `resume_test.go`, `character_log_test.go`,
     `viewpoint_internal_test.go`, most of `server_visibility_test.go`, and
     the projection's QA files;
   - `server_test.go`, `authz_test.go`, `viewpoint_test.go` and
     `scenario_test.go` where they read a player's or a spectator's stream.
3. `internal/campaign`, if a seat needs to be told of an append it receives
   no entry for. That is a decision for the plan.
4. `internal/harness` (`engine.go` `leakedSince` and `runReconnectStep`,
   `soak.go` `leakedBelow`) and its tests. `cmd/vtt`'s end-to-end tests
   where a player or a spectator is dialled.
5. `scenarios/goldens/*/projections/` and `scenarios/goldens/README.md`.
   `client/test/projection-parity.test.ts`.
6. `tools/toolgen/main.go`'s `set_viewpoint` description, then
   `cmd/vtt/tools.json` and `contract/testdata/expected_tools.json`.
7. `docs/requirements.md`, `tools/comment-ceilings.txt`,
   `tools/coverage-thresholds.txt`, `tools/mutation-equivalents.txt` and
   `.go-arch-lint.yml` as the change needs.

The order:
- the contract comes first;
- the gateway and the harness change together, because the scenario tests
  run the harness over the gateway;
- the deletions land in the same change as the seat that replaces them.

## Specifications this moves

docs/specifications/015-the-seat-and-the-perch.md
docs/specifications/016-the-projection.md
docs/specifications/011-the-connection.md
docs/specifications/013-authorization.md
docs/specifications/007-the-wire-contract.md
docs/specifications/009-identity-and-joining.md
docs/specifications/021-each-party-member-keeps-its-own-log.md

## What could not be established

- **How a seat that is sent no entry learns that an append changed what it
  may watch.** A player granted a first party member while connected, and a
  watched actor removed (its log pauses, so no entry arrives), both need a
  wake that no entry gives. The candidates are:
  - a payload-free signal from the campaign after each append;
  - one world subscription per server that wakes the seats and forwards
    nothing;
  - re-checking only when an entry or a command arrives.

  The third misses both cases above. The plan chooses, and the choice
  decides whether `internal/campaign` changes.
- **How a client names the log its cursor was read from.** The rule is
  above. The wire for it, such as a query parameter beside `after`, is the
  plan's. Until the fourth ticket the client sends no such thing. It
  reconnects at `seenSeq - 1` (`client/src/wire.ts`), so its reconnect is
  sent the opening log from its first entry. What its fold then does with
  entries it already holds was not established.
- **Whether a player may watch nobody by choice.** The rules above refuse an
  empty `set_viewpoint` from a player, since the default would take a party
  member again at once. Not asked.
- **The size.** The list crosses more than two components: the contract,
  the gateway, the harness, the scenarios and a client test. The owner's
  ruling that the projection goes in the ticket that replaces it keeps them
  together. A further split is for the owner at sign-off.
