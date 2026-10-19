# The window shows what happened, and no log number: the change

**Ticket:** `docs/superpowers/specs/2026-10-19-the-window-shows-no-log-number-design.md`,
verified by an agent that did not write it (passes with gaps G1-G6).
**Plan:** `docs/superpowers/plans/2026-10-19-the-window-shows-no-log-number.md`,
written by that verifier and amended by the reading review of `1c549ea`, the
commit that carried both.
**The owner's rulings, 2026-10-19:**
- asked to start G11 ("kör G11"), whose recorded question was a player's
  `CommandResult.sequence`, he said the point was the window: "i själva
  'fönstret' som visar vad som har hänt för spelarna och dm, så visas inget
  'logg id/nummer' - utan det är bara texten";
- every log number stays inside the engine for tracing: "det enda som gäller
  är att vi 'döljer' lognummer i fönsteret ... Vi behöver ha alla lognummer,
  internt, så vi kan spåra problem och vad som har hänt";
- the purpose: "Nej en spelare ska inte se världsekvensen - för att en
  spelare inte ska kunna se från ett meddelande i chat fönsteret till ett
  annat att DM har gjort flera saker mellan de meddelanden som dyker upp i
  chatfönstret";
- asked whether a player's `CommandResult.sequence`, a world sequence seen
  only in the raw frames, should go too: "Bara fönstret";
- at sign-off: "Ja, kör", the e2e opening as the player ("Spelaren"), and a
  fold failure's reason left as the rule's exception ("Låt det vara").

These settle G11 of `docs/reports/2026-10-17-a-seat-streams-its-characters-log.md`,
which left a player's `CommandResult.sequence` to its own ticket: the window
shows no sequence, and the result keeps its own. That report stays as written.

**Last code commit:** `21648b0`, on `866bc28`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `1c549ea` | The ticket, the plan, VTT-393 |
| `21648b0` | Task 1: the ticker, its tests, QA's tests, the bundle |

## What the code did on `866bc28`

Every role's window is `renderSpectator`'s (`client/src/app.ts`), and its
"Events" ticker, `renderTicker` in `client/src/view/spectator.ts`, began each
row with a `span.seq` holding "#" and the entry's sequence: on a player's or a
spectator's screen their character log's, on the DM's the world's. The "Story"
feed showed text alone. The number was a decision of the client's first design
(`docs/superpowers/specs/2026-07-26-client-design.md`). The e2e test of the
ticker, "the story feed and ticker record what happened" in
`client/e2e/table.spec.ts`, was red on that tree: its spectator watches
nobody, a connection watching nobody is sent no events (SPEC-015), and
`task check` does not run the e2e.

## Done looks like, answered

1. [x] `client/test/spectator-view.test.ts` "each ticker row is its event's
   description alone, newest first" asserts each row's text is exactly its
   label, newest first, that its only attribute is `class` and that it has
   no child element. It was red on `866bc28`'s source, with the rest of Task
   1's tests written first.
2. [x] "the ticker shows the newest event first" and "each ticker row carries
   its description, under the Events heading" replace the two tests that
   named the number; "the feed and the ticker show a move's reason"
   (VTT-263's evidence) expects the label alone; `tickFor` in
   `client/test/qa-move-reason.test.ts` finds a row by its place. "the ticker
   is bounded — an all-night session must not render thousands of rows" is
   unchanged. All pass.
3. [x] "the story feed and ticker record what happened" opens as the player
   and passes: one row reads exactly "tok-lera placed at x,y", no row holds
   "#" and a digit, and no row's `::before` or `::after` has content. It is
   red on `866bc28`'s bundle at the exact row. `task e2e`: 16 passed, 1
   skipped (handover's `test.skip`).
4. [x] `git diff --stat 866bc28..21648b0 -- contract internal cmd` names
   `cmd/vtt/webdist/assets/index.css` and `index.js` and nothing else.
5. [x] `task check:drift` exited 0 after `21648b0`, and inside `task check`.
6. [ ] SPEC-007 gains it (the plan's D11) in the specification step that
   follows this report, in its own commit. The item stands.
7. [x] `task check`, whole, exited 0 on `21648b0` in 4501 s, launched
   detached; no floor was lowered and no ceiling raised. The figures are
   below.

## What the rule became

The ticket's one rule, with the exception the owner ruled at sign-off, became
**VTT-393**: "The client's window shows no log sequence number to any role,
except in the reason it gives for a log it cannot fold." Held by
`client/test/app.test.ts`'s "no role's window shows a log sequence number"
(spectator, player, DM and agent; text, `title` and `aria-label`; bare or
grouped spellings), item 1's test, the e2e, and four of QA's tests: "player:
neither the world sequence in a command's result nor the character log's own
reaches the window", "describe, the label feed and ticker show for an event,
names no sequence for any kind of event", and the `test.each` families "%s:
entries arriving one at a time after the first paint never show a sequence"
and "%s: a log that cannot fold shows the reason, and no sequence outside it",
the register's first citations of a `test.each` name.

The sort refused five candidates, listed in the plan's Requirements; that a
player's result names a world sequence is held already by
`TestThreeRoleExitScenarioOverLiveWebSockets`, whose "player-move-lera" step
has `issueAndVerify` compare the player's `result.Sequence` with the event the
DM and the agent receive.

## Phase 4a

One QA agent, given VTT-393, the rulings, SPEC-007 and SPEC-015 whole, the
client's declarations without bodies, the DOM support file and the harness
half of `client/test/app.test.ts` on main, and withheld the source, the
existing tests, the plan and the diff. It wrote 23 tests in
`client/test/qa-window-number.test.ts`, all passing. Of its 18 injections
into their own inputs and expectations, 17 reddened at least one test, and
the eighteenth, a `data-` attribute, is a control that stays green. One of
the 17, a hex spelling, passes the pattern tests and reddens only the test
that draws one log under two numberings.

### QA adjudications

- QA finding 1: the named exception — the fold's reason names the failing
  entry's sequence — no change; the owner's "Låt det vara".
- QA finding 2: behaviour defect outside VTT-393 — the client does not reset
  its fold at a later `ViewpointChanged` (SPEC-007, "What the connection opens
  with"), so a switch whose new log re-introduces a scene or a session the
  old one held replaces a player's or a spectator's window with "Cannot
  derive state from the log: …" — no change here; it is G8 of 2026-10-17,
  ticket 4's.
- QA finding 3: spec gap — whether an `event_id` is a "logg id" — no change;
  VTT-393 is about sequences, and no role's window shows an event id.
- QA finding 4: spec gap — spellings of a number other than decimal — no
  change; the test that draws one log under two numberings holds every
  spelling.
- QA finding 5: spec gap — counts or ordinals could tell a player the DM
  acted, though no sequence is shown — no change; nothing specifies them.
- QA finding 6: spec gap — a player's result's world sequence is not "an
  Envelope the connection was sent" — no change; QA tested it as the ruling
  reads, and it passes.
- QA finding 7: spec gap — a CSS `counter()` would be invisible to happy-dom —
  settled: `client/src/style.css` has no `counter`.
- QA finding 8: the DM's roster and join-link panels were not drawn, their
  routes unstubbed — no change; they are not drawn from the log.
- QA observation: a spectator whose opening `ViewpointChanged` names a party
  member still reads "Perched on: nobody" — G8, ticket 4's.
- Sentences needing an id: SPEC-007's reset at each switch — none allocated;
  ticket 4 is where it is held. The ordering sentence was QA's input only.

The ticket's writer then corrected three of QA's comments and added an
`actorControlRevoked` entry to its world log, at the Task 1 review's word.

## Phase 4b

- Task 0 (`1c549ea`): one reviewer, by command on copies of the tree. No Must.
  Shoulds: SPEC-007's draft said a player's result matches no event it is
  sent, but it can equal an unrelated entry's sequence (measured: result 8, a
  `SceneSeen` at 8, the player's own move at 9); a row given a `title` of its
  sequence passed T1 and T2; the ticket's rule lacked the owner's exception
  and its problem quoted the rulings in part; its search sentence and its
  MapTool sentence were inexact.
- Task 1 (`21648b0`): one reviewer, by command on copies of the tree, the
  bundle rebuilt there and identical. No Must. Shoulds: T2 matched only the
  bare spelling, so a DM-only number in a grouped spelling reddened only
  uncited tests; a `data-` attribute drawn by CSS `attr()` passed every test
  and the e2e.

## The verifier's gaps, G1-G6

- G1: "to any role" had no observation; T2 in `client/test/app.test.ts`.
- G2: a fold failure's reason names a sequence; the owner's exception.
- G3: a spectator's results carry no sequence, so item 6 names no
  spectator, and the SPEC-007 draft (D11) says `set_viewpoint`, a
  spectator's one command, answers with none.
- G4: item 3 is met by a role change, hiding the spectator's empty window
  from `task e2e` until ticket 4.
- G5: `spectator.ts` ends above its comment ceiling; a notice, accepted.
- G6: how long `task check` takes (item 7: 4501 s); its second point
  settled by the writer, as above.

## The deliberate breaks

Each in `client/src`, run against the whole client suite, restored by copy
and `cmp`; the three marked bundle were also built into `cmd/vtt/webdist` and
run through `client/e2e/table.spec.ts`, and after each the rebuilt bundle was
identical to the good one (`diff -r`). Six of the breaks were first measured
before QA's file existed, so every unit-suite red set below is the break run
again on a copy of `21648b0`'s tree, QA's file included, by the report's
reviewer, and the span, `describe` and bare console breaks again by the
writer; the e2e halves are the writer's runs. "QA's
eighteen" are every `qa-window-number` test except its four fold tests and
its `describe` test.

| Break | Red |
|---|---|
| the span put back (bundle) | T1, T2, the three rewritten ticker tests, three of `qa-move-reason`'s, QA's eighteen; the e2e at its exact row |
| `describe`'s session label prefixed with the sequence (bundle) | T1, T2, "each ticker row carries its description, under the Events heading", `describe`'s "describe renders a real label for every event kind it handles, not the fallback", QA's eighteen and its `describe` test; the e2e at "#" and a digit |
| `renderFeed`'s line suffixed with its sequence | T2, VTT-263's test, three of `qa-move-reason`'s, QA's eighteen |
| the console's session label with its start sequence | T2, `dm-view`'s "the session group names the open session and offers End", three of QA's |
| the same, in an en-US grouped spelling | T2, the same `dm-view` test, the same three of QA's |
| a row's `title` of its sequence, bare or grouped | T1 (by its attribute check, since the Task 1 review's fix), T2, QA's eighteen |
| a `data-seq` drawn by `.tick::before { content: attr(data-seq) }` (bundle) | T1; the e2e at its pseudo-element check |
| a number beside the fold failure's reason, in `app.ts` | QA's four fold tests |
| `renderTicker` without `.reverse()` | T1, "the ticker shows the newest event first", `qa-move-reason`'s mixed-log test |

## Rule 9: how MapTool does this

`MessagePanel.addMessage(final TextMessage message)` renders the message's
text, and a `TextMessage` holds `channel` (an `int`, the channel's code),
`target`, `message`, `source` and `transform`, and no sequence. Every method
of `ServerCommand` returns `void`. The window here now does what MapTool's
does; the sequences it has no equivalent of stay on the wire.

## Figures

From the one `task check` on `21648b0`:

- Go coverage: 21 packages at or above their floors; the gate notes 11 that
  clear theirs by more than 1.0pp.
- Client coverage: 22 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors. `internal/gateway`
  was mutated again (1 survivor, adjudicated); the other 14 reused their last
  passing verdicts, which hold the run's 7 timed-out mutants.
- Client mutation: re-mutated in its Linux container, Stryker's run 36
  minutes 31 seconds: 2940 mutants, 2864 killed, 9 timed out and counted as
  killed, 67 survivors all adjudicated equivalent.
- Race: every package passes.
- Requirements chain: 393 rows, 267 test files, 15 specifications.
- Comments: 305 files, no comment line added; two notices,
  `client/src/view/spectator.ts` (49.84 against 49.6) and
  `internal/gateway/export_test.go` (49.18 against 45.9).
- Free disk: 24.4 GiB as the Go mutation gate began, 23.3 GiB at the end.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| VTT-393 held by T1 and T2 | Also by four of QA's tests | Two (the player's result, `describe`) cover paths T1 and T2 do not reach; the two `%s` families joined at the Task 1 review, which showed a DM-only number in a grouped spelling and a number beside the fold reason reddened only uncited tests |
| T2 finds a sequence by its bare spelling | Bare or grouped in thousands | The same review: "7,001" passed T2 |
| T1 reads each row's text | Also its attributes and children; the e2e reads `::before` and `::after` | The plan's requirements sort refused a "no `data-` sequence" rule as how rather than what, since neither hover nor a screen reader shows a `data-` attribute, but CSS `attr()` draws one, and that passed every test |
| The e2e is no evidence | It cites VTT-393 and is in its row | It is the one check that sees the shipped bundle's window |
| QA's file committed as written | Three comments corrected, one fixture entry added | The Task 1 review: its header's "five digits" and "an Envelope the connection was sent" were false of two tests; `recordingContext`'s "answers nothing" was false, since it answers `measureText` and `getLineDash`; its world log lacked `actorControlRevoked`, so "of nearly every kind" became "of every kind" with the entry |
| QA given SPEC-007's and SPEC-015's named sections and `app.test.ts` at main, and withheld T1, T2, the plan, the verdict and four files' bodies | Both specifications whole, `app.test.ts`'s harness half alone, and every existing test and every body in the client's source withheld | The dev-cycle's Phase 4a withholds the existing tests and the implementation source, which is more than the plan withheld; giving the specifications whole was a preference, and leaves out nothing the plan gave |
| The plan's D3: the search finds only `style.css`'s rule and the span | It also finds `feed.ts`'s `a.seq` | A property, as D3's next sentence says; the conclusion holds |

## What could not be established

- Whether counts or ordinals in the window let a player tell that the DM
  acted (QA finding 5). Nothing specifies them, and no test looks.
- Whether a style rule shows a number anywhere but the ticker: the unit
  tests run in happy-dom, which computes no styles, and the e2e reads the
  `::before` and `::after` of the ticker's rows only.

## What was deliberately left out, and where it went

- A player's `CommandResult.sequence`: stays, by "Bara fönstret".
- The fold failure's reason: stays, by "Låt det vara"; ticket 4 removes the
  switch that reaches it today.
- The client's reset at a later `ViewpointChanged` (QA finding 2) and the
  spectator's "Perched on" label: G8, ticket 4.
- `04-spectator-populated` left the e2e's screenshots, and the spectator's
  empty window has no red test in `task e2e` until ticket 4.
- `client/src/view/spectator.ts` stands at 49.84 against its 49.6 comment
  ceiling with no comment added; the next change that adds a comment line to
  it must bring it under. `internal/gateway/export_test.go`'s notice was on
  `main` before this ticket.
