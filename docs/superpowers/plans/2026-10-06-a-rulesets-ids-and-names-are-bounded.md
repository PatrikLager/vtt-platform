# A ruleset's ids and names are bounded — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-06-a-rulesets-ids-and-names-are-bounded-design.md`
**Verified:** 2026-10-06 by `verify-ticket` (dev-cycle 0.4.0). The verifier is
an agent that did not write the ticket. It verified against `fcd367c` on
`feat/ruleset-names-are-bounded`, which equals `main`, with the ticket
untracked. Verdict: **Passes with gaps.** The gaps are listed at the end and
travel with this plan. This plan does not edit the ticket.

**Goal, in the ticket's words:** `rules.Load` refuses a ruleset that declares
an ability id, a condition id, or an attribute, defense or resource name longer
than the bound, naming the file and the field, and loads one whose every such
string is exactly the bound. Both folds refuse a `ConditionApplied` whose
condition id, and an `AbilityUsed` whose ability id, is longer than the bound,
and accept one of exactly the bound. The debt entry's recipe is refused at
load. Every shipped ruleset and every fixture meant to load still loads, every
golden still folds, and `task check` whole is green.

**Where this meets the second ticket.** The owner split the remaining
unbounded id-like strings by source. This ticket takes the ruleset-sourced
ones. The second takes the command- and file-sourced ones: participant ids,
`module_id`, an actor's `resources` and `attributes` keys from `add_actor` and
the adventure loader, scene object ids, session ids, and `TokenMoved`'s
`scene_id` and `from`. They meet where a resource, attribute or defense name
is also an actor key. What this ticket does there, and what it leaves, is D7.

**MapTool (CLAUDE.md rule 9): a vocabulary is declared once, checked for
emptiness and uniqueness, and never for length.** Read in
`~/dev/RPTool/maptool` at `f4b7fef6c`:

- A campaign's vocabulary lives in its Campaign Properties and is shipped whole
  to every client: token property names, and token states, MapTool's nearest
  thing to a condition.
- A declared name is checked for emptiness and uniqueness only.
  `TokenPropertiesManagementPanel.parseTokenProperties` trims each line, skips
  an empty one, and refuses a case-insensitive duplicate (`CaseInsensitiveHashMap`,
  `msg.error.mtprops.properties.duplicate`). `TokenStatesController` enables
  Add only when the trimmed name is non-empty and not already declared
  (`getNames().contains(text)`). Nothing measures a length.
- A use is checked against the declaration. `TokenStateFunction` refuses a
  state the campaign does not declare
  (`getTokenStatesMap().containsKey(stateName)`, `unknownState`) and echoes
  the name it was given.
- There is no frame to protect. `AbstractConnection.readMessage` reads a 4-byte
  length prefix and allocates that many bytes, so any name arrives.

**Borrowed:** the declaration is the authority, and every use is checked
against it by membership. That is what makes a bound on the declaration bound
every use here: `Resolve` emits only declared names (`compile.go` refuses an
undeclared one), and the adventure loader admits only declared names as actor
keys. **Not borrowed:** MapTool's absence of a length bound. It can afford
none, because its frames carry any length. Here the server reads at most
`maxWSFrameBytes` (32768), and a Go client reads 32 KiB unless it raises the
cap (`internal/harness/client.go`'s `readLimit`, 200 KiB). **Checked and
rejected:** trimming and case-folding a declared name. That would rename an
author's id without telling them.

## Verification, check by check

1. **Every path resolves (by command).** `[ -e ]` finds all 21 paths the
   ticket names: 14 files and 7 directories. `rulesets/` holds exactly
   `dnd45e-minimal` and `tavern-brawl`. `grep` finds each named symbol where
   the ticket puts it:
   - `func Load` in `internal/rules/load.go`, `func Resolve` in `resolve.go`;
   - `func Apply` and `maxIDBytes = 128` in `internal/engine/apply.go`;
   - `handleUseAbility`, returning `err.Error()`, in `internal/gateway/ruleset.go`;
   - `"minLength": 1` on `ability.schema.json`'s and `condition.schema.json`'s
     `id`, and `^[A-Za-z_][A-Za-z0-9_]*$` three times in `ruleset.schema.json`;
   - `ability:%s:usage`, `ability:%s:%s` and `threshold:%s` in `resolve.go`;
   - the debt entry "A ruleset's names reach a `use_ability` refusal at any
     length".

   One claim is false. The problem paragraph says `rules.Load` "validates a
   ruleset against the JSON schemas in `internal/rules/schema/`". It does not:
   `internal/rules/schema.go`'s doc says "Load itself does not run a JSON
   Schema validator against these", and the schemas are documents that
   `schema_test.go` cross-checks against the loader (Gap 1).
2. **"Done" is an observation (by command).** None of the named tests exists
   yet. Sketches of each, in a scratch clone, fail on the base and pass on the
   probe tree (P1, P6):
   - Item 1: eight load subtests red with the ruleset loaded (ability id,
     condition id, each also multibyte, attribute, defense, resource, branch
     label).
   - Item 2: four engine cases red with `err = <nil>`: a 129-byte and an empty
     condition id, a 129-byte and an empty ability id.
   - Item 3: four TS cases red with "Received function did not throw".
   - Item 4: the recipe loads on the base.
   - Item 5 is the gate. Every Go package and every TS file is green on the
     probe tree (P6).

   The at-bound halves of items 1 to 3 cannot fail today, as the ticket says.
   K7 to K12, K18, K19, K28 and K29 hold them.
3. **Each rule is breakable (a reading, then probes).** Each has a single edit
   that reds a named sketch test. K1 to K34 and K36 were run on the probe
   (P14). K35 needs C1's tree.
   - The first rule becomes row A, and B if Q2 is (b).
   - The second becomes row C, and D if Q4 is (b).
4. **The scope matches the claim (by command, then a reading).** Roughly, with
   these differences:
   - `rules.Load` has four production callers (`cmd/vtt/serve_compose.go`,
     `cmd/vtt/mcp.go`, both conformance packages) and 18 test files. In the
     59 tracked `ruleset.json` directories, every id, name, label and
     expression is at most 37 bytes; only a condition's description is
     longer (P2). `vtt mcp` refuses to start on a load failure too, as
     `serve` does.
   - **An adjudication must be deleted, not re-keyed.** The TS `"abilityUsed"`
     label mutant (`client/src/fold.ts 462:10 StringLiteral ""`) becomes
     killable once its arm checks something. `check-ts-mutation.py` fails an
     adjudicated mutant that is killed ("Remove the entry rather than let it
     pre-approve a future mutant"). The neighbouring `default:` entry's reason,
     "shared with the two no-effect event kinds above it", becomes false.
   - **A test comment becomes false.** `client/test/fold-unit.test.ts` says
     `attackRolled` "shares its arm BODY with `abilityUsed` and
     `adventureLoaded`". That is already false for `adventureLoaded`, and this
     change makes it false for `abilityUsed`.
   - **A second debt entry miscounts.** "Nothing ties a byte bound's copies to
     the engine's constant" counts "fourteen bounds" and lists the copies;
     this change adds two bounds and a copy.
   - **`cmd/vtt/webdist/assets/index.js`** is rebuilt after `fold.ts` changes.
   - **Two items listed are not touched.** `tools/comment-ceilings.txt` needs
     no row changed: every touched share falls 0.1 to 0.6 points, inside the
     1.0 band (P10). "The mutation adjudication keys … in `internal/rules`":
     `load.go` holds none, and Q6(b)'s edit to `resolve.go` replaces two lines
     in place (P8).
   - **`loadManifest` is under `//nolint:gocyclo` at 32.** Three inline checks
     make it 35. A pre-pass makes it 33 (P7, D3).
   - **The `use_ability` debt's symptom survives names alone.** A threshold's
     `when` expression reaches a `use_ability` refusal whole: 70,016 bytes in,
     70,105 bytes out (P12). The ticket says the entry closes for names only;
     the owner asked for the debt closed (Q6).

   `engine.Apply`'s callers are unchanged. The four events have one producer in
   production code, `rules.Resolve`, apart from two. The projection's
   introductions and corrections re-send a folded condition id or resource
   name, with no source and no reason (SPEC-016). `remove_condition`'s
   `ConditionRemoved` must name a held condition.
5. **No recorded decision is contradicted silently (a reading).** SPEC-018
   records that these fields have no bound in the fold, and the ticket names
   SPEC-018 as moved. The decision `rules.Schemas`' doc records, that `Load`
   hand-validates and runs no schema validator, is misdescribed by the problem
   paragraph but not overturned. This plan keeps it (D9). SPEC-016's
   introduction sentence ("carrying its id and no source") stays true, and
   forces any bound on `source` to admit an empty one (Q3). ADR-002 and
   ADR-007 are untouched: no proto changes.
6. **The records the work moves are named (by command, then a reading).** The
   section lists SPEC-018, and it resolves. The reading:
   - `grep` over `docs/specifications/` for thresholds, conditions, abilities,
     `use_ability`, the ruleset and the schemas finds SPEC-012 (the read
     surface), SPEC-013 (`use_ability`'s and `remove_condition`'s
     authorization) and SPEC-016 (what a seat is sent). None holds a sentence
     this change makes false, Q6(b) included: no specification quotes the
     threshold refusal.
   - Outside `docs/specifications/`, the second debt entry, the TS adjudication
     file and the `fold-unit.test.ts` comment become false (check 4).

## Measurements this plan stands on

Every measurement was taken by command, never in the working tree. Three
scratch clones of `fcd367c` (`git clone --no-hardlinks`, `bun install
--frozen-lockfile`):
- `base`, left as committed, for baselines and red sketches;
- `instr`, `engine.Apply` and `fold.ts`'s `apply` wrapped to log every
  condition, ability and resource string that is empty or over 20 bytes;
- `probe`, the sketch of D1 to D8 under Q2(b), Q4(b) and Q6(b), with the keys
  of D13 re-pointed. D9's schema sentences were not sketched.

The sketch tests were named `zz*`. All three clones were deleted afterwards.

| # | What | Result |
|---|---|---|
| P1 | Base. Sketch tests over each Done item | Rules: every 129-byte case loads (ability, condition, each also as `é`×64 + `i`; attribute, defense, resource, branch label), and so do 70,000-byte names. The recipe loads, and `Resolve` returns a 70,089-byte refusal, as the debt entry says. Engine: a 129-byte, a 10,000-byte and an empty condition id fold with a 100,000-byte source; a 129-byte and an empty ability id fold, by an actor that does not exist. TS: the four refusal cases fail with "did not throw"; the at-cap case passes. |
| P2 | Every string in the 59 tracked `ruleset.json` directories, by kind (`git ls-files '*ruleset.json'`, then every condition, atom and ability file beside each) | Longest: ability id 16 (`longsword-strike`), condition id 12 (`dazed-by-ale`), attribute 6, defense 7 (`footing`), resource name 11 (`flurry_uses`), branch label 8 (`only-one`), outcome branch 7, atom id 21, param name 11, graph key 10, ability name 18, condition name 13, condition description 370, `roll` 37, `vs` 23, `delta_expr` 34, threshold `when` 29, manifest id 35, manifest name 22. |
| P3 | Every `conditionApplied`, `conditionRemoved`, `abilityUsed` and `resourceChanged` in the 945 tracked JSON files that parse | Ability id at most 15 (`reckless-strike`), condition id 12, source 15 (`threshold:drink`), resource 5, reason 20 (`ability:rally:effect`). None empty, none absent. |
| P4 | `instr`, Go. `go test -count=1 -p 2 ./internal/... ./contract/... ./tools/... ./cmd/...` | 21 packages ok (`cmd/vtt` 165 s). **No `ConditionApplied` with an empty or over-20-byte condition id, no `AbilityUsed` with an empty or over-20-byte ability id, no `ResourceChanged` with an empty or over-20-byte resource is folded anywhere.** Condition ids run 1 to 12 bytes. Sources: 1,326 folds of an empty one (the projection sends none, and many tests write none), the longest 27. `ConditionRemoved` reasons at most 30. |
| P5 | `instr`, TS. Each of the 36 `client/test` files and `contract/events.test.ts` run alone; the hook proved by folding an empty ability id | All 37 exit 0. Nothing logged: no empty or over-20-byte condition or ability id, and no over-20-byte source, is folded by any TS test. |
| P6 | The whole probe tree | `go test` as P4: 21 packages ok (`cmd/vtt` 166 s, the scenario goldens and both conformance suites included). `bun test client/test contract contract-spike`: 1011 pass, 0 fail (1006 on the base, plus the sketches). `bunx tsc --noEmit -p client/tsconfig.json` clean. `semgrep scan --config .semgrep/ --error internal/ cmd/` exit 0. `go-arch-lint check` OK. `golangci-lint run ./internal/rules/... ./internal/engine/...`: production code clean; one `thelper` finding on a sketch helper, so every real test helper begins with `t.Helper()`. |
| P7 | `gocyclo` on `internal/rules/load.go` and `apply.go` | Base: `loadManifest` 32 and `decodeOutcomeContribution` 31, both under `//nolint:gocyclo`; `decodeResolutionContribution` 16, `loadAtoms` 15, `loadCompositions` 14, `loadConditions` 8; `Apply` 108, under `//nolint:gocyclo`. Inline checks: `loadManifest` 35. With the pre-pass: `loadManifest` 33, `checkManifestBounds` 7, `decodeResolutionContribution` 17, `loadCompositions` 15, `loadConditions` 9. `Apply` 112. |
| P8 | Keys mapped base to probe by `difflib`, every line compared | `apply.go`: `351:58` unchanged; `465:15`→`468:15` and `468:30`→`471:30` (+3); `612:41`→`618:41` (+6). `fold.ts`: `385:48`→`386:48`, `414:19`→`415:19`, `423:11`→`424:11`, `424:26`→`425:26` (+1); `461:10`→`465:10` (+4); `462:10` gone; `463:5`→`466:5`, `492:7`→`495:7`, `582:34`→`585:34`, `604:32`→`607:32`, `709:7`→`712:7`, `709:21`→`712:21` (+3); `326:11`, `327:11`, `328:11` and `328:37` unchanged. `internal/rules`: `load.go` holds no key; `resolve.go` is 702 lines before and after Q6(b). |
| P9 | Both mutation self-tests on the probe | Before re-pointing: the Go self-test names `apply.go`'s keys; the TS self-test flags 9 entries and **cannot see** `385:48`, `414:19` or `604:32` move. After re-pointing and deleting `462:10`: `OK`, `OK`. |
| P10 | `check-comments.py HEAD` and `--report` on the probe | Clean: 2 added comment lines (the two pointers of D14). `load.go` 23.2 → 22.7 against 23.3; `apply.go` 41.1 → 40.7; `fold.ts` 42.5 → 42.3; `resolve.go` 38.0 → 37.9. All inside the band, so no ledger row changes. The notice on `cmd/vtt/library_test.go` is on the base too. |
| P11 | The recipe | Base: loads, and the refusal is 70,089 bytes. Probe: refused, `…/ruleset.json: field "resources[0].name": must be at most 128 bytes, got 70000`, 166 bytes with its temporary path. |
| P12 | A threshold's `when` of 70,016 bytes (`#pool_a + @brawn` and 17,500 × ` + 0`) in a copy of `testdata/valid`; `guard-stance` resolved by an actor with `pool_a` and no `brawn` | Base, and the probe with names only: it loads, and the refusal is 70,105 bytes, the expression whole. Probe with Q6(b): 88 bytes, `rules: resolve: threshold 0 on resource "pool_a": rules: expr: unknown attribute "brawn"`. `internal/rules`' own tests reach that refusal arm zero times (`-coverprofile`). |
| P13 | Probe. A copy of `testdata/valid` with a 128-byte condition, resource and ability id; the ability resolved on its caster and the batch folded by `engine.Apply` | A 3-event batch folds; the stored source is 143 bytes (`ability:<128>:effect`). With the engine's `maxIDBytes` at 127: `fold envs[0]: engine: ability id must be 1-127 bytes, got 128`. |
| P14 | The breaks, one edit each, file restored from its saved text and checked by hash | Every K from K1 to K34, and K36, red where the K table says, against the sketches. K36 must restore the range variable as well, or it does not build. K35 was not run. |
| P15 | Records and gates on the working tree | `check-requirements-chain.py .`: 282 rows, 227 test files, 13 specifications. `check-comments.py HEAD`: 268 files, clean. `check-doc-owner.py .`: 80 files. `requirement-id` is at `~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`. Free on `/System/Volumes/Data`: 34,125,596 KiB at the start and 33,603,712 KiB at the end. Load 6.0 at the start, 1.7 later. Go cache 9.2 GiB. |

## Constraints that bind every task

- **Rule 2.** No gate is weakened. No `//nolint` is added, and the existing
  ones on `loadManifest` and `Apply` cover no more than D3's single added
  branch and D4's two checks. The ledger goes down only through
  `--write-ledger`, and P10 says it need not run. A key is re-pointed by
  statement.
- **Rule 3.** No proto changes. `check:breaking` reports nothing.
- **Rule 4.** Both folds refuse the same events at the same counts, in the same
  order within each arm (D4, D5). A TS fold stricter than Go's freezes a
  client, so neither side lands alone.
- **Rule 5.** No new vocabulary. "Ability", "condition", "resource",
  "attribute", "defense" and "branch" are the contract's and the ruleset
  format's words, and semgrep passed on the probe (P6).
- **Rule 8.** Cite names, never lines. The adjudication files keep their
  coordinates.
- **Rule 9.** Answered above.
- **Rule 10 and SPEC-010.** New comments are limited to test citation lines and
  the two one-line pointers of D14. No banned term appears on an added line.
  No comment block of more than six lines gains a line: `Load`'s 24-line doc,
  `loadManifest`'s complexity note, the defense loop's 11-line block and
  `Apply`'s doc are not edited.
- **SPEC-008.** Ids come from `requirement-id`, after sign-off, in letter order.
- **Architecture.** `internal/rules` may import `engine`, `contract` and itself
  (`.go-arch-lint.yml`). Every test file this plan adds stays inside its
  package's list, and so does every QA file (D16).
- **Order and hygiene.**
  - `check:drift` passes only on a committed tree, so the order is review,
    commit, gate.
  - The review package is `git diff HEAD`, and nothing is stashed or checked
    out while a reviewer reads.
  - `git add` and `git commit` run in separate calls, with `git show --stat
    HEAD` after each.
  - A new test goes after a closing brace, never above another test's
    citation line.
  - The pre-push hook takes about three minutes; let it finish.
- The ticket, every report under `docs/reports/` and every ticket under
  `docs/superpowers/specs/` are not edited.

## Decisions this plan makes

**D1. The bound: 128 bytes of UTF-8, inclusive, and the fold's own
`maxIDBytes` (Q1).** No constant is added to `apply.go`'s const block.
`internal/rules/load.go` gains its own `const maxIDBytes = 128`, as
`internal/adventure` and `internal/mapdef` hold theirs, pinned by
`TestTheIDBoundMirrorsEngine`. What forced 128 and the one constant:
- **A condition id and an ability id are ids.** The fold already bounds the
  four ids that create an identity at `maxIDBytes`. A second id bound would be
  a second number to keep equal for no measured reason.
- **A declared name becomes an actor key.** The adventure loader admits only a
  declared resource, attribute or defense name as an actor key (its membership
  checks against `resSet` and `attrOrDefSet`). If the second ticket bounds the
  keys below the names, a ruleset could declare a name no actor can carry, and
  an adventure using it would load and then be refused by the fold. One
  constant makes the two bounds equal by construction.
- **Nothing comes near it.** The longest such string in any ruleset is 16
  bytes (P2), and no test folds a condition or ability id over 20 bytes (P4,
  P5).

The alternatives:
- **A constant of its own** costs a line in the const block, which moves all
  four `apply.go` keys by one more, and a second number the second ticket must
  match.
- **Exporting `engine.MaxIDBytes`** for `internal/rules` to read, which
  `.go-arch-lint.yml` allows, would remove one copy for good. It changes the
  engine's exported surface, and `internal/adventure`, which may import
  `engine` too, holds a copy instead. That is the owner's call (Q1(c)).

**D2. Scope: the five kinds the owner named, and a resolution's branch labels
(Q2(b)).** A branch label is a ruleset-chosen word, like a name, and it is
the one other ruleset string a `use_ability` puts into the fold's state. `Resolve` writes it into
`ConditionApplied.source` as `ability:<id>:<label>`, which both folds keep in
`st.Conditions`, and into `ResourceChanged.reason`, `ConditionRemoved.reason`
and `AbilityUsed.outcome_summary`. With labels bounded, every source a server
writes is at most 265 bytes (8 + 128 + 1 + 128).

What stays out, and why:
- **Display names.** An ability's `name` reaches `outcome_summary` once per
  target, and both names reach `/api/ruleset`. They are testimony and a read
  route, never state and never a refusal. Longest: 18 bytes (P2).
- **Expressions.** `roll`, `vs` and `delta_expr` reach `AbilityUsed.rolls` as
  testimony. A threshold's `when` reaches a refusal, which Q6(b) closes without
  bounding it.
- **Atom ids, param names and graph keys.** They reach only load-time errors
  at boot, never an event or a connection.
- **The manifest's own id and name.** The id reaches `load_adventure`'s
  ruleset-mismatch refusal only when an adventure names a different ruleset,
  and both reach `/api/ruleset`. Longest: 35 bytes. The owner's list does not
  name them.

The report raises these as a ticket.

**D3. The loader's checks.** Each refusal reads `fieldErr(path, <field>,
fmt.Sprintf("must be at most %d bytes, got %d", maxIDBytes, n))`, the
adventure and map loaders' form. The field is named by its index and the
value is never quoted:
- **Conditions** (`loadConditions`): after `raw.ID == ""`, field `id`.
- **Abilities** (`loadCompositions`): after `raw.ID == ""`, field `id`.
- **Branch labels** (`decodeResolutionContribution`, under Q2(b)): after `b ==
  ""`, field `contributes[i].branches[j]`.
- **Attributes, defenses and resources**: in a new unexported
  `checkManifestBounds(path string, raw *manifestJSON) error`, called once
  from `loadManifest` after the `format_version` check and before the
  attribute loop. Fields `attributes[i]`, `defenses[i]` and
  `resources[i].name`.

What forced it:
- **gocyclo.** Inline, the three checks take `loadManifest` from 32 to 35
  under its existing `//nolint:gocyclo`. The pre-pass takes it to 33, and
  `checkManifestBounds` is 7 (P7). The nolint's stated reason ("a long
  sequence of independent field checks") covers one more call. It should not
  be made to cover three more branches.
- **The value is never quoted.** Every refusal after the length check quotes
  the name with `%q`: charset, reserved word, duplicate. Checking the length
  first means no load refusal prints more than 128 bytes of a name. Today's
  `attributes` refusals name the field without an index and quote the name,
  so the new refusal needs the index to say which one.
- **Empty first, then length.** The order between them cannot be observed:
  an empty string is never over-long.

The ticket's "possibly the schemas" is not where the bound goes. `Load` runs
no schema validator, so a `maxLength` would bound nothing (Gap 1). D9 says
what the schemas carry.

**D4. The Go fold.**
- **`ConditionApplied`**, first in its arm, right after `ca :=
  p.ConditionApplied`: `if len(ca.ConditionId) == 0 || len(ca.ConditionId) >
  maxIDBytes { return fmt.Errorf("engine: condition id must be 1-%d bytes, got
  %d", maxIDBytes, len(ca.ConditionId)) }`. Then the known-actor check and the
  duplicate check.
- **`AbilityUsed`**: the same form, `engine: ability id must be 1-%d bytes,
  got %d`, before the existing `return nil // testimony, …`, whose line is
  kept as it is.

First, as SPEC-018's id checks come first in their arms: the length is a
property of the event alone. Against the known-actor check the order can be
observed, so a test holds it (D10) and the TS mirror matches it. Under Q4(a),
both use the "must be at most" form and drop the `== 0` arm.

**D5. The TS mirror.**
- `checkLen("condition id", v.conditionId, 1, 128);` first in
  `conditionApplied`, before the unknown-actor throw.
- `abilityUsed` leaves the shared fall-through for an arm of its own, after
  `adventureLoaded`: `case "abilityUsed": checkLen("ability id",
  p.value.abilityId, 1, 128); return;`. The two-line comment and `case
  "attackRolled": default: return;` stay below it.

Under Q4(a), both minimums are 0. The literals mirror the constant, as every
bound's do (Gap 7).

**D6. What the fold does not bound, and why (Q3, Q5).**

| Field | Held by | Notes |
|---|---|---|
| `ConditionRemoved.condition_id` | the presence check against `st.Conditions` | Stored only by a bounded `ConditionApplied`. A `remove_condition` echoes its own id in the refusal, which the frame bounds. |
| `ResourceChanged.resource` | the `actor.Resources` lookup | From `Resolve`, a declared name of at most 128 bytes (row A). An actor's keys are the second ticket's (D7). |
| `ConditionApplied.source`, the two `reason`s | nothing in the fold (Q3(a)) | Composed by `Resolve` from bounded parts: at most 265 bytes under Q2(b), 143 with an effect phase (P13). Never sent to a player or spectator (SPEC-016); the projection's frames carry none, and the Go tests fold an empty source 1,326 times (P4). |
| `AbilityUsed.actor_id`, `target_ids`, `rolls`, `outcome_summary` | nothing (testimony) | From `Resolve`, the actor and target ids are lookups into `st.Actors`, so at most 128 bytes. The rest is D2's. |

**D7. The meeting point with the second ticket.**
- **This ticket must** bound the declared names at the same number the second
  ticket will use for actor keys. It does, by using `maxIDBytes` (D1). The
  second ticket's plan should bound an actor's keys at `maxIDBytes` too, and
  the report says so.
- **This ticket leaves** the actor keys themselves: `add_actor`'s, which no
  check compares with the ruleset, and the fold's `ActorAdded` arm. It also
  leaves `ResourceChanged.resource` in the fold (Q5), and the adventure
  loader's refusal of an undeclared key, which quotes the key at any length
  (at boot only: `adventure.Load` has no caller but boot and its conformance
  package).
- **It changes for free** the adventure path: once this lands, an adventure's
  actor keys are at most 128 bytes, because the loader admits only declared
  names.

**D8. The threshold refusal names its position (Q6(b)).** In
`evalThresholds`, `for _, th := range def.Thresholds` becomes `for j, th :=
range …`, and the refusal becomes `fmt.Errorf("rules: resolve: threshold %d on
resource %q: %w", j, key.resource, err)`. What forced it:
- **The owner asked for the `use_ability` debt closed.** Names alone leave a
  70,105-byte refusal through a threshold's `when` (P12).
- **It is the only path.** Every other ruleset string a `use_ability` refusal
  carries is an ability id or a declared name: the stats, usage, range and
  outcome refusals, `EvalScoped`'s `unknown attribute %q`, and the fold's
  condition and resource refusals. Under row A, each is at most 128 bytes.
- **It is cheap.** Two lines are replaced in place. No key moves, and no line
  is added (P8). The refusal still says which threshold: the resource and its
  position in `ruleset.json`'s `thresholds` list, which is the field the
  author edits.
- **No test reaches the arm today** (P12), so the new test is its first.

A `use_ability` refusal also echoes the command's own ability id, actor id and
target ids. Those are the issuer's own text, bounded by the frame, and are not
the ruleset's.

**D9. The schemas state the bound (Q7).** Each of these gains the sentence "At
most 128 bytes of UTF-8.":
- `ability.schema.json`'s and `condition.schema.json`'s `id`, as a new
  `description`;
- `ruleset.schema.json`'s `attributes` and `defenses` descriptions, and
  `resources.items.properties.name`'s;
- under Q2(b), `atom.schema.json`'s `resolutionContribution.branches`.

None carries `maxLength`. JSON Schema counts characters, as D9 of the id plan
said for the tools. It would be exact only for the three ASCII-patterned
names, and stating one rule two ways invites the reader to think they differ.
`TestTheSchemasStateTheIDBound`, in the new
`internal/rules/id_bound_internal_test.go` (`package rules`), reads `Schemas`
and requires `fmt.Sprintf("At most %d bytes", maxIDBytes)` in each. Nothing
in `schema_test.go` changes: it checks properties, `required`, enums and
`oneOf`, and none of these moves.

**D10. The tests, each red today except the at-bound ones (P1).**
- **`internal/rules/load_test.go`, at the end, after
  `TestLoadAcceptsValidIdentifierNames`' closing brace:**
  - `rulesetAtTheBound(t *testing.T)`, a helper that begins with
    `t.Helper()` and returns the directory and the 128-byte strings it used. It copies `testdata/valid` into `t.TempDir()`
    with `copyDir` and, at exactly 128 bytes:
    - renames the condition `"guarded"` and the resource `pool_a` wherever
      they appear;
    - appends a declared attribute and a declared defense;
    - adds `abilities/at-the-bound.json`, whose id is 128 bytes, composing
      `self-delivery` and `apply-guarded` and spending the resource;
    - under Q2(b), renames `strike-roll.json`'s lt-label `"miss"`.
  - `TestARulesetWhoseIDOrNameExceedsTheBoundIsRefused`. One subtest per
    kind: ability id; condition id as `é`×64 + `i` (129 bytes, 65 characters);
    attribute; defense; resource name; and under Q2(b) branch label. Each
    builds a copy with one 129-byte string and asserts the refusal ends with
    `<file>: field "<field>": must be at most 128 bytes, got 129`.
  - `TestARulesetWhoseIDsAndNamesAreExactlyTheBoundLoads`. It loads
    `rulesetAtTheBound` and asserts each 128-byte string is held whole: in
    `Compiled`, `Conditions`, `Attributes`, `Defenses`, `Resources`, and
    under Q2(b) the compiled resolution's `Branches`. Gremlins kills each of
    the six new boundary mutants only with an at-bound value for each check
    (the id arc's P8).
  - `TestARulesetNameOfSeventyThousandBytesIsRefusedAtLoad`, the debt recipe.
    It copies `conformance/testdata/minimal-smoke-fail`, replaces `"focus"` in
    `ruleset.json` and `abilities/big-move.json` with a 70,000-byte name, and
    asserts the refusal ends with `ruleset.json: field "resources[0].name":
    must be at most 128 bytes, got 70000` and is shorter than its directory's
    path plus 128 bytes.
- **`internal/rules/resolve_test.go`, at the end:**
  - `TestARulesetAtTheBoundResolvesToEventsTheFoldAccepts`. It loads
    `rulesetAtTheBound`, resolves the 128-byte ability on its own caster, who
    holds the 128-byte resource and stands on a token, and folds the batch
    through `engine.Apply`. It asserts the 128-byte condition is stored
    whole. This observes the ticket's ordering claim, that the fold refuses
    nothing a loaded ruleset produces, and reds when the fold's bound falls
    below the loader's (K24, P13).
  - Under Q6(b), `TestAThresholdRefusalNamesItsPositionNotItsExpression`. A
    copy of `testdata/valid` whose `pool_a` threshold `when` is `#pool_a +
    @brawn` followed by 17,500 × ` + 0`. `guard-stance`, used by an actor that
    holds `pool_a` and not `brawn`, is refused with exactly `rules: resolve:
    threshold 0 on resource "pool_a": rules: expr: unknown attribute "brawn"`.
- **`internal/rules/id_bound_internal_test.go`, new, `package rules`:**
  `TestTheIDBoundMirrorsEngine` (`maxIDBytes == 128`, a one-line message), and
  under Q7 `TestTheSchemasStateTheIDBound` (D9).
- **`internal/engine/apply_test.go`, at the end:**
  - `TestAConditionWhoseIDExceedsTheBoundIsRefused` and
    `TestAnAbilityWhoseIDExceedsTheBoundIsRefused`. Each sends 129 bytes,
    asserts the exact error, and asserts an unchanged `Snapshot`. The state
    holds the actor.
  - Under Q4(b), `TestAnEmptyConditionOrAbilityIDIsRefused`, two subtests
    asserting `… must be 1-128 bytes, got 0`.
  - `TestAConditionIDIsMeasuredBeforeItsActorIsLookedUp`: a 129-byte id on an
    unknown actor is refused with the id's text.
- **`internal/engine/apply_boundary_test.go`, at the end:**
  `TestConditionAndAbilityIDsAtCapAreAccepted`. A 128-byte condition and a
  128-byte ability id fold, and the condition is stored whole.
- **`client/test/fold-rejections.test.ts`, at the end, in a "ruleset ids"
  section:**
  - "a condition id over 128 UTF-8 bytes is rejected" (`é`×64 + `i`);
  - "an ability id longer than 128 bytes is rejected";
  - under Q4(b), "an empty condition id is rejected" and "an empty ability id
    is rejected";
  - "a condition id is measured before its actor is looked up";
  - "condition and ability ids of exactly 128 bytes are ACCEPTED", which
    asserts the stored condition.

  The existing "an event kind the fold does not know is skipped" folds an
  `abilityUsed` with id `"x"`, which still folds.
- **No gateway test.** The fold refuses nothing a loaded ruleset produces, so
  no command can carry an over-long one (P13). The refusal text `Resolve`
  builds reaches the issuer verbatim over a real connection, which VTT-132's
  `TestUseAbilityResolveValidationErrorIsCleanOkFalse` already observes.
  That is the "off a real connection" half of the debt entry's closing
  condition (D11, Gap 2).

The goldens need no case: no stream holds such an id within 113 bytes of the
bound (P3).

**D11. The records.**

**SPEC-018**, written with the `specification` skill against the final tree:
- **Status:**
  - "`Apply`'s `SessionStarted`, … and `AdventureLoaded` arms" gains
    `AbilityUsed` and `ConditionApplied`;
  - `fold.ts`'s "the same eight arms" becomes ten;
  - add "for a ruleset's ability and condition ids, attribute, defense and
    resource names (and branch labels) by `internal/rules/load.go`'s
    `maxIDBytes`";
  - add the new test files, and the QA files when they exist.
- **"Fourteen fields are bounded"** becomes sixteen. The table gains
  `ConditionApplied.condition_id` and `AbilityUsed.ability_id`, each 128
  (`maxIDBytes`), "May be empty" no. Under Q4(a), yes.
- **The order sentences** gain: a `ConditionApplied` is checked for its
  condition id, then a known actor, then a duplicate; an `AbilityUsed` for its
  ability id alone.
- **"These fourteen are the only texts"** becomes sixteen.
- **The clause "one a map or adventure file carries is bounded only by what
  its loader checks"** gains "or a ruleset". Under Q3(a), add: a
  `ConditionApplied`'s `source`, which `Resolve` composes from an ability id
  and a branch label or a resource name, is bounded by no fold.
- **"for each of the fourteen fields"** becomes sixteen.
- **"Other mirrors"** gains `internal/rules/load.go`'s `maxIDBytes`, pinned by
  `TestTheIDBoundMirrorsEngine` and held by the refusal and at-bound tests.
  "A map or an adventure over one of them" becomes "a map, an adventure or a
  ruleset". Under Q7, add the schemas' statement and
  `TestTheSchemasStateTheIDBound`.
- **Consequences:** the second bullet gains `internal/rules`' copy (and under
  Q7 the schemas). The third bullet's "a map or adventure file" gains "or a
  ruleset".
- **Requirements:** add the new rows.
- **The title and path** stay.

**The register:** the new rows, after sign-off. No existing row changes.

**`docs/verification-debt.md`:**
- **"A ruleset's names reach a `use_ability` refusal at any length"** gains,
  under Q6(b): `**Closed by** TestARulesetNameOfSeventyThousandBytesIsRefusedAtLoad
  in internal/rules/load_test.go, which reds when the resource-name bound is
  removed (K5), and TestAThresholdRefusalNamesItsPositionNotItsExpression in
  internal/rules/resolve_test.go, which reds when the refusal quotes the
  threshold's expression (K36); the refusal reaches the issuer verbatim over a
  real connection by TestUseAbilityResolveValidationErrorIsCleanOkFalse
  (VTT-132)`, with the observation date.
  - Under Q6(a), the line names the load test alone, and a new open entry
    records the threshold path. Its recipe is P12's, its labels `outside the
    tool` and `test data missing`.
- **"Nothing ties a byte bound's copies to the engine's constant"**, which
  stays open, is amended:
  - "fourteen bounds" becomes sixteen;
  - add "`internal/rules/load.go` its own `maxIDBytes`" and
    `TestTheIDBoundMirrorsEngine` (internal/rules) to the list;
  - under Q7, add that the schemas state it, held to `internal/rules`' copy
    rather than the engine's.

  Its claim stays true: nothing compares a copy with the engine's constant.

**D12. The sort.** Forced by SPEC-008. The rows are lettered so nothing reads
as an id.

| # | Rule | Proposed | Red when | Tests | Record |
|---|---|---|---|---|---|
| A | A ruleset that declares an ability id, a condition id, or an attribute, defense or resource name longer than 128 bytes of UTF-8 is refused when it loads, by an error naming the file and the field; one whose every such string is exactly 128 bytes loads. | accept: the ticket's first rule and Done items 1 and 4 | K1 to K5, K7 to K11, K13 to K15 | D10's load tests, `TestTheIDBoundMirrorsEngine` | SPEC-018 |
| B | A ruleset whose resolution carries a branch label longer than 128 bytes of UTF-8 is refused when it loads, naming the file and the field. | accept under Q2(b) | K6, K12, K13 | the branch subtest and the at-bound load | SPEC-018 |
| C | Both folds refuse a `ConditionApplied` whose condition id, or an `AbilityUsed` whose ability id, is longer than 128 bytes of UTF-8, and accept one of exactly 128. | accept: the ticket's second rule and Done items 2 and 3 | K16 to K19, K22 to K29, K32 to K34 | D10's engine and TS tests, `TestARulesetAtTheBoundResolvesToEventsTheFoldAccepts` | SPEC-018 |
| D | Both folds refuse a `ConditionApplied` or an `AbilityUsed` whose id is empty. | accept under Q4(b) | K20, K21, K30, K31 | the empty cases | SPEC-018 |
| E | The ruleset schemas state the bound `rules.Load` enforces on the ids and names it bounds. | accept under Q7 | K13, K35 | `TestTheSchemasStateTheIDBound` | SPEC-018 |
| F | A `use_ability` refused while a resource's threshold is evaluated names the resource and the threshold's position, not the threshold's expression. | accept under Q6(b) | K36 | `TestAThresholdRefusalNamesItsPositionNotItsExpression` | none; rows with no record exist (VTT-253, VTT-257, VTT-263). SPEC-012's command paragraph is its home if one is wanted (Q6) |

**Refused, one line each:**
1. "A ruleset name is at most 128 bytes" on its own: A holds it.
2. "`ResourceChanged.resource` is bounded in the fold": it follows from the
   actor-key lookup, and the keys are the second ticket's (Q5).
3. "`ConditionRemoved`'s condition id is bounded": it follows from C through
   the presence check.
4. "The loader's copy equals the engine's constant": a consistency of copies,
   SPEC-018's consequence and the open debt entry.
5. "The condition id is checked before its actor": SPEC-018's order text,
   held by a test cited under C.
6. "A refusal names the bound and the length": how A and C are observed, by
   exact text.
7. "Every event `Resolve` emits from an at-bound ruleset folds": it follows
   from A and C, and its test is cited under C.
8. "An over-long name is refused before any check that quotes it": the
   loader's order, at boot only, unobserved (Gap 6).
9. "Every shipped ruleset loads": existing behaviour, held by both
   conformance suites.
10. "A `ConditionApplied`'s source is bounded": no rule under Q3(a).

Six rows, A to F, are proposed under the recommendations. Two, A and C, are
proposed if every option is declined.

**D13. Mutation keys and adjudications: last, by statement.** These apply
after the review settles and before the commit. P8 gives the positions; re-read
each at its statement.
- **`apply.go`**, three of four keys move, under any answer to Q2, Q4, Q6 or
  Q7:
  - `id < standing` (`ActorRemoved`), `351:58`: unchanged;
  - `computed < 0`: from `465:15` to `468:15`;
  - `computed > int64(res.Max)`: from `468:30` to `471:30`;
  - `len(objs) > 0`: from `612:41` to `618:41`.

  The shift is the `AbilityUsed` check (+3), then the `ConditionApplied`
  check (+3). Under Q3(b), the source check adds 3 more to `618:41`.
- **`fold.ts`**:
  - `386:48`, `415:19`, `424:11`, `425:26` (+1, the `conditionApplied` line);
  - `465:10` (+4);
  - `466:5`, `495:7`, `585:34`, `607:32`, `712:7`, `712:21` (+3);
  - `326:11` to `328:37` are unchanged.
- **The `"abilityUsed"` entry, `462:10`, is deleted, not re-keyed.** Its
  mutant at the new `case "abilityUsed":` is killed by the ability refusal
  (K34), and the gate fails an adjudicated mutant that is killed.
- **The `default:` entry, now `466:5`, has its reason corrected.** "shared
  with the two no-effect event kinds above it" becomes the one kind,
  `attackRolled`. The `attackRolled` entry, now `465:10`, stays true.
- **`internal/rules`:** no key moves (P8).

Then run `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q`. The TS self-test cannot see `386:48`,
`415:19` or `607:32` move (P9), so its `OK` is necessary, not sufficient, and
Phase 4b re-reads every key. If a review adds a line above an edit point,
re-measure every key.

The new mutants die as follows:
- Go `>=`: by the at-bound tests.
- Go negations: by the refusals and by every valid fold or load.
- The TS labels: by the exact messages.
- `case "abilityUsed"` emptied: by the ability refusal.

**D14. Comments and the ledger.** Tests carry their `// VTT-NNN` line only.
Two comment lines are added in production code:
- `// maxIDBytes mirrors internal/engine's bound on an id (SPEC-018).` above
  `internal/rules/load.go`'s constant.
- `checkManifestBounds`, unexported, carries no comment, or the one pointer
  line `// checkManifestBounds holds SPEC-018's bound on a ruleset's declared
  names.`. Rule 10 allows a pointer and nothing else on an unexported
  function. P10 measured two lines.

Two other comments are corrected:
- `client/test/fold-unit.test.ts`'s sentence that `attackRolled` shares its
  arm body with `abilityUsed` and `adventureLoaded` is made true: it shares
  it with `default:` alone. It sits under `client/test`, outside
  `check:comments`.
- The `default:` adjudication's reason (D13).

No ledger row changes (P10). `--write-ledger` is not run unless the final
tree's `check-comments.py main` asks for it.

**D15. One code commit and the report (Q8).** The folds land together (rule 4).
The loader could land first, as the ticket says, but nothing is gained by a
separate commit: each one costs a review record. **C1** holds:
- the code, the tests and the four schema files;
- the rebuilt `cmd/vtt/webdist/assets/index.js` (`task build:client`);
- SPEC-018, the rows and both debt edits;
- the keys and the adjudication edits.

**C2** holds the report.

**D16. Phase 4a for C1.** One QA agent per `qa-prompt.md`. It is never given
the diff, the source, the existing tests or the implementer's report. It is
given:
- the accepted rows, VTT-132, VTT-161 and VTT-162;
- SPEC-018 whole;
- SPEC-016's sentences on `source` and `reason` and on the introductions;
- both debt entries' text;
- the text of `AbilityUsed`, `ConditionApplied`, `ConditionRemoved`,
  `ResourceChanged`, `UseAbility` and `RemoveCondition`;
- the four schema files;
- `go doc -all` of `./internal/rules`, `./internal/engine`,
  `./internal/gateway` and `./internal/campaign`;
- `grep -n '^export' client/src/fold.ts client/src/state.ts`;
- `internal/rules/testdata/valid/` and `rulesets/tavern-brawl/` as committed
  format examples. They are data, and naming them avoids the id arc's
  unplanned fixture read.

It writes, each file importing only what `.go-arch-lint.yml` allows its
package:
- `internal/rules/qa_ruleset_bound_test.go` (`rules_test`: `rules`, `engine`,
  `contract`);
- `internal/engine/qa_ruleset_bound_test.go` (`engine_test`: `engine`,
  `contract`);
- `client/test/qa-ruleset-bound.test.ts` (`client/src`, `contract/gen/ts`);
- and only if it drives a server, `internal/gateway/qa_ruleset_bound_test.go`
  (`gateway`, `rules`, `engine`, `campaign`, `identity`, `contract`; never
  `store`). A test of a log that no longer opens goes in
  `internal/campaign/qa_ruleset_bound_test.go` (`campaign`, `store`,
  `engine`, `contract`, `eventgen`).

Before it reports, it runs `go vet` and `golangci-lint run` on each package it
touched (every helper begins with `t.Helper()`), `go-arch-lint check`, `bunx
tsc --noEmit -p client/tsconfig.json`, and its own tests. It builds every
ruleset in `t.TempDir()` and commits no fixture. It runs no command that
writes outside its files: no `python3 -m json.tool a b`, no glob as an output
argument. The tree is hashed before and after.

Adjudications go in the report, one line per finding. An escape goes to the
debt file as a recipe. A QA test that pins an ABSENCE no rule states is
dropped: a display name, an expression, a source or an actor key left
unbounded, or a referring id checked by length.

**D17. Phase 4b for C1, after 4a.** One reviewer at high effort, given `git
diff HEAD`, the untracked files, this plan and the draft message, and briefed
to:
- verify by command every sentence C1 adds or rewrites: SPEC-018, both debt
  entries, the schema sentences, the `default:` reason and the
  `fold-unit.test.ts` sentence;
- re-read every re-pointed key at its statement, the three the TS self-test
  cannot see included, and confirm `462:10`'s mutant is killed (K34);
- check the break lines in the draft message;
- write nothing: no formatter, no `json.tool` with two arguments. The tree is
  hashed before and after.

If the reviewer dies on a model limit, say so and re-dispatch the same brief
on `fable`.

**D18. The breaks: exactly the K table, and none named elsewhere.**
- **Run** K1 to K36 in a scratch clone of C1's final tree, against the real
  tests. The ones that exist only under a question:
  - K6 and K12 under Q2(b);
  - K20, K21, K30 and K31, and the empty halves of K16, K17, K26, K27 and
    K34, under Q4(b);
  - K35 under Q7;
  - K36 under Q6(b).
- **The script** is written from this table alone and asserts each K's
  expected reds by name.
- **Each break** is one edit, made after every gate is clean. Each file is
  restored from its saved text and checked by hash.
- **K36's edit** restores both the range variable and the refusal, or the
  package does not build (P14).

| # | Edit | Red |
|---|---|---|
| K1 | `loadCompositions`' ability-id bound removed | the ability subtest |
| K2 | `loadConditions`' condition-id bound removed | the condition subtest |
| K3 | `checkManifestBounds`' attribute bound removed | the attribute subtest |
| K4 | its defense bound removed | the defense subtest |
| K5 | its resource-name bound removed | the resource subtest, the recipe test |
| K6 | the branch-label bound removed | the branch subtest |
| K7 | the ability-id bound `>=` | the at-bound load, the at-bound `Resolve` |
| K8 | the condition-id bound `>=` | the at-bound load, the at-bound `Resolve` |
| K9 | the attribute bound `>=` | the at-bound load |
| K10 | the defense bound `>=` | the at-bound load |
| K11 | the resource-name bound `>=` | the at-bound load, the at-bound `Resolve` |
| K12 | the branch-label bound `>=` | the at-bound load |
| K13 | `internal/rules`' `maxIDBytes` 129 | the mirror pin, every 129-byte load subtest, the schema test |
| K14 | the condition-id bound measured in runes (`len([]rune(…))`) | the condition subtest (65 runes, 129 bytes) |
| K15 | `loadManifest`'s call to `checkManifestBounds` disabled | the attribute, defense and resource subtests, the recipe test |
| K16 | Go condition-id check disabled | engine condition refusal, its empty case, the order test |
| K17 | Go ability-id check disabled | engine ability refusal, its empty case |
| K18 | Go condition-id `>` → `>=` | engine at-cap |
| K19 | Go ability-id `>` → `>=` | engine at-cap |
| K20 | Go `len(ca.ConditionId) == 0 \|\|` removed | engine empty condition case |
| K21 | Go `len(…AbilityId) == 0 \|\|` removed | engine empty ability case |
| K22 | Go condition-id check moved after the known-actor check | the engine order test |
| K23 | `engine`'s `maxIDBytes` 129 | engine condition and ability refusals, the order test, and at least VTT-277's refusals and `TestTheToolsStateTheFoldsIDBound` |
| K24 | `engine`'s `maxIDBytes` 127 | engine at-cap (this change's and `TestIDsAtCapAreAccepted`), the at-bound `Resolve` |
| K25 | Go condition-id refusal reworded | engine condition refusal, the order test |
| K26 | TS condition `checkLen` removed | TS condition refusal, its empty case, the order case |
| K27 | TS ability `checkLen` removed | TS ability refusal, its empty case |
| K28 | TS condition maximum 127 | TS at-cap, the condition refusal and the order case (their text) |
| K29 | TS ability maximum 127 | TS at-cap, the ability refusal |
| K30 | TS condition minimum 0 | TS empty condition case |
| K31 | TS ability minimum 0 | TS empty ability case |
| K32 | TS condition `checkLen` moved after the unknown-actor throw | the TS order case |
| K33 | TS condition id measured with `.length` (characters) | TS condition refusal (`é`×64 + `i`, 65 characters) |
| K34 | TS `case "abilityUsed":` label emptied | TS ability refusal, its empty case |
| K35 | one schema's sentence says 200 | `TestTheSchemasStateTheIDBound` (not run: the sentences do not exist yet) |
| K36 | the threshold refusal quotes `th.WhenSrc` again, with `_` restored | `TestAThresholdRefusalNamesItsPositionNotItsExpression` |

**D19. Disk and load.** Forced by `check:mutation`'s 16 GiB floor, by MCP e2e
deadlines under load, and by this machine's 8 GiB of memory. Immediately
before `task check`:
- run `go clean -cache`, `df -k` and `python3 -c "import os;
  print(os.getloadavg())"`;
- launch the gate once after C1, in its own session
  (`start_new_session=True`);
- below 16 GiB free, stop; above a load of about 8, wait.

## Tasks, in dependency order

### Task 0 — Baselines

**Done when:** `df -k`, the load, the chain, comments and doc-owner baselines
and both self-tests match P15, and `requirement-id` resolves.

### Task 1 — Rows

**Files:** `docs/requirements.md`. After sign-off, run `requirement-id` once
per accepted row, in letter order. **Done when:** that many new OPEN rows
exist.

### Task 2 — The loader

**Files:** `internal/rules/load.go`, `load_test.go`, the new
`id_bound_internal_test.go`.

D10's load tests and `TestTheIDBoundMirrorsEngine` come first (the refusals
and the recipe are red), then D3. **Done when:**
- `go test -count=1 ./internal/rules/...` is green;
- `gocyclo -over 29 internal/rules/load.go` prints only the two nolint'd
  functions, `loadManifest` at 33;
- `golangci-lint run ./internal/rules/...` reports 0 issues;
- `go test -count=1 ./internal/adventure/... ./cmd/vtt/` is green (every
  shipped ruleset and adventure loads).

### Task 3 — The Go fold

**Files:** `internal/engine/apply.go`, `apply_test.go`,
`apply_boundary_test.go`, `internal/rules/resolve_test.go` (the at-bound
`Resolve`).

D10's engine tests first (red), then D4. **Done when:**
- `go test -count=1 ./internal/engine/ ./internal/rules/...` is green;
- `go test -count=1 -p 2 ./internal/...` shows nothing else red (P6).

### Task 4 — The TS mirror

**Files:** `client/src/fold.ts`, `client/test/fold-rejections.test.ts`,
`client/test/fold-unit.test.ts` (D14's sentence), `cmd/vtt/webdist/`.

The cases first, then D5, then `task build:client`. **Done when:**
- `bun test client/test` is green;
- `client:typecheck` is clean;
- `git status` shows `index.js` as the only webdist change.

### Task 5 — The threshold refusal (under Q6(b))

**Files:** `internal/rules/resolve.go`, `resolve_test.go`.

The test first (red, with the 70 KB expression in its text), then D8.
**Done when:**
- `go test -count=1 ./internal/rules/... ./internal/gateway/` is green;
- `wc -l internal/rules/resolve.go` prints 702.

### Task 6 — The schemas (under Q7)

**Files:** the four `internal/rules/schema/*.json`,
`internal/rules/id_bound_internal_test.go`.

`TestTheSchemasStateTheIDBound` first (red), then D9. **Done when:** `go test
-count=1 ./internal/rules/` is green, `schema_test.go` included.

### Task 7 — The records

**Files:** SPEC-018, `docs/requirements.md` (the new rows' evidence),
`docs/verification-debt.md`.

D11. **Done when:**
- the chain prints 282 rows plus the new ones and `13 specifications`, with
  none of the new rows OPEN;
- `grep -c 'Fourteen fields\|These fourteen\|of the fourteen' docs/specifications/018-*`
  prints 0;
- `grep -c 'fourteen bounds' docs/verification-debt.md` prints 0;
- the `use_ability` entry carries its `**Closed by**` line.

### Task 8 — Local gates

Run:
- `gofmt -l` over the touched Go files only;
- `go vet` and `task lint`;
- `go test -count=1 -p 2 ./internal/... ./contract/... ./tools/... ./cmd/...`;
- `bun test client/test contract contract-spike`;
- semgrep, `go-arch-lint check`, `check:comments`, `check:doc-owner` and
  `check:new-prose`.

**Done when:** each prints its own completion line. A Go failure is captured
whole and re-run in isolation before it is called a flake.

### Task 9 — Phase 4a, then 4b

D16 and D17. Findings are fixed, and the affected task's "done" is re-run. The
review settles before Task 10.

### Task 10 — Keys, adjudications, commit C1

D13, then C1's message, which lists the ids and D18's lines. **Done when:**
- both self-tests print `OK`, and every key reads its statement;
- `grep -c '^client/src/fold.ts 462:10' tools/ts-mutation-equivalents.txt`
  prints 0;
- `git show --stat HEAD` lists C1's files;
- `task check:drift` is clean.

### Task 11 — Breaks and the whole gate

D18, then D19, then `task check` once. **Done when:** each break gives its
reds, and `task check` exits 0 with every step's own verdict,
`check:mutation` and `check:ts-mutation` included.

### Task 12 — The report

`docs/reports/2026-10-06-a-rulesets-ids-and-names-are-bounded.md`, per the
`implementation-report` skill. It covers:
- each Done item with its observation;
- the rows and refusals;
- the rulings taken at sign-off;
- the rule-9 answer;
- the breaks;
- D2's texts left out, raised to the owner as a ticket;
- D7's word to the second ticket: bound an actor's keys at `maxIDBytes`;
- the gaps.

**Done when:** C2 holds it alone. Push after C2, and let the pre-push hook
finish.

## Gaps that travel with this plan

1. **The problem paragraph says `rules.Load` validates against the JSON
   schemas; it does not.** `grep -n "Load itself does not run" internal/rules/schema.go`.
   The schemas are documents `schema_test.go` cross-checks, so a `maxLength`
   there would bound nothing. D3 puts the bound in `load.go`; D9 has the
   schemas state it.
2. **Done item 4 departs from the debt entry's own closing condition without
   saying so.** The entry asks for "a test that reads the refusal off a real
   connection". With the bound at load, the recipe cannot be built, so there
   is no long refusal to read. D10 points at VTT-132's existing wire test for
   the verbatim half. And names alone do not close the symptom: a threshold
   expression still reaches the refusal whole (P12, Q6).
3. **"What it touches" omits work that Done item 5 needs:**
   - the deletion of the `"abilityUsed"` TS adjudication and the `default:`
     entry's reason;
   - `client/test/fold-unit.test.ts`'s false sentence;
   - the amendment of "Nothing ties a byte bound's copies";
   - the webdist rebuild;
   - `checkManifestBounds`, for gocyclo.

   It lists `tools/comment-ceilings.txt` and "keys in `internal/rules`",
   neither of which moves (P8, P10).
4. **The at-bound halves of Done items 1 to 3 cannot fail today**, as the
   ticket says. K7 to K12, K18, K19, K28 and K29 hold them.
5. **A campaign whose log already holds an over-long or empty condition id or
   ability id stops opening.** No shipped content, fixture, golden, eventgen
   draw or test fold comes within 113 bytes of the bound or holds an empty
   one (P3 to P5). Nobody uses the product (the owner, 2026-09-04).
6. **The loader's order against the checks that quote a name is unobserved.**
   The length check comes first so no boot refusal prints more than 128 bytes
   of a name, and no test combines an over-long name with a second fault.
7. **Nothing links `fold.ts`'s `128`, `internal/rules`' `maxIDBytes` or the
   schemas' sentence to the engine's constant** beyond each side's tests. The
   open debt entry is amended rather than closed.
8. **The TS self-test cannot see three of the moved `fold.ts` keys** (P9).
   Phase 4b's re-read is the check.
9. **What stays unbounded:** `ConditionApplied.source` and the reasons in the
   fold under Q3(a); `AbilityUsed`'s actor and target ids, rolls and summary
   as testimony; display names and expressions in the log; the manifest's
   id and name; an actor's keys, until the second ticket. D2 and D6 say why
   for each.
10. **The mutation gates were not run** (8 GiB of memory, swap nearly full
    today). D13's kills are predicted from the sketches' reds (P14) and the id
    arc's P8, and Task 11's `task check` is the measurement.

## Questions for sign-off

1. **The number and its constant.**
   (a) 128 bytes, the fold's own `maxIDBytes`, with `internal/rules` holding a
   pinned copy;
   (b) 128 in a constant of its own, in the fold and the loader;
   (c) as (a), but `internal/rules` reads an exported `engine.MaxIDBytes`,
   which `.go-arch-lint.yml` allows, instead of a copy.
   Recommend (a) (D1). A condition and an ability id are ids, a declared name
   becomes an actor key the second ticket will bound, and one constant keeps
   them equal by construction. (c) removes a copy for good, but changes the
   engine's exported surface, which `internal/adventure` chose not to do.
2. **Which other ruleset texts are in scope?**
   (a) only the five kinds you named;
   (b) those and a resolution's branch labels;
   (c) (b) plus display names and expressions.
   Recommend (b) (D2). A branch label is the one other ruleset string a
   `use_ability` puts into the fold's state, inside `ConditionApplied.source`.
   Display names and expressions reach testimony and the read route, never
   state. Q6(b) closes the one refusal an expression reaches. Apart from a
   condition's description, the longest of any is 37 bytes (P2).
3. **Does `ConditionApplied.source` need a fold bound of its own?**
   (a) No: it is bounded by its parts on every server path, at most 265
   bytes under Q2(b), and SPEC-018 says so;
   (b) yes, a constant of its own in both folds, admitting empty (the
   projection's frames carry none, and the Go tests fold an empty source
   1,326 times) and at least 265 bytes.
   Recommend (a) (D6). A bound would encode `Resolve`'s
   `ability:<id>:<label>` format in the engine, and no author chooses a source
   directly. The reasons are not stored.
4. **Empty ids:**
   (a) keep accepting an empty condition id and ability id in the folds;
   (b) refuse both, as VTT-278 refuses an empty scene, token or adventure id.
   Recommend (b). No test, golden or eventgen draw folds one, in Go or TS (P4,
   P5). The loader already refuses both. On the wire an empty id cannot be
   told from an absent one.
5. **`ResourceChanged.resource`:** no bound of its own here?
   Recommend yes, none (D6, D7). It must name a key the actor holds, as a
   referring id names a stored one. From `Resolve` it is a declared name,
   bounded by row A. The keys themselves are the second ticket's, which this
   plan asks to use `maxIDBytes`.
6. **Closing the `use_ability` debt.**
   (a) Close it for names with the load test, and record the threshold
   expression as a new open entry with P12's recipe;
   (b) also make the threshold refusal name its resource and position instead
   of its expression, with a test, and close it whole;
   (c) bound expressions at load as well.
   Recommend (b) (D8). Names alone leave a 70,105-byte refusal. (b) is two
   lines replaced in place, moves no key, and leaves every ruleset string in a
   `use_ability` refusal a name of at most 128 bytes. (c) is a new bound on a
   new kind of text, for a path (b) already closes. Under (b), SPEC-012's
   command paragraph could gain a sentence on what a refusal carries; the
   ticket names SPEC-018 alone, so that would move a record it does not list,
   and this plan does not add it unless you ask.
7. **Do the ruleset schemas state the bound**, in prose and without
   `maxLength`, held by an internal test? Recommend yes (D9). They are the
   author's contract, and an author who validates against them should learn
   the bound before `Load` refuses.
8. **Rows A to F** (A and C alone if every option above is declined), **and
   one code commit plus the report?** Recommend yes (D12, D15). The folds
   cannot land apart, and a separate loader commit buys nothing but a second
   review record.
