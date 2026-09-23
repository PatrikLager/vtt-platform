# Implementation report — sub-project 5a: ruleset loader, expression language, `Resolve`

**Plan:** `docs/superpowers/plans/2026-07-25-ruleset-interpreter.md`
**Spec:** `docs/superpowers/specs/2026-07-25-ruleset-interpreter-design.md`
**Range:** `0434d5e..68008ef`, merged `83be590` (2026-07-25 09:47 +0200) — the repo's first merge.
**Report written:** 2026-09-19, against the tree at `c262290` plus sub-project 14's 17 uncommitted files (untouched; nothing in `internal/rules` is among them).

---

## 1. Vad som byggdes

Sub-project 5a turned ADR-002's founding promise ("game logic lives in rule-module data interpreted by one engine") into running code: four generic rules events and two commands in the contract, three generic folds in the engine, an atomic `AppendBatch` in store and campaign, and then the package this report is about — `internal/rules`, a stdlib-only loader that reads a ruleset directory of JSON plus a `guide.md` and validates it completely at load time; a hand-written lexer, recursive-descent parser and integer evaluator for a closed expression language with dice as a first-class term; and `Resolve`, a pure function that takes a loaded ruleset, a read-only engine state, a `UseAbility` command and an injectable `Roller`, and returns the exact ordered batch of event payloads for one `AppendBatch` call. Alongside it shipped the P4 proof: `internal/rules/conformance`, a suite driven entirely by what `Load` returns and by data the ruleset itself carries, and `rulesets/tavern-brawl`, the deliberately silly toy system that passes it. The arc is eight commits in one day, 203 files, +11,718/−177; four of those commits touch `internal/rules` (`c70bf2b` format/loader/grammar, `3c249a3` Resolve/toy ruleset/conformance, `22e2581` wiring plus `CryptoRoller`, `37c8d8b` the final-review fix wave).

---

## 2. Hur det fungerar i dag

Measured on the current tree: `internal/rules` (8 non-test Go files including `conformance/`) is **1,492 comment lines against 3,083 code lines and 368 blank**, 4,943 lines total, in 195 comment blocks of which **32 run to ten lines or more**. `expr.go` is 1,303 lines; `load.go` is 1,088. `go test ./internal/rules/...` is green.

### `Load` — and the fork that is not vestigial

The shape a previous reader flagged is real and reproduces:

```go
var supportedFormatVersions = map[string]bool{"2": true}   // load.go:22
...
return loadV2(dir, manifest)                               // load.go:68 — unconditional
```

A one-element set and a tail call. **It is not vestigial, and it is not this arc's.** `Load` genuinely forked on `manifest.FormatVersion` at `64a7131` — `if manifest.FormatVersion == "2" { return loadV2(...) }` followed by the whole v1 decode/cross-validate path inline — and `supportedFormatVersions` was `{"1": true, "2": true}` at that commit. The v1 arm, `loadAbilities`, `crossValidate`, `adaptV1Abilities` and the `AdaptV1Ability` adapter were deleted at `2ac4ea9` ("v1 sunset"), which is sub-project **5c** (`feat/format-v2-composition`, merged `5fbc666`, 2026-07-26), one day after this arc. The single-branch remainder is the residue of that deletion. It belongs to the format-v2 report, not this one, and `load.go:13-21` and `load.go:32-55` say so at length in their own words.

So the honest statement for a reader today: **this arc built format v1 and the loader for it; almost none of the v1 format survives.** What survives from 5a is the loader's *discipline*, and that is what `Load` still does:

- `os.Stat` the directory, then `loadManifest(ruleset.json)`, then `loadV2`, which reads `conditions/`, `atoms/`, `abilities/` (as compositions), and `guide.md` — a missing `guide.md` is a load error.
- Every file goes through `decodeStrict` (`load.go:1046`): `json.Decoder` with `DisallowUnknownFields()`. An unknown key is a rejection, not a shrug.
- Every field is checked by hand and every error names file and field through `fieldErr`. Measured today: **83 `fieldErr` call sites in `load.go`, 37 in `compile.go`.**
- Every expression in the ruleset is `Parse`d at load, so a loaded `Ruleset` can never fail to parse an expression at runtime. Dice are rejected at load in the two positions whose rolls are never recorded — a resource's `default_max_expr` (`load.go:256`) and a threshold's `when` (`load.go:288`).
- Cross-references are resolved at load: attributes, defenses, resources and condition ids must all be declared.
- `Ruleset.Compiled` is always populated on success, and `Resolve` reads it exclusively. `Ruleset.Abilities` no longer exists — 5c removed it.

### The expression language

`expr.go` is a hand-written lexer, recursive-descent parser and evaluator with no dependency outside `fmt` and `strconv`. The productions this arc froze still hold verbatim:

```
expr    := term (('+'|'-') term)*
term    := factor (('*'|'/') factor)*
func    := ('floor'|'max'|'min'|'half') '(' expr (',' expr)* ')'
IDENT   := [A-Za-z_][A-Za-z0-9_]*
```

`factor` and `ref` were widened by 5c (expression-sized dice, `@caster.`/`@target.` scopes). Verified on the current tree: arity is enforced at parse time from `funcArity` (`expr.go:971` — `floor`/`half` exactly 1, `max`/`min` at least 2); there is deliberately no unary minus (`expr.go:1151`, `TestParseRejectsUnaryMinus`); arithmetic is integer-only with `/` as floor division, so `floor(-7/2) = -4` and `half(-7) = -4` (`TestEvalFloorDivisionNegative`, `TestEvalHalfNegative` — both run and pass); recursion is capped at `maxExprDepth = 200` (`expr.go:895`); dice bounds are count 1..100, sides 1..1000 (`expr.go:879-882`), checked at parse time for literal operands and at eval time for computed ones.

### `Resolve`

Pure, and provably so by construction: `resolveState` layers a `running map[resKey]int` and a `condOverride` map over `engine.State`, which is never mutated (`resolve.go:345-360`). Validation runs in the documented order and the code matches it: ability known → actor exists → at least one target, no more than `max_targets`, no duplicate target id, every target exists → every `@`-ref in `Roll`/`Vs` present on the actor its scope names → usage resource present on the caster with enough current → Chebyshev range from the caster's token, every target in that same scene. `chebyshevDistance` (`resolve.go:689`) is `max(|dx|,|dy|)`, so a diagonal is distance 1. Output order is fixed: `AbilityUsed` first, then the usage-spend `ResourceChanged`, then per-target outcome events in target order, then threshold-driven condition events. Thresholds fire only for `(actor, resource)` pairs that actually changed, in **first-touch change order** then declaration order (`evalThresholds`, `resolve.go:509`). Every value crossing to the wire goes through `int32Checked` (`resolve.go:628`); the usage-spend path is deliberately exempt and says why.

### The schemas

`internal/rules/schema/` holds **four** JSON Schema documents (`ruleset`, `ability`, `condition` from this arc; `atom` from 5c). `Load` never reads them. `rules.Schemas` has exactly one reader in the whole repo: `internal/rules/schema_test.go`. There is no JSON-Schema validator among the module's direct dependencies.

---

## 3. Besluten och varför

**The expression language is parsed, not handed to a library.** Rejected: an off-the-shelf Go expression evaluator (CEL, `expr-lang`, or similar). The reason is not dependency weight — `google/cel-go` is already in the module graph as an indirect dependency via `buf` — it is that a general evaluator would have given the wrong *shape* of answer three times over. First, dice are a term in this grammar, not a function call: `1d20+@brawn` must roll through a caller-supplied `Roller` and hand back the individual die results, because spec §2 decision 3 ("rolled once, recorded forever") requires those results on the `AbilityUsed` event so replay never re-rolls. Second, the grammar had to be *closed* so that `Load` can prove, at load time, that no expression in a ruleset will ever surprise `Resolve` — which means the parser must be able to report unknown identifiers against the manifest's declared names with file and field, and must reject dice in the two positions whose rolls are not recorded. A library's parse tree is not built to be interrogated that way. Third, a ruleset is author-supplied content, some of it destined to be written by an LLM: a closed grammar with an explicit depth cap, explicit dice bounds and explicit integer-literal range checks is a much smaller thing to reason about than a Turing-adjacent evaluator's surface. Spec §11 froze the grammar extensions as a deliberate deferral, and 5c's widening a day later went through the same hand-written parser — which is the evidence the choice held.

**The loader hand-decodes and hand-validates, and the JSON Schemas ship anyway.** Rejected: `santhosh-tekuri/jsonschema` (named in the plan's Tech Stack as the known candidate, with the controller's sign-off required to adopt it). The plan deliberately left this open — "DECIDED AT TASK 4" — and Task 4 decided against the dependency, for the platform's stated reason: the errors this loader must produce are file-and-field errors a ruleset author can act on, and it must enforce rules a JSON Schema cannot state at all (an expression's grammar, a cross-reference to a declared name, a dice term in a position that does not record rolls). A schema validator would have been a second, weaker gate in front of the real one. But the schemas still ship, as *documents*, because the format needs an external contract for authors outside this repo — that is ADR-007's note that ruleset content schemas are JSON Schema. The obvious failure mode of that arrangement is the two drifting apart, and this arc built the countermeasure in the same commit: `schema_test.go`'s nested-`required` walker, which descends every schema document through `properties`, array `items` and `$ref`-to-`$defs`, and for each required field it finds, deletes that field from whichever valid fixture demonstrates the structure and asserts `Load` rejects it. The walker's own comment records why it exists — it was written after three fields (`resources[].thresholds[].remove_when_false`, `usage.limited.cost`, `targeting.range`) turned out to be declared required by a schema and quietly not enforced by the loader.

**Atomicity is a batch, not a sequence of appends.** Rejected: emitting one event at a time from the gateway. One ability use produces testimony plus resource changes plus condition events; a partial write would leave the log describing an attack that half-happened. `AppendBatch` takes one transaction with contiguous sequences, notifying only after commit. Spec §2 decision 2 called this data-integrity-core work and it got that review depth.

**The engine verifies `new_value`; it does not compute it.** Rejected: letting the engine do the arithmetic and the interpreter emit only a delta. The interpreter computes the post-clamp value and writes it into the event; `engine.Apply` recomputes the clamp independently and *rejects* on mismatch. Two implementations of the same rule, disagreeing loudly, is what keeps the log's testimony honest. It is also why `applyDelta` (`resolve.go:381`) carries the comment "EXACTLY engine.Apply's ResourceChanged clamp" — that duplication is deliberate, and the pairing is the invariant.

**Duplicate `target_ids` are rejected, not deduped.** Rejected: silently collapsing them. Patrik ruled this on 2026-07-25 (spec §12 amendment 6). The execution loop applies per-target outcomes once per entry, so a repeated id would concentrate the ruleset author's `max_targets` fan-out onto a single actor from the wire. Deduping would have changed a caller's intent behind their back; rejecting never does.

**Ability-outcome condition changes are idempotent.** Rejected: emitting the event unconditionally. Spec §12 amendment 1. A re-applied condition is rejected by the engine fold, and a rejection inside an atomic batch dooms the whole batch — so a harmless no-op at the ruleset level would have become a total failure at the log level.

**The conformance suite is driven by ruleset data, not by per-ruleset test code.** Rejected: a Go test per ruleset. `Run(dir)` knows nothing about any game system: it loads, smoke-resolves every declared ability against a fixture actor synthesized from the manifest's own declarations, and replays every golden the ruleset ships under `goldens/*.json` with a fixed roll sequence. That genericity — not tavern-brawl's content — is the P4 proof ADR-004 demands, and it is why 5b's `dnd45e-minimal` could pass the same suite untouched.

---

## 4. Vad som visade sig fel

Using `docs/verification-debt.md`'s labels where they fit.

**The P4 proof did not prove what it claimed.** Spec §8 requires "golden scenario per ability (fixed-seed rolls → exact expected event batch)". What `3c249a3` shipped was a loop over `goldens/*.json` — it ran whatever goldens existed and asserted nothing about coverage. The arc's own final review caught it; `37c8d8b` added the coverage check and the missing `splash-of-water` golden, with the reasoning in the code: *"Enforce that every declared ability has at least one golden — otherwise the forever-gate can silently lose all its pins (a goldens/ rename, a .JSON typo, an accidental deletion) with zero signal."* Label: `test asserts nothing`.

**A comment was false in the commit that shipped it.** `crypto_roller.go:16-17` says: *"Tests use the package's own deterministic test Rollers instead (never this type)."* The same commit, `22e2581`, added `internal/rules/crypto_roller_test.go`, which calls `rules.NewCryptoRoller()` at lines 24, 53 and 67. The sentence was never true. It is still in the tree.

**Four defects the arc's own final review found before merge** (`37c8d8b`, all with tests):
- `AppendBatch` validated the batch by folding it against a snapshot with `Sequence == 0`. `SessionEnded` writes `EndSeq = env.Sequence` and treats `EndSeq == 0` as "still open", so a batch containing one validated differently than it live-applied — the commit calls the result "the double-SessionEnded brick". The fix folds clones stamped with provisional contiguous sequences `head+1..head+N`, and the comment now carries the proof that those equal what the store will assign (same lock ⇒ same head).
- `Resolve` truncated to int32 silently. A legal, loadable expression can evaluate past int32; truncating either broke Resolve↔engine clamp parity or, in the mod-2³² corner, wrote false `Delta` testimony into an append-only log. `int32Checked` and an int64 clamp in the engine closed it.
- Duplicate `target_ids` were accepted.
- Dice were legal in `default_max_expr` and threshold `when`, drawing from the `Roller` without recording — a direct breach of spec §2 decision 3. Banned at load.

**What escaped the arc entirely, and was found weeks later by the mutation gate:**
- *The smoke test's own proof was a substring.* `TestRunSmokeFailure` (this arc's, `3c249a3`) asserted only that the error mentioned the failing ability `big-move`. That fixture produces such an error by two different routes, and with the fixture actor's resource max falling back to 1000 instead of its declared 1, the smoke pass *succeeded* and `Run` failed later with `has no golden scenario` — still naming `big-move`, still green. Six of ten surviving mutants lived behind that one test (`aff5f23`, 2026-08-04). Label: `test asserts nothing`.
- *A tie was never played.* `hit := total >= vsTotal` — every hit test cleared the defence outright and every miss fell short, so equality, the one input where `>=` and `>` disagree, was never exercised. Whether an attack that exactly equals the defence connects is a game rule, and nothing held it (`492a776`, 2026-08-05). Label: `test data missing`.
- *Vertical distance was never measured.* Seventeen of eighteen range tests placed both tokens on `y=0`, so `chebyshevDistance`'s `dy` branch had effectively never run. Negated, a target two squares north reads as distance −2 and reach becomes unlimited along one axis (same commit). Label: `test data missing`.
- *`int32Checked`'s inclusive bounds were only ever approached from outside.* Every test drove a value past the range, so `<` vs `<=` at exactly `MinInt32` was untested; under either mutant the widest legal resource change is refused as "outside the int32 wire range", the opposite of true (`715fbcb`).
- *Eight invalid-fixture assertions passed on their own directory name*, because every `fieldErr` names the path. Three of the eight are this arc's fixtures: `duplicate-ability-id` asserting `"strike"`, `duplicate-condition-id` asserting `"guarded"`, `unknown-field` asserting `"ruleset.json"` (`37b22ac`). Label: `test asserts nothing`.
- The package was gated at `3774ea6` (2026-08-05): 553 mutants, 49 survivors down to 16, all sixteen adjudicated equivalent. That commit also corrected two published numbers about this package — the mutant count had been published pre-exclusion as 613 rather than 553.

**Claims in the specs that were later withdrawn:**
- Spec §6 said: *"Undo interaction: a batch is retractable as a range like any events (the existing range machinery already covers contiguous spans)."* Amended in place 2026-08-30: *"there is no undo interaction, because there is no undo."* Retraction left the platform on Patrik's ruling; `campaign.Undo` and the range machinery went with it in `133e896`. The plan's Task 3 carries the same correction inline.
- The spec's own header now reads: *"Superseded (2026-07-26): format v1 defined here is superseded by format v2 ... §§4-5's format/expression definitions are historical."* The execution semantics survive; the format does not.

**Prose in this arc's code that has rotted since:**
- `format.go:9-10` says `Load` "fully validates one (**schemas**, cross-references, and every expression's grammar)". `schema.go:8-10`, ten lines away in the same commit, says "Load itself does not run a JSON Schema validator against these ... it hand-decodes and hand-validates." The second is true. The first has misled at least one reader into looking for schema validation in `load.go`, where the string "schema" appears once, in a comment.
- `crypto_roller.go:13` cites "resolve.go's `evalRecording`". The function is `evalRecordingScoped` (`resolve.go:585`) — renamed by 5c's `1f17166`.
- `expr.go:740` cites "the resource-naming note in the task-4 report". `.superpowers/sdd/` is git-ignored and `task-4-report.md` has since been overwritten by a different sub-project's Task 4 report (it is now `internal/engine — State and the single fold`). The note it cites is gone and cannot be recovered from the repo.
- `schema_test.go:189-190` names three fields the loader failed to enforce: "`resources[].thresholds[].remove_when_false`, `usage.limited.cost`, `targeting.range`". The first two are still walked by the test today; `targeting` was removed from `ability.schema.json` by 5c and the third example now names a field that does not exist.

**One thing that was right and stayed right.** There is no benchmark claim anywhere in `internal/rules` to re-run — I checked; the package has no `Benchmark` function. The measurement I could re-run is the territory count itself: 1,492 comment lines against 3,083 code lines, 32 blocks of ten or more. It reproduced exactly.

---

## 5. Spår

No mechanical link from plan to commits exists in this repo. The chain that established the range:

1. `git log --oneline --follow -- internal/rules/` → oldest entry `c70bf2b`, "feat: internal/rules — ruleset format v1, loader, closed expression grammar".
2. `git log --merges --oneline --ancestry-path c70bf2b..main | tail -1` → `83be590`, "Merge feat/ruleset-interpreter: sub-project 5a — ruleset loader & rules interpreter". `git log --merges --reverse` confirms it is the repo's first merge, so the brief's warning applies but resolves in this arc's favour: **`83be590` is this arc's.**
3. `git rev-parse 83be590^1 83be590^2` → `0434d5e` (main side: "docs: ruleset interpreter spec and plan (sub-project 5a)", 02:56) and `68008ef` (branch tip: "docs: sub-project 5a merge-gate spec amendments", 09:47).
4. `git log --oneline --no-merges 0434d5e..68008ef` → eight commits, in order: `09a7cab` contract, `af9f4de` engine folds, `03096d3` AppendBatch, `c70bf2b` format/loader/grammar, `3c249a3` Resolve/tavern-brawl/conformance, `22e2581` wiring, `37c8d8b` final-review fix wave, `68008ef` spec amendments. Four touch `internal/rules`.

The neighbouring arc, for the boundary: `git rev-parse 5fbc666^1 5fbc666^2` → `26a0b9a..4dee79d`, six commits (`7e90b7e`, `64a7131`, `1f17166`, `2ac4ea9`, `13d0f61`, `4dee79d`), merged 2026-07-26 as sub-project 5c. Ownership of every comment block below was settled by `git blame -L <range> --line-porcelain`, counting lines per commit, not by reading the prose.

---

## 6. Kommentarsblock denna rapport friar

Block boundaries are contiguous runs of comment lines in the eight non-test Go files of `internal/rules`, measured on the current tree. Ranges will shift as the files are edited; the anchors given are function and identifier names.

### Freed by this report (8 whole, 2 partial)

**`internal/rules/format.go:1-12`** — package doc: what `internal/rules` is and what `Load`/`Resolve` do.
*Stays:* no game-system word may appear anywhere in this package — that is ADR-002/004 and `.semgrep/vocabulary.yml` enforces it over `internal/` and `cmd/`; a ruleset is a directory `rulesets/<id>/` of JSON files plus a `guide.md`; `Load` strictly decodes and validates cross-references and every expression's grammar; `Resolve` is pure.
*Goes:* the word "schemas" in the parenthetical (false — see §4), and "(Task 5)" as a pointer into a plan.

**`internal/rules/schema.go:5-14`** — why four JSON Schema documents exist that `Load` never reads.
*Stays:* `Load` does not run a JSON Schema validator against these; it hand-decodes and hand-validates. These documents are the external contract for ruleset authors. `schema_test.go` is the only thing keeping the documents and the loader from drifting — change a `required` array in a schema document and the walker test will demand the loader reject that field's absence.
*Goes:* the spec §4 quote and the "task-4 controller decision" pointer.

**`internal/rules/expr.go:8-161`** — *partial.* The grammar block is jointly owned: 123 lines are 5c's (`7e90b7e`, scoped refs and expression-sized dice), 22 lines are `c70bf2b`'s and 7 are `37c8d8b`'s. This report frees only the 5a-authored sentences: the `expr`/`term`/`func`/`IDENT` productions (lines 12-13, 19-20), the arity paragraph (130-136), the integer-arithmetic paragraph (138-143) and the no-dice-in-unrecorded-positions paragraph (147-154). The "Dice chaining WITHOUT parentheses", "Tokenizing 'd'" and "Scoped refs" sections belong to the format-v2 report and are left for it.
*Stays, from the 5a share:* the four productions verbatim; arity is checked at parse time, so it is part of the load-time surface (`max`/`min` ≥ 2 args, `floor`/`half` exactly 1); arithmetic is integer-only and `/` is floor division, not Go's truncate-toward-zero, so `floor(-7/2) = -4` and `half(x) = floor(x/2)` gives `half(-7) = -4` — a deliberate choice pinned by `TestEvalFloorDivisionNegative` and `TestEvalHalfNegative`; dice are rejected at load in `default_max_expr` and threshold `when` because a die there would draw from the `Roller` unrecorded.
*Goes:* the "extended from v1 with", "unchanged from v1" and "(v1 load-time restriction, unchanged by v2)" framing — the rule is current, the version history around it is not.

**`internal/rules/expr.go:735-744`** — `isValidIdentName`: why the loader reuses the lexer's own charset.
*Stays:* a manifest attribute/defense/resource name that does not match IDENT can never be referenced via `@`/`#` — a hyphenated name lexes as subtraction, not one identifier — so the loader rejects it; and this check is built on the lexer's own `isIdentStart`/`isIdentCont` precisely so it cannot drift from what `Parse` accepts after a sigil. Do not replace it with a hand-written regex.
*Goes:* "see the resource-naming note in the task-4 report" — that file is git-ignored and has been overwritten.

**`internal/rules/crypto_roller.go:9-18`** — `CryptoRoller`: why production rolling has no seed.
*Stays:* dice are rolled exactly once, at `Resolve` time; the per-die results `Roll` returns are what `Resolve` records onto `AbilityUsed.Rolls`, and that recorded testimony — never a second call to `Roll` — is what any later replay observes; `CryptoRoller` is wired in at exactly one live call site, `internal/gateway/server.go:302` (verified today).
*Goes:* "(Task 6 wiring)"; "resolve.go's `evalRecording`" (the function is `evalRecordingScoped`); and "Tests use the package's own deterministic test Rollers instead (never this type)" — false, and false in the same commit.

**`internal/rules/crypto_roller.go:26-38`** — `Roll`: the degenerate-sides guard and the panic.
*Stays:* `sides <= 0` is treated as a degenerate always-1 die rather than dividing by zero, so a caller that bypasses the parser gets a harmless answer instead of a crash; and `crypto/rand.Int` failing panics deliberately, because the `Roller` interface has no error return through which a genuine entropy failure could be propagated — do not swap that panic for a zero result.
*Goes:* "already bounded by the grammar's **parse-time** DICE limits" — the bound still holds at the call, but since 5c a computed dice operand is bounded at eval time (`expr.go:601-605`), not at parse time.

**`internal/rules/conformance/conformance.go:1-57`** — the P4 proof harness, and the golden-file format it owns.
*Stays:* essentially all of it, and the golden JSON skeleton most of all — it is the authoring contract for `goldens/*.json` and it matches `goldenFile` field for field today. Specifically: `Run` is driven entirely by what `Load` returns and by data the ruleset itself ships, and that genericity — not any one ruleset's content — is the P4 proof; the `rolls` array is one entry per expression that *actually rolls*, in the exact order `Resolve` evaluates them, per target; `want_error` and `want_events` are XOR, and `want_events` must match the produced batch exactly, field for field, in order.
*Goes:* "(ruleset-interpreter spec §8)" and "5b's dnd45e-minimal" written in the future tense — 5b shipped 2026-07-26. Read "every declared ability" as "every key of `rs.Compiled`".

**`internal/rules/conformance/conformance.go:162-176`** — `buildFixtureState`: why the stand-in actor looks the way it does.
*Stays, nearly whole* — this block is already the corrected version (`aff5f23` rewrote six of its lines) and every sentence is load-bearing: both fixture actors carry *every* declared attribute and defense, because a smoke test cannot know which role an ability expects of which actor; a `default_max_expr` that evaluates to exactly **0** falls back like a missing expression, because a stand-in that can afford nothing makes every smoke failure a statement about conformance rather than about the ruleset (pinned by `minimal-v2-zero-default-max` — do not "simplify" the `> 0` to `>= 0`); both tokens share one grid cell so every declared range, including 0, is satisfied.

**`internal/rules/resolve.go:13-132`** — *partial.* 44 lines are `3c249a3`'s and 4 are `37c8d8b`'s; 63 are 5c's `1f17166` (the "ONE execution path" section and the v1-adaptation narrative). This report frees the 5a share.
*Stays:* `Resolve` is pure — it never mutates `st`, never touches store or campaign, and is deterministic given `rng`; every returned envelope has **only** its `Payload` set, and `Sequence`/`EventId`/`SessionId`/`ActorRole`/`ParticipantId`/`OccurredAt` are the caller's to stamp; the five-step validation order, each step a clean error returning nothing; range is Chebyshev from the caster's token, all targets in that same scene, so range 0 accepts only the caster's own cell and a diagonal is distance 1; `Resolve` never creates a resource entry — the resource must already exist on the actor; outcome-list expression errors are deliberately *lazy* and abort the whole batch the instant they occur, so no partial batch is ever returned; `apply_condition`/`remove_condition` outcomes are idempotent, because a duplicate or absent condition event would doom the atomic batch at the engine fold; thresholds are evaluated only for pairs that changed, in first-touch change order then declaration order; the batch order is `AbilityUsed`, usage spend, per-target outcomes in target order, threshold events.
*Goes:* "(Task 3, spec 5c §6-7)", and every "v1-adapted" / "matching v1's own `>=`" / "keeping every existing v1 golden byte-identical" clause — those are the format-v2 report's to free, and format v1 no longer loads.

**`internal/rules/resolve.go:609-624`** — `int32Checked`: why the wire range is enforced inside `Resolve`.
*Stays:* `Resolve` computes in `int` but `ResourceChanged.delta`/`new_value` and `AbilityUsed_Roll.total` are int32, and a legal loadable expression can evaluate past int32; truncating would either poison Resolve↔engine clamp parity (the engine independently recomputes and rejects a mismatch) or, in the mod-2³² corner, write false `Delta` testimony into an append-only log. The usage-spend `ResourceChanged` is deliberately exempt: `cost <= current` is already proven by the insufficient-resource guard and `current` is an int32, so both provably fit. The range is **inclusive** at both ends and is pinned from inside as well as outside (`715fbcb`) — `<` must not become `<=`.

### Not freed — other arcs' to free

Twenty-one of the 32 blocks belong to sub-project 5c (`docs/superpowers/plans/2026-07-25-format-v2-composition.md`, merged `5fbc666`) and are left for that report: all five in `compile.go` (3-28, 301-341, 441-450, 654-695, 712-721); `conformance.go:503-520` (compiled-form goldens); six in `expr.go` (284-293, 378-388, 481-490, 823-838, 981-998, 1136-1153); three in `format.go` (74-92, 179-206, 214-225 — `Atoms`/`Compiled`, `AtomDef`, `ParamDef`); three in `load.go` (32-55 the `Load` doc, 215-225, 547-556); three in `resolve.go` (170-179, 445-459, 574-584). `expr.go:8-161` and `resolve.go:13-132` are shared, as noted above.

One block belongs to neither: **`load.go:425-435`** (`checkExprRefs`), written at `37b22ac` (2026-08-05) during the mutation-gating work. It is itself a correction — *"An earlier version of this comment claimed compile.go's v2 cross-reference checks shared it; they do not"* — of a sentence 5c wrote at `64a7131`. It should be freed by whoever reports the gating arc.

Finally, one observation outside the ≥10-line inventory, offered as evidence rather than claimed: `resolve.go:642` cites "`resolve.go:473,481`" for the two `int32Checked` calls. Those line numbers were correct when written (at `a7b3bd3`, the golangci-lint arc) and are now `targetCtx := ...` and a closing brace; the calls have drifted to 478 and 486. The substance of the claim is still true. A line-number citation in this repo has a half-life of about a month.
