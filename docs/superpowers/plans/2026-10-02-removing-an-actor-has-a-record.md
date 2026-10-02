# Removing an actor has a record — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-02-removing-an-actor-has-a-record-design.md`
**Verified:** 2026-10-02, by `verify-ticket`, an agent that did not write the
ticket, against `e52b4c2` on `record/removing-an-actor` (equal to `main`; the
ticket untracked). Verdict: **Passes with gaps.** The gaps are listed at the
end and travel with this plan. This plan does not edit the ticket; where an
item is thin, the plan decides around it and says so.

**Goal, in the ticket's words:** a specification states what `remove_actor`
appends and in what order, that the batch is accepted or refused whole, that
the handler checks nothing and what the fold refuses in its place, and that no
event ends a control grant; SPEC-013 no longer says it "has no record of its
own"; the false "(server.go's" sentence is gone from `convert.go`; every rule
the sort accepts has a row cited by a gateway test; `server.go`, `convert.go`
and `join.go` carry only warnings, pointers and exported symbols' doc
sentences, with no banned line and no block over the bound; and no code line
changes.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool has no
actor apart from its token: a `Token` is the character, and who may move it is
the token's own `ownerList`, so removing the token removes its owners with it,
with no event of their own. That is the shape this platform already has
(`controller_ids` lives on the `Actor`, and `ActorRemoved` takes it away), and
it is borrowed as confirmation, not as a change. Removal there is
client-first: `ServerCommandClientImpl.removeTokens` deletes the local tokens
and then sends `RemoveTokensMsg`; `ServerMessageHandler`'s handler calls
`Zone.removeTokens`, which skips an id it does not hold and fires one
`TokensRemoved` for the rest; nothing checks the ids and nothing orders them,
because there is no log to keep foldable. Refused, each with its reason: the
client-first delete, because here the server's fold is the only judge and a
client applies only what it is sent; the silent skip of an unknown id, because
here the fold refuses an unknown actor with its own message and the issuer is
told; and the unordered removal, because a log that is folded from the start
needs the tokens gone before their actor. Read by the verifier in
`~/dev/RPTool/maptool` (`Zone.removeToken` and `removeTokens`,
`ServerCommandClientImpl.removeTokens`, `ServerMessageHandler`'s
`RemoveTokensMsg` handler, `Token.ownerList`). This is a record and sweep of
existing behaviour; nothing is borrowed into code.

## Verification, check by check

1. **Every path resolves — by command.** `grep -F` finds `handleRemoveActor`,
   `handleCommand`, `authorize`, `credentialGone`, `handleJoinDoor`,
   `announcePresence`, `handlePromotion` and the `writeTimeout` field in
   `server.go`, `ToEvent` and its `RemoveCondition` arm in `convert.go`, and
   `handleJoin` in `join.go`; `campaign.AppendBatch` is in
   `internal/campaign/campaign.go`, `engine.Apply` and its "removed unknown
   actor" refusal in `internal/engine/apply.go`, `handleUseAbility` in
   `internal/gateway/ruleset.go`. The five test files, SPEC-007, 009, 011, 013,
   015 and 016, `docs/requirements.md` and `tools/comment-ceilings.txt` exist
   (`ls` counts 13). `docs/specifications/017*` does not exist.
2. **"Done" is an observation — by command.** Item 1: `grep -c 'has no record
   of its own' docs/specifications/013-authorization.md` prints 1. Item 2:
   `grep -c "(server.go's" internal/gateway/convert.go` prints 1. Item 3:
   `python3 tools/check-requirements-chain.py .` prints `253 rows, 206 test
   files, 10 specifications; every citation resolves and every row's evidence
   holds`; "more than 253" holds for any N of one or more, and the report
   states N. Item 4: `--report` prints `banned 8 blocks>6 11` for `server.go`,
   `7 3` for `convert.go`, `4 4` for `join.go`, and `0 0` for every other
   production file under `internal/gateway/`. Item 4's "every block left in
   them" reaches all 99, 8 and 8 blocks of the three files, not only the
   eighteen over the bound (D6 to D8, Q9). Item 5 is an invariant (D10). Item
   6 is the gate.
3. **Each rule is breakable — a reading, then probes.** D4 names, per
   candidate, the test that goes red and the edit that reds it. Every edit was
   run in a scratch clone of `e52b4c2` (`go vet` clean, then `go test
   -count=1 ./internal/gateway/...`, restored after each). Where a candidate
   is observed by no test the claim is bounded by search: no test under
   `cmd/vtt` drives `remove_actor` or `set_join_door` (`grep -l
   'remove_actor\|RemoveActor\|SetJoinDoor' cmd/vtt/*_test.go` prints
   nothing), and `internal/mcp`'s tests run against a fake server. The one
   probe that reaches `cmd/vtt` (H4b) was run over the whole closure.
4. **Scope matches the claim — by command, then a reading.** No code line
   changes, so callers do not widen the work: `handleRemoveActor` is called by
   `handleCommand` alone (`grep -rn handleRemoveActor` over `.go` files). What
   widens it is the reach of item 4 (check 2) and the records in check 6. No
   comment in the five test files points into a block this sweep cuts (`grep
   -rn` for the files' names, for "own doc comment", "comment says", and for
   phrases of the cut text, over `internal/`, `cmd/` and `client/src`). The
   pointers elsewhere name `handleRemoveActor` or `ToEvent` by symbol and stay
   true (`internal/engine/apply.go`, `internal/eventgen/model.go`,
   `contract/roundtrip_test.go`, `project_test.go`, `client/src/fold.ts`,
   `client/src/commands.ts`). Two that become weaker are left and named (D13).
5. **No recorded decision is contradicted — a reading.** SPEC-007 (the batch's
   contents, order and atomicity; what appends nothing), SPEC-009 (the door,
   the link, promotion, revocation, the join), SPEC-011 (presence, the
   bounds, `credentialGone`), SPEC-013 (the command path, `ToEvent`, the
   validators) and SPEC-016 (what a viewer is sent of a removal) were read
   against the ticket and the code; the ticket overturns none. Two of its
   compressions are loose: "presence and its announcements ... in SPEC-011"
   misses that a presence frame that cannot be encoded is sent to nobody and
   ends no connection, which only `announcePresence`'s comment says (Q8); and
   "the grant's kind in SPEC-013" — SPEC-013 holds the refusal of a kindless
   grant, but no record states that conversion carries every field (row H).
   One comment contradicts SPEC-013: `convert.go`'s grant arm says a kindless
   grant demotes the character; SPEC-013 and the fold say it leaves the kind
   as it was (Measurements).
6. **The records the work moves are named — by command, then a reading.** The
   section reads `New: removing an actor ...` and
   `docs/specifications/013-authorization.md`; the path resolves. The reading:
   SPEC-013 moves at two sentences, not one ("has no record of its own" and
   "the batch handlers are ... `handleRemoveActor`'s"), plus a sentence for
   row H and its Requirements line. Not named: SPEC-011 (one sentence, Q8) and
   SPEC-009 (its Requirements line, for row G, Q5). The ticket's second open
   question anticipated exactly this growth, so it is a gap, not a return.
   SPEC-007 is pointed at and not edited (Q2).

## Measurements this plan stands on

All at `e52b4c2`, by command, run by the verifier. `--report`, the checker's
own `measure`, and the token instrument (D10):

| File | Comment / non-blank | Share | Ceiling | Banned | Blocks > 6 | Cites | Tokens |
|---|---|---|---|---|---|---|---|
| `internal/gateway/server.go` | 358 / 959 | 37.3 | 37.4 | 8 | 11 | 0 | 4657 |
| `internal/gateway/convert.go` | 59 / 165 | 35.8 | 35.8 | 7 | 3 | 0 | 850 |
| `internal/gateway/join.go` | 78 / 147 | 53.1 | 53.1 | 4 | 4 | 0 | 399 |
| `internal/gateway/remove_actor_test.go` | — | 37.0 | 37.0 | 3 | 7 | 0 | 1645 |
| `internal/gateway/server_test.go` | — | 27.3 | 27.4 | 34 | 27 | 31 | 13016 |
| `internal/gateway/server_internal_test.go` | — | 36.8 | 36.9 | 27 | 28 | 11 | 6583 |
| `internal/gateway/convert_test.go` | — | 14.4 | 14.4 | 9 | 2 | 0 | 2940 |
| `internal/gateway/join_test.go` | — | 25.7 | 25.7 | 6 | 8 | 10 | 2323 |
| `cmd/vtt/scenario_goldens_test.go` (Q6) | — | 25.7 | 25.7 | 3 | 2 | 1 | 1235 |

The ticket's figures for the three production files are these.

**False sentences, each established by command.**

1. `convert.go`, the `RemoveCondition` arm: "`server.go`'s `handleUseAbility`".
   `grep -rn 'func (s \*Server) handleUseAbility'` prints `ruleset.go`. (The
   ticket's.)
2. `convert.go`, the `GrantActorControl` arm: "an accepted grant, written
   kindless, DEMOTES the character it was meant to hand over" and "the dropped
   field fails closed rather than open". `engine.Apply`'s `ActorControlGranted`
   arm sets `Kind` only when the grant states one; a scratch engine test that
   adds an actor as `PARTY_MEMBER`, then as `NON_PARTY`, and applies a
   kindless grant to each logs the kind unchanged both times. A dropped kind
   leaves the actor as it was, which is open for a grant meant to make a party
   member a non-party.
3. `server.go`, the `noProgress` field: "serveWS force-closes the socket". No
   `serveWS` exists (`grep -rn serveWS internal cmd` prints only this
   comment); the pump in `serve` closes it (SPEC-011).
4. False as worded: the block above `broadcast` in `announcePresence`,
   "Presence is the ONE delivery path that does not run through the pump".
   `grep -n 'outCh <- b' internal/gateway/server.go` shows the catch-up head
   and every command result queued by `serve` outside the pump (SPEC-011
   lists both).

`join.go`'s "That cost is real and is recorded in the spec" is not false — the
joining design ticket records it — but it points at a ticket, not a record,
and no specification states it.

**What the tests observe, by probe.** Each a single edit in the scratch clone:

| # | Edit | Result |
|---|---|---|
| P1 | `sort.Strings(tokenIDs)` made a reverse sort | red: `TestRemoveActorEmitsEveryTokenThenTheActor` (`batch envelope 0 subject = "t1", want "t0"`) |
| P2 | the token filter made `tok.ActorID == cmd.GetActorId() \|\| id != ""` | red: `TestRemovingAnActorWithNoTokensYieldsABatchOfOne` (`log head moved from 6 to 8`) alone |
| P3 | the `ActorRemoved` built first, the tokens after | red: `TestRemoveActorEmitsEveryTokenThenTheActor`, `TestARemovedActorLeavesALogThatStillFolds` (`still has token "t0" on the board`) |
| P4 | an `ActorControlRevoked` per controller inserted before the `ActorRemoved` | red: `TestRemoveActorEmitsEveryTokenThenTheActor` (`batch envelope 2 kind = "other"`) |
| P5 | `AppendBatch` replaced by a `campaign.Append` loop | green. A scratch internal test that hands `handleRemoveActor` a snapshot taken before a second token was placed is green at the base and red here (`t1 was removed by a refused removal`); it was never committed |
| P6 | `engine.Apply`'s unknown-actor check switched off | red: `TestRemoveActorAppendsNothingWhenThePartyIsUnknown`, and the engine's `TestActorRemovedUnknownActorErrorMatchesTokenRemovedWording` |
| P7 | `engine.Apply`'s standing-token guard switched off | red: `TestAnOutOfOrderRemovalBatchIsRejectedWhole`, `TestAMidBatchRefusalPersistsNothingThatCameBeforeIt`, `TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`, and the engine's `TestActorRemovedRefusesWhileOneOfItsTokensStillStands`; none drives `handleRemoveActor` |
| P8 | the batch's `ParticipantId` and `ActorRole` left unstamped | green |
| J1 | the result's `Sequence` made the batch's last | red: `TestRemoveActorEmitsEveryTokenThenTheActor` |
| J2 | the result's `Sequence` made `firstSeq + 1` | red: the same, and `TestRemovingAnActorWithNoTokensYieldsABatchOfOne` |
| P9 | `JOIN_DOOR_UNSPECIFIED` treated as closed | red: `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` (`an unspecified door must be refused`) |
| P9b | the refusal arm calling `SetJoinOpen(false, ...)` before it refuses | green (the door was shut before and after) |
| H1, H2 | `revoke_actor_control`'s conversion dropping `ActorId`, then `ParticipantId` | red: `TestDMGrantsControlOverTheWire` |
| H4b | `add_actor`'s actor kind zeroed in `ToEvent` | the gateway green; over `./internal/gateway/... ./cmd/vtt/...` red only in `TestScenarioGoldenStreamsHaveNotDrifted`, eight subtests |
| H5 | `ConditionRemoved`'s `Reason` made empty | red: `TestToEventRemoveConditionProducesConditionRemoved` |

Every other conversion arm's fields are asserted in its own `convert_test.go`
test (read); `TestToEventAddActorProducesActorAdded` asserts the actor's id
alone.

**A dry run of D6 to D8, in the scratch clone and never in the tree.** With the
draft texts below: the token loop prints `same` at 4657, 850 and 399; `gofmt
-l internal/gateway/` prints only `scenario_test.go`; `go vet` clean; `go test
-count=1 ./internal/gateway/` green; `golangci-lint run ./internal/gateway/`
(2.11.4) `0 issues`; `check:doc-owner` `80 files, every doc comment sits on
its own function`; `check:new-prose main` `80 added line(s) across 3 file(s),
all clean`; `check:comments main` refuses exactly the three files for having
fallen more than the band; `--write-ledger` then changes exactly three rows
(`server.go` 37.4 to 25.4, `convert.go` 35.8 to 9.5, `join.go` 53.1 to 17.9)
and `check:comments` ends `251 files, 80 added comment lines, 251 ledger rows;
clean`; `--report` prints `banned 0 blocks>6 0` for the three. With `// VTT-162`
then added above eight of the tests D5 names, four of them below doc blocks
over the bound, `check:comments` still ends `clean`, every share is unchanged
and the token loop prints `same` for each test file. `go doc` prints `New`'s
sentence and warning and `ToEvent`'s sentence and pointer. The clone was deleted afterwards.

**Mutation keys.** `internal/gateway` is in `PACKAGES`. `grep -n -E
'internal/gateway/(server|convert|join)\.go'` over
`tools/mutation-equivalents.txt` prints nothing, and neither adjudication file
names the three files or their symbols in prose; the gateway's keys are
`authz.go` (one) and `project.go` (three), which this work does not touch. A
key is a coordinate in its own file, and test files carry none, so no key
moves. `python3 tools/check_ts_mutation_test.py -q` prints `OK` (50 tests).
`python3 tools/check_mutation_test.py -q` FAILS two of 107 at the base, both
on the disk floor (`15.7 GiB free ... needs at least 16 GiB`), not on a key
(D17).

**Records and gates.** `docs/specifications/` holds 007 to 016; 017 is free.
`docs/requirements.md` has 253 rows, the last `VTT-253`. `python3
tools/check-comments.py main` ends `251 files, 0 added comment lines, 251
ledger rows; clean`. `check:doc-owner` ends `80 files, ...`. `gofmt -l
internal/gateway/` prints only `scenario_test.go`. `requirement-id` is at
`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`. Free space on
`/System/Volumes/Data` read 16,787,292, 16,420,680 and 17,909,004 KiB during
verification (16.0, 15.7, 17.1 GiB), the Go cache 1.7 then 2.0 GiB. Load
averages ran between 3.2 and 14.1.

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is held
  by the Phase 4b reading; VTT-050, VTT-052, VTT-053, VTT-055 by
  `check:comments`; VTT-059 says a citation line moves no share and joins no
  block.
- `CLAUDE.md` rule 2: no gate is weakened; the ledger only goes down, through
  `--write-ledger`.
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: no line number, date,
  commit hash, plan, task, `§`, `#NNN`, ADR or ticket section in a comment.
  This plan and the report name blocks by the symbol they sit on.
- `CLAUDE.md` rules 3, 4, 5: the contract, the fold and the vocabulary are not
  touched; SPEC-017 describes `engine.Apply`'s refusals and adds no fold.
- `CLAUDE.md` rule 9: the answer above is recorded before any task runs.
- SPEC-008: ids come from `requirement-id`, after sign-off, never by hand.
- The `specification` skill and `catches.md`: five headings, present tense, no
  `Why`, no `Rejected`, no measurement, no line number, one decision per file,
  no claim taken from a comment without reading the code (item 8: three
  comments here are false), every "only/never/every/none" with its search
  (item 10), the old text not left beside the new (item 13).
- The `requirements` skill: one thing, breakable, named by a check or
  knowingly OPEN; refuse more than you accept; every refusal in one line.
- Done item 5: the token stream of every Go file touched is `e52b4c2`'s.
- Not touched: `internal/engine`, `internal/campaign`, `internal/store`,
  `contract/`, `client/src`, every other file under `internal/gateway`, and
  the comments of the test files beyond citation lines.

## Decisions this plan makes

**D1. `remove_actor`'s record is a new `docs/specifications/017-removing-an-actor.md`.**
Forced by the `specification` skill and by SPEC-013. SPEC-007's one decision is
the wire contract: its Status names `contract/` and the contract tests, and its
Consequences promise that a consumer who reads it needs nothing else. The
handler's decisions — the batch is built from the snapshot `authorize` took,
stamped by the handler, appended in one `AppendBatch`, checked by the fold
alone, with no event for the grants — are about the gateway, not the wire;
written into SPEC-007 they are a second decision in one file (`catches.md`
item 14). SPEC-013 already sends each batch handler to a record of its own
(`use_ability` and `load_adventure` to SPEC-012, `load_map` to SPEC-014), and
SPEC-014 points at SPEC-007 for its batch's shape; a section of SPEC-007 would
make `remove_actor` the one batch handler whose record is the wire contract.
Title: `SPEC-017: An actor leaves the world in one batch that only the fold
checks`. Q1.

**D2. SPEC-017's sections.** Forced by ticket item 1. One bold-led paragraph
each, every sentence naming its symbol:

| Section | Source today | What it states | Rows |
|---|---|---|---|
| The batch is built from the snapshot | `handleRemoveActor`'s doc and body | `handleCommand` hands it the state `authorize` took (SPEC-013); it collects the ids of `st.Tokens` whose `ActorID` is the command's `actor_id`, sorts them with `sort.Strings`, and builds one `TokenRemoved` per id in that order and then the `ActorRemoved`: the batch SPEC-007 states, pointed at and not restated; an actor with no token yields the `ActorRemoved` alone | A, B |
| The stamping | the block above the stamping loop | each envelope an `EventId` from `newEventID`, the issuer's `ParticipantId` and `ActorRole`, one `OccurredAt` for the batch; `store.AppendBatch` refuses an envelope with no `EventId` | none (I) |
| One append | the "ONE campaign.AppendBatch" paragraph | `campaign.AppendBatch` folds every envelope against a snapshot under the campaign's lock before it persists any, so the batch is appended whole or not at all (SPEC-007's "atomically"); an ok result carries the first sequence; a refusal carries the fold's message | W, J |
| The fold refuses in the handler's place | the "NOTHING IS CHECKED HERE" paragraph | `handleRemoveActor` checks nothing; `engine.Apply`'s `ActorRemoved` arm refuses an actor the state does not hold (`engine: removed unknown actor "<id>"`) and an actor with a token still on the board, naming the first in id order; the snapshot is read before `AppendBatch` takes the lock, so a token placed for the actor in between makes the whole batch refused | none (C, D) |
| Control ends with the actor | the "CONTROL GRANTS NEED NO EVENT" paragraph | `controller_ids` is a field of the `Actor`, which `ActorRemoved` deletes, so no event revokes a grant; an actor added later under the same id is controlled by nobody, since an `add_actor` that names a controller is refused (SPEC-013) | none (E) |
| What this record does not decide | — | the batch's wire shape (SPEC-007); who may remove (SPEC-013, VTT-141); what a viewer is sent of a removal (SPEC-016); what the fold refuses in general and that the actor's conditions leave with it (`engine.Apply`) | — |

Status: `Accepted. Implemented by internal/gateway/server.go (handleRemoveActor,
dispatched from handleCommand), over internal/campaign's AppendBatch and
internal/engine's Apply; pinned by internal/gateway/remove_actor_test.go.`
Principles served: no blueprint; the principle missing from the record is that
a fact about the world is checked once, by the fold. Consequences: a client
receives a removal as contiguous `TokenRemoved` events then the
`ActorRemoved`; removing an unknown actor is refused with the fold's message;
a removal refused because a token was placed meanwhile may be sent again;
control need not be revoked first; whoever changes the handler adds no check
the fold already makes and appends in one call. Requirements: the ids of A, B,
J and W (Q2). The reasons the cut comment argued (Go randomises map order and
the log is permanent; `handleLoadMap` as the precedent; the client's re-fold
freezing) go to the report.

**D3. The other records.** Forced by check 6. SPEC-013, three changes and its
Requirements line (row H): "`remove_actor` (`handleRemoveActor`) has no record
of its own" becomes "`remove_actor` (`handleRemoveActor`) is SPEC-017's"; "the
batch handlers are SPEC-012's, SPEC-014's and `handleRemoveActor`'s" becomes
"... and SPEC-017's"; "Every other command reaches `ToEvent` and becomes one
envelope." gains ", carrying every field the command gave, a grant's `kind`
included". SPEC-011 (Q8), one sentence after "presence is repaired by the next
snapshot, never by the log": "A presence frame that cannot be encoded is sent
to nobody, a joiner's snapshot included, and ends no connection:
`announcePresence` returns, and `joinAndSend`, `announceIfPresent` and
`announceIfAbsent` send nothing for a nil frame." SPEC-009 (Q5): G's id on its
Requirements line, no prose. SPEC-007: not edited.

**D4. The sort.** Forced by SPEC-008 and the `requirements` skill; lettered so
nothing reads as an id.

| # | Rule (what) | Proposed | Evidence, read in the test body | The edit that reds it |
|---|---|---|---|---|
| A | Removing an actor appends a `TokenRemoved` for each of that actor's tokens and for no other token, then its `ActorRemoved`, and no other event. | accept — the first candidate's content half; E's "no event of its own" is its last clause; F is it at zero tokens | `TestRemoveActorEmitsEveryTokenThenTheActor` (exact kinds and subjects), `TestRemovingAnActorWithNoTokensYieldsABatchOfOne` (head moves by one) | P2; P4 |
| B | A removal's `TokenRemoved` events are in token-id order, and its `ActorRemoved` comes last. | accept — the first candidate's order half, reddened by different edits | `TestRemoveActorEmitsEveryTokenThenTheActor` (`t0` sorts first and was placed second), `TestARemovedActorLeavesALogThatStillFolds` | P1; P3 |
| C | Removing an actor that does not exist is refused and appends nothing. | refuse, Q3 — the refusal is `engine.Apply`'s and only an engine edit reds it (P6); "appends nothing" is VTT-162 | `TestRemoveActorAppendsNothingWhenThePartyIsUnknown` | P6 |
| D | A token placed after the snapshot makes the removal refused, never leaves a token without its actor. | refuse, Q3 — the guard is `engine.Apply`'s (P7), its tests drive `AppendBatch` by hand, and the command's half of it is W | `TestAnOutOfOrderRemovalBatchIsRejectedWhole`, `TestAMidBatchRefusalPersistsNothingThatCameBeforeIt` | P7 |
| E | Removing an actor ends every control grant on it, with no event of its own. | refuse — "no event of its own" is A's last clause (P4); "ends every grant" is structure, breakable only by adding a store, and stays SPEC-017 prose | — | — |
| F | Removing an actor with no token appends the `ActorRemoved` alone. | refuse — A at zero tokens; its test is A's evidence | `TestRemovingAnActorWithNoTokensYieldsABatchOfOne` | P2 |
| G | A `set_join_door` that says neither open nor closed is refused. | accept, narrowed, Q5; "and changes nothing" stays prose: P9b writes the door and stays green | `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` | P9 |
| H | The event a one-envelope command becomes carries every field the command gave, a grant's kind included. | accept, Q6 | eleven `TestToEvent*Produces*` tests (all but `EndSession`'s, which has no field) and `TestToEventGrantActorControlCarriesTheKind` in `convert_test.go`, `TestDMGrantsControlOverTheWire` (revoke), `TestScenarioGoldenStreamsHaveNotDrifted` in `cmd/vtt` (`add_actor`'s kind) | H1, H2, H4b, H5 |
| W | A removal is appended whole or not at all. | OPEN — no test yet, Q4; from the ticket's problem paragraph | none: no test drives `handleRemoveActor` with a stale snapshot | P5 (green) |
| J | A `remove_actor` result carries the first sequence of the batch it appended. | accept, Q7 — as VTT-137 and VTT-175 are for the other batch handlers | `TestRemoveActorEmitsEveryTokenThenTheActor`, `TestRemovingAnActorWithNoTokensYieldsABatchOfOne` | J1; J2 |

Refused from the problem paragraph, one line each. **I**, every envelope of a
removal carries the issuer's id and role — no test observes it (P8); it is
`ToEvent`'s and every batch handler's (SPEC-009 states it), and a row for one
handler would be a fragment; the gap goes to the debt file (Q10). **K**, a
fresh event id per envelope and one `OccurredAt` per batch — how, not what;
the store's unique `event_id` refuses a repeat. **L**, the tokens come from
the snapshot `authorize` took — how; SPEC-013 owns the snapshot. Two clauses
stay prose: G's "changes nothing", E's "ends every grant".

Five accepted (A, B, G, H, J), one OPEN (W), seven refused (C, D, E, F, I, K,
L) and two refused clauses.

**D5. Rows and citation lines.** Forced by SPEC-008. Ids from `requirement-id`
after sign-off; ids start at `VTT-254`. Evidence cells written by hand
afterwards as `path#Test`; W's cell stays `**OPEN — no test yet**` with no
citer. A citation line is `// VTT-NNN` directly above `func Test`, below any
doc block, several ids on one line: `TestRemoveActorEmitsEveryTokenThenTheActor`
carries A, B and J; `TestRemovingAnActorWithNoTokensYieldsABatchOfOne` A and
J; `TestARemovedActorLeavesALogThatStillFolds` B;
`TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` G;
`TestDMGrantsControlOverTheWire`'s existing `// VTT-147` line gains H; the
twelve `convert_test.go` tests carry H; per Q6, the existing `// VTT-240`
line of `TestScenarioGoldenStreamsHaveNotDrifted` gains H.
`server_internal_test.go` and `join_test.go`, which the ticket lists, gain
nothing. Requirements lines are copied from the register last: SPEC-017 (A,
B, J, W), SPEC-013 (H), SPEC-009 (G).

**D6. `server.go`: what each block becomes.** Forced by ticket item 4. Each
text was built in the dry run. A text above an unexported function opens with
that function's name (a pointer) or is a warning.

| Block (by symbol) | Decision held by | Sentence no record holds, and where it goes | Stays at the line |
|---|---|---|---|
| `Server`'s doc (3, banned) | SPEC-011 | — | `Server is the WebSocket and HTTP gateway over one open Campaign and identity DB (SPEC-011).` |
| `writeTimeout` (7, over) | SPEC-011 (both bounds; `New` sets the same figure) | the history ("how its absence went unnoticed"): report | `Keep writeTimeout apart from noProgress: with one knob the store always drops first, and the bound on a write is never exercised (SPEC-011).` |
| `presence` (3, banned) | SPEC-007, SPEC-011 | — | `Never append presence to the log: who is online is not campaign history (SPEC-007, SPEC-011).` |
| `announcePresence`'s doc (16, over, banned) | SPEC-011 (arrivals; the departure re-check) | "an encode failure is dropped, unlike the head's": SPEC-011 (D3, Q8); the arrival's no-recheck argument: report | `announcePresence tells every other connection that pc's participant arrived (SPEC-011). Drop the frame on an encode failure, never the connection: presence is repaired by the next snapshot.` |
| the block above `broadcast` in `announcePresence` (4, banned, false as worded) | SPEC-009, SPEC-011, VTT-032 | — | `Pass revoked(): presence does not run through the pump, so this is where a revoked participant stops hearing it (SPEC-009, VTT-032).` |
| `announceDeparture`'s doc (11, over, banned) | SPEC-011 (`announceIfAbsent`) | the inversion count and the manual reconnect: report | `announceDeparture announces pc's participant gone only while no connection of theirs remains (SPEC-011, announceIfAbsent).` |
| the block above `deny := s.revoked()` (9, over) | SPEC-011 (no identity read under the registry's locks; check and targets in one hold) | — | `Resolve revoked here, before announceIfAbsent, even for a departure it suppresses: an identity read inside it holds the registry's locks (SPEC-011).` |
| `revoked`'s doc (15, over) | SPEC-009 (`ErrInvalidToken` alone), SPEC-011 | the cost per frame: report | `revoked answers the connected participants whose Lookup is ErrInvalidToken (SPEC-009). Never call it under the registry's locks (SPEC-011).` |
| `announcePromotion`'s doc (16, over) | SPEC-009, SPEC-011, VTT-047 | why a nudge, found by the e2e: report | `announcePromotion re-announces a promoted participant to every connection, theirs included (SPEC-009, VTT-047).` |
| `handleRemoveActor`'s doc (33, over, banned) | SPEC-007 (shape, atomicity); the rest SPEC-017 (D2) | the map-order and permanence reason, the `handleLoadMap` precedent, the client freeze: report | `handleRemoveActor appends the batch SPEC-017 states. Check nothing here: the fold refuses an unknown actor and a token placed after st was read, and a refused batch appends nothing.` |
| `handleJoinDoor`'s refusal arm (7, over) | SPEC-009; the enum's reason is `JoinDoor`'s doc in `commands.proto` | — | `Refuse, never default: guessing open admits strangers and guessing closed locks the table out (SPEC-009, JoinDoor).` |
| `handlePromotion`'s doc (7, over) | SPEC-009, SPEC-013, SPEC-007 | — | `handlePromotion applies promote_participant (SPEC-009).` |
| the block above `s.ids.Lookup` in `handlePromotion` (12, over, banned) | SPEC-009, VTT-026 | "nobody left at the table could undo it": report | `Refuse a dm or agent target here: Authorize does no I/O and cannot read the target's role (SPEC-009, VTT-026).` |
| `credentialGone`'s doc (7, over) | SPEC-009, SPEC-011, VTT-033 | — | `credentialGone reports whether Lookup answers ErrInvalidToken for the participant (SPEC-009). Match nothing else: an operational failure must not drop a frame (VTT-033).` |

Under the bound, and descriptions that Done item 4 does not leave (Q9):
`buffer` → `Override only from this package's tests: New sets gatewayBuffer
(SPEC-011).`; `noProgress` (FALSE: `serveWS`) → `Override only from this
package's tests; zero means store.SubscriberNoProgressTimeout (SPEC-011).`;
`pingInterval` → `Override both only from this package's tests: New sets
gatewayPingInterval and gatewayPingTimeout (SPEC-011).`; `encodeFrame` → `See
EncodeFrame (SPEC-011).`; `onServeDone` → `Leave nil in production.
TestAClientThatStopsReadingEntirelyIsTornDown sets it to see a teardown its
own client cannot.`; `static` → `See WithStatic (SPEC-011).`; `New`'s doc →
`New constructs a Server over an already-open campaign and identity DB. Close
both after it stops serving: the caller owns them (SPEC-011).` ("no ruleset
loaded" is SPEC-012's); the block inside `announcePromotion` → `Resolve,
encode and send inside announceIfPresent, never as separate steps: a
connection that unwinds between them leaves a ghost (SPEC-011).`; the stamping
block in `handleRemoveActor` → `Stamp the four fields the batch leaves zero:
store.AppendBatch requires an EventId (SPEC-017).`; `handleJoinDoor`'s doc (a
ticket section) → `handleJoinDoor applies set_join_door (SPEC-009).`; the
budget block → `Pass the limit through as it came: SetJoinOpen owns the
default (SPEC-009).`; `handleRotateJoinLink`'s doc → `handleRotateJoinLink
applies rotate_join_link (SPEC-009).` and its body block → `Never return the
secret here: a result travels the channel every participant's frames use, and
the DM reads it from GET /api/join-link (SPEC-009).`; the block above
`errEncodeFrame` → `Keep these two apart: deliver closes with a reason for
errEncodeFrame alone (SPEC-011).` The other 71 blocks are this sweep's
predecessors' warnings and pointers, re-read by Phase 4b and not rewritten.

**D7. `convert.go`.** Forced by ticket items 2 and 4. The arms' blocks each say
"nothing to validate here"; one warning above the switch replaces them, and
the arm blocks go.

| Block (by symbol) | Decision held by | Stays at the line |
|---|---|---|
| `ToEvent`'s doc (5) | SPEC-013 | `ToEvent converts an authorized ClientCommand into the past-tense Envelope it becomes, stamping EventId, ParticipantId, ActorRole and OccurredAt (SPEC-013). TestEveryClientCommandConverts lists the commands that do not convert here.` |
| new, above the type switch | SPEC-013 | `Validate nothing in these arms: Authorize, the validators and the fold have (SPEC-013).` |
| the `RemoveToken` arm (5, banned) | SPEC-013; the fold's unknown token | deleted |
| the `RemoveCondition` arm (12, over, banned, FALSE) | SPEC-013 (dispatch of `use_ability`; `campaign.Append` folds before it persists) | deleted; `grep -c "(server.go's"` prints 0 |
| the `AddNarration` arm (3, banned) | `engine.Apply`'s `NarrationAdded` arm (size and anchors) | deleted |
| the `GrantActorControl` arm (22, over, banned, FALSE) | SPEC-013 (the kindless refusal; D3's new sentence, row H) | `Carry Kind through (TestToEventGrantActorControlCarriesTheKind): a dropped kind answers ok=true and records a grant that changes no kind.` |
| the `OpenDoor` arm (7, over, banned) | SPEC-013 (`mayWorkDoor` in `Authorize`) | deleted |
| `newEventID`'s doc (3) | the store's unique `event_id` | deleted |

`ErrUnknownCommand`'s doc sentence stays.

**D8. `join.go`.** Forced by ticket item 4. Every decision is SPEC-009's.

| Block (by symbol) | Stays at the line |
|---|---|
| `maxJoinBody` (2) | `Keep the cap: /join is unauthenticated, so its caller chooses how much is read (SPEC-009).` |
| `maxDisplayNameRunes` (4) | `Count runes, never bytes: a byte cap gives a non-ASCII name a third of the room (VTT-012).` |
| `usableDisplayName`'s doc (24, over) | `usableDisplayName holds the display-name rule SPEC-009 states (VTT-012).` |
| the block above the `switch` in its loop (7, over) | `Keep Other_Default_Ignorable_Code_Point: U+3164 HANGUL FILLER is a letter and draws nothing (VTT-012).` |
| `handleJoin`'s doc (17, over, banned) | `handleJoin is the POST /join SPEC-009 states. Mint a spectator and nothing else: whoever holds a shared link may only watch (VTT-010).` |
| the block above `strings.TrimSpace` (3) | `Refuse the name first, and distinctly: it says nothing about the door (VTT-013).` |
| the block above `JoinAdmits` (15, over, banned) | `Ask JoinAdmits alone, and refuse its error as a refusal: a database that cannot answer must not open the door (SPEC-009, VTT-046).` |
| the refusal block (6, banned) | `Answer every refusal with this status and body: a difference tells a prober which half it got right (VTT-009).` |

The history ("the two calls it used to be"), the logs-and-CLI argument and the
spent-budget cost go to the report.

**D9. The sweep's rules.** Forced by rule 10, SPEC-010 and the earlier sweeps.
A warning is imperative and at most three lines; a pointer is `SPEC-NNN`,
`VTT-NNN`, a test or symbol name, or a `docs/` path; a doc sentence is what
`go doc` prints. A block above a function opens with that function's name or
with a word that names no function (`check:doc-owner`; `Resolve`, `Apply`,
`Append`, `Project`, `Fold` name functions). Every field that has a comment
keeps one, so no field joins another's gofmt alignment group. Lines sit in the
wrap band `check:new-prose` applies (`tools/check-comment-wrap.py`); a long
test name goes where it does not leave a short line before it. Every fact a
cut block held that no record states goes to SPEC-017 or SPEC-011 (D2, D3) or
is named in the report as dropped, with the reason. The reading may shorten a
text, never lengthen one past three lines.

**D10. Token-stream identity, Done item 5.** The command is the seat-and-perch
report's: a `go/scanner` program, mode 0, printing `tok %q lit` per token
with comments dropped (the listing in
`docs/superpowers/plans/2026-09-24-sweep-identity.md`, Task 0), built once in
the scratchpad as `$S/codetokens/codetokens`, and run from the repository
root:

    for f in internal/gateway/server.go internal/gateway/convert.go internal/gateway/join.go \
             internal/gateway/remove_actor_test.go internal/gateway/server_test.go \
             internal/gateway/server_internal_test.go internal/gateway/convert_test.go \
             internal/gateway/join_test.go cmd/vtt/scenario_goldens_test.go; do
      git show "e52b4c2:$f" > "$S/before.go"
      "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
      "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
      printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
      cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
    done

Done reads nine `same` lines at 4657, 850, 399, 1645, 13016, 6583, 2940, 2323
and 1235. A zero is a failed run; a `DIFFERS` on a test file is a changed
literal, which Phase 4b names.

**D11. The ledger, last.** Forced by SPEC-010's band. After Phase 4b has
settled, `python3 tools/check-comments.py --write-ledger`; `git diff
tools/comment-ceilings.txt` must show exactly the three production rows, each
lowered (the dry run: 37.4 to 25.4, 35.8 to 9.5, 53.1 to 17.9; the final texts
decide). Citation lines move no share (VTT-059). Any other changed row means a
file changed that this plan does not name: stop and ask.

**D12. Mutation keys: none to re-point, the self-tests last.** Forced by the
measurement above. No key names the three files, so no re-point runs. Both
self-tests still run after the last edit of any `.go` file and before the
commit, because "no key moves" is the claim a moved line would falsify; a red
on anything but the disk floor means a key this plan says does not exist does:
stop.

**D13. The test files.** Forced by ticket item 3 and the third open question.
A citation line is in no block (VTT-059; the dry run), and no test file in the
ticket's list holds a pointer into a cut block, so no test-file block is
forced to the bound and none is touched beyond citation lines. Left, named in
the report: `TestRemoveConditionAppliedThenRemoved`'s doc in `ruleset_test.go`
("see convert.go", a seven-line block, which after D7 names the file and no
comment); `TestDecodeCommandMalformedJSONErrorsCleanly` in `codec_test.go`,
whose bare line citation of `server.go` already points into
`handleRemoveActor`'s doc and moves again; `TestEveryClientCommandConverts`'s
string literal crediting `use_ability` to "(server.go)" (a literal, so Done
item 5 forbids it); `TestToEventGrantActorControlCarriesTheKind`'s doc, which
says a dropped kind reads back as something nobody declared; and
`remove_actor_test.go`'s package doc, which restates the cut comment with
ticket tasks. All the test-prose sweep's.

**D14. The deliberate breaks.** In a scratch clone (`git clone
--no-hardlinks`, the final `git diff HEAD` applied, untracked files copied,
committed there as that clone's `main`), each gate first clean, then one edit
per break, the inverse edit by hand, `git diff --stat` empty before the next:

| # | Break | Expected red |
|---|---|---|
| B1 | `// VTT-999` above `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt` | `check:requirements-chain`: `cites VTT-999 and no row ... defines it` |
| B2 | one new row's evidence re-pointed at a test that does not carry its id | `check:requirements-chain`: `does not carry the id` |
| B3 | SPEC-017's Requirements line gains `VTT-999` | `check:requirements-chain`: the specification citation |
| B4 | after the ledger, `// SPEC-017` between two code lines of `handleRemoveActor` | `check:comments`: `server.go: comment share ... is above its ceiling` |
| B5 | `sort.Strings(tokenIDs)` made a reverse sort | D10 prints `DIFFERS` for `server.go`; `TestRemoveActorEmitsEveryTokenThenTheActor` red (P1) |
| B6 | "(server.go's handleUseAbility" re-inserted in the warning above the switch | `grep -c "(server.go's" internal/gateway/convert.go` prints 1 |
| B7 | `Task 9` added to the warning above `handleRemoveActor` | `check:comments`: the banned term |
| B8 | that warning rewritten to open with `Apply` | `check:doc-owner`: the doc above `handleRemoveActor` describes `Apply` |

**D15. Phase 4b holds VTT-051 and SPEC-017, sentence by sentence.** The
reviewer gets `git diff HEAD`, SPEC-007, 009, 010, 011, 013 and 017, the new
rows and VTT-009, 010, 012, 013, 026, 032, 033, 046, 047, 141, 162, the three
files, the touched test files, `campaign.AppendBatch`, `engine.Apply`'s
`ActorRemoved`, `TokenPlaced` and `ActorControlGranted` arms,
`internal/gateway/presence.go` and `commands.proto`'s `JoinDoor`. For every
surviving block: its kind and, for a warning, whether the consequence is true
of the code it guards. For every deleted block: did it hold a fact no record
states. For every SPEC-017 sentence: the symbol and whether the code does it
(`catches.md` item 8). For every row: the edit that reds it. For SPEC-013 and
SPEC-011: that nothing else changed. For the rule-9 answer: that SPEC-017
names no other project.

**D16. Phase 4a is skipped, with its reason.** No behaviour changes: D10 shows
every touched Go file's code identical to `e52b4c2`'s. What can be wrong is a
sentence, which D15 holds. The report records the skip under its own heading.

**D17. Disk and load before any gate.** Forced by the measurements: free space
was below the 16 GiB floor once during verification, and the Go self-test
fails there. Before Task 0's self-tests and again immediately before `task
check`: `go clean -cache`, then `df -k`; below 16 GiB, stop. `check:coverage`
rebuilds the cache before `check:mutation` measures, so a launch with little
margin can still be refused after half an hour (Q11). `uptime` first; the
`TestQAMCPConnect*` deadline tests have failed under load before. `task check`
is launched in its own session (`start_new_session=True`) on a tree the review
has settled.

**D18. One commit for the change, the report in its own.** Forced by the band
(a file's cut and its row land together), by the chain (a row and its
citation land together) and by the pointers (`SPEC-017` needs its target in
the same tree). C1 carries the ticket, this plan, SPEC-017, the SPEC-013,
SPEC-011 and SPEC-009 edits, the register, the three production files, the
test files' citation lines and the ledger. C2 carries the report and, per
Q10, one debt entry.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository; the scratchpad, `$S`.

D17's disk steps; build D10's program and run its loop; `--report` for the
nine files; `check-requirements-chain.py .`; `check-comments.py main`;
`check-doc-owner.py .`; `gofmt -l internal/gateway/`; both mutation
self-tests; Done items 1 and 2's greps; `requirement-id` on the path.

**Done when:** the outputs match Measurements: nine `same`; `253 rows, 206
test files, 10 specifications`; `251 files ... clean`; `80 files`; only
`scenario_test.go`; both self-tests `OK` (the Go one only above the floor);
`1` and `1`.

### Task 1 — SPEC-017

**Files:** `docs/specifications/017-removing-an-actor.md`, new.

D1, D2; the `specification` skill's steps 1 to 5 and 7. The Requirements line
reads `None yet; the sort of 2026-10-02-removing-an-actor-has-a-record-design.md
fills it` until Task 7.

**Done when:** `grep -c '^## '` prints 5; no `Why`, no `Rejected`, no
measurement, no line number, no other project's name; every sentence names a
symbol and a sentence-to-symbol table is kept for Task 8; every "only",
"never", "every" and "none" names its search; the chain prints `11
specifications`.

### Task 2 — SPEC-013 and SPEC-011

**Files:** `docs/specifications/013-authorization.md`,
`docs/specifications/011-the-connection.md` (per Q8).

D3's sentences.

**Done when:** `grep -c 'has no record of its own'` on SPEC-013 prints 0, and
`grep -n handleRemoveActor` on it prints only the dispatch sentence; `git diff`
on each shows only D3's lines.

### Task 3 — `server.go`

**Files:** `internal/gateway/server.go`.

D6 under D9.

**Done when:** D10 prints `same` at 4657; `--report` shows `banned 0` and
`blocks>6 0`; `grep -c serveWS` prints 0; `gofmt -l` silent for it;
`check:doc-owner` ends `80 files`; `go doc ./internal/gateway Server` and `New`
print D6's text.

### Task 4 — `convert.go` and `join.go`

**Files:** `internal/gateway/convert.go`, `internal/gateway/join.go`.

D7, D8 under D9.

**Done when:** D10 prints `same` at 850 and 399; `--report` shows `banned 0`
and `blocks>6 0` for both; `grep -c "(server.go's" internal/gateway/convert.go`
prints 0; `grep -c 'DEMOTES' internal/gateway/convert.go` prints 0.

### Task 5 — Local gates, first pass

**Files:** none changed.

`gofmt`, `go vet ./internal/gateway/`, `go test -count=1
./internal/gateway/...`, D10's loop, `task check:comments` (expected to refuse
the three files for the band and nothing else), `task check:doc-owner`, `task
check:new-prose`, `task lint`.

**Done when:** each prints its completion line, `check:comments` excepted as
stated.

### Task 6 — Rows

**Files:** `docs/requirements.md`, by the dispenser and then evidence cells by
hand.

After the plan's sign-off: `requirement-id "<rule>"` once per accepted row and
once for W, in D4's order.

**Done when:** the register's last id is `VTT-253` plus the number dispensed;
W's cell reads `**OPEN — no test yet**`.

### Task 7 — Citation lines and Requirements lines

**Files:** `internal/gateway/remove_actor_test.go`, `server_test.go`,
`convert_test.go`, `cmd/vtt/scenario_goldens_test.go` (per Q6); the
Requirements lines of SPEC-017, SPEC-013 and SPEC-009.

D5.

**Done when:** `python3 tools/check-requirements-chain.py .` prints `<253 + N>
rows, 206 test files, 11 specifications; every citation resolves and every
row's evidence holds`; W has no citer; D10 prints `same` for every test file;
`--report` shows each test file's share unchanged and `cites` up by its
citation lines.

### Task 8 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

D15. Findings are fixed and the affected task's "done" re-run; the review
settles before Task 9. If the reviewer dies on a model's limit, say so and
re-dispatch with the same brief on `fable`.

**Done when:** the review record names every surviving block's kind, every
deleted block's outcome, every SPEC-017 sentence's verdict and every row's
red-making edit, with no open finding; D10 prints every `same`.

### Task 9 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

D11.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly the three
rows, each lowered; `task check:comments` ends `clean`; Done item 4's grep
prints `banned 0` and `blocks>6 0` on all three lines.

### Task 10 — Self-tests, breaks, the whole gate

**Files:** none in the repository.

D12's self-tests on the final tree; D14 in a scratch clone, deleted
afterwards; then D17 and `task check` whole, once.

**Done when:** both self-tests print `OK`; each break gives its red; `task
check` exits 0 with every step's own verdict, `check:mutation` included.

### Task 11 — Commit, then the report

**Files:** C1 per D18; then `docs/reports/2026-10-02-removing-an-actor-has-a-record.md`
and, per Q10, `docs/verification-debt.md`.

`git add` and `git commit` in separate calls; `git show --stat HEAD` checked.
The C1 message lists the ledger rows old and new, D10's counts and the ids.
The report, per the `implementation-report` skill: shares before and after,
blocks kept by kind, the facts SPEC-017 and SPEC-011 took and the facts
dropped with reasons, every row with its id and every refusal in one line,
W's recipe (the stale-snapshot test), the false sentences, the breaks' lines,
D13's list, the gaps below, the rule-9 answer.

**Done when:** `git show --stat HEAD~1` lists C1's files and nothing else;
`git diff --quiet e52b4c2 -- internal/engine internal/campaign internal/store
contract client/src` exits 0.

## Commits

| Commit | Carries | Gate steps |
|---|---|---|
| C1 | D18's list | Tasks 5, 9 and 10, then the pre-commit hook |
| C2 | the report, and the debt entry per Q10 | the pre-commit hook |

Push after C2; pre-push takes about three minutes, let it finish.

## Gaps that travel with this plan

1. **Done item 4 reaches every block**, so the sweep changes 43 blocks: the
   24 over the bound or carrying a banned term that the problem paragraph
   counts, and 19 descriptions under the bound. Q9.
2. **The records the work moves exceed the ticket's section**: SPEC-011 takes
   a sentence (Q8), SPEC-009 a Requirements id (Q5), and SPEC-013 two pointer
   sentences, a sentence for row H and a Requirements id. The ticket's writer
   grows the section; this plan does not edit it.
3. **Done item 1 asks SPEC-017 to state the batch's contents, order and
   atomicity, which SPEC-007 states already.** D2 points at SPEC-007 and
   states the handler's construction. Q2.
4. **A removal appended whole is observed by no test (P5)**; W is OPEN with a
   recipe.
5. **The removal batch's participant and role stamping is observed by no test
   (P8)**, and a refused door that writes the door stays green (P9b). Q10.
6. **`add_actor`'s actor beyond its id is observed only in `cmd/vtt`** (H4b);
   `TestToEventAddActorProducesActorAdded` asserts the id alone. Q6.
7. **Three comments are false and one is false as worded** (Measurements);
   none may reach SPEC-017.
8. **Pointers left weaker outside the ticket's files** (D13).
9. **Disk.** Free space read 15.7 to 17.1 GiB with a 2.0 GiB cache;
   `check_mutation_test.py` failed two of 107 at the base on the floor. Q11.
10. **Load** ran to 14 during verification; the MCP deadline tests are known
    to fail under it.
11. **The "green" probes ran over the gateway package**, bounded by the greps
    in check 3; only H4b ran over `cmd/vtt` too.

## Questions for sign-off

1. **Where does `remove_actor`'s record live?** Recommend a new
   `docs/specifications/017-removing-an-actor.md` (D1), not a section of
   SPEC-007: SPEC-007's one decision is the wire contract, and the handler's
   decisions are a second one, which the `specification` skill puts in a file
   of its own; and SPEC-013 already sends each batch handler to a record of
   its own, so a section of SPEC-007 would make `remove_actor` the one whose
   record is the wire contract.
2. **Does SPEC-017 point at SPEC-007 for the batch's shape and atomicity
   rather than restate them, with A, B, J and W on SPEC-017's Requirements
   line?** Recommend yes: SPEC-014 does the same for `load_map`, SPEC-007
   promises a consumer needs nothing else, and one order stated in two
   records is `catches.md` item 13. SPEC-007 is not edited.
3. **Rows C and D, the fold's refusals: refuse here?** Recommend refuse: only
   edits to `engine.Apply` red them (P6, P7), "appends nothing" is VTT-162,
   and the command's half of D is W. Rows under SPEC-017 would make it the
   fold's record by the back door; SPEC-017 states both and points at
   `engine.Apply`, and their tests stay uncited until the fold has a record.
4. **Row W, a removal appended whole: OPEN — no test yet?** Recommend OPEN:
   the per-envelope append is green (P5); the scratch stale-snapshot test reds
   it and is green at the base, and the report carries it as the recipe.
   Writing it is outside a comments-only ticket.
5. **Row G: accept as "is refused", its id on SPEC-009's Requirements line?**
   Recommend yes: SPEC-009 states the refusal and owns the door; "changes
   nothing" stays prose because P9b is green. SPEC-009 is not in the ticket's
   section.
6. **Row H: accept, with a sentence in SPEC-013 and `TestScenarioGoldenStreamsHaveNotDrifted`
   among its evidence?** Recommend yes: no record states that conversion
   carries every field, and the golden is the only test that reds
   `add_actor`'s kind dropped (H4b); its existing `// VTT-240` line takes the
   id. `cmd/vtt/scenario_goldens_test.go` is outside the ticket's list.
7. **Row J, a removal's result carries its batch's first sequence: accept
   though the ticket does not list it?** Recommend accept, as VTT-137 and
   VTT-175 are for the other three batch handlers.
8. **SPEC-011 takes one sentence: a presence frame that cannot be encoded is
   sent to nobody and ends no connection?** Recommend yes: only
   `announcePresence`'s comment says it, and SPEC-011 already says what an
   unencodable head and event do. The ticket's section grows by it.
9. **Sweep every block in the three files, not only those over the bound or
   carrying a banned term?** Recommend yes: Done item 4 says every block left,
   and the 19 under-bound descriptions in D6 to D8 would fail Phase 4b's
   reading.
10. **One debt entry in C2 for what no test observes: the removal batch's
    participant and role (P8) and a refused door that writes the door (P9b)?**
    Recommend yes: `CLAUDE.md` names `docs/verification-debt.md` as the one
    file for known coverage gaps. It is outside the ticket's list.
11. **Disk: may the plan stop before `task check` until more space is free?**
    Free space read 15.7 to 17.1 GiB with a 2.0 GiB cache, so `go clean
    -cache` gives at most about 19 GiB, and `check:coverage` rebuilds the
    cache before `check:mutation` measures its 16 GiB floor. Recommend that
    Patrik frees space until `df -k` shows at least 20 GiB after `go clean
    -cache`, or accepts that the run may be refused at `check:mutation`.
12. **One commit for the change, the report in its own (D18)?** Recommend yes.
