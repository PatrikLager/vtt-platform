# A reload keeps whose eyes the window looks through: the change

**Ticket:** `docs/superpowers/specs/2026-10-22-a-reload-keeps-the-viewpoint-design.md`,
verified by an agent that did not write it (passes with gaps G1-G8, which
the writer carried into the ticket before its commit).
**Plan:** `docs/superpowers/plans/2026-10-22-a-reload-keeps-the-viewpoint.md`,
written by that verifier and amended at sign-off and by the reading review of
`f270b22`, the commit that carried both.
**Where it came from:** finding A-9 of the alpha test of 2026-10-22,
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the second of the
nine tickets in the order the owner agreed.
**The owner's rulings, 2026-10-22:** the memory is kept per tab ("per filk",
meaning per flik), which also answers that nothing promises it outlives the
tab; the plan signed off ("Ja, kör"); and Q1, whether the key carries the
token, "Ja, nyckla på token".

**Last code commit:** `e9cb153`, on `0f8c46b`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `f270b22` | The ticket, the plan and VTT-442 to VTT-444 |
| `491fbed` | Task 1: `Wire` and `Session` take a remembered actor and name it in every dial before the first `ViewpointChanged`; `onViewpoint`; W1-W5; QA's `client/test/qa-remembered-log.test.ts`; the bundle; the register's cells |
| `e9cb153` | Task 2: `startSession` keeps the memory per tab under the token; E1, E2, A1-A7, the `beforeEach` lines; QA's `client/test/qa-reload-viewpoint.test.ts`; the bundle; the register's cells |

## What the code did on `0f8c46b`

`Wire` held the log it watched in its `log` field, which only a
`ViewpointChanged` set, and `boot` kept nothing in the browser but the token.
So a page loaded afresh dialled with no `log`, and the gateway opened the
participant's default (`openSeat`, `defaultViewpoint`; SPEC-015): nobody for a
spectator, the first party member by id for a player. The gateway already
opened a named log from its first entry when the participant may watch it
(VTT-339, VTT-349), so nothing on the server changed.

## Done looks like, answered

1. [x] "a spectator's reload keeps the shoulder, and another tab does not" in
   `client/e2e/table.spec.ts` perches a spectator on the Human Fighter,
   reloads, and with no click requires "Perched on: Human Fighter",
   `tok-fighter` on the board, and entries in the Story feed with no
   empty-feed line.
   It reads no ticker (ruling B-3). On Task 1's bundle it was red at the
   reload, reading "Perched on: nobody".
2. [x] "a DM hands a second character over, and the player keeps both across
   a reconnect" in `client/e2e/handover.spec.ts` now reloads in beat 3b after
   the switch to Bram (`act-hand-b`, not first by id) and requires Bram's chip
   `chip sel`, Ash's `chip` and Bram's token on the board. On Task 1's bundle
   it was red there: Bram's chip read `chip`.
3. [x] "a page loaded afresh in the tab dials naming the party member its last
   page watched, from after 0" in `client/test/app.test.ts` (A1) requires the
   second page's dial `log=a2`, `after=0`, after a first page that folded a2's
   log. It was red before the code, the dial naming no `log`.
4. Still true afterwards:
   - [x] A DM's or an agent's page dials no `log`, a fresh one included:
     VTT-397's tests are green, and A3's first case boots a DM under
     `?token=tok-dm` in a tab whose player page was told a2.
   - [x] A remembered party member the participant may no longer watch gives
     way to the one the gateway names, and until that `ViewpointChanged` the
     window shows nobody as watched, a failed dial included: A2 and A5.
   - [x] A page under another token in the tab names no `log`, the token from
     a `?token=` link or from storage: A3's two cases.
   - [x] A page in another tab under the same token names no `log`: E1's
     second page, which `context.newPage()` opens, and A4.

`task check`, whole, exited 0 on `e9cb153` in 4701 s, launched detached; no
floor was lowered and no ceiling raised. `task e2e` on the same commit: 19
passed and 1 skipped, in 23 s. The skipped one, "a network drop while the
table plays on ends in a Reconnect", is a `test.skip` in
`client/e2e/handover.spec.ts`.

## What the rules became

- The ticket's first rule, "a page loaded afresh in a tab opens on the party
  member its participant's last page in that tab watched, while the
  participant may still watch it", is **VTT-442** on the client, "until a
  `ViewpointChanged` arrives, each dial of a browser page names in `log`, with
  `after` 0, the actor named by the last `ViewpointChanged` sent to an earlier
  page in its tab under its token, and names no `log` when that one named
  nobody", and the existing VTT-339 on the gateway. E1 and E2 observe the two
  together.
- The ticket's second rule became two rows, since its halves break apart (K3
  and K2): **VTT-443**, another token in the tab, and **VTT-444**, another
  tab. Both were reworded before their first commit, when nothing cited
  them: their first wording forbade naming an ACTOR another token or tab had
  been named, and two tabs that both open on a player's default are each
  named it.
- VTT-397, VTT-398 and VTT-407 gained evidence. VTT-397 reaches a redial once a
  `ViewpointChanged` has arrived; before the first one the dial's `log` is
  VTT-442's to VTT-444's (D5).

The sort refused six candidates: the ticket's first rule as a row of its own;
"nothing promises it outlives the tab", which has nothing to go red; a
duplicated or restored tab opening where the original was, which the ruling
neither promises nor forbids; the window showing nobody until the gateway
answers, which is VTT-407; the next dial naming the gateway's choice, which is
VTT-397; and the key's spelling, the dial argument, the hook and the
`beforeEach` lines, which are how, not what.

## Phase 4a

QA ran twice, once per task that delivers behaviour; Task 0 is prose. Each
agent was given the rules, SPEC-022 whole and a hand-written surface (the
project records `go doc` for Go only), and was withheld the source, the
existing tests, the plan and the diffs.

- **Task 1:** 22 tests against `Wire` and `Session`, of which 21 passed; one
  was removed (finding 1 below). About 25 injections into its own fixtures,
  all red.
- **Task 2:** 24 tests through `boot`, all passing, about 30 injections, all
  red. It dumped the storages once at runtime with a probe in its own file,
  so it saw the key's spelling; it opened no source, and its file does not
  depend on the spelling.

### QA adjudications

- Task 1, QA finding 1: spec ambiguity — a remembered wire sent entries before
  any `ViewpointChanged` dials `after` at their cursor, where VTT-442 says 0 —
  an input no gateway sends: a log seat is sent its `ViewpointChanged` before
  any entry (VTT-368), a DM's or an agent's seat is sent none, and a promotion
  names only player or spectator and refuses a dm or agent target
  (`authorizePromotionTarget`, `handlePromotion`), so no world seat's token is
  ever remembered — the test was removed and VTT-442 stands. The Task 1
  reviewer re-derived it by command and agreed.
- Task 1, spec gap 2: `Wire.viewpoint` after a first `ViewpointChanged` naming
  nobody — it is `""`, asserted after review — no record change.
- Task 1, spec gap 3: `after` on a dial of a wire remembering nobody or
  nothing — VTT-398's — no change.
- Task 1, spec gap 4: whether "its first `ViewpointChanged`" is per socket —
  the wire's lifetime, as QA read it — SPEC-022, updated after this report,
  says so.
- Task 1, spec gap 5: whether `Session.onViewpoint` binds — inferred from the
  page's use — no change.
- Task 2, spec gap 1: SPEC-022's VTT-397 sentence lacks the scope before the
  first `ViewpointChanged` — SPEC-022, updated after this report, qualifies
  it.
- Task 2, spec gap 2: `after` on a no-`log` dial before a switch — VTT-398's;
  the gateway ignores it — no change.
- Task 2, spec gap 3: whether "sent" means received — a page acts only on
  what arrives — no change.
- Task 2, spec gap 4: a duplicated tab — D1 records that the ruling neither
  promises nor forbids it — no change.
- Task 2, spec gap 5: a browser without `sessionStorage` — near-theoretical;
  the page reads `localStorage` unguarded already — no change.
- Task 2, spec gap 6: the perch line before any `WatchableChanged` — existing
  behaviour, outside this ticket — no change.
- Task 2, spec gap 7: a tab's close — the ruling promises nothing past it —
  no change.
- Task 2, an untagged clause: SPEC-022's sentence on the perch's wording
  follows its VTT-403 tag, and whether VTT-403 covers it is unclear —
  existing, outside this ticket — no change.

## Phase 4b

One reviewer per task, two passes each, by command, on copies of the tree.

- Task 0 (`f270b22`). Pass 1 found three Musts: A3 could not catch K3, since
  it set the key by name; VTT-443 and VTT-444 were worded on the actor; and D5
  conflicted with VTT-397 as the register words it, now scoped for QA and for
  SPEC-022. Pass 2 found two: the rule 9 section said nothing MapTool keeps
  outlives the process, where `PersistenceUtil.saveCampaign` writes the GM's
  current map into the campaign file; and Task 3 would have written the old
  VTT-443/444 shape into SPEC-022.
- Task 1 (`491fbed`). Pass 1 found one Must, a QA comment over
  `check:new-prose`'s 85 columns, and five Shoulds: the hook ran before the
  arm applied the switch, so a handler read the old log and a throw stopped
  the switch; `wire.ts`'s header was false before the first
  `ViewpointChanged`; the register cells were not written; one QA assertion
  was only negative; and two QA tests fed entries before a `ViewpointChanged`.
  Pass 2 found two nits. K9, the hook only when the log changes, was added
  when the hook moved.
- Task 2 (`e9cb153`). Pass 1 found no Must and one Should: the warning beside
  the key pointed at VTT-443, which does not back it, and its "wherever the
  token is forgotten" also covered `auth.set` replacing a token, where removing
  the key would break VTT-442; it now says the key carries the token and a
  logout must remove both. Its nits included the design D6 rejected, writing
  `session.viewpoint` from `onChange`, which passed every test; A7 now catches
  it. Pass 2 found one nit, A7's name.

## The deliberate breaks

Each applied by a script to the working tree and undone, from its inverse edit
or the saved text, with the file's hash compared afterwards. Red counts are
the unit tests, QA's included, on the tree of the task that ran them: K4 to K9
on `491fbed`, before Task 2's tests existed, and the rest on `e9cb153`. This
report's reviewer re-ran K4 to K9 on a copy of `e9cb153`, where they also
redden Task 2's tests: K4 8 in all (A5 among them), K5 7 (A5), K6 10 (A2),
K7 4, K8 5 (A6), K9 3.

| Break | Red |
|---|---|
| K4: `this.log = remembered` in `Wire`'s constructor | W4 and three of QA's |
| K5: the remembered actor on the first dial only | W1 and three of QA's |
| K6: `remembered ?? log` | W2, W3 and five of QA's |
| K7: `log \|\| remembered` | W3 and two of QA's |
| K8: no hook for nobody | W5 and two of QA's |
| K9: the hook only when the log changes | W5 and two of QA's |
| K1: the `onViewpoint` line in `startSession` deleted | 22, A1 and A7 among them; E1 and E2 in the browser |
| K2: `localStorage` for `sessionStorage` | 5 (A2, A4, A5, two of QA's); E1's second tab in the browser |
| K3: the key without the token | 7 (A2, A3, A5, four of QA's, among them `qaRV: two tokens in one tab each keep their own remembered actor`, which interleaves two tokens in one tab) |
| D6's rejected writer, `session.viewpoint` from `onChange` | A7 alone |
| F1: A1 remembering the actor the default picks, under K1 | A1 still red: it reads the dial |
| F2: `act-hand-a` sorts before `act-hand-b` | read; E2 would be green on the old tree otherwise |

## Rule 9: how MapTool does this

MapTool hands a reconnecting client no map of its own: `SetCampaignMsg` calls
`setCampaign(campaign, null)`, which takes the campaign's landing map, the
GM's setting, and then the first map the player may see. What it keeps is
the GM's own view, in the campaign file `PersistenceUtil.saveCampaign`
writes, and the host's current map across `startServer`, in memory. Nothing
is kept for a remote client, so there was no per-client memory to borrow; the
server-side default is borrowed, and this platform already had it
(`defaultViewpoint`). The plan's rule 9 section carries the detail.

## Figures

From the one `task check` on `e9cb153`:

- Go coverage: 21 packages at or above their floors; the gate notes 11 that
  clear theirs by more than 1.0pp. The ticket changed no Go code.
- Client coverage: 22 files gated, each at 100.00%, `client/src/wire.ts`,
  `client/src/session.ts` and `client/src/app.ts` among them; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors. Each reused its
  last passing verdict, since nothing in its dependency closure differed. 38
  survivors, all adjudicated; 7 timed out and counted as killed, not
  measured.
- Client mutation: in its container, Stryker's own "Done in 59 minutes and 26
  seconds": 2964 mutants, 2889 killed, 10 timed out and counted as killed, 65
  survivors all adjudicated equivalent. `client/src/wire.ts`: 123 killed, 1
  timed out and 2 survived, against 119, 1 and 2 on `38e837c`; its timeout is
  the same mutant on both, the `BlockStatement` of `ws.onerror` in
  `connect`. `client/src/app.ts`: 224 killed, none survived.
  `client/src/session.ts`: 60 killed, 2 survived. The one timeout the run on
  `38e837c` did not have is in `client/src/fold.ts`, which this ticket did
  not touch: an `ArrayDeclaration` on `?? []`, where that file went from 15
  survivors and no timeout to 14 and one.
- Race: both runs pass.
- Requirements chain: 444 rows, 286 test files, 16 specifications.
- Comments: 319 files, 9 comment lines added against `main`, clean: the
  warning beside the key in `startSession`, seven one-line docs in
  `client/src/wire.ts` and `client/src/session.ts`, and one line of
  `wire.ts`'s header. Its two notices are the
  ones `38e837c` had: `cmd/vtt/events_tail_truncation_test.go` at 22.83
  against 22.6 and `internal/gateway/export_test.go` at 49.18 against 45.9.
  `tools/comment-ceilings.txt` is unchanged.
- Disk: 48 GiB free at the launch, after `go clean -cache`, and 44 GiB when
  the run ended.
- Duration: 4701 s, from the log's `START` line at 22:46:39 to its `END` line
  at 00:05:00, against 7905 s on `38e837c`, where thirteen Go packages were
  re-mutated; here none was.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| SPEC-022 at Task 3, before the gate and the report | After the report, its own step and commit, so the gate ran on `e9cb153` and not on SPEC-022's commit. Of the gates only `check:requirements-chain` reads a specification, and no hook runs it, so that commit gets it by hand | The dev-cycle's Phase 5 orders the record last, after the report it is derived from |
| The hook fired before the arm's early return (D6) | It fires after the arm applies the switch, for every `ViewpointChanged` | The Task 1 review: before, a handler read the old log, and a throw stopped the switch while the cursor moved |
| VTT-397 and VTT-407 gain evidence | VTT-398 too: five of QA's Task 1 tests | They pin the cursor a redial names once a `ViewpointChanged` has arrived, which is VTT-398's rule |
| A1-A6 | A1-A7 | The Task 2 review: D6's rejected design passed every test |
| D3's warning, "whatever forgets the token must forget these" | "The key carries the token: a logout (`Auth.clear`) must remove both" | The Task 2 review: as first written it pointed at VTT-443, which does not back it, and covered `auth.set` replacing a token, where removing the key would break VTT-442 |

## What could not be established

- How the alpha's two windows held two participants in one browser: two tabs
  of one profile cannot each reload as their own participant (below), and
  whether the alpha used two profiles, two browsers or a private window was
  not recorded. The ticket's open point stands.

## What was deliberately left out, and where it went

- A logout. `Auth.clear` has no caller. The warning beside the key says a
  logout must remove both, and a logout that does makes a re-login under the
  same token in that tab name no `log`, which VTT-442 as worded forbids. No
  ticket carries a logout. SPEC-022, updated after this report, records the
  consequence the warning points at, and a logout withdraws VTT-442, whose
  row stays, marked, and brings a new id.
- Two participants in two tabs of one profile share `vtt.token`, so a reload
  of the first tab comes back as the second participant, with no remembered
  viewpoint (VTT-443). Not this ticket's; the signed-off plan names it under
  Forward coupling as one the owner may want a ticket for.
- A duplicated or restored tab, which browsers give a copy of the tab's
  storage: the ruling neither promises nor forbids it (D1).
- B-4, the fourth ticket: a player who controls no party member will watch
  as a spectator does, so such a player's remembered party member will be
  opened when `MayWatch` accepts it, which fits the ticket's first rule,
  "while the participant may still watch it"; it goes to B-4's plan, as this
  ticket's plan says under Forward coupling.
- S-2, the seventh: a player who lost their token gets a new one, which
  starts with no memory.
