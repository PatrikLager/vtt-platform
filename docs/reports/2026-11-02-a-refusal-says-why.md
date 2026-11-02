# A refusal tells the player why, in plain words: the change

**Ticket:** `docs/superpowers/specs/2026-11-02-a-refusal-says-why-design.md`.
An agent that did not write it verified it and passed it with gaps.
**Plan:** `docs/superpowers/plans/2026-11-02-a-refusal-says-why.md`, written
by that verifier, narrowed with the ticket, and amended by Task 0's review.
**Where it came from:** findings A-3 and A-7 of the alpha test of
2026-10-22, item 8 of the nine in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`: a refusal read
`refused: gateway: cannot move there — a wall`, and a player whose
character was taken away was not told. The owner: "då kör vi A-3/A-7".
**The owner's rulings, all 2026-11-02:**
- "Servern, en text för alla": the server writes the words where it
  refuses, one text for every reader, the agent's included; the window shows
  them without `refused: ` before them.
- "Toast + rad i perch-vyn": the window says "You no longer play Lera." when
  it happens, and the perch tells a player who controls no party member that
  they play none. The question had described the window as perching without
  explanation; a browser probe showed the Story's "control of Lera taken
  back" in one case and nothing in another, and the owner kept the ruling:
  "Ja, beslutet står".
- After verification, the line this ticket stays inside: sight, movement,
  joining and a character granted or taken back are the board engine's; the
  rules engine, which runs a game's rules on the board, is separate and must
  take any game system. The refusals of `rules.Resolve` keep their words.
  Withdrawn with it, before sign-off: Q1, D2, D6, T5 to T12, the requirement
  labels P4 to P8 and the probes P2, P11 and P13; none of these numbers is
  reused.
- At sign-off: one ticket ("Ett ärende"), and the words of the plan's D3,
  D11 and D12 ("Godkänn orden").

**The last code commit is `27d4e28`.** Every code reference below is to
that tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `460ab3d` | Task 0: the ticket, the plan, VTT-601 to VTT-607 |
| `37bd20e` | Task 1: the move gate's two texts, `plainRefusal` and the two nameless texts; QA's `internal/gateway/qa_plain_refusal_test.go` |
| `3d23235` | Task 2: no `refused: ` in `act` and `switchTo`, `Session.onPartyMembersLeft` and the toast, the perch's line; QA's `client/test/qa-refusal-words.test.ts` |
| `27d4e28` | Task 3: E1 in `client/e2e/handover.spec.ts` |

No ledger commit follows: `check-comments.py main` found no file more than
1.0 under its ceiling after the last code commit, so D14's condition for
running `--write-ledger` was not met.

## What the code does now

**The move gate.** For a player's `move_token` of a token whose actor they
control, `handleCommand` in `internal/gateway/server.go` asks sight first
and refuses a square the eyes judging the move do not see with `You cannot
move there: you cannot see that square.`, whatever stands on it. It then
asks `engine.State.Blocked` and refuses a blocked square with `You cannot
move there: `, the reason `describeBlockage` gives, and `.`: `a wall`, `a
closed door`, `something (a boulder) is in the way`. A square off the grid
is refused by sight, so a player never reads `outside the grid`.

**The nameless refusals.** `mayVoice` refuses a player's line in character
from a party member of theirs with no name with `This character has no name.
Ask the DM for one, or speak out of character with /ooc.`, and `mayWhisper`
a whisper to the DM from one with `This character has no name. Ask the DM
for one with /ooc, then whisper.`. Both are a `plainRefusal`, a string type
in `internal/gateway/authz.go` whose `Unwrap` returns `ErrUnauthorized`, so
every refusal `Authorize` returns still wraps it. Every other refusal keeps
its words, `Authorize`'s `gateway: not authorized: …` and every refusal of
`rules.Resolve` among them.

**The toast.** `act` and `switchTo` in `client/src/app.ts` show a refused
command's `CommandResult.error` as the toast with nothing before it, in
every window.

**Who left.** `Session.onPartyMembersLeft` in `client/src/session.ts` calls
its handlers, before a `WatchableChanged` replaces the set it holds, with
the party members the previous set named as controlled and the new one does
not, by id, in the previous set's order, when there are any; a set names
party members as controlled only when it does not say the participant
controls none. A `Session` starts with no set, compares by id, and keeps
its set across a redial, so a page's first set, an equal set, a renamed one
and a redial sent the same set call no handler. `app.ts` words it for a
window whose role `/api/me` last gave as `player`: `You no longer play `
and the names joined by ` and `, `an unnamed character` for an empty name,
and `.`.

**The perch.** `PerchControl.playsNone` makes `renderPerch` in
`client/src/view/spectator.ts` draw `You play no character; the DM can give
you one.` under "Whose eyes"; `app.ts` sets it for a perching window whose
role is `player`. It is drawn from the role and the set, so a reload keeps
it.

## Done looks like, answered

1. **[x]** A player's move to a seen square a wall blocks reads `You cannot
   move there: a wall.`, and a closed door and scenery in the same form.
   `TestAPlayersMoveOntoWhatTheySeeIsRefusedInPlainWords` in
   `internal/gateway/plain_refusal_test.go`, with the move onto the door
   accepted once the DM opens it; QA's
   `TestQAPRPlainASeenObstructionIsNamedInPlainWords`. Red before Task 1's
   code, and red again under K1.
2. **[x]** A player's move to an unseen square is refused with one plain
   text whatever stands there. The tests that hold VTT-156,
   `TestAPlayerCannotProbeTheDarkWithMoveCommands`,
   `TestQAMGAnUnseenSquareIsRefusedWithOneStringWhateverStandsOnIt` and
   `TestQAMGASeenBlockedSquareIsRefusedWithTheObstructionNamed`, run with
   the new text; the two QA move-gate tests and
   `TestAMoveRightAfterARevocationIsNotJudgedByTheLostCharacter` compare it
   exactly; QA's `TestQAPRPlainAnUnseenSquareIsOneTextWhateverStandsOnIt`
   tries floor, wall, door and scenery.
3. **[x]** A line in character and a whisper to the DM from a nameless party
   member are refused in plain words, with no package.
   `TestANamelessCharacterIsToldInPlainWordsWhyItCannotSpeakOrWhisper`,
   over the wire and through `Authorize` with `errors.Is`; QA's
   `TestQAPRPlainTheNamelessTextIsToldOnlyOfTheSpeakersOwnPartyMember` shows
   the text is told of no other actor.
4. **[x]** The window shows a refused command's error with nothing before
   it. app's "a refused command is reported verbatim rather than silently
   doing nothing" and "the indicator follows the SERVER's answer, never the
   click", both red on `37bd20e`'s client.
5. **[x]** A player is told "You no longer play" and the name. session's "a
   set that stops naming a party member as controlled tells who left, and a
   first, an equal or a renamed set tells nothing", app's "a player is told
   You no longer play and the names of the party members that left, and an
   unnamed one as an unnamed character", and in the browser E1, in
   `client/e2e/handover.spec.ts`'s "the DM takes a player's only character
   back, and the player chooses another shoulder": the DM revokes Bram and
   then Ash, and Lera's window says each.
6. **[x]** A player who controls none is told so on the perch; a spectator
   is not. app's "a player who controls no party member is told so on the
   perch, and a spectator is not", spectator-view's "the perch says you play
   no character when told to, and nothing when not", and E1, before and
   after the reload.
7. **[x]** Every refusal outside items 1 to 3 reads as it does today.
   `git diff 8f933f8 27d4e28 -- internal/rules rulesets scenarios` prints
   nothing, and the scenario library and the soak pass unedited; QA's
   `TestQAPRPlainAMoveRefusedBeforeTheGateKeepsItsOldWords` and
   `TestQAPRPlainRefusalsOutsideTheNamelessCaseKeepTheirOldWords` read
   `gateway: not authorized` where it stays.
8. **[x]** `task check` exits 0 on the branch. On `27d4e28`, the last code
   commit: `TASK_EXIT=0`, from 2026-11-02 23:56 to 2026-11-03 00:51, the
   tree unchanged across the run. See Figures.

## What the rules became

- The move gate's refusal and the nameless refusals name no package and no
  id: **VTT-601** (an unseen square), **VTT-602** (a seen square that
  blocks), **VTT-603** (a nameless line or whisper, still wrapping
  `ErrUnauthorized`).
- The window shows a refusal as the server words it: **VTT-604**.
- A player is told which party member left: **VTT-605**, and **VTT-606**
  for every set that must tell nothing.
- A player who controls none is told so on the perch: **VTT-607**.
- **VTT-156** stands as worded: an unseen square is still one text.

## Phase 4a

Task 1's QA wrote 14 Go tests and Task 2's 18 client tests, from the
requirements and the specifications without the source; every one passed.
Task 3 adds assertions to an existing browser test and no behaviour, so it
had none. Task 2's QA said it
had seen the first three lines of `renderPerch` while reading the
signature of `PerchControl`; the sentence it asserts is VTT-607's own.

### QA adjudications

- Task 1: `outside the grid` is unreachable for a player, since sight
  refuses first. Not a defect; the plan's D3 said otherwise (see the plan's
  gaps), and SPEC-013 does not repeat it.
- Task 1: a name of only spaces counts as a name. The rows say "empty"; a
  near-theoretical case with a small consequence: no code.
- Task 1: an empty scenery kind reads `something (a ) is in the way.` —
  `describeBlockage` as before, from map content: no code.
- Task 1: `describeBlockage`'s unknown-scene words are unreachable for a
  player. As before.
- Task 1: a move to the token's own square is accepted. Outside the task;
  the panel never sends it.
- Task 2: an empty error would show no toast. Unreachable: every refusal
  the gateway sends carries a text.
- Task 2: the client's own `wire: not connected` and `wire: connection
  closed before a result arrived` are shown bare. Outside the ticket, whose
  rule is the server's words; told to the owner.
- Task 2: a party member renamed as it leaves is named by the previous
  set. D11's choice.
- Task 2: a redial that drops a party member tells it. D9's choice.
- Task 2: how long the loss toast stays is not specified, as for every
  toast.
- Task 2: a set with a duplicate id would name it twice; the gateway never
  sends one.

## Phase 4b

| Commit | Must | Should | Nit |
|---|---|---|---|
| `460ab3d` | 1 | 5 | 4 |
| `37bd20e` | 0 | 1 | 3 |
| `3d23235` | 0 | 3 | 2 |
| `27d4e28` | 0 | 1 | 1 |

Every finding was taken. The ones that changed what the work says:
- VTT-601 to VTT-603 claimed more than the code does: `authorize` runs
  before the move gate, and `mayWhisper` checks the recipients before the
  name. They were reworded before Task 0's commit.
- QA's off-grid test allowed `outside the grid`; it now asserts the unseen
  text alone (Task 1).
- "a command that succeeds clears the toast" could not fail and its comment
  was false; it now shows a refusal first, and a stale-toast mutant of
  `act` turns it red (Task 2).
- The "last read" clause of VTT-605 and VTT-606 is cited to QA's two
  re-read tests (Task 2), and VTT-606 and VTT-607 to QA's first-set test,
  which does not depend on the order a browser's answers arrive in
  (Task 3).

## The deliberate breaks

Each was made with a script that asserted the original text occurred once
and the broken text not at all, and undone by its inverse edit, checked by
hash.

| Break | What went red |
|---|---|
| K1: the blocked text keeps `gateway: cannot move there — ` | M1 |
| K2: the unseen text loses its full stop | the two QA move-gate tests; `TestAPlayerCannotProbeTheDarkWithMoveCommands` stayed green, as it compares the answers with one another |
| K3: `plainRefusal.Unwrap` returns nil | N1, `TestAPlayerCannotSpeakAsAPartyMemberWithNoName` |
| K4: the nameless whisper goes back to `fmt.Errorf` | N1 |
| K5: `act` keeps the prefix | U1 |
| K6: `switchTo` keeps the prefix | U2 |
| K7: `Session` ignores `controls_no_party_member` | S1, U3 |
| K8: the names joined by a comma | U3 |
| K9: compared by name, not id | S1 |
| K10: the role check narrowed to the DM (`me?.role === "dm"`) | U4 |
| K11: the unnamed fallback removed | U3 |
| K12: the perch line on every perch | U6, V1 |
| After Task 2's review: `act`'s ok arm keeps a stale toast | "a command that succeeds clears the toast rather than leaving one up" |

## Rule 9: how MapTool does this

MapTool keeps every sentence a user reads in `i18n.properties`, plain and
in the second person ("You must be a GM to use \"/{0}\".", `slash.mustBeGM`),
apart from its exceptions, and names the token in `impersonate.mustOwn` by
`token.getName()`, the name players see. A blocked move is not refused: its
walker routes around what blocks. No message for a change of ownership was
found. Borrowed: the second person, and the sentence written where the
refusal is decided. Not borrowed: the catalog file, a second home for each
sentence; refusals decided in the client; routing around a wall, since a
move here is one square and the owner's preview refuses it in words.

## The plan's gaps

- G1 (a creature named by its GM name), G3 (item 8 against the rule) and
  G8 (the vocabulary rule for ruleset tests) were settled by the
  narrowing; G7 (the browser test's names) by rewording the ticket's item.
- G4: the set a player controls is not `WatchableChanged.actors`; D9 and
  D10 hold it.
- G6: four `not.toContain("refused")` assertions went vacuous with the
  prefix; each was replaced by the positive it stood for, or removed where
  the test held that positive.
- G9: every file P8 measured sat within 0.1 of its comment ceiling; the new
  gateway tests went into a new file, `plain_refusal_test.go`, and
  `plainRefusal` has no doc comment, since `authz.go` sat at its ceiling and
  K3 shows the tests hold its unwrap.
- G10: the ticket's first rule was wider than its items; it was narrowed to
  the move gate before Task 0's commit.
- A gap the plan did not list: D3 said `outside the grid` follows the
  colon. Sight refuses an off-grid square first, so no player reads it
  (Task 1's review).

## The measurements the plan stood on

The verifier's probes (P1, P3 to P10, P12, P14) ran in a copy of the tree on
`8f933f8`:
a prototype of the server half turned 25 gateway tests red, every one
through five pinned texts, and a prototype of the client half four client
tests. Task 1 bore it out: rewriting those five pins was all the existing
gateway tests needed.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Task 2: the tests, red, then the code | The code was written first | An ordering slip. The ten new and rewritten tests were then run against `37bd20e`'s `app.ts`, `session.ts` and `spectator.ts`, copied in and restored by hash: all ten red, 274 others green |

## What could not be established

- Whether the loss toast should look unlike a refusal. It is drawn in the
  toast's one style, the red a refusal has (E1's screenshot); D12 left
  style a preference, and nobody asked.
- The pointer beside the move gate's sight check names
  `TestAPlayerCannotProbeTheDarkWithMoveCommands`, which stays green when a
  wall is answered before sight (Task 1's review: its unseen squares hold
  no wall). The QA move-gate tests catch it. Left as it is: a change to
  `server.go`'s comment is outside this ticket's items.
- E1's no-toast check after the reload catches a first set that tells only
  if `/api/me` answers first; otherwise the role check hides it. QA's
  first-set test waits for the role.

## What was deliberately left out, and where it went

- Every refusal of `rules.Resolve`: to the rules engine's own
  investigation, by the owner's line between the engines.
- `Authorize`'s other refusals, reached by a player only by a race: they
  keep their words; the soak and two scenarios pin "not authorized" (D1).
- The DM console's and the agent's other refusals: the owner's scope.
- The client's own `wire: …` texts: told to the owner as a candidate.
- The promotion race of D10: near-theoretical, by the owner's rule; no
  code, no test.
- The client's full mutation run (D15): `client/src` changed, so the
  stored report is out of date until the owner asks for
  `task check:ts-mutation:full`.

## Figures

From `task check` on `27d4e28`, 2026-11-02 to 2026-11-03. Before it,
`go clean -cache` ran and the verifier's tree copy was deleted, since 9 GiB
free was under the Go mutation gate's 16 GiB floor; 18.5 GiB were free when
that gate began.

- **Go tests:** every package passed, `internal/gateway` (137 s) and
  `cmd/vtt` (203 s), the scenario library among them, under `-race` too.
- **Go coverage:** 21 packages at or above their floors; `internal/gateway`
  94.9% (floor 93.5), `cmd/vtt` 87.1% (85.0).
- **Client coverage:** 24 files gated, each at its floor; `app.ts`,
  `session.ts` and `view/spectator.ts`, the three this branch changed, at
  100%.
- **Go mutation:** 15 packages, zero unadjudicated survivors;
  `internal/gateway` 2 survivors, both adjudicated before this branch, which
  adds no adjudication. Seven mutants timed out and were counted as killed,
  NOT measured: one in `internal/perceive`, four in `internal/sight`, one
  in `internal/rules` and one in `internal/mcp`, packages this branch did
  not touch.
- **Client mutation:** not measured. The stored report measured
  `526885bc0c77`; since then the tickets for the console lock keys, a name
  belonging to one participant, the DM booting a participant, whispering,
  and this one. `task check:ts-mutation:full` measures it when the owner
  asks.
- **Requirements chain:** 607 rows, 343 test files, 17 specifications;
  every citation resolves.
- **New prose:** 1080 added lines across 17 files, clean.
- **Comments:** 353 files, 4 added comment lines, clean.
- **Browser tests** (outside `task check`): `task e2e` in Task 3, 35
  passed and 1 skipped; `handover.spec.ts` passed again on its own.
