# The console's Remove token asks the DM first — report

Ticket: `docs/superpowers/specs/2026-11-22-remove-token-asks-first-design.md`.
Plan: `docs/superpowers/plans/2026-11-22-remove-token-asks-first.md`.
Branch `fix/remove-token-asks-first` from main `6d07bdf`. Commits: `6a9f27b`
(Task 0: ticket, plan, register) and `209964b` (Task 1: the question, its
tests, the comment ledger and the rebuilt bundle; the last code commit). Every
code reference below is to `209964b`. SPEC-022 moves next, written from this
report.

## Done looks like

1. [x] **Remove token asks first.** Remove token's press in
   `client/src/view/dm.ts`, inside the press `sending` runs and after the empty
   check, asks `Remove <token> from the board? Its place, names and notes go
   with it.` through `DMDeps.confirm`, the token shown by its GM name, else its
   actor's name, else its name, else the trimmed id, read from the console's
   fold; a no returns nothing, a yes returns `remove_token` as before.
   - `client/test/dm-view.test.ts`'s "Remove token asks first …" (VTT-952),
     "a Remove token the DM declines keeps the token field as typed, at the
     next paint too" (VTT-954) and "removing a token sends its trimmed id and
     clears the field", which now asserts one question (VTT-953).
   - Independently: `client/test/qa-remove-token-asks-first.test.ts`.
   - On `6a9f27b` the VTT-952 and VTT-954 tests failed (nothing asked, a send
     on a no), and the existing Remove token test failed on its new question
     count.
2. [x] **An empty field still asks nothing.** "each guard refuses with its own
   exact wording" and "a whitespace-only id is refused everywhere one is
   required" in `dm-view.test.ts` assert that no question is asked (VTT-955),
   the second newly for its Remove token arm; the QA file's blank-field tests
   likewise.
3. [x] **The comment above it is a warning or a pointer.** It reads: ask inside
   the press `sending` runs, after the empty check, so a press `sending` holds
   back or the check stops asks nothing; then the ids it holds. The history and
   description it held are below.
4. [x] **The gate.** `task check` and `task e2e` are green on `209964b`.
   `cmd/vtt/webdist` is rebuilt in that commit, and `check:drift`, a step of
   `task check`, finds no difference. `check:comments` is clean with the
   ledger row the commit lowers. The Go mutation gate measures no client
   code, and `check:ts-mutation` judged no client report: the stored one
   measured `b90fc1a` and is out of date (SPEC-023).

## What the rule became

The ticket's rule, "the console's Remove token asks the DM first, in a message
naming the token, and sends nothing when the DM declines", became, after the
sort at Task 0:

- **VTT-952.** Remove token, pressed with a token id that is not blank while no
  `remove_token` it sent is unanswered, asks first, in a message holding the GM
  name the console's state holds for that token, else the name it holds for
  the token's actor, else the token's name, else the id, and sends nothing on
  a no.
- **VTT-953.** A Remove token the DM accepts sends one `remove_token` naming
  the trimmed id.
- **VTT-954.** A Remove token the DM declines leaves the token field as typed,
  at the next paint too.
- **VTT-955.** Remove token, pressed with the field empty or white space while
  no `remove_token` it sent is unanswered, shows `name the token to remove`,
  asks nothing and sends nothing.

The lock clause, whose absence withdrew VTT-946 and VTT-949 in the precedent,
was drafted into VTT-952 and VTT-955 from the start; no row needed
withdrawing. Refused by the sort, as the precedent's was: the question's words
as a row (the tests pin them), and a row that a press the lock holds back asks
nothing (a consequence of where the question sits; `dm-view.test.ts`'s
"a Remove token pressed again while its command is unanswered asks nothing and sends nothing" cites, by a comment,
VTT-523 for "sends nothing"); with the candidates the plan lists.

## Rule 9

MapTool asks before deleting tokens, by default: `msg.confirm.deleteToken`,
behind the preference `tokensWarnWhenDeleted`, default on, which its dialog's
"Yes, don't show again" switches off. Borrowed: asking before a token is
removed. Not borrowed: the switch (OD-3): the client keeps `window.confirm`,
which has no third answer, and holds no settings store to turn the question
back on.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket as verified | Its problem statement corrected before Task 0's commit | The verifier found it called Remove token "the one removal left" that asks nothing, where deleting a note, removing a condition, taking an ability and revoking control do not ask either; and that a player's window draws no line for the token going out of sight (VTT-823) |
| Task 1: a fixture over `newState()` with a named actor and a nameless one | `tokenTable()` is built on `leraTable()`, the file's existing fixture, which starts from `newState()` | It already holds a named actor; its tokens cover each step of the naming order, an actor the fold does not hold, and an unknown id |

## History cut from the code (rule 10)

Above Remove token in `dm.ts`, removed: "Takes a piece off the board for good
(retraction-leaves Task 8, spec §5.1) — the DM control this branch's own Task 3
left without one when the Undo group came out. Shares tokId with Place rather
than a field of its own: naming a token to remove needs nothing else."

## The deliberate breaks

Each applied, seen red, and restored from a saved copy compared byte for byte:
the question dropped (the VTT-952, VTT-953 and VTT-954 tests red); asked before
the empty check (both guard tests red, the whitespace test through its new
assertion); asked outside the press `sending` runs (that held-press test red);
always the id; `??` in place of `||` inside and at the end of the name chain;
an untrimmed lookup; the token's name before the actor's; the token's name
dropped; a non-optional actor lookup (each the VTT-952 test red); the field
cleared on a no (the VTT-954 test red); an untrimmed send (the VTT-953 test
red).

## Phase 4a adjudications

No findings. Its tests in `client/test/qa-remove-token-asks-first.test.ts` land,
covering each step of the naming order, an actor the fold does not hold and an
id it holds no token of. Its assertion that a blank press the lock holds back
shows no notice carried VTT-955, which is worded for no unanswered command:
that citation is dropped. Tests it names by a template (`naming: ${label}`, and its blank-field tests,
`${label} field shows the notice …`) cannot be cited by name in the register;
each arm of VTT-952 is also held by a named test in `dm-view.test.ts`, and
VTT-955 by the guard tests there. With the question dropped from `dm.ts` (and
restored), the tests that need a question went red and the blank-field ones
stayed green. Task 1's reading review asked that its token fixture hold the
empty names the fold writes; its `token()` was edited after delivery so that
the names default to `""`.

## The plan's gaps

- **G1.** The whitespace test's Remove token arm did not check that nothing is
  asked; it does now.
- **G3.** `tools/comment-ceilings.txt`, the QA file and the report, beside the
  ticket's list; and the adjudication in `tools/ts-mutation-equivalents.txt`
  keyed on Remove token's line, which is byte-identical, so it holds.
- **The ticket's open points.** The naming order (OD-1 A: the first two steps of
  the DM's board's order, VTT-484, then the token's name and the id), the words (OD-2 A) and the lock: the question sits inside
  the press `sending` runs, so a held-back press asks nothing.

## What could not be established

Nothing was left open.

## What was deliberately left out

- An e2e test: no spec presses Remove token, and the `window.confirm` wiring
  is the one Boot, Make spectator and New link share, each driven through a
  real dialog by `booting.spec.ts`, `handover.spec.ts` and `joining.spec.ts`.
- The agent's removal over MCP; deleting a note, removing a condition, taking
  an ability and revoking control, which do not ask either.
- Telling apart several tokens of one actor with no GM name, which read alike
  in the question; the board labels no square.
- The client's full mutation run: the stored report measured `b90fc1a`.
