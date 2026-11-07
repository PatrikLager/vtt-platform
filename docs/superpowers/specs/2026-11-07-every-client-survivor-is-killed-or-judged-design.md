# Every client mutant that survived at 37929bf is killed or adjudicated equivalent

## The problem

**The stored client mutation report is current and fails the step that
judges it.** `reports/mutation/mutation.json` and its stamp
`reports/mutation/ts-mutation-stamp.json` (both gitignored, so they exist only
in the checkout that ran `task check:ts-mutation:full`, here
`~/dev/vtt-platform-visibility`) measured `37929bf` from a clean tree.
`python3 tools/ts-mutation-report.py check`, the command `check:ts-mutation`
runs as a step of `task check`, says the report is current, judges it with
`tools/check-ts-mutation.py`, prints a `SURVIVED` line for each mutant below
and exits 1. No entry in `tools/ts-mutation-equivalents.txt` claims any of
them.

They sit in code written by four tickets: the whispering
(`2026-11-01-whispering-to-and-from-characters`), a refusal that says why
(`2026-11-02-a-refusal-says-why`), Boot
(`2026-10-31-the-dm-boots-a-participant`) and a companion's sheet
(`2026-11-06-a-companions-sheet-is-seen-with-the-companion`). They fall in
two kinds.

**Mutants no test can tell apart, all in `readLine` in
`client/src/player.ts`.** `t` is `raw.trim()`, so no capture of it ends in
whitespace, and a greedy `\s+` or `\s*` before a capture leaves no whitespace
at its start; `\s` and `String.prototype.trim` strip the same characters.

- `/^\/(?:dm|gm|togm)(?:\s+|$)(.*)$/is` with its final `$` dropped, and the
  `/w` pattern `/^\/(?:w|whisper)(?:\s+|$)(.*)$/is` with its final `$` dropped:
  `.*` under the `s` flag always reaches the end of the input, so `$` after it
  always holds.
- The `/dm` pattern with `\s+` as `\s`: the capture may then start with
  whitespace, and `dm[1]!.trim()` strips it.
- `dm[1]!.trim()` as `dm[1]!`: the capture has no whitespace to strip.
- `close < 0` as `close <= 0`: `close` is `rest.indexOf('"', 1)`, which is
  never 0.
- `/^(\S*)\s*(.*)$/s` with its `^` dropped: `exec` tries index 0 first, and
  the pattern matches there for every input; with its final `$` dropped, for
  the first reason; with `\s*` as `\S*`: the name is the same greedy run, and
  `text!.trim()` strips the whitespace the text then starts with.
- `text!.trim()` as `text!`: the text has no whitespace to strip.

**Mutants a test can kill, which no test does today.**

- In `readLine`: the `/dm` pattern with its `^` dropped, which makes a line
  with `/dm` later in it a whisper to the DM (VTT-592 says the first word);
  `rest.slice(1, close).trim()` without the `trim()`, which keeps the spaces
  inside a pair of double quotes in the name; `rest.slice(1).trim()` without
  the `trim()`, which keeps the spaces after an opening double quote that
  nothing closes. VTT-592 says a `/w` line whispers to the next word or to the
  words between a pair of double quotes; for an unclosed quote the code reads
  the rest of the line after the quote as the name, which neither half says,
  so VTT-592 is false as worded there.
- In `whisperTargets` in `client/src/view/dm.ts`: `name === ""` as `false`
  and its `""` as `"Stryker was here!"`, and `named.length > 1` as `false`.
  On the lines the tests of VTT-594 send, each still refuses, with another
  refusal's reason, and those tests check only that a reason was given.
- In Boot's `confirm` in `client/src/view/dm.ts`: the second and the third
  string of its question emptied. The test of VTT-569 checks only that the
  question holds the participant's name.
- In `renderPlayerPanel`'s `submit` in `client/src/view/player.ts`:
  `line.kind === "toDM"` as `true`, which tells a blank line, or `/ooc` with
  nothing after it, the empty `/dm` hint.
- In `boot` in `client/src/app.ts`: `me?.role` as `me.role` in the
  `onPartyMembersLeft` handler, which throws when a set drops a party member
  before `/api/me` has answered, or when it never does. The throw leaves the
  rest of `Session`'s handler unrun, so the window goes on offering the set
  before it, which VTT-457 forbids.
- In `Session`'s `onWatchable` handler in `client/src/session.ts`: the `[]`
  that keeps nothing when a set says the participant controls no party member
  as `["Stryker was here"]`, which keeps a party member whose id is that
  string. The fold bounds an actor id in bytes and nothing else, so such an
  id can be folded.
- In `reintroductions` in `client/src/view/feed.ts`: `back.case !==
  "actorAdded"` as `false`, which throws when the entry after a bare
  `actorRemoved` has no payload this client can read. The fold passes such an
  entry over, so the Story is built from a log that holds one.

**Of the second kind, mutants only a direct call can tell apart, with an
argument no caller in `client/src` passes.** The `client/src/view/player.ts`
section of `tools/ts-mutation-equivalents.txt` records a mutant of that kind
killed with a state built past the fold rather than excused, and keeps no
entry; these are killed the same way, by tests that hold no row.

- In `renderPlayerPanel`'s `submit`: `notify?.(...)` as `notify(...)` at the
  `/w` hint and at the empty `/dm` hint. Each throws when the panel is given
  no `notify`, and happy-dom reports the throw through the window's `error`
  event. The one caller, `paint` in `client/src/app.ts`, always passes one.
- In `reintroductions`: `back.value.actor?.actorId` as
  `back.value.actor.actorId`, which throws when the entry after a bare
  `actorRemoved` is an `actorAdded` with no actor. The fold refuses such an
  entry ("actor added with no actor or empty id"), and `paint` then shows the
  refusal and builds no Story, so only a direct call of `buildFeed` is handed
  that log.

## Done looks like

1. Each mutant of the first kind has an entry in
   `tools/ts-mutation-equivalents.txt`. Against the stored report of
   `37929bf`, `python3 tools/ts-mutation-report.py check` prints no
   `SURVIVED` line for any of them, and with any one entry removed from a
   copy of the file, `python3 tools/check-ts-mutation.py <copy>` prints
   `SURVIVED` for that mutant. On `37929bf` it prints one for each.
2. The entries are true of the code. In a copy of the tree outside the
   repository, each mutation of the first kind applied in turn leaves
   `bun test client/test` passing.
3. Each mutant of the second kind fails a test this ticket adds. With the
   mutation applied by hand, a named new test in `client/test` fails; on
   the unmutated tree it passes. On `37929bf` the same mutation leaves
   `bun test client/test` passing.
4. `task check:ts-mutation:full` on the branch, run on the owner's word,
   reports no unadjudicated survivor.
5. `task check` exits 0 on the branch.

## Rules this puts on the system

- The player panel sends nothing, and tells the player nothing, for a line
  that is blank or that is `/ooc` with nothing after it.
- Boot's question to the DM says that the participant's sign-in stops working
  for good, that they no longer control any character, and that they can come
  back only through the share link while it is open.
- VTT-592's reading of a Say line, with the unclosed quote said: a line whose
  first word is `/w` and whose next word opens with a double quote that no
  later one closes reads as a whisper to the rest of the line after that
  quote, trimmed, with no text. VTT-592 is withdrawn, succeeded by a row that
  says all of it.

The two direct-call tests hold no row. The other tests hold rules already in
the register: VTT-457, VTT-594, VTT-605, VTT-606 and VTT-662.

## What it touches

- `tools/ts-mutation-equivalents.txt`: entries in its `client/src/player.ts`
  section.
- `client/test/player.test.ts`, `client/test/dm-view.test.ts`,
  `client/test/spectator-view.test.ts`, `client/test/app.test.ts`,
  `client/test/session.test.ts`, `client/test/feed.test.ts`: new tests.
- `docs/requirements.md`: the new rows, VTT-592 withdrawn, and the new tests
  as evidence on the rows they hold.
- `client/src/player.ts` (`readLine`'s doc comment), `client/test/player.test.ts`
  and `client/test/qa-whisper-say.test.ts`: VTT-592's citations pointed at its
  successor. Nothing else under `client/src` changes.
- `docs/reports/2026-11-07-every-client-survivor-is-killed-or-judged.md`: new,
  the implementation report.

An edit under `client/test` or `client/src` makes the stored report out of
date, so the order is the entries first, while the report still judges them,
and the tests after.

## Specifications this moves

- docs/specifications/022-a-client-holds-one-log.md
- docs/specifications/009-identity-and-joining.md

## What could not be established

- Whether the mutants survive on another machine. The report is local and
  gitignored; item 2's and item 3's hand mutations are the part another
  machine can repeat.
- The run's timed-out mutants, in `client/src/view/canvas.ts`,
  `client/src/view/scene-plan.ts` and `client/src/wire.ts`. The checker
  counts them as killed and says the count measures the machine; this ticket
  does not judge them.
- Whether item 4 runs. The full run happens on the owner's word.
