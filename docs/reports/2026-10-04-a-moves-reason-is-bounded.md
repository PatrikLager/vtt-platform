# A move's reason is bounded: the change

**Ticket:** `docs/superpowers/specs/2026-10-04-a-moves-reason-is-bounded-design.md`,
revised by its writer (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-04-a-moves-reason-is-bounded.md`,
verified by `verify-ticket` (Passes with gaps).
**The owner's rulings:** on 2026-10-04, a go-ahead for one ticket bounding a
move's reason and describing the MCP tool's field, on the proposal of 256
bytes and a refusal rather than a cut; and at sign-off the same day, Q1,
Q2, Q3, Q5 and Q6 as the plan proposed (256 bytes of UTF-8, inclusive, empty
allowed; a new SPEC-018 for the fold's byte bounds; the number stated in the
description and held to the constant by a test; one code commit and the
report; rows A and B) and Q4 against the plan's recommendation: the client
fold's missing bound on a narration's speaker is fixed in the same commit
rather than recorded as debt.
**Last code commit:** `a4543e6`, on `23b7152`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline 23b7152..a4543e6

    a4543e6 A move's reason is bounded, and its tool says who reads it

`git diff --stat 23b7152..a4543e6`: 23 files changed, 1702 insertions(+), 36
deletions(-).

The gate, `task check` whole, after `a4543e6`, once: it exited 0 with no step
failed. Its check steps' own verdict lines read `check:comments` clean over
254 files, `check:requirements-chain` 266 rows, 211 test files and 12
specifications, `check:doc-owner` 80 files, `check:new-prose` 966 added lines
clean, `check:coverage` 20 packages at or above their floors, `check:no-pack`,
`check:no-retraction` and `check:no-create-scene` clean, `task lint` 0 issues,
`check:breaking` reporting pre-release with no objection, `check:mutation` 14
packages with zero unadjudicated survivors (`internal/artlib`,
`internal/campaigncfg`, `internal/identity`, `internal/mapdef` and
`internal/store` reusing their verdicts, the nine whose closure holds
`internal/engine` mutated afresh, six mutants timed out in `internal/sight`,
`internal/rules` and `internal/mcp` and counted as killed), and
`check:ts-mutation` 2917 mutants, 2818 killed, 70 survivors all adjudicated
equivalent, zero unadjudicated, 29 timed out and counted as killed.
`check:mutation` began with 38.7 GiB free; `task check:drift` exited 0 after
the commit.

## Done looks like, answered

1. `[x]` A `move_token` whose reason is longer than 256 bytes is refused and
   appends nothing, and one of exactly 256 bytes is accepted:
   `internal/engine/apply_test.go#TestAMoveWhoseReasonExceedsTheBoundIsRefused`
   (red on `23b7152`'s fold with `err = <nil>`),
   `internal/engine/apply_boundary_test.go#TestMoveReasonAtCapIsAccepted`, and
   `internal/gateway/server_test.go#TestAMoveWhoseReasonExceedsTheBoundAppendsNothing`
   over the wire (red on `23b7152`'s fold with `257-byte reason: ok=true`),
   which also holds the log head unchanged and the 256-byte move appended whole.
   The at-cap half could not be red on `23b7152`, where nothing refused; the
   `>=` break holds it.
2. `[x]` `client/src/fold.ts` refuses the same `TokenMoved` at the same byte
   count: `client/test/fold-rejections.test.ts#a move reason longer than 256
   bytes is rejected`, `#a move reason is measured in UTF-8 bytes, not
   characters` (both red on `23b7152`) and `#a move reason of exactly 256 bytes
   is ACCEPTED`.
3. `[x]` The MCP `move_token` tool's `reason` says it is optional, that only
   the DM and the agent read it, and that it is at most 256 bytes of UTF-8:
   `tools/toolgen/main_test.go#TestMoveTokenReasonSaysOnlyTheDMAndTheAgentReadIt`
   (red with an empty description on `23b7152`),
   `TestToolsMatchGolden` against the hand-edited
   `contract/testdata/expected_tools.json`, and
   `internal/engine/move_reason_internal_test.go#TestTheMoveToolStatesTheFoldsReasonBound`,
   which holds the stated number to `maxMoveReasonBytes`. Both `tools.json`
   files were regenerated, and `task check:drift` exited 0 after the commit.
4. `[x]` Every golden stream is byte-identical:
   `cmd/vtt/scenario_goldens_test.go#TestScenarioGoldenStreamsHaveNotDrifted`
   is green in the gate.
5. `[x]` `task check` whole is green, as the gate paragraph above records.
6. `[x]` `client/src/fold.ts` refuses a `NarrationAdded` whose `as` is longer
   than 256 bytes, as `engine.Apply` does:
   `client/test/fold-rejections.test.ts#a narration speaker longer than 256
   bytes is rejected`, red on `23b7152`, and `#a narration speaker of exactly
   256 bytes is ACCEPTED`. `docs/verification-debt.md` records how the gap
   escaped, under 2026-10-04, closed by that test.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A move's reason is at most 256 bytes. | VTT-264: both folds refuse a `TokenMoved` whose reason is longer than 256 bytes of UTF-8, and accept one of exactly 256 |
| A move whose reason is longer is refused and appends nothing. | VTT-264 for the refusal; "appends nothing" is VTT-162's, which gains the wire test, as VTT-161 does |
| The tool that offers a move's reason says who reads it and how long it may be. | VTT-265 |
| The client's fold refuses a narration speaker the server's fold refuses. | VTT-266, from the owner's ruling on Q4 |

## The sort

Two rows accepted as the plan's D8 proposed (A and B, now VTT-264 and
VTT-265), one added by the owner's ruling on Q4 (VTT-266), none withdrawn,
and eight candidates refused as D8 lists them; none was reversed.

## Phase 4a: QA adjudications

**`a4543e6` (VTT-161, VTT-162, VTT-264 to VTT-266), QA on opus, given those
rows, SPEC-018 whole, SPEC-013's command-path paragraph, the text of
`MoveTokenRequest`, `TokenMoved` and `NarrationAdded`, the generated
`move_token` tool entry, `go doc -all` of `internal/engine`,
`internal/gateway`, `internal/campaign` and `internal/identity`, and
`fold.ts`'s export lines; it wrote `internal/gateway/qa_move_reason_bound_test.go`
and `client/test/qa-move-reason-bound.test.ts`.** Ten Go tests and thirty-one
TS tests, none failing; thirty-eight injections into its own files, all red,
the files restored byte for byte.

- SPEC-018's "checked for its token, then its destination" did not say that
  the destination check is for presence only: a spec ambiguity, and SPEC-018
  now reads "checked for a known token, then for a destination at all, then
  its reason".
- The TS field labels and the other Go refusal texts are not quoted: SPEC-018
  quotes two texts as the form, and the tests pin the exact text of each field
  they cover. No change.
- A log holding an over-bound event not opening went untested for want of the
  store's surface: `campaign.Open` returns `rebuildLocked`'s error, and the
  reading review's probe printed `campaign: corrupt log at seq 4: engine: move
  reason must be at most 256 bytes, got 257`. No change.
- The two folds agree on accept and refuse, not on message: by design. No
  change.
- VTT-162 is seen from an unprojected observer only: a refusal appends
  nothing, so no seat receives anything. No change.
- Dropped: `TestQAMoveReasonBoundGoFoldLeavesNamesUnbounded` and the TS file's
  three "leaves a 10000-byte name unbounded" tests. They pinned the absence of
  a bound, which no rule states, and would red when one is added; nine Go and
  twenty-eight TS tests remain.
- The implementer also changed QA's files: three citation lines merged into
  one, a comment re-aimed at the sentence its test exercises (the reading
  review's), the quotation of SPEC-018's check order brought to the amended
  sentence in both files, and spec quotations rewrapped for
  `check:new-prose`.
- Requirements QA asked an id for: the table's other rows, the check order,
  the message form and the refusal at open are SPEC-018 text the sort refused
  or no rule. None dispensed.

## The breaks

The commit's message carries them, one line each, with the check that spoke;
each was run in a scratch clone carrying the commit's code and restored after
each. Ten: the Go reason check removed; the Go check made `>=`; `fold.ts`'s
reason check removed; its bound made 255; `checkLen` counting characters; the
constant made 300; the description saying 200, regenerated; the who-reads
sentence dropped, regenerated; the Go refusal reworded; `fold.ts`'s speaker
check removed.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool`: MapTool bounds no free
text, neither a chat message nor a token's notes, and its connection reads a
length prefix without a ceiling; its one cut is a tooltip shortened for
display. Borrowed: nothing. Refused: an unbounded field, since the log is
append-only, and a display-side cut, since the feed shows a reason whole.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D9: "The `as` gap is a debt entry, per Q4, option (i)." | fixed in the commit, with VTT-266 and two TS cases; SPEC-018 states no exception to the client's mirror | the owner's ruling on Q4 |
| D11: "Re-point the four `apply.go` keys by +4 and the eighteen `fold.ts` keys by +1"; the TS self-test "cannot see twelve of the eighteen" | eight `fold.ts` keys moved +1 and ten +2, each read at its new line; the TS self-test flagged eight | the speaker check is a second new line in `fold.ts`, below the first |
| the plan's constraint: "The ticket and every report under `docs/reports/` are not edited." | the ticket's writer corrected two false sentences in its problem paragraph, added the speaker bound to it, added the new specification to "Specifications this moves", listed the files the verification found, and corrected a third sentence on the reading review's finding | the verification's gaps 1, 2 and 4, the owner's ruling on Q4, and the reading review's finding that the ticket said every other text is bounded by the WebSocket frame |
| D7: SPEC-018 states "that `internal/adventure`'s loader mirrors three bounds, pinned by `TestSizeCapsMirrorEngine`" and that every other text is bounded only by `maxWSFrameBytes` | SPEC-018 says the test pins the loader's copies to literals, and that a map's or an adventure's names are bounded by nothing | the reading review: the test never reads the engine's values, and file-borne texts never cross a WebSocket frame |

## What could not be established

- **Nothing ties a bound's copies to the engine's constant** but each side's
  tests (What was deliberately left out).
- **Which adjudication goes with which `fold.ts` mutant, where two keys share
  a mutator and a replacement.** `check:ts-mutation`, which fails on a key that
  matches no live mutant, passed after the commit, so all eighteen re-pointed
  keys sit on live mutants; a swap between two such keys would pass it too, and
  uniform shifts of +1 and +2 make one unlikely.
- **A map's or an adventure's names have no bound**: every scene's name, an
  adventure's actor names. An adventure's scene id, notes and opening
  narration are bounded at load. Nothing here decides whether the names
  should be.

## What was deliberately left out, and where it went

- A bound on a map's or an adventure's names: raised to the owner with this
  report; no ticket yet.
- A link between a bound's copies (`fold.ts`'s literals, `internal/adventure`'s
  constants) and the engine's constants: `docs/verification-debt.md`, open.
- No proto changed; `check:breaking` names nothing.
