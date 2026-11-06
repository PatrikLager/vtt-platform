# Plan: a companion's sheet is seen with the companion

- **Ticket:** `docs/superpowers/specs/2026-11-06-a-companions-sheet-is-seen-with-the-companion-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-06 by an agent that did not write the ticket, read-only
    against `84c4ac8` on `feat/a-sheet-is-seen-with-its-character`, with the
    ticket uncommitted and intent-to-add.
  - Re-verified the same day against the ticket's revision, which closes G1
    to G4 and decides Q1 by a new "Done" item, a new rule and the client in
    "What it touches".
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and gremlins were not run. The probes (P5, P6, P11)
    ran on copies of the tracked tree outside the repository, deleted after.
- **To be committed as:** `docs/superpowers/plans/2026-11-06-a-companions-sheet-is-seen-with-the-companion.md`.
- **Base:** `84c4ac8`.
- **Branch:** `feat/a-sheet-is-seen-with-its-character`. The local branch
  `backup/per-property-visibility` holds the design the owner rejected; nothing
  here reads from it.
- **Owner's rulings taken in:**
  - 2026-11-06: "En spelare som "ser" sin medspelare, kan se dess character
    sheet, allt, för att göra det enkelt. En spelare som inte ser sin
    medspelare, kan inte se character sheet, för du kan inte se "karaktären" -
    DM ser allt. NPC och monster, kan endast ses av DM." With it, the rejection
    of visibility per property as too much for "ett spel bland vänner": no
    ruleset field, no API per role, no client change the code does not force.
  - B-1, 2026-10-22: event data follows sight strictly.
  - 2026-10-21: a near-theoretical edge with a small consequence gets no code,
    no test, no debt entry (D7, D12).
  - 2026-09-04: nobody uses the product; nothing is migrated (D8).
  - 2026-09-23: a fact has one home.
  - 2026-11-04: no counts or measured figures in anything written down.
  - 2026-10-26: the client's full mutation run is the owner's to ask for.
- **Signed off:** 2026-11-06, by the owner ("ok, kör"), with G6's reading of
  the name (a companion's name is not part of its sheet). Q1 is decided by the
  ticket's item 3.
- **Ids:** allocated at Task 0, after sign-off: R1 is VTT-657, R2 VTT-658, R3
  VTT-659, R4 VTT-660, R5 VTT-661, R6 VTT-662 and R7 VTT-663. VTT-634 to
  VTT-656 were dispensed and committed on the local branch
  `backup/per-property-visibility` for a design the owner set aside; by
  SPEC-008 a committed number is spent, so each has a withdrawn row here.

## The verdict, check by check

Item numbers are the revised ticket's.

1. **Every path resolves. By command** (P1). Every file the ticket names
   exists, and every symbol is in the file it names: `look` (with its comment
   "Know every party member, seen or not"), `shownAs`, `shownWith`,
   `introduce`, `correct`, `Resume`, `recall`, `transitions` and `classify`'s
   `ResourceChanged` arm in `internal/perceive/projection.go`; `describe`, its
   callers `renderFeed` and `renderTicker`, and "is in the party" in
   `client/src/view/spectator.ts`; `watchable` in
   `internal/gateway/viewpoint.go`; ADR-012's §2; SPEC-016, SPEC-021 and
   SPEC-022; the rows the ticket cites.
2. **"Done" is an observation. By command where one can reach it.**
   - Items 1, 2, 3 and 5 name tests that do not exist yet, so none could be
     run. What each will pin is absent today, by command: an unseen companion
     is introduced whole (P2, P3), and the Story tells every entry (P11).
   - Item 3 does not say its test fails today. R6's does: `buildFeed` keeps
     every entry today (VTT-449).
   - Item 4 now holds: it says which fixtures move, and keeps their
     assertions (G1, closed).
   - Item 6 is the gate.
3. **Each rule is breakable. A reading.**
   - "Introduced ... alone": K1, K2 and K5.
   - "Brought to the whole sheet at the first sight": K4; K6, K1's reverse,
     turns R1 red.
   - "A window does not describe ... a departure and a fresh arrival": K8 to
     K11.
   - "VTT-485 and VTT-471 are withdrawn and succeeded": a rule on the
     register, held by `check:requirements-chain` for the successors' evidence
     and by the reading review for the withdrawals.
   - The plan adds R5, the guard on the design's new comparison; K3 breaks it.
4. **The scope matches the claim. By command, then a reading** (P4, P5, P11).
   Every caller of `shownAs`, `shownWith`, `introduce`, `correct` and
   `statusFrames` is in `projection.go`. `describe` is called by `renderFeed`
   and `renderTicker`, and `renderFeed` is handed `buildFeed`'s entries. The
   Go change is smaller than "What it touches" item 1 says (D6), and the
   client's lands in `client/src/view/feed.ts`, not `spectator.ts` (G9). No
   `internal/gateway` test changes, and no existing client test (P11).
5. **No recorded decision is contradicted silently. A reading.**
   - ADR-012 §2's visibility per property is set aside by the owner's ruling,
     and the ticket says so.
   - VTT-485 and VTT-471 are overturned, and the ticket names each.
   - VTT-449, "The Story shows every entry of the log its window holds",
     becomes false as worded, and the ticket does not name it (G7).
   - SPEC-016's "a party member whole" and its roster consequence, SPEC-021's
     comparison and SPEC-022's Story move; the ticket names each.
   - B-1 and blueprint principles 16 and 17 are served, not moved.
6. **The records it moves. By command, then a reading** (P9). The section
   exists, and SPEC-016, SPEC-021 and SPEC-022 resolve. SPEC-015 and SPEC-024
   do not move: whom a seat may watch is `WatchableChanged`'s, and SPEC-024's
   "every participant is served every property" is about `/api/ruleset`'s
   declarations, not an actor's values.

G7 is the omission the skill's letter returns a ticket for. This plan carries
it as a gap, as `docs/superpowers/plans/2026-11-05-a-ruleset-names-its-properties.md`
carried its own, for the writer to close before sign-off. The writer closed
G7, G8 and G9 in the ticket before sign-off: it names VTT-449, the Story
alone, and `buildFeed` in `feed.ts`.

## The gaps this plan inherits

G1 to G5 were found in the first verification and are closed in the ticket's
revision. They describe the ticket as it was first verified.

- **G1, closed.** Item 3 then said VTT-469's and VTT-471's tests pass; some
  correct, or hold whole, a companion never seen. Item 4 now says their
  fixtures move so the viewer sees it first.
- **G2, closed.** VTT-471 becomes false as worded. Withdrawn and succeeded by
  R4.
- **G3, closed.** SPEC-021's pause-and-resume comparison moves (D4).
- **G4, closed.** Conditions are among what an unseen companion is held
  without (D1).
- **G5, closed.** The first sight's withdrawal showed as "<A> left" and "<A>
  is in the party". The ticket's item 3 and rule now forbid it (D11).
- **G6. The ticket's own gap on the name.** Kept, as the ticket reads it;
  VTT-361 (every log introduces every party member) and VTT-475 (the Story
  names a party member by its actor name) rely on it. Logs already written:
  D8.
- **G7. VTT-449 becomes false as worded.** It says the Story shows every
  entry. Withdrawn and succeeded by R6 at Task 0. Closed: the ticket names
  it.
- **G8. "Its Story and Events" is the Story alone.** Only a DM's and an
  agent's window draws the Events panel (VTT-445, VTT-446), and they hold the
  world log (VTT-176), where a removal carries an event id (P12). No window on
  a character log draws the panel, so `renderTicker` stays as it is, and
  VTT-447's "one row per entry" stays true (D13).
- **G9. The file.** Closed: the ticket's "What it touches" item 4 now names
  `buildFeed` in `feed.ts`. `describe` keeps every payload's words
  (VTT-476); the change is in `buildFeed`, in `feed.ts`, which
  `renderSpectator` hands to `renderFeed`.

## Measurements this plan stands on

All on 2026-11-06 at `84c4ac8`, by command.

| # | Probe | Result |
|---|---|---|
| P1 | `test -f` on every path; `grep` for every symbol and row | all resolve |
| P2 | `go test ./internal/perceive -run 'TestQACSAPartyMemberIsIntroducedWhole\|TestQACSATokenArrivesWithItsNameAndNotesAndWithoutItsGMTexts'` on the worktree | they pass: the friend, whose token is never placed on the hero's board, is introduced whole |
| P3 | `go test ./internal/campaign -run 'TestQALogALateLogOpensWithThePresent\|TestQALogALogOpensForAPartyMemberThatHadNone'` on the worktree | they pass, and each asserts that p's opened log holds q's pool though p never sees q |
| P4 | `grep` for callers of `shownAs(`, `shownWith(`, `.introduce(`, `.correct(` and `statusFrames(` outside tests | only `projection.go`. Outside the package, `fanout.go` calls `New`, `Resume`, `Event` and, through `opening`, `Perch`; `internal/gateway` calls `CanSee` and `SeesActor`, which read `look` alone |
| P5 | the Go probe: D1 and D3 to D5 applied to a copy; `go test ./internal/perceive ./internal/campaign ./internal/gateway ./cmd/vtt` | `internal/gateway` and `cmd/vtt` pass (`cmd/vtt` run on the first variant only). Failing: in `internal/perceive`, `TestQACSAPartyMemberIsIntroducedWhole`, `TestQACSAKindChangeOutOfSightWaitsUntilTheActorComesIntoSight`, `TestQACSAPartyMemberAddedAgainIsIntroducedAfreshOrCorrected`, `TestAResourceStartedBelowZeroIsCorrectedOnSight`, `TestADifferenceOfExactlyMaxInt32IsOneCorrection`, `TestARemovedActorWhoseIdNeverReturnsStaysInTheFold`, `TestAnActorAddedAgainWithAnotherAttributeIsIntroducedAfresh`; in `internal/campaign`, `TestQALogALateLogOpensWithThePresent`, `TestQALogALogOpensForAPartyMemberThatHadNone`. The same with and without D1's conditions, and with and without an `IsPartyMember` guard in D4 |
| P6 | in the Go probe: a companion with attributes and module data alone, one with resources, one with neither, each behind a shut door, given a condition and a controller, then the door opened | before the door: the companion's unseen `ActorAdded` and nothing else of it. At the door: for the companions with something to strip, a bare `ActorRemoved`, the `ActorAdded` whole, the grant, the condition, the `TokenPlaced`, then the forwarded `DoorOpened`; for the one with neither, the condition and the grant as a correction, no withdrawal. A log opened for an actor already a party member before its event is introduced its own character unseen, and its first event withdraws it and introduces it whole |
| P7 | the client: `describe`'s `actorAdded` and `actorRemoved` arms, `buildFeed`, `fold.ts`'s `actorAdded`, `actorRemoved` and `resourceChanged` arms, `grid.ts`'s chips, `player.ts`'s `affordable` | the Story shows every entry; the fold accepts an actor with no resources or attributes, a removal with no token standing, and refuses a change to a resource the actor lacks; chips are drawn for tokens on the board; the panel acts for the watched actor, whose own log holds it whole |
| P8 | `tools/mutation-equivalents.txt` | the `internal/perceive` entries sit on `objectInSight`, `sortedSceneIDsUnion` and `recall`'s scene loop, none on code this plan changes |
| P9 | `grep -rn -i` over `docs/specifications` for "seen or not", "whole", "sheet", "each entry" and the rows' ids | SPEC-016; SPEC-021's pause-and-resume paragraph; SPEC-022's Story paragraph; SPEC-024's served properties, which are declarations |
| P10 | `engine.Apply`'s `ActorAdded` and `ResourceChanged` arms; `fold.ts`'s | only `ActorAdded` writes attributes and module data; each fold refuses an `ActorAdded` of an id it holds and a change to a resource the actor lacks |
| P11 | the client probe: a copy of the tracked `client` and `contract` trees; `env -u CLAUDECODE bun test client/test` before and after D11 sketched in `buildFeed` | the same tests fail before and after, each a test of the shipped campaign's art, which the copy lacks; no other test moves |
| P12 | the `Envelope` message in `contract/vtt/v1/events.proto`; SPEC-021's entry numbering; `app.ts`'s `ticker: isDM` | an entry carries its own log sequence and no world sequence; a projection's entry carries no event id; the Events panel is drawn for `dm` and `agent` alone |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone, once, last.
  - 3: the contract does not change.
  - 4: one fold. `engine.Apply` and `client/src/fold.ts` do not change.
  - 5: no game-system vocabulary in Go, tests included; the fixtures' keys
    come from those already in `internal/perceive`'s tests.
  - 8: cite names; never `file:line`, never `.superpowers/`.
  - 9: answered below.
  - 10: a comment is a warning, a pointer or an exported symbol's doc. The
    comments this change makes false are rewritten to that shape: `shownAs`'s
    doc, `introduce`'s doc ("its conditions behind it"), the field comment on
    `reshaped`, which gains R3, and `feed.ts`'s header ("Everything in the log
    is shown"), which points at R6. `look`'s "Know every party member, seen or
    not" stays true.
- **No file outside these changes:** `git diff main -- contract internal/engine
  client/src/fold.ts client/src/view/spectator.ts` prints nothing, and
  `git diff main -- internal/campaign/fanout.go` changes only the comment on
  `opening`, whose "(VTT-317)" becomes R7's id (Task 1).
- **`internal/perceive` imports** only its own package, `contract`, `engine`
  and `sight`, tests included (`TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight`).
- **The mutation gate runs over `internal/perceive`** in `task check`. Each
  new condition has a test that goes red when it is negated (K1 to K4). No
  `switch` is added.
- **The coverage floor** for `internal/perceive` in
  `tools/coverage-thresholds.txt`, and the client's for `feed.ts`, hold;
  `projection.go`'s and `feed.ts`'s comment shares stay at or under their
  ceilings; a new test file is held to the default ceiling.
- **The client:** `task client:typecheck`, the client's lint and `bun test`
  (read with `env -u CLAUDECODE`) pass. `task build:client` rebuilds
  `cmd/vtt/webdist`, committed with the change; `check:drift` passes only
  after that commit. A client test reads no file outside `client/` but one
  `tools/ts-mutation-inputs.py` hashes; Task 2's build theirs inline.
- **`check:new-prose`**: `python3 tools/check-new-prose.py main`,
  `python3 tools/check-requirements-chain.py .` and
  `python3 tools/check-comments.py main` (after staging) before every commit.
- **A new function or test goes after a closing brace** (`check:doc-owner`);
  a test the register cites has a literal name and its rule's id line.
- Stage and commit in separate calls, and verify what landed. Never
  `git checkout --` to undo a break: the inverse edit, checked with grep,
  after asserting the broken text is not already there. The review gate
  refuses untracked files. The pre-push hook takes minutes: its longest
  timeout, its output to a file, the machine's load checked first.

## Rule 9: how MapTool does this

MapTool sends every client the whole campaign, every token's properties
included, and its `TokenProperty`'s `ownerOnly` and `gmOnly` filter what a
stat sheet shows in the client (RPTool `63862e4`). That is the per-property
design the owner rejected, and a filter over data already sent; MapTool has
no answer to whether a character has seen another. Nothing is borrowed.

## Decisions

1. **D1. What a log may hold of a companion its character has not seen.**
   `unseenAs` gives its id, name, module id and kind, and its controllers,
   sent as grants behind the `ActorAdded`; no attributes, resources or module
   data; and no `ConditionApplied` behind it.
   - Forced by the ticket's rule ("alone"), item 1 and the owner's ruling.
   - The name, by the ticket's reading (G6).
   - A creature is introduced only in sight, so `unseenAs` applies to party
     members alone.
2. **D2. What tells "seen" from "never seen" is the log's own fold.** A held
   actor that agrees with the world's in every field only `ActorAdded` writes
   (`sameIncarnation`) has been seen, or sits in a log written before this
   change (D8); one that agrees with `unseenAs` of the world's has not. Where
   the forms are equal, the companion has nothing to strip and the difference
   does not matter.
   - Not `sighted`: it holds the last look's `sees` alone, so it cannot say
     "ever".
   - No new memory: forced by VTT-312. The projection keeps nothing but
     `sighted` and is rebuilt from the fold and the world after a dropped frame
     set, a failed write and a reopen (SPEC-021); a log written so far is the
     only record of what its character has seen.
3. **D3. The first sight is a withdrawal and a fresh introduction, through
   `reshaped`.** `recall` already marks reshaped a held actor that differs from
   `shownAs` of the world's. A companion held in the unseen form differs
   whenever it has something to strip, and `transitions` withdraws a reshaped
   actor and introduces it again only in sight. The frames at that event, in
   order, each with its sequence: `hideTokensOf`'s, none, since a companion
   out of sight has no token on the board; the bare `ActorRemoved`; the
   `ActorAdded` of `shownAs` with its controllers cleared; a grant per
   controller; a `ConditionApplied` per condition; then the `TokenPlaced` of
   its tokens in sight. `engine.Apply` and `fold.ts` accept them, as they do
   for a kind change (VTT-468). A companion with nothing to strip is not
   reshaped, and `correct` sends its conditions and controllers (P6).
   - Forced by the contract (P10): only `ActorAdded` carries attributes and
     module data, `engine.Apply` and `fold.ts` refuse an `ActorAdded` of an id
     they hold, and the owner ruled out a new mechanism.
4. **D4. `recall`'s "gone" test accepts the unseen form too.** A held actor
   is another only when it agrees with neither `shownWith` of the world's
   under the held kind nor `unseenAs` of it. Without this, every companion
   held unseen would be withdrawn and introduced again at every event (K3,
   R5). No `IsPartyMember` guard on the new half: a creature is held without
   its name, so `unseenAs` can agree with it only where the creature's form
   already does, and the guard would be an equivalent mutant (P5).
5. **D5. `introduce` is told whether the look sees the actor.** `transitions`
   passes `now.sees[id]`, and `correct`'s re-introduction passes true, since
   `correct` runs only in sight. A parameter rather than a second function: a
   preference.
6. **D6. Unchanged:** `look`, whose `actors` and `party` are the roster and
   whose `sees` is the sight; `Resume`, `correct` and `statusFrames`, each of
   which compares only actors in sight, where `shownAs` is the only form a log
   may hold; `classify`, whose `ResourceChanged` arm already requires `saw`;
   `fanout.go`; `fold.ts`; `describe`; the contract. `Resume` refuses a fold
   that holds an actor in sight in the unseen form, as it refuses any lagging
   fold, and the next event re-introduces it (T3).
7. **D7. A log opened for an actor already a party member before its event.**
   The opening's `Perch` passes no eyes, so it introduces the log's own
   character unseen, and the first event's frames withdraw it and introduce it
   whole (P6). This happens only for a campaign written before character logs
   existed, or after an opening's frames did not fold (SPEC-021). No code and
   no test, by the owner's ruling of 2026-10-21; SPEC-016 says so (Task 5).
   In every other opening the actor is not yet a party member in `before`,
   and the first event introduces it whole as its own eye. R1 and R2 speak of
   the eyes a decision is made through, and the opening's are none. D11 keeps
   the pair out of its Story.
8. **D8. A log already written** keeps each companion as it holds it
   (principle 18). One held whole that its character never saw agrees with the
   world's whole form, so it is not withdrawn. Nothing is migrated.
9. **D9. Records.** VTT-485 is withdrawn, succeeded by R1 and R2; VTT-471 by
   R4; VTT-449 by R6. SPEC-016, SPEC-021 and SPEC-022 move at Task 5.
10. **D10. The client folds as it does.** `fold.ts` folds every frame of D3,
    the panel acts for the watched actor, and chips are drawn for tokens on
    the board (P7). Only the Story changes (D11).
11. **D11. The Story leaves a re-introduction out.** The client cannot read
    "the same event" from a sequence: every entry of a character log carries
    the log's own sequence and no world sequence (P12). What it has is the
    order, and that a projection's entry carries no event id. VTT-252 holds
    a bare `ActorRemoved` directly before the introduction of the same id, and
    no character log is written a forwarded `ActorRemoved` (SPEC-016). So
    `buildFeed` leaves out, before it groups, each run of the log that is:
    - an `actorRemoved` with no event id;
    - the `actorAdded` of the same actor directly after it;
    - the `actorControlGranted` and `conditionApplied` entries of that actor
      with no event id directly behind, which restate what the introduction
      carries.

    So at a first sight none of "A left", "A is in the party", "control of A
    given" or "A gained …" is drawn for the pair. What is drawn is what a
    companion already held is drawn when it comes into view: its token's
    arrival, "A is at x,y", and the event's own lines, such as "a door opened
    at x,y". A condition the companion already had shows on its disc, as a
    held companion's does. A window that holds the world log is unchanged: a
    world removal carries its event id. The fold, `describe` and `namer` are
    handed the whole log as today.
12. **D12. Every re-introduction gets D11, the kind change's included.**
    `buildFeed` cannot tell a first sight from a kind change (VTT-468,
    VTT-481), a correction that re-introduces, or an id the world adds again
    as another (VTT-470): each is the same run. And none of them is a
    departure: "X left" and a second "X is in the party" are false for each.
    The kind change's forwarded grant carries its event id, so "control of X
    given" is still drawn for it. These stay as today, near-theoretical
    or cosmetic by the owner's 2026-10-21 ruling: a kind change in sight still
    reads "X went out of sight" and "X is at x,y" for its token's hiding and
    placing again; and a party member the world adds again under a used id
    out of sight arrives in the Story unannounced, since `add_actor`'s
    description forbids reusing an id (VTT-390).
13. **D13. The Events panel is not changed.** No window on a character log
    draws it (G8), so applying D11 to `renderTicker` would change nothing a
    player sees and would make VTT-447 false as worded.

## What the change does at each edge

- **A companion whose controller changes.** While unseen, the grant or
  revocation is withheld (VTT-245) and the roster's controllers go stale; the
  first sight's introduction carries the present ones. A grant that changes
  its kind waits for sight (VTT-481) and arrives in the form for that kind. A
  companion seen before is corrected on sight (VTT-469). A player given
  control of a companion watches its own log, where it is whole.
- **A perch.** No projection runs: the seat is sent the other log from its
  first entry (SPEC-015), which holds each companion as that log's character
  saw it; the window's Story is built afresh from that log, D11 included.
- **The log's own character** is always among the look's eyes and `sees`, so
  it is introduced whole; D7 is the exception, gone by its first event.
- **A log resumed after a reopen.** `Resume` starts from the log's fold. A
  companion held unseen and still unseen agrees with `unseenAs`, so nothing
  is sent; the next event sends what the fed decision sends (VTT-312, T3, and
  the reopen half of `TestQALogALogOpensForAPartyMemberThatHadNone`, VTT-324,
  which passes under the probe).
- **The opening of a new log.** `opening`'s `Perch` through no eyes sees
  nothing, so every party member then in the world is introduced unseen, and
  the Story says "A is in the party" for each, as today; the log's own
  character comes with the first event (D7).
- **A pause and a resume.** A companion held whole stays whole; one added
  during the pause is introduced unseen at the resume's event unless it is in
  sight.

## Q1, resolved

Q1 asked whether the Story could say "A left" and "A is in the party" at a
first sight. The ticket's item 3 and its rule say it may not; D11 is how, D12
says what else it covers, and Task 2 holds it.

## Requirements

Allocated at Task 0 with `requirement-id`, after the `requirements` sort and
its reading review, each `**OPEN — no test yet**` until its test lands. The
wording below is a draft for the sort; the register's is the one that counts.

| R | Ticket | Draft | Red under |
|---|---|---|---|
| R1 | rule 4: VTT-485's half in sight | A character log is introduced to a party member its eyes see with its name, attributes, resources and module data. | K6 |
| R2 | rule 1, items 1 and 5; rule 4: VTT-485's half out of sight | A character log is introduced to a party member its eyes do not see with its id, name, module id and kind, and its controllers behind it, and with no attributes, resources, module data or conditions. | K1, K2, K5 |
| R3 | rule 2, items 2 and 5 | A party member a character log holds without its sheet is brought, at the event at which the log's eyes first see it, to its present name, attributes, resources, module data, conditions, controllers and kind. | K4, K7 |
| R4 | rule 4: succeeding VTT-471 | (as the register's VTT-660 words it) | T1's checks after the introduction: an unseen grant, revocation, condition or resource change sends nothing; red under a `classify` arm that forwards without `saw`, and under K3 |
| R5 | D4 | A character log is sent a bare `ActorRemoved` of a party member it holds without its sheet only at the event at which its eyes first see it, or when the world adds its id again with another name or module id. | K3 |
| R6 | rule 3, item 3; succeeding VTT-449 | The Story shows every entry of the log its window holds, whatever the participant's role, except while the window shows the reason it cannot fold its log, and except each `actorRemoved` with no event id, the `actorAdded` of the same actor directly after it, and that actor's `actorControlGranted` and `conditionApplied` entries with no event id directly behind it. | K8 to K11 |
| R7 | succeeding VTT-317, found at Task 0's review | A character log opens with the open session, the public notes and each party member of the world before its event, each introduced as R2 introduces a party member its eyes do not see | K1, K5 |

- **Withdrawn:** VTT-485, succeeded by R1 and R2; VTT-471, succeeded by R4;
  VTT-449, false as worded (G7), succeeded by R6; VTT-317, found at Task 0's
  review, succeeded by R7.
- **R7** (VTT-663) succeeds VTT-317: an opening introduces each party member
  of the world before its event as R2 does. VTT-317's tests become R7's: the
  QA opening tests that the plan changes assert the opening's party as R2
  gives it, and q's pool on q's own log.
- **R3, R5 and R6's drafts above** are superseded by the register's VTT-659,
  VTT-661 and VTT-662, which the sort and Task 0's review reworded; the
  register's wording counts.
- **D7 and VTT-365.** At D7's opening the log's own character is introduced
  as R2 gives it, while VTT-365 says it counts as seen in that log. The
  event's frames then bring it whole. The owner's ruling of 2026-10-21
  covers that edge: no code, no row.
- **VTT-471's successor, R4,** is cited by T1, which holds its never-seen
  half, and by the tests that cite VTT-471 today, which hold its seen half:
  `TestDamageTakenOutOfSightArrivesWhenTheEyeWalksIn`,
  `TestAKindChangedOutOfSightIsBroughtToItsPresentOnSight`,
  `TestQACSAKindChangeOutOfSightWaitsUntilTheActorComesIntoSight`,
  `TestQACSAnUnseenCreatureIsHeldAsLastSeenAndBroughtToItsPresentOnSight` and
  `TestQACSAnUnseenPartyMemberIsHeldAtItsResourcesThenAndBroughtToThemOnSight`,
  and the `VTT-208 VTT-471` label in `qa_testimony_eyes_test.go`.
- **VTT-449's successor, R6,** is cited by VTT-449's tests, which pass under
  P11, and by T5.
- **Kept, wording still true** (read against D1 to D13): VTT-176, VTT-209,
  VTT-210, VTT-243, VTT-245, VTT-252, VTT-312, VTT-324, VTT-365 (it speaks of
  a log's event frames; an opening is decided through no eyes, as R7 says), VTT-361, VTT-445 to VTT-448, VTT-465, VTT-466,
  VTT-468, VTT-469, VTT-470, VTT-475, VTT-476, VTT-481, VTT-486. The search:
  the probes' failing tests (P5, P11), each traced to the rows it cites, and
  `grep -n -i -E` for "party member", "roster", "introduc", "whole", "Story"
  and "Events panel" over the register's rows.
- **Refused before the sort:** the ticket's items 4 and 6, which are checks
  this work keeps green, not rules it adds.
- **The sort, at Task 0, and its review:**
  - R1 is scoped to an event's frames, and R2 names the opening.
  - R3 and R5 speak of an actor held as a party member without the
    attributes, resources or module data the world's actor has. That keeps
    R5 from contradicting VTT-468 when a held creature becomes a party
    member.
  - R4 is rewritten as one rule, what a fold holds of an actor it does not
    see: as last seen, or as introduced when never seen, with a party
    member's resources when the viewer saw it.
  - R6 names the pair it leaves out, so that a bare removal followed by
    another actor's introduction is shown.
  - R7 is added for VTT-317.
  - Refused: D7's and D12's edges, which get no row by the 2026-10-21
    ruling.

| Ticket item | Observed by |
|---|---|
| 1 | T1 (R2, R4, R5) |
| 2 | T2 (R1, R3) |
| 3 | T5 (R6) |
| 4 | the kept rows' tests, unedited, and the tests below, their assertions kept |
| 5 | T4 (R2, R3 at the wire) |
| 6 | Task 3 |

## The existing tests that change

From the Go probe (P5). No client test changes (P11).
- `internal/perceive/qa_creature_shown_test.go`:
  - `TestQACSAPartyMemberIsIntroducedWhole`: the hero, its log's eye, stays
    whole (R1); the friend, never seen in `qacsTable`, is expected as
    `unseenAs` gives it (R2). Its id line becomes R1 and R2.
  - `TestQACSAKindChangeOutOfSightWaitsUntilTheActorComesIntoSight`: the
    friend steps into the hero's sight and back before its kind change, so
    the assertion that it is held as last seen stands (item 4). Its id line
    becomes R4.
  - `TestQACSAPartyMemberAddedAgainIsIntroducedAfreshOrCorrected`: "Bram the
    Second", added unseen, is withdrawn and introduced unseen at its add, and
    withdrawn and introduced whole at its placement in sight (R3); the check
    is made per event.
- `internal/perceive/status_test.go`:
  - `TestAResourceStartedBelowZeroIsCorrectedOnSight` and
    `TestADifferenceOfExactlyMaxInt32IsOneCorrection`: the wraith's token is
    placed in the hero's room first and moved behind the door before the
    resource changes, so it is corrected, not introduced, on sight (item 4).
  - `TestARemovedActorWhoseIdNeverReturnsStaysInTheFold`: the rogue's first
    sight sends a bare `ActorRemoved` directly before its introduction, which
    VTT-252 allows; the check becomes VTT-252's own: every bare
    `ActorRemoved` of the rogue is directly followed by its `ActorAdded`, and
    none follows its removal.
- `internal/perceive/unseen_test.go`'s
  `TestAnActorAddedAgainWithAnotherAttributeIsIntroducedAfresh`: the new
  scout, added unseen, is withdrawn and introduced unseen, then introduced
  whole when its token comes into sight; the check becomes that the first
  frame naming it after its add is a bare `ActorRemoved` with an introduction
  behind it, and the fold ends holding the new scout's weight.
- `internal/campaign/qa_character_log_test.go`'s
  `TestQALogALateLogOpensWithThePresent` and
  `TestQALogALogOpensForAPartyMemberThatHadNone` (VTT-317, now R7): q is
  held in p's opening as `unseenAs` gives it, which is R7's evidence; the
  assertion on q's pool is made on q's own log, in the event's frames, which
  is R1's (VTT-657).
- Id lines only: `// VTT-485` on `roster_test.go`'s
  `TestACreatureIsIntroducedWithItsIdModuleKindAndConditionsAlone` becomes R1;
  every other `// VTT-471` and `VTT-471:` label becomes R4; every
  `// VTT-449` in `client/test` becomes R6 (Task 2); every `VTT-317` in
  `internal/campaign/character_log_test.go`, `qa_character_log_test.go`,
  `qa_creature_shown_test.go`, `qa_pause_resume_test.go`,
  `internal/gateway/creature_test.go` and `qa_creature_test.go` becomes R7
  (Task 1). After Task 5, `git grep -n 'VTT-\(317\|449\|471\|485\)\b' -- internal
  cmd client/src client/test docs/specifications` prints nothing.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** sign-off; the writer closes G7 to G9 in the ticket if they choose;
  run the `requirements` sort over R1 to R6 and report what it refuses;
  allocate the ids; withdraw VTT-485, VTT-471 and VTT-449 with their
  successors named; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the chain passes.

### Task 1: the projection, and its tests at every layer (R1 to R5; D1 to D6)

- **Files:** `internal/perceive/projection.go`; new
  `internal/perceive/companion_sheet_test.go` and
  `internal/gateway/companion_sheet_test.go`; the Go tests of "The existing
  tests that change".
- **The code,** as the probe ran it:
  - `unseenAs(a)`: `shownWith(a, false)` with `Name` set from `a`; its doc
    names R2.
  - `recall`: an actor is gone when
    `!sameIncarnation(a, shownWith(w, engine.IsPartyMember(a))) && !sameIncarnation(a, unseenAs(w))`;
    the reshaped test stays `!sameIncarnation(a, shownAs(w))`.
  - `introduce(id, seq, a, seen, st)`: `unseenAs(a)` when not `seen`, and the
    conditions loop only when `seen`.
- **New tests,** each with a literal name and its id line:
  - **T1** `TestACompanionItsCharacterHasNotSeenArrivesWithoutItsSheet` (R2,
    R4, R5): a companion with attributes, resources, module data, a condition
    and a controller, behind a shut door, already in the world when the
    viewer's first event comes, as at an opening. Its introduction equals
    `{id, name, module id, kind}`, its grant follows, no `ConditionApplied`;
    then its resource change, a condition, a grant and a revocation, each
    unseen, send nothing of it and no `ActorRemoved`; the fold holds it with no
    attributes, resources, module data or conditions. Red today: the
    introduction carries the sheet.
  - **T2** `TestACompanionsSheetArrivesWhenItsCharacterFirstSeesIt` (R3, R1):
    a case per shape of P6: attributes and module data with no resource;
    resources; neither. At the door's event, those with something to strip
    are sent the frames of D3 in order, with the event's sequence and no event
    id, and the one with neither a correction; each fold then holds the
    companion as the world does. After that, the companion leaves sight,
    changes, and returns: a correction and no `ActorRemoved`. Red today: no
    withdrawal at sight, since the sheet arrived at the introduction. D3's
    order is what D11 reads, so this test holds the client's premise too.
  - **T3** `TestAResumedLogHoldingAnUnseenCompanionInSightIsBroughtToItsSheet`
    (R3, VTT-325): a fold that holds the companion unseen, as after a dropped
    frame set, and a world where it is in sight; `Resume` from them, then the
    companion's `ResourceChanged`: the decision sends the re-introduction with
    the present value and does not forward the change, and every frame folds.
    Green today; K7 is its red.
  - **T4** `TestASeatIsSentNoCompanionsSheetBeforeItsCharacterSeesIt` (R2,
    R3), with `qacsWireTable`: a companion added before Ayla, so introduced by
    her log's opening, and one after, by an event, each with secrets in its
    attribute keys, resource names and module data; a player's seat and a
    spectator's on Ayla. No raw frame carries a secret before the companions
    come into sight; then each is sent its `ActorAdded` whole. Red today.
- **Order:** T1, T2 and T4, red; the code; the changed tests; T3; the breaks.
- **True when:** `go test ./internal/perceive/ ./internal/campaign/
  ./internal/gateway/ ./cmd/vtt/` passes, every Go test outside "The existing
  tests that change" unedited; `go vet`, `semgrep` and `go-arch-lint` are
  clean.
- **Breaks,** each undone by its inverse edit:
  - **K1:** `introduce` ignores `seen`. T1, T4,
    `TestQACSAPartyMemberIsIntroducedWhole`'s friend and the changed campaign
    tests red.
  - **K2:** the conditions loop ignores `seen`. T1's condition assertion red.
  - **K3:** `recall`'s gone test drops its `unseenAs` half. T1's
    no-withdrawal assertion red.
  - **K4:** `recall` leaves `reshaped` unmarked for a held actor that agrees
    with `unseenAs` of the world's. T2's case with no resource red: its sheet
    never arrives. The resourced case stays green through `correct`'s
    fallback, which is why the case exists.
  - **K5:** `unseenAs` leaves out the name. T1's name assertion red.
  - **K6:** `introduce` always gives `unseenAs`. R1's tests red:
    `TestQACSAPartyMemberIsIntroducedWhole`'s hero, T2's whole introductions.
  - **K7:** `Resume` also accepts an actor in sight held as `unseenAs` gives
    it. T3 red: the forwarded change does not fold.
- **Then:** Phase 4a, Phase 4b, the fixes, a commit.

### Task 2: the window (R6; D11 to D13; item 3)

- **Files:** `client/src/view/feed.ts` (`buildFeed`, and a function it calls
  that names the entries D11 leaves out); `client/test/feed.test.ts`;
  `client/test/spectator-view.test.ts`; `cmd/vtt/webdist`, rebuilt by
  `task build:client`.
- **New tests,** each with a literal name and R6's id:
  - **T5** in `spectator-view.test.ts`, "at a companion's first sight the
    Story shows it coming into view and neither its leaving nor its joining":
    a log shaped as P6 shows a character log: the opening's `actorAdded` of
    Bram, a party member, and his grant; later, with no event id, Bram's
    `actorRemoved`, his `actorAdded` whole, his grant, his condition and his
    `tokenPlaced`, then a `doorOpened` with an event id. `renderSpectator`'s
    Story holds "Bram is in the party" from the opening, "Bram is at x,y" and
    "a door opened at x,y", and holds neither "Bram left", nor a line of the
    pair, nor its grant's or condition's line. Red today.
  - **T6** in `feed.test.ts`, "a removal that carries an event id, or that
    another actor's introduction follows, is told": a world log's
    `actorRemoved` with an event id followed by an `actorAdded` of the same
    id; a bare `actorRemoved` followed by another actor's `actorAdded`. Each
    entry is in `buildFeed`'s entries. Green today; K9 and K11 are its red.
- **True when:** `bun test`, `task client:typecheck` and the client's lint
  pass, every existing client test unedited but its id lines; `task
  build:client` has rebuilt `cmd/vtt/webdist`, committed with the change;
  `git diff main -- client/src` names `feed.ts` alone.
- **Breaks,** each undone by its inverse edit:
  - **K8:** `buildFeed` leaves nothing out. T5 red: "Bram left" is drawn.
  - **K9:** the run is matched whatever its event id. T6's world case red.
  - **K10:** the run ends at the `actorAdded`. T5 red: the grant's and the
    condition's lines are drawn.
  - **K11:** the `actorAdded` is matched whatever its actor. T6's other-actor
    case red.
- **Then:** Phase 4a, Phase 4b, the fixes, a commit.

### Task 3: the gate

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once if
  a ceiling can come down; check free space and the load; run `task check`
  whole on the last code commit, detached, all output to a file; require the
  gate's own completion line and `check:ts-mutation`'s. The client changed,
  so the stored mutation report is out of date: tell the owner which commit it
  measured and the tickets since; the full run is the owner's to ask for.
- **True when:** the gate is green (item 6).

### Task 4: the report

- **Do:** write `docs/reports/2026-11-06-a-companions-sheet-is-seen-with-the-companion.md`
  with the `implementation-report` skill and give it its reading review.
- **It covers:** each "Done looks like" item with its observation; K1 to K11
  with what went red; Rule 9; G1 to G9; D7 and D12; Phase 4a's
  adjudications under their own heading; the gate.

### Task 5: the specifications

With the `specification` skill, from the report:
- **SPEC-016:**
  - Status: `unseenAs` and the new test files.
  - Principles served, 16: party members seen or not, each one's sheet only
    once seen.
  - "What a look sees": a party member out of sight is introduced without its
    sheet.
  - "What a log may hold of an actor": `shownAs` in sight, `unseenAs` out of
    it, by the owner's ruling of 2026-11-06, VTT-485 replaced by R1 and R2.
  - "What the projection remembers": `recall`'s gone test with D4's second
    form; a companion held unseen is reshaped until its first sight (R3).
  - "What `transitions` sends": `introduce` by `unseenAs` out of sight, its
    conditions only in sight.
  - "A perch": the opening introduces every party member unseen, and D7.
  - Consequences: the roster consequence ("with the status it had when the
    viewer last saw it or at its introduction"); "A new party member is
    introduced at once, seen or not"; the fields compared for an actor added
    again, with a companion held unseen compared by its name and module id;
    the bare `ActorRemoved` before a reshaped actor's re-introduction now
    including the first sight.
  - The sentence that a party member made a creature out of its sight "is
    held whole until it is seen again": a never-seen one is held without
    its sheet.
  - The sentence that "an actor that becomes a party member is introduced,
    with its present status and controllers, to every viewer that does not
    hold it": out of sight it is introduced without conditions or values.
  - Requirements: R1 to R5 and R7 in; VTT-485, VTT-471 and VTT-317 out.
- **SPEC-021:** its pause-and-resume sentence on what a held actor is
  compared with; its opening's party, as R7 says; and its citations of
  VTT-317, in its prose and its Requirements line, which become R7's id.
- **SPEC-022:** the Story's "each entry of the log the window holds" with
  D11's exception and why it is safe (VTT-252, no forwarded `ActorRemoved` in
  a character log); the Events panel unchanged (D13); the consequence "the
  window's Story shows each of them", re-run; VTT-449 out, R6 in.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer, and every id resolves.

Commit the report, then the specifications. Push and merge on the owner's
word.

## For Phase 4a

Hand the QA agent the rows of R1 to R6, VTT-469, VTT-470 and VTT-252, the
owner's ruling, and the surfaces as signatures, never the source:
`go doc ./internal/perceive` (`Eyes`, `Projection`, `New`, `Resume`,
`Event`, `Perch`, `SeesActor`); `go doc ./internal/campaign`
(`CharacterLog`); the gateway's `/ws?log=` seat; `buildFeed`, `FeedEntry`
and `renderSpectator`'s signature. Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- its files are new `qa_` files beside Task 1's and Task 2's, held to the
  default comment ceiling; `internal/perceive`'s may import only `contract`,
  `engine` and `sight`; `semgrep` bans game words in tests too; a client test
  reads no file outside `client/` but a hashed one;
- known and not findings: D7; D8; D12's token lines at a kind change in sight
  and its unannounced reused id; a companion with nothing to strip corrected
  rather than re-introduced; the name kept; the Events panel unchanged;
- give each test a literal name and its rule's id;
- the tree is hashed before and after it.

## For Phase 4b

Hand the reviewer the diff with `git diff HEAD` and tell it to check:
- that each changed test asserts as much of its own rule as before, its
  assertion moved, not dropped;
- each "only", "never" and "every" against the search that bounds it, R5's
  and R6's above all;
- that each comment under `internal/` and `client/src` is a warning or a
  pointer (rule 10), those named under the constraints first;
- that `cmd/vtt/webdist` was rebuilt from the committed `client/src`, and
  that the diff of the constraints prints nothing.
