# Each party member keeps its own log: implementation plan

- **Ticket:**
  `docs/superpowers/specs/2026-10-16-each-party-member-keeps-its-own-log-design.md`,
  in the revision written after the owner's rulings of 2026-10-16.
- **Verdict:**
  - The first verification returned the ticket (R1-R4).
  - The second, against R1-R4 only, PASSES.
  - Checking the resume rule, as the coordinator asked, found gap G1. The
    owner answered it on 2026-10-16 with option (a), and the ticket's resume
    rule now carries the hide.
  - The probes behind both verdicts ran in a scratch copy of `4fe0472` and
    are not committed.
- **Signed off** by the owner on 2026-10-16, with G1 answered (a).
- **Base:** main `4fe0472`.
- **Branch:** `feat/each-party-member-keeps-its-own-log`.

Written by the agent that verified the ticket. Before writing it, that agent
read:

- `internal/store/store.go` and `subscribe.go`, and the store's tests;
- `internal/campaign/campaign.go` and `foldprefix.go`, and the campaign's test
  helpers and property walk;
- `internal/perceive/projection.go` (`go doc -all`);
- `engine.Apply`'s session, actor, token, condition, narration and note arms;
- `internal/eventgen/model.go`;
- `internal/gateway/project.go`, `seat.go`, `export_test.go`, `resume_test.go`
  and `keystone_test.go`'s corpus readers;
- `.go-arch-lint.yml`, `check-mutation.py`'s `PACKAGES` and runner, and
  `tools/coverage-thresholds.txt` and `comment-ceilings.txt`;
- SPEC-015, SPEC-016 and SPEC-019, and ticket 1's plan and report.

## Constraints that bind every task

These are named here, not restated.

- **CLAUDE.md rules:**
  - 2: `task check` is never weakened, and a gate change is its own reviewed
    decision (Task 1).
  - 3: additive contract. This plan changes no `.proto`.
  - 4: one fold. A character log is folded by `foldEvents` on open and by
    `engine.Apply` per frame at append time, the shape `Append` and
    `AppendBatch` already use for the world. There is no second loop of the
    fold's own.
  - 5: no game-system vocabulary. The commit hook refused the resource name
    `hp` in ticket 1, so fixtures name resources `pool` or `stamina`.
  - 8: citations name durable things.
  - 9: RPTool first (answered below).
  - 10: a comment is a warning or a pointer. History goes to the report.
- **The `dev-cycle` package's phases.** Tests first, a deliberate break for
  every new check, Phase 4a QA, a Phase 4b review before every commit, the
  report, then the specifications. They live in the package and are not
  repeated here. One Phase 4b review and one commit per task.
- **The owner's rulings:**
  - 2026-10-15: four tickets in order; one active character at a time; a late
    log opens with the present; the viewer's fold is the memory (VTT-312);
    no dual path, no flag, no migration; no log is ever regenerated; old
    solutions are deleted outright.
  - 2026-10-16:
    - Q1 (a): anchors are re-anchored.
    - Q3: a log pauses and resumes without a second opening.
    - Q5: frames that would not fold are not written.
    - Q2: the cost is accepted, with no memo.
    - G1 (a): a resume hides every token the paused board holds.
- **Decided by the ticket's writer, not asked:** Q4, entry identity, as the
  verifier recommended (D3).
- **SPEC-008:** ids come from `requirement-id`, after sign-off.
- **SPEC-010:**
  - a file with no ledger row may carry at most a 25% comment share;
  - ceilings only go down;
  - run `tools/check-comments.py` against both `main` and `HEAD` before every
    commit, and never delete and regenerate a row already committed on the
    branch.
- **SPEC-020:** an adjudication names the code it excuses.
- **SPEC-015's** sentence that `campaign.Append` and `campaign.AppendBatch`
  are the store's only append callers (`grep -rn 'c.log.Append'`) stays true.
  The store's one write keeps the name `AppendBatch` (D1).
- **Arch:**
  - `internal/gateway` may not import `internal/store`.
  - `internal/campaign` may not import `internal/sight` or
    `internal/gateway`.
- **Operational lessons in the project memory, binding here:**
  - Review packages are built with `git diff HEAD`.
  - Never stash while a reviewer reads.
  - Settle the review before starting the gate, and re-point mutation keys
    last.
  - Read the machine's load with `os.getloadavg()` and launch `task check`
    with `start_new_session=True`.
  - Run `check:new-prose` by hand before the last gate run (ticket 1 lost a
    run to it).
  - Verify what landed after every commit.

## Rule 9: how MapTool solves this

Checked against `~/dev/RPTool/maptool` at `f4b7fef6c`.

**Borrowed: the pause.**

- `Zone.exposeArea(area, tok)` adds to a token's exposure history
  (`exposedAreaMeta`, keyed by `Token.exposedAreaGUID`) under individual fog
  or on a personal server, and only when `AppUtil.playerOwns(tok)` holds.
- `Zone.exposeArea(Area, Set<GUID>)` adds only for a token with sight
  (`tok.getHasSight()`).
- Nothing clears a token's history when ownership ends. Only explicit
  actions do: `clearExposedArea`, `setFogArea`, and `TokenPopupMenu`'s
  `ClearSelectedExposedAreaAction`.
- So MapTool's memory per pair of eyes pauses while nobody owns the token,
  keeps what it has, and resumes when someone does. The owner's Q3 ruling is
  the same shape.

**Where MapTool does not help: what a resume must correct.**

- Its memory is accumulated terrain only.
- Every client is sent the whole campaign (`Zone.toDto`,
  `MapToolServer.addRemoteConnection`'s `SetCampaignMsg`), so token
  positions are never part of a per-viewer memory and cannot go stale.
- This platform's character log holds a board. G1 is that board after a
  pause, and MapTool has no precedent for it.

**Refused, as in ticket 1:** the distribution model.

**Not met there at all:** a per-character event history, anchored
narration, and a session a memory must stay inside.

## Decisions this plan makes

Each decision names what forced it.

**D1. The store has one write, `AppendBatch(envs, entries)`, and
`store.Append` is deleted.**

- `Entry` is `{ActorID, Cause, Env}`:
  - `Cause` is the index in `envs` of the world event that caused the entry,
    or -1 for none;
  - `Env.Sequence` must be 0;
  - the event id may be empty.
- The store stamps the world sequences, then each actor's next sequences from
  `MAX(seq)` of that actor. It inserts the world rows first, then the entries
  in the order given, in one transaction.
- On every failure path it resets every world envelope's and every entry's
  `Sequence`.
- `campaign.Append` passes one envelope.
- The tests that called `store.Append` call a helper over `AppendBatch`,
  one per test package, since a `_test.go` helper is visible only in its own
  package:
  - the store's tests;
  - `internal/identity/identity_test.go`;
  - the five campaign tests that call `s.Append`: `forward_only_test.go`,
    `qa_cmd_id_bound_test.go`, `qa_id_bound_test.go`,
    `qa_ruleset_bound_test.go` and `writer_hold_test.go`.

*Forced by:*

- Item 2's rule spans the world rows and the entries in one transaction.
- `store.Append` has no production caller once `campaign.Append` writes
  entries.
- The owner's "delete old solutions outright".
- SPEC-015's grep, which the name `AppendBatch` keeps true.
- No register row cites a store test (`grep internal/store/
  docs/requirements.md` is empty).

**D2. The schema is the arc design's §5, unchanged.**

`character_events(actor_id, seq, world_seq, event_id, occurred_at, payload)`,
with primary key `(actor_id, seq)` and no `UNIQUE` on `event_id`.

- `world_seq` is the world sequence of the event at whose append the entry
  was written, and 0 for an opening or resume entry.
- `session_id` lives in the payload.

*Forced by:*

- Design §5.
- Q4: one world event remembered by several logs keeps its id in each.
- The anchor rule (D9) leaves out the entries no world event caused, and it
  must still know them after a reopen, so the table marks them.

**D3. An entry is stored as the decision gives it.**

- Forwarded entries keep the world event's `event_id`, `session_id` and
  `occurred_at` (`forwardable` keeps them).
- Synthesized entries carry none of the three.
- The exceptions are the `SessionStarted` of an opening or a resume, which
  carry the open session's id. `engine.Apply` refuses a session id of 0
  bytes.

*Forced by:* the ticket's writer's Q4 decision, taken as the verifier
recommended it and not put to the owner.

**D4. Append and AppendBatch share one path.**

`appendLocked(envs)` does, in order:

- validate (today's loop);
- fan out (D5);
- persist (D1);
- on success: commit the characters' memory, apply live, notify;
- on a store failure: discard the pending entries and rebuild the decisions
  (D8).

`AppendBatch`'s empty-batch refusal stays its own; `Append` has none. The poison
contract is unchanged.

*Forced by:*

- Items 3 and 8 hold for both entry points, so the rules must live in one
  place.
- Gremlins measures one implementation rather than two.
- *Partly a preference:* the merge could be avoided by calling a shared
  fan-out from both.

**D5. The fan-out runs after validation, envelope by envelope, against its
own copy of the world.**

For each envelope `i` (with `w` a fresh `c.state.Snapshot()`):

- `before := w.Snapshot()`; apply the stamped clone to `w`.
- Walk, by sorted id, every actor holding a log state together with every
  party member in `w`:
  - **Not a party member in `w`:** the log is paused; write nothing.
  - **Log empty:** open (D6), then decide.
  - **Paused:** resume (D7), then decide.
  - **Otherwise:** decide.
- To decide: call `decision.Event(Eyes{Shoulder: id}, clone_i,
  workingFold, w)`, rewrite the anchors (D9), and fold a stamped copy of each
  frame onto the working fold.
  - If any fails, the actor's frames for envelope `i` are dropped, with
    `slog.Error` naming the actor and the world sequence, and a count (D10).
    The actor's decision then starts again as `perceive.New(eyes, nil)`:
    `Event` has already moved `sighted` past frames the log never got, and a
    `sighted` claiming more than the fold holds forwards frames the fold
    refuses (D6's probe), which would drop the next event too.
  - Otherwise they become entries with `Cause = i`.

*Forced by:*

- `Event` judges against the world after the event, and the opening against
  the world before it.
- The validation loop must leave every decision untouched when an envelope is
  refused.
- Q5.
- Measured: a world `Snapshot` with three 60x60 tiled scenes costs about
  0.5 ms, against 5-35 ms of sight per party member per event.

**D6. The opening is the open session's `SessionStarted`, the party, and the
public notes, sorted by key.**

- The party is `perceive.New(Eyes{}, nil).Perch(Eyes{}, empty, before, 0)`.
  An empty `Eyes` sees nothing (SPEC-016: "and otherwise none, the empty
  shoulder included"), so
  `Perch` gives exactly the party members' introductions.
- A public note becomes a `NoteUpserted` of its title, text and visibility.
- The decision starts as `perceive.New(eyes, nil)`. `perceive` is unchanged.

*Forced by:*

- The ticket's list: "the open session, the party and the public notes".
- "What it touches" item 4: perceive changes only if needed.
- Probe: an oracle seeded with exactly this opening matches every log over
  eventgen seeds 1-12 and the corpus (86 logs, 8424 events, 0 mismatches).
- Probe: starting the decision as `New(eyes, before)` where the actor is
  already a party member (the case of `TestALogOpensForAPartyMemberThatHadNone`)
  fails. An NPC it sees takes a condition; the decision forwards a
  `ConditionApplied` after introducing the NPC with it, and the fold refuses
  the duplicate. Starting from `New(eyes, nil)` folds.

**D7. A resume brings the log to the world's open session, and its decision
starts as `perceive.New(eyes, nil)`.**

Given L, the session the log's fold holds open, and W, the one `before` holds
open:

- L open and W closed, or a different session: a bare `SessionEnded`.
- W open and L closed, or a different session: a `SessionStarted` with W's
  id.
- Both entries carry world sequence 0.
- Then, by G1's answer (a), a `TokenHidden` for every token the log's board
  holds, by id.

*Forced by:*

- The ticket's resume rule.
- `engine.Apply`'s session refusals.
- Probe: a session that ended and another that started during the pause
  gives `SessionEnded` then `SessionStarted`; the log folds, and so does the
  later `SessionEnded`.
- `New(eyes, nil)` because a decision whose `sighted` claims more than the
  fold holds forwards frames the fold refuses (D6's probe). An empty
  `sighted` corrects every held actor in sight.

**D8. After a failed store write, every active decision is rebuilt as
`perceive.New(eyes, c.state)`, and pending openings and resumes are
dropped.**

*Forced by:*

- The rule "an append that fails changes no character's memory".
- `Event` advances the `Projection`'s `sighted` (SPEC-016), and perceive
  exports no copy.
- VTT-312 makes a decision rebuilt from the world and the log's fold send
  what the continuing one sends.

**D9. An anchored narration is re-anchored onto its log's entries.**

- The range is the first to the last entry whose `world_seq` is non-zero and
  inside the narration's world range.
- When there is none, both ends are 0.
- It is done before the frame is folded.
- The campaign keeps each log's non-zero `(world_seq, seq)` pairs in order,
  rebuilt on open.

*Forced by:* the ticket's anchor rule, under the owner's Q1 (a), with the
opening and resume entries (world sequence 0) left out by D2.

**D10. A dropped frame set is counted, and the count is visible to tests
only.**

- The counter is per `Campaign`.
- It is read by `DiscardedForTest` in a new `internal/campaign/export_test.go`.
- The every-prefix test and the property walk require 0. Item 8's test
  requires 1.

*Forced by:*

- Q5 makes every stored log fold by construction, so item 7's test alone
  cannot fail on a fan-out whose frames do not fold.
- Gremlins runs `internal/campaign`'s own tests (ticket 1's D8).

**D11. Open folds every character log.**

- A log that does not fold refuses the `Open`, naming the actor.
- A log is active when it is non-empty and its actor is a party member at
  the world's head.
- An active log's decision is `perceive.New(eyes, world)` (VTT-312).

*Forced by:*

- The rule "after a reopen, a character's memory is the fold of its log".
- A log the campaign wrote always folds (Q5's trial), so a refusal means a
  write from outside, which SPEC-019 already treats so for the world's log.

**D12. `Campaign.CharacterLog(actorID) ([]CharacterEntry, error)`.**

- `CharacterEntry` is `{WorldSeq, Env}`, the store's field names, and is
  declared in `internal/campaign`.
- It reads the store, and returns `errPoisoned` when poisoned.

*Forced by:*

- Items 4 and 6 need the world sequence of each entry.
- `internal/gateway` may not import `internal/store`.
- Ticket 3's seat reads a log too; its cursor read is that ticket's.

**D13. How the failure tests force their failures.**

- Items 2 and 3: a `BEFORE INSERT` trigger on `character_events` with
  `RAISE(ABORT, …)`, created and dropped on a second `sql.Open` handle.
  Probe: such a trigger from a second handle fails the store's insert after
  an earlier row of the same transaction; the transaction rolls back, every
  sequence is reset, and the retry succeeds once the trigger is dropped.
- Item 8, with the campaign closed:
  - `store.AppendBatch` writes one world `NarrationAdded` and, as its entry,
    a `ConditionApplied(x, c)` onto A's log, where A sees x.
  - The campaign then reopens, and its next append is the world's
    `ConditionApplied(x, c)`.
  - The decision forwards it to A, and A's fold refuses the duplicate.

*Forced by:*

- The ticket's first "could not be established" point.
- No production seam: `campaign` may not import `internal/testdb`.

**D14. Where the whole-system tests live, and what they cost.**

- Item 7 lives in `internal/campaign`, on a sample of seeds that the task
  measures to meet the guard. The ensemble over seeds 1-12 has 25 logs with a
  `SessionEnded` of a session open at their opening.
- Item 4 lives in `internal/gateway` (tier 2), on eventgen seeds 1-6 and the
  corpus, plus Task 6's hand-written pause histories.
- Each task records the suite's runtime in its commit, and the report records
  `check:mutation`'s runtime for store, campaign and gateway.

*Forced by:*

- Gremlins reruns a package's whole suite per mutant. Campaign's suite takes
  about 1 s today, against about 91 s of mutation.
- Ticket 1's precedent of sampled eventgen seeds (its D13).

**D15. Item 4 compares at every event after which the actor is a party
member.**

- At those events: envelope for envelope, the sequence and the anchor range
  aside.
- At every other event: the log has no entry.
- The `Projector` is seeded with the log's entries so far at the opening,
  and again at every resume.

*Forced by:*

- The rule "an event is written to an actor's log only when the actor is a
  party member in the world after that event".
- A spectator perched on a paused actor is still sent frames.
- Neither eventgen seeds 1-12 nor the corpus resumes a log: 9 removals of a
  party member, 0 resumes, and no pause at all in the corpus. So the
  resumed case comes from Task 6's hand-written histories.

## Gaps carried from the verdict

- **G1 (answered (a) by the owner, 2026-10-16).** After a pause by a kind flip, the log's board
  is stale. It is the question below.
- **G2 (closed).** Item 4 now says what D15 does: seeded again at each
  resume, compared envelope for envelope at the events after which the actor
  is a party member.
- **The ticket's own two:**
  - (1) is D13.
  - (2), the cost after the third ticket, is not this ticket's.
- **Established in review, not a gap:** `internal/eventgen` never reuses an
  actor id (`addActor` names `prop-actor-N` from a counter that only rises,
  and `removeActor` adds nothing back), so it pauses a log only by a removal
  and never resumes one. Resumes come from Task 6's hand-written
  histories.

## Questions for the owner

**G1. What a resume does with the board a paused log holds.** Answered
2026-10-16: (a).

The case: a party member stops being one by a grant with
`ACTOR_KIND_NON_PARTY` and keeps its tokens. Its log keeps the board as it
stood. When it resumes, the decision trusts that board: it never re-states the
square of a token it holds that is still in view. Two probes:

- **Kind flip with moves.** Party members A and B stand in sight of each
  other. A is flipped. Both tokens move. A is flipped back.
  - The log folds.
  - It shows both tokens on their pre-pause squares, A's own included.
  - Each stays wrong until it next moves.
- **Kind flip with a reused id.** During the pause, the DM removes an NPC
  that A's board held, with its token, and adds a different actor under its
  id.
  - At the resume, the decision withdraws the old actor before it hides that
    actor's token, and the fold refuses.
  - Under Q5 the log is written nothing for the resume, or for any event
    after it: the log stops for good, silently.
  - A removal pause does not do this: an actor loses its tokens before its
    removal, and that blinds it while still a party member.

Options:

- **(a)** A resume also hides every token the log's board holds, before the
  decision's frames. The decision then places what is in view on its present
  square.
  - Probe: both histories fold, the squares are right, and nothing is
    dropped.
  - Cost: one `TokenHidden` per held token at a resume. A client redraws
    those tokens once.
  - The ticket's resume rule gains a clause, which is its writer's to add.
- **(b)** Accept it.
  - Who notices: the player of a character that was flipped while tokens
    moved. After ticket 3 their own token is on the wrong square until it
    moves.
  - The reused-id case stops that character's log forever.
- **(c)** Send a `TokenMoved` for each held token whose square differs.
  - It does not cure the reused-id stall, since the withdraw still comes
    before the hide. It needs (a)'s hide for those tokens anyway.

**Recommendation: (a).** It reuses a frame the decision already sends, it is
one sentence of rule, and it is the only option that keeps the log from
stopping. Task 6 carries it as its last step and drops that step if he answers
(b).

## Tasks

The tasks are in dependency order. Each one is one Phase 4b review and one
commit.

### Before Task 1: allocate the requirement ids (done 2026-10-16)

The sort ran after sign-off over the ticket's twelve rules and D11. It
accepted thirteen, each allocated by `requirement-id`, each row reading
`**OPEN — no test yet**` until its test lands:

| Id | Rule, in short | Test that holds it | Task |
|---|---|---|---|
| VTT-315 | one transaction for the world event and its entries | `TestAFailedCharacterWriteLeavesNoWorldEvent` | 2 |
| VTT-316 | written only while a party member after the event | `TestANonPartyActorHasNoLog`, `TestALogPausesWhileItsActorIsNotAPartyMember` | 4, 6 |
| VTT-317 | the opening, as the world stood before the event | `TestALateLogOpensWithThePresent`, `TestALogOpensWhenAGrantMakesAPartyMember`, `TestALogOpensForAPartyMemberThatHadNone` | 4 |
| VTT-318 | a resume: no second opening, the session, the hide | `TestALogPausesWhileItsActorIsNotAPartyMember`, `TestANewActorUnderAnOldIdContinuesItsLog` | 6 |
| VTT-319 | the decision's frames for those eyes, and nothing else | `TestACharacterLogIsWhatASpectatorPerchedOnItIsSent` | 8 |
| VTT-320 | frames that would not fold are not written | `TestFramesThatWouldNotFoldAreNotWritten` | 4 |
| VTT-321 | own sequence, from 1, no gaps | `TestACharacterLogNumbersItsEntriesFromOne` | 2 |
| VTT-322 | an anchored narration names its own log's entries | `TestAnAnchoredNarrationNamesTheEntriesOfItsOwnLog` | 5 |
| VTT-323 | every log folds at every prefix | `TestEveryCharacterLogFoldsAtEveryPrefix` | 7 |
| VTT-324 | a reopen introduces nothing again | `TestACharacterLogContinuesAfterReopen` | 4 |
| VTT-325 | a failed append changes no memory | `TestAFailedFanOutDoesNotAdvanceACharactersMemory` | 4 |
| VTT-326 | an entry is never changed or removed | `TestACharacterLogContinuesAfterReopen` (every entry reads back unchanged after later appends and a reopen) | 4 |
| VTT-327 | Open refuses a log that does not fold | `TestOpenRefusesACharacterLogThatDoesNotFold` | 4 |

Refused, one line each:

- "An actor that is never a party member has no log": a consequence of
  VTT-316, held by the same tests.
- D10's "a dropped frame set is logged": no ticket item asks for it, and the
  count D10 adds is test apparatus, not a rule.
- D3's entry identity: how an entry is stored. Its observable half is
  VTT-319's comparison (Task 8's third break).
- D1's and D4's shapes (one store write, one append path, `store.Append`
  deleted): how, not what.

**True when done:** `task check:requirements-chain` is green.
**Gate:** pre-commit hook.

### Task 1: `campaign` may depend on `perceive` (a gate change)

**Files:** `.go-arch-lint.yml`.

- Add `perceive` to `campaign`'s `mayDependOn`.
- Rewrite the `perceive` entry's last sentence ("campaign gains its edge in
  the ticket that calls it, not before"), which this makes false, as a
  pointer to this ticket.

**True when done:**

- `go-arch-lint check` is green.
- `TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight`
  passes, unchanged.

**Deliberate breaks:** each is made in the working tree, seen red, and
undone.

1. A `campaign` file importing `perceive` with the edge removed: red. Seen
   in the probe: "Component campaign shouldn't depend on …/perceive".
2. The same file importing `internal/sight`: red with the edge in place.

**Gate:** pre-commit hook (arch included).

### Task 2: the store writes a world event and its entries in one transaction (items 1, 2)

**Files:**

- `internal/store/store.go`:
  - D1 and D2;
  - `CharacterEntry{WorldSeq, Env}`;
  - `ReadCharacterLogs() (map[string][]CharacterEntry, error)`;
  - `ReadCharacterAfter(actorID, afterSeq)`, which fails loudly on a corrupt
    payload, as `readAfterLocked` does;
  - `Append` deleted.
- `internal/store/*_test.go`:
  - `TestACharacterLogNumbersItsEntriesFromOne`;
  - `TestAFailedCharacterWriteLeavesNoWorldEvent` (D13's trigger, on an
    entry after the world row);
  - tests for an entry with a non-zero sequence, an empty actor id or an
    out-of-range cause, each refused with every sequence reset;
  - entries persist across a reopen;
  - a corrupt entry fails its read;
  - every `s.Append(env)` becomes a helper over `AppendBatch`, and
    `store_test.go`'s `Append` tests move to the batch form.
- `internal/campaign/campaign.go`: `Append` calls
  `c.log.AppendBatch([]*vttv1.Envelope{env}, nil)`. Behaviour is unchanged.
- `internal/identity/identity_test.go` and the five campaign tests D1
  names: each package's helper.
- `internal/campaign/append_sequence_validation_test.go`: its header names
  `store.Store.Append` and `store.Append`, which this task deletes.
- `docs/requirements.md`: evidence for items 1 and 2's rows.
- `tools/comment-ceilings.txt`: `check-comments.py --write-ledger`, which only
  lowers, when a file falls more than the band under its ceiling.

**True when done:**

- Store, identity and campaign suites green.
- The two named tests pass, and fail to compile on `main`.
- `grep -rn 'c.log.Append' internal/campaign` still finds the appends.

**Deliberate breaks:**

1. Insert the entries in a second transaction: item 2 reds.
2. Number entries from the table's `MAX(seq)` rather than per actor: item 1
   reds.
3. Skip resetting the entries' sequences on failure: item 2 reds.
4. Stamp entries before the world rows are validated: the up-front refusal
   test reds.

**Gate:** pre-commit hook; `go test ./internal/store/ ./internal/campaign/
./internal/identity/ -race`; `check:requirements-chain`; `check-comments.py`
against `main` and `HEAD` (`store.go` at 20.8% or below).

### Task 3: one append path, unchanged in behaviour (D4, without the fan-out)

**Files:** `internal/campaign/campaign.go`.

- `appendLocked(envs)`, called by both `Append` and `AppendBatch`.
- Their doc comments are rewritten to warnings and pointers (rule 10); the
  history they carry goes to the report.

**True when done:**

- Every campaign and gateway test passes unchanged:
  `git diff --stat HEAD -- '*_test.go'` is empty.
- The goldens are byte-identical.

**Deliberate break:** stamp a batch's session ids against `c.state` rather
than the evolving snapshot.

- Expected: `appendbatch_session_test.go` reds. This is not yet run.
- If it stays green, the review names the test that reds, or records that
  none does.

**Gate:** pre-commit hook; `go test ./internal/campaign/ ./internal/gateway/`;
`check-comments.py` against `main` and `HEAD` (`campaign.go` falls from
49.3%), and `tools/comment-ceilings.txt` lowered with `--write-ledger` when
it falls more than the band.

### Task 4: the fan-out, the opening, and the memory across a failure and a reopen (items 3, 5's first four, 8, 9)

**Files:**

- `internal/campaign/fanout.go` (new, at most 25% comment): the log state,
  D5, D6, D8, D10 and D11, and `CharacterLog` (D12).
- `internal/campaign/campaign.go`: `rebuildLocked` and `appendLocked` call
  into it.
- `internal/campaign/export_test.go` (new): `DiscardedForTest`.
- `internal/campaign/character_log_test.go` (new), each test citing its id:
  - `TestANonPartyActorHasNoLog`.
  - `TestALateLogOpensWithThePresent`: a session open with an id read back
    from the world; a public note, a secret note, and narration before the
    party member is added. The log opens with that session's id, the party
    and the public note only, and holds no narration.
  - `TestALogOpensWhenAGrantMakesAPartyMember`.
  - `TestALogOpensForAPartyMemberThatHadNone`:
    - world events are written with `store.AppendBatch(envs, nil)` on a
      closed directory;
    - the campaign opens it;
    - the next append is a `ConditionApplied` on an NPC the party member
      sees;
    - the log opens with the present, folds, and `DiscardedForTest` is 0.
  - `TestAFailedFanOutDoesNotAdvanceACharactersMemory`:
    - D13's trigger fails a `TokenMoved` that brings NPC x into sight;
    - after the trigger is dropped, a `ConditionApplied` on x gives the log
      nothing, and nothing is dropped;
    - the move appended again introduces x with its condition;
    - the log folds.
  - `TestFramesThatWouldNotFoldAreNotWritten` (D13): the log gets nothing
    for that event, and the next event that reaches the character is written
    and folds (D5's restart).
  - `TestACharacterLogContinuesAfterReopen`.
  - `TestOpenRefusesACharacterLogThatDoesNotFold` (D11, VTT-327): an entry
    that does not fold, written on a closed directory, makes `Open` fail
    naming the actor.
  - A scene load in one `AppendBatch` gives the party member a non-empty log
    in batch order that folds.
- `docs/requirements.md`: evidence.

**True when done:**

- The new tests pass.
- Every existing campaign and gateway test passes.
- `TestRebuildEqualsLiveProperty` is green.
- The goldens are byte-identical.

**Deliberate breaks:**

1. Judge `Event` against `before`: the late-log test reds.
2. Drop the opening's `SessionStarted`: the late-log test reds.
3. Drop the public notes: the late-log test reds.
4. Start the decision as `New(eyes, before)`: the had-none test reds (a
   duplicate is dropped).
5. Commit the memory before the store returns: item 3 reds.
6. Skip D8's rebuild: item 3 reds (a dropped frame).
7. Write the frames that fail the trial: item 8 reds.
   Skip D5's restart after a drop: item 8's next-event clause reds.
8. Skip loading the logs on `Open`: item 9 reds (a second opening).
9. Skip D11's fold on `Open`: the refusal test reds.

**Gate:** pre-commit hook; `go test ./internal/campaign/ ./internal/gateway/
-race`; `check:requirements-chain`; `check-comments.py` against `main` and
`HEAD`, with `tools/comment-ceilings.txt` rows for the new files from
`--write-ledger`; `go-arch-lint check`.

### Task 5: anchored narration names its own log's entries (item 6)

**Files:**

- `internal/campaign/fanout.go`: D9.
- `internal/campaign/character_log_test.go`:
  `TestAnAnchoredNarrationNamesTheEntriesOfItsOwnLog`, with four cases:
  - a range wholly seen;
  - a range with several entries for one world event;
  - a range that starts before the log opened (the opening left out);
  - a range none of whose events reached the log (unanchored).
  - Each case also requires the log to fold and nothing to be dropped.
- `docs/requirements.md`: evidence.

**True when done:** the test passes, and so does everything that passed after
Task 4.

**Deliberate breaks:**

1. Keep the world range: the test reds (the narration is dropped, since its
   range is not behind it).
2. Include world-sequence-0 entries: the "before the log opened" case reds.
3. Map to the first entry only: the several-entries case reds.

**Gate:** as Task 4.

### Task 6: a log pauses and resumes (item 5's last two; D7; G1)

**Files:**

- `internal/campaign/fanout.go`: D7, with G1's hide (answer (a)).
- `internal/campaign/character_log_test.go`:
  - `TestALogPausesWhileItsActorIsNotAPartyMember`:
    - a grant with `ACTOR_KIND_NON_PARTY`;
    - then narration, a session ending and another starting, and both tokens
      moving;
    - then a party grant.
    - The log gets nothing between the grants and no second opening. It
      holds the world's open session after the resume, it folds, and
      nothing is dropped. Every held token is on its world square.
  - `TestANewActorUnderAnOldIdContinuesItsLog`: removal, then a different
    party member under the id. The withdraw precedes the introduction, and
    the log folds.
  - A reused-id case: an NPC the paused board held is removed
    and its id reused. Nothing is dropped after the resume.
- `docs/requirements.md`: evidence.

**True when done:** these pass, and so does everything that passed after
Task 5.

**Deliberate breaks:**

1. No session bridging: the pause test reds (a dropped `SessionEnded`).
2. A second opening on a resume: the pause test reds.
3. A resume decision as `New(eyes, before)`: the pause test reds where a
   held actor changed while paused.
4. No hide: the squares assertion and the reused-id case red.

**Gate:** as Task 4.

### Task 7: every character log folds at every prefix (item 7)

**Files:**

- `internal/campaign/character_log_property_test.go` (new):
  - eventgen seeds, the number set by D14's measurement, plus the corpus
    (`scenarios/goldens/*/stream.json`, read with a local `protojson`
    reader, sequences cleared) through `campaign.Append`;
  - `FoldPrefix` on every prefix of every log;
  - `DiscardedForTest` 0;
  - the ticket's three-part ensemble guard.
- `internal/campaign/property_test.go`: `runPropertyWalk` also requires every
  character log to read back unchanged across each reopen, and
  `DiscardedForTest` to be 0.
- `docs/requirements.md`: evidence.

**True when done:**

- The test passes.
- Its runtime and the campaign suite's are recorded in the commit message.

**Deliberate breaks:**

1. Undo D9 in the working tree: `DiscardedForTest` reds. A world range not
   behind the log's own sequence is refused, so its narration is dropped.
2. Drop the opening's `SessionStarted`: `DiscardedForTest` reds, since a
   `SessionEnded` of a session the log never started is dropped.
   - Undoing D7 cannot red here: no seed resumes a log (D15). Task 6's tests
     hold D7, and the report says so.
3. Make every decision return nothing: the guard's non-empty clause reds,
   not the fold.

**Gate:** pre-commit hook (tier 1); `go test ./internal/campaign/`.

### Task 8: a log holds what a spectator seeded with it is sent (item 4)

**Files:**

- `internal/gateway/export_test.go`: `NewProjectorWithSentForTest(v, sent)`.
- `internal/gateway/character_log_test.go` (new):
  `TestACharacterLogIsWhatASpectatorPerchedOnItIsSent`, as D15 says, over
  eventgen seeds 1-6, the corpus, and Task 6's histories driven through a
  campaign.
  - A per-seat vacuity guard: each log compares at least one event with
    frames.
  - A history guard: at least one log resumes.
- `docs/requirements.md`: evidence.

**True when done:**

- The test passes.
- Its runtime is recorded, aiming at 30 s or less (the gateway is tier 2 on
  pre-push).

**Deliberate breaks:**

1. Hand `Event` the world before the event: red.
2. Rewrite anchors onto world-sequence-0 entries: no red expected (anchors
   are excluded from the comparison). Named so the review sees that item 4
   does not hold D9, Task 5 does.
3. Clear `occurred_at` on forwarded entries: red (D3).

**Gate:** pre-commit hook; `go test ./internal/gateway/ -run
CharacterLogIsWhat`.

### Task 9: the gates hold the new code (item 12)

**Files:**

- `tools/coverage-thresholds.txt`: store and campaign floors raised to a
  little under what they measure, if they rose; never lowered.
- `tools/mutation-equivalents.txt`: survivors in store and campaign are
  killed with tests; an adjudication only with a one-sentence observable
  (SPEC-020). Run the gate's two self-tests first.
- `tools/comment-ceilings.txt`: `check-comments.py --write-ledger` lowers
  rows and adds the new files' rows. Never raise one by hand.

**True when done:**

- `check:coverage`, `check:comments` and `check:requirements-chain` are
  green.
- `check:mutation` is green for store, campaign and gateway, with each
  runtime recorded for the report.
- `check:new-prose` has been run by hand and is green.

**Deliberate breaks:**

1. Delete one adjudication added here, if any: the gate reds naming it.
2. Delete `TestAFailedFanOutDoesNotAdvanceACharactersMemory`: a campaign
   mutant on the commit order survives, or coverage reds. Record which.

### Task 10: `task check`, whole

- Check the load with `os.getloadavg()` first.
- Launch with `start_new_session=True`.
- Do not touch the tree during the run.

**True when done:**

- Green, which is item 12.
- Items 10 and 11 seen: `git diff --stat main -- scenarios/goldens` prints
  nothing, and the arch and goldens tests are green.

### Then the process's Phases 4a, 4b and 5

- **Phase 4a** runs on Task 4's and Task 6's behaviour. QA is given the
  rules, SPEC-016, the new specification's draft and the exported surface
  (`go doc ./internal/campaign`, `go doc ./internal/store`). The QA brief
  names `go-arch-lint`'s lists. The QA adjudications go in the report under
  their own heading.
- **The report**, `docs/reports/2026-10-16-each-party-member-keeps-its-own-log.md`,
  holds:
  - the probes' figures:
    - 86 logs and 0 mismatches;
    - 9 removals and 0 resumes;
    - the cost table: 5-8 ms sparse and 33-35 ms dense per party member;
    - 0.5 ms per snapshot;
  - G1 and the owner's answer;
  - D14's runtimes;
  - the comment history moved out of `campaign.go`;
  - the QA adjudications.
- **The specifications, after the report:**
  - **New: SPEC-021, the character log.** What it holds; when it opens,
    pauses and resumes; how an append writes it; the anchor rule; entry
    identity (D3); dropped frames; Open's refusal. Its consequences include:
    - a paused log misses narration and notes made public while paused;
    - a forwarded entry shares its event id with the world's.
  - **SPEC-016:**
    - Status names `internal/campaign/fanout.go` as a caller of `New`,
      `Event` and `Perch`;
    - the sequence section says a character log replaces each frame's
      sequence (SPEC-021);
    - "What this record does not decide" points to SPEC-021;
    - the empty `Eyes` the opening uses is described where `eyes` is.
  - **SPEC-015 and SPEC-019** are re-read. Their sentences on the append
    callers and on `Open` stay true by D1 and D11. After D4,
    `grep -rn 'c.log.Append'` finds the one call inside `appendLocked`, not
    one in each of `Append` and `AppendBatch`, so SPEC-015's sentence names
    that. Any other that does not hold is amended, and the report names it.

## What this plan does not do

- No seat reads a character log; `set_viewpoint` is unchanged; the gateway
  projection stays (ticket 3).
- No client change (ticket 4).
- No sight memo (Q2).
- No change to `.proto`, `engine`, `sight` or `perceive`.
