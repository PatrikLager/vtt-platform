# A display name belongs to one participant at a time: the change

**Ticket:** `docs/superpowers/specs/2026-10-31-a-name-belongs-to-one-participant-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G8.
**Plan:** `docs/superpowers/plans/2026-10-31-a-name-belongs-to-one-participant.md`,
written by that verifier from probes in copies of the tree outside the
repository, and amended by Task 0's review.
**Where it came from:** a finding of
`docs/reports/2026-10-26-a-player-speaks-as-their-character.md`: two
participants may join under one display name, and nothing in an out of
character line tells them apart. The owner chose it as the next ticket on
2026-10-31: "Kör två deltagare".
**The owner's rulings, all 2026-10-31:**
- A join through the share link under a name another participant who is
  not revoked holds is refused, "choose another": compared ignoring case,
  DM and agent included, and only once the link is accepted. Rejected: a
  suffix added by the server, duplicates tagged in the console, and a
  refusal only while the other is connected.
- `vtt invite --name` is held to the same rule. Duplicates already in a
  campaign stay.
- The plan signed off as one ticket on one branch: "Ja, kör som ett
  ärende".

**The last code commit is `1fb5a9a`.** Every code reference below is to
that tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `90cddc6` | Task 0: the ticket, the plan, the new rows and six withdrawn |
| `d5e9b76` | Task 1: the rule in `internal/identity`, held by `vtt invite` |
| `e54e1de` | Task 2: the join's 400 for a taken name |
| `ceb73cd` | Task 3: the client's tests and comments; VTT-547 withdrawn |
| `1fb5a9a` | Task 4: the browser beat; the last code commit |
| `eb04a87` | the comment ledger, written once by the tool |

## What the code does now

- **`nameHeld` in `internal/identity/identity.go`** reads `SELECT
  display_name FROM participants WHERE revoked = 0` and answers
  `ErrNameTaken` when one of them equals the name under
  `strings.EqualFold`. The compare is in Go because SQLite's `lower()` and
  `NOCASE` fold ASCII alone: `'Åsa' = 'åsa' COLLATE NOCASE` is 0 on
  `modernc.org/sqlite`, measured by the verifier and again by Task 1's
  reviewer.
- **`JoinAdmits(candidate, name)`** calls it after every refusal the door's
  one read decides and before the `UPDATE` that spends the admission. A
  wrong secret, a shut door and a spent budget therefore still answer
  `(false, nil)` without reading the names, and a held name answers
  `(false, ErrNameTaken)` having written nothing.
- **`CreateInvite`** calls it after `ParseRole` and before it inserts, so
  every path that mints a participant is held to it: `handleJoin`,
  `newInviteCmd` and `mintInvites`. `vtt invite` returns the error before
  it prints anything, and exits 1.
- **`handleJoin` in `internal/gateway/join.go`** answers `ErrNameTaken`
  with 400, "gateway: that name is already taken at this table; choose
  another", ahead of the door's 403, and every other error from
  `JoinAdmits` with the door's 403 as before. It trims the name before
  anything judges or stores it.
- **The client** needed no new behaviour. `requestJoin` already read any
  400's body as the name's refusal, and `runJoin` never emptied the box.

## Done looks like, answered

1. [x] **A join under a held name is refused, and nothing is minted.**
   - `TestAJoinUnderANameSomebodyHoldsIsToldTheNameIsTaken` in
     `internal/gateway/join_test.go` posts `kim`, `  KIM `, `åsa` and `dm`
     while `Kim`, `Åsa` and a DM named `DM` are held. Each gets 400, the
     participant count does not move, and `Kimberly` is admitted.
   - `TestAJoinUnderANameSomebodyHoldsIsRefusedWhateverItsCase` in
     `internal/identity/identity_test.go` holds it at `JoinAdmits`, with an
     agent among the holders.
   - It failed on the tree it was written against. The identity test was
     red until Task 1's code. The gateway test was red on `d5e9b76`, where
     a held name got the door's 403. On `690ff7e`, by the verifier's probe
     P1, joins as `kim`, `åsa` and `dm` were admitted with 200.
2. [x] **The joiner reads that the name is taken, and keeps what they
   typed.**
   - `client/test/app.test.ts`'s "a join refused for a taken name says so,
     and keeps the name in the box" boots on `?join=`, answers 400 with the
     server's body, and reads the exact text, `Kim` still in the box, no
     credential stored, no socket dialled and Join enabled.
   - In a real browser, against the built binary, beat 2b of
     `client/e2e/joining.spec.ts`'s "a stranger with one link joins,
     watches, is promoted, and plays" joins as `robin` while Robin is
     connected, and reads the exact text in the form, `robin` still in the
     box, and one robin on the server's participant list. A run writes its
     screenshot, `j4b-second-robin-told-the-name-is-taken.png`, to the
     uncommitted `client/e2e/.artifacts/`.
   - This half held before the ticket, as the verifier found (G4). It can
     fail on the old tree only end to end, where the join is admitted.
3. [x] **A refusal for a taken name spends no admission.**
   `TestAJoinRefusedForATakenNameSpendsNoAdmission` opens a door with a
   budget of 1, has `kim` and `  KIM ` refused while `JoinBudget` reads 0,
   then admits `Ann` and refuses `Bo` at the door.
   `TestATakenNameAtTheDoorReachesNoWrite` in
   `internal/identity/fault_internal_test.go` goes further, with every
   write verb armed in turn and SQLite's `data_version` watched: nothing
   is written.
4. [x] **A held name tells a stranger nothing.**
   `TestATakenNameGetsTheDoorsOneAnswerWhenTheDoorRefuses` posts `kim`
   while `Kim` is held in three cases: with a wrong secret, at a shut door
   with the current secret, and against a spent budget. Each gets the
   status and raw body a free name gets with a wrong secret.
   `TestADoorThatRefusesNeverReadsTheNames` shows the names
   are not read in those three cases, with an armed read as the control
   that is reached at an open door.
5. [x] **A name held only by a revoked participant is free.**
   `TestAJoinTakesANameOnlyARevokedParticipantHeld`,
   `TestANameOnlyARevokedParticipantHeldIsFree`, and
   `cmd/vtt/cli_test.go`'s `TestInviteTakesANameOnlyARevokedParticipantHeld`.
6. [x] **`vtt invite` refuses a held name.**
   `TestInviteRefusesANameAnUnrevokedParticipantHolds` runs `invite --name
   kim --role dm` while `Kim` is held, and `åsa` while `Åsa` is. Each is
   an error that `errors.Is` `ErrNameTaken`, prints no token, and leaves
   the participant list as it was. It was red before Task 1's code.
7. [x] **A free name still joins and still invites.** The existing tests of
   `internal/gateway/join_test.go`, `cmd/vtt` and
   `client/test/join.test.ts` pass, each run whole after its task, and in
   the gate (Figures).
8. [x] **VTT-072 still names a passing test.**
   `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder` now writes its four
   `Kim` rows with raw SQL, as a campaign from before the rule, with ids
   inserted out of order (`c3`, `a1`, `d4`, `b2`). A lost id tie-break can
   no longer pass by random ids landing sorted.
9. [x] **`task check` exits 0 on the branch.** It ran whole on `eb04a87`,
   the ledger's commit, whose parent `1fb5a9a` is the last code commit,
   and exited 0 (Figures).

## What the rules became

| Ticket's rule | Became |
|---|---|
| No two participants who are not revoked hold one display name, by either path | Refused as worded: false in every campaign that already holds two, which the owner's ruling keeps (G1). Its two refusals are VTT-542 (the join) and VTT-543 (`vtt invite`), and VTT-544 frees a revoked participant's name |
| A refusal for a taken name spends no admission | VTT-545, strengthened to "writes nothing", which is what SPEC-009's lack of rate limiting on `POST /join` stands on (D12) |
| A taken name is told so only when the door would admit | VTT-009 as already worded: no new row (D13) |
| A refusal for a taken name says so, distinctly from the door | VTT-546 on the server; VTT-556 and VTT-548 on the client |

Task 0's review added VTT-555, which says the join trims the name before
it is judged or stored, because otherwise no row said which name is
compared. Six rows that a held name makes false as worded were withdrawn
and succeeded:

| Withdrawn | Successor | Found by |
|---|---|---|
| VTT-012 | VTT-549 | the verifier |
| VTT-019 | VTT-550 | the verifier |
| VTT-023 | VTT-554 | Task 0's reading review |
| VTT-024 | VTT-551 | the verifier |
| VTT-042 | VTT-552 | the verifier |
| VTT-044 | VTT-553 | the verifier |

VTT-547, allocated in Task 0 for the client, was withdrawn at Task 3's
Phase 4a and succeeded by VTT-556 (QA finding 3.1).

## Phase 4a

Tasks 1, 2 and 3 each had one QA agent. Its dispatch named SPEC-009 as the
governing spec and handed it the owner's rulings, the rows and the exported
surface; never the source, the existing tests or a diff. Task 4 had none:
it adds a test of behaviour Tasks 2 and 3 delivered.

| Task | QA's tests | Kept | Failures |
|---|---|---|---|
| 1 | 17 | 15 | none |
| 2 | 12 | 12 | none |
| 3 | 12 | 12 | one |

Task 1's commit message says 18; it was 17. The tree holds 15 of Task 1's
(12 in `internal/identity/qa_names_test.go`, 2 in
`cmd/vtt/qa_invite_names_test.go`, 1 appended to
`internal/identity/qa_joining_internal_test.go`), and review removed 2.

### QA adjudications

- QA finding 1.1: none, every test passed. At review two were removed,
  `TestQANamesAreNotTrimmedNorNormalised` and
  `TestQAInviteDoesNotTrimOrNormalise`: they admitted `" Kim"`, a
  composed `Å` against its decomposed form, `SS` and `İ` under rows that
  say only what is refused, and the owner's ruling of 2026-10-21 gives a
  near-theoretical case no test.
- QA finding 1.2: spec gap — duplicates already in a campaign stay — not
  a rule, since nothing observes an absent migration; the sort refused it
  as a row.
- QA finding 1.3: needs an id — the DM and the agent count as holders —
  held by VTT-542's "a participant who is not revoked", any role.
- QA finding 1.4: needs an id — the names are read only past the door —
  VTT-009 holds what a caller can observe (D13); the order is how.
- QA finding 1.5: needs an id — identity neither trims nor normalises —
  no row, by the ruling of 2026-10-21 (D5).
- QA finding 1.6: spec gap — VTT-072 says nothing of collation for mixed
  case — existing, not this ticket's.
- QA finding 2.1: none, every test passed. At review VTT-010 came off two
  tests that read only the reply's role, which `handleJoin` writes as a
  constant, VTT-009 off two that compared the status alone, and the door
  test gained a check that the name's 400 says "taken".
- QA finding 2.2: spec gap — the taken body's wording — by design (D7):
  VTT-546 holds that it says "taken" and is not the door's.
- QA finding 2.3: spec gap — "64 characters" read as runes — existing,
  `maxDisplayNameRunes`, VTT-549's bound; not this ticket's.
- QA finding 2.4: spec gap — VTT-544's `vtt invite` half is not reachable
  through `POST /join` — held by Task 1's
  `TestInviteTakesANameOnlyARevokedParticipantHeld` and QA's
  `TestQAInviteReusesARevokedName`.
- QA finding 2.5: needs its own row — the names are consulted only past
  the door — the sort refused it as how; VTT-009 holds the observable
  (D13).
- QA finding 3.1: spec ambiguity — a 400 whose body is only `gateway: `
  is shown as `gateway:`, where VTT-547's "a fixed refusal of the name
  when nothing remains" can be read to require the fixed text — the server
  never sends such a body, so no code (ruling of 2026-10-21) — VTT-547
  withdrawn, succeeded by VTT-556, which says the body is trimmed and then
  loses a leading `gateway: `, with the fixed refusal only when the
  trimmed body is empty; the bare-prefix assertion removed.
- QA finding 3.2: needs an id — a refused join stores no credential,
  dials no socket and frees Join — existing behaviour, held by
  `app.test.ts`'s "a refused join leaves the form up, says why, and stores
  nothing"; not this ticket's rule.
- QA finding 3.3: needs an id — a 403 shows the door's text, not the taken
  one — VTT-013's client side; no row of this ticket's.

## Phase 4b

One reviewer for each commit of Tasks 0 to 4, each given the diff and the
plan. The ledger's commit, `eb04a87`, is the tool's output; its diff was
read before it was committed: every row lowered or new, none raised.

| Commit | Must | Should | Nit |
|---|---|---|---|
| `90cddc6` | 1 | 6 | 6 |
| `d5e9b76` | 0 | 4 | 4 |
| `e54e1de` | 3 | 3 | 4 |
| `ceb73cd` | 0 | 4 | 4, though its message gives no count |
| `1fb5a9a` | 0 | 2 | 2 |

Every finding was taken or adjudicated. The ones that changed what the
work says:
- VTT-551 as first allocated was false of a third participant's name and
  of an empty one; it was reworded before its commit.
- The plan's D7 said a holder of the link could join under a free name
  and read every held name in the presence strip. The strip draws only
  who is present. The taken 400 tells a caller holding the current link,
  at an open door with budget left, whether any name is held, an absent
  participant's, the DM's and the agent's included, and they can ask as
  often as they like, since a refusal spends nothing. A participant who
  is connected also learns the name of anyone who departs, from the
  `PresenceChanged` frame SPEC-009 sends every connection.
- `join.go`'s comment "Refuse the name first, and distinctly: it says
  nothing about the door" became true only of the unusable name, and now
  says so.
- The client comments that said `CreateInvite` "does not validate the
  name", then "does not refuse an empty name", were each half true: a
  second empty name is now refused. They say it does not refuse a name
  for being empty.
- No test cited for VTT-556 observed "a fixed refusal of the name" until
  QA's empty-body test also compared it with the door's text.
- Beat 2b's count of the DM's roster could not see a second participant:
  the roster is re-read on a connect, a presence frame or the DM's own
  sends, and a refused joiner causes none. It reads the server's
  participant list instead.

The first commit of Task 2 was stopped by the hook's lint:
`usestdlibvars` in QA's file, `resp.StatusCode == 200`.

## The deliberate breaks

Each was applied, its tests run, and undone by the inverse edit, the file
then compared with a copy of the green one.

| Break | Went red |
|---|---|
| K1: `EqualFold` made an exact compare | the case tests in identity, the join and `vtt invite` |
| K2: the compare moved into SQL, `COLLATE NOCASE` | the `åsa` halves alone, and the test of a name that cannot be read |
| K3: the read loses `revoked = 0` | the revoked-holder tests |
| K4: the read moved above the door's refusals | `TestADoorThatRefusesNeverReadsTheNames` |
| K5: the read moved after the `UPDATE` | the no-write test and the failed-read test |
| K6: `CreateInvite`'s call removed | its tests and `vtt invite`'s; the join's identity test stayed green |
| K7: `JoinAdmits`'s call removed | the join's tests; the gateway's then answered 500, `CreateInvite`'s own check |
| K8: the read's query error taken as "not held" | `TestANamesReadThatFailsAdmitsAndMintsNobody` |
| K9: the read narrowed to spectators | the `dm` and `AGENT` cases |
| K10: the 400 branch removed | the gateway's two taken-name tests, with the door's 403 |
| K11: the branch answering with the unusable name's body | the gateway's taken-name test, and QA's door test once it checked "taken" |
| K12: `JoinAdmits` given the untrimmed name | the taken-name and budget tests on `  KIM `, 500 after the spend |
| K15: `CreateInvite` given the untrimmed name | `TestAJoinStoresItsNameTrimmed` |
| K13: `runJoin` empties the box on a refusal | `app.test.ts`'s "a join refused for a taken name says so, and keeps the name in the box", and the existing test that keeps the joiner typing |
| K14: `requestJoin` reads a 400 as the door's | that same test, and the existing prefix test |
| An empty 400 body shown as the door's text | QA's empty-body test |
| The 400 branch removed, in the browser | beat 2b, the form showing the door's text |
| The 400 branch also minting `robin ` | beat 2b, two robins on the server's list |

Twice the break script stopped with a file still broken, because the text
it was undoing occurred twice: `join.go` after K11, whose body also stands
in the unusable name's branch, and `join.ts` after K14, whose door answer
also stands in the 403 branch. Each was restored by hand with the
neighbouring line as context and compared equal to its green copy.

## Rule 9: how MapTool does this

`ServerHandshake.handle(ClientInitMsg)` in `~/dev/RPTool/maptool` at
`f4b7fef6c` asks `isPlayerConnected` before it checks the password, and
refuses with "That player name is already connected". It compares with
`String.equalsIgnoreCase`, one character at a time, and the client trims.
Borrowed: a taken name refused in its own words, and a compare that folds
case letter by letter, non-ASCII included. Not borrowed: "connected" as
the scope, by the owner's ruling, since here control survives a
disconnection and an absent participant still has a name on the console;
and the order, since here the door decides first. The two folds differ on
a few letters: Java folds `İ` with `i`, Go does not.

## The plan's gaps

- **G1:** the first rule is false as worded; it became two refusals.
- **G2:** five rows made false that the ticket did not name; Task 0's
  review found a sixth, VTT-023.
- **G3:** `JoinAdmits` took the name, and its 32 test calls changed.
- **G4:** the client half held before the ticket.
- **G5:** the comment above `joinSecretFrom` was cut, by the ruling of
  2026-10-30 that put it in the next ticket touching the join flow.
- **G6:** every existing Go file touched sat at its comment ceiling; each
  stayed under it, and the ledger recorded the drops. The new QA files
  had no ceiling until the ledger gave them one.
- **G7:** the ticket's open points were answered: the race and the names
  alike to the eye measured, the duplicates and the stale participant
  left by the owner's rulings (below).
- **G8:** `vtt invite` neither trims nor runs `usableDisplayName`;
  unchanged.

## The history cut from `client/src/join.ts`

The block above `joinSecretFrom` told how the reader came to be a
function. It was once pinned by reading `app.ts`'s source and asserting
the text `params.get("join")` appeared in it. Stryker rewrites the string
literals of every file it mutates, so that assertion failed in the dry run
and the client's mutation gate died before it tested a mutant. It had been
dead since 2026-08-11 and nothing said so. The block then said the gate
skips itself on macOS and that the container target was the only thing
that ran it unasked. Both have been false since SPEC-023 (`83d9661`):
`task check:ts-mutation:full` runs on macOS in its container, and nothing
runs Stryker unasked. Calling the function with a URL built from
`contract/testdata/join_url_format.json` survives instrumentation because
it never reads source, and it is the stronger assertion: the old one
passed on text alone, even in dead code. The comment now keeps only why
the reader is a function. The fixture's own comment named `app.ts` as the
reader and `join-url-format.test.ts` as the browser writer's test; it
names `joinSecretFrom` and `dm-view.test.ts`.

## Specifications

SPEC-009, `docs/specifications/009-identity-and-joining.md`, the one the
ticket named, is rewritten in its own commit after this report. One
sentence of it is false of a taken name today: "A refusal, and an identity
error, are answered with one 403 and one body." `ErrNameTaken` is an
identity error, answered with 400. It says nothing yet of a held name, and
its Requirements list still names the six rows withdrawn here.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Six withdrawn rows, as the plan records after Task 0's review | Seven: VTT-547 too, in Task 3 | VTT-547 was ambiguous for a bare-prefix body (QA finding 3.1) |
| Beat 2b cites C1, C5, C6, C7 | It cites VTT-556 for C6, and VTT-011 | C6 was withdrawn; the beat reads the server's list, which observes VTT-011 |
| Beat 2b counts the DM's roster rows | It reads `/api/participants` | the roster never re-reads for a refused joiner |
| Beat 2b checks the twin shows no `.conn` | No such check | the join view never draws `.conn`, so it could not fail; the server's list shows the twin was not admitted |
| Task 3 changes comments in the client | It also corrected the URL fixture's comment | a review nit: it named the wrong reader and the wrong test |
| QA's tests committed with their task | Two of Task 1's removed | they pinned what the rows do not require (finding 1.1) |

## What could not be established

- **Two joiners racing under one name.** In the verifier's simulation of
  the plan's order as single statements on `690ff7e` (P9), from the names
  read to the insert's commit took about a millisecond (p99 1.25 ms over
  300 runs). Two joiners released together under one name were both admitted
  in 200 of 200 pairs; started 50 ms apart, never. Left open as
  near-theoretical with a small consequence, by the ruling of 2026-10-21.
- **Names alike to the eye that the compare calls different:** a composed
  `Å` against its decomposed form, a trailing space that `vtt invite`
  keeps, `ß` against `SS`. Left the same way.
- **`joining.spec.ts` cannot be run twice against one campaign.** With
  `--repeat-each=2` the second repetition's Robin is now refused at beat
  2. On `690ff7e` it already failed, at beat 3, on two roster rows
  matching Robin. The file and `client/e2e/setup.ts` say a rerun is how a
  new e2e is checked for flakiness. Recorded under Open debt in
  `docs/verification-debt.md`.
- **The chain gate does not refuse a citation of a withdrawn row.**
  `tools/check-requirements-chain.py` names no such check, so a test
  citing VTT-547 would pass. Recorded under Open debt in
  `docs/verification-debt.md`.

## What was deliberately left out, and where it went

- Duplicates already in a campaign stay, by the owner's ruling, and
  nothing in the console tells them apart; revoking one is `vtt revoke
  --id`, and the console shows no id. Raised with the owner as a candidate
  ticket.
- A per-repetition guest name for `joining.spec.ts`, and a chain check
  that refuses a citation of a withdrawn row: open debt in
  `docs/verification-debt.md`, and candidates for the owner.
- The client's full mutation run. The ticket changed `client/src` and
  `client/test`, so the stored report, which measured `526885b`, is out of
  date, and `check:ts-mutation` judges nothing until the owner asks for
  `task check:ts-mutation:full`.

## Figures

`task check`, whole, on `eb04a87`, launched detached with its output to a
file: 2026-10-31 12:21:40 to 13:06:43, `TASK_EXIT=0`. The tree hashed the
same before and after.

- **Coverage:** 21 Go packages at or above their floors; 25 client source
  files expected, 24 gated, 1 excluded, each at its floor.
- **The Go mutation gate:** 15 packages, zero unadjudicated survivors.
  `internal/identity` and `internal/gateway`, which the ticket changed,
  were measured afresh: identity 0 survivors, gateway 2, both adjudicated
  before this ticket. The other 13 reused their last verdicts, their
  dependency closures unchanged, and 7 mutants in those reused verdicts
  are counted as killed on a timeout, as before.
- **The client's mutation:** `check:ts-mutation` judged nothing and said
  so. "THE CLIENT'S MUTATION WAS NOT MEASURED": the stored report measured
  `526885b`, and its inputs have changed since, with two tickets added,
  the console lock keys' and this one. No Stryker ran.
- **The records:** `check:requirements-chain` read 556 rows, 327 test
  files and 17 specifications; `check:new-prose` held 1499 added lines;
  `check:comments` was clean on 342 files after the ledger;
  `check:doc-owner` held 81 files.
- **The browser:** `task e2e`, whole, after Task 4's review: 32 passed,
  1 skipped.
