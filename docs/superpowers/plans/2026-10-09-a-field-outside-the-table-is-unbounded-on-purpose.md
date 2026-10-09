# Plan: a field outside SPEC-018's table is unbounded on purpose

Ticket: `docs/superpowers/specs/2026-10-09-a-field-outside-the-table-is-unbounded-on-purpose-design.md`.
Verified 2026-10-09 by an agent that did not write the ticket, on the tree at
`121d61e`. Verdict: **Passes with gaps**. The gaps are listed at the end, and
the questions for Patrik come before the tasks because two of them change the
draft text.

## How the ticket was verified

| Check | How settled | Result |
|---|---|---|
| 1. Every path resolves | command | Every file and symbol the ticket names exists where it says. The quoted phrase "nowhere yet; raised to the owner" is in both reports, wrapped across a line break. |
| 2. Done is an observation | command | Done item 1's command is written below. On today's tree it prints 44 names and exits 1. Run against the draft below, it prints nothing and exits 0. With three names removed from the draft, it prints exactly those three. |
| 3. Each rule is breakable | reading | "None" can be defended: the criterion governs later decisions. The ticket's closing claim, though, is a property of the system that a later change can break with nothing going red (Gap G3). |
| 4. Scope matches the claim | command, then reading | One file was right as the ticket then stood; Patrik's answer to Q2 added `docs/map-format.md`. Several claims in "The problem" are false or overstated (G1). No other doc contradicts the new text. |
| 5. No decision contradicted silently | reading | Nothing in `docs/specifications/` or `docs/adr/` is overturned. SPEC-007 already gives `Actor.module_data`'s shape to rule modules, and `maxNarrationAsBytes`'s own comment gives the reason behind the criterion's first test. |
| 6. Records moved are named and reachable | command, then reading | SPEC-018 is the only record this moves. SPEC-014 points readers to SPEC-018 for its byte bounds. SPEC-012, -013 and -016 own things the new text points at, but none of them states a length. |

## The constraints

These bind every task. They are named here, not restated:

- CLAUDE.md rule 2: `task check` is the gate, it is run whole, and it is never weakened. Done item 5 is that gate.
- CLAUDE.md rule 8: a citation names a durable thing. No bare `file.go:123`, no path into `.superpowers/`, and no "this task".
- CLAUDE.md rule 10: this plan changes no comment under `internal/`, `cmd/` or `client/src`. Any edit there is out of scope.
- CLAUDE.md rule 3: no proto changes.
- The specification form (the dev-cycle `specification` skill and its catches list):
  - present tense;
  - the five headings `Status`, `Principles served`, `How it works`, `Consequences`, `Requirements` stay unchanged and in that order;
  - no section headed `Why`, and no line opening `Rejected` or `Alternatives considered`;
  - no past-tense account, no added count or measurement, no line number;
  - every sentence that says only, never, every or none names its search;
  - no old text left beside the new.
- Requirement ids come only from the dispenser (SPEC-008). The ticket adds no rule, so none is expected and the `Requirements` line does not change.
- Phase 4a is skipped, because the change is prose with no behaviour to derive. Phase 4b's reading review is required.

## Decisions

**D1. Placement.** The new text goes into `How it works`, directly before the paragraph that opens `**The refusal reaches the issuer.**`. It has three parts: a bold-lead paragraph, a second table, and a paragraph on file strings. The sentence that today ends the first paragraph is deleted, from "A `ConditionApplied`'s `source`" through "is bounded by no fold.", because the new table states it. Keeping both would leave two sources (catch 13). *Forced by:* a reader who has just found an unbounded field arrives at the first table, so the answer belongs right after it.

**D2. Status, title and Consequences.** Status and title stay as they are.
- The new decision is an absence, so no code implements it.
- Each holder the text cites (`Resolve`, `ToEvent`, `BuildSceneCreated`, `project.go`, `stampSessionIDAgainst`, `describeBlockage`) implements another record's decision. Listing them under SPEC-018's "Implemented by" would claim their code as SPEC-018's.

In `Consequences`, the third bullet is amended and two bullets are added (draft below).

**D3. Every field is spelled `Message.field`, each one separately.**
- A nested field is written `AbilityUsed.Roll.expression`.
- A map's keys are written as "`SceneCreated.tiles` keys".
- No collective forms such as "a control event's `actor_id`".

*Forced by:* Done item 1's command. It matches whole names at identifier boundaries, so `Actor.controller_ids` does not count as naming `Actor.controller_id`. That was checked by removing the shorter name from the draft.

**D4. The criterion for joining the table: two tests.**
1. A role other than the DM and the agent can put text it chose into the field in the log, other than an id that `engine.Apply` or `Resolve` requires to name something already folded. This is the reason `maxNarrationAsBytes`'s comment gives for bounding `NarrationAdded.as`.
2. A test or a recipe shows that a value of the field, written by a real command or file, breaks a frame, a view or a refusal that a participant other than its writer receives.

Test 2 is narrower than the ticket's suggestion ("its length is shown to break something a participant sees"). *Forced by:*
- The MCP agent connects through `harness.Dial` (`internal/mcp`'s `harnessDial`), and that client's `readLimit` is 200 KiB.
- So any file-carried text with no loader bound meets the unqualified test in principle, for example a map object's `kind` that carries a `SceneCreated` past that limit.
- If the spec stated the unqualified test and also said no field meets it, the text would contradict itself.

The draft therefore says "no field meets the first" with the search behind it, and makes no present-day claim about the second. The wording of test 2 is Patrik's to settle (Q5).

**D5. Wording: "has no bound of its own".** The text does not call these fields "unbounded". *Forced by:* a field held by a lookup equals an id or name the table bounds, so calling it "unbounded" would be false.

**D6. File strings are named by class.** The list comes from the two reports, with two exceptions:
- Tile names are left out, because `CheckTileNamesKnown`/`StandardTile` refuses a name outside the vocabulary.
- An adventure placement's `actor_id` is left out, because `internal/adventure`'s loader refuses one that names no declared actor.

**D7. Refusals get one sentence, under the same decision.** It corrects the ticket's "each reaches only the issuer": a ruleset's or an adventure's loader refusal happens at boot (`cmd/vtt`'s `loadAdventuresDir`, `serve_compose.go`'s `rules.Load` call) and goes to whoever starts the server.

**D8. `Actor.module_data` is named, and the command also lists `google.protobuf.Struct` fields.** This is a superset of the ticket's "string field". *Forced by:* `module_data` is the one text outside both tables that a command writes and the fold keeps (`proto.Clone(a)` in the `ActorAdded` arm). A command that lists strings only cannot see it.

**D9. No implementation report.** The dev-cycle asks for one only after a ticket's last commit that changes code. The ticket file, committed, carries the ruling and its date. Patrik confirms (Q4).

**D10. No CLAUDE.md line.** CLAUDE.md's own text says a rule kept in two places drifts. The readers this binds are working on a bound, and they read SPEC-018 because their work moves it. Patrik confirms (Q1).

**D11. `docs/map-format.md` §10 is in scope, on Patrik's answer to Q2.** It lists bounds that exist (a map's name and id, a placement's token id). The verifier recommended leaving it out, since this ticket is about bounds that do not exist; Patrik chose to include it, and Task 4 carries it.

**D12. One added Consequence: a text field added to an event goes into the second table in the same change.** It is held by reading. Done item 1's command is how to check it, and no gate is built (Q6). *Forced by:* without it, the second table is a snapshot that stops being complete the first time `events.proto` gains a text field.

## Questions for Patrik

- **Q1. A CLAUDE.md rule, or SPEC-018 alone?** Recommendation: SPEC-018 alone (D10).
- **Q2. Fix `docs/map-format.md` §10's missing bounds here?** Recommendation: no, not in this ticket. If you want it, it is about three list items. Either the ticket writer adds the file and a Done item, or it becomes its own small ticket.
- **Q3. Do refusals that quote such a value fall under the same answer?** Recommendation: yes, in one sentence (D7).
- **Q4. Is there an implementation report?** Recommendation: no (D9).
- **Q5. Test 2's wording.** Does a length nobody has written, only reached in principle, count as "a need shown"? Recommendation: no. The draft requires a test or recipe, a value a real command or file writes, and a participant other than the writer. Reason: the DM, the agent and the person who installs a file are trusted with a text's length just as with its content. Frame size already has its own recorded ceiling (`mapdef.MaxWireTiles`, and the `SceneCreated.tiles` comment in `events.proto`).
- **Q6. Add D12's Consequence, and should Done item 1's command become a gate step?** Recommendation: add the Consequence, but do not build the gate step now. It would be new apparatus, and ADR-007 makes event additions rare and additive.

## Patrik's answers at sign-off, 2026-10-09

- Q5: the two tests as D4 words them, test 2 the narrow form.
- Q1 and Q4: SPEC-018 alone; no CLAUDE.md line and no implementation report.
- Q2: yes, here. The ticket now names `docs/map-format.md` §10 under "What it
  touches" and as Done item 4, so Task 4 below is no longer conditional.
- Q6: the plan as written; D12's Consequence is added and Done item 1's
  command stays out of `task check`.
- Q3 was not asked on its own; the plan's sentence on refusals (D7) went to
  sign-off inside the draft.

## Tasks

### Task 0 — Patrik answers Q5 and Q6 (blocks Task 1)

Q1, Q2 and Q4 only add or remove the conditional tasks at the end.

### Task 1 — Edit SPEC-018

File: `docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`. Nothing else.

1. In `How it works`, delete the sentence from " A `ConditionApplied`'s" through "is bounded by no fold." It ends the paragraph that opens `**Twenty-four fields are bounded**`.
2. Insert the draft below directly before `**The refusal reaches the issuer.**`, with a blank line on each side.
3. In `Consequences`, replace the third bullet and add the two new bullets from the draft.

Done when all of these hold:

- (a) Done item 1's command, run from the repo root, prints nothing and exits 0. It printed 44 names at verification.
- (b) `grep -c "A \`ConditionApplied\`'s" docs/specifications/018-the-fold-bounds-free-text-in-bytes.md` prints `0`.
- (c) `grep -n '^## ' docs/specifications/018-the-fold-bounds-free-text-in-bytes.md` prints exactly the five headings, in order. `grep -n -E '^(Rejected|Alternatives considered)|^## Why' docs/specifications/018-the-fold-bounds-free-text-in-bytes.md` prints nothing.
- (d) `git diff -U0 HEAD -- docs/specifications/ | grep -E '\.(go|ts):[0-9]|\.superpowers/'` prints nothing.
- (e) Every row's command in "Verification commands" below still prints what is recorded there.

### Task 2 — Phase 4b reading review

The reviewer receives:
- `git diff HEAD`, which includes the untracked ticket and plan as added files;
- the ticket;
- this plan's draft and its verification commands.

The reviewer reruns every sentence against the tree and does not trust the plan's notes. The review is done when its findings are settled.

### Task 3 — Commit, then the gate

- Commit the ticket, the plan and the SPEC-018 edit, following the dev-cycle commit rules and the `review-gate` hook.
- Then run `task check` whole and see it green (Done item 5). Launch it in its own process session (`start_new_session=True`), so that ending the launching shell does not end the gate.

### Task 4 — `docs/map-format.md` §10 (Q2: yes)

Edit `docs/map-format.md` §10 and nothing else in it. Item 8 already states the object id bound. Add these three:
- a map's `id` is at most 128 bytes (`mapdef`'s `maxIDBytes`);
- its `name` is at most 256 bytes (`maxNameBytes`);
- a placement's `token_id` is at most 128 bytes.

The ticket names the file and carries it as Done item 4.

### Task 5 — not done (Q1: no)

Patrik answered SPEC-018 alone, so no CLAUDE.md line is added.

## Done item 1's command

Run it from the repo root. It prints every text field of `contract/vtt/v1/events.proto` that no table row of SPEC-018 names:
- each field is written `Message.field`;
- a nested message goes under its own name;
- map keys and repeated fields are included;
- `google.protobuf.Struct` fields are included (D8), and so are `optional` and `bytes` fields;
- only lines of SPEC-018 that start with `|` are searched, so a mention in running text does not count.

It exits 1 when it prints anything, or when it finds no fields at all. On stderr it reports how many fields it checked.

```sh
python3 - <<'EOF'
import re, sys
proto = re.sub(r"//[^\n]*", "", open("contract/vtt/v1/events.proto").read())
spec = "\n".join(l for l in open("docs/specifications/018-the-fold-bounds-free-text-in-bytes.md") if l.startswith("|"))
FIELD = r"(?:(?:repeated|optional)\s+)?(?:map<\s*string\s*,[^>]*>|string|bytes|google\.protobuf\.Struct)\s+(\w+)\s*=\s*\d+"
stack, fields = [], []
for tok in re.findall(r"[{}]|[^{}]+", proto):
    if tok == "{":
        continue
    if tok == "}":
        stack.pop()
        continue
    for stmt in tok.split(";"):
        s = stmt.strip()
        head = re.search(r"\b(message|oneof|enum)\s+(\w+)\s*$", s)
        if head:
            stack.append((head.group(1), head.group(2)))
            continue
        f = re.match(FIELD, s)
        if f:
            owner = [n for k, n in stack if k == "message"][-1]
            fields.append(f"{owner}.{f.group(1)}")
missing = [n for n in fields
           if not re.search(r"(?<![A-Za-z0-9_])" + re.escape(n) + r"(?![A-Za-z0-9_])", spec)]
print("\n".join(missing))
print(f"checked {len(fields)} fields; {len(missing)} not named", file=sys.stderr)
sys.exit(1 if missing or not fields else 0)
EOF
```

At verification (`121d61e`) it reported `checked 68 fields; 44 not named` and exited 1. An independent grep over the same proto also counted 68 text fields. Against a copy of SPEC-018 with the draft applied, it reported `0 not named` and exited 0. The Phase 4b review tightened it to table rows and to `optional` and `bytes` fields; a scratch copy of the proto with an `optional string` field added had still reported 68.

## The draft

SPEC-018 as committed differs from this draft: the Phase 4b review's findings (M1, S1 to S5 and the nits) were applied to the spec directly, and the spec is the text to read.

### How it works: inserted before `**The refusal reaches the issuer.**`

```markdown
**A text outside the table has no bound of its own, by decision.** A field
joins the table when one of two things is shown: that a role other than the
DM and the agent can put text it chose into the field in the log, other than
an id `engine.Apply` or `Resolve` requires to name something already folded;
or, by a test or a recipe, that a value of the field which a real command or
file writes breaks a frame, a view or a refusal that a participant other than
its writer receives. No field outside the table meets the first: each of
SPEC-013's player cells puts text it chose only into a field of the table or
into an id that is looked up. What holds each text instead:

| Field | What holds it |
|---|---|
| `TokenMoved.token_id`, `TokenPlaced.scene_id`, `TokenPlaced.actor_id`, `TokenRemoved.token_id`, `ActorRemoved.actor_id`, `DoorOpened.scene_id`, `DoorClosed.scene_id`, `ResourceChanged.actor_id`, `ResourceChanged.resource`, `ConditionApplied.actor_id`, `ConditionRemoved.actor_id`, `ConditionRemoved.condition_id`, `NoteDeleted.key`, `ActorControlGranted.actor_id`, `ActorControlRevoked.actor_id` | `engine.Apply` refuses a value that names nothing it has folded, so each equals an id or a name the table bounds |
| `Actor.controller_id`, `Actor.controller_ids` | `engine.Apply` refuses an `ActorAdded` that declares either, and fills them from `ActorControlGranted.participant_id` |
| `AbilityUsed.actor_id`, `AbilityUsed.target_ids` | `Resolve` refuses a value that names no folded actor |
| `ConditionApplied.source`, `ResourceChanged.reason`, `ConditionRemoved.reason` | `Resolve` composes each from an ability id and a branch label, `effect` or `usage`, or from a resource name, each of which `internal/rules/load.go` bounds; `ToEvent` writes a `remove_condition`'s reason as `manual` |
| `AbilityUsed.outcome_summary`, `AbilityUsed.Roll.expression` | `Resolve` writes the first from an ability's display name, each target's id and, for an ability with a resolution, a branch label and two totals, and the second from the ruleset's compiled expressions; no loader bounds a display name or an expression |
| `TileRef.kind`, `TileRef.material` | `BuildSceneCreated` writes them from `StandardTile`'s vocabulary |
| `TileRef.art`, `SceneObject.art` | `BuildSceneCreated` writes an art id only when `internal/artlib` resolves it, and `isArtID` refuses one longer than `maxArtIDLen` |
| `SceneCreated.tiles` keys | `BuildSceneCreated` writes a key only for a square of the grid |
| `TokenHidden.token_id`, `SceneSeen.scene_id`, `SceneSeen.tiles`, `SceneSeen.visible` | none reaches the log: `internal/gateway/project.go` builds them for one viewer from folded state (SPEC-016) |
| `Envelope.event_id`, `Envelope.participant_id`, `Envelope.actor_role`, and `Envelope.session_id` but a `SessionStarted`'s | the server writes them: `newEventID`, the issuing participant's id and role, and `stampSessionIDAgainst`, which writes the open session's id or none |
| `AttackRolled.attacker_id`, `AttackRolled.target_id`, `AttackRolled.expression`, `AttackRolled.versus`, `AttackRolled.outcome`, `Modifier.source` | nothing in production writes an `AttackRolled` (`git grep -n 'AttackRolled' -- '*.go' ':!*_test.go' ':!contract/gen' ':!contract-spike'` prints only `engine.Apply`, its doc comment included, and the projection's `classify`), and `engine.Apply` reads none of it |
| `SceneObject.kind` | nothing: it is a map's or an adventure's object `kind`, which no loader checks, and the fold keeps it; `describeBlockage` clips it where a player's move refusal names it (SPEC-013) |
| `Actor.module_data` | the WebSocket frame alone: `add_actor`, a command of the DM and the agent, is its one writer, and the fold keeps it; its shape is a rule module's (SPEC-007) |

A string a map, an adventure or a ruleset file carries and its loader does not
bound is under the same decision: a ruleset's display names, descriptions,
expressions, atom ids, param names and graph keys and its manifest's id and
name; a map's or an adventure's `overrides` values and objects' `kind` and
`art`; and a map's placement `actor_id`. One reaches the log only as a field
of the two tables above, and a ruleset's display names, descriptions and
manifest id and name reach every participant through `/api/ruleset`
(SPEC-012). A refusal that quotes such a value, or an id a command gave that
names nothing, goes to the issuer of the command that met it (SPEC-013), or,
when a file is refused at boot, to whoever starts the server.
```

### Consequences: the third bullet replaced, two bullets added after it

```markdown
- A field added to an event without a bound here is bounded by no fold: by the
  WebSocket frame when a command carries it, by nothing when a map, adventure
  or ruleset file does. It goes into the second table above, with what holds
  it, in the same change.
- No field joins the table unless one of the two tests in How it works is met.
  A change that takes away what holds a text in the second table, or that lets
  a role other than the DM and the agent write one, puts that text to the
  tests again.
- A bound added to a field later refuses every log that already holds a
  longer value, as a lowered bound does.
```

The draft's AttackRolled cell quotes its search, as catch 10 requires. If the reading review finds the cell too long, the search can move to a sentence after the table.

## Verification commands

Each command below supports one sentence of the draft. Run them from the repo root. Each was run at `121d61e` and printed what is noted.

- **Lookup row.** `grep -n -E 'st\.(Tokens|Scenes|Actors|Notes)\[(tm|tr|tp|ar|do|dc|rc|ca|cr|nd)\.|actor\.Resources\[rc\.Resource\]|st\.Actors\[actorID\]|c\.ID == cr\.ConditionId' internal/engine/apply.go` prints a lookup in the `TokenPlaced` (scene and actor), `TokenMoved`, `TokenRemoved`, `ActorRemoved`, `DoorOpened`, `DoorClosed`, `ResourceChanged` (actor and resource), `ConditionApplied`, `NoteDeleted` and `ConditionRemoved` (actor and condition) arms, and in `controlTarget`.
- **Controller row.** `grep -n -E 'declares a controller|append\(actor\.ControllerIds' internal/engine/apply.go` prints the refusal and the grant's append.
- **`AbilityUsed` ids row.** `grep -n -E 'st\.Actors\[(casterID|tid)\]' internal/rules/resolve.go` prints two lines.
- **Composed reasons row.**
  - `grep -n -E 'Sprintf\("(ability|threshold):' internal/rules/resolve.go` prints the `ability:%s:usage`, `ability:%s:%s` and `threshold:%s` formats.
  - `grep -n '"manual"' internal/gateway/convert.go` prints one line.
  - `grep -n maxIDBytes internal/rules/load.go` prints the id, branch, attribute, defense and resource-name bounds.
- **Summary and expression row.**
  - `grep -n -E 'summaryParts = append|Expression: ' internal/rules/resolve.go` prints the two summary formats and the `RollSrc`, `VsSrc` and `DeltaExprSrc` writes.
  - `grep -n -E 'len\([^)]*(Name|Expr|Src|Description)[^)]*\) >' internal/rules/load.go` prints only the resource-name bound.
- **Tile kind and material row.** `grep -n 'StandardTile(base)' internal/mapdef/resolve.go` prints one line.
- **Art row.**
  - `grep -n -E 'if !isArtID|func isArtID|maxArtIDLen = ' internal/artlib/artlib.go` prints the constant, the function and the check in `Library.Lookup`.
  - `resolveObjectArtWith` returns `o.Art` only after the error switch falls through.
- **Tiles keys row.** `grep -n 'key := squareKey(x, y)' internal/mapdef/compile.go` prints one line, inside `BuildSceneCreated`'s grid walk.
- **Projection row.** `git grep -n -E 'TokenHidden\{|SceneSeen\{' -- '*.go' ':!*_test.go' ':!contract/gen' ':!contract-spike'` prints only `internal/gateway/project.go`.
- **Stamps row.**
  - `git grep -n -E 'env\.(EventId|ParticipantId|ActorRole) =|ParticipantId: p\.ID|ActorRole: +string\(p\.Role\)' -- 'internal/gateway/*.go' ':!*_test.go'` prints `ToEvent` and the four batch handlers (`adventure.go`, `map.go`, `ruleset.go`, `handleRemoveActor` in `server.go`).
  - `stampSessionIDAgainst`'s body gives a `SessionStarted` a fresh `sess-` id and every other envelope the open session's id or `""`.
- **AttackRolled row.** `git grep -n 'AttackRolled{' -- '*.go' ':!*_test.go' ':!contract/gen' ':!contract-spike'` prints nothing; the spec quotes that search after the table.
- **`SceneObject.kind` row.**
  - `git grep -n -E 'o\.Kind|obj\.Kind|\.Kind == ""|len\([a-z.]*Kind\)' -- internal/mapdef internal/adventure ':!*_test.go'` prints two conversions and the actor kind check, and no check on an object's kind.
  - `grep -n 'artlib.Clip(kind' internal/gateway/server.go` prints `describeBlockage`'s clip.
- **`module_data` row.**
  - `git grep -n -i -E 'module_?data' -- '*.go' ':!*_test.go' ':!contract/gen' ':!contract-spike'` prints only `tools/toolgen`'s descriptions and an `internal/mcp` instruction string. Nothing in `internal/adventure` writes the field.
  - `grep -n 'proto.Clone(a)' internal/engine/apply.go` prints the `ActorAdded` arm's store.
  - `grep -n 'harnessDial = harness.Dial' internal/mcp/server.go` prints one line, which shows the agent's commands cross the frame.
- **File strings and refusals paragraph.**
  - `git grep -n -E 'rules\.Load\(|adventure\.Load\(|mapdef\.LoadInstalled\(' -- cmd internal/gateway ':!*_test.go'` prints the boot calls in `cmd/vtt` and `handleLoadMap`'s on-demand lookup.
  - SPEC-012's `/api/ruleset` paragraph names the served fields, and states that `handleRuleset` consults no role.
- **Player-cells sentence (reading).** For each player cell in SPEC-013's table, take `ToEvent`'s arm or the handler: `move_token`, `use_ability` (`Resolve`), `remove_condition`, `add_narration`, `revoke_actor_control`, `open_door` and `close_door`. Each puts text the player chose only into `TokenMoved.reason`, `TokenMoved.token_id`, `AbilityUsed.ability_id`, `actor_id` and `target_ids`, `ConditionRemoved.actor_id` and `condition_id`, `NarrationAdded.text` and `as`, `ActorControlRevoked.actor_id` and `participant_id`, or `DoorOpened`/`DoorClosed.scene_id`. Each of these is in the table or looked up. `Resolve` also copies the player's ability id into the `ability:` reasons and the target ids into `outcome_summary`; the ability id is in the table and declared by the ruleset, the target ids are folded actors. The spectator's `set_viewpoint` appends nothing (`handleSetViewpoint` answers without an append).

## Gaps

- **G1. False or overstated claims in the ticket's "The problem".** The draft corrects each one. Phase 4b checks that the spec text has not inherited any of them.
  - "every envelope's `session_id` but a `SessionStarted`'s" is wrong: `stampSessionIDAgainst` also stamps a `SessionStarted`, with a fresh `sess-` id. That envelope's id is in the table because the fold bounds it, not because nothing stamps it.
  - "it keeps no `AbilityUsed` field but the ability id": the fold keeps no `AbilityUsed` field at all. It checks the ability id's length and returns.
  - `SceneCreated.tiles` keys are written by `BuildSceneCreated`'s own walk of the grid. `CheckTilesInsideGrid` is the loaders' refusal of a file key outside the grid; it is not what writes the keys.
  - `outcome_summary` also carries each target's id, and, when the ability has a resolution, a branch label and two totals. It is not only display names.
  - The closing claim is false as worded. A player's `use_ability` writes `outcome_summary`, `Roll.expression`, `ResourceChanged.reason`, `ConditionApplied.source` and `ConditionRemoved.reason`, and `remove_condition` writes `ConditionRemoved.reason`. None of these is looked up, and every envelope carries server-written fields. What is true is that no text a player chose reaches such a field.
  - The 2026-10-07 report sends the envelope fields and `module_data` "nowhere, unless the owner names them", not "nowhere yet; raised to the owner".
  - "Each reaches only the issuer" (open question 3): an adventure loader's refusal happens at boot and goes to whoever starts the server.
  - The ticket's list leaves out `Modifier.source` and `Actor.module_data`.
- **G2. `Actor.module_data` is a `google.protobuf.Struct`, not a string.** A command that follows the ticket literally would not see it. The plan's command includes Struct fields (D8).
- **G3. The closing claim, correctly worded, is a property of the system that nothing holds.** Granting players `add_actor`, or adding a string a player sets to a player-allowed command, would break it silently. D12's Consequence and the second bullet hold it by reading only. Whether it becomes a requirement with a test is Patrik's call (Q6). The recommendation is not now.
- **G4. The criterion's test 2 meets the agent's 200 KiB read limit (`internal/harness/client.go`'s `readLimit`) in principle** (D4, Q5).
- **G5. Done item 1's command is not a gate step.** The second table can fall out of date when an event gains a text field (Q6).
- **G6. Out of SPEC-018's subject.** `commands.proto` and `ServerFrame` strings that never reach the log (for example `PresenceChanged.display_name` and `CommandResult.warnings`) are not covered. The ticket scopes the work to `events.proto`, which is correct for a record about the fold.
