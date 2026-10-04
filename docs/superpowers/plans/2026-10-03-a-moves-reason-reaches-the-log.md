# A move's reason reaches the log — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-03-a-moves-reason-reaches-the-log-design.md`
**Verified:** 2026-10-03, by `verify-ticket`, an agent that did not write the
ticket, against `347796c` on `feat/move-reason` (equal to `main`; the ticket
untracked). Verdict: **Passes with gaps.** The gaps are listed at the end and
travel with this plan. This plan does not edit the ticket; where an item is
thin, the plan decides around it and says so.

**Goal, in the ticket's words:** a `move_token` that carries a reason appends
a `TokenMoved` whose reason is the command's, and one with none records none;
what a player or spectator is sent of such a move follows the owner's ruling
at sign-off, while the DM and the agent receive the event unchanged; the
client's feed and ticker show a move's reason when the frame carries one; the
contract change is additive and regenerated; SPEC-013 no longer says the
reason is dropped, VTT-257 no longer needs its qualifier, and the debt entry
is closed by the test that holds it; `task check` whole is green.

**The owner's ruling of 2026-09-29, which bounds this plan:** only what a
viewer sees gives it information. SPEC-016 implements it.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool's moves
carry no annotation. `StartTokenMoveMsg` holds `player_id`, `zone_guid`,
`key_token_id` and `selected_tokens`; `UpdateTokenMoveMsg` a zone, a token and
a point; `StopTokenMoveMsg` a zone and a token
(`messages/src/main/proto/message_types.proto`). `ServerMessageHandler`'s
`handleMessage` relays all three, and `SET_TOKEN_LOCATION_MSG`, to every other
client by `sendToClients`. Free text with an audience is a chat `TextMessage`
whose `channel` names it (`TextMessage.Channel.GM`, "GM visible only"; `GMME`;
`WHISPER`; and others); `MESSAGE_MSG` is relayed to every other client the
same way, and `MapTool.addServerMessage` drops a GM-channel message on a
client whose player is not a GM. **Borrowed:** the audience. Text a GM writes
beside an action is the GM's, so a move's reason goes to the DM and the agent
and to no player or spectator. The author stays on the move: `player_id`
travels with `StartTokenMoveMsg`, as the envelope's `ParticipantId` does on a
forwarded `TokenMoved` today. **Refused, each with its reason:** the
client-side filter, because here the projection never sends what a seat may
not read, which is the distribution model rule 9 names; and a channel chosen
per message, which would be a visibility on the reason (Q1, option c2): no
client offers one, and the ruling needs none. Read by the verifier in
`~/dev/RPTool/maptool` at `f4b7fef6c`: the three messages,
`TextMessage.Channel`, `addServerMessage`'s filter and `handleMessage`'s relay
list. SPEC-013, SPEC-016, the register and the code name no other project;
this plan and the report do.

## Verification, check by check

1. **Every path resolves — by command.** `ls` finds all 26 paths the ticket
   names. `grep` finds each symbol where the ticket puts it: `MoveTokenRequest`'s
   `optional string reason = 3` and its "shown in the log"; `tools.json`'s
   `move_token` with a `reason` property outside `required`; `moveToken`'s
   `reason?`; `commandRoles`' `move_token` row for the DM, the agent and the
   player; `TokenMoved` with exactly `token_id`, `scene_id`, `from`, `to`;
   `ToEvent`, `campaign.Append`, `handleCommand`, `Projector.Project`,
   `classify`, `describe`'s "moved to", `TestTheDMReceivesEverythingUnchanged`,
   `TestToEventMoveTokenProducesTokenMoved`; `tools_test.go`'s "testing
   dispatch" and `client_command.json`'s reason. SPEC-013's "has none in
   `TokenMoved` and is dropped", VTT-257's "that the event has a field for" and
   the debt entry's title each `grep -c` to 1. `handleCommand` is unexported; a
   test reaches it over the wire.
2. **"Done" is an observation — by command.** Item 1's test does not exist;
   a sketch (P2) fails today and passes after, and `go test -count=1 ./cmd/...`
   is green on the probe tree, so every golden stream is byte-identical. Item
   2 waits on the ruling (Q1); for option (a) the wire sketch (P3) and the
   unit sketch (P7) fail on a tree with the field and the conversion and pass
   after.
   `TestTheDMReceivesEverythingUnchanged` passes before and after, but it
   projects a `TokenPlaced` and observes nothing about a reason (Gap 2). Item
   3's sketch fails on the field-only tree (P8). Item 4 measured (P1). Item 5:
   `TestToEventMoveTokenProducesTokenMoved` passes today and sets no reason;
   the three greps of check 1. Item 6 is the gate.
3. **Each rule is breakable — a reading, then probes.** The ticket's four rules
   and their breaks are the sort's (D9); each break was run in a scratch clone
   (Measurements). One finding: an over-stripping copy stays green against a
   test envelope with no event id, participant or role (K3), so C's unit test
   builds a full envelope.
4. **The scope matches the claim — by command, then a reading.** `ToEvent` has
   one production caller (`handleCommand`), `Project` one (`seat.receive`),
   `describe` two (`renderFeed`, `renderTicker`); production code builds a
   `TokenMoved` in `convert.go` and `internal/eventgen` only, and reads one in
   `engine.Apply`, `classify`, `handleCommand`'s backfill and `client/src/fold.ts`
   (`git grep` over non-test Go and `client/src`). No test under `cmd/vtt`,
   `internal/harness` or `internal/mcp` sends a move with a reason through the
   gateway. The work reaches files the ticket does not list:
   `contract/vtt/v1/commands.proto`'s comment (D5), `contract/roundtrip_test.go`
   and `contract/events.test.ts` (D2), `server_visibility_test.go` (D4),
   `internal/eventgen/model.go` and `project_property_test.go` (D6, Q6), and
   the VTT-257 citation lines in `server_test.go` and
   `cmd/vtt/scenario_goldens_test.go` (D10). None widens it past the declared
   three components.
5. **No recorded decision is contradicted silently — a reading.** SPEC-013's
   "is dropped" is overturned; the ticket names it. SPEC-016's "for
   `forwarded`, those frames and then the event" and its "every frame it builds
   is new" search change under option (a); the ticket names SPEC-016 and
   defers the ruling. No ADR or earlier ticket decides a move's annotation
   (`git grep -i` over `docs/adr`, `docs/superpowers/specs` and
   `docs/specifications`). The 2026-10-01 testimony report's open question is
   left as it stands (Q3).
6. **The records the work moves are named — by command, then a reading.** The
   section lists SPEC-007, SPEC-013 and SPEC-016; all three resolve. The
   reading: no specification but SPEC-013 and SPEC-016 mentions `TokenMoved`
   or `move_token` (`grep` over `docs/specifications/*.md`), and both move
   (D8). SPEC-007 holds no sentence a move's reason makes false and none it
   lacks, so it is named and not moved (Q7).

## Measurements this plan stands on

All by command, by the verifier, in a scratch clone of `347796c` (`git clone
--no-hardlinks`), never in the working tree; the sketches are the verifier's,
were discarded, and the clone was deleted.

| # | Tree | Result |
|---|---|---|
| P1 | `string reason = 5` on `TokenMoved`, `task generate:contract`, `task build:client` | `events.pb.go`, `events_pb.ts` and `cmd/vtt/webdist/assets/index.js` change; `tools.json` does not; `go build ./...` clean; `go test ./contract/...` and `bun test contract/events.test.ts` green; `task check:breaking` prints its three header lines and nothing else |
| P2 | P1, a wire test (DM moves `t1` with a reason, then without; reads `f.log`) | red: `appended reason = "", want "the floor gives way under the hero"` |
| P3 | P2 with `ToEvent` copying `GetReason()` | P2 green; a wire test of the player's stream red: `the player was sent the move's reason` |
| P4 | P3 with option (a), D4's text | both wire tests green; `go vet`, `gofmt`, `golangci-lint` (2.11.4) clean; the three `project.go` keys read the same statements |
| P5 | P4 with eventgen giving alternate moves a reason (D6) | `internal/eventgen`, `internal/campaign`, `internal/harness`, `internal/gateway` green |
| P6 | P5 with the property clause | green; forwarded moves carrying a reason, per seed 1 to 6: 0, 0, 0, 0, 1, 3 (every move reasoned: 0, 0, 0, 0, 3, 3) |
| P7 | C's unit test (player, spectator on `hero`, DM) | on P3's tree red: `player: forwarded the reason`; on P4's green |
| P8 | P1 with a render test | red: `Received: "t moved to 3,4"`; green after D7, `client:typecheck` clean |
| P9 | `token_moved_reason_envelope.json` and its two round trips | at the base, Go `unknown field "reason"`, TS `key "reason" is unknown`; after P1 both green |
| P10 | `ToEvent` writing `"moved" + GetReason()` | red: `TestScenarioGoldenStreamsHaveNotDrifted` (session-zero, smoke, story-table, three-role-exit) and P2 |

On the final probe tree, P1 to P10's changes together (P10's undone): `go test
-count=1 ./internal/... ./contract/... ./tools/... ./cmd/...` green; `bun test
client/test contract contract-spike` 817 pass, 0 fail; `semgrep --config
.semgrep` clean on the three changed Go files; `check-comments.py main` clean;
`check-doc-owner.py` "80 files"; `task check:new-prose` "all clean"; both
mutation self-tests `OK`. A comment-only edit of `commands.proto` changes
`commands.pb.go` and `commands_pb.ts` and leaves the bundle byte-identical.

**What breaks what** (each a single edit on the final probe tree, restored after):

| # | Edit | Red |
|---|---|---|
| K1 | `forwardable` returns `env` | the property test, C's unit test, C's wire test |
| K2 | `forwardable` clears the reason on `env` itself | C's unit test alone; the wire test stays green |
| K3 | the copy also clears `EventId` | green against `envelope()`'s bare event; red once the test event carries an id, participant, role and session |
| K4 | `Project`'s DM arm returns `forwardable(env)` | C's unit test's DM half alone: the DM's seat has no projector (`seat.receive`), so no wire test reaches that arm |
| K5 | clone every forwarded move, reason or not | green: no behaviour changes |

**Comment shares** (`check-comments.py --report`, share / ceiling, at the base
then on the probe tree): `convert.go` 8.6 → 8.5 / 8.7, `project.go` 18.6 →
18.6 / 18.7, `spectator.ts` 49.8 → 49.7 / 49.9, `eventgen/model.go` 29.2 →
29.1 / 29.2. Test files at the base: `project_test.go` 25.8/25.9,
`server_test.go` 27.3/27.4, `server_visibility_test.go` 28.6/28.7,
`convert_test.go` 14.4/14.4, `project_property_test.go` 20.7/20.7.

**Mutation keys.** `internal/gateway` is in `PACKAGES`; `internal/eventgen` is
not. `project.go` holds three, all below `Project`: the two loop bounds in
`objectInSight` and the `len(a)+len(b)` in `sortedSceneIDsUnion`.
`convert.go` holds none. `client/src/view/spectator.ts` holds two, both below
`describe`: the two comparator boundaries in `renderPerch`'s sort. A
line-neutral edit in `Project` and in `describe`, with the new functions at the
end of their files, moves none (P4, P8).

**Records and gates at the base.** `check-requirements-chain.py .`: `259 rows,
206 test files, 11 specifications`. `check-comments.py main`: `251 files, 0
added comment lines, 251 ledger rows; clean`. `check-doc-owner.py .`: `80
files`. The register's withdrawn rows read `WITHDRAWN <date>: <the text>. <why>.
Succeeded by VTT-NNN.` with the evidence `**READING — verify-ticket check 5 of
<date>; withdrawn, held by nothing**` (VTT-035, VTT-207), and their citing
tests were re-pointed (VTT-207's, in `9705c43`). A debt entry is closed by
moving it to a dated section with a `**Closed by**` line (`1724a69`). Free
space on `/System/Volumes/Data` read 29,643,656 and 28,312,864 KiB (28.3, 27.0
GiB), the Go cache 4.2 GiB after the probes; load averages 1.6 to 2.4.

## Constraints that bind every task

- `CLAUDE.md` rule 2: no gate is weakened; the ledger only goes down, through
  `--write-ledger`; a key is re-pointed by its statement text, never deleted.
- Rule 3: additive only. One field under a new number; `check:breaking`
  reports, and its report must name nothing.
- Rule 4: one fold. `engine.Apply` and `fold.ts` do not read the reason;
  `forwardable` writes only to its own copy.
- Rule 5: "reason" is platform vocabulary.
- Rule 8: this plan, the records and the report cite symbols and tests, never
  a line; the adjudication files keep their coordinates.
- Rule 9: the answer above is recorded before any task runs.
- Rule 10 and SPEC-010: a comment is a warning, a pointer or an exported
  symbol's one-line doc. Proto comments are SPEC-007's and warn.
- SPEC-008: ids from `requirement-id` only, after sign-off; the withdrawal is
  written in VTT-035's form.
- SPEC-016's "Nothing may write to an envelope a seat is handed" and VTT-222.
- `check:drift` passes only on a committed tree: a commit that regenerates the
  contract or changes `client/src` carries its rebuilt `cmd/vtt/webdist`, and
  the order is review, commit, gate.
- The review package is `git diff HEAD`; nothing is stashed or checked out
  while a reviewer reads; `git add` and `git commit` in separate calls, `git
  show --stat HEAD` after each; a new test goes after a closing brace, never
  between another test's doc block and its `func`.
- The ticket and every report under `docs/reports/` are not edited.

## Decisions this plan makes

**D1. The field.** Forced by rule 3 and by the events' own style.
`TokenMoved` gains `string reason = 5;`, the next number. A plain `string`,
as `ResourceChanged.reason` and `ConditionRemoved.reason` are; `events.proto`
declares no `optional` field, and toolgen reads `optional` on commands only.
protojson omits an empty string, so a move with no reason serializes as today
and no golden moves (P1, P5). Its comment is a warning: `Never forward reason
to a player or spectator: it is free text its issuer wrote and can name what
they do not see (SPEC-016).` Regenerated by `task generate:contract`, then
`task build:client`; `task check:drift` after the commit.

**D2. The fixture.** Forced by SPEC-007's "the fixtures in
`contract/testdata/`" and the `note_upserted_envelope.json` precedent. A new
`contract/testdata/token_moved_reason_envelope.json`, an agent's move with a
reason, held by a new `TestTokenMovedReasonEnvelopeRoundTrip` in
`contract/roundtrip_test.go` and a row in `contract/events.test.ts`'s `cases`.
`token_moved.json` and `envelope.json` stay reason-less, so the omitted shape
stays held. The generated TS type gains `reason: string`, and nothing in
`client/src` builds a `TokenMoved` by hand (P8's typecheck).

**D3. The conversion.** Forced by Done items 1 and 5. `ToEvent`'s `MoveToken`
arm gains `Reason: c.MoveToken.GetReason()`, and `handleCommand`'s backfill is
unchanged. `TestToEventMoveTokenProducesTokenMoved` gives a reason and asserts
it, the one existing test that changes. Two wire tests in `server_test.go`,
after `TestMoveTokenBroadcastBackfillsSceneAndFrom`'s closing brace, read the
appended event through `f.log`: `TestAMoveWithAReasonAppendsItsReason` (row A,
red today, P2) and `TestAMoveWithNoReasonAppendsNone` (row B).

**D4. The projection, option (a), per Q1.** Forced by the ruling, by
SPEC-016's shared-envelope consequence and by the measured breaks. A new
function at the end of `project.go`:

    // forwardable is env, or for a move with a reason a copy without it (SPEC-016).
    // Clone before clearing: every seat and the DM share env.
    func forwardable(env *vttv1.Envelope) *vttv1.Envelope {
        if env.GetTokenMoved().GetReason() == "" {
            return env
        }
        c := proto.Clone(env).(*vttv1.Envelope)
        c.GetTokenMoved().Reason = ""
        return c
    }

and `Project`'s `out = append(out, env)` becomes `out = append(out,
forwardable(env))`, one line for one, so no key moves. It copies only a move
that carries a reason: every other forwarded event stays the same pointer, as
the QA tests that recognise a forwarded frame by pointer
(`qa_testimony_eyes_test.go`, `qa_testimony_sight_test.go`,
`qa_note_projection_test.go`) assume, and copying every move changes nothing
observable (K5). `seat.receive` is safe with a copy: `pastResume` filters on
`GetSequence`, which the copy keeps, and `s.received` keeps the original, which
`campaign.FoldPrefix` folds. The DM's and the agent's arm still returns `env`.

Tests: `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` in
`project_test.go`, for a player and a spectator perched on `hero` over
`twoRooms`, with an event carrying an event id, participant, role and session
(K3): the forwarded frame equals the event less its reason; the event is
unchanged (VTT-222, K2); a DM projector then handed the same event returns it,
reason and all (K4). `TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt`
in `server_visibility_test.go`: the player's stream holds the move and
`mentions` no reason; the DM's does.

The keystone needs nothing: it compares folded states, and `engine.State`
holds no reason; and `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`
compares golden bytes, and no golden gives a move a reason (Done item 1). So
neither holds row C (Gap 4). `fold.ts`'s `tokenMoved` arm reads `tokenId` and
`to`, and no parity test sees a reason (P8's bun run).

**D5. A player may give a reason, per Q2.** Forced by nothing refusing one
today and by option (a): a player's reason reaches the log the DM and the agent
read, and no other player or spectator. No authorization rule is added. The
`MoveTokenRequest.reason` comment's "DM/agent annotation shown in the log"
stops describing who may write it and becomes `Optional annotation recorded as
TokenMoved.reason, which no player or spectator is sent (SPEC-016).`, its
`optional` sentence kept. The regeneration changes `commands.pb.go` and
`commands_pb.ts` comments only; the bundle does not move (measured).

**D6. The property walk learns the reason, per Q6.** Forced by row C's reach
across every transition. `eventgen`'s `Step` passes `idx` to `moveToken`, which
sets `Reason: moveReasons[idx%len(moveReasons)]` over `{"", "prop-reason"}`, a
draw by index, so the walk is unchanged (`Step`'s warning). In
`TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` a player frame
whose `TokenMoved` carries a reason fails the walk, and a vacuity floor
requires at least two seeds to forward a reasoned move (P6 measured two).

**D7. The client.** Forced by Done item 3 and the keys. `describe`'s
`tokenMoved` arm stays one line, and a function goes at the end of
`spectator.ts`:

    return because(`${p.value.tokenId} moved to ${p.value.to?.x ?? 0},${p.value.to?.y ?? 0}`, p.value.reason);

    /** label, followed by the reason an event gives when it gives one. */
    function because(label: string, reason: string): string {
      return reason === "" ? label : `${label} — ${reason}`;
    }

The em dash is a preference, after `session started — <name>`. Every role's
feed and ticker run `describe` through `renderSpectator`, which `app.ts`'s
`paint` calls for every role, the DM console included; so the DM and the agent
see a reason, and under option (a) a player or spectator never does. Tests in
`client/test/spectator-view.test.ts`: "the feed and the ticker show a move's
reason", rendering a log of one reasoned move and asserting `.feed
.mechanical` and `.ticker .tick` exactly (P8); and a reasoned `tokenMoved` row
in the `describe` table, beside the reason-less one, which holds the bare
label. `task build:client` rebuilds `cmd/vtt/webdist`.

**D8. The records.** Forced by check 6 and by moving each sentence with the
code that makes it true.

- **SPEC-013:** "carrying every field the command gave that the event has a
  field for, a grant's `kind` included; `move_token`'s `reason` has none in
  `TokenMoved` and is dropped." becomes "carrying every field the command
  gave, a grant's `kind` and a move's `reason` included, and a move that gave
  no `reason` records none." The Requirements line swaps VTT-257 for A and
  adds B.
- **SPEC-016:** Status adds `forwardable`. "for `forwarded`, those frames and
  then the event" ends "... and then `forwardable`'s answer: the event itself,
  or, for a `TokenMoved` whose `reason` is not empty, a copy of it with the
  `reason` cleared and every other field as the event has it." The "every frame
  it builds is new" parenthesis adds "and `forwardable`'s `proto.Clone`". The
  `TokenMoved` ruling gains "What it forwards carries no `reason`, which its
  issuer wrote and the DM and the agent read in the log." Consequences gain "A
  player's or a spectator's `TokenMoved` never carries a `reason`; the DM and
  the agent are sent it as the log holds it." Requirements add C.
- **SPEC-007:** not edited (Q7).

**D9. The sort.** Forced by SPEC-008 and the `requirements` skill; lettered so
nothing reads as an id.

| # | Rule (what) | Proposed | The observation that goes red | Test | Record |
|---|---|---|---|---|---|
| A | The event a one-envelope command becomes carries every field the command gave, a grant's kind and a move's reason included. | accept, VTT-257's successor (D10); holds the ticket's first rule | `ToEvent` without its `Reason` line; any arm dropping a field | VTT-257's fourteen entries, `TestToEventMoveTokenProducesTokenMoved` now with a reason, `server_test.go#TestAMoveWithAReasonAppendsItsReason` (new) | SPEC-013 |
| B | A `move_token` that gives no reason appends a `TokenMoved` with none. | accept, the second rule; a default has a precedent in `ConditionRemoved`'s `"manual"` | P10 | `server_test.go#TestAMoveWithNoReasonAppendsNone` (new), `cmd/vtt/scenario_goldens_test.go#TestScenarioGoldenStreamsHaveNotDrifted` | SPEC-013 |
| C | A player or spectator is sent no move's reason. | accept, the third rule under Q1's (a) | K1 | `project_test.go#TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` (new; also cites VTT-222), `server_visibility_test.go#TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt` (new; also cites VTT-176), `project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` | SPEC-016 |
| D | The feed and the ticker label a move with its reason when its frame carries one. | accept, the fourth rule | `describe` ignoring the reason (P8) | `client/test/spectator-view.test.ts#the feed and the ticker show a move's reason` (new) | none: no record describes the client (Gap 6) |

**Refused, one line each:** (1) "a move's reason reaches the event the move
appends", as its own row — A holds it and its wire test cites A; (2) "the DM
and the agent are sent the reason" — VTT-176, which the wire test cites, and
in `Project` the unit test's DM half; (3) "projecting a reasoned move leaves
the event unchanged" — VTT-222, which the unit test cites; (4) "a forwarded
move is the event less its reason and nothing else" — how C is achieved, and
the unit test's equality assertion (K3); (5) "a player may give a reason" — the
absence of a rule (Q2); (6) "the contract change is additive" — rule 3; (7) "a
move without a reason is labelled as before" — D's table row; (8) "a
correction frame carries no reason" — VTT-249; (9) "the issuer's own seat sees
its reason" — not adopted (Q1, c1). Four accepted, one withdrawn, nine refused.

**D10. VTT-257 withdrawn, A its successor, per Q4.** Forced by Done item 5
and the removal report's deviation: the qualifier "that the event has a field
for" was written to exempt `move_token`'s reason, and once `TokenMoved` carries
it the qualifier exempts nothing, so a command field added later with no
event field would satisfy the register. VTT-257's text is not changed: its
cell becomes `WITHDRAWN 2026-10-03: <its text>. Superseded, not false: its
qualifier exempted move_token's reason, which TokenMoved now carries.
Succeeded by VTT-NNN.`, its evidence `**READING — verify-ticket check 5 of
2026-10-03; withdrawn, held by nothing**`. Its fourteen citing lines (twelve
in `convert_test.go`, `TestDMGrantsControlOverTheWire`'s, and
`TestScenarioGoldenStreamsHaveNotDrifted`'s `// VTT-240 VTT-257`) swap the id
for A's, as VTT-207's were.

**D11. The debt entry.** Forced by Done item 5 and the `1724a69` form. The
entry moves verbatim from "Open debt" to a new dated section after the
2026-09-30 one, headed "2026-10-02 — `move_token`'s `reason` never reached the
log", and gains a closing line: "**Closed by**
`TestToEventMoveTokenProducesTokenMoved` in `internal/gateway/convert_test.go`
and `TestAMoveWithAReasonAppendsItsReason` in `internal/gateway/server_test.go`,
which red when `ToEvent` drops the reason (observed <date>)."

**D12. Comments and the ledger.** Forced by the measured shares. Two doc lines
for `forwardable`, one for `because`, none in `convert.go` or `model.go`; test
docs at most three lines. `--report` before each commit; `--write-ledger` only
if a share fell more than 1.0 under its row, which the probe tree did not.

**D13. Mutation keys: none should move; the self-tests decide, last.** After
the review settles and before each commit, `python3
tools/check_mutation_test.py -q` and `python3 tools/check_ts_mutation_test.py
-q`. If a review adds a line above `Project`'s forward or above `describe`'s
arm, all three `project.go` keys or both `spectator.ts` keys move by that
count; each is re-pointed by reading the statement its mutator rewrites at the
new line, never by offset. A red on anything else means a key this plan says
does not move does: stop. New code: `forwardable`'s condition negated returns
a reasoned move unstripped (C's tests) and derefs a nil move for every other
forwarded event; `because`'s mutants are killed by the two exact labels.

**D14. Two code commits and the report, per Q8.** Forced by the measured
leak and by the field's own warning. The field, the conversion and the
projection land together: with the field and `ToEvent` alone a player is sent
the reason (P3), and the field alone is a field nothing writes, the defect
this ticket closes. The client is its own component, with its own QA and its
own bundle rebuild. C1: the contract, the gateway, eventgen, the records, rows
A to C, the withdrawal, the debt entry, the rebuilt webdist. C2: the client,
row D, the rebuilt webdist. C3: the report. Each is green alone: C1's tree is
the probe tree less D7, which no Go test and no TS test other than D's reads.

**D15. Phase 4a per code commit.** Forced by the dev-cycle skill. One QA agent
per `qa-prompt.md`, never given the diff, the source, the existing tests or the
implementer's report. C1: rows A, B, C, VTT-176, VTT-206, VTT-222, SPEC-013
and SPEC-016 whole as C1 leaves them, the text of `TokenMoved` and
`MoveTokenRequest` from the protos, and `go doc ./internal/gateway`; QA writes
`internal/gateway/qa_move_reason_test.go`. C2: row D, the ticket's problem
paragraph and fourth rule (no record describes the client), `grep -n
'^export' client/src/view/spectator.ts` and the generated `TokenMovedSchema`;
QA writes `client/test/qa-move-reason.test.ts`. Adjudications go in the report
under their own heading; an escape goes to `docs/verification-debt.md` as a
recipe.

**D16. Phase 4b per code commit, after 4a.** One reviewer at high effort,
briefed to verify by command every sentence the commit adds (SPEC-013,
SPEC-016, both proto comments, `forwardable`'s and `because`'s docs, the debt
closure, VTT-257's withdrawn cell) against the code; that `git grep -n
VTT-257` outside `docs/` prints nothing; the keys; the break lines in the
draft message. If the reviewer dies on a model's limit, say so and re-dispatch
the same brief on `fable`.

**D17. The deliberate breaks, one per check relied on.** In a scratch clone of
each commit's final tree, each gate first clean, one edit per break, the
inverse edit by hand:

| Commit | Check | Break | Expected red |
|---|---|---|---|
| C1 | D2's fixture | `"reason"` spelled `"reasn"` | `TestTokenMovedReasonEnvelopeRoundTrip` and its TS case (P9) |
| C1 | A | `ToEvent`'s `Reason` line removed | `TestToEventMoveTokenProducesTokenMoved`, `TestAMoveWithAReasonAppendsItsReason` (P2) |
| C1 | B | `ToEvent` writes `"moved" + GetReason()` | `TestAMoveWithNoReasonAppendsNone`, four golden subtests (P10) |
| C1 | C | `forwardable` returns `env` | the unit, wire and property tests (K1) |
| C1 | VTT-222 on this path | `forwardable` clears `env` in place | the unit test alone (K2) |
| C1 | C's equality | the copy also clears `EventId` | the unit test (K3) |
| C1 | the DM arm | `Project`'s DM arm returns `forwardable(env)` | the unit test's DM half (K4) |
| C1 | D6's floor | `moveReasons` becomes `{""}` | the property test's floor |
| C2 | D | the arm drops `because` | the render test (P8) |
| C2 | D's bare label | `because` always appends | the `describe` table's reason-less row |

**D18. Disk and load.** Forced by `check:mutation`'s 16 GiB floor. `go clean
-cache`, `df -k` and `uptime` immediately before `task check`, launched once
after C2 in its own session (`start_new_session=True`) on a settled tree;
below 16 GiB, stop.

## Tasks, in dependency order

### Task 0 — Baselines

`df -k`, `uptime`; `check-requirements-chain.py .`, `check-comments.py main`,
`check-doc-owner.py .`, both mutation self-tests; the three `grep -c` of check
1; `requirement-id` on the path. **Done when:** the outputs match Measurements.

### Task 1 — The contract

**Files:** `contract/vtt/v1/events.proto`, `contract/gen/`,
`cmd/vtt/webdist/`, `contract/testdata/token_moved_reason_envelope.json` (new),
`contract/roundtrip_test.go`, `contract/events.test.ts`.

The fixture and both round trips first, red (P9); then D1 and the
regeneration. **Done when:** both round trips green; `go build ./...` clean;
`check:breaking` prints its header and nothing else; `git status` shows
`tools.json` unchanged.

### Task 2 — Rows

**Files:** `docs/requirements.md`. After sign-off, `requirement-id` once each
for A, B and C, in that order. **Done when:** three new OPEN rows.

### Task 3 — The conversion

**Files:** `internal/gateway/convert.go`, `convert_test.go`, `server_test.go`.

D3, tests first: the changed ToEvent test and A's wire test red, B's green;
then the copy. **Done when:** all three green; `go test -count=1
./internal/gateway/ ./cmd/vtt/` green.

### Task 4 — The projection

**Files:** `internal/gateway/project.go`, `project_test.go`,
`server_visibility_test.go`.

D4, tests first, both red against Task 3's tree. **Done when:** green; the
three keys read the same statements.

### Task 5 — The walk

**Files:** `internal/eventgen/model.go`, `project_property_test.go`. D6.
**Done when:** the four packages of P5 green and the floor met.

### Task 6 — The records

**Files:** SPEC-013, SPEC-016, `contract/vtt/v1/commands.proto` and its
regeneration, `docs/verification-debt.md`, `docs/requirements.md` (VTT-257's
cell, the evidence of A to C), the fourteen citation lines.

D5, D8, D10, D11. **Done when:** `grep -c 'is dropped'` on SPEC-013 prints 0;
`git grep -n VTT-257 -- internal cmd` prints nothing; the chain prints `262
rows, 206 test files, 11 specifications` with no OPEN row among A to C.

### Task 7 — Local gates, C1

`gofmt`, `go vet`, `task lint`, `go test -count=1 ./internal/... ./contract/...
./cmd/...`, `bun test client/test contract contract-spike`, `task
check:comments`, `check:doc-owner`, `check:new-prose`. **Done when:** each
prints its completion line.

### Task 8 — Phase 4a, then 4b, for C1

D15, D16. Findings fixed and the affected task's "done" re-run; the review
settles before Task 9.

### Task 9 — Keys, ledger, commit C1

D12, D13; then C1's message, which lists the ids, D17's C1 lines and the
withdrawal. **Done when:** both self-tests `OK`; `git show --stat HEAD` lists
C1's files; `task check:drift` clean on the committed tree.

### Task 10 — The client

**Files:** `client/src/view/spectator.ts`, `client/test/spectator-view.test.ts`,
`cmd/vtt/webdist/`, `docs/requirements.md` (D, dispensed here).

D7, the test first, red. **Done when:** green; `client:typecheck` clean; both
`spectator.ts` keys read the same statements.

### Task 11 — Phase 4a, 4b, keys and commit C2

As Tasks 7 to 9 for C2. **Done when:** `git show --stat HEAD` lists C2's
files; `check:drift` clean.

### Task 12 — Breaks and the whole gate

D17 in a scratch clone, deleted afterwards; then D18 and `task check` once.
**Done when:** each break gives its red; `task check` exits 0 with every
step's own verdict, `check:mutation` included.

### Task 13 — The report

`docs/reports/2026-10-03-a-moves-reason-reaches-the-log.md`, per the
`implementation-report` skill: each Done item with its observation, the rows
and refusals, the rulings taken at sign-off, the rule-9 answer, the breaks,
the gaps below. **Done when:** C3 holds it alone.

## Commits

| Commit | Carries | Gate steps |
|---|---|---|
| C1 | D14's list | Tasks 7 to 9, then the pre-commit hook; `check:drift` after |
| C2 | the client, row D, webdist | Task 11, the hook; `check:drift` after |
| C3 | the report | the hook |

`task check` whole once, after C2. Push after C3; pre-push takes about three
minutes, let it finish.

## Gaps that travel with this plan

1. **Done item 2 waits on the owner's ruling** (Q1); the plan proposes (a).
2. **`TestTheDMReceivesEverythingUnchanged`, which Done item 2 relies on,
   projects a `TokenPlaced`** and cites no row; it cannot see a reason. C's
   unit test holds `Project`'s DM arm (K4); the DM's seat on the wire has no
   projector, so no wire test reaches that arm.
3. **Only C's unit test holds VTT-222 on this path** (K2): the wire test
   cannot see a reason cleared in place.
4. **No golden gives a move a reason**, by Done item 1, so the keystone and the
   projected goldens do not hold C; the property walk does, on two seeds of six
   (P6).
5. **A forwarded move still carries its author** (`ParticipantId`,
   `ActorRole`), the other half of the 2026-10-01 open question (Q3).
6. **No record describes the client**, so row D has no specification line.
7. **A reason is bounded only by `maxWSFrameBytes`** (32768), where narration
   and note text have the fold's `maxTextBytes`. Not this ticket's.
8. **The MCP `move_token` tool's `reason` has no description** saying who reads
   it; `tools.json` is unchanged. A later ticket if wanted.
9. **SPEC-007 is named by the ticket and not moved** (Q7); the files of check
   4 are not in the ticket's list.

## Questions for sign-off

1. **What is a player or spectator sent of a move that carries a reason?** (a)
   the move without its reason, a `proto.Clone` with the reason cleared; (b)
   the reason as is; (c1) the issuer's own seat keeps its reason, every other
   seat is sent none; (c2) a visibility on the reason, chosen by the issuer.
   Recommend (a): the reason is free text that can name what the viewer does
   not see, which the ruling of 2026-09-29 forbids; (b) leaks it (P3); (c1)
   adds a viewer-dependent branch for a case no shipped client produces, since
   `view/player.ts` sends no reason; (c2) is new contract and UI for no asked
   need, and (a) is where it would start from.
2. **May a player give a reason at all?** (i) yes, recorded for the DM and the
   agent and sent to no other seat, with no new rule; (ii) refuse a player's
   `move_token` that carries one, a new `playerRules` clause in SPEC-013 with
   a row; (iii) drop it silently. Recommend (i), with `MoveTokenRequest`'s
   comment rewritten (D5): under (a) nothing leaks, a player may already write
   to the table through `add_narration`, and (iii) is the defect this ticket
   closes.
3. **Leave the forwarded `ResourceChanged`'s reason and author, and every
   forwarded frame's author, a move's included, to the ticket made of them on
   2026-10-01?** Recommend yes: this ruling covers free text an issuer wrote on
   a move; `ResourceChanged.reason` is written by the server and names an
   ability, and the author is the open question's other half. No other payload
   changes here.
4. **Withdraw VTT-257 and dispense A as its successor, re-pointing its
   fourteen citations?** Recommend yes (D10); the alternative, keeping VTT-257
   as worded, leaves a qualifier that exempts nothing and would let a later
   command field go unrecorded unnoticed.
5. **Accept row B, a move with no reason records none?** Recommend yes: a
   default reason is a plausible edit (`ConditionRemoved`'s `"manual"`), and
   four subtests of `TestScenarioGoldenStreamsHaveNotDrifted` already red on
   it (P10).
6. **Should eventgen give alternate moves a reason, and the property walk fail
   on any reasoned move a player is sent, with a floor of two seeds?**
   Recommend yes (D6): it holds C across every transition at no change to the
   walk; the floor is at its measured value, so an eventgen change that stops
   forwarding reasoned moves reds it.
7. **Leave SPEC-007 unedited, though the ticket names it?** Recommend yes: no
   sentence of it becomes false and none is missing; who is sent the field is
   SPEC-016's, and the field's carriage SPEC-013's.
8. **Two code commits and the report (D14)?** Recommend yes: the field, its
   writer and its redaction cannot land apart without leaking (P3) or adding
   a field nothing writes, and the client is a separate component with its
   own QA.
