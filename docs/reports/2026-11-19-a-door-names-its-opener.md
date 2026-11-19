# A door's line in the Story names the character that opened or closed it — report

Ticket: `docs/superpowers/specs/2026-11-19-a-door-names-its-opener-design.md`.
Plan: `docs/superpowers/plans/2026-11-19-a-door-names-its-opener.md`.
Branch `feat/a-door-names-its-opener` from main `b022434`.
Commits: `210b47c` (Task 0: ticket, plan, register), `96b6db8` (Tasks 1 and
2: the contract, the folds, the projection), `b3d7381` (Task 3, the lines),
`e086190` (Task 4, the window), `e5dcc3b` (Task 5, the gateway), `dc84a3a`
(Task 6, the tools' texts), `2998cda` (Task 7, the real wire; the last code
commit) and `c773283` (the comment ledger). Every code reference below is
to `2998cda`.

## Done looks like

1. [x] **The event names the opener.** `OpenDoor`, `CloseDoor`, `DoorOpened`
   and `DoorClosed` carry `actor_id` (field 3, empty for none); `ToEvent`
   (`internal/gateway/convert.go`) copies it; `engine.Apply`'s door arms and
   the client fold's (`client/src/fold.ts`) refuse a non-empty one they do
   not hold, after the scene and the position.
   - `internal/engine/door_opener_test.go`'s
     `TestADoorEventNamingAnActorTheStateLacksIsRefused`,
     `TestADoorEventIsRefusedForItsSceneOrPositionBeforeItsActor` and
     `TestADoorEventNamingAnActorTheStateHoldsOrNoneFolds`;
     `client/test/fold-rejections.test.ts`'s "a door event naming an actor
     the fold does not hold is rejected, after its scene and its position";
     `internal/gateway/convert_test.go`'s
     `TestToEventOpenDoorProducesDoorOpened` and
     `TestToEventCloseDoorProducesDoorClosed`; the round trips of
     `contract/testdata/open_door_command.json` and
     `door_opened_envelope.json`.
   - Independently: `internal/engine/qa_door_opener_test.go`,
     `client/test/qa-door-opener-fold.test.ts`,
     `internal/gateway/qa_door_opener_test.go`.
   - On `b022434` the messages carry no actor: the Go tests do not compile,
     and the client's and the round trips' fail.
2. [x] **A named character exists.** `handleCommand`'s door gate
   (`internal/gateway/server.go`), after `IsDoor`, refuses a non-empty actor
   the snapshot does not hold with `There is no such character.`, for every
   role, appending nothing (VTT-868); a player the door rule refuses keeps
   that rule's text whatever actor the command names (VTT-880).
   - `internal/gateway/door_opener_test.go`'s
     `TestADoorCommandNamingAnActorTheWorldLacksIsRefusedInPlainWordsForEveryRole`
     and `TestAnAbsentActorIsAnsweredAfterTheIssuerAndTheSquare`;
     independently, `internal/gateway/qa_door_opener_gate_test.go`.
   - On `b022434` no command names an actor for the gate to find absent.
3. [x] **The DM and the agent may name any character, or none.**
   - `internal/gateway/door_opener_test.go`'s
     `TestTheDMAndTheAgentNameAnyActorOrNoneOnADoorFromAnywhere`: the
     player's character on another scene, the player's other character in
     another scene's room, an actor with no token, and none, each accepted
     for the DM and the agent and named on the event; independently,
     `qa_door_opener_gate_test.go` (actors on the door's own scene, far from
     it) and `qa_door_tool_claims_test.go`.
4. [x] **A viewer sees the opener only if it saw them.** `classify`'s door
   arm (`internal/perceive/projection.go`) forwards a door event when the
   square is in the look and `saw` the actor; `Event` passes `transitions`
   the event as its cause only when it forwards it, so `doorTransitions`
   sends eyes that see the square and were not forwarded the event the
   door's new state with no event id, which draws no Story line (VTT-820).
   - `internal/perceive/door_opener_test.go`'s
     `TestADoorOpenedByAnActorTheEyesNeverSawArrivesAsItsStateAlone` (the
     goblin behind the door, whom the opening itself brings into sight),
     `TestADoorClosedByAnActorTheEyesNeverSawArrivesAsItsStateAlone`, and
     `TestADoorEventNamingAnActorTheEyesSawOrNoneIsForwardedOnceWithItsEventID`,
     the first two for a player's eyes and a spectator's shoulder;
     independently, `internal/perceive/qa_door_opener_test.go`.
   - Written before the code and seen red: the opener was forwarded, and the
     closer's frame did not fold.
   - `internal/campaign`'s character-log oracle and property tests walk
     door events naming actors (`internal/eventgen/model.go`'s `opener`).
   - On the real wire, `client/e2e/doors.spec.ts` (item 8).
5. [x] **The lines name the opener.** `door` in
   `client/src/view/spectator.ts` words a door event naming an actor
   "<actor> opened a door" / "<actor> closed a door", the actor named by
   `names.actor`, and today's words for none; `tell` draws it, `describe`
   adds " at x,y".
   - `client/test/spectator-view.test.ts`'s "a door event naming an actor
     names it as the window names an actor, in the Story without its square
     and on the Events panel with it" (a player's window: the party member's
     name, the token's name, "a creature"; the DM's: the GM chain);
     independently, `client/test/qa-door-lines.test.ts`.
   - Written before the code and seen red: `tell` and `describe` named no
     opener.
6. [x] **The window sends the opener.** `actingActor`
   (`client/src/view/player.ts`) is the actor a board click moves;
   `doorCommandFor` (`client/src/view/doors.ts`) names it on a player's door
   command and names none on a DM's or an agent's; `mayWorkDoor` asks that
   actor's token alone, so the toggle and the click follow it.
   - `client/test/doors.test.ts`'s "a player's door names the actor a click
     moves, and only that actor's token beside the door offers it" and "a
     DM's and an agent's door command names no actor, whatever actor it is
     handed"; `client/test/player.test.ts`'s "the toggle follows the actor a
     click moves, whatever else of the player's stands beside a door";
     `client/test/app.test.ts`'s "a player's armed door click names the
     character they watch, and a switch to one far from the door withdraws
     the toggle and disarms", "with doors armed, a player's click on a door
     the watched character does not stand beside sends nothing, whoever
     else of theirs does" and "the DM's armed door click names no actor,
     though a chip selects an actor of theirs"; `client/test/commands.test.ts`'s
     "openDoor naming an actor matches the committed open_door_command
     fixture"; independently, `client/test/qa-door-window.test.ts`.
   - Written before the code and seen red, but for the DM's, which pins
     today's behaviour: a door command named no actor, and the door tool
     asked any controlled token.
7. [x] **The agent's tools carry it.** `open_door` and `close_door`
   (`tools/toolgen/main.go`) offer `actorId` without requiring it and say
   that it names the actor that opens or closes the door, any actor the
   campaign holds, or none; that a player's door command needs a token of a
   character of theirs beside the door; and that an actor the campaign does
   not hold is refused, once those checks pass.
   - `tools/toolgen/main_test.go`'s
     `TestTheDoorToolsOfferAnActorWithoutRequiringIt` and
     `TestTheDoorToolsSayWhomTheirActorMayName`; independently,
     `tools/toolgen/qa_door_tools_test.go`,
     `tools/toolgen/qa_door_tool_texts_test.go`, and
     `internal/gateway/qa_door_tool_claims_test.go`, which runs each of the
     texts' claims for the agent.
8. [x] **The gate.** `task check` is green on `c773283`, the ledger commit
   on top of `2998cda`, and `task e2e` is green on the same commit,
   `client/e2e/doors.spec.ts` among its specs; the embedded bundle under
   `cmd/vtt/webdist` was rebuilt in every commit that changed the client
   (`96b6db8`, `b3d7381`, `e086190`). `task check` did not measure the
   client's mutation: its stored report measured `b022434` (SPEC-023).

## What the rules became

- The event copying the command's actor: VTT-503, its door tests extended.
- A door event naming an actor the fold has not folded: VTT-866.
- An absent actor refused in plain words, for every role: VTT-868; the
  order after the player rule: VTT-880 (added at Task 5's review).
- The DM and the agent naming any actor, or none: VTT-867.
- Forwarded only to eyes that saw the actor: VTT-869; the door's state
  without an event id otherwise: VTT-870.
- The Story's words: VTT-871, succeeding VTT-819 (withdrawn at Task 3);
  the Events panel's: VTT-872.
- The player's click and the door tool: VTT-873 (the click names the
  acting actor), VTT-874 (a door it does not stand beside sends nothing),
  VTT-875 (the toggle); and VTT-879 (a switch disarms the door tool,
  added at Task 4).
- The DM console names no actor: VTT-876.
- The tools: VTT-877 (the schema), VTT-878 (the descriptions).

Refused at sign-off: a player's door command checked on whose actor it
names, or where that actor stands (the plan's R2 and R3) — the owner ruled
it theoretical. VTT-149, VTT-212 and VTT-609 stand as worded.

## The deliberate breaks

Each was made by an edit, run, and restored, the tree's hash checked
afterwards; each turned its test red.

- **Tasks 1 and 2:** `ToEvent` dropping either arm's actor; `close_door`'s
  required override removed; a fixture's key misspelled, or its actor
  emptied (the TypeScript round trip); the engine's actor check dropped, moved
  before the scene, or refusing an empty actor; `fold.ts`'s check dropped,
  looked up on a plain object, or refusing an empty actor; the
  projection's `saw` dropped, replaced by sight after the event, or the
  closer not read; the cause passed whatever the verdict, or never; the
  walk naming an actor the model lacks.
- **Task 3:** `tell` naming no opener; the opener named by its id;
  `describe` dropping a closed door's square; a door naming none read as
  "someone".
- **Task 4:** a player's door naming none; the first controlled actor named
  in place of the watched one; `mayWorkDoor` asking any controlled token;
  the DM's door naming the acting actor; the toggle gated on the first
  controlled actor; a switch leaving the door tool armed; `openDoor`
  dropping the actor.
- **Task 5:** the check dropped (the fold's text answers instead); the
  actor checked before the square; the check for the DM and the agent
  alone; the id named in the text; the absent actor answered before the
  player rule; `ToEvent` dropping the actor.
- **Task 6:** a description saying the actor must stand beside the door;
  the property's doc without its refusal; `actorId` required; a description
  promising a Story line.
- **Task 7:** a player's door naming none, the bundle rebuilt for it and
  then rebuilt back byte-identical; the projection forwarding an opener the
  eyes never saw.

The break script's guard stopped one break before it was applied (Task 3):
its replacement text already stood in `tell`, so undoing it would have been
ambiguous; it was rewritten to a unique edit.

## Rule 9: how MapTool does this

MapTool's core has no door: a door is drawn geometry that blocks vision
(`DungeonDraftImporter` imports a map's portals as vision-blocking lines),
and nothing in the core opens one or says who did. What a macro says in
the chat speaks as the token the user impersonates (`ImpersonateMacro`,
`CommandPanel.TokenIdentity`), which `ServerPolicy.isRestrictedImpersonation`
can restrict to tokens the player owns (`Token.isOwner`). Borrowed: the act
is the character's the player is acting as, and the GM speaks as anyone or
as nobody. Not borrowed: the restricted policy's server check of whom a
player may name (the owner's ruling, D6); and MapTool's answer to who sees
what, since every MapTool client is sent the whole chat.

## The plan's gaps and the owner's answers

- **G1 to G4** (SPEC-007 unnamed; the fifth rule overclaiming; a false
  citation; files the ticket did not name): amended in the ticket before
  Task 0's commit.
- **G5** (a DM's or an agent's absent actor): OD-1 B, the gateway's check.
- **G6** (no test classifies every string field of every event against
  SPEC-018): unchanged; VTT-866's tests hold the new field.
- **G7** (how `<actor>` is named): D8, as the window names an actor.
- **G8** (the ticket's open points): OD-1, OD-3, and M6 (a window that can
  act watches only a character its player controls).

The plan's D5 and D6 were rewritten to these answers before Task 0's
commit; the code does what the committed plan says.

The owner's answers, 2026-11-19: the opener is the character; a player opens
as the one they watch; the DM and the agent may name any actor or none, the
console none; an unseen opener gets no line; OD-1 B (`There is no such
character.`); OD-2 overtaken — "detta är ett teoretiskt fall, vi bygger inte
kod/kontroller för det", so no check on a player's named actor; OD-3 A (the
e2e spec); OD-5 A (one ticket). OD-4 A (SPEC-007 gains a section) was the
writer's, a records question.

During Task 4's review the owner asked why the door tool needs arming at
all, rather than distance alone deciding. The answer, from
`docs/superpowers/specs/2026-08-30-dm-hands-and-retraction-design.md` §4.1,
"A door is armed, then clicked": a board click already means a move, so a
click needs disarming before it can mean something else; and, this report's
own reasoning, an open door is a square a token can stand in, so a click on
one could mean either. His ruling: a button per door the watched character
stands beside, with no arming for players, as the next ticket; this one
lands with the arming and VTT-879.

## The existing tests that changed

- `internal/gateway/convert_test.go`'s two door tests name an actor and
  assert it on the event.
- `client/test/doors.test.ts`: every call of `mayWorkDoor` and
  `doorCommandFor` passes the actor its fixture places (`a1`; the empty id
  for the DM's; `ghost` for the orphan-token test), asserting what it
  asserted.
- `client/test/app.test.ts`, `qa-story-mechanical.test.ts` and
  `spectator-view.test.ts` cite VTT-871 where they cited VTT-819;
  `feed.test.ts` drops VTT-819 and cites neither, as it tests no wording.
- `contract/testdata/expected_tools.json` takes the regenerated tools.

## Phase 4a adjudications

No QA agent found a behaviour defect. One line per finding.

- Task 1, requirement needing an id (the actor's JSON name, empty
  omitted): refused — held by the round-trip fixtures and VTT-873's
  commands test.
- Task 2, spec ambiguity (the correction's actor): resolved by VTT-866 —
  a correction naming an actor the viewer never saw would be refused by
  its own fold; the tests assert none.
- Task 2, spec gap (SPEC-016 describes the old door arms): goes into
  SPEC-016.
- Task 2, spec gaps (a no-change door event naming a seen actor; an
  out-of-sight door): today's behaviour, VTT-869 and VTT-212.
- Task 2, ids asked for (an unseen actor on a no-change event; an actor
  leaving sight with the event): VTT-869 and VTT-213 hold them.
- Task 2, process: QA read the import lines of an existing test by
  mistake; nothing taken.
- Task 3, spec gap (SPEC-022's old wording): goes into SPEC-022.
- Task 3, spec gap (a DM naming a tokenless actor): not a gap — the namer's
  chain, held by the spectator-view test.
- Task 3, spec gap ("x,y" spacing): SPEC-022 writes "x,y".
- Task 3, process: one grep of an existing test was blocked; nothing used.
- Task 4, spec gap (VTT-879 only indirectly observable): not a gap — the
  app test's click after the switch moves, which only a disarmed board does.
- Task 4, cells not covered (a perching window, a spectator): today's
  behaviour, SPEC-022's perch rule.
- Task 4, process: QA read a few lines of `client/src/wire.ts` to fix its
  fake socket; nothing asserted from it.
- Task 5, spec gap (the player's own connection showed no door event in
  its drain): its stream begins with a replay — the projection's.
- Task 5, spec gap (the player's text at a square that is not a door):
  held by VTT-608.
- Task 5, ids asked for (the gates' order; one text for every role and id
  shape): VTT-868's words, and VTT-880.
- Task 6, spec gap ("the Story tells it to whoever saw the actor" cannot be
  run for the agent): the sentence was false and was dropped at review.
- Task 7 had no QA: it adds a test and no behaviour.

## Phase 4b

A reviewer read each task's diff before its commit; Task 0's records were
read once and fixed.

- **Task 0:** VTT-873 contradicting VTT-874; the plan's MapTool section
  still borrowing the dropped check; sentences resting on the dropped
  player rule; blueprint 10 claimed served where a hand-sent command breaks
  it; rows joining two obligations, or claiming a frame not always sent.
- **Task 1:** a lint failure in a QA helper; and Task 1 alone forwarding an
  unseen opener's id to every eyes that see the door's square — so Tasks 1
  and 2 were committed as one.
- **Task 2:** `classify` over the complexity limit (one door arm reading
  `doorAct`, no threshold touched); QA citations no break of the row turns
  red; two comments reading a nil cause as a perch only.
- **Task 3:** a citation no wording break turns red; a fixture whose DM
  chain could not be told apart.
- **Task 4:** QA's file failing the typecheck; tests cited for VTT-874 and
  VTT-876 that could not fail; a comment describing the old rule; and the
  stuck armed state on older paths, taken to the owner (above).
- **Task 5:** tests cited for VTT-868 that no break of it reddens — the
  not-a-door order moved to VTT-608, the order after the player rule to
  VTT-880; SPEC-013's one-text bullet and its sentence on the fold's door
  arms, for the specification.
- **Task 6:** a description promising the Story would tell the opening to
  whoever saw the actor — false for eyes that saw the actor and not the
  square — dropped; the player's rule stated, the texts being served to
  every role.
- **Task 7:** the spec failing most runs under load — a repaint after
  arming replaced the tokens' elements between finding and measuring them;
  it now measures in the page in one step, after the armed label shows; a
  negative check racing the frame that would fail it, moved after a later
  line on the same socket.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The plan's Task 1 carries D3 (both folds refuse an absent opener) and D12 (the walk names openers) | Both moved to Task 2, and Tasks 1 and 2 were committed as one | With them and without the projection's `saw`, `internal/campaign`'s character-log tests went red: a log was forwarded an opener its eyes never saw and its fold refused it; and Task 1's reviewer found Task 1 alone forwarded an unseen opener's id to eyes that see the square |
| The plan's D15: the player's Story shows no line naming the participant | The spec has no such check | After "Brannoc closed a door" has passed, no line can name the participant there, so the check could not fail; Task 7's review dropped it |
| No row for the window's disarming | VTT-879: a player's switch of character disarms the door tool | The toggle now follows the watched character; armed and switched to one far from a door, a player kept doors armed with no toggle to disarm them |
| No row for the gate's order after the player rule | VTT-880 | Task 5's reviewer found the order held by no row |
| `classify` keeps its two door arms | One arm for both, reading `doorAct` | The lint's complexity limit |

## What could not be established

- **The e2e spec under a slower machine.** It passed repeated separate runs
  after its review's fix, under load; whether a slower machine still races
  the board's repaint is not known.
- **The client's stored mutation report** measured `b022434`; this ticket
  changed the client after it, so `task check` judges none of it until a
  full Stryker run, which is the owner's to ask for (SPEC-023).

## What was deliberately left out

- A door's lock, key or any rule of who may open what.
- A name for the DM console's own opening.
- The square in the Story; a participant's name anywhere a player reads; a
  change to which door a player may work.
- A check of whose character a player's door command names. Blueprint 10
  and 20 are therefore not held for a command sent by hand: it may name any
  actor the world holds, and the Story names that actor; and it learns
  whether an id names an actor. The owner ruled the case theoretical (D6).
- The stuck armed state on older paths: a player's door tool stays armed
  with no toggle when the DM moves the watched character's token away from
  the door, when the door goes, or when a newer scene becomes current.
  Present before this ticket. Deferred to the next ticket, a button per
  door with no arming for players, by the owner's ruling of 2026-11-19;
  this report records the deferral until that ticket is written.
