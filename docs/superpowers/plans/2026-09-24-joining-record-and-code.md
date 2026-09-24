# Joining record and code — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-24-joining-record-and-code-design.md`
**Verified:** 2026-09-24, by `verify-ticket`, an agent that did not write the
ticket, against `8ebb2b1`. Verdict: **Passes with gaps.** The gaps are listed
at the end and travel with this plan. This plan does not edit the ticket.

**Goal, in the ticket's words:** the joining-a-table arc's decisions are one
record, and its code carries only warnings.

**Scope, from Patrik's ruling of 2026-09-23:** this is the SECOND HALF of the
arc. The first half (`docs/reports/2026-09-23-joining-rules-and-the-chain.md`)
landed the chain gate and thirty-seven rows. This half writes SPEC-009, sorts
`internal/identity/identity.go`'s comments, removes `JoinAllows`, removes the
pointers that resolve nowhere, closes the three `OPEN` rows with tests, and
registers the rules the arc's tests already hold.

**Design reused:** the sort of the comments follows section 6 of
`docs/reports/2026-08-09-joining-a-table.md`, which says for each of sixteen
blocks what must stay at the line. That report keys its blocks by line range;
this plan re-keys them by the symbol each block sits on (the "Per-block sort"
table below), because the sort moves every line below the first edit.

**MapTool (CLAUDE.md rule 9), answered in writing.** Read on 2026-09-24 in
`~/dev/RPTool`, `maptool/src/main/java/net/rptools/maptool/model/player/`:
`Player.java` holds `enum Role { PLAYER, GM }` on the Player object; the role
is fixed at the handshake by which password the client presented
(`ServerHandshake`, `getRolePassword(Role.GM)`); a runtime change goes through
`Players.setRole`, which writes the server's password-file player database
(`PasswordFilePlayerDatabase`) and, by its own Javadoc, "will not change the
role of a currently logged in player, they will have to log out and log back
in". No hit for `Role` in `Campaign.java` or `PersistenceUtil.java`: the role
is never persisted with the campaign, and MapTool has no event log for it to
enter. So for the two rules this ticket closes with tests — no event payload
names a role, a promotion appends no event — MapTool agrees in shape: a role
lives beside the credential, never in campaign data, and a role change is a
database write with no campaign-side record. That is exactly `participants.role`
beside `token_hash`. Where MapTool differs, the arc has already ruled against
it: the reconnect-to-take-effect model was rejected by Patrik on 2026-08-09
(report of that date, section 3; VTT-031 holds the opposite), and its
distribution model is out by rule 9's own text. The comment sort, the
specification and the removal of a dead function are process work with no
tabletop precedent; there is nothing to borrow and nothing to reject.

---

## Constraints that bind every task

The project's standing rules, `CLAUDE.md`, by number:

- **Rule 2.** No gate is weakened. The coverage floor for `internal/identity`
  in `tools/coverage-thresholds.txt` is not lowered to absorb the deletion of
  `JoinAllows` and two tests; `check:doc-owner` and `check:new-prose` are not
  hatched to pass a rewritten comment.
- **Rule 3.** The contract is additive only. Task 2's deliberate break for the
  role-in-payload test adds a field to a proto message, regenerates, observes
  red, and reverts both the proto and the generated tree before anything is
  staged. Nothing of it is committed; `check:drift` on the committed tree stays
  clean.
- **Rule 4.** Nothing here touches the fold.
- **Rule 5.** SPEC-009 and the register rows use platform vocabulary (door,
  secret, budget, spectator, promotion), which rule 5 permits; `check:invariants`
  scans `internal/` and `cmd/`, where only comments and tests change.
- **Rule 8.** Every citation this work writes — register rows, test comments,
  SPEC-009, rewritten code comments, the report — names a test, a file, a
  symbol, a constant or a dated decision. Never a line number, never a path
  into `.superpowers/`, never "this task". The two pointers the ticket names
  (`task-3-brief.md`, the `15.5µs` figure) go; so does `(P6 Task 4 review)` in
  `Open`'s body, which is the same shape and which the ticket's grep does not
  hunt (gap 4).
- **Rule 9.** Answered above.

The process's rules, from the installed `dev-cycle` package (0.4.0):

- Requirement ids come from `requirement-id` and from nothing else, allocated
  AFTER Phase 2 sign-off. Measured 2026-09-24 on a scratchpad copy of the
  register: the dispenser allocates `VTT-038`, writes
  `| VTT-038 | <sentence> | **OPEN — no test yet** |`, and leaves no `.lock`.
  The real register was untouched (`git status --short docs/requirements.md`
  printed nothing).
- Tests before code, then the deliberate break: one break per check relied
  on, an act and not an artifact, recorded as a line in the commit.
- Phase 4a's QA receives the requirement and the specification, never the
  diff, the source or the existing tests. It runs once here, on the three
  behaviour rules (D3), and is skipped for the comment sort, the rows, the
  citations and the report (prose; no behaviour to derive).
- One reading review per commit, recorded with `review-record.sh --summary`.
  The recorder fingerprints `git diff HEAD` over the whole working tree plus
  every untracked file, so a commit's working tree must be exactly its diff.
- The implementation report follows the last code commit, per the
  `implementation-report` skill, against the ticket's nine items in the
  ticket's numbering.
- A specification is written per the `specification` skill's steps and checked
  against `catches.md` twice at most.
- A code comment may hold what the code cannot say about itself — a
  constraint, a reason a line is unusual, a warning to whoever edits next —
  and nothing else (`requirements` skill, "Where prose may live"). That
  sentence is the rule the sort applies to every block.

Patrik's rulings, taken as given:

- Single source of truth: point rather than restate. A rewritten comment that
  needs a reason names SPEC-009; it does not repeat SPEC-009.
- Delete the old solution first, no phased removal: `JoinAllows` goes whole,
  with its comment and its twin tests, in one commit.
- A report is never revised for later work: `docs/reports/2026-08-09-joining-a-table.md`
  and `docs/reports/2026-09-23-joining-rules-and-the-chain.md` are not edited,
  even though the former's section 6 keys its blocks by line ranges that this
  work makes wrong. The per-block table below is the durable map instead.
- An ADR is frozen: `docs/adr/011-identity-and-authorization.md` is read, not
  edited, unless Patrik chooses form (ii) of D7 at sign-off.

Records that constrain how documents are edited:

- `docs/verification-debt.md` is the ONE home for an escaped defect's recipe
  and for a known coverage gap (`CLAUDE.md`, "The ONE file where escaped
  defects go"). Its entry dated 2026-09-23 gains a "Closed by" paragraph, the
  shape its 2026-09-16 entries already use.
- `internal/perceive` is absent on purpose (memory: the per-character-logs arc
  is being redone under the process). SPEC-009 describes identity, joining and
  promotion; it says nothing about what a seat sees, and names neither
  `perceive` nor `eyes()`.

Environment, measured 2026-09-24:

- `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...` is
  green on `8ebb2b1` (identity in about one second, gateway in about half a
  minute).
- `task check:requirements-chain` prints `37 rows, 185 test files, 2
  specifications; every citation resolves and every row's evidence holds.`
- `tools/mutation-equivalents.txt` holds NO key in `internal/identity/identity.go`,
  `internal/gateway/server.go` or `internal/gateway/join.go`, so the sort and
  the pointer removals move no adjudication; both self-tests
  (`tools/check_mutation_test.py`, `tools/check_ts_mutation_test.py`) are run
  anyway after every source edit, per memory, because they take seconds.
- `internal/identity`'s coverage floor is 92.1 (`tools/coverage-thresholds.txt`);
  the 2026-08-09 report measured 93.7 on 2026-09-19. `JoinAllows` is fully
  covered, so deleting it lowers the covered count and the total by the same
  amount; Task 0 measures the figure before and Task 3 after.
- `task check` whole takes over an hour; a background run is launched with
  `start_new_session` (memory: background gate runs die at 38 minutes), and
  the mutation gate refuses below 16 GiB free on the temp volume.

---

## Decisions this plan makes

**D1. SPEC-009 is written FIRST, as the first commit, and its `Requirements`
section is completed LAST.** The ticket orders it last, "after the code it
describes is final". Three measurements force the reversal. First, no
behaviour changes in this ticket: the tests are added, a dead function and
comments are removed, and the decisions ADR-011 states are as true of
`8ebb2b1` as of the final tree, so the record can be derived today and checked
against the same code (the `specification` skill's step 1). Second, the sort
(Task 3) rewrites comments that today cite "spec §2" and "spec §3.1" — the
arc's ticket of 2026-08-08 — and the only present-tense home to point them at
is SPEC-009, which must therefore exist before the sort lands. Third, Phase
4a's QA (Task 2) receives a specification, not an ADR. The `Requirements`
section names VTT-005 to VTT-037 less the withdrawn row (D2) at the first
commit, gains the successor id at Task 2 and the nine at Task 4, so the
chain gate is green at every commit; Status is amended when the pinning tests
change. A question at the end.

**D2. VTT-035 "No event carries a role." is FALSE as worded against the
tree, and the plan carries two remedies; the choice is Patrik's.**
Measured 2026-09-24: `contract/vtt/v1/events.proto` declares `string
actor_role` on `Envelope` itself, and the gateway stamps it on every event it
appends (`internal/gateway/convert.go`, `ruleset.go`, `adventure.go`,
`map.go`, and `server.go`'s append path all set `ActorRole: string(p.Role)`).
Nothing in `internal/engine`, `internal/campaign` or `internal/store` reads it
(`grep -rn ActorRole` there hits four test fixtures and no production line;
`internal/eventgen/model.go`'s own comment says the same), and no PAYLOAD
message names a role (`grep -nE 'role[a-z_]*\s*=' contract/vtt/v1/*.proto`
finds `actor_role` on `Envelope` and `role` on the `PromoteParticipant`
COMMAND, nothing else). So the rule the arc means — the fold's inputs carry no
role, a role is not campaign state — is true, and the sentence the register
holds is not, because every envelope carries the issuer's role as testimony.
The `requirements` skill: a requirement that restates a false sentence is
worse than the sentence was. The ticket's own "could not be established"
entry narrows the test to "the Envelope payload's messages" without noticing
the field beside the payload.

- **Form A (recommended).** Withdraw VTT-035: its requirement cell becomes
  `WITHDRAWN 2026-09-24: false as worded — every Envelope carries the issuer's
  role in actor_role, read by nothing that folds; superseded by VTT-0NN.` and
  its evidence cell `**READING — verify-ticket check 5 of 2026-09-24; withdrawn,
  held by nothing**`, which the chain gate accepts (a `READING` naming a
  review). Dispense the successor after sign-off: `No event payload names a
  role, and the fold's state holds none.` Neither the dispenser, the checker
  nor the skills define a withdrawn row's form beyond "keeps its row, marked";
  the form above is a stopgap and a ticket goes up to the process repository
  (a question at the end).
- **Form B.** Keep VTT-035 as worded; the test holds the payload reading;
  SPEC-009's `How it works` states the vocabulary (an event is a payload
  message; the envelope's `actor_role` is a stamp about the issuer, read by
  nothing in engine, campaign or store). Cheaper by one withdrawn row, and the
  register goes on holding a sentence a reader of `events.proto` will call
  false. Not recommended.

Under either form the test is the same (D4); only the id it cites differs.

**D3. Phase 4a runs once, on the three behaviour rules, after Task 2's tests
are green and before the reading review of that commit.** Forced by the
process: QA is for behaviour, and the three rules are the only behaviour this
ticket touches. It receives the three rows (VTT-008, VTT-036, and VTT-035 or
its successor), SPEC-009 whole, the ticket whole, and — because two of the
three rules are held at the wire and the third at the contract — `go doc` of
`internal/identity` and the docstring-level shape of the gateway test fixture
(`newGWFixture`, `dial`, `sendCommand`, `readResult`, `head`, `postJoin`)
extracted to a scratchpad file, never the test files themselves. What it
cannot write without the internal `testdb` seam (the shut-door fault test) it
records as underdetermined, and the adjudication says so.

**D4. The role test walks the payload oneof of `Envelope` with protoreflect,
recursively, and refuses any field whose name contains `role`; the fold half
is held by the same walk.** Forced by the shape the repo already uses over the
contract (`TestEveryClientCommandHasRoleCells` and
`TestEveryClientCommandConverts` iterate `Descriptor().Oneofs().ByName(...)`)
and by what the fold reads: `engine.Apply`'s only input is the payload, so if
no payload message reachable from the oneof names a role, the fold cannot hold
one. `engine.State` (`go doc ./internal/engine State`) has no role-named field
today; a `reflect` walk over it would descend into protobuf internals through
`Actors map[string]*vttv1.Actor`, so the state half stays a reading (`grep
-rn Role internal/engine/` prints nothing, measured 2026-09-24) and the plan
says so rather than writing a brittle second half. The Envelope's own fields
are NOT walked, on purpose: `actor_role` is there (D2).

Name: `TestNoEventPayloadNamesARole`, in `internal/gateway/authz_test.go`
beside its sibling walkers. Shape: take `(&vttv1.Envelope{}).ProtoReflect().Descriptor().Oneofs().ByName("payload")`;
for each field, walk its message descriptor and, recursively, every
message-typed field's descriptor, with a visited set keyed on the message's
full name; `t.Errorf` naming `<message>.<field>` for any field whose
lower-cased name contains `role`; a `t.Run` per top-level arm so the failing
event is named. Deliberate break: add `string role = 99;` to `NarrationAdded`
in `contract/vtt/v1/events.proto`, `task generate:contract`, run the test —
red naming `narration_added.role`; `git checkout -- contract/ cmd/vtt/tools.json`
and regenerate to confirm the tree is back; run `task check:drift` before
staging.

**D5. The promotion test reads the log head before and after a promotion
over the wire, with a positive control.** Forced by `server_test.go`'s own
instrument: `head` on the fixture dials a throwaway DM connection and returns
the catch-up head, which `TestRevokingRemovesSomebodyWhoIsStillConnected`
already uses in the before/after shape, and dialing appends nothing. Name:
`TestAPromotionAppendsNoEvent`. Shape: fixture; `before := f.head(t)`; the DM
promotes the spectator over the wire exactly as `TestDMPromotesASpectatorOverTheWire`
does; `readResult` ok; `f.ids.Verify(f.spectatorToken)` reports `RolePlayer`
(the positive control — a refused promotion must not pass this test
vacuously); `f.head(t)` equals `before`. Deliberate break: in `handlePromotion`
(`internal/gateway/server.go`), after `SetRole`, append one envelope through
`s.campaign.Append` — red on the head; revert.

**D6. The shut-door test is the third of the set its two siblings form.**
Forced by `docs/verification-debt.md`'s entry dated 2026-09-23, which names
the instrument, and by `fault_internal_test.go`'s two siblings. Name:
`TestAShutDoorRefusesWithoutTouchingTheDatabase`, in
`internal/identity/fault_internal_test.go`. Shape: `withFaultDriver`; `Open`;
`JoinSecret` (mints the row, so the guard is reached rather than
`sql.ErrNoRows`); `SetJoinOpen(true, 5)` then `SetJoinOpen(false, 5)` — the
door shut, the budget 5 with nothing spent, the secret intact, so the door
term is the ONLY term that refuses (a fixture check per memory: a degenerate
fixture that also trips the budget or the secret would pass with the door term
gone); `reached := testdb.Arm("SET admitted = admitted + 1", errDBDown)`;
`JoinAdmits(secret)`; `reached()` must be false; the answer must be
`(false, nil)`. Deliberate break: the debt entry's recipe — remove the
`open != 1` term from `JoinAdmits`'s guard — red; revert. The citation
`// VTT-008` sits on the last comment line above the `func`.

**D7. ADR-011 is not edited (form i), and the plan carries the other form.**
Forced by `CLAUDE.md`'s "the file itself is never rewritten" and by the
convention that paragraph states: the present tense moves out to a numbered
file under `docs/specifications/`, which is how a reader of any ADR finds its
specification. The reverse link ADR → SPEC is carried by this ticket's report,
which names the record it produced. Form (ii): append one line to ADR-011,
`Present tense: docs/specifications/009-identity-and-joining.md`; that
overturns the sentence in `CLAUDE.md`, which would then have to say a pointer
is the one permitted edit, and it sets the form for every ADR read this way.
Patrik's call at sign-off.

**D8. SPEC-009's `How it works` is derived from ADR-011's Decision
paragraphs, each checked against the code before it is written.** Per the
`specification` skill's steps 1 and 3. The derivation, with the file that
verifies each paragraph:

| ADR-011 decision | Verified against |
|---|---|
| Identity is not event-sourced; the fold holds no role | `identity.go` (`participants`, `join_access`, own `sql.Open`); `grep -rn Role internal/engine/` empty; `.semgrep/event-sourcing.yml` |
| Authentication once, authorization continuously; only an invalid token ends a connection | `server.go`: `Verify` before the upgrade, `Lookup` in the read loop, `credentialGone` per delivery, `revoked()` per presence frame; the `ErrInvalidToken` arm closes, every other error goes to `answerCommand` |
| One shared link admits, the DM promotes; reconnecting keeps the participant | `join.go` `handleJoin` → `JoinAdmits` → `CreateInvite(name, RoleSpectator)`; `handlePromotion` → `SetRole` |
| Every refusal is decided in Go, from one read, before anything is written; the UPDATE re-states the condition | `JoinAdmits` (the guard, then the `UPDATE ... WHERE id = 1 AND open = 1 AND admitted < admit_limit`) |
| The three refusals give one answer | `handleJoin`'s single `http.StatusForbidden` body |
| Admission is a count, per opening; absent or non-positive becomes the default | `SetJoinOpen` (`admitLimit <= 0`, `admitted = 0` on every call), `DefaultAdmitLimit`, `RotateJoinSecret` (`admitted = 0`, `open` and `admit_limit` untouched) |
| Promotion is a command; two bounds in two places; changes only the role; revoked stays revoked | `commandRoles` and `authorizePromotionTarget` in `authz.go`; the FROM check in `handlePromotion`; `SetRole`'s single `UPDATE participants SET role`; `notConverted` in `convert_test.go` |
| Revoked participants are not listed | `List` (`WHERE revoked = 0`) |
| Not decided: rate limiting; who may control which character | `handleJoin` has none; `grant_actor_control` is the gateway's, against the log |

What SPEC-009 says that ADR-011 does not, because the tree does: every
envelope carries `actor_role`, a stamp about the issuer read by nothing that
folds (D2); the empty-secret guard (`secret == ""`) and why
(`subtle.ConstantTimeCompare` of two empty strings is a match, and an omitted
JSON field decodes to `""`); the display-name bound and its refusal being
distinct from the door's (`usableDisplayName`); the body cap (`maxJoinBody`).
Where SPEC-007 already states a wire-level fact (which commands append
nothing; the roles `promote_participant` accepts), SPEC-009 names SPEC-007
and does not restate it. `Principles served` carries the same line as
SPEC-008: no blueprint, the principle is missing rather than absent. Status:
`Accepted. Implemented by internal/identity/identity.go,
internal/gateway/join.go, internal/gateway/authz.go, internal/gateway/server.go;
pinned by the checks the rows under Requirements name, in
internal/identity/identity_test.go, internal/identity/fault_internal_test.go,
internal/gateway/join_test.go, internal/gateway/authz_test.go and
internal/gateway/server_test.go.` No count, no line number, no path outside
the project, no "only/never/every" without the search named beside it.

**D9. The sort is applied to EVERY comment block in `identity.go`, using
the report's sixteen "stays" lists where the report has one and the
`requirements` skill's rule where it does not.** Forced by the ticket's Done
item 4, whose grep and reading are over the whole file: three of the lines it
matches today sit outside the sixteen blocks (`migrate`'s read-first block,
`ensureJoinRow`'s read-first block, `newSecret`'s doc), and `CreateInvite`'s
doc carries a date. The per-block table below is the design; the report's
line-keyed section 6 is its source and is not edited.

**D10. Three things the sort moves OUT of `identity.go` get a home in the
same commit, or the sort is not a sort but a deletion.** Forced by the
`requirements` skill's table (a reason moves to a report or a specification;
a known coverage gap goes to the debt file). (a) `migrateLocked`'s note that
its re-read error arm is uncovered and unreachable through `testdb` becomes a
known-gap entry in `docs/verification-debt.md`, labelled `test data missing`
and `outside the tool`, and the comment keeps one line pointing there. This
is a file the ticket already lists, and one paragraph beyond what the ticket
names; the report records it as a deviation. (b) The empty-secret warning in
`JoinAllows`'s doc — the one sentence in that block that the code cannot say
— MOVES to `JoinAdmits`'s doc, whose guard carries `secret == ""` with no
sentence beside it today. (c) Reasons that are decisions go to SPEC-009,
which D1 lands first; the rewritten comment names SPEC-009 where a reader
would otherwise ask why.

**D11. `Open` gets its own doc comment.** Measured 2026-09-24: the block
opening "Open opens (creating if necessary)..." sits above `var driverName`,
not above `func Open`, so `go doc ./internal/identity Open` prints no
description and the block documents a variable. `check:doc-owner` does not see
it (it inspects blocks above `func` lines). The sort splits the block: `Open`'s
sentences above `func Open`, `driverName`'s above the var. No gate change.

**D12. Two tests move from `JoinAllows` to `JoinAdmits` and hold the same
thing; two are deleted as twins; one loses an assertion.** From the
verification's reading, recorded here so the reviewer of Task 3 checks it
rather than re-derives it:

- `TestRotatingBeforeAnythingElseLeavesTheDoorSHUT`: after `RotateJoinSecret`
  on an untouched campaign the row exists with `open = 0`, `admitted = 0`,
  `admit_limit = 0`; `JoinAdmits(secret)` refuses at the door term, spends
  nothing, returns `(false, nil)`. Same holding.
- `TestTheDoorNeedsBOTHTheFlagAndTheSecret`: `SetJoinOpen(c.open, 100)` runs
  before EVERY cell and resets `admitted` to 0, so the one admitting cell
  spends one of a hundred and the three refusing cells return before the
  `UPDATE`; the four expectations are unchanged. The move adds the spend path
  to what the test exercises, which other tests already cover. A budget of 1
  would also hold; 100 stays because nothing turns on it.
- `TestAnEmptyStoredSecretAdmitsNobody` (offers `""` and `"anything"`) is
  deleted; `TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath` holds the
  `""` case on the live function, and `"anything"` against an empty stored
  secret is refused by the match term regardless, so it holds nothing about
  the empty-secret guard.
- `TestCheckingTheDoorMintsNothing` is deleted;
  `TestJoinAdmitsOnACampaignWithNoDoorRowRefusesWithoutCreatingOne` is the
  same test on `JoinAdmits`, row-count assertion included.
- `TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting` drops its
  `JoinAllows` assertion; its `JoinAdmits` assertion, three lines above, holds
  the rule.

**D13. The nine candidates are handed to the sort with what the reading
found, not decided here.** The sort is the `requirements` skill's, run in
Phase 2 after sign-off. What it will face:

- "Rotating the link leaves the door as it was": the ticket names
  `TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse`, whose last assertion
  holds only that an OPEN door stays open. `TestRotatingTheSecretLeavesTheDoorAlone`
  in `identity_test.go` walks both `open` states and is the stronger
  evidence; the ticket does not name it. Both, or it.
- "Rotating on a campaign nobody has opened leaves the door shut" is the
  untouched-campaign case of the rule above, pinning `RotateJoinSecret`'s
  INSERT-branch literal. One row with three tests, or two rows; the sort's
  call, and "state fewer rules" leans to one.
- "The door admits only when open and the secret matches, and refuses the
  other three cases": the refusing cells are VTT-005 and VTT-007; the
  admitting cell over the wire is VTT-010's precondition. The distinct
  observable is the positive cell at the package boundary; "each for its own
  reason" is not observable (every refusal is `(false, nil)`). Accept as "A
  join carrying the current secret at an open door with budget is admitted",
  or refuse as scenery.
- "A database that cannot answer keeps the door shut": three tests, one at
  the closed-handle level, one with the SELECT fault armed, one across the
  package's writers. Distinct from VTT-033, which is the gateway's side.
- The other five (rotation resets the budget; an admission that cannot be
  spent is not granted; a promotion is announced to the promoted person; the
  secret is stable until rotated; a token is never stored) each name one test
  whose assertion is the sentence; no overlap found.

**D14. Commits, in order.** C1: SPEC-009 (D1). C2: the three tests, the
three rows' evidence, the debt entry's "Closed by", and under form A the
withdrawn row and the successor. C3: `JoinAllows` and its twins gone, the two
tests re-pointed, the one assertion removed, the sort of `identity.go`, the
pointer removals in `identity.go`, `identity_test.go`, `join.go`, `join_test.go`
and `server.go`, `Open`'s doc, the debt file's known-gap entry. C4: the nine
rows and their citations, SPEC-009's `Requirements` completed. C5: the
report. Forced by: every sentence true at every commit (SPEC-009 must exist
before a comment names it; a row's evidence changes in the commit that adds
its test); the recorder's whole-tree fingerprint; one reviewed diff per task;
and C4 after C3 because two of the nine rows cite tests that C3 re-points, so
no row ever cites a test on a function nobody calls.

**D15. `task check` whole runs once, over the tree carrying C1 to C3, before
C3 is committed; C4 and C5 run the steps their files can reach.** Forced by
measurement: C3 is the last commit that changes a `.go` file that is not a
test, and the tiers, the coverage floor and the mutation gate read that tree;
C4 adds comment lines to test files and rows to the register
(`check:requirements-chain`, `check:new-prose`, `go test` on the two
packages); C5 is a report. C1 and C2 run their reachable steps (C2: `go test`
on both packages, the mutation self-tests, `check:new-prose`,
`check:doc-owner`, `check:requirements-chain`, `check:drift` after the proto
break is reverted). The whole gate is over an hour. A question at the end.

**D16. The dispenser writes at the start of the task whose commit carries
the row, after sign-off.** Under form A, the successor of VTT-035 at the
start of Task 2; the nine at the start of Task 4. Same reason as the first
half's D10: the recorder's fingerprint, and one reviewed diff per task.

---

## Per-block sort of `internal/identity/identity.go`

Named by the symbol each block sits on. STAYS is what the code cannot say
about itself; GOES is history, reasons and measurements, which the report of
2026-08-09 already carries and SPEC-009 carries as present-tense decisions
where they are decisions. "→ SPEC-009" marks a pointer the rewritten comment
may keep in place of the reason.

| Block (on) | STAYS at the line | GOES |
|---|---|---|
| `package identity` (package doc) | Own SQLite handle on the campaign file; deliberately not event-sourced; revocation is not undone by replaying the log. | `(spec §5)` → SPEC-009; "such as retraction" (retraction left the platform 2026-08-30; memory: retraction is out). |
| `schema`, Go comment above the const (the `controls` column) | There is no `controls` column and no statement in this package names one; a campaign file still carrying it must open and the column is inert (`TestJoinIsClosedOnAnExistingCampaign`'s fixture carries it); do NOT add a migration to drop it: the shape check makes `migrationPending` answer yes, which takes `migrate` to `BEGIN IMMEDIATE`, which read-only media cannot give. Control is `Actor.controller_ids` in the log, read by `gateway/authz.go`. | Both dates; "It used to sit between..."; the `/api/me` and "Hollis" account; "shipped on this branch and was removed the same day"; "no campaign is in use by anyone". |
| `schema`, SQL comment on `join_access` | A separate TABLE because `CREATE TABLE IF NOT EXISTS` is a no-op on an existing table, so a new COLUMN never reaches an existing campaign; `id = 1` plus the `CHECK` make it one row by construction; closed-by-default is carried by `ensureJoinRow`'s explicit `open = 0`, and this `DEFAULT 0` is not the guard. | `(joining-a-table spec §2, §4)` → SPEC-009; the injection story ("flipping the DEFAULT fails nothing, flipping the INSERT fails two tests", the named test) — the report's section 4 already corrects its numbers. |
| `schema`, SQL comment on `admitted`/`admit_limit` | `admitted` counts THIS opening; `admit_limit` is what the DM allowed; both reset when the door opens; these are COLUMNS on an existing table, which this schema cannot deliver — see `migrate`. | `(spec §2, amended 2026-08-11)`; "a DM who opens the door twice means twice" (reason → SPEC-009). |
| `migrate` (doc) | Runs on every `Open` and must stay idempotent — `ALTER TABLE ADD COLUMN` errors on a column already present; the budget lives on the same single row as `open` so that spending is ONE conditional `UPDATE` against ONE row; touches only `join_access`. | "which the join_access comment records as the reason..."; the amendment date; "the branch that dropped participants.controls was removed on 2026-08-24. See the schema comment for why..." |
| `migrate` (body, read-first) | Read first; take no write lock when there is nothing to do: this runs on every `Open`, the file is shared with `internal/store`'s per-append transaction, and an unconditional `BEGIN IMMEDIATE` takes the write lock on a read-only user's open (`vtt state dump`, the console's polling) and makes a campaign on read-only media impossible to open. | "the same discipline ensureJoinRow documents, for the same measured reason"; past-tense narrative → present-tense warning. |
| `migrate` (body, pinned connection) | The scan and the `ALTER`s are separate statements, so two processes opening the same campaign at once both see the columns missing; `BEGIN IMMEDIATE`, never `db.Begin()`: DEFERRED takes a read lock and must upgrade it, and `busy_timeout` does not retry an upgrade; losers wait and re-read inside the transaction. | The trial counts (35 of 40, 36, 0 of 40); "The server refuses to start or the DM gets a raw SQL error..." |
| `migrationPending`, `shapeReader`, `columnNames` body (PRAGMA column order) | Unchanged. | — |
| `tableShape` (doc) | Pragma and label travel as ONE value so a call site cannot mislabel the one message an operator gets; the pragma is a LITERAL because SQLite will not bind a parameter in a `PRAGMA`; both callers must name the table identically. | "ONE SHAPE since 2026-08-24: participantsShape went with the controls migration. Kept as a value rather than folded back..." |
| `columnNames` (doc) | One implementation for both call sites (before the lock and under it); the rows handle is closed by `defer`, once, and `migrateLocked` must close it BEFORE its `ALTER TABLE` (SQLite will not alter a table with an open cursor); `r` is `*sql.DB` or `*sql.Conn` and `shapeReader` is the narrowest interface serving both. | The gosec sentence about inline `rows.Close()` (a lint history); "deliberately the narrowest thing that does" trimmed to the constraint. |
| `migrateLocked` (body, re-read) | Re-read inside the transaction: a concurrent opener may have completed the migration while this one waited; wrap `identity: migrate:` here, which `migrationPending`'s identical read does not, or an operator cannot tell which failed; this error arm is unreachable through `testdb` — `docs/verification-debt.md` carries the gap (D10a). | "Measured, not assumed — this arm read `1 0` in the coverage profile at HEAD on 2026-08-24"; "The only under-lock shape read a fault ever reached was the participants one"; "Said here so it reads as a known hole rather than an unexplained gap in a report"; "It also matches how every other failure..." |
| `migrateLocked` (body, door repair) | A door left open keeps working on a fresh budget: the columns arrive at 0 and 0 admits nobody; keyed on the STATE (`open = 1 AND admit_limit = 0`), never on which `ALTER` ran — a database carrying one column and not the other would open clean, keep a budget of 0 and refuse every joiner at a door reading open; the predicate cannot match a legitimate row because `SetJoinOpen` coerces every budget to at least 1. | "Nesting it under the admit_limit branch made it depend on this function's own statement order" — rephrased as the warning it already is. |
| `Role`, `ParseRole`, `Participant`, `ErrInvalidToken`, `DB` | Unchanged (`Participant`'s "(see schema's note)" points at a block that stays). | — |
| `Open` (D11: its own doc, above `func Open`) | Independent handle from `store.Open`; both may be open on the same campaign file at once. Body: `busy_timeout(5000)`, same hardening as `internal/store/store.go`'s `Open`. | "(P6 Task 4 review)" — a pointer of the `.superpowers/` shape (rule 8). |
| `driverName` (doc, above the var) | `"sqlite"` in production; a variable only so `internal/testdb` can substitute a fault-injecting wrapper; set only by `fault_internal_test.go` and restored by it; closing a handle is not a substitute, because it fails the FIRST statement and leaves every later error arm unreached. | "in the same spirit as gateway's encodeFrame field and the presence registry's sendBudget" (a cross-reference that states no constraint). |
| `JoinOpen` (doc) | FALSE on any error, deliberately: it gates an unauthenticated, row-minting endpoint. | Unchanged otherwise. |
| `SetJoinOpen` (doc and body) | `admitLimit` is ignored when closing; opening RESETS the count; `admitted` resets on EVERY call, including a close; a non-positive `admitLimit` becomes `DefaultAdmitLimit` because protojson omits zero values, so an absent field and a deliberate 0 are the same bytes; ONE upsert, atomic, cannot leave the row half-made. | "the second opening is a decision about a fresh set of people..."; "Carrying the count over would let a campaign run out of admissions permanently, curable only by editing the database"; "silently opening a door that admits no one is the one nobody can debug" (→ SPEC-009 Consequences); "a shape that also had an error branch no test could reach" (history). |
| `JoinBudget` (doc) | Read-only, mints nothing — the console polls it; both zero on an untouched campaign and on a shut one. | "JoinSecret's own comment records what happens when a poll takes SQLite's write lock" (cross-reference to a block that goes). |
| `JoinSecret` (doc) | Stable until rotated: the DM shares it. | — |
| `JoinAllows` (doc and func) | NOTHING — the function goes (Task 3). Its one sentence the code cannot say, the empty-secret warning, moves to `JoinAdmits` (D10b). | All of it. |
| `DefaultAdmitLimit` (doc) | A number, not "unlimited", because protojson omits zero values. | "Sized for a table — four to six players and a DM..." (reason → SPEC-009). |
| `JoinAdmits` (doc) | The secret is compared HERE, in Go, in constant time, and never leaves this package; every refusal is decided here from the same read (wrong secret, shut door, spent budget, EMPTY STORED SECRET — `ConstantTimeCompare` of two empty strings is a match and an omitted field decodes to `""`), so a refused request writes NOTHING: an `UPDATE` matching zero rows still takes SQLite's write lock on the file `internal/store` appends to; the increment re-states door and budget in its `WHERE`, the read is a fast path; the secret is NOT re-checked in the `UPDATE`, so a rotation between the `SELECT` and the `UPDATE` admits one in-flight holder of the old secret — accepted; a `CreateInvite` failure after `true` BURNS the slot; do not add a compensating decrement. | "which spec §2 rests on" → SPEC-009; "A microsecond window on a deliberate, rare, DM-authorized action... trades a timing side channel for a race nobody can reach on purpose" and "Losing one admission... costs the cap its meaning" (the arguments → SPEC-009 Consequences; the warnings stay). |
| `RotateJoinSecret` (doc and body) | Closes a leaked link to newcomers and touches nobody already through it; the `DO UPDATE` branch must NOT touch `open`; the `INSERT` branch writes 0, which is not an exception (no row already means closed); `admitted` RESETS — a new secret is a new opening; `admit_limit` is NOT touched, or rotating becomes a second way to set a budget. | `(spec §2)`; "Without this, rotating a leaked link after its budget ran out hands the DM a door that reads OPEN and admits nobody — every legitimate player gets the byte-identical stranger's 403... the cure was worse than the leak" (report section 4). |
| `ensureJoinRow` (doc and body) | Read first: the upsert is a genuine write even on the conflict path, this file is shared with `internal/store`, and the console polls this; the upsert's `SET secret = secret` is a no-op whose only job is to make `RETURNING` fire on the conflict path (`INSERT OR IGNORE` returns no row there). | "Measured by review: with another handle holding a write txn, this blocked for the full busy_timeout(5000)..."; "so using it unconditionally made every 'show me the link' take the write lock" → present tense. |
| `newSecret` (doc) | 32 `crypto/rand` bytes, base64url, the same shape and strength as an invite token; no error return because `crypto/rand.Read` never returns one. | "Carrying an error here meant three propagation branches no test could ever reach, which the coverage ratchet correctly refused to accept as tested. (CreateInvite above still carries the older shape; changing it is not this task's business.)" — this is the line the ticket's grep matches through `refused to` (gap 3). |
| `SetRole` (doc and body) | Role lives in `participants.role` beside the token, never in the log — the fold has no `Role`; changes ONLY the role: token, id and display name survive (a promotion that rewrote the credential would log them out); characters live in the log, which `SetRole` cannot reach; a revoked participant stays revoked; `n == 0` is reported, not swallowed. | `(joining-a-table spec §3.1)` → SPEC-009; "What a second source of truth costs is on the record — controller_id mirroring controller_ids needed an invariant, fault-injection proof on both folds and a golden scenario". |
| `Close` | Unchanged. | — |
| `CreateInvite` (doc) | The token is returned exactly ONCE; only its SHA-256 hash is persisted; takes NO list of actors — control is an `ActorControlGranted` in the log. | "(2026-08-24)"; "The parameter it used to take was recorded and never read by anything that decides." |
| `Verify` (doc) | `WHERE token_hash = ?` is a plain indexed equality and safe although SQLite's comparison is not constant-time: the hash is not timing-sensitive (matching it means the token is already known); the confirmation still uses `subtle.ConstantTimeCompare` to make the contract explicit (`TestVerifyUsesConstantTimeCompare` holds it). | "DESIGN NOTE (task-3-brief.md Step 2, binding)". |
| `Lookup` (doc; body arm unchanged) | Resolves a participant as they are NOW; `Verify` is the connection-time half; the gateway re-resolves through here on every command, delivery and presence frame; a revoked participant does not resolve, and revoked and unknown share one error, as in `Verify`. | "Two things that used to require a reconnect now take effect...The second was a real hole..."; `(spec §3.2)` → SPEC-009; "MEASURED before adopting... 15.5µs... Microseconds against milliseconds." |
| `List` (doc; body arm unchanged) | REVOKED PARTICIPANTS ARE OMITTED: they cannot connect or act, and listing them would offer promote controls for people who are gone; ordered in SQL by `display_name` then `id`, a total order, so two consumers cannot disagree; a role must NOT be folded into a presence frame — presence is connection-scoped, a role is campaign-scoped. | "It exists because the DM console has to answer..."; `(spec §3.2)`, `(spec §3.1)` → SPEC-009; "a console does not reshuffle under the DM's cursor". |
| `Revoke` (doc) | A direct table mutation, not a logged event; not undone by replaying the log. | `(spec §5)`; "game-log/retraction mechanism". |

Outside `identity.go`, the ticket's named edits and nothing more:

- `identity_test.go`: the two twins deleted; the two tests re-pointed (their
  comments unchanged); the `JoinAllows` assertion removed; the comment above
  `TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath` loses "the twin of
  the JoinAllows test three functions up... Review deleted the guard... every
  suite stayed green" and keeps the `ConstantTimeCompare` warning; the comment
  above `TestVerifyUsesConstantTimeCompare` loses "(task-3-brief.md, binding
  DESIGN NOTE)" and keeps the rest.
- `join.go`: "See identity.JoinAllows." becomes "See identity.JoinAdmits.";
  "JoinAdmits, not JoinAllows: it SPENDS..." becomes "JoinAdmits SPENDS...";
  the sentence "why this is not the two calls it used to be" in the same
  block is NOT touched — the ticket names the `JoinAllows` lines and item 4's
  grep is over `identity.go` (gap 5).
- `join_test.go`: the two comments reworded so neither names a function that
  no longer exists, keeping their point (a read-only check in identity can
  still leave the handler minting; a check that answers without spending
  would leave the cap perfect in identity and absent from the product).
- `server.go`: the "Measured at 15.5µs..." sentence and its blank comment line
  go from the block above `s.ids.Lookup(p.ID)`; the rest of the block stays
  (it is the warning "THIS IS HALF OF IT").

---

## Tasks, in dependency order

Every task ends with what must be true, by command or by a reading, and names
the files it touches.

### Task 0 — Measure before editing

**Files:** none.

**Do:**
- `git status --short` prints the ticket and this plan and nothing else;
  `git log --oneline -1` prints `8ebb2b1`.
- `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...` green.
- `go test -cover ./internal/identity/` — record the percentage (the
  before-figure for the report; the floor is 92.1).
- `python3 tools/check_mutation_test.py -q` and
  `python3 tools/check_ts_mutation_test.py -q` pass.
- `grep -rn 'JoinAllows' internal/ | wc -l` prints 14; the ticket's item 3 and
  item 4 greps print 4 and 17.
- `grep -cE '^\s*//' internal/identity/identity.go` and `grep -cE '^\s*--'`
  print 400 and 26 (the before-count for the report).
- `df -h "$TMPDIR"` shows at least 16 GiB free, or `go clean -cache` first.
- On a scratchpad copy of `docs/requirements.md`, `requirement-id --register
  <copy> "<any sentence>"` allocates `VTT-038` and leaves no `.lock`.

**True when done:** all hold, recorded in the report as the starting state.

### Task 1 — SPEC-009, commit C1

**Files:** new `docs/specifications/009-identity-and-joining.md`.

**Do:**
1. Per the `specification` skill: open every file in D8's table and the tests
   the register's rows VTT-005 to VTT-037 name; write the five headings in
   order; Status per D8; `How it works` per D8's derivation, in the present
   tense, each paragraph checkable against a file named in it; `Consequences`:
   what is refused (a refusal writes nothing; the three refusals are one
   answer; promotion cannot reach dm or agent or unmake one), what is skipped
   (no rate limiting, and the condition under which that decision is void),
   what must exist elsewhere (control of a character is the log's; SPEC-007
   holds the wire-level facts); `Principles served` as SPEC-008's line;
   `Requirements`: VTT-005 to VTT-037, less VTT-035 under form A.
2. Two passes against `catches.md`. In particular: no `Why` heading, no
   `Rejected`, no past tense that is not about the code, no measurement (the
   default budget is named `identity.DefaultAdmitLimit`, never its value), no
   line number, no path outside the project, every "only/never/every" with
   its search named, `perceive` and `eyes()` absent.
3. `task check:requirements-chain` green (3 specifications scanned);
   `task check:new-prose`.
4. Phase 4b: one reviewer; every sentence verified BY COMMAND against the
   file it names; `review-record.sh --summary`; commit.

**True when done:** `docs/specifications/009-identity-and-joining.md` exists
with the five headings; `grep -c 'perceive\|eyes()' docs/specifications/009-*.md`
prints 0; the chain gate reports 3 specifications; `git status --short` prints
the ticket and this plan only.

### Task 2 — The three tests, red on the break, commit C2

**Files:** `internal/identity/fault_internal_test.go`,
`internal/gateway/server_test.go`, `internal/gateway/authz_test.go`,
`docs/requirements.md`, `docs/verification-debt.md`; under form A also the
register's VTT-035 row and its successor.

**Do, in this order:**
1. Under form A: from the repository root, `requirement-id "No event payload
   names a role, and the fold's state holds none."` once; mark VTT-035 per
   D2; `ls docs/requirements.md.lock` fails.
2. Write `TestAShutDoorRefusesWithoutTouchingTheDatabase` per D6, citing
   `VTT-008`. Run: GREEN on today's tree (the term is present). BREAK: remove
   `open != 1` from `JoinAdmits`'s guard — RED, the fault reached; revert;
   GREEN. Fixture check: with the same edit, `TestAClosedDoorSpendsNothing`
   stays green (the debt entry's claim, re-observed).
3. Write `TestAPromotionAppendsNoEvent` per D5, citing `VTT-036`. GREEN.
   BREAK per D5 — RED; revert; GREEN.
4. Write `TestNoEventPayloadNamesARole` per D4, citing `VTT-035` or its
   successor. GREEN. BREAK per D4 — RED naming `narration_added.role`;
   revert proto and generated tree; `task check:drift` clean; GREEN.
5. Register: VTT-008's evidence `internal/identity/fault_internal_test.go#TestAShutDoorRefusesWithoutTouchingTheDatabase`;
   VTT-036's `internal/gateway/server_test.go#TestAPromotionAppendsNoEvent`;
   the role row's `internal/gateway/authz_test.go#TestNoEventPayloadNamesARole`.
   Under form A, SPEC-009's `Requirements` gains the successor id.
6. `docs/verification-debt.md`: the entry dated 2026-09-23 gains
   `**Closed by** TestAShutDoorRefusesWithoutTouchingTheDatabase, which reds on
   the recipe above.`; the Open-debt item about a test citing an OPEN row is
   unchanged (it is the gate's, not this arc's).
7. Phase 4a per D3; adjudicate; write the adjudications down for the report.
8. `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...`;
   `task check:requirements-chain`; `task check:new-prose`;
   `task check:doc-owner`; both mutation self-tests.
9. Phase 4b; `review-record.sh --summary`; commit with the Phase 3 line (three
   breaks, which check spoke for each).

**True when done:** Done items 5 and 6 by their commands; no row of the
register reads `OPEN`; `grep -rn 'VTT-008\|VTT-036' internal/` lists exactly
the two new tests; the gate names 3 specifications and every citation
resolves.

### Task 3 — `JoinAllows` gone, the pointers gone, the sort, commit C3

**Files:** `internal/identity/identity.go`, `internal/identity/identity_test.go`,
`internal/gateway/join.go`, `internal/gateway/join_test.go`,
`internal/gateway/server.go`, `docs/verification-debt.md`.

**Do, in this order:**
1. Delete `JoinAllows` and its doc; delete `TestAnEmptyStoredSecretAdmitsNobody`
   and `TestCheckingTheDoorMintsNothing`; re-point the two tests and remove
   the one assertion (D12). `go build ./...` and `go vet ./internal/identity/`
   clean; `go test ./internal/identity/` green. Coverage: `go test -cover
   ./internal/identity/` at or above 92.1, and the drop against Task 0's
   figure explained by `JoinAllows`'s statements alone (memory: a deleted
   test covers more than its name says — the two twins exercise only
   `JoinAllows` and shared helpers, which the profile confirms).
2. The two pointers: `Verify`'s doc and `TestVerifyUsesConstantTimeCompare`'s
   lose `task-3-brief.md`; `Lookup`'s doc and the block above
   `s.ids.Lookup(p.ID)` in `server.go` lose the `15.5µs` sentence. The
   ticket's item 3 grep prints nothing.
3. The sort, block by block per the table above, in file order; D10b moves
   the empty-secret sentence into `JoinAdmits`'s doc; D11 gives `Open` its
   own doc; D10a writes the known-gap entry in `docs/verification-debt.md`
   (`migrateLocked`'s re-read arm: labels `test data missing`, `outside the
   tool`; what holds the arm — nothing; why `testdb` cannot reach it — `Arm`
   is one-shot and substring-matched and `migrationPending` runs the same
   `PRAGMA` first). Every rewritten block opens with the same first word it
   had (`check:doc-owner`), wraps to the band (`check:new-prose`), and cites
   SPEC-009 by number where a reader would ask why.
4. `join.go`, `join_test.go`, `server.go` per the list above. The ticket's
   item 2 grep prints nothing.
5. The reading that item 4 asks for: every remaining block in `identity.go`
   read once against the rule — a constraint, a reason a line is unusual, a
   warning to whoever edits next — and the item's grep prints 0. Record the
   after-count of `//` and `--` lines for the report.
6. `gofmt -l internal/identity internal/gateway` names nothing this task
   touched (memory: two files elsewhere are already listed; leave them).
7. Both mutation self-tests; `go test -count=1 -p 1 ./internal/identity/...
   ./internal/gateway/...`; `task check:doc-owner`; `task check:new-prose`;
   `task check:requirements-chain`.
8. `task check` WHOLE, locally, in the background with `start_new_session`,
   over this tree (D15). Green. If the mutation gate lists a survivor in
   `internal/identity` that `JoinAllows`'s removal exposed, it is adjudicated
   or killed here, not by narrowing the gate (rule 2).
9. Phase 4b: one reviewer at high effort; the per-block table is the
   checklist — for each block, STAYS present, GOES absent, nothing invented;
   `review-record.sh --summary`; commit.

**True when done:** Done items 2, 3 and 4 by their commands (0, 0, 0 lines);
`go doc ./internal/identity Open` prints a description; the three tests item
8 names pass on `JoinAdmits`; `task check` whole green on this tree; Done
item 9's diff prints nothing.

### Task 4 — The nine rows, their citations, SPEC-009 complete, commit C4

**Files:** `docs/requirements.md`; `internal/identity/identity_test.go`,
`internal/identity/fault_internal_test.go`, `internal/gateway/server_test.go`;
`docs/specifications/009-identity-and-joining.md`.

**Do:**
1. Run the `requirements` skill's sort over the nine candidates with D13 in
   hand; report accepted and refused, one line each.
2. For each accepted rule, `requirement-id "<sentence>"` once, in the sort's
   order; set each evidence cell to its `path#Check` entries; add the bare id
   on the last comment line above each cited `func` (a check holding two
   rows carries both ids on that line, as the first half did).
3. SPEC-009's `Requirements` lists every id, copied from the register.
4. `task check:requirements-chain` green. BREAK: one citation changed to an
   id one above the highest row — red naming the file; revert. One evidence
   entry's check renamed — red naming the row; revert.
5. `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...`;
   `task check:new-prose`.
6. Phase 4b: each row's sentence checked against the test it names BY
   READING THE TEST; `review-record.sh --summary`; commit with the Phase 3
   line.

**True when done:** Done item 7 by the register (every accepted rule has a
row citing its test); Done item 1 whole (SPEC-009's `Requirements` names every
row of the arc and the gate is green with it); `git status --short` prints
nothing but the ticket and this plan if they are not yet committed.

### Task 5 — The report, then Phase 5, commit C5

**Files:** new `docs/reports/2026-09-24-joining-record-and-code.md`.

**Do:** per the `implementation-report` skill against the ticket's nine
items in the ticket's numbering, each with its observation; the comment-line
counts before and after (item 4); the sort's accepted and refused lists;
the QA adjudications under their own heading; the deliberate breaks of Tasks
2, 3 and 4; Task 0's starting state; the deviations (D1's order, D10a's debt
entry, D2's form, D11); what could not be established. Names the last code
commit (C4). Reading review; record; own commit. Then the Phase 5 step:
SPEC-009 is already the record; check that nothing else moved. Fetch, check
the branch against `origin/main`, push `chore/adopt-the-process`.

**True when done:** five commits above `8ebb2b1`; `git status --short` prints
nothing; the report answers items 1 to 9.

---

## Gaps that travel with this plan

1. VTT-035 is false as worded (D2); Done item 6's first half cannot be met
   without Patrik's choice of form. The plan carries both.
2. The ticket counts "nine existing files and one new"; its own list names
   ten existing (`fault_internal_test.go`, `server_test.go`, `authz_test.go`,
   `identity.go`, `identity_test.go`, `join.go`, `join_test.go`, `server.go`,
   `requirements.md`, `verification-debt.md`) and one new, eleven in all,
   which crosses the `ticket` skill's file threshold as well as its component
   threshold. The count is the writer's to fix; the scope is unchanged.
3. Done item 4's grep matches `refused to` in `newSecret`'s doc through its
   `used to ` alternative, so 17 includes one line that is not the phrase it
   hunts; the line goes anyway (history), so the item still reaches 0. Any
   future comment containing "refused to" trips it; the grep is an
   observation, not a gate.
4. `(P6 Task 4 review)` in `Open`'s body is a pointer of the shape the ticket
   removes and its grep does not hunt; the sort removes it under item 4's
   reading.
5. `join.go`'s block above `JoinAdmits` says "the two calls it used to be";
   the ticket's item 2 names only the `JoinAllows` lines and item 4 is over
   `identity.go`. Left as found, named here.
6. `docs/verification-debt.md` gains a known-gap entry the ticket does not
   list (D10a); the ticket names the file, not the entry.
7. Whether `task check` whole is green at `8ebb2b1` was not established here
   (over an hour); the first half's report says it was green at `fcf4ea3`,
   and `8ebb2b1` applied review findings to that report.
8. The ticket's "three pointers" are two citations in four places; Done item
   3's "four lines in three files" is the accurate count.
9. The withdrawn-row form (D2, form A) is a stopgap the process does not
   define.

## Questions for sign-off

One line each; the plan proceeds on the defaults named.

- VTT-035: withdraw and re-dispense (form A), or keep the sentence and let
  SPEC-009 carry the vocabulary (form B)? Recommendation: A.
- SPEC-009 first (D1), or last as the ticket orders? Recommendation: first;
  the sort's comments and QA both need it to exist.
- ADR-011: no edit (D7 form i), or a one-line pointer with the matching
  amendment to `CLAUDE.md`'s ADR paragraph (form ii)? Recommendation: i.
- One ticket in five commits, or a split at the seam "tests and rows" /
  "sort, `JoinAllows`, SPEC-009"? Recommendation: one ticket. The file count
  is at the threshold, the four areas are the two packages and the two record
  files every arc ticket touches, SPEC-009 must name the rows this ticket
  adds, and two of the nine rows cite tests the sort re-points, so a split
  either registers rows on a dead function's tests or writes SPEC-009 twice.
  If split anyway: sort, `JoinAllows` and SPEC-009 first, then the tests and
  rows, so no row ever cites a test on a function nobody calls.
- `task check` whole once, before C3 (D15)? Recommendation: yes.
- A process ticket to `~/dev/development_setup` for a withdrawn row's form
  (dispenser and gate)? Recommendation: yes, raised in Task 2's commit line
  and the report, developed there.
- The known-gap entry for `migrateLocked`'s re-read arm (D10a): in this
  ticket, or left as a comment and ticketed? Recommendation: this ticket; it
  is one paragraph in a file the ticket already touches.
