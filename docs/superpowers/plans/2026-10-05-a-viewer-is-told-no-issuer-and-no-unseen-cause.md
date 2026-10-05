# A viewer is told no issuer and no unseen cause — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-05-a-viewer-is-told-no-issuer-and-no-unseen-cause-design.md`
**Verified:** 2026-10-05, by `verify-ticket`, an agent that did not write the
ticket, against `e628e18` on `feat/viewer-told-no-issuer` (equal to `main`; the
ticket untracked). Verdict: **Passes with gaps.** The gaps are listed at the
end and travel with this plan. This plan does not edit the ticket. Where the
ticket is wrong or thin, the plan says so under "Defects the writer should
revise", then decides around it.

**Goal, in the ticket's words:** a player or a spectator is sent no frame that
carries the participant or the role that issued its event, and no status
change's cause it did not see. What it is sent of a cause it did see follows the
owner's ruling at sign-off. The DM and the agent are still sent every event as
the log holds it. The projected goldens change only by the fields removed, and
`task check` whole is green.

**The owner's rulings that bound this plan.**
- 2026-09-29: only what a viewer sees gives it information. SPEC-016
  implements this ruling.
- 2026-10-01: an attack, an ability or a control change naming any actor the
  viewer does not see now is withheld entirely. The record is the rulings
  paragraph of `docs/superpowers/plans/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it.md`.
  The ticket's paraphrase ("sees its hit points drop and not who hit it") is
  not in the tree.
- 2026-10-03, Q3 of the move-reason sign-off: every forwarded frame's author
  goes to this ticket. `docs/reports/2026-10-04-a-moves-reason-reaches-the-log.md`
  records it.

**MapTool (CLAUDE.md rule 9), the answer this plan records.**
- **A status change in MapTool carries no author and no cause.**
  - A token's states, bars and properties change by `PutTokenMsg` (`zone_guid`,
    a whole `TokenDto`) or by `UpdateTokenPropertyMsg` (`zone_guid`,
    `token_guid`, a `TokenUpdateDto` such as `setState` or `setProperty`, and
    its values).
  - `ServerMessageHandler.handleMessage` relays both to every other client by
    `sendToClients`.
  - Neither message names a player or a reason. A MapTool player sees a bar
    drop, and the update says nothing of who or what dropped it.
- **"Who" travels in only two places.**
  - `StartTokenMoveMsg.player_id` and `MovePointerMsg.player`.
  - Chat: `TextMessageDto.source` names the sender of a line. Macro and roll
    output therefore names its roller to every client.
  - `MapTool.addServerMessage` then drops a GM-channel line on a client whose
    player is not a GM.
- **Borrowed:** the status update's shape. A change to an actor carries what
  changed and nothing about who or why. Here that becomes the forwarded
  `ResourceChanged`, `ConditionApplied` and `ConditionRemoved` without
  `reason` or `source`, on an envelope without `participant_id` or
  `actor_role`.
- **Refused, each with its reason:**
  - Chat's broadcast `source` and the client-side channel filter. That is the
    distribution model rule 9 names: here the projection never sends what a
    seat may not read.
  - A per-message audience on the author. No client asks for one, and the
    ruling needs none.
- **Read by the verifier** in `~/dev/RPTool/maptool` at `f4b7fef6c`: the four
  messages, `TokenUpdateDto`, `TextMessageDto`, `handleMessage`'s two relay
  arms and `addServerMessage`'s filter.
- SPEC-016, the register and the code name no other project. This plan and
  the report do.

## Verification, check by check

1. **Every path resolves — by command.**
   - `test -e` finds every file the ticket names, and `ls -d
     scenarios/goldens/*/projections/*/stream.json | wc -l` prints 12.
   - `grep` finds each symbol where the ticket puts it:
     - in `project.go`: `forwardable`, `Projector.Project` and `classify`;
     - `ToEvent`, and its `"manual"`;
     - the five stamping sites: `ToEvent`, and `handleLoadAdventure`,
       `handleLoadMap`, `handleUseAbility` and `handleRemoveActor` in
       `adventure.go`, `map.go`, `ruleset.go` and `server.go`;
     - the presence snapshot's `ParticipantId` and `DisplayName` in
       `presence.go`;
     - `resolve.go`'s `ability:%s:usage`, `ability:%s:%s` and
       `threshold:%s`;
     - `describe` in `spectator.ts`, and `fold.ts`'s `Source: v.source`;
     - `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`.
   - `git grep` over every non-test `.Append(`/`.AppendBatch(` finds those
     five gateway sites and `campaign`'s two pass-throughs, and nothing else.
2. **"Done" is an observation — by command.**
   - Item 1's test does not exist. A sketch (P13) fails today on both the
     `ResourceChanged` and the `ConditionApplied`, for a player and for a
     spectator, and passes after.
   - Item 2's test does not exist. A wire sketch walking the player's, the
     issuer's and a perched spectator's streams (P13) fails today and passes
     after.
   - Item 3 waits on the ruling (Q1). Under the recommended option the same
     wire sketch observes it.
   - Item 4: `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection` passes
     before and after (P8). It cannot fail under the two breaks that item 4 is
     about, though (K2, K10; Gap 3).
   - Item 5: the emitted streams equal a mechanical deletion of the fields
     (P9). `task check` is the plan's last task.
3. **Each rule is breakable — a reading, then probes.** Each break was run in a
   scratch clone (the K table):
   - Rule 1 breaks by keeping either issuer field (K4, K5).
   - Rule 2 breaks by keeping a cause (K6, K7).
   - Rule 3 breaks by clearing the event in place (K2).

   One finding, under Defects: rule 1 as worded is false of correct work.
4. **The scope matches the claim — by command, then a reading.**
   - `Project` has one production caller (`seat.receive`). `forwardable` has
     one (`Project`).
   - Nothing outside tests reads an envelope's `ParticipantId` or `ActorRole`
     (P3), and `client/src` reads neither (P15).
   - The work reaches four test files the ticket does not list:
     `qa_move_reason_test.go`, `qa_note_projection_test.go`,
     `qa_testimony_removal_test.go` and `qa_testimony_sight_test.go`. They hold
     32 of the 34 tests that a correct `forwardable` reds (P7), each through a
     recogniser that knows a forwarded frame by pointer or by
     `proto.Equal(f, env)`.
   - `keystone_test.go`, which the ticket lists, needs no edit: its goldens move
     and its code does not.
   - All of it is under `internal/gateway`, so it stays one component.
5. **No recorded decision is contradicted silently — a reading.**
   - SPEC-016's "the event itself, or, for a `TokenMoved` whose `reason` is not
     empty, a copy" is overturned. The ticket names SPEC-016.
   - The ruling of 2026-10-03 (Q3) sends the author here. The 2026-10-01
     plan's Q6 left a seen cause open, and the ticket leaves it open (item 3).
   - `git grep` over `docs/adr`, `docs/superpowers/specs` and
     `docs/specifications` finds no decision that a viewer is told the issuer.
   - Two QA tests name the overturned sentence as their oracle. They move with
     it (D5).
6. **The records the work moves are named — by command, then a reading.**
   - The section lists SPEC-016, which resolves.
   - The reading: the specifications that mention the issuer are SPEC-009
     ("An envelope carries `actor_role`, the issuer's role ... stamped by the
     gateway"), SPEC-012 and SPEC-013 (the batch handlers and `ToEvent` stamp
     `ParticipantId` and `ActorRole`). SPEC-015 says `Project` answers the DM
     and the agent unchanged.
   - Each of those sentences describes the log's envelope or the DM's seat,
     which do not change, so none becomes false. `None.` would have been
     wrong, and the ticket's one name is right (Q8).
   - `events.proto` is named conditionally (item 4). Q3 rules on it.

### Defects the writer should revise

1. **Rule 1, and Done item 2's wording, are false of correct work.**
   - "A player or a spectator is never sent the participant ... that issued an
     event" and "no frame that names the participant ... that issued an event"
     forbid a participant id wherever it appears.
   - But a participant who issues events is often also a controller. Control
     frames name it as their subject, in `ActorControlGranted.participant_id`:
     forwarded grants, an introduction's grants and a correction's. Both folds
     refuse a grant without that id (`requireControlTarget` in `fold.ts`,
     `controlTarget` in `engine.Apply`).
   - Measured: `toy-brawl/act-patron`'s stream names `p-brawler` on the
     brawler's introduction grant at sequence 8, and `p-brawler` issued
     sequences 9 to 11.
   - The rule is about the envelope's own `participant_id` and `actor_role`.
     Row A says so (D8).
2. **"What it touches" omits four QA files** (check 4). It also lists
   `keystone_test.go`, which needs no edit.
3. **The 2026-10-01 ruling is quoted in words the tree does not hold.** The
   record is the rulings paragraph of the 2026-10-01 plan, quoted above.

## Measurements this plan stands on

All by command, by the verifier, in a scratch clone of `e628e18` (`git clone
--no-hardlinks`) and two worktrees of it, never in the working tree. The
sketches are the verifier's and were discarded, and the clone was deleted.
"M1" is a `forwardable` that clones every forwarded frame. "M2" clones only when
a field to clear is set (D1).

| # | Tree | Result |
|---|---|---|
| P1 | base, a `Project` probe: the goblin behind `twoRooms`' shut door uses `claw` on the hero, a batch stamped `p-agent`/`agent` | for a player and for a spectator on `hero`: the `AbilityUsed` is withheld. The `ResourceChanged` arrives with `reason` `ability:claw:hit` and `participant_id` `p-agent`, `actor_role` `agent`. So does the `ConditionApplied`, with `source` `ability:claw:hit` |
| P2 | base, the real path: `newRulesetFixture`, the brawler's `use_ability` `fists` on the patron | the patron's player is sent seq 9 to 11 stamped with the brawler's server-assigned id and `player`, `reason` `ability:fists:hit` and `source` `threshold:drink` |
| P3 | `git grep` for `ParticipantId`/`ActorRole` over non-test Go, and for the protojson spellings | readers are control events' payload (`engine.Apply`), commands (`authz.go`, `handlePromotion`) and writers (`convert.go`, the four batch handlers, `presence.go`, `project.go`'s built grants, `eventgen`, `harness/soak.go`). No reader of an envelope's issuer |
| P4 | `git grep` for `Reason:`/`Source:` writers | `rules.Resolve` (`resourceChangedEnvelope`, `conditionAppliedEnvelope`, `conditionRemovedEnvelope`, fed `ability:<id>:usage`, `ability:<id>:<phase>`, `threshold:<resource>`) and `ToEvent`'s `"manual"`. `eventgen` writes `"prop"`. No production code writes an `AttackRolled`, so `Modifier.source` is unwritten |
| P5 | the 12 projected streams, parsed | 61 forwarded frames (an event id) and all carry `participantId` and `actorRole`. 7 carry a cause: `adventure-night/act-fighter` 1 (`ability:rally:effect`), and `toy-brawl/act-brawler` and `act-patron` 3 each (`ability:fists:hit`, `threshold:drink`, `manual`). In every one of the 7 the viewer was forwarded the batch's `AbilityUsed` or saw the actor, so no golden holds an unseen cause |
| P6 | the 9 log-level streams | 3 `ability:` causes, each directly after its batch's `AbilityUsed` naming the same ability with the same issuer. 2 `threshold:`, 1 `manual` |
| P7 | M2, `go test ./internal/gateway/` | 34 top-level tests red. `keystone_test.go` 1 (12 golden subtests), `project_test.go` 1 (`TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt`), `qa_move_reason_test.go` 8, `qa_note_projection_test.go` 2, `qa_testimony_removal_test.go` 13, `qa_testimony_sight_test.go` 9. M1 reds the same plus `TestAnEventNamingAnUnknownActorIsWithheld`, whose bare envelope M2 passes through |
| P8 | M1 with every recogniser pointed at the event less the six fields (D5) and the emitted goldens | `go test ./internal/gateway/` green (about 43 s). The 34 hid no other failure |
| P9 | M2's `projectWholeLog` over each seat, written by `marshalStream`, against a Python deletion of `participantId`, `actorRole` and the three causes from the committed streams | 12 of 12 equal. Golden diff 133 lines out and 4 in: 122 issuer lines, 7 cause lines, and 4 trailing commas that move. No `state.json` or `viewer.json` changes, and none of the 9 log-level `stream.json`. `bun test client/test/projection-parity.test.ts` 15 pass |
| P10 | every Go package but `internal/gateway`, on M2 and on M1 | 20 packages `ok`, both trees. `bun test client/test contract contract-spike` on M1: 873 pass, 0 fail |
| P11 | `BenchmarkClone` against `BenchmarkEncodeFrame` | `proto.Clone` 0.32 to 0.44 µs, 4 to 8 allocs. `EncodeFrame` of the same frame 2.2 to 11.8 µs, which every frame already pays per connection, and `seat.receive` refolds the seat's prefix per event |
| P12 | a sketch of Q1 (c)'s premise, by reading | the projection can tell a seen ability cause only by remembering this seat's last `AbilityUsed` verdict and by parsing `rules`' reason format. It also needs `Resolve` putting `AbilityUsed` first in its batch and `campaign.AppendBatch` keeping the batch contiguous. Both facts are true in code and stated in no specification |
| P13 | the three new tests (D4) | the unit and wire tests red on the base, as P1 and P2, and green on M1. The guard is red on the base (`forwardable keeps "actor_role"`, `"participant_id"`) and green on M1 |
| P14 | M1 plus the property clause and the walk's stamped `ParticipantId` | green. Status changes whose event carried a cause, forwarded to a player, per seed 1 to 6: 0, 4, 8, 2, 13, 3. Every forwarded frame's event carries an issuer. On the base the clause reds seed 1 at action 1 and seed 2 at action 0 |
| P15 | `git grep` over `client/src` | the client reads `env.sessionId` (`fold.ts`'s `sessionStarted` arm), `payload` and `sequence`, and no envelope `participantId`, `actorRole` or `eventId`. `app.ts`'s `participantId` is a presence event's. `fold.ts` writes a condition's `Source`, and nothing in `client/src` reads it |
| P16 | a comment-only edit of `events.proto` (warnings on `actor_role` and `ResourceChanged.reason`), then `go tool buf generate`, toolgen, `task build:client` | `events.pb.go` (comments, and gofmt realigning two structs) and `events_pb.ts` change. `tools.json` and `cmd/vtt/webdist` are byte-identical, and `go build ./...` is clean |
| P17 | M1 with the new tests placed in their files, each with a one-line doc and a citation line | `golangci-lint` 2.11.4 `0 issues`. `go vet` and `gofmt` clean. `check-doc-owner.py` "80 files". Both mutation self-tests `OK`. `check-comments.py main` names two shares to record (below) |
| P18 | the three new tests' failure under each break | the K table |

**What breaks what.** Each is a single edit on M1's final tree, restored after,
with `go test -count=1 ./internal/gateway/` whole.

| # | Edit | Red, besides the QA recognisers |
|---|---|---|
| K1 | `forwardable` returns `env` | 34 QA tests. The unit, wire, guard and property tests, `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt`, `TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt`, all 12 golden subtests |
| K2 | `forwardable` clears `env` in place | 28 QA tests. `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` (its VTT-222 half), the property walk (the DM tripwire), `TestThreeRoleExitScenarioOverLiveWebSockets`. **Not** VTT-176's wire test, and no golden |
| K3 | the copy also clears `EventId` | 32 QA tests. The unit, wire, guard and property tests, `TestAPlayersOwnTokenlessCharacterIsHeardOf`, `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`, all 12 goldens |
| K4 | the copy keeps `ActorRole` | 32 QA tests. The unit, wire, guard and property tests, all 12 goldens |
| K5 | the copy keeps `ParticipantId` | as K4 |
| K6 | the copy keeps every status cause (Q1 b) | 16 QA tests. The unit, wire and property tests, 3 goldens (`adventure-night/act-fighter`, both `toy-brawl`) |
| K7 | the copy keeps `ConditionRemoved.reason` only | 4 QA tests. The wire and property tests, both `toy-brawl` goldens. The unit test does not see it |
| K8 | the copy keeps a move's reason | 6 QA tests. VTT-262's `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` and `TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt`, the property walk |
| K9 | the copy also clears `SessionId` | 26 QA tests. The unit and guard tests, 11 goldens (their `state.json` sessions), `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` |
| K10 | `Project`'s DM arm returns `forwardable(env)` | 39 QA tests. `TestTheDMReceivesEverythingUnchanged`, `TestTheAgentSeatReceivesEverythingUnchangedToo`, the keystone's identity arm, `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt`. **Not** VTT-176's wire test: the DM's seat has no projector |

**Comment shares** (`check-comments.py --report`, share / ceiling, at the base
then on M1):

| File | Base | M1 |
|---|---|---|
| `project.go` | 18.6 / 18.7 | 18.4 |
| `project_test.go` | 25.5 / 25.6 | 25.2 |
| `project_property_test.go` | 20.0 / 20.1 | 19.3 |
| `project_internal_test.go` | 33.3 / 33.4 | 20.0 |
| `server_visibility_test.go` | 27.8 / 27.9 | 26.6 |
| `qa_move_reason_test.go` | 0.5 / 0.6 | 0.5 |
| `qa_note_projection_test.go` | 1.0 / 1.1 | 1.0 |
| `qa_testimony_removal_test.go` | 0.0 / 0.0 | 0.0 |
| `qa_testimony_sight_test.go` | 0.0 / 0.0 | 0.0 |

The two shares that fall more than 1.0 below their rows,
`project_internal_test.go` and `server_visibility_test.go`, make
`check-comments.py main` ask for `--write-ledger`. That is the
`tools/comment-ceilings.txt` change the ticket names.

**Mutation keys.**
- `internal/gateway` is in `PACKAGES`. `tools/mutation-equivalents.txt` holds
  three `project.go` entries: the two loop bounds of `objectInSight`'s
  footprint walk (`y < sc.GridHeight`, `x < sc.GridWidth`) and the
  `len(a)+len(b)` of `sortedSceneIDsUnion`'s `seen` map.
- `forwardable` is the file's last function, below all three. On M1 the three
  lines are byte-identical to the base's.
- The new `forwardable` holds no token any configured mutator rewrites (no
  comparison, arithmetic, increment or logical operator, counted by `grep`).
  The base's holds one `==`, which goes with it. So the change adds no mutant
  and removes one killed one.
- No other gated file changes.

**Records and gates at the base.**
- `check-requirements-chain.py .`: `271 rows, 215 test files, 13
  specifications`.
- `check-comments.py main`: `258 files, 0 added comment lines, 258 ledger
  rows; clean`, with a notice that `cmd/vtt/library_test.go` sits at 44.88
  over its 44.8 with no line added. That file is not this ticket's.
- `check-doc-owner.py .`: `80 files`. Both mutation self-tests `OK`.
- `requirement-id` is not on the path. It is at
  `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/requirement-id`.
- `docs/verification-debt.md`'s open entry "Nothing observes a removal batch's
  participant and role" stays open (Gap 5).
- Free space on `/System/Volumes/Data` read 31,313,284 then 30,286,396 KiB
  (29.9 and 28.9 GiB), with the Go cache at 5.7 GiB. Load averages ran 1.9 to
  7.4, the peak from work outside this verification.

## Constraints that bind every task

- `CLAUDE.md` rule 2: no gate is weakened.
  - A recogniser is re-pointed to what a correct frame is, never loosened to
    accept anything (D5).
  - The ledger only goes down, through `--write-ledger`.
  - A floor sits at its measured value.
- Rule 3: no contract field is added, removed or renumbered. Only comments
  change in `events.proto` (Q3).
- Rule 4: one fold. `engine.Apply` and `fold.ts` are not touched, and
  `forwardable` writes only to its own copy.
- Rule 5: "reason", "source", "issuer" and "cause" are platform vocabulary.
  The tests' `claw` and `bleeding` are fixture strings, as `twoRooms`' `goblin`
  is.
- Rule 8: this plan, the records and the report cite symbols and tests, never
  a line. The adjudication file keeps its coordinates.
- Rule 9: the answer above is recorded before any task runs.
- Rule 10 and SPEC-010: a code comment is a warning, a pointer or an exported
  symbol's one-line doc. Proto comments warn (SPEC-007). The two
  `qa_testimony_*` files sit at a 0.0 ceiling, so their edits add no comment
  line.
- SPEC-008: ids come from `requirement-id` only, after sign-off.
- SPEC-016's "Nothing may write to an envelope a seat is handed" and VTT-222.
- `check:drift` passes only on a committed tree. The order is review, commit,
  gate, and a commit that edits `events.proto` carries its regeneration.
- The review package is `git diff HEAD`. Nothing is stashed or checked out
  while a reviewer reads.
- `git add` and `git commit` run in separate calls, with `git show --stat
  HEAD` after each.
- A new test goes after a closing brace, never between another test's doc
  block and its `func`.
- The ticket and every report under `docs/reports/` are not edited.

## Decisions this plan makes

**D1. Clone every forwarded frame (M1).** Forced by VTT-222, which forbids
clearing `env`, and by P7, which shows M2 buys nothing: every gateway append
stamps the issuer (P3), so M2's pass-through is reachable only by a test's bare
envelope.

A new body for the function at the end of `project.go`, with `Project`
unchanged (its forward line already calls `forwardable`):

    // forwardable is a copy of env without its issuer or any cause (SPEC-016).
    // Clone before clearing: every seat and the DM share env.
    func forwardable(env *vttv1.Envelope) *vttv1.Envelope {
        c := proto.Clone(env).(*vttv1.Envelope)
        c.ParticipantId = ""
        c.ActorRole = ""
        switch p := c.GetPayload().(type) {
        case *vttv1.Envelope_TokenMoved:
            p.TokenMoved.Reason = ""
        case *vttv1.Envelope_ResourceChanged:
            p.ResourceChanged.Reason = ""
        case *vttv1.Envelope_ConditionApplied:
            p.ConditionApplied.Source = ""
        case *vttv1.Envelope_ConditionRemoved:
            p.ConditionRemoved.Reason = ""
        }
        return c
    }

Why this is safe and cheap:
- It moves no key (P17), and P11 measures its cost.
- `seat.receive` is safe with a copy. `s.received` keeps the original, which
  `campaign.FoldPrefix` folds, and `pastResume` filters on `GetSequence`,
  which the copy keeps.
- The DM's and the agent's arm still returns `env` (K10).

The alternative is a copy built from the kept fields with the payload shared.
It is refused. It would make SPEC-016's "every frame it builds is new" false,
and D4's guard gives a hand-built allowlist's fail-closed property to this
denylist anyway.

**D2. The cause: option (a), per Q1.** Forced by P12 and by the client (P15).
Every `ResourceChanged.reason`, `ConditionApplied.source` and
`ConditionRemoved.reason` a player or spectator is sent is empty. It is the only
option that needs no projector state, no parsing of `rules`' reason format and
no reliance on `Resolve`'s batch order. It also makes a player's fold uniform:
VTT-249 already sends introductions and corrections with no source. Under
(c) or (d), D1's switch and row B change as Q1 says.

**D3. The fields, ruled one by one.** Forced by ticket item 4 and the brief's
list. The guard of D4 holds the envelope half.

| Field | Ruling | Why |
|---|---|---|
| `Envelope.participant_id`, `actor_role` | cleared | the issuer. Presence maps an id to a name (SPEC-011) |
| `ResourceChanged.reason`, `ConditionApplied.source`, `ConditionRemoved.reason` | cleared (Q1) | names the ability, threshold or hand behind a change (P1) |
| `TokenMoved.reason` | cleared, as today | VTT-262 |
| `event_id` | kept | random hex from `newEventID`, naming nothing. SPEC-016's consequence "a frame with no event id is the projection's own" is how a client, and D5's recognisers, tell a forwarded frame (K3 reds 32 QA tests and 12 goldens) |
| `sequence` | kept | the fold and the resume cursor (SPEC-015) |
| `session_id` | kept | `fold.ts` takes a session's `ID` from it, and the projected `state.json` hold it (K9 reds 11 goldens). Every viewer is sent the `SessionStarted` it names |
| `occurred_at` | kept | the time the viewer receives the frame anyway. A batch shares one, so a forwarded change's time equals a withheld `AbilityUsed`'s, which names nothing (Gap 6). Goldens omit it |
| `NarrationAdded.as` | kept | ruling 1(a) of 2026-09-29: narration, voice included, is addressed to the table |
| `AttackRolled` and `AbilityUsed` fields | kept | forwarded only when the viewer saw every actor they name (VTT-244), so `outcome_summary`'s target names are seen. `Modifier.source` is unwritten (P4, Gap 7) |
| `ActorControlGranted`/`Revoked.participant_id` | kept | the subject, not the issuer. Both folds refuse a grant without it, and introductions and corrections already send it |

**D4. The tests.** Forced by Done items 1 to 3 and by the K table.

- `TestAChangeByAnUnseenUserNamesNoAbilityAndNoIssuer` in `project_test.go`,
  after the last test's closing brace (Done item 1, P13).
  - Over `testimonyLog`, so the goblin stays behind the shut door and the rogue
    in the west room (no `rogueLeaves`). The goblin's `AbilityUsed` on the
    rogue is followed by a `ResourceChanged` on the rogue's `pool` and a
    `ConditionApplied`, both `ability:claw:hit`.
  - Viewers: `p-2` (the rogue's player) and a spectator perched on `rogue`.
  - The ability is withheld. Each forwarded frame equals the event less its
    cause, issuer and role, with its event id and session kept (K3, K9).
  - Cites rows A, B, VTT-222 and VTT-243.
- `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause` in
  `server_visibility_test.go`, over `newRulesetFixture` (Done items 2 and 3,
  P13).
  - The spectator perches on `patron`. The brawler uses `fists` on the patron,
    and the DM removes `dazed-by-ale`.
  - The patron's, the brawler's and the spectator's streams each hold three
    forwarded status changes and no frame with an issuer or a cause. The
    brawler is the issuer's own seat.
  - The DM's stream carries an issuer on every event after the seed and all
    three causes.
  - Fields are read, never searched as text: the brawler's id appears in the
    patron's stream legitimately, as a grant's subject (Defect 1).
  - Cites A, B and VTT-176.
- `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable` in
  `project_internal_test.go`, per Q6.
  - Walks the `Envelope` descriptor's non-oneof fields against a kept table
    (`event_id`, `sequence`, `occurred_at`, `session_id`) and a cleared table
    (`actor_role`, `participant_id`).
  - A field in neither table fails as "no ruling". Each field is set, and
    `forwardable` must keep or drop it as its table says.
  - It mirrors `TestEveryEnvelopePayloadArmHasAnExplicitRuling`, and a contract
    field added later reds it.
  - Cites A.
- In `TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`, per Q5:
  - a player frame with an issuer or a cause fails the walk;
  - the walk stamps `ParticipantId` `prop-issuer` on each event before
    projecting it (eventgen stamps `ActorRole` and no participant, and the
    stamp draws nothing);
  - a floor requires five of six seeds to forward a player a status change
    whose event carried a cause (P14).
  - Cites A and B.

**D5. The recognisers move to what a forwarded frame now is.** Forced by P7
and P8 and by rule 2: no test is deleted, and none is made to accept less.

- `project_test.go` gains `forwardedOf(env)`, the event less the six fields,
  written from D3's table and not from `forwardable`. Then:
  - `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt`'s want becomes
    `forwardedOf(in)`.
  - `TestAnEventNamingAnUnknownActorIsWithheld`'s `e == known` becomes
    `proto.Equal(e, forwardedOf(known))`.
- `qa_testimony_removal_test.go`:
  - `checkFrames`' `fwd` becomes `proto.Equal(f, forwardedOf(env))`;
  - `rmDescribe`'s mark accepts `env` or `forwardedOf(env)`, since a DM seat is
    handed the event.
- `qa_testimony_sight_test.go`: the recogniser becomes `f.GetEventId() != "" &&
  proto.Equal(f, forwardedOf(env))`. The `f == env` arm goes, since no
  forwarded frame is the event.
- `qa_note_projection_test.go`: the two projected-seat equalities compare
  against `forwardedOf(env)`.
- `qa_move_reason_test.go`:
  - `qaMRWithoutReason` is replaced by `forwardedOf`.
  - `TestQAMoveReasonOverTheWireAMoveWithNoReasonAppendsNone` wants
    `forwardedOf(logged)` for the player and the spectator.
  - Two tests whose names and messages state the overturned sentence are
    renamed and re-asserted. `...AMoveWithNoReasonIsForwardedAsTheEventItself`
    becomes `...IsForwardedLessItsIssuer`.
    `...AMoveWithNoReasonIsTheSamePointerForAPlayer` becomes
    `...IsACopyForAPlayer`, asserting `got != env` and equality with
    `forwardedOf(env)`.
  - Their two-line SPEC-016 quotation becomes a pointer of at most two lines.
- No register row or record cites the two renamed tests (`grep` over
  `docs/requirements.md`, `docs/reports` and `docs/specifications`).
- Every recogniser still demands an event id for a forwarded frame, and the
  "projection's own frame carries no metadata" checks stand.

**D6. The goldens are re-derived, twice.** Forced by Done item 5 and the corpus
README (a projected stream is derived).

- The 12 `projections/*/stream.json` are copied from
  `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`' emitted stream.
- Before commit they are held to P9's independent derivation: a Python
  deletion of the six fields from the committed streams. The two must be
  byte-equal after `marshalStream`'s formatting.
- No `state.json`, `viewer.json` or log-level `stream.json` moves. No
  `viewer.json` "why" makes a claim about issuer or cause (`grep`).
- Under Q1 (c), 122 lines go and the 7 cause lines stay. Under (d), 3 cause
  lines go.

**D7. The records.** Forced by check 6 and Q3.

- **SPEC-016**, How it works.
  - Replace "the event itself, or, for a `TokenMoved` whose `reason` is not
    empty, a copy of it with the `reason` cleared and every other field as the
    event has it" with "a copy of the event with its `participant_id` and
    `actor_role` cleared, a `TokenMoved`'s or a `ResourceChanged`'s `reason`,
    a `ConditionApplied`'s `source` and a `ConditionRemoved`'s `reason`
    cleared, and every other field as the event has it".
  - After the sentence that forwards the status payloads, add: "What `Project`
    sends of a `ResourceChanged`, a `ConditionApplied` or a `ConditionRemoved`
    carries no `reason` or `source` (`forwardable`): the cause names an
    ability, a threshold or a hand, and an ability's user may be an actor the
    viewer does not see."
  - Add to the first paragraph: "No frame `Project` sends a player or a
    spectator carries a `participant_id` or an `actor_role`, which name who
    issued the event; presence names every connected participant (SPEC-011)."
- **SPEC-016**, Consequences.
  - "A player's or a spectator's `TokenMoved` never carries a `reason`; the DM
    and the agent are sent it as the log holds it." becomes "No frame a player
    or a spectator is sent carries a `participant_id` or an `actor_role`, and
    none carries a `reason` or a `source`; the DM and the agent are sent each
    event as the log holds it."
  - "A frame with no event id is the projection's own: ..." gains "; one with
    an event id is the event less its issuer and its cause".
- **SPEC-016**, Requirements: add A and B. Status: add QA's new file if
  Phase 4a writes one.
- **`events.proto`**, per Q3: warnings in the form `TokenMoved.reason` already
  has.
  - Above `actor_role`: "Never forward actor_role or participant_id to a player
    or spectator: they name who issued the event (SPEC-016)."
  - Above each of the three cause fields: "Never forward reason to a player or
    spectator: it can name an ability whose user they do not see (SPEC-016)."
    `source` gets the same wording.
  - Then `task generate:contract`. P16 measured that only `events.pb.go` and
    `events_pb.ts` move, while `tools.json` and the bundle stay put.
- SPEC-007, SPEC-009, SPEC-012, SPEC-013 and SPEC-015 are not edited (Q8).

**D8. The sort.** Forced by SPEC-008 and the `requirements` skill. Rows are
lettered so nothing reads as an id.

| # | Rule (what) | Proposed | The observation that goes red | Test | Record |
|---|---|---|---|---|---|
| A | No envelope a player or spectator is sent carries a participant id or an actor role. | accept; the ticket's first rule, bounded to the envelope (Defect 1) | K4, K5 | `project_test.go#TestAChangeByAnUnseenUserNamesNoAbilityAndNoIssuer`, `server_visibility_test.go#TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`, `project_internal_test.go#TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`, `project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`, `keystone_test.go#TestTheProjectedGoldensAreWhatTheProjectionActuallySends` (all new but the last two) | SPEC-016 |
| B | No `ResourceChanged`, `ConditionApplied` or `ConditionRemoved` a player or spectator is sent carries a reason or a source. | accept under Q1 (a); the ticket's second rule, which (a) satisfies for every cause, seen or not | K6, K7 | the unit, wire and property tests of A, and the goldens | SPEC-016 |

**Refused, one line each:**
1. "projecting an event writes nothing to it": VTT-222, which the unit test
   cites.
2. "the DM and the agent are sent issuer and cause": VTT-176, which the wire
   test's DM half cites.
3. "a forwarded frame is the event less exactly these fields": how A and B are
   achieved, held by the unit test's equality and the guard (K3, K9).
4. "every envelope field outside the payload has a ruling": the guard is how A
   survives contract growth, like the payload-arm guard, which has no row.
5. "the issuer's own seat is told nothing of its own issue": A as worded, with
   no exemption, which the wire test's brawler stream holds.
6. "a correction carries no issuer or cause": VTT-249.
7. "a move's reason is not sent": VTT-262, unchanged.
8. "the log keeps its issuer": existing behaviour, held for `ToEvent` by
   `TestToEventStampsParticipantRoleAndID`, and not this ticket's (Gap 5).
9. "a seen cause is kept": not adopted (Q1).
10. "the client renders no issuer or cause": no client change (P15).

Two accepted, ten refused.

**D9. Comments and the ledger.** Forced by P17.
- `forwardable`'s doc stays two lines. `forwardedOf` gets one line.
- Each new test gets a doc of at most three lines and its citation line.
- The QA files gain no comment line.
- `--report` runs before the commit, then `--write-ledger`, which lowers
  `project_internal_test.go`'s and `server_visibility_test.go`'s rows and
  raises none.

**D10. Mutation keys: none moves; the self-tests decide, last.** After the
review settles and before the commit, run `python3 tools/check_mutation_test.py
-q` and `python3 tools/check_ts_mutation_test.py -q`.
- If a review adds a line to `project.go` anywhere above `objectInSight`, from
  `Viewer` down to `sceneSeenFor` and their docs, all three keys move by that
  count.
- A line added between `objectInSight` and `sortedSceneIDsUnion` moves only
  the third.
- Each moved key is re-pointed in `tools/mutation-equivalents.txt` by
  searching for its own quoted expression and reading its column at the new
  line, never by adding an offset. The two live sibling coordinates in the
  first entry's body move with it.
- A red on anything else means a key this plan says does not move does: stop.

**D11. One code commit and the report, per Q7.** Forced by P7: the code, the
recognisers and the goldens cannot be green apart. Each test file depends on the
others' helper, and the goldens red on any `forwardable` change.
- C1: `project.go`, the test files, the 12 goldens, SPEC-016, rows A and B,
  `events.proto` and its regeneration, the ledger.
- C2: the report.

**D12. Phase 4a for C1.** Forced by the dev-cycle skill. One QA agent per
`qa-prompt.md`, never given the diff, the source, the existing tests or the
implementer's report.
- Inputs:
  - rows A and B, with VTT-176, VTT-222, VTT-243, VTT-244, VTT-249 and VTT-262;
  - SPEC-016 whole as C1 leaves it;
  - the text of `Envelope`, `ResourceChanged`, `ConditionApplied`,
    `ConditionRemoved`, `TokenMoved` and `AbilityUsed` from `events.proto`;
  - `go doc -all` of `internal/gateway`, `internal/engine`,
    `internal/campaign`, `internal/identity` and `internal/rules`;
  - `rulesets/tavern-brawl/` as data, which is how a real cause is produced
    over the wire.
- QA writes `internal/gateway/qa_issuer_test.go`.
- Adjudications go in the report under their own heading. An escape goes to
  `docs/verification-debt.md` as a recipe.

**D13. Phase 4b for C1, after 4a.** One reviewer at high effort, briefed to
verify by command:
- every sentence C1 adds (SPEC-016's, the proto warnings, `forwardable`'s and
  `forwardedOf`'s docs, the renamed tests' pointers);
- that each D5 recogniser still demands an event id and full equality, so none
  is loosened;
- that the golden diff equals D6's deletion;
- the keys, the ledger, and the break lines in the draft message.

If the reviewer dies on a model's limit, say so and re-dispatch the same brief
on `fable`.

**D14. The deliberate breaks, one per check relied on.** In a scratch clone of
C1's final tree, each gate first clean, one edit per break, restored by the
inverse edit:

| Check | Break | Expected red |
|---|---|---|
| A, envelope | K4, then K5 | the unit, wire, guard and property tests; 12 goldens |
| B | K6 | the unit, wire and property tests; 3 goldens |
| B, removal | K7 | the wire and property tests; 2 goldens |
| VTT-222 on this path | K2 | `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt`, the property walk's DM tripwire |
| equality, no over-strip | K3, K9 | the unit and guard tests; the goldens |
| the DM arm | K10 | `TestTheDMReceivesEverythingUnchanged`, the keystone's identity arm |
| the walk's floor | eventgen's `ConditionApplied` and `ConditionRemoved` causes set to `""` | the property test's new floor |
| the guard | the cleared table loses `actor_role` | the guard |

**D15. Disk and load.** Forced by `check:mutation`'s 16 GiB floor.
Immediately before `task check`, run `go clean -cache`, `df -k` and `python3 -c
"import os; print(os.getloadavg())"`. `task check` is launched once, after C1
and in its own session (`start_new_session=True`), on a settled tree. Below 16
GiB, stop.

## Tasks, in dependency order

### Task 0 — Baselines

- Read `df -k` and the load, then run `check-requirements-chain.py .`,
  `check-comments.py main`, `check-doc-owner.py .` and both mutation
  self-tests.
- Re-run P5's parse over the 12 streams.
- **Done when:** the outputs match "Records and gates at the base" and P5.

### Task 1 — Rows

**Files:** `docs/requirements.md`. After sign-off, run `requirement-id` once
each for A and then B (B's text per Q1). **Done when:** two new OPEN rows.

### Task 2 — Tests first

**Files:** `project_test.go`, `server_visibility_test.go`,
`project_internal_test.go`, `project_property_test.go`.

D4's four tests and `forwardedOf`. **Done when:** each is red against the base
for the reason P13 and P14 name, and no other test changes colour.

### Task 3 — The projection

**Files:** `internal/gateway/project.go`. D1, at the end of the file. **Done
when:** Task 2's tests are green, and the three key lines are byte-identical to
the base's.

### Task 4 — The recognisers

**Files:** `qa_move_reason_test.go`, `qa_note_projection_test.go`,
`qa_testimony_removal_test.go`, `qa_testimony_sight_test.go`, and
`project_test.go`'s two edits. D5. **Done when:** P7's 33 non-golden tests and
`TestAnEventNamingAnUnknownActorIsWithheld` are green, and no QA file gained a
comment line.

### Task 5 — The goldens

**Files:** the 12 `scenarios/goldens/*/projections/*/stream.json`. D6.
**Done when:**
- `go test -count=1 ./internal/gateway/` is green;
- D6's deletion equals the committed bytes;
- `bun test client/test/projection-parity.test.ts` passes;
- `git status` shows no `state.json`, no `viewer.json` and no log-level
  `stream.json`.

### Task 6 — The records

**Files:** `docs/specifications/016-the-projection.md`,
`contract/vtt/v1/events.proto`, `contract/gen/`, `docs/requirements.md` (A's
and B's evidence). D7, then `task generate:contract` and `task build:client`.
**Done when:**
- `git status` shows `events.pb.go` and `events_pb.ts` and no
  `cmd/vtt/webdist` or `tools.json`;
- `grep -c 'is not empty, a copy of it' docs/specifications/016-the-projection.md`
  prints 0 (1 today);
- the chain prints `273 rows` with neither A nor B OPEN.

### Task 7 — Local gates

Run `gofmt`, `go vet`, `task lint`, `go test -count=1 ./internal/...
./contract/... ./cmd/... ./tools/...`, `bun test client/test contract
contract-spike`, `task check:comments`, `check:doc-owner` and
`check:new-prose`. **Done when:** each prints its completion line.

### Task 8 — Phase 4a, then 4b

D12, D13. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 9.

### Task 9 — Keys, ledger, commit C1

D9, D10. Then C1's message, which lists the ids and D14's lines. **Done when:**
- both self-tests print `OK`;
- `git show --stat HEAD` lists C1's files;
- `task check:drift` is clean on the committed tree.

### Task 10 — Breaks and the whole gate

D14 in a scratch clone, deleted afterwards. Then D15 and `task check` once.
**Done when:** each break gives its red, and `task check` exits 0 with every
step's own verdict, `check:mutation` included.

### Task 11 — The report

`docs/reports/2026-10-05-a-viewer-is-told-no-issuer-and-no-unseen-cause.md`,
per the `implementation-report` skill:
- each Done item with its observation;
- the rows and the refusals;
- the rulings taken at sign-off;
- the rule-9 answer;
- the breaks;
- the gaps below.

**Done when:** C2 holds the report alone.

## Commits

| Commit | Carries | Gate steps |
|---|---|---|
| C1 | D11's list | Tasks 7 to 9, then the pre-commit hook; `check:drift` after |
| C2 | the report | the hook |

`task check` runs whole once, after C1. Push after C2. Pre-push takes about
three minutes, so let it finish.

## Gaps that travel with this plan

1. **Done item 3 waits on the owner's ruling** (Q1). The plan proposes (a).
2. **Rule 1 and Done item 2 need the envelope bound** (Defect 1). Row A
   carries it, and the ticket's words do not.
3. **VTT-176's wire test cannot see the two breaks Done item 4 is about.** It
   stays green under an in-place write (K2) and under a stripped DM arm (K10),
   since the DM's seat has no projector and its test connects no player.
   - K2 is held by `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` and
     the property walk.
   - K10 is held by `TestTheDMReceivesEverythingUnchanged` and the keystone.
4. **"The cause was seen" is decidable only through facts no record states.**
   They are `Resolve`'s batch order and `rules`' reason format (P12). This
   matters only if Q1 picks (c).
5. **The log's issuer on the batch paths.** It is held for `ToEvent` alone, by
   `TestToEventStampsParticipantRoleAndID`. The wire test's DM half observes
   the `use_ability` batch.
   - `docs/verification-debt.md`'s open entry on `handleRemoveActor`'s batch
     stays open.
   - Neither stamping has a row.
6. **A batch shares one `occurred_at`, and withheld events leave gaps in a
   seat's sequences.** So a viewer can tell that a forwarded change came with
   an event it was not sent. It cannot tell which event. This was so before,
   and it is the 2026-10-01 plan's Gap 6.
7. **`AttackRolled` has no production writer**, so `Modifier.source` is
   unwritten and kept (D3). It needs a ruling if a writer arrives.
8. **No golden holds a cause the viewer did not see** (P5). The goldens hold B
   only for seen causes. The unit test and the walk hold unseen ones.
9. **`NarrationAdded.as` can name an actor a viewer does not see.** Narration
   is forwarded whole by the ruling of 2026-09-29, so this is not this
   ticket's.
10. **Presence tells every connection every participant's name.** With control
    frames, a viewer can work out who runs an actor it sees. That is SPEC-011's
    and SPEC-016's existing design, and not this ticket's.

## Questions for sign-off

1. **What is a player or a spectator sent of a status change's cause?**
   - (a) None, ever: `reason` and `source` cleared on every forwarded
     `ResourceChanged`, `ConditionApplied` and `ConditionRemoved`.
   - (b) The cause as the log holds it, which is today's behaviour.
   - (c) The cause kept when it was seen: `ability:<id>:*` when this seat was
     forwarded the batch's `AbilityUsed`, `threshold:*` always, and `manual`.
   - (d) Stateless by prefix: `ability:*` stripped, `threshold:*` and `manual`
     kept.

   Recommend (a):
   - (b) leaks the ability of an unseen user (P1, K6).
   - (c) needs a new `Projector` field (all three keys move), a record of each
     `AbilityUsed` verdict in `Project`, and a parser of `rules`' reason format
     in the gateway. It also needs `Resolve`'s batch order, which no record
     states (P12). All of that is to keep a string the client renders nowhere
     (P15), which repeats the forwarded `AbilityUsed`'s `ability_id` and
     `outcome_summary`.
   - (d) parses the same format to keep "manual", which tells a viewer a person
     rather than the rules removed a condition.
   - (a) costs four lines in `forwardable` and seven golden lines.
2. **Clone every forwarded frame (M1), or only when a field to clear is set
   (M2)?** Recommend M1 (D1). M2's pass-through is unreachable in production,
   since every append stamps the issuer. It adds a guard whose operators are
   mutants, and spares one test (P7).
3. **Warn in `events.proto` on `actor_role`/`participant_id` and on the three
   cause fields?** Recommend yes (D7). It follows the `TokenMoved.reason`
   precedent, and the regeneration is comment-only (P16).
4. **Re-point the 34 tests' recognisers through one `forwardedOf`, and rename
   the two QA tests whose names state the overturned sentence, rather than
   delete them?** Recommend yes (D5). Every recogniser still demands an event
   id and full equality (K table), and deleting a test deletes coverage nobody
   named.
5. **Add the property clause, the walk's stamped participant, and a floor of
   five seeds?** Recommend yes (D4). It holds A and B across every transition
   at no change to the walk's draws, and the floor is at its measured value
   (P14).
6. **Add the envelope-field guard?** Recommend yes (D4). Without it, a field
   added to `Envelope` later reaches every player by default. With it, the
   contract change reds until somebody rules on the field.
7. **One code commit and the report (D11)?** Recommend yes. The code, the
   recognisers and the goldens cannot be green apart (P7).
8. **Leave SPEC-007, SPEC-009, SPEC-012, SPEC-013 and SPEC-015 unedited?**
   Recommend yes. Their issuer sentences describe the log's envelope or the
   DM's seat, which do not change. Who is sent the field is SPEC-016's.
