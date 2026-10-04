# A move's reason is bounded — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-04-a-moves-reason-is-bounded-design.md`
**Verified:** 2026-10-04 by `verify-ticket` (dev-cycle 0.4.0), run by an agent
that did not write the ticket, against `23b7152` on `feat/reason-bounded`
(equal to `main`, with the ticket untracked). Verdict: **Passes with gaps.**
The gaps are listed at the end and travel with this plan. This plan does not
edit the ticket.

**Goal, in the ticket's words:** a `move_token` whose reason is longer than
256 bytes is refused and appends nothing, and one of exactly 256 bytes is
accepted. The refusal is the fold's, and `client/src/fold.ts` refuses the same
`TokenMoved` at the same byte count. The MCP `move_token` tool's `reason` says
that it is optional, who reads it and how long it may be. Every golden stream
is byte-identical, and `task check` whole is green.

**MapTool (CLAUDE.md rule 9): it does not solve this.** MapTool bounds no free
text. `TextMessageDto.message` (`messages/src/main/proto/data_transfer_objects.proto`)
is a plain `string`, as are `Token`'s `notes` and `gmNotes`
(`model/Token.java`). `AbstractConnection.readMessage` (`clientserver`) reads
a four-byte length prefix and allocates that many bytes, with no ceiling.
`git grep -niE "too long|too large|exceeds"` over `src/main/java` finds no
refusal of any text. The one cut is display-side: `EditTokenDialog`'s
property-table tooltip stops at 100 characters and adds " ...".
**Borrowed:** nothing. **Refused:** the unbounded field, because the log is
append-only and the world-layer design's §4 amendment rules that "caps must
precede any live log"; and a display-side cut, because the feed shows a reason
whole (`client/test/qa-move-reason.test.ts`'s "a long reason is shown whole,
not cut"). Read in `~/dev/RPTool/maptool` at `f4b7fef6c`.

## Verification, check by check

1. **Every path resolves (by command).** `ls` finds all 17 paths the ticket
   names. `grep` finds each named symbol where the ticket puts it:
   `TokenMoved`'s `string reason = 5`, `const maxWSFrameBytes = 32768`, the
   four constants at the values given, `fold.ts`'s `case "tokenMoved"` and
   `function checkLen`, `spectator.ts`'s `describe`, toolgen's `var manifest`
   and `fieldOverride.fieldDocs`, `check:drift` and VTT-261. Two claims in the
   problem paragraph are false, and both are gaps (1, 2).
2. **"Done" is an observation (by command).** None of the named tests exists
   yet; sketches of each fail today and pass after. Item 1: P1 (engine) and
   P2 (wire); its "exactly 256 is accepted" half cannot fail today, because
   nothing refuses (Gap 3). Item 2: P3. Item 3: P5. Item 4, measured: the 9
   golden streams hold 7 `tokenMoved` events and none carries a reason, and
   `go test ./cmd/vtt/` is green on the probe tree. Item 5 is the gate.
3. **Each rule is breakable (a reading, then probes).** Each of the three rules
   has a single edit that reds a named test (K1 to K9). The first rule and the
   refusal half of the second become row A. The "appends nothing" half is
   VTT-162. The third rule becomes row B (D8).
4. **The scope matches the claim (by command, then a reading).**
   `engine.Apply`'s production callers are five calls in `internal/campaign`
   and one in `internal/harness`, which fold whatever the log holds.
   `TokenMoved` is built by `ToEvent` and `internal/eventgen` only, and
   eventgen's `moveReasons` hold at most 11 bytes. `fold()` is called by
   `client/src/session.ts`, and a TS fold refusal freezes the session
   (`client/src/wire.ts`), so the two folds must agree exactly. toolgen's own
   tests read the manifest through `buildTools`, and `cmd/vtt` embeds the
   generated `tools.json`, which `check:drift` holds. No existing test sends a
   reason over 256 bytes to either fold: `qa-move-reason.test.ts` builds a
   512-byte reason and hands it to `describe` only, which does not fold. The
   work reaches files the ticket does not list: `tools/toolgen/main_test.go`,
   a new `internal/engine` internal test (D6), both adjudication files (D11),
   `tools/comment-ceilings.txt`, `docs/verification-debt.md` (D9) and a new
   specification (D7). None of them widens the three components.
5. **No recorded decision is contradicted silently (a reading).** The
   world-layer design's §4 amendment set the size posture (UTF-8 bytes, `as`
   at most 256 B and possibly empty, caps before any live log), and the ticket
   follows it. SPEC-013's "what the fold refuses is `engine.Apply`'s" and
   `handleCommand`'s "not in the fold (SPEC-013)" govern form validators of
   unspecified enums; a byte bound in the fold follows the note and narration
   precedent and contradicts neither. ADR-007: no contract change. The
   predecessor plan's "`engine.Apply` and `fold.ts` do not read the reason"
   was a plan constraint, not a record, and its report is history and is not
   edited.
6. **The records the work moves are named (by command, then a reading).** The
   section lists SPEC-013, and the path resolves. The reading: SPEC-013's
   "`engine.Apply`'s `TokenMoved` arm reads only `TokenId` and `To`" becomes
   false, which the named file covers (D7). No specification records the
   fold's byte bounds (`grep` over `docs/specifications` finds no 8192 and no
   256). The ticket leaves the choice of record to the plan, and the plan
   proposes `New:` SPEC-018 (Gap 4, Q2). SPEC-011's `maxWSFrameBytes` and
   SPEC-016 are unchanged. SPEC-007 records VTT-232, the one earlier
   tool-schema row; it records `required` and not `fieldDocs`, and nothing in
   it becomes false.

## Measurements this plan stands on

Every measurement was taken by command, by the verifier, in a scratch clone of
`23b7152` (`git clone --no-hardlinks`, `bun install --frozen-lockfile`), never
in the working tree. The sketches are discarded and the clone is deleted.

| # | Tree | Result |
|---|---|---|
| P1 | an engine refusal test and an at-cap test, at the base | refusal red: `err = <nil>, want the move reason's bound`; at-cap green |
| P2 | D4's wire test, at the base | red: `257-byte reason: ok=true error="", want the fold's refusal` |
| P3 | D4's three TS cases, at the base | the two refusals red, `Received function did not throw`; at-cap green |
| P4 | D2 and D3 applied | P1 to P3 green |
| P5 | D5's `fieldDocs` | `TestToolsMatchGolden` and `TestRunOutputMatchesCommittedToolsJSON` red. `task generate:contract` changes `contract/gen/tools/tools.json` and `cmd/vtt/tools.json` by one line each, and nothing else. After the one-line `expected_tools.json` edit, toolgen is green, and `go test ./cmd/vtt/` is green (139.6 s, scenario goldens included) |
| P6 | D6's link test | green; with the constant at 300, red: `move_token's reason description = "…", want it to state "At most 300 bytes"` |
| P7 | the whole probe tree | `go test -count=1 ./internal/... ./contract/... ./tools/...` green; `bun test client/test contract contract-spike` 843 pass, 0 fail; `golangci-lint` 0 issues; `go vet` clean; `semgrep --config .semgrep` 0 findings; `go-arch-lint check` OK; `client:typecheck` clean; `task build:client` changes `cmd/vtt/webdist/assets/index.js` only; `check:new-prose` 157 lines across 9 files, all clean; `check-doc-owner.py` 80 files |
| P8 | P7 with ids simulated (two rows, VTT-161 and VTT-162 evidence, a SPEC-018 stub) | `check-requirements-chain.py`: `265 rows, 209 test files, 12 specifications` |

**What breaks what.** Each is one edit on the probe tree, restored afterwards:

| # | Edit | Red |
|---|---|---|
| K1 | Go check removed | engine refusal test, wire test |
| K2 | Go `>` becomes `>=` | engine at-cap test, wire test |
| K3 | TS `checkLen` line removed | the two TS refusals |
| K4 | TS bound 255 | all three TS cases |
| K5 | `checkLen` counts `s.length` | the multibyte case **alone**: it is the only test in the tree that holds `checkLen`'s byte counting |
| K6 | constant 300 | link test, engine refusal test, wire test |
| K7 | description says 200, regenerated | link test, `TestToolsMatchGolden` |
| K8 | the who-reads sentence dropped, regenerated | toolgen's who-reads test, `TestToolsMatchGolden` |
| K9 | Go refusal reworded | engine refusal test, wire test |

**Comment shares** (`check-comments.py --report`, base then probe, against the
ceiling): `fold.ts` 43.2 → 43.1 / 43.2, `apply.go` 43.0 → 42.7 / 43.0,
`apply_test.go` 13.4 → 13.2 / 13.4, `apply_boundary_test.go` 27.2 → 25.0 /
27.2, `server_test.go` 26.8 → 26.5 / 26.9, and the new internal test 2.9 with
no row. `apply_boundary_test.go` falls more than 1.0 under its row, so
`check:comments` fails until `--write-ledger`. That run lowers six rows
(`spectator.ts` 49.9 → 49.6 among them), adds the new file's row, and then
reads clean over 253 files.

**Mutation keys.** Both edit points sit above every adjudicated key in their
files, and nothing honest makes either edit line-neutral: the constant belongs
with its siblings and the check in its arm. `apply.go` moves +4 (one constant
line, three check lines) and carries four keys: `id < standing` in
`ActorRemoved`, `computed < 0` and `computed > int64(res.Max)` in
`ResourceChanged`, and `len(objs) > 0` in `SceneSeen`. The Go self-test flags
all four. `fold.ts` moves +1 and carries all 18 TS keys, of which the TS
self-test flags only six: `!sc.Tiles`, `computed < 0`, `computed > res.max`,
`case "attackRolled"`'s string, `Number(e.sequence) > head` and the `actor_id`
string. The other twelve still read as valid at their old coordinates; for
example, `abilityUsed`'s string key then holds `attackRolled`'s string.
`st.Conditions[v.actorId] ?? []` occurs twice, and the key is
`conditionRemoved`'s, the second. Re-pointed by statement text, all 22 moved by
exactly +4 and +1, and both self-tests then print `OK`. toolgen is not in
`PACKAGES`.

**Records and gates at the base.** `check-requirements-chain.py .`: `263 rows,
208 test files, 11 specifications`. `check-comments.py main`: `252 files, 0
added comment lines, 252 ledger rows; clean`, with a standing notice on
`cmd/vtt/library_test.go`. `check-doc-owner.py .`: `80 files`. Both mutation
self-tests: `OK`. Free space on `/System/Volumes/Data`: 35,276,576 KiB at the
start and 34,655,208 KiB (33.0 GiB) after the clone was deleted. The Go cache
is 7.1 GiB, and load averages ran 1.7 to 2.4.

## Constraints that bind every task

- **Rule 2:** no gate is weakened. The ledger goes down only through
  `--write-ledger`, and a key is re-pointed, never deleted.
- **Rule 3:** no proto changes, so `check:breaking` prints its header and
  nothing else.
- **Rule 4:** both folds refuse the same `TokenMoved` at the same count, in the
  same order (token, destination, reason). Neither stores the reason:
  `engine.State` has no field for it.
- **Rule 8:** cite names, never lines; the adjudication files keep their
  coordinates. **Rule 9:** answered above, before any task runs. **Rule 10 and
  SPEC-010:** the only new comments are test citation lines and D2's header.
- **SPEC-008:** ids come from `requirement-id`, after sign-off, A then B.
- **Order and hygiene.** `check:drift` passes only on a committed tree, so the
  order is review, commit, gate. The review package is `git diff HEAD`, and
  nothing is stashed or checked out while a reviewer reads. `git add` and `git
  commit` run in separate calls, with `git show --stat HEAD` after each. A new
  test goes after a closing brace.
- The ticket and every report under `docs/reports/` are not edited.

## Decisions this plan makes

**D1. The bound: 256 bytes, inclusive, an empty reason allowed (Q1).** Forced
by the posture's precedent. 256 is the fold's bound for a one-line label
(`maxNoteTitleBytes`, `maxNarrationAsBytes`), a reason is shown on one line
after the move's label, and longer prose belongs in `add_narration` (8192).
Inclusive, because every cap in `Apply` is written `> max`. Empty stays
allowed, because VTT-261 makes an empty reason mean none, just as `as` and a
note title may be empty.

**D2. The Go fold.** `maxMoveReasonBytes = 256` goes last in `apply.go`'s
const block. In the `TokenMoved` arm, after the destination check: `if
len(tm.Reason) > maxMoveReasonBytes { return fmt.Errorf("engine: move reason
must be at most %d bytes, got %d", maxMoveReasonBytes, len(tm.Reason)) }`, in
the form of `narration as must be at most`. The block's two-line header is
factually stale: it says "Size/anchor limits for the world layer (spec §4)",
yet the block holds no anchor limit and a move is not world layer. It is
rewritten in the same two lines as a warning and a pointer: `// Inclusive byte
bounds on an event's free text (SPEC-018). Never lower one: a` / `// log that
folded under the old bound would stop folding.` Measured: this moves no key
and passes `check:comments` and `check:new-prose`. The eight-line history above
`maxNarrationAsBytes` stays, because rule 10's conversion is not this
ticket's.

**D3. The TS mirror.** `checkLen("move reason", v.reason, 0, 256);` goes after
`tokenMoved`'s destination check, as a literal like its siblings. The error
reads `move reason exceeds 256 bytes`. Nothing links the literal to Go's
constant but each side's tests, as for every bound (Gap 5).

**D4. The tests, each red today but the at-cap ones (P1 to P3).**

- `internal/engine/apply_test.go`, at the end:
  `TestAMoveWhoseReasonExceedsTheBoundIsRefused`, with a 257-byte reason, the
  exact error and an unchanged `Snapshot`, and a `seedMovableToken` helper
  after it.
- `apply_boundary_test.go`, at the end: `TestMoveReasonAtCapIsAccepted`, with
  256 bytes.
- `internal/gateway/server_test.go`, after `TestAMoveWithNoReasonAppendsNone`'s
  closing brace: `TestAMoveWhoseReasonExceedsTheBoundAppendsNothing`. The DM
  moves `t1` with 257 bytes and gets ok=false with the exact text, and
  `f.head` is unchanged. The 256-byte move then answers ok with `head+1`, and
  the agent's first event is that move, with all 256 bytes. It cites A,
  VTT-161 and VTT-162.
- `client/test/fold-rejections.test.ts`, at the end, in a "a move's reason"
  section: "a move reason longer than 256 bytes is rejected", "a move reason
  is measured in UTF-8 bytes, not characters" (129 × `é`), and "a move reason
  of exactly 256 bytes is ACCEPTED" (128 × `é`).

The fold-parity keystone and the goldens need no case: a stream the fold
refuses cannot be a golden, and each side keeps its own refusal suite.

**D5. The tool description.** `manifest`'s `move_token` gains
`overrides: {"vtt.v1.MoveTokenRequest": {fieldDocs: {"reason": …}}}`, with
`requiredOverride` left nil so the derived `required` stands. The text:

    Optional; why the token moved, recorded with the move in the log. Only the DM and the agent read it: no player or spectator is sent it. At most 256 bytes of UTF-8; a longer reason refuses the whole move.

It carries no `maxLength`, since JSON Schema counts characters, not bytes, and
it is the first `fieldDocs` to state a number. The tests:

- `TestMoveTokenReasonSaysOnlyTheDMAndTheAgentReadIt` in
  `tools/toolgen/main_test.go` asserts the three phrases.
- `TestToolsMatchGolden` compares `buildTools()` with
  `contract/testdata/expected_tools.json`.
- `TestRunOutputMatchesCommittedToolsJSON` compares the generator's output
  with `cmd/vtt/tools.json`.

`task generate:contract` regenerates both `tools.json` files.
`expected_tools.json` is not generated and gains its one `description` line by
hand. `task check:drift` runs after the commit.

**D6. The number is held in step by a test, per Q3, option (a).** toolgen may
depend on `contract` alone (`.go-arch-lint.yml`), and it cannot import an
engine constant without widening that rule, which is a gate change. A new
`internal/engine/move_reason_internal_test.go` (`package engine`, the package's
first internal test) holds the link instead.
`TestTheMoveToolStatesTheFoldsReasonBound` reads
`../../contract/gen/tools/tools.json` and requires `move_token`'s `reason`
description to contain `fmt.Sprintf("At most %d bytes", maxMoveReasonBytes)`.
A file read is not an import, so arch-lint stays OK (P7). It reds on K6 and
K7.

**D7. The records (Q2).**

**New: `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`**,
written with the `specification` skill against the code. It states:

- the six bounded fields, each with its bound and whether it may be empty;
- that bounds are inclusive and counted in UTF-8 bytes (`len` in Go,
  `TextEncoder` in `checkLen`);
- each side's refusal form, and the path: `campaign.Append` returns the
  refusal and appends nothing (SPEC-013's step 8, VTT-162);
- that `fold.ts` mirrors each bound by literal, except `as` (D9);
- that `internal/adventure`'s loader mirrors three bounds, pinned by
  `TestSizeCapsMirrorEngine`;
- that the tool states the reason's bound, and D6 holds the two equal;
- that every other text an event carries is bounded only by
  `maxWSFrameBytes` (SPEC-011), since `apply.go`'s `len` checks are exactly
  these six;
- Consequences: a bound may be raised and never lowered, and changing one
  changes its mirrors;
- Requirements: A and B.

**SPEC-013:** "`engine.Apply`'s `TokenMoved` arm reads only `TokenId` and
`To`." becomes "`engine.Apply`'s `TokenMoved` arm reads `TokenId`, `To` and the
length of `Reason` (SPEC-018), and never `SceneId` or `From`." Its "What this
record does not decide" adds "its byte bounds are SPEC-018's". Its
Requirements are unchanged.

**The register:** VTT-161 and VTT-162 each gain the wire test as evidence.
VTT-260 is unchanged: `ToEvent` still carries every field, the fold then
refuses the event, and a refused command appends nothing (VTT-162).

**D8. The sort.** Forced by SPEC-008. The rows are lettered so nothing reads as
an id.

| # | Rule | Proposed | Red when | Tests | Record |
|---|---|---|---|---|---|
| A | Both folds refuse a `TokenMoved` whose reason is longer than 256 bytes of UTF-8, and accept one of exactly 256. | accept: the first rule, and the refusal half of the second | K1 to K5, K9 | D4's six | SPEC-018 |
| B | The `move_token` tool describes its `reason` as optional and read by the DM and the agent alone, and states the bound the fold enforces. | accept: the third rule | K6 to K8 | D5's toolgen test, D6's link test | SPEC-018 |

**Refused, one line each:** (1) "a move's reason is at most 256 bytes" on its
own, which A holds; (2) "a refused move appends nothing", which is VTT-162 and
gains the wire test; (3) "it is answered ok=false on an open connection",
which is VTT-161, likewise; (4) "`fold.ts` refuses what `engine.Apply`
refuses" as a parity row, since A names both folds; (5) "an empty reason is
allowed", which VTT-261 and every golden move hold; (6) "the refusal names the
bound", which is how A is observed, by the exact-text assertions; (7) "the
description's number equals the constant", which is B's second half; (8) "the
bound is checked after sight for a player", which is SPEC-013's step order,
unchanged. Two rows accepted, eight refused, none withdrawn.

**D9. The `as` gap is a debt entry, per Q4, option (i).** Forced by the
measurement (Gap 1) and by CLAUDE.md's one file for escaped defects. A
paragraph goes under "Open debt" in `docs/verification-debt.md`:
`engine.Apply` refuses an `as` over `maxNarrationAsBytes`, while `fold.ts`'s
`narrationAdded` arm has never checked it (`8551df4`). Its label is `test data
missing`. Closing it needs `checkLen("narration as", v.as, 0, 256)` and a
`fold-rejections.test.ts` case.

**D10. Comments and the ledger.** Tests carry their `// VTT-NNN` line only, and
the helper and the internal test file have no doc. `fieldDocs` is prose under
`tools/`, outside `check:comments`. `--write-ledger` runs once, before C1.

**D11. Mutation keys: re-point last, by statement text.** After the review
settles and before the commit:

1. Re-point the four `apply.go` keys by +4 and the eighteen `fold.ts` keys by
   +1, changing the key line only, as `5999523` did.
2. Find each key's statement at its new line and re-read the column there.
3. Then run `python3 tools/check_mutation_test.py -q` and `python3
   tools/check_ts_mutation_test.py -q`.

The TS self-test cannot see twelve of the eighteen, so its `OK` is necessary
and not sufficient; Phase 4b re-reads all 22. If a review adds a line above an
edit point, re-measure every key. The new mutants are killed as follows: Go
`>=` by the at-cap test, Go negation by the refusal test, and TS `"move
reason"` to `""` by the exact message.

**D12. One code commit and the report, per Q5.** Forced by measurement. Go
alone is safe, since no client is sent what Go refuses. TS alone is not: a DM
client folding a reason Go accepted would freeze (`wire.ts`). A description
stating a bound no fold enforces would be false. The change is about ten
production lines. **C1** holds the code, the tests, the regenerated files, the
webdist, SPEC-018, SPEC-013, the rows, the debt entry, the keys and the
ledger. **C2** holds the report.

**D13. Phase 4a for C1.** One QA agent per `qa-prompt.md`, never given the
diff, the source, the existing tests or the implementer's report. It is given
rows A, B, VTT-161 and VTT-162; SPEC-018 whole; SPEC-013's command-path
paragraph; the text of `TokenMoved` and `MoveTokenRequest`; `go doc -all` of
`internal/engine`, `internal/gateway`, `internal/campaign` and
`internal/identity`; `grep -n '^export' client/src/fold.ts`; and
`move_token`'s entry in `tools.json`. It writes
`internal/gateway/qa_move_reason_bound_test.go` and
`client/test/qa-move-reason-bound.test.ts`. Adjudications go in the report,
and an escape goes to the debt file as a recipe.

**D14. Phase 4b for C1, after 4a.** One reviewer at high effort, briefed to
verify by command every sentence C1 adds (SPEC-018, SPEC-013's two edits, the
description, D2's header and the debt entry), to re-read all 22 keys at their
statements, and to check the break lines in the draft message. If the reviewer
dies on a model limit, say so and re-dispatch the same brief on `fable`.

**D15. The breaks, one per check relied on.** K1 to K9, in a scratch clone of
C1's final tree. Each gate is clean first, each break is one edit, and each is
undone by hand with the inverse edit.

**D16. Disk and load.** Forced by `check:mutation`'s 16 GiB floor.
Immediately before `task check`, run `go clean -cache`, `df -k` and `uptime`.
Launch it once after C1, in its own session (`start_new_session=True`). Below
16 GiB, stop.

## Tasks, in dependency order

### Task 0 — Baselines

**Done when:** `df -k`, `uptime`, the chain, comments and doc-owner baselines
and both self-tests match Measurements, and `requirement-id` is on the path.

### Task 1 — Rows

**Files:** `docs/requirements.md`. After sign-off, run `requirement-id` once
each for A and B, in that order. **Done when:** there are two new OPEN rows.

### Task 2 — The Go fold

**Files:** `internal/engine/apply.go`, `apply_test.go`,
`apply_boundary_test.go`.

D4's two engine tests come first (the refusal red), then D2. **Done when:**
`go test -count=1 ./internal/engine/` is green.

### Task 3 — The wire

**Files:** `internal/gateway/server_test.go`.

Write D4's wire test, which is red on the base and green on Task 2's tree.
**Done when:** `go test -count=1 ./internal/gateway/` is green.

### Task 4 — The TS mirror

**Files:** `client/src/fold.ts`, `client/test/fold-rejections.test.ts`,
`cmd/vtt/webdist/`.

The three cases come first, then D3, then `task build:client`. **Done when:**
the cases are green and `client:typecheck` is clean.

### Task 5 — The tool

**Files:** `tools/toolgen/main.go`, `main_test.go`, both `tools.json` files,
`contract/testdata/expected_tools.json`,
`internal/engine/move_reason_internal_test.go`.

Write D5's test, red, then the `fieldDocs`, `task generate:contract`, the
golden line and D6's test. **Done when:** `go test ./tools/toolgen/
./internal/engine/` is green and `git status` shows exactly the two
`tools.json` files regenerated.

### Task 6 — The records

**Files:** SPEC-018 (new), SPEC-013, `docs/requirements.md` (A and B's
evidence, plus VTT-161 and VTT-162), `docs/verification-debt.md`.

D7 and D9. **Done when:** the chain prints `265 rows, 209 test files, 12
specifications`, with A and B not OPEN, and `grep -c 'reads only'
docs/specifications/013-authorization.md` prints 0.

### Task 7 — Local gates

Run `gofmt`, `go vet`, `task lint`, `go test -count=1 ./internal/...
./contract/... ./tools/... ./cmd/...`, `bun test client/test contract
contract-spike`, `check:comments`, `check:doc-owner` and `check:new-prose`.
**Done when:** each prints its own completion line.

### Task 8 — Phase 4a, then 4b

D13 and D14. Findings are fixed and the affected task's "done" is re-run. The
review settles before Task 9.

### Task 9 — Keys, ledger, commit C1

D10 and D11, then C1's message, which lists the ids and D15's lines. **Done
when:** both self-tests print `OK` and every key reads its statement, `git show
--stat HEAD` lists C1's files, and `task check:drift` is clean.

### Task 10 — Breaks and the whole gate

D15, then D16, then `task check` once. **Done when:** each break gives its red,
and `task check` exits 0 with every step's own verdict, `check:mutation` and
`check:ts-mutation` included.

### Task 11 — The report

`docs/reports/2026-10-04-a-moves-reason-is-bounded.md`, per the
`implementation-report` skill: each Done item with its observation, the rows
and refusals, the rulings taken at sign-off, the rule-9 answer, the breaks and
the gaps. **Done when:** C2 holds it alone. Push after C2; the pre-push hook
takes about three minutes, so let it finish.

## Gaps that travel with this plan

1. **The problem paragraph says each bound is "mirrored in `fold.ts` by
   `checkLen`". It is false for `maxNarrationAsBytes`.** `fold.ts` has never
   checked `as`, and nothing records it (D9, Q4).
2. **"Every other free text the fold holds has a bound there" is false.**
   `apply.go`'s `len` checks cover narration text and `as`, and a note's key,
   title and text, and nothing else. `SessionStarted.name`,
   `SceneCreated.name`, `Actor.name` and others are bounded only by
   `maxWSFrameBytes`. SPEC-018 states which fields are bounded and claims no
   more.
3. **Done item 1's "exactly 256 bytes is accepted" cannot fail today.**
   Nothing refuses yet, so its test is held by K2, not by a red on the base.
4. **"Specifications this moves" names SPEC-013 alone.** D7 adds `New:`
   SPEC-018, a choice the ticket left to the plan. The writer may revise the
   section.
5. **Nothing links `fold.ts`'s literals to Go's constants** beyond each side's
   own tests, for this bound and the five before it. Gap 1 is what that costs.
6. **The TS self-test sees six of the eighteen keys this change moves.** Its
   `OK` is not evidence that the other twelve were re-pointed (D11).
7. **A log written since `904f79b` holding a reason over 256 bytes would stop
   folding.** No golden, fixture or eventgen draw holds one (measured), and
   nobody uses the product (the owner, 2026-09-04).

## Questions for sign-off

1. **Is the bound 256 bytes of UTF-8, inclusive, with an empty reason still
   allowed?** Recommend yes (D1). 256 is the fold's bound for a one-line
   label, `>` is every cap's form, and VTT-261 makes empty mean none. 128
   would be tighter than a note title for no measured reason.
2. **Where is the bound recorded?** (a) A new SPEC-018 recording the fold's
   byte bounds, all six fields, with rows for the reason only; (b) a new
   record of the reason's bound alone; (c) a paragraph in SPEC-013, amending
   its "what the fold refuses is `engine.Apply`'s"; (d) the rows alone.
   Recommend (a). The six share one const block, one `checkLen` and one ruling
   (the world-layer §4 amendment), and no record holds any of them. (b) splits
   that ruling. (c) contradicts SPEC-013's own scope. (d) leaves the decision
   in a ticket.
3. **Does the tool's description state the number, and what holds it to the
   constant?** (a) It states it, held by an internal engine test that reads
   `tools.json` (D6); (b) it states it, held by toolgen's literal alone, as
   `internal/adventure`'s mirror is; (c) no number, and the refusal names it;
   (d) export the constant and widen toolgen's arch rule to `engine`, so the
   description is generated from it. Recommend (a). The agent learns the bound
   before a refused move rather than after one. (b) goes stale silently (K6
   would stay green). (d) is a gate change for one sentence.
4. **The TS fold's missing `as` bound: (i) record it as a debt entry in C1,
   or (ii) fix it in C1?** (ii) means one `checkLen` line and a case, and its
   key shifts fold into D11's. Recommend (i). Nothing breaks at the table,
   since no client is sent an `as` the server refused, and the ticket is the
   reason's. (ii) is cheap if the owner wants it.
5. **One code commit and the report (D12)?** Recommend yes. The TS mirror
   cannot land first without risking a frozen DM client, and the description
   cannot land before the bound it states.
6. **Rows A and B, with VTT-161 and VTT-162 gaining the wire test and VTT-260
   unchanged?** Recommend yes (D8). The "appends nothing" rule and the
   connection staying open are VTT-162's and VTT-161's for every refused
   command. VTT-260 describes `ToEvent`, which still carries every field.
