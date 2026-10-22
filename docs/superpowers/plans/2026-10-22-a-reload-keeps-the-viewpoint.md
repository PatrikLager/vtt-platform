# Plan: a reload keeps whose eyes the window looks through

- **Ticket:**
  `docs/superpowers/specs/2026-10-22-a-reload-keeps-the-viewpoint-design.md`,
  from finding A-9 of `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`,
  the second ticket in that note's order of work.
- **Verdict:** passes with gaps (G1-G8, below). Verified 2026-10-22 by an agent
  that did not write the ticket. It was read-only against HEAD `0f8c46b`.
  Probes ran outside the tree, in a `git archive HEAD` copy: scratch bun tests
  through `boot`, and a prototype of the change for its measurements.
  `task check`, `task e2e` and the mutation gates were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-22-a-reload-keeps-the-viewpoint.md`.
- **Base:** main `0f8c46b`.
- **Branch:** `fix/a-reload-keeps-the-viewpoint`.
- **Owner's ruling taken in:** the memory is kept per tab ("per filk",
  2026-10-22, meaning "per flik"). It settles where the memory lives (per tab)
  and whether it outlives the tab (nothing promises it). D1 records it.
- **Signed off:** 2026-10-22, by the owner ("Ja, kör"). Q1 answered the same
  day: "Ja, nyckla på token", the recommendation (D3).

## The gaps this plan inherits

Each gap gives the text the verifier proposed. The writer revised the ticket
for each, and the ticket's text is the authority: where it differs, the
ticket stands.

- **G1. Rule 1 and item 3 promise more than one tab.** "Its participant's last
  page" names the participant's last page in any tab. The owner's ruling keeps
  the memory per tab.
  - Rule 1, proposed: "A page loaded afresh in a tab opens on the party member
    the tab's last page watched, while the participant may still watch it."
  - Item 3, proposed: "A client test in `client/test/` shows that a page loaded
    afresh in a tab whose last page, under the same token, watched an actor
    dials naming that actor in `log` with `after` 0. It fails on today's tree,
    whose first dial names no `log`."
- **G2. Rule 2 does not carry the ruling.** Proposed: "A remembered viewpoint is
  used only in the tab whose page remembered it, and only by the participant
  whose page it was. Nothing promises it outlives the tab (the owner's ruling of
  2026-10-22)."
- **G3. Item 4's third point says "the same browser".** Under the ruling,
  another tab never sees the memory. What is left is the tab itself under
  another token. Proposed: "A page loaded afresh in a tab under another
  participant's token does not dial with the viewpoint the earlier
  participant's page in that tab remembered. The token may come from a
  `?token=` link opened in that tab, or from another tab of the profile that
  replaced `vtt.token`. A test observes both."
- **G4. Nothing observes the per-tab half of the ruling.** Proposed, a new point
  under item 4: "A page in another tab of the same browser profile, under the
  same token, does not open on what the first tab remembered. A browser e2e
  opens a second page in the first page's browser context after the first page
  perched, and observes that its dial names no `log`. A client test observes the
  same for a page booted with the token in `localStorage` and an empty
  `sessionStorage`." Both are green today. The first goes red if the memory is
  kept in `localStorage` (K2).
- **G5. Item 1's "the window shows that party member's log" names nothing to
  read, and it must not be the ticker.** Proposed: "... and that the window
  shows that party member's log: its token on the board (`tok-fighter` for the
  adventure's Human Fighter) and its entries in the Story (`.feed`), which on
  nobody reads "Nothing has happened yet.". It reads neither the ticker nor
  `.tick`, which the owner's ruling B-3 gives to the DM alone."
- **G6. Item 4's first point, "a reload included", names no observation.**
  VTT-397's tests redial one page, and none of them loads a page afresh.
  Proposed: "A DM's or an agent's page dials naming no `log`, a page loaded
  afresh included. VTT-397's tests stay green, and a client test loads a DM's
  page in a tab whose earlier page, under a player's token, watched a party
  member, and observes a dial with no `log`."
- **G7. Item 4's second point says nothing of the window before the gateway
  answers.** VTT-407 already rules on it. The most likely implementation breaks
  it: holding the remembered log as `Wire`'s `log`. A spectator page whose
  remembered dial fails then reads "Perched on: a2", an id no
  `ViewpointChanged` named (P4). Proposed, appended to that point: "Until that
  `ViewpointChanged` arrives, a dial that fails included, the window shows
  nobody as watched, the remembered one included (VTT-407). A test observes it."
- **G8. The file list is short.** Proposed for "What it touches" item 1:
  "`client/src/app.ts`, which boots the page and keeps what a tab remembers;
  `client/src/wire.ts`, whose dials name the log; and `client/src/session.ts`,
  the only code that builds a `Wire`. Their tests: `client/test/app.test.ts`
  and `client/test/wire.test.ts`. Also the `beforeEach` of every test file that
  calls `boot()` (`app.test.ts`, `qa-one-log.test.ts`, `qa-switch.test.ts` and
  `qa-window-number.test.ts`), which clears `sessionStorage` beside
  `localStorage`." For item 3: "`client/e2e/table.spec.ts` for the spectator and
  `client/e2e/handover.spec.ts` for the player."
  - Without the `beforeEach` change, two tests in `qa-one-log.test.ts` go red
    (P2).
  - `client/src/auth.ts` and `client/test/auth.test.ts` are not touched (D2).

The ticket's "What could not be established" gets its owner update from the
ticket's writer. Its first point stands after the ruling: two tabs of one
profile still share `vtt.token` ("Forward coupling").

## Established here, not in the ticket

- **E1. The tests share one `sessionStorage` (P2, P3).** bun runs every test
  file in one process with one happy-dom window, and the boot tests use the
  token `tok` for every role. So a memory a player test leaves is read by a
  later DM test, in the same file or a later one. Each boot test file must clear
  `sessionStorage` in its `beforeEach`, as it already clears `localStorage`.
- **E2. `auth.ts`'s header cannot be edited in place (P8).** It is one comment
  block of 20 lines, over `check:comments`' bound of 6, and it holds a banned
  term (`spec §3`). Editing a line of it while it stays over 6 lines makes the
  block an added over-long block, which the gate refuses; editing it at all
  means rewriting the trust posture in 6 lines or fewer, without `spec §3`.
- **E3. The remembered log must not become the held log (P4).** If it did, the
  window would show it as watched before any `ViewpointChanged` named it.
- **E4. The gateway needs no change (P6).** A dial naming a log the participant
  may watch, `after` 0, opens on it from its first entry. One naming a log they
  may no longer watch opens on the default.

## Measurements this plan stands on

Every probe ran outside the repository, at `0f8c46b`.

| # | Probe | Result |
|---|---|---|
| P1 | Scratch test through `boot`: a player page is opened on `a2` (set `a1`, `a2`; `ViewpointChanged` `a2`), closed, and `boot` runs again on the same storage | red: the second dial is `?token=tok&after=0`, with no `log`. Item 3 fails today for the stated reason. happy-dom provides `sessionStorage` |
| P2 | Prototype of D3-D6 (`sessionStorage`, key `vtt.viewpoint.` + token), whole `client/test` suite, no `beforeEach` change | every test but 2 passes; the 2 that fail are "dm: a redial names no log and the highest world sequence sent" and its agent twin in `qa-one-log.test.ts`. An earlier player test's memory under `tok` named a log |
| P3 | P2 plus `sessionStorage.clear()` in the four boot files' `beforeEach` | every test passes. `bunx tsc --noEmit -p client/tsconfig.json` is clean |
| P4 | The page's memory holds `a2`; a spectator page boots and its dial fails. Two variants: `Wire` names the remembered log only in the dial, or holds it as `log` | dial-only: "Perched on: nobody"; held as `log`: "Perched on: a2" |
| P5 | The dial-only prototype | A Reconnect after a failed first dial sends `?token=tok&after=0&log=a2`. A memory of `a2` that the opening `ViewpointChanged` answers with `a1` is then `a1`. A DM token in the tab, after a player's memory, sends `?token=tok-dm&after=0`. "Another tab" (`sessionStorage` cleared, `localStorage` kept) sends `?token=tok&after=0` |
| P6 | `go test ./internal/gateway -run` on `TestACursorIsHonouredOnlyInTheLogItWasReadFrom`, `TestALogNamingAnActorItMayNotWatchOpensOnTheDefault`, `TestQASeatANamedLogOpensAfterItsCursor` and `TestQASeatASpectatorWatchesNobodyUntilItPerches` | all pass. The cases include a player at `after=0&log=scout` opening on scout from its first entry, a spectator at 0 naming `a1` opening on `a1`, and a player naming a revoked `a1` opening on the default `a0` |
| P7 | `check-comments.py`'s `measure` | see "The comment budget" below |
| P8 | Same | `auth.ts`: one 20-line header block, with the banned term `spec §3` inside it. Over-bound blocks in `wire.ts` sit in `PresenceEvent`'s doc, `onPresence`'s doc and `handleFrame`'s result arm; in `session.ts`, in three docs this plan does not touch. The lines this plan rewords are one-line blocks: `connect`'s and `reconnect`'s docs in `wire.ts`, and `reconnect`'s doc in `session.ts` |
| P9 | grep | In `client/src`, `new Wire(` only in `Session`'s constructor. `new Session(` in `app.ts`, `session.test.ts` and `qa-one-log.test.ts`. `new Auth(` only in `boot`. `boot(` in `app.test.ts`, `qa-one-log.test.ts`, `qa-switch.test.ts` and `qa-window-number.test.ts`. No e2e reloads a page or opens a second page in one context. No specification mentions browser storage. `identity` has no way to give a participant a second token (`RotateJoinSecret` rotates the join secret only) |
| P10 | `stryker.conf.json`, `tools/ts-mutation-equivalents.txt` | every `client/src` file but `main.ts` is mutated. `wire.ts` has two adjudications (`env.sequence >= this.lastSeq`, the `default:` arm) and `auth.ts` one, none on code this plan edits |

Not measured: the browser e2e (a verifier constraint), and the mutation run.

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no floor, ceiling or exemption moves. `tools/comment-ceilings.txt` and
    `tools/ts-coverage-thresholds.txt` are not touched. `app.ts`, `wire.ts`
    and `session.ts` are held at 100% TS coverage.
  - 3: the contract does not change.
  - 4 and 5: no fold and no game-system word is added.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment is a warning or a pointer, or an exported symbol's one-line
    doc. No date, hash or task goes beside the code.
- **The comment budget (P7).** These are maxima, compared unrounded. Run
  `python3 tools/check-comments.py main` after staging.

  | File | Comment lines of non-blank | Share | Ceiling | Most comment lines with 4 / 8 code lines added |
  |---|---|---|---|---|
  | `client/src/wire.ts` | 78 of 255 | 30.59% | 30.6 | 1 / 3 |
  | `client/src/session.ts` | 63 of 175 | 36.00% | 36.0 | 2 / 4 |
  | `client/src/app.ts` | 261 of 526 | 49.62% | 49.7 | 4 / 8 |

- **"You log in with no character."** Every party member a player holds in an
  e2e is granted by the DM through the console. Both e2e use existing grants.
- **Ruling B-3.** No new e2e reads `.ticker` or `.tick`.
- **A near-theoretical edge case with small consequence is no debt.** It gets
  no code, no test and no debt entry (D1's browser behaviours).
- **Requirement ids** come from `requirement-id` after sign-off, never by hand
  (SPEC-008). A test or spec file that cites an id needs the row first. An e2e
  spec named as evidence must carry the id in a `// VTT-NNN` line.
- **Tier 1 runs at pre-commit:** `bun test client/test contract contract-spike`
  and `task client:typecheck`. Source, tests and the `beforeEach` change go in
  one commit.
- **bun is quiet under `CLAUDECODE`.** Run `env -u CLAUDECODE bun test …`.
- **The bundle.**
  - Every commit that changes `client/src` carries `cmd/vtt/webdist`, rebuilt
    by `task build:client`. The commit hook does not catch a stale bundle.
  - `check:drift` passes only after the commit. Pre-push runs it.
  - `task e2e` serves whatever webdist is in the tree, so a client change
    reaches it only after `task build:client`.
  - Check the machine load before `task e2e` (`os.getloadavg()`).
- **TS mutation.** Any change under `client/src` or `client/test` re-mutates the
  client. No adjudication sits on code this plan edits (P10).
- **`review-record` refuses untracked files.** Commit the ticket and this plan
  in Task 0.
- **Mechanics.**
  - A new test goes after a closing brace.
  - Stage and commit in separate calls.
  - Undo a break with its inverse edit and check with grep, never with
    `git checkout --`.

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool`:

- **What MapTool does.**
  - `ClientMessageHandler`'s `handle(SetCampaignMsg)` calls
    `MapTool.setCampaign(campaign, null)`. So a connecting or reconnecting
    client is handed no map of its own.
  - With no map given, `setCampaign` takes the campaign's landing map
    (`Campaign.getLandingMapId`, the GM's setting, sent by
    `ServerCommandClientImpl.setLandingMap` and stored by
    `ServerMessageHandler`). Failing that, it takes the first map the player may
    see.
  - The callers that pass a map are the GM's own load of a campaign file
    (`AppActions`, `campaign.currentZoneId`, which
    `PersistenceUtil.saveCampaign` wrote from the saving client's current map,
    its zoom beside it as `currentView`) and `MapTool.startServer`, which keeps
    the host on the map its frame shows when it adopts its local connection.
    Neither is a client reconnecting.
  - The impersonated token lives in `CommandPanel`'s memory.
    `AppPreferences` remembers neither a map nor a token.
- **What we borrow.** The server-side default. We already have it:
  `defaultViewpoint`, the first party member by id or nobody (SPEC-015).
- **What MapTool does not solve.** A remote client that comes back where it
  was: `SetCampaignMsg` hands it no map. What MapTool keeps is the GM's own
  view, in the campaign file it saves, and the host's current map across
  `startServer`, in memory. Neither is kept for a remote client, so D1-D6 are
  this platform's own.
- **Not borrowed, as rule 9 requires:** the distribution. That a reload asks for
  a log decides nothing. The gateway judges the named log with `MayWatch` and
  opens the default otherwise (P6).

## Design question for the owner

**Q1. Does a tab keep the token itself beside what it remembered?** Answered
2026-10-22: yes ("Ja, nyckla på token").

The memory must know whose it is (rule 2). A tab's `sessionStorage` outlives a
change of `vtt.token` in `localStorage`. Another tab of the profile can replace
the token, and this tab's next load then reads the new token beside the old
participant's memory (P5). The page dials before `/api/me` answers, so the
participant id is not known in time.

- **Recommended: yes.** The key is `vtt.viewpoint.` plus the token, and the
  value is the actor id, `""` for nobody.
  - Same readers as `vtt.token`, any script on the origin (the trust posture in
    `auth.ts`'s header).
  - Lives no longer than the tab.
  - Exact for both ways a tab changes participant: a link opened in it, and a
    token replaced by another tab.
  - Three lines of code in `startSession`.
- **Alternative: no copy of the token.**
  - Clear the memory when a page boots with a link token that differs from the
    stored one, and on a `storage` event for `vtt.token` from another tab.
  - Costs a listener, a comparison and their mutants.
  - It misses a tab the browser froze while another tab replaced the token.
    That tab reloads with the old memory under the new token, and rule 2 breaks
    there.
- **Tradeoff of the recommendation:** a logout added later (`Auth.clear`, which
  nothing calls today) must also clear `vtt.viewpoint.*` in its tab. It cannot
  reach other tabs' copies, which die with those tabs. D3 puts a one-line
  warning beside the key.
- **What changes if the alternative is chosen:** Task 2's `app.ts` change and
  the preconditions of A2 and A5. Nothing else.

## Decisions

1. **D1. Per tab: `sessionStorage`.**
   - Forced by the owner's ruling of 2026-10-22 ("per filk").
   - A reload keeps the memory. Nothing promises it past a tab's close. Two tabs
     do not share it.
   - Browsers copy a tab's `sessionStorage` into a duplicated tab, and Chromium
     restores it for a closed tab reopened. Then that tab opens where the
     original was. The ruling neither promises nor forbids this, so it gets no
     code and no test.
2. **D2. The memory is kept by `startSession` in `client/src/app.ts`, not in
   `auth.ts`.**
   - Chosen over the cost E2 names: `auth.ts`'s header says "Token storage",
     and its posture cannot be edited in place. `app.ts` already chooses the
     storage (`new Auth(localStorage)`).
   - `startSession` serves both ways in, the invite and the join.
   - Rejected: a new module, which a third reader would reach for. That is a
     preference, named as one.
3. **D3. The key is `vtt.viewpoint.` plus the token, and the value is the actor
   id, `""` for nobody** (Q1's recommendation).
   - Keying by token keys by participant, since a participant holds one token
     (P9).
   - A reload after a deploy reads what the previous bundle wrote, so the key's
     spelling is a contract between builds. A2 and A5 write it by name.
   - One warning line goes beside it: whatever forgets the token must forget
     these.
4. **D4. The remembered actor is a dial argument, never the held log.**
   - `Wire` takes it as a third constructor argument, `remembered: string |
     null = null`, and each dial names `this.log ?? remembered`.
   - `viewpoint` stays what the last `ViewpointChanged` named.
   - `Session` takes the same third argument and hands it on.
   - Forced by VTT-407 and P4.
5. **D5. Every dial before the first `ViewpointChanged` names it**, a Reconnect
   after a failed first dial included.
   - Forced by rule 1. A Reconnect that named nothing would open on the default.
     The default's `ViewpointChanged` would then overwrite the memory.
   - It is the same code as naming it on the first dial alone (P5).
   - VTT-397 stands. Before any `ViewpointChanged` a page holds no log (SPEC-022
     defines the held log by the last one), so VTT-397 does not reach that case,
     and R1 does.
6. **D6. The memory is written at every `ViewpointChanged`, nobody's included,
   through a hook of its own.** The hooks are `Wire.onViewpoint(fn: (actorId:
   string) => void)`, fired in the `viewpointChanged` arm before its early
   return, and `Session.onViewpoint`, which forwards to it.
   - Rejected: writing `session.viewpoint` from `onChange`.
     - `onChange` fires on the opening's presence snapshot, before the opening
       `ViewpointChanged`. It would write `""` over the memory before the
       gateway answered.
     - It also fires on every entry, a write per event.
   - Nobody's is included so that a spectator who chose "No shoulder" reloads on
     nobody.
7. **D7. A fresh page dials `after` 0.** Forced by VTT-398: a page that has
   folded nothing has cursor 0. Nothing about the cursor is remembered.
8. **D8. Every boot test file clears `sessionStorage` in its `beforeEach`.**
   - Forced by E1, P2 and P3.
   - Rejected: injecting the storage into `boot`, which changes `boot`'s
     signature and `main.ts`. That is a preference, named as one.
9. **D9. Where the tests go.** Wire's rules go in `wire.test.ts` and the page's
   in `app.test.ts`, the two the ticket names.
   - `Session` only hands on, and `app.test.ts` holds it: coverage and the
     mutation gate reach it through `boot`.
   - A unit test asserts the DIAL. Its fake gateway answers whatever the test
     scripts, so what the window shows after a `ViewpointChanged` is the
     script's; the dial, and the window before one, are the client's. Only the
     e2e has a real gateway judging.
10. **D10. The two e2e.**
    - The spectator: a new test in `table.spec.ts`, after "a spectator chooses a
      shoulder and is shown that log".
    - The player: a reload in `handover.spec.ts`'s beat 3b, right after the
      switch to Bram.
      - Bram (`act-hand-b`) is not first by id. Ash (`act-hand-a`) is, so
        today's tree comes back on Ash.
      - That test already holds two party members, both granted through the DM
        console.
      - Reloading inside it instead of writing a new test with its own setup is
        a preference, named as one.
    - Neither reads `.tick` (B-3).
11. **D11. The gateway and `auth.ts` are not touched** (E4, D2).
12. **D12. No verification-debt recipe.**
    - A-9 is behaviour no rule asked for. The ticket's problem says so, and the
      note files it under "Cost time". It is not a defect that escaped a gate.
    - The breaks K1-K8 are the record of what turns its tests red.
13. **D13. `Specifications this moves: SPEC-022` stands (a reading).**
    - SPEC-015 describes the gateway, which does not change.
    - SPEC-007's wire does not change.
    - No specification describes the browser's storage (P9).
14. **D14. One commit per task.** Each commit that changes `client/src` carries
    the rebuilt `cmd/vtt/webdist`.

## Requirements

Three rows, allocated by `requirement-id` at Task 0 and written
`**OPEN — no test yet**`. The sort split the verifier's R2 in two, since its
halves break apart (K3 and K2), so R2 is the token and R3 the tab.
R2's and R3's first wording was broken by correct behaviour: it forbade an
ACTOR another token or tab had been named, and two tabs that both open on a
player's default are each named it. Both rows were reworded before their
first commit, when nothing cited them, to say that a page whose own tab and
token were told nothing names no `log`, whatever another token or another tab
was told.

- **R1 = VTT-442.** Until a `ViewpointChanged` arrives, each dial of a browser
  page names in `log`, with `after` 0, the actor named by the last
  `ViewpointChanged` sent to an earlier page in its tab under its token, and
  names no `log` when that one named nobody.
  - Evidence: W1, W2, W3, W5, A1, A2, A5, A6, E1 and E2.
- **R2 = VTT-443.** Until a `ViewpointChanged` arrives, a browser page none of
  whose earlier pages in its tab under its token was sent a `ViewpointChanged`
  names no `log` in its dial, though an earlier page in its tab under another
  token was sent one.
  - Evidence: A3.
- **R3 = VTT-444.** Until a `ViewpointChanged` arrives, a browser page none of
  whose earlier pages in its tab under its token was sent a `ViewpointChanged`
  names no `log` in its dial, though a page under its token in another tab was
  sent one.
  - Evidence: A4, and E1's second page.

The sort refused these candidates:

- The ticket's first rule as its own row: it is VTT-442 on the client and
  VTT-339 on the gateway, and E1 and E2 observe the two together.
- "Nothing promises it outlives the tab": a promise not made has nothing to go
  red.
- D1's browser behaviours, a duplicated or restored tab opening where the
  original was: the ruling neither promises nor forbids them.
- The window showing nobody as watched until the gateway answers: VTT-407
  already says it.
- The next dial naming the log the gateway chose: VTT-397 says it.
- The key's spelling (D3), the dial argument (D4), the hook (D6) and the
  `beforeEach` lines (D8): how, not what.
- **Existing rows** gain evidence, and the tests carry their ids:
  - VTT-397 gains W2, W3 and A2.
  - VTT-407 gains W4, A2 and A5.

| Ticket item | Observed by |
|---|---|
| 1 (spectator reload, e2e) | E1 |
| 2 (player reload, e2e) | E2 |
| 3 (fresh page dials the remembered actor) | A1, W1 |
| 4.1 (DM or agent, no `log`) | VTT-397's tests, A3 |
| 4.2 (remembered one refused) | A2, A5 |
| 4.3 (another token in the tab) | A3 |
| 4.4 (another tab) | A4, E1's second page |

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - The ticket's writer revises the ticket for G1-G8.
  - Get the owner's sign-off on this plan and an answer to Q1.
  - Allocate R1 to R3 with `requirement-id`, which writes their rows to
    `docs/requirements.md` with OPEN evidence, then commit the ticket, this
    plan and `docs/requirements.md`.
- **True when:** `git status` is clean and the three rows exist.

### Task 1: `Wire` and `Session` carry a remembered log (R1, D4-D6)

- **Files:** `client/test/wire.test.ts`, `client/src/wire.ts`,
  `client/src/session.ts`, `cmd/vtt/webdist/` from `task build:client`, and
  `docs/requirements.md`.
- **Tests,** after "a redial while watching nobody names no log", each with a
  `// VTT-NNN` line for the rows that name it as evidence (Requirements, above):
  - **W1** "a wire handed a remembered log names it, from after 0, on every dial
    until a ViewpointChanged arrives". It checks `connect`, then `reconnect`
    with no frame between them. Red today: no `log`. Kills K5.
  - **W2** "a ViewpointChanged naming another log replaces the remembered one on
    the next dial". Remembered `a2`, then `ViewpointChanged` `a1` and entries 1
    and 2; the redial is `log=a1`, `after=2`. Green today. Kills K6.
  - **W3** "a ViewpointChanged naming nobody leaves the next dial naming no log,
    though a log was remembered". Green today. Kills K7.
  - **W4** "a wire handed a remembered log shows no viewpoint until a
    ViewpointChanged names one". `wire.viewpoint` is `null` after `connect`.
    Green today. Kills K4.
  - **W5** "each ViewpointChanged reaches onViewpoint, nobody's included". `a1`,
    then `a1`, then nobody give `["a1", "a1", ""]`. Red today: there is no
    `onViewpoint`. Kills K8.
- **Code:** D4 and D6.
  - Reword `connect`'s and `reconnect`'s one-line docs in `wire.ts`, and
    `reconnect`'s in `session.ts`. Each names the log held, or before any
    `ViewpointChanged` the one handed in, and the cursor.
  - Reword `Wire.viewpoint`'s and `Session.viewpoint`'s one-line docs so that
    null, or `""` on `Session`, also covers a page before its first
    `ViewpointChanged` (D5).
  - One one-line doc for each `onViewpoint`. No other comment.
  - This stays inside the comment budget.
- **Order:** the tests, W1 and W5 red; then the code; then `task build:client`.
- **True when:**
  - `env -u CLAUDECODE bun test client/test contract contract-spike` is green,
    and so is `task client:typecheck`.
  - `check-comments.py main` passes after staging.
  - `git status` lists the files above and `cmd/vtt/webdist/assets/index.js`.
  - `app.ts` is untouched, so the page still behaves as today.
  - `Session.onViewpoint` is reached by no test until Task 2's `app.ts`
    change. No gate runs between the two commits, so nothing goes red, and
    the 100% floor is measured at Task 4.
- **Breaks,** each applied, run, and undone by its inverse edit:
  - **K4:** set `this.log = remembered` in the constructor. W4 goes red.
  - **K5:** name `remembered` on the first dial only. W1 goes red on the
    `reconnect`.
  - **K6:** dial `remembered ?? this.log`. W2 goes red.
  - **K7:** dial `this.log || remembered`. W3 goes red.
  - **K8:** skip the hook for `""`. W5 goes red.

### Task 2: the page remembers per tab (R1-R3, items 1-4)

- **Files:**
  - `client/e2e/table.spec.ts` and `client/e2e/handover.spec.ts`;
  - `client/test/app.test.ts`;
  - the `beforeEach` of `app.test.ts`, `qa-one-log.test.ts`, `qa-switch.test.ts`
    and `qa-window-number.test.ts` (D8);
  - `client/src/app.ts`;
  - `cmd/vtt/webdist/`;
  - `docs/requirements.md`.
- **E1 in `table.spec.ts`,** under `// VTT-442 VTT-444`, named "a
  spectator's reload keeps the shoulder, and another tab does not". It takes `{
  page, context }`.
  - `openAs(page, "spectator")`, click the `.perch .shoulder` "Human Fighter",
    and wait for `.perched-on` to read "Perched on: Human Fighter".
  - `page.reload()`, then `.conn` reads "connected". With no click:
    - `.perched-on` reads "Perched on: Human Fighter";
    - `[data-token-id="tok-fighter"]` is visible (the fighter stands at 0,0,
      per `adventures/goblin-ambush/guide.md`);
    - `.feed .mechanical` is not empty, and `.feed .empty` is absent.
  - Screenshot `04c-spectator-reloaded-on-the-shoulder`.
  - The second tab: `context.newPage()`, with `other.on("websocket", …)`
    keeping each socket's URL. Then `openAs(other, "spectator")`, and
    `.perched-on` reads "Perched on: nobody".
  - Every kept URL carries no `log` parameter. This is the page's own question,
    so it does not depend on when the gateway's answer lands.
- **E2 in `handover.spec.ts`,** beat 3b, after screenshot `h6b-player-switches-to-bram`.
  The test gains a `// VTT-442` line.
  - `player.reload()`, then `.conn` reads "connected".
  - The "Bram" chip has class `chip sel` and the "Ash" chip `chip`.
  - `[data-token-id="tok-act-hand-b"]` is visible.
  - Screenshot `h6b2-player-reloaded-on-bram`. Then the beat goes on to Ash as
    now.
- **Tests in `app.test.ts`,** after the last test, each with the id lines of the
  rows that name it as evidence:
  - **A1** "a page loaded afresh in the tab dials naming the party member its
    last page watched, from after 0". A player page is sent the set `a1`, `a2`
    and `ViewpointChanged` `a2`, then closes. The second `boot`'s dial is
    `log=a2`, `after=0`. Red today (P1). Kills K1.
  - **A2** "a remembered party member the gateway refuses gives way to the one
    it names, and the next dial names that one".
    - `sessionStorage` holds `vtt.viewpoint.tok` = `a2`, and the player's dial
      is `log=a2`.
    - The fake answers the set `a1` and `ViewpointChanged` `a1`. The `a1` chip
      is `sel`, and there is no `a2` chip.
    - After a close, Reconnect dials `log=a1`.
    - Red today: the first dial names no `log`. Kills K6 at the page.
  - **A3** "a page under another token in the tab names no remembered log".
    A player page under `tok` is sent the set `a1`, `a2` and
    `ViewpointChanged` `a2`, then closes, as in A1; then two cases:
    - a DM through `?token=tok-dm`;
    - a spectator through `localStorage` `vtt.token` = `tok-s`.
    Each dials no `log`. Green today. Kills K3.
  - **A4** "a page in another tab names no log its first tab remembered". A
    spectator page is sent `ViewpointChanged` `a2`. Then `sessionStorage.clear()`
    runs with `localStorage` kept, and `boot` dials no `log`. Green today. Kills
    K2.
  - **A5** "until the gateway names a viewpoint the window shows nobody as
    watched, a failed dial included, and Reconnect still names the remembered
    log".
    - A spectator with `vtt.viewpoint.tok` = `a2`.
    - The first dial never opens (`onerror`, then `onclose`, as
      `qa-one-log.test.ts`'s `fail` does). The perch reads "Perched on:
      nobody" (P4), and Reconnect dials `log=a2`.
    - That socket opens and is sent the set `a1`, `a2`, but no
      `ViewpointChanged`. The perch still reads "Perched on: nobody".
    - Red today on the Reconnect's `log`. Kills K4 at the page.
  - **A6** "a spectator who left the shoulder reloads on nobody". The page is
    sent `ViewpointChanged` `a2`, then nobody. The next `boot` dials no `log`.
    Green today. Kills K8 at the page.
- **Code (D2, D3).** In `startSession`, before the `Session` is built:
  - read `sessionStorage.getItem(key)`, where `key` is
    `` `vtt.viewpoint.${token}` ``;
  - pass it as `Session`'s third argument;
  - add `session.onViewpoint((id) => sessionStorage.setItem(key, id))`.
  One warning line, with a pointer to SPEC-022 and to VTT-443.
- **Order:**
  1. Write E1 and E2, and run `task e2e` on Task 1's bundle. They go red: E1 on
     "Perched on: Human Fighter" (the page reads nobody), and E2 on Bram's
     `chip sel` (Ash has it). This is items 1 and 2's "fails on today's tree".
  2. Add the four `beforeEach` lines, then A1-A6. A1, A2 and A5 go red.
  3. Make the `app.ts` change.
  4. Run `task build:client`. Then every client test, `task client:typecheck`,
     `task e2e` and `check-comments.py main` after staging.
- **True when:** all of the above are green, and `git status` lists the files
  above and `cmd/vtt/webdist/assets/index.js`.
- **Breaks,** each applied, rebuilt where the e2e is named, run, then undone and
  rebuilt:
  - **K1:** delete the `onViewpoint` line. A1 goes red, and E1 and E2 go red.
  - **K2:** `sessionStorage` becomes `localStorage` in both calls. A4 goes red,
    and E1's second page goes red (its URL names `act-fighter`).
  - **K3:** the key loses the token (`"vtt.viewpoint"`). A3 goes red.
- **Fixture checks:**
  - **F1:** in A1 remember `a1`, the fake's first by id, and apply K1. A1 must
    still go red, because it asserts the dial and not the chip. Undo it.
  - **F2:** read before running E2 that `FIRST` (`act-hand-a`) sorts before
    `SECOND` (`act-hand-b`). If they were swapped, E2 would be green today.

### Task 3: SPEC-022 (the specification step)

- **Do,** with the `specification` skill:
  - In "What the browser client holds", say:
    - a page keeps, in its tab's `sessionStorage` under its token, the actor
      each `ViewpointChanged` names (D3, D6);
    - until one arrives, every dial names that actor in `log` beside `after` 0
      (R1), and the page holds no log and shows nobody as watched (VTT-407);
    - a page whose own tab and token were sent no `ViewpointChanged` names
      none, whatever a page under another token in its tab or a page in
      another tab was sent (R2, R3);
    - the gateway judges the named log (SPEC-015).
  - Qualify SPEC-022's VTT-397 sentence: "Once a `ViewpointChanged` has
    arrived, a redial dials …".
  - Cite the owner's ruling of 2026-10-22.
  - Add E1, E2 and the new unit tests to the Status paragraph, R1 to R3 to the
    Requirements list, and one Consequence: a reload keeps the viewpoint, and
    nothing promises it past the tab.
- **True when:**
  - `python3 tools/check-requirements-chain.py` passes.
  - Each new sentence has been run against the code by someone other than its
    writer, Phase 4b's reading included.

### Task 4: the gate, whole

- **Do:** run `task check` whole on Task 3's commit, then `task e2e` on the same
  commit.
- **True when:** both are green, `check:drift` included, and
  `check:requirements-chain` resolves R1 to R3.

### Task 5: the report

- **Do:** write the implementation report under `docs/reports/`. It covers:
  - each "Done looks like" item with its observation (the table above);
  - K1-K8, F1 and F2, with what went red;
  - Q1's answer;
  - Phase 4a's adjudications under their own heading.

## For Phase 4a

Hand the QA agent:

- VTT-442 to VTT-444, VTT-397 and VTT-407;
- the signatures `new Wire(url, token, remembered?)`, `Wire.onViewpoint`,
  `new Session(url, token, remembered?)`, `Session.onViewpoint` and
  `boot(root)`;
- that the page reads its token from `localStorage` `vtt.token`.

Tell it:

- VTT-397 reaches a redial once a `ViewpointChanged` has arrived; before the
  first one, the dial's `log` is VTT-442's to VTT-444's (D5).

- bun runs every test file in one process with one happy-dom window, so it
  clears both storages in `beforeEach`. Every boot test shares the token `tok`
  (E1).
- a fake gateway answers what the test scripts, so the dial URL is the client's
  observable (D9).

## Forward coupling

- **B-3 (the third ticket)** takes the ticker from players and spectators. E1
  and E2 do not read it. The existing "a spectator chooses a shoulder and is
  shown that log" does read `.tick`, and changing it is B-3's work.
- **B-4 (the fourth)** lets a player who controls no party member watch as a
  spectator does. Such a player's remembered party member would then be opened
  if `MayWatch` accepts it. That fits rule 1's "while the participant may still
  watch it", and B-4's plan should know it.
- **S-2 (the seventh)** gives a player who lost their token a way back. A new
  token starts with no memory (VTT-443).
- **Two participants in two tabs of one profile** still share `vtt.token`, which
  is the ticket's first open point. The per-tab memory does not change that: a
  reload of the first tab comes back as the second participant, with no
  remembered viewpoint (VTT-443). This is not in scope. The owner may want a
  ticket for it.
