# A refused command keeps what was typed, and the DM's speaker stays until changed

## The problem

**The console and the player panel forget what was typed when they send, not
when the server accepts it.** In `renderDMConsole` in
`client/src/view/dm.ts`, each button that sends a command calls `d.send(...)`
and then `clearDraft(...)` in the same turn, without waiting for the
`CommandResult` that `DMDeps.send` resolves to: Start session
(`session-name`), Add actor (`actor-id`, `actor-name`, `actor-kind`), Remove
actor (`actor-id`), Add from JSON (`actor-json`), Place token (`token-id`,
`token-scene`, `token-actor`, `token-x`, `token-y`), Remove token
(`token-id`), a row's Describe (its four `describe-*:<token>` fields), Save
note (`note-visibility`, whose select it also sets to `""`), and Grant
(`grant-<actor>`, `grant-kind-<actor>`). `clearDraft`'s doc comment says it
clears "after the command it fed succeeded"; it runs before any answer. The
player panel's `submit`, in `renderPlayerPanel` in
`client/src/view/player.ts`, empties `say-text`, `ui.sayAs` and `ui.sayText`
and repaints right after `send`, whose parameter type there is
`(c: ClientCommand) => void`. `client/src/app.ts` passes both the panel and
the console `act`, which resolves to the `CommandResult` and shows a refusal
as the toast `refused: <error>`. So a refused command's text is gone and has
to be typed again. `Wire.send` in `client/src/wire.ts` answers a command sent
on a closed socket with `ok: false` ("wire: not connected", "wire: connection
closed before a result arrived"), so a line sent while the connection is
down is lost the same way. `client/test/dm-view.test.ts`'s "a REFUSED start
still clears the name — the text is gone with the command" pins the
behaviour.

**The DM's speaker box empties at every Send.** `renderPlayerPanel` draws
`say-as`, "in character as… (optional)", when `speaksFreely`, the panel of a
DM or an agent; a player's has none (VTT-500). `submit` empties it with the
text, so a DM voicing one character over several lines types its name again
before each one. A DM's panel draws the Say form only while they control
an actor: with none, `renderPlayerPanel` draws "You do not control an actor
yet." and no Say form.

**Eight comments say a mutant is visible in a real browser, and it is
not.** `el()` in `client/src/view/dm.ts`, `client/src/view/spectator.ts` and
`client/src/view/player.ts` each carries `// Stryker disable next-line
ConditionalExpression` above `if (text !== undefined) n.textContent = text`,
with a reason saying that without the guard a real browser writes the word
"undefined" above every container while happy-dom writes nothing.
`client/test/dm-view.test.ts`'s "a group's row holds only its elements, with
no stray text" says that guard forced true leaks text into containers, and
its "a group carries no stray text of its own" says it renders the literal
word "undefined". `tools/ts-mutation-equivalents.txt` says the same twice,
in the sentence under `client/src/view/dm.ts`'s header that begins "Two
further mutants, el()'s" and in the one under `client/src/view/spectator.ts`'s
that begins "One mutant is handled in the SOURCE". The header comment of
`client/src/view/join.ts` says it too: happy-dom drops an undefined
`textContent` and a real browser renders the word `"undefined"`. Measured
in Chromium 143 through Playwright: setting `textContent` to `undefined` on
a `div` that holds a `span` leaves `""` and no child nodes; `null` on an
empty `div` leaves `""`; happy-dom gives the same. `textContent` is a
nullable `DOMString`, so `undefined` reaches it as `null`. The mutant changes
nothing in either, so it is equivalent. `docs/verification-debt.md`'s open
entry "A dm-view comment says el()'s guard forced true leaks text" names
only the comment of "a group's row holds only its elements, with no stray
text".

**How MapTool does this (rule 9).** In `~/dev/RPTool/maptool` at `f4b7fef6c`,
`CommandPanel.commitCommand()` adds the line to `commandHistory`, sends it,
and sets `commandTextArea` to `""`, so the box empties at send; the line can
be recalled from the history with the arrow keys. A chat line there is not
refused by a server, so MapTool has no answer for a refused one. The speaker
is the impersonated token, `CommandPanel`'s `TokenIdentity`, kept until it is
changed or cleared. This ticket borrows the speaker that stays; the box that
empties at send it does not borrow, because here the server can refuse a line.
The history is not taken up (see the last section).

## Done looks like

1. A client test types `Night One` into the console's `session-name`, clicks
   Start session with a `send` that answers `ok: false`, and finds
   `session-name` holding `Night One` at the paint after the answer. Today it
   is empty: "a REFUSED start still clears the name — the text is gone with
   the command" asserts so, and goes.
2. The same test with a `send` that answers `ok: true` finds `session-name`
   empty at the paint after the answer.
3. For each other form named in the problem, a client test finds every field
   it clears today as typed after a refused answer, and after an accepted
   one as it is drawn with nothing typed in it: empty, the blank option, or
   for a Describe box the token's present text. Save note's key, title and
   text, which no answer clears today, are as typed after either.
4. A client test types `I draw my sword` into a player's `say-text` and
   presses Send with a `send` that answers `ok: false`: the next paint shows
   `I draw my sword` in `say-text`. Answered `ok: true`, it shows `say-text`
   empty. The same holds for `/ooc where is the pizza`.
5. A client test on a DM's panel types `Grimble` into `say-as` and `Hello`
   into `say-text` and presses Send, answered `ok: true`: the next paint shows
   `say-as` holding `Grimble` and `say-text` empty, and a second line, `Bye`,
   sent without touching `say-as`, carries `as` `Grimble`. Today `say-as` is
   empty and the second line carries no `as`.
6. A client test presses Send twice, or Send and then Enter, before the first
   answer arrives: one command is sent.
7. A browser test under `client/e2e/` sends a console command the server
   refuses and finds its fields as typed beside the toast `refused: …`, and has
   a DM send two lines in character with one `say-as`.
8. `git grep -n -E 'word "undefined"|browser stringifies|harness-blind|both leak|literal word|renders the word' -- client/src client/test/dm-view.test.ts tools/ts-mutation-equivalents.txt`
   prints nothing; today it prints a line in each of the eight places. None
   of the eight says the mutant is visible in a real browser, which a
   reading checks: the two paragraphs of the equivalents file say the guard
   forced true leaves no text in Chromium or in happy-dom, the three el()
   comments carry a warning, and the two test comments say what each test
   holds, and `join.ts`'s header keeps only its reason for not using el().
   The three `// Stryker disable next-line ConditionalExpression`
   before `if (text !== undefined) n.textContent = text` remain, and the
   verification-debt entry is closed.

## Rules this puts on the system

- A box or field of the console or the player panel that a command reads
  keeps what was typed until the server accepts that command; a refusal, or
  no connection, leaves it as typed. This withdraws VTT-493 (the plan's
  D13).
- A command the server accepts clears what was typed in the fields that fed
  it that the console or the panel clears today, except the speaker box.
- A DM's or an agent's speaker box keeps what it holds after Send, until it is
  changed.
- One press of Send, or of a console button whose command clears a field
  today, sends one command, however often it is pressed before the answer.

The corrected comments add no rule.

## What it touches

One ticket and one branch, the comments first (the owner's ruling "Ett
ärende, en gren", 2026-10-29). It names more than ten files across the
client, its tests, `tools` and `docs`; the owner was asked how to split it
before it was written.

1. The comments: `el()` in `client/src/view/dm.ts`,
   `client/src/view/spectator.ts` and `client/src/view/player.ts`; the header
   of `client/src/view/join.ts`; the two tests in
   `client/test/dm-view.test.ts`; the two paragraphs in
   `tools/ts-mutation-equivalents.txt`; the entry in
   `docs/verification-debt.md`.
2. The console: the handlers named in the problem, and `clearDraft`, in
   `client/src/view/dm.ts`; tests in `client/test/dm-view.test.ts`, among them
   "a REFUSED start still clears the name — the text is gone with the
   command", "removing a token sends its trimmed id and clears the field" and
   "removing an actor sends its trimmed id and clears the field".
3. The panel: `submit`, `PlayerUIState` and `renderPlayerPanel`'s `send` in
   `client/src/view/player.ts`, and what `client/src/app.ts` hands it; tests
   in `client/test/spectator-view.test.ts` ("Send empties both Say boxes for
   the paints after it", "sending narration requires text, and clears the box
   afterwards", "Send narrates the trimmed text, clears the box and
   repaints"), `client/test/qa-say-boxes.test.ts` and
   `client/test/qa-speaking.test.ts`.
4. A browser test under `client/e2e/`.
5. VTT-493's row in `docs/requirements.md` and the rows that cite the tests
   above.
6. `cmd/vtt/webdist`, rebuilt by `task build:client`.

Any change under `client/src` moves the client's mutation-report input hash,
so after this ticket `task check` reports the stored client mutation report
out of date until the owner asks for a full run (SPEC-023).

## Specifications this moves

docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- Firefox and WebKit: `playwright.config.ts` names no browser project, so the
  e2e runs Playwright's default, Chromium, alone, and what `textContent =
  undefined` does in the other two was not measured. The DOM standard's
  nullable `textContent` gives them the same answer.
- How often the gap between a send and its answer is long enough to type
  in was not measured. The owner's answer to the plan's Q1, "Töm ändå", is
  that an accepted answer empties a box typed into in that gap as well.
- A DM who controls no actor has no Say form, so the speaker rule is seen
  only on a DM's panel that has one. That the DM's browser console has no
  Say form was raised to the owner by
  `docs/reports/2026-10-26-a-player-speaks-as-their-character.md`, under
  "What was deliberately left out"; it is not this ticket.
- MapTool's command history, recalling a sent line with the arrow keys, is
  not taken up here; it would be a ticket of its own.
- The console's controls that clear nothing today (End session, Revoke, the
  join door, rotating the join link, promotion, Delete note, a condition's
  ✕, map and adventure loading) are not changed.
