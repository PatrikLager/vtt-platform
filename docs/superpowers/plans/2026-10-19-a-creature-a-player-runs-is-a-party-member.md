# Plan: a creature a player runs is granted as a party member, and the agent is told so

- **Ticket:**
  `docs/superpowers/specs/2026-10-19-a-creature-a-player-runs-is-a-party-member-design.md`.
- **Verdict:** passes with gaps (G1-G7, below). Verified 2026-10-19 by an
  agent that did not write the ticket, read-only, against HEAD `abf4eb5`. Its
  probes ran through `go test -overlay` and `go run -overlay` and wrote only
  to the scratchpad.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-19-a-creature-a-player-runs-is-a-party-member.md`.
- **Base:** main `abf4eb5`.
- **Branch:** `fix/a-creature-a-player-runs-is-a-party-member`.
- **Signed off:** 2026-10-19, by the owner ("Ja, kör"). The question rule
  holds ("Ja, samma regel").

## The gaps this plan inherits

The ticket's writer revised the ticket for G1 and G3-G7 and cited G2's
history before this plan was committed; the gaps are kept here as the
verifier found them.

- **G1.** `ActorKind`'s comment also names "a neutral, a familiar, a summon"
  as a foreseeable third kind. The owner ruled out a new kind, and item 4 as
  worded would leave the sentence in place. Task 2 rewrites it (D6) and greps
  for it.
- **G2.** The owner already ruled the charmed-into-a-player's-hands case on
  2026-08-23: the visibility ticket's §5.1, revised, and
  `docs/reports/2026-08-18-visibility.md`. "a charmed monster is still a
  monster" and "a familiar, a summon" date from `80dfa0e`, the first draft
  that revision overturned the same day. "for someone to operate" is
  `42f7af9`'s own, written beside its "grant again with the other value when
  a monster is charmed into the party's service" (corrected by the reading
  review before Task 0's commit). Task 5 writes the escape's recipe (D12).
- **G3.** The ticket does not name three things this work moves:
  - `contract/gen/tools/tools.json`, the third copy;
  - `TestRunOutputMatchesCommittedToolsJSON`, red until regeneration;
  - two sentences in `tools/toolgen/main_test.go` that become false. Both are
    in `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`: its doc
    says actorId "must state the fold's bound rather than steer", and a
    comment in its body says "a charmed monster is still a monster".

  Task 1 covers all three.
- **G4.** Two of the ticket's open questions are answered:
  - `expected_tools.json` is edited by hand (D5).
  - Generated TS comments do not reach `webdist`.
- **G5.** The continuation of a log under a reused id is VTT-318's. VTT-313
  covers how a viewer is introduced to the actor. Cite VTT-318 for it.
- **G6.** The rule 9 answer gains `Token.Type` (below).
- **G7.** What kind `add_actor` states for a creature a player is to run: the
  grant decides it, whatever the actor was added as (D2).

## Constraints that bind every task

Named here; each is stated where it lives:

- CLAUDE.md rules:
  - 2: `task check` is never weakened.
  - 3: no field, enum value or message changes; a comment is not a contract
    change.
  - 4: nothing here applies an event.
  - 5: `.semgrep/vocabulary.yml` scans Go under `internal/` and `cmd/`, and
    no added word is in its list.
  - 8: cite names, never `file:line`.
  - 9: below.
  - 10: the new gateway test carries citation lines only.
- The owner's rulings of 2026-10-19, as the ticket lists them, and the log
  rules of 2026-10-15 and 2026-10-16 that SPEC-021 records. No new actor
  kind. No new log identity.
- Tier 1 runs at pre-commit, so `TestToolsMatchGolden` and
  `TestRunOutputMatchesCommittedToolsJSON` must be green in the same commit
  that changes a description.
- `check:drift` diffs `git status --porcelain` over `contract/gen`,
  `cmd/vtt/tools.json` and `cmd/vtt/webdist`, so it can pass only after the
  commit. Run it after each of Tasks 1 and 2 is committed, and inside Task 4.
- The existing tests that read the descriptions stay green. Each of the
  following is a probe result:
  - `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`: `kind` keeps
    "not about who controls"; `actorId` keeps "At most 128 bytes" and never
    says "Optional".
  - `TestAddActorAndPlaceTokenStateTheIDBound`.
  - `internal/engine`'s `TestTheToolsStateTheFoldsIDBound`, which reads
    `contract/gen/tools/tools.json`.
- SPEC-010 does not reach `tools/` or `contract/`. The new gateway test file
  gets its row from `python3 tools/check-comments.py --write-ledger`, which
  adds the measured share (0.0 with citation lines only).
- The coverage floors in `tools/coverage-thresholds.txt` are
  `internal/gateway` 93.5 and `tools/toolgen` 85.0. A new test does not
  lower either.
- Neither mutation gate moves:
  - Go: no production line under its packages changes.
  - TS: its inputs (`tools/ts-mutation-inputs.py`'s `TREES`) are `client/src`
    and `client/test`, not `contract/gen/ts`.
- `go doc ./internal/gateway` is unchanged. Phase 4a is handed it as it is.
- `review-record` refuses untracked files. Commit the ticket and this plan
  before Task 1's review.

## Rule 9: how MapTool does this

MapTool has a kind, and it has two values. `Token.Type` is `PC` or `NPC`, and
the GM sets it on any token at any time.

- `Zone.getPlayerTokensWithSight` keys the shared player view on `PC` alone.
- `Zone.getOwnedTokensWithSight` filters owned sight by `TokenSelection`:
  `PC`, `NPC`, `ALL` or `GM`.
- `ZoneViewModel.makePlayerView` gives a player a view through every selected
  owned token with sight. Failing that, it uses the owned tokens with sight
  under individual views, or the `PC` tokens otherwise.
- Under the default `ALL`, a player looks through any token they own,
  creature or character alike, which is the ticket's sentence.

What we take: the two-valued kind, changed at will by the table's authority.
That is a re-grant here, and it has no third value for a summon, which agrees
with the ruling. What we leave: the union of views, ownership-as-sight, and
the client-side filter. Rule 9 forbids taking those, and SPEC-015 gives each
connection one viewpoint.

## Decisions

1. **D1. `grant_actor_control`'s `kind` reads, exactly:**

   > REQUIRED. What this actor IS \*\*right now\*\*, which decides whether the
   > whole party is told it exists. ACTOR_KIND_PARTY_MEMBER when you are
   > assigning a character to the player who will run it, and when you are
   > handing a player a creature to run for a while, such as one they
   > summoned, one they possessed or one charmed into their service: while a
   > player runs it, it is a party member, with
   > a character log of its own that they may watch. ACTOR_KIND_NON_PARTY for a
   > monster or NPC you run yourself, including one you take over to run. This
   > is NOT about who is holding it: a character whose player is away is still
   > a party member, and a goblin you are running yourself is still a goblin.
   > It describes standing RIGHT NOW and it can change: when a player's run of
   > a creature ends, grant it to them again with ACTOR_KIND_NON_PARTY and then
   > revoke_actor_control them, or remove_actor it. Revoking alone leaves it a
   > party member, on every roster.

   Why it is forced:
   - It keys on who runs the creature, which is the ticket's choice.
   - It names charm, by the owner's 2026-08-23 ruling that a monster charmed
     into a player's hands is a party member
     (`TestTheGrantDeclaresTheActorsKind`). The verdict's draft left charm
     out; the reading review before Task 0's commit found the ruling.
   - The run's end names both calls. The non-party grant takes the creature
     off the party, and the revocation takes it out of the player's hands,
     since a non-party creature still in their control moves under their own
     character's eyes. Added by the ticket's writer after the verdict, which
     left it open (its owner point 3).
   - The revocation-alone sentence is true by `engine.Apply`'s
     `ActorControlRevoked` arm, which leaves kind alone. It warns against the
     move the ruling says does not end a log.
   - "such as" follows the ticket's key: any creature a player is to run,
     which the owner's "så länge den ägs av någon" agrees with.
2. **D2. In `add_actor`'s `kind`, only the last sentence changes. It reads:**

   > Kind is not about who controls the actor: a character whose player is
   > away is still a party member, and a monster you run yourself is still a
   > monster. grant_actor_control states it again every time it grants
   > control,
   > and a creature a player is to run for a while is made a party member
   > there, whatever it was added as.

   Forced by item 2, by G7, and by
   `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`'s "not about
   who controls".
3. **D3. `add_actor`'s `actorId` reads:**

   > The new actor's id. Never reuse the id of an actor this campaign has had,
   > even one since removed: a character log is named by its actor's id, so a
   > party member added under a used id continues the earlier actor's log, if
   > it had one. At most 128 bytes of UTF-8; a longer id refuses the whole
   > command.

   Forced by rule 2 of the ticket, by VTT-318's
   `TestANewActorUnderAnOldIdContinuesItsLog`, by `engine.Apply`'s
   `ActorAdded` arm (it refuses only an id the world holds now), and by the
   two bound tests above. Nothing refuses a reuse. Only this sentence guards
   it.
4. **D4. `T1` and `T2` assert exact substrings, the shape of VTT-265's
   `TestMoveTokenReasonSaysOnlyTheDMAndTheAgentReadIt`.** Each assertion was
   red on today's text and green on D1-D3 (probe).
   - **T1** `TestGrantActorControlSaysACreatureAPlayerRunsIsAPartyMember`. The
     `kind` description contains "a creature to run for a while", "summoned",
     "possessed", "charmed into their service", "when a player's run of a
     creature ends", "ACTOR_KIND_NON_PARTY and then revoke_actor_control them,
     or remove_actor it" and "Revoking alone leaves it a party member". It
     does not contain "for someone to operate". The charm and run-end phrases
     were changed by the writer after the probe; Task 1 measures each red on
     today's text before relying on it.
   - **T2** `TestAddActorSaysAnIDIsNeverReused`.
     - `add_actor`'s `kind` contains no "charm", and does contain "a creature
       a player is to run for a while is made a party member there".
     - `actorId` contains "Never reuse the id of an actor this campaign has
       had" and "continues the earlier actor's log".
5. **D5. Regeneration.**
   - `task generate:contract` regenerates `contract/gen/tools/tools.json`
     (`go run ../tools/toolgen -o gen/tools/tools.json`) and its `cp` line
     refreshes `cmd/vtt/tools.json`.
   - `contract/testdata/expected_tools.json` is edited by hand, the same
     three lines. No task names it, and the plans of 2026-10-04 to 2026-10-07
     did the same.
   - Then
     `cmp contract/testdata/expected_tools.json contract/gen/tools/tools.json`
     and `cmp cmd/vtt/tools.json contract/gen/tools/tools.json` both print
     nothing. They are byte-identical today.
   - The probe's `diff` against today's `cmd/vtt/tools.json` showed three
     changed lines, the `add_actor` `actorId` and `kind` descriptions and the
     `grant_actor_control` `kind` description, and nothing else.
6. **D6. `ActorKind`'s comment.** The paragraph beginning "Control is
   transient" and the "AN ENUM RATHER THAN A BOOL" paragraph are replaced by:

   ```
   // Control does not decide it. Holding an actor and what the actor is are two
   // facts, and only a creation or a grant states the second. Before this field
   // the roster exception read "has any controller", which got both ordinary
   // cases backwards: a player's character whose player had left, and been
   // revoked, was still a party member and would have been dropped from every
   // roster, and a monster a participant held only to move it, the agent
   // running a goblin among them, would have been on every roster with its
   // whole stat block published to the party.
   //
   // AN ENUM RATHER THAN A BOOL, and not for symmetry with JoinDoor/PresenceState.
   // A third case is foreseeable — a neutral, say — and a bool would have to be
   // RENAMED to admit it, which ADR-007 forbids. A creature a player runs for a
   // while, summoned or possessed, is not that case: it is a party member while
   // they run it (VTT-389). Growing this enum is additive; every future value is
   // "not a party member" to the rule below until something deliberately says
   // otherwise, so growth fails closed.
   ```

   The rest of the comment is kept word for word.
   - The argument that control does not decide kind is kept.
   - "what a creature IS is not [transient]" goes, because standing changes
     (`ActorControlGranted.kind`'s own comment says so).
   - "a familiar, a summon" goes, for G1.
   - `ActorControlGranted`'s comment stays. "A charmed monster becomes a
     player's to run and then becomes a monster again" agrees with the ruling
     and with 2026-08-23.

   VTT-389 is R1's row. It points at a row, not at SPEC-021,
   because SPEC-021 gains the ruling only in Task 6.
7. **D7. The walk lives in a new file, `internal/gateway/creature_test.go`,
   package `gateway_test`.**
   - Forced by the ticket's "a new test file".
   - It reuses `newGWFixture`, `participantID`, `appendTwoRooms`, `tokenAt`,
     `partyMemberOf`, `appendEvents`, `logOf`, `requireLog`, `setViewpoint`,
     `moveTo`, `unseenSquare`, `(*gwFixture).watch` and `watchConn`'s
     `opening`, `entries`, `command`, `untilViewpoint` and `quiet`.
   - Its one new helper builds a `GrantActorControl` command with a kind, and
     carries no doc comment. `grantActorControlCmd` (in `authz_test.go`) is
     fixed to `a1` and takes no kind, and QA files' helpers are not depended
     on.
   - The grants go through the DM's connection (`dm.command`), not
     `appendEvents`. The ticket's subject is what the agent's
     `grant_actor_control` causes, and the command path includes
     `validateGrantActorControl` and `ToEvent`. This is a preference, and it
     has that reason.
8. **D8. The walk's fixture.** These are the probe's choices:
   - The player is the fixture's other player, who controls nothing in
     `newGWFixture`.
   - Their party member is `ana` at (1,1) on `rooms` (`twoRoomsTiles`, with
     the door at (3,1) closed).
   - `wolf` is added `ACTOR_KIND_NON_PARTY` at (5,1), as `add_actor`'s
     guidance tells the agent to add a creature.
   - A public note `lore` is upserted before the grant.
   - Forced:
     - `wolf` sorts after `ana`, so `defaultViewpoint` stays on `ana` and the
       demotion's move is to the character, not to an accident of ids.
     - The fixture's session stays open throughout, so a correct resume
       writes no `SessionStarted`.
9. **D9. The accepted move comes first, onto (4,1), which `ana` cannot see.
   The refused move comes second, onto (2,1), which `ana` sees and the wolf,
   now at (4,1) behind the closed door, does not.** Forced so that each half
   tells the creature's eyes from the character's, and has a break that reds
   it alone (Task 3).
10. **D10. "No second opening" is observed as exactly one `SessionStarted`
    and exactly one `NoteUpserted` of `lore` across the second replay.**
    Forced because `opening` writes both and `resume` writes neither while
    the session it holds is the world's (`resume`'s doc, SPEC-021). Around
    it, three more checks:
    - every entry numbered from 1 without a gap (VTT-321);
    - the first `len(paused)` entries `proto.Equal` to what the connection
      was first sent (VTT-326);
    - more entries than at the pause.

    Compare with `proto.Equal`, never by `EventId` alone: an opening's, a
    resume's and the projection's own entries carry none, and a forwarded
    entry carries the world event's id, which other logs share (SPEC-021).
11. **D11. The walk cites VTT-316, VTT-317, VTT-318, VTT-321, VTT-326,
    VTT-340, VTT-342, VTT-343 and VTT-350, and joins each row's evidence
    cell.** The ticket names six. VTT-321, VTT-326 and VTT-342 are added
    because the walk asserts each one: numbering, an unchanged prefix, and a
    switch followed by the new log from its first entry. The walk does not
    reuse an id, so it cites no VTT-313.
12. **D12. The escape gets a recipe in `docs/verification-debt.md`.**
    - Forced by CLAUDE.md ("the ONE file where escaped defects go") and
      `docs/verification-debt.md`'s own "write one when a defect escapes":
      this is its fix.
    - The recipe: put back "a charmed monster is still a monster" in
      `add_actor`'s `kind`, and "for someone to operate" in
      `grant_actor_control`'s.
    - Should have caught it: the reading review of `42f7af9`. That commit put
      the kind on the grant after the 2026-08-23 revision, wrote "for someone
      to operate" beside its own charm clause, and left `80dfa0e`'s "a
      charmed monster is still a monster" in `add_actor`'s description.
    - Label: `spec silent`. No requirement named what the descriptions say,
      and T1 and T2 now do.

13. **D13. SPEC-021 gets the ruling, and no other specification changes.**
    The following were read: SPEC-015's "who may watch whom", "the default",
    "what moves a seat" and "the move gate's viewpoint"; SPEC-016's roster
    and kind consequences; SPEC-017's "control ends with the actor";
    SPEC-018's description bounds; and SPEC-007's "what a command refuses".
    None says what kind a creature a player runs has, and none is made false.

## Requirements

Allocated by `requirement-id` after sign-off, on 2026-10-19. Each new row
starts as `**OPEN — no test yet**`.

- **VTT-389, R1** (ticket rule 1): "The `grant_actor_control` tool describes
  its `kind` as `ACTOR_KIND_PARTY_MEMBER` for a creature a player is to run
  for a while, and tells the agent to end that run by a non-party grant and a
  revocation, or by removing the actor." Verified by T1, which goes red when
  the description offers `ACTOR_KIND_NON_PARTY` for a creature handed to
  someone to operate or drops how a run ends.
- **VTT-390, R2** (ticket rule 2): "The `add_actor` tool's `actorId`
  description tells the agent never to reuse the id of an actor the campaign
  has had." Verified by T2, which goes red when the reuse sentence is gone.
- **VTT-391** (found by the reading review before Task 0's commit): "A grant
  restates the actor's kind even when the participant already controls the
  actor." Existing behaviour that D1's non-party grant relies on, held by
  `internal/engine/actor_control_test.go`'s
  `TestARegrantToAnExistingControllerStillRestatesTheKind`.
- **VTT-392** (the same): "A revocation leaves a party member a party member."
  Existing behaviour that D1's "Revoking alone" sentence states, held by
  `TestRevokingControlLeavesAPartyMemberAPartyMember`. Its row says a party
  member, since that test holds no other kind.

Both tests gained their id line in Task 0's commit. The second test's
eight-line argument became a two-line warning, so the file's comment share
stays under its ceiling.

The sort refused six candidates:

- `add_actor`'s `kind` saying the grant decides: the same guidance as R1, from
  the other tool. T2 checks it, and it earns no row.
- Watching another character changes no kind: not new. Only `ActorAdded` and
  `ActorControlGranted` write a kind, and VTT-391 and VTT-392 hold the two
  ways control moves.
- A creature a player runs is a party member: no rule the system holds. It is
  the DM's choice at the grant, and R1 is the guidance.
- `ActorKind`'s comment keeping its argument: prose, with nothing observable
  at run time.
- The walk's steps: VTT-316, 317, 318, 321, 326, 340, 342, 343 and 350 hold
  them.
- A party member removed out of a character's sight staying on its roster:
  SPEC-016's behaviour, which the owner ruled to leave as it is on 2026-10-19
  ("Låt det vara").

Two further notes:

- T2's `kind` half is held by no row. It stays as the description's own
  check, as the 2026-10-18 plan left T7's half.
- The walk adds no row (the ticket: "adds no rule"). It joins nine rows'
  evidence (D11).

## Tasks

The pattern in every code task:

1. Write the tests and see each fail for the reason stated.
2. Implement, and see them pass.
3. Apply each break, see the named test go red, then restore it by the
   inverse edit, never `git checkout --`, and verify the restore with grep.
4. Run `python3 tools/check-comments.py main` after staging.

Each task is its own commit through the review gate. Stage and commit in
separate calls.

### Task 0: sign-off and ids

- Files: `docs/requirements.md`, the ticket and this plan, all committed.
- True when the owner has signed off D1-D13. The verifier's three open points
  are settled in the plan as it stands: any creature a player is to run
  (D1's "such as"), charm by the same key, and a run ended by both a non-party
  grant and a revocation. R1 and R2 are allocated, each OPEN.

### Task 1: the agent's descriptions (ticket items 1-3)

- Files:
  - `tools/toolgen/main.go`: the three `fieldDocs` per D1-D3.
  - `tools/toolgen/main_test.go`:
    - T1 and T2, each inserted after a closing brace, carrying `// VTT-389`
      and `// VTT-390`;
    - `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication`'s doc, so
      that actorId "must state the fold's bound and say nothing of being
      optional";
    - the comment in that test's body, so that it ends "a monster the agent
      runs is still a monster", and its sentence "where actorId's description
      only states the bound", which D3 makes false (G3 and the reading
      review).
  - `contract/gen/tools/tools.json` and `cmd/vtt/tools.json`, by
    `task generate:contract`.
  - `contract/testdata/expected_tools.json`, by hand (D5).
  - `docs/requirements.md`: R1's and R2's evidence.
- True when:
  - T1 and T2 fail on today's `main.go` and pass on D1-D3.
  - Each of the three JSON files differs from main in three lines, and the
    `cmp`s in D5 print nothing.
  - `go test -count=1 ./tools/toolgen/ ./internal/engine/ ./internal/mcp/ ./contract/`
    is green, and so is `go test -count=1 ./cmd/vtt/` (about 140 s).
  - After the commit, `task check:drift` is green.
- Breaks, each measured red in the probe and green again once restored:
  - Put "for someone to operate" back in the NON_PARTY clause: T1.
  - Replace the run-ends sentence with "it may change.": T1.
  - Delete ", such as one they summoned, one they possessed or one charmed
    into their service": T1.
  - Delete " and then revoke_actor_control them": T1.
  - Delete the revocation-alone sentence: T1.
  - Put "and a charmed monster is still a monster" back in `add_actor`'s
    `kind`: T2.
  - Put "unique in the campaign" back in `actorId` in place of the reuse
    sentence: T2.
- Phase 4a: none. The task delivers prose to the agent and generated files,
  checked by T1 and T2. Whether an agent follows the prose is out of scope by
  the ticket's own non-goal, and no gate runs it.

### Task 2: `ActorKind`'s comment (ticket item 4, G1)

- Files:
  - `contract/vtt/v1/events.proto` (D6), then `task generate:contract`. Of the
    generated files it should move only `contract/gen/go/vtt/v1/events.pb.go`
    and `contract/gen/ts/vtt/v1/events_pb.ts`, as `a0c48c9`'s comment-only
    proto edit did; that edit also moved alignment lines in `events.pb.go`.
  - `internal/gateway/add_actor_validate_test.go`, the doc of
    `TestAddActorAcceptsAKindTheEnumDoesNotYetDefine`, and
    `internal/gateway/grant_validate_test.go`, the doc of
    `TestEveryActorKindTheContractOffersIsAcceptedByAGrant`. Both repeat "a
    neutral, a familiar, a summon" from the comment D6 rewrites (found by the
    reading review). Rewrite each to D6's words and run
    `python3 tools/check-comments.py` against main and HEAD after staging.
- True when:
  - `grep -rn 'still a monster\|a familiar\|a summon' contract/vtt/v1/events.proto contract/gen/go/vtt/v1/events.pb.go contract/gen/ts/vtt/v1/events_pb.ts internal/gateway`
    prints nothing.
  - `grep -c 'Control does not decide it' contract/vtt/v1/events.proto` prints
    1.
  - `git diff --stat` names the three files alone.
  - `go test -count=1 ./contract/` and `bun test contract/` are green.
  - `task check:breaking` prints what it printed before the edit. Pre-release
    it reports and exits 0, and buf compares no comments.
  - After the commit, `task check:drift` is green and `cmd/vtt/webdist` is
    unchanged. The webdist holds none of today's comment text.
- Break: none; it is prose.
- Phase 4a: none; it is prose.

### Task 3: the walk (ticket item 5)

- Files:
  - `internal/gateway/creature_test.go` (new):
    `TestACreatureGrantedToAPlayerAsAPartyMemberIsRunAsOne` (D7-D11).
  - `tools/comment-ceilings.txt`: its row, by `--write-ledger`.
  - `docs/requirements.md`: nine evidence cells.
- The walk, in order:
  - **A0.** The fixture is per D8. The connection opens on `ana` and is
    drained to `logOf("ana")`. The wolf's log is empty.
  - **A1.** The DM grants `wolf` to the player as `ACTOR_KIND_PARTY_MEMBER`.
    `ana`'s new entries are drained by length. `set_viewpoint wolf` is ok,
    and `untilViewpoint` answers `wolf` with no entry before it. The wolf's
    whole log follows, `requireLog` against `logOf("wolf")`, and its entry 1
    is a `SessionStarted` with sequence 1.
  - **A2.** The move onto (4,1) is ok, and the move onto (2,1) is refused
    with `unseenSquare` (D9). The move's entries are drained.
  - **A3.** The DM grants `ACTOR_KIND_NON_PARTY`. `untilViewpoint` answers
    `ana` with no entry before it, then `ana`'s log arrives from its first
    entry. The wolf's log length is unchanged.
  - **A4.** The DM grants `ACTOR_KIND_PARTY_MEMBER` again, and `ana`'s
    entries are drained. `set_viewpoint wolf` brings `wolf` with no entry
    before it, then the whole log, checked per D10. The connection then stays
    `quiet(300ms)`.
- True when:
  - The walk passes on the tree as it stands. Item 5's point is that it
    passes before and after.
  - `go test -race -count=20 -cpu 1,2 -run TestACreatureGrantedToAPlayerAsAPartyMemberIsRunAsOne ./internal/gateway/`
    is green. The probe passed it at a load of about 4; check
    `os.getloadavg()` first.
  - `go test -count=1 ./internal/gateway/` is green.
- Breaks: an edit to production code for each assertion. Each was measured
  red at its own assertion, with every earlier one green.
  - **A1, accepted:** in `engine.Apply`'s `ActorControlGranted` arm, make the
    kind write unconditional-false (`if false {`). It reds with "not a party
    member you may watch".
  - **A1, from its opening:** in `pumpState.onPerch`, change
    `ps.sub.watch(actorID, 0)` to cursor 1. It reds with "no frame within
    3s".
  - **A2, accepted half:** make `seat.judgedBy` answer
    `defaultViewpoint(p, actors)` always. It reds on the move onto (4,1).
  - **A2, refused half:** disable the `!canSee(...)` refusal in
    `handleCommand`. It reds on the move onto (2,1), which is accepted.
  - **A3:** make `seat.resolve` keep any non-empty `shown` without
    `MayWatch`. It reds with no `ViewpointChanged`.
  - **A4, no second opening:** in `fanOutLocked`, have the pause branch
    `delete(logs, id)` in place of `l.decision = nil`. It reds with "2
    `SessionStarted`". The numbering stays continuous under this break,
    because the store numbers from its own head.
  - **A4, resumed:** in `fanOutLocked`, set `lead = nil` in place of
    `resume(l.fold, before)`. It reds with "the resume wrote nothing".
  - **A4's numbering and prefix:** no break leaves A1 green. The store's
    `COALESCE(MAX(seq), 0) + 2` reds the walk at A1's sequence-1 check. These
    clauses are held in depth by VTT-321's and VTT-326's own tests.
- Phase 4a: one QA agent. The claim "no code is needed" rests on one test
  written by people who expect it to pass, and an independent derivation is
  the check against confirming that hypothesis.
  - It receives the 2026-10-19 ruling, VTT-316/317/318/321/326/340/342/343/350,
    SPEC-015 and SPEC-021 whole, and `go doc ./internal/gateway` and
    `go doc ./internal/campaign`.
  - It is withheld the walk, this plan, the verdict and the source.
  - Name `go-arch-lint` in its brief. Hash the tree before and after it runs.
  - Its tests are committed with the task. Adjudications go in the report,
    one line each.

This is the last code commit.

### Task 4: `task check`, whole, on Task 3's commit (ticket item 7)

- True when `task check` exits 0 and prints its own completion lines. Launch
  it with `start_new_session=True`. It includes `check:drift`, which is now
  over committed files.
- No floor or ceiling is lowered to pass, and no ceiling row is raised.
- The records after Task 4 change no `.go` or `.ts` file. Before each record
  commit, run `task check:requirements-chain`, `task check:new-prose` and
  `python3 tools/check-comments.py` against main and HEAD; no hook runs them.

### Task 5: the implementation report and the escape's recipe (D12)

- Files:
  - `docs/reports/2026-10-19-a-creature-a-player-runs-is-a-party-member.md`,
    by the `implementation-report` skill, against the ticket's seven items,
    each with its observation. It includes G1-G7, each break and what it
    reddened, Phase 4a's adjudications, and the figures from Task 4's run.
  - `docs/verification-debt.md`: a dated section above `## Open debt`, per
    D12.

### Task 6: the specification step (ticket item 6)

- File: `docs/specifications/021-each-party-member-keeps-its-own-log.md`.
  - After "A pause and a resume", a paragraph headed **A creature a player
    runs.** Roughly: a creature a player runs for a while, summoned or
    possessed, is a party member while they run it (the owner's ruling,
    2026-10-19). It has no kind of its own. It is granted
    `ACTOR_KIND_PARTY_MEMBER`, and `ACTOR_KIND_NON_PARTY` or removed when the
    run ends. Its log is therefore a party member's:
    - it opens at the grant (VTT-317);
    - it pauses at the second grant or the removal (VTT-316);
    - it resumes without a second opening when the creature is granted to a
      player again (VTT-318).

    The agent is told so (R1), and told never to reuse an id (R2).
  - Consequences, three bullets:
    - A creature possessed a second time resumes its old log.
    - A creature added under a used id continues that id's log, and nothing
      but R2's sentence prevents it (VTT-318; VTT-313 for the viewers).
    - Revoking a creature's last controller leaves it a party member and its
      log written (`engine.Apply`'s `ActorControlRevoked` arm).
  - The Status line gains the walk's file. The Requirements list gains R1 and
    R2.
- True when the passage passes the `specification` skill's catches list,
  read against the committed code.

## For Phase 4a

- Tasks 1 and 2 have none (prose and generated files). Task 3 has one agent,
  as scoped above.
- `go doc ./internal/gateway` is unchanged.

## Amended by the reading review before Task 0's commit

One reviewer read the ticket, this plan and the new rows against `abf4eb5`,
by command. Its Must and Shoulds were applied before the commit:

- the sentences' origin: "for someone to operate" is `42f7af9`'s, the other
  two `80dfa0e`'s (G2, D12, the ticket);
- charm: the owner ruled it on 2026-08-23, so D1 names it and T1 asserts it;
- MapTool: under its defaults an unselected owned NPC is not looked through,
  and the GM making it a `PC` is what does that (rule 9, the ticket);
- D2's "every time it grants control", since a revocation states no kind;
- D1's revocation sentence, which contradicted D1's own "while a player runs
  it";
- D10's reason for `proto.Equal`;
- a third false sentence in `main_test.go` (Task 1) and two gateway test
  comments that repeat D6's removed words (Task 2);
- VTT-391 and VTT-392 for the two fold behaviours D1 relies on;
- Task 4's claim that a hook runs the record gates.

Its Nits were applied too: the constraints' rulings, a rule 8 phrase, the
sort's count, `a0c48c9`'s alignment lines, D3's condition, and D6's two
examples.
