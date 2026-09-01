# What check:ts-mutation gates

`stryker.conf.json`'s `mutate` list is the gated set. As of 2026-08-03 it is
**every file under `client/src/`** except the browser entry point, and this
file records how that was reached and what it cost, so the next person working
a mutation backlog can copy the parts that worked.

The rule this follows is the one in tools/check-mutation.py: a gate scoped to
what its author happened to touch is not a gate. The full client WAS measured
before any scoping — 1667 mutants, **63.89% mutation score with 602
survivors**, against 95.91% line coverage. That gap is the entire argument for
this gate existing, and closing it took two passes.

## Gated

| file | before | survivors worked | adjudicated equivalent |
|---|---|---|---|
| `client/src/fold.ts` | 69.51% | 93 → 0 | 14 |
| `client/src/view/dm.ts` | 40.71% | 166 → 0 | 1 (+1 disabled in source) |
| `client/src/player.ts` | 75.76% | 22 → 0 | 15 |
| `client/src/view/spectator.ts` | 71.62% | 63 → 0 | 1 (+1 disabled in source) |
| `client/src/view/player.ts` | 50.83% | 89 → 0 | 0 |
| `client/src/app.ts` | — | 48 → 0 | 1 |
| `client/src/view/feed.ts` | — | 42 → 0 | 15 |
| `client/src/wire.ts` | — | 22 → 0 | 2 |
| `client/src/view/grid.ts` | — | 14 → 0 | 7 |
| `client/src/commands.ts` | — | 11 → 0 | 0 |
| `client/src/metadata.ts` | — | 11 → 0 | 0 |
| `client/src/auth.ts` | — | 2 → 0 | 1 |
| `client/src/session.ts` | — | 1 → 0 | 0 |
| `client/src/state.ts` | — | 0 | 0 |
| `client/src/view/scene-plan.ts` | 100% lines | 33 → 0 | 1 |
| `client/src/view/canvas.ts` | 100% lines | 32 → 0 | 1 |
| `client/src/view/pack-assets.ts` | 100% lines | 2 → 0 | 0 |
| `client/src/view/camera.ts` | 100% lines | 1 → 0 | 0 |
| `client/src/join.ts` | — | 0 | 0 |
| `client/src/view/join.ts` | — | 0 | 0 |

`client/src/undo.ts` HAD A ROW HERE — 78.75% before, 17 survivors worked to 0,
2 adjudicated equivalents — until `d3e2f28` deleted the file (sub-project 13,
spec `2026-08-30-retraction-leaves`; `133e896` took `campaign.Undo` on the Go
side the same day). Its two entries left `tools/ts-mutation-equivalents.txt`
with it. The measurement is
kept in this sentence because it is part of the record of how the backlog was
worked; it is out of the table because the table says what is GATED TODAY, and
a row for a file `stryker.conf.json` cannot mutate reads as a gate that is
still watching something. (Removed from the table 2026-09-01.)

`client/src/main.ts` is the only file with no row, and it is the only file
excluded from `mutate`: it is the browser entry point, wiring rather than
logic. It still carries a normal 100.00 line-coverage floor.

**This table is now every gated file, checked mechanically rather than
believed** — 21 gated files, 21 rows. It was not before: four files had no row
while being gated, which is how 68 survivors went unrecorded.

The last six rows were added 2026-08-25 and the first four of them had **no
row at all** until then — not because they were out of scope, but because the
`mutate` glob gated them the moment the maps and visibility arcs created them,
so nobody ever ran the work that produces a row. Their "before" column is not a
mutation score but a LINE-COVERAGE floor, and that is the point: all four sat
at 100.00% line coverage while carrying 68 survivors between them. Every line
ran; nothing checked a boundary.

### 2026-08-25: 125 survivors, and what actually caused them

The gate stopped testing mutants on 2026-08-11 and nobody could tell, because
the native task self-skips on macOS (kernel bug FB21686886) and CI stopped
running per-PR when the Actions allowance was exhausted. A test that read
`app.ts`'s own SOURCE and asserted the text `params.get("join")` broke Stryker's
dry run — instrumentation rewrites string literals, so the text it looked for
stopped existing. One failing assertion, zero mutants tested, for two weeks.

Repaired 2026-08-25 (`69434ef`), which surfaced 125 unadjudicated survivors:
**57 REGRESSION** in seven files this table already recorded as `→ 0`, and
**68** in the four never-worked files above. Closed at 113 killed and 11
adjudicated, plus one that was neither: a mutant both unkillable and not
equivalent, the combination the equivalents file has no shape for, so `fb6b2d8`
restructured the source instead.
Latest green run, 2026-08-25 14:08 (recorded in `76e45c2`): **2777 mutants,
2673 killed, 31 timed out, 73 adjudicated equivalent, zero unadjudicated
survivors** — against 1772 mutants at the previous green run on 2026-08-07, so
the client grew 57% and all of that growth was ungated while the gate was
silent.

"Latest" rather than "final", because it is a measurement with a date on it and
this line has already been a run behind once: the earlier 2764/2660 figures
were `3c01695`'s, taken thirteen hours before the merge re-ran the gate over a
larger tree.

**And it went a run behind again.** Runs on 2026-08-27, 08-28 and 08-29 over
BYTE-IDENTICAL inputs read 94, 322 and 31 timed out against this row's 31 —
every other column unchanged. The timeout count measures machine load, not the
client, so this row is the floor rather than the current figure. What that cost
a reader, and why both checkers stopped advising a test change on the strength
of one run, is in `tools/mutation-scope.md`.

**One root cause dominates, and it is worth internalising: a fixture chosen so
that two different operators produce the same answer.** Every camera in the
suite was `fitCamera()` at exactly scale 1 and offset 0, where `*scale` IS
`/scale` and `+offset` IS `-offset` — that single degeneracy hid 18 arithmetic
mutants. Every Y coordinate in the fold suite was 0, so `?? 0` and `&& 0` were
indistinguishable. Every perch fixture had exactly two party members, and a
two-element sort calls its comparator once, so "always before" and "always
after" each score 50%. Five separate instances of the same shape, all in files
at or near 100% line coverage. **The test looks specific and cannot fail** —
which is precisely the thing coverage cannot see and this gate can.

### Eleven of those "kills" are timeouts, not evaluated detections

The authoritative figure, from the first full-strength CI run (2026-08-03,
ubuntu-latest): **1647 mutants, 1577 killed, 11 TIMED OUT, 59 adjudicated
equivalent, zero unadjudicated survivors.** Eleven resolve by hitting
`timeoutMS: 20000` rather than by a test failing.

Seven of the eleven were localised first, over `wire.ts`, `grid.ts`, `auth.ts`
and `feed.ts` (7 of 155 in that scope); the other four are elsewhere and have
not been attributed. Six of the seven are in `wire.ts`:

| mutant | mutator |
|---|---|
| `wire.ts 76:45`, `77:25`, `81:26`, `112:42` | `BlockStatement -> {}` (the `ws.onopen`/`onerror`/`onclose` bodies) |
| `wire.ts 98:22` | `ArrowFunction -> () => undefined` (`ws.onmessage`) |
| `wire.ts 153:37` | `BlockStatement -> {}` |
| `feed.ts 38:55` | `UpdateOperator -> s--` |

Emptying `ws.onopen` means `resolve()` never fires, so a test awaiting
`connect()` blocks until the timeout instead of failing. `check-ts-mutation.py`
counts timeouts as detections (Patrik's ruling, 2026-07-28) while its own
comment says they are "NOT reliably detections" and asks that the blocking test
be made to fail fast. The table above says `wire.ts | 22 → 0`, which is true
but hides this: six of those are neither survivors nor evaluated kills. They
also cost ~2 minutes of gate time. Recorded here because this file is the
honesty ledger for the gate's narrowness, and an unevaluated kill is exactly
the kind of thing it exists to stop from passing as a real one.

## Deliberately NOT gated: client/src/main.ts

The browser entry point, and the only file excluded. It used to be a
module-level block at the foot of `app.ts`:

```ts
if (typeof document !== "undefined") {
  const root = document.getElementById("app");
  if (root) boot(root);
}
```

That runs on IMPORT. By the time any test executes the module is imported and
the branch is taken, so no in-process test can reach its other side. Measured
rather than asserted: the block carried **8 mutants, 7 of which survived** —
the one death is `if (root)` forced true, because `boot(null)` throws during
import and takes every test with it. Note what those 7 are and are not: they
are perfectly observable in a browser, so the obstacle was the HARNESS, not the
code. Both easy answers would have asserted something false — a `Stryker
disable` claims "not worth testing", an equivalence entry claims "no test could
ever kill this" — so the file was split instead. `app.ts` is a library and is
gated as one; `main.ts` is the wiring that runs it, is excluded from `mutate`,
and is kept small enough that reading it IS the review.

It is excluded from `mutate` ONLY. It carries a normal 100.00 floor in
`tools/ts-coverage-thresholds.txt`, because `all-modules.test.ts` walks
`client/src` and imports every file, `main.ts` included, so it has always been
measured at 100%. It was briefly ALSO exempt from the coverage gate on the
stated grounds that "no test imports it — by design"; that was false, and the
2026-08-03 review caught it. Two gates, two different questions: mutation asks
what a test could observe, coverage asks what a test executed.

**The `typeof document` guard stays in `main.ts`, and moving it out was a
regression.** The first version of this split dropped it as redundant — this is
the browser entry, so a DOM is a given. It is not:
`client/test/all-modules.test.ts` imports every source module to prove each one
loads, bun shares one process across test files, and whether a DOM exists at
that moment depends on which file registered happy-dom first. Without the guard
that test failed on its own and passed in the suite, which is precisely the
ordering accident it exists to catch. Excluding a file from the gates removes
the machine that would have told you; the guard's own comment was the only
warning, and it was in the code that got moved.

Vite still names the bundle after the HTML entry, so `assets/index.js` did not
move. `client/index.html` points at `/src/main.ts`.

## What the survivors actually were

Every file told the same story, and it is worth expecting it rather than
rediscovering it: **the happy path was tested and the refusals were not.**

`fold.ts`'s 93 survivors were not 93 problems — 37 emptied an error MESSAGE and
35 deleted a GUARD, in a module whose opening line is "parity includes
REJECTING what Go rejects". `view/dm.ts` opened at 40.71%, the worst in the
client, with 80 of 166 survivors emptying a string LITERAL: what was untested
was never the rendering, it was every refusal message, every field name, and
every piece of input normalisation on the way to a command. The existing tests
asserted with loose regexes (`/name/i`), which is exactly why blanking a
message survived them.

The second pass found the same shape in the remaining eight, plus two new ones.

Three things worth copying:

* **Assert the SENT COMMAND, not just that something was sent.** A removed
  `.trim()` is invisible until you look at the payload: the form still works
  and the server quietly stores `" s1 "` as a distinct scene id.
* **Assert PRESENCE, not wording, for prose.** Pinning placeholder text breaks
  on every copy edit; asserting each box HAS one kills the `-> ""` mutants and
  survives rewording.
* **Count the calls to a confirm/deny seam.** "Nothing was sent" cannot tell a
  validation refusal from a declined dialog, so a version that confirmed first
  and validated second passes without a call counter.

## The failure mode this pass kept hitting: tests that pass for the wrong reason

Three separate tests in `app.ts` looked like they covered the mutant and did
not, each because the assertion was **already true for an unrelated reason**:

* "no Abilities heading" held because `renderPlayerPanel` short-circuits before
  the ability list when the player controls no actor — the list was never read.
  Fixed by seeding a scene, a controlled actor and a placed token.
* "no `Stryker` in the text" held because a malformed adventure renders as
  `Load undefined`, which contains nothing to grep for. Fixed by asserting the
  SECTION is absent.
* "the adventures list is empty when its fetch fails" held because
  `fetchAdventures` answers `[]` on a 404 rather than throwing — it overwrote
  the initializer under test with an identical value. Fixed by failing the
  chain at the hop BEFORE it.

In all three the repair was not a stronger assertion, it was **reaching the code
at all**. A surviving mutant against a green test is the gate telling you the
test is hollow; that is the whole difference between line coverage and this.

**A review pass then found four more of the same, including two in tests
already "fixed" above** — the seeded replacements were ADDED beside the hollow
originals instead of replacing them, so both shipped and the vacuous copies
still passed. Also caught: a toast test that asserted nothing because it
delivered an inbound EVENT rather than a command result, so the code under test
never ran; a test claiming to cover two `[...session.events]` spreads while
asserting only narration text, which only the feed renders (the DM console read
that same log for the Undo group's own bounds, so blanking the spread killed
undo and changed no prose — both that group and the functions it called are
gone with `d3e2f28`, and the shape is recorded here rather than the names);
and an `expect(...).not.toThrow()` around a dispatched DOM
event, which **cannot fail** — happy-dom reports listener exceptions inside its
own dispatch rather than propagating them.

The lesson is narrower than "write better tests". Every one of these was found
by asking a mechanical question — *break the production code; does this test go
red?* — and none was found by reading. Ask it of each new test, especially the
ones whose assertion is a `.not.` something.

## Adjudicating equivalents: what actually recurred

Three shapes, and nothing else:

1. **Comparator arms over unique map keys.** `a < b ? -1 : a > b ? 1 : 0` where
   the values are Map keys. The equal case cannot occur, so `<`→`<=` and
   `>`→`>=` are unobservable. Separately, forcing the second arm to 0 is also
   invisible: `Array.prototype.sort` branches on `comparator(...) < 0` and never
   distinguishes 0 from a positive result. That last one was CHECKED, not
   assumed — a 20-element reversed array, enough to leave insertion sort for the
   merge paths, sorts identically under it.
2. **Guards whose bypassed branch is neutralised downstream.** Forcing a
   payload-kind check true makes the body read fields that do not exist on that
   payload; `undefined <= undefined` is false, so loops run zero times and
   filters match nothing. The guard is worth keeping and the mutant is still
   dead.
3. **Mutually-redundant fallbacks.** `[...(a || b || "?")][0] ?? "?"` — each
   `"?"` is unobservable alone because the other covers the same case. Neither
   can be deleted: the second is load-bearing for the TYPE even where it is
   unreachable at run time.

`tools/ts-mutation-equivalents.txt` carries the per-entry reasoning, and its own
header explains why a wrong entry there is worse than an unadjudicated
survivor.

## Real defects this pass pinned

Not everything was message-blanking. Three behaviours had no test at all:

* **`wire.ts` sending into a dead socket.** The not-connected guard has two
  halves and only "never connected" was covered. The `readyState` half catches
  the ordinary case — a click a moment after the gateway went away — and
  without it the command is written to a closed socket while the caller's
  promise is registered where nothing will ever resolve it.
* **`view/feed.ts` anchoring past an empty range.** Drop the upper bound from
  the placement search and narration anchored to a moment absent from this log
  is stapled onto the next unrelated event — the DM appearing to describe
  something they did not.
* **The DM console's confirmation.** Nothing drove `window.confirm`. The
  closure could have been `() => undefined` — always falsy, so every guarded
  action silently cancels — with the whole suite green. It guarded undo when
  this was measured; `DMDeps.confirm` outlived it and now guards the join-link
  rotation, which is the same defect if it is left undriven.

## A trap this cost an hour

Stryker's JSON reports a `replacement` string but NOT which sub-expression it
replaces. At undo.ts:39 the line is

    if (best === null || e.sequence > best) best = e.sequence;

and there are mutants at 39:9, 39:26 and two more, with replacements `false`,
`true`, `e.sequence >= best`. Reading `39:9 -> false` as "the whole condition
becomes false" is wrong: 39:9 is the LEFT OPERAND, `best === null`, and the
mutated line still assigns via the right operand. Hand-applying the reading
rather than the actual mutant produced a suite failure and very nearly a bug
report against Stryker.

(That file is gone — `d3e2f28` deleted it — so the coordinate cannot be
re-read today. The trap is kept because it is about how to READ a Stryker
report, not about undo.ts, and it cost an hour once.)

**Use the start AND end columns to disambiguate.** `auth.ts:37` has mutants
spanning 12-34 (the whole condition, all killed) and 12-22 (`t === null` alone,
equivalent). Same start column, different claims. Read the instrumented file in
`.stryker-tmp` (set `cleanTempDir: false`) before concluding anything about a
mutant you cannot place.

## Two things that will bite the next person

**Verify a measurement actually ran.** Twice this pass a Stryker invocation
reported a clean result while having done nothing: once `--commandRunner.command`
failed with "unknown option" and the surrounding script read the delta as zero,
once a background run had not finished and the report on disk was the previous
one — the tell was numbers identical to the last run. Check the log, or the
report's mtime, before believing a number.

**`bun test` does not type-check.** A test using `ws.data` for per-connection
state passes `bun test` and fails `task client:typecheck`, minutes later, in a
file that was "already passing". Run `task client:typecheck` (seconds) rather
than discovering it via `task check`.
