# The log only goes forward

**Sub-project 13.** Undo leaves the platform. Corrections happen by appending,
which is what an append-only log is for.

**Prerequisite for sub-project 14** (`2026-08-30-per-character-logs-design.md`),
several of whose decisions — carrying a world sequence as provenance, above all
— are only safe once nothing can delete by sequence number.

---

## 1. Why this

**Patrik, 2026-08-30: "Retractions are not allowed in the platform. They make no
sense, since the human mind will not forget."**

A retraction's purpose is to make something not have happened. It cannot do
that. The player read the log, saw the goblin, and knows where it was. Rewriting
the record does not rewrite them.

**What pretending otherwise cost.** Sub-project 12 spent four fix rounds, a
contract change and roughly a thousand lines chasing a permanently frozen
client. The chain, recorded because the shape recurs:

> an unachievable goal (un-showing) → a stamping decision to serve it
> (visibility spec §4.2 gives a synthesized introduction the sequence of the
> event that revealed it, so retracting the cause removes the sighting) → a
> derived per-viewer artifact made deletable by an operation with authority over
> the log and none over what a person saw → a dangling reference → a client
> frozen permanently, because `session.ts` re-folds its whole log on every
> event → a `Restart` frame → a second `ServerFrame` consumer nobody had named →
> a soak keystone that could no longer keep its counts.

Every link exists to deliver forgetting. Remove the goal and the chain does not
start.

**The corrections people actually want are appends.** The goblin was in the
wrong place, so the DM moves it — and everyone who saw it move learns something
true. That is the operation an append-only log is built around, and it works
today.

---

## 2. Non-goals

**Migrating existing logs.** The platform has never been used for real. There
are no stored campaigns to preserve, which is what makes total removal available
rather than a compatibility exercise.

**`delete_scene`.** §5.3 says why, and the absence is principled rather than
deferred.

**Converting the repo's 82 bare line-number citations.** §7 lands the
convention; the sweep is its own piece of work, and the gate that would enforce
it is apparatus, which is paused.

**A general "correction" framework.** Two commands, both concrete, both earned
by a mistake somebody can actually make.

---

## 3. What leaves

Total removal, code and contract and fixtures:

- **Contract:** `RetractEvents`, `EventsRetracted`, and their oneof arms.
- **Campaign:** `campaign.Undo`, `retractedSet`, and `FoldPrefix`'s two-pass
  structure. The fold collapses to a single pass, because the only reason it
  collected a set before applying anything was that a retraction is retroactive.
- **Harness:** `harness.Fold`'s matching two-pass, and the retraction steps in
  the scenario engine.
- **Client:** `client/src/undo.ts` entirely; `fold.ts`'s two-pass; the feed's
  retraction rendering; the DM console's Undo controls.
- **Gateway:** the `retract_events` handler and its authorization row.
- **MCP:** the tool, which removes it from the agent's surface.
- **Fixtures:** the two scenario definitions that retract — `denials.json` and
  `three-role-exit.json` — and their goldens. (An earlier draft said four; it
  had counted `goldens/README.md` and a recorded golden stream, which are
  artifacts rather than definitions.)

Roughly fifty files carry the word today.

**The single-pass fold is the prize.** Both languages currently walk the log
twice — once to learn what was retracted, once to apply — and every reader of
either fold has to hold that in their head. Afterwards the fold is what it looks
like: apply each event in order.

---

## 4. ADR-007 binds from release, and the gate learns the same rule

ADR-007 says contract evolution is additive only, and `check:breaking` enforces
it with `buf breaking --against main` under `use: [FILE]`, buf's strictest
category. Deleting a message or a oneof arm fails that gate.

**What additive-only protects is compatibility with artifacts outside this
repo** — a client someone else built, a message already stored. None of that
exists. The rule is guarding a hazard that has not arrived.

**Amendment (Patrik, 2026-08-30):** additive-only binds from the first release
that others can run. Before that, a breaking change is permitted with a stated
reason.

**And the gate learns the same trigger, because a rule stricter than it means is
a rule people work around.** `check:breaking` looks for `contract/RELEASED`:

- **Absent** — pre-release. `buf breaking` still runs and still prints what
  would break, so the information is never lost. It does not fail the build.
- **Present** — released. It fails, exactly as it does today.

Creating that file is one deliberate commit. It makes "we have released" an
event in the repository rather than a belief different people hold on different
days, and it is the day the cost of every contract decision changes.

This is a gate change, and gate work is otherwise stopped (Patrik, 2026-08-27).
It rides along here because this sub-project cannot proceed without it, and
because the alternative — a one-off exception — leaves the rule misfiring on
every pre-release deletion that follows.

---

## 5. What must exist because undo left

**The rule these follow:** removal means *no longer part of the world going
forward*, never *this never was*. Nothing here un-shows anything.

### 5.1 `remove_token` → `TokenRemoved`

Takes a piece off the board. Distinct from `TokenHidden`, which is
projection-only and means "you cannot see it"; this means it is not there.

### 5.2 `remove_actor` → an ordered atomic batch

A `TokenRemoved` for each of that actor's tokens, then `ActorRemoved`.

Control grants need no event of their own: `controller_ids` is a field ON the
actor, so removing the actor removes them with it. An earlier draft left "grants
go with it" ambiguous between a cascade and a consequence. It is a consequence.

**The cascade is a correctness requirement, not a convenience.** `engine.Apply`
and `client/src/fold.ts` both reject a token whose actor is unknown, in almost
the same words. An `ActorRemoved` that left tokens behind would produce a log
that no longer folds — the exact defect class §1 describes. The batch is the
shape `load_map` already uses: one `SceneCreated` plus one `TokenPlaced` per
placement, accepted or rejected atomically.

### 5.3 There is no `delete_scene`

A scene is not *in* the world; it **is** the world. A room does not stop
existing because the party left it, and a room nobody ever entered costs
nothing — it is an unused place, not corruption. A room somebody has stood in
cannot honestly be deleted at all, which is §1's problem wearing different
clothes.

An earlier draft listed `delete_scene` as a gap. It was enumerating what undo
used to cover rather than asking what needs covering.

---

## 6. `create_scene` requires complete terrain

`create_scene` is the improvised path — how a place comes into existence during
play, when no authored map file exists. An LLM DM narrating a room into being
needs it, and it stays.

> **AMENDED 2026-09-01 — Patrik overruled the second sentence.** The completeness
> rule below shipped and stands (`e110e9b`). The conclusion that `create_scene`
> "stays" did not survive the whole-branch pass that landed with it. **The kernel
> serves maps; it does not make them.** `create_scene` leaves the platform in
> sub-project 15, and a future map editor's output arrives as `load_map`.
>
> The argument, recorded because it generalises past this command: `create_scene`
> is a ONE-SHOT command and terrain authoring is ITERATIVE. A scene's tiles are
> written once by `SceneCreated`, which refuses a second one under a live id
> (`engine.Apply`: "scene %q already exists"), so nothing can revise them
> afterwards — and with retraction gone, nothing can take the mistake back either.
> That was survivable while a bare grid was legal; the completeness rule below
> makes it acute, because now every square must be named correctly on the first
> and only attempt. The moment terrain has any variety worth authoring, the author
> needs to see it, change it, and see it again — which is an editing surface, and
> an editing surface is a map editor, which is its own tool with its own file
> output.
>
> The same conclusion had just arrived as a defect rather than as a principle.
> `e110e9b` gave the DM console's new-scene form a `wall` option beside `floor`,
> on the argument that "fill solid, then carve" is how a room gets built; nothing
> carves, so a DM could build a room no player could ever stand in, five clicks
> from a blank form, with `load_map` unable to repair it because the scene id is
> taken and no command left that could take the mistake back. `fcf45cf` removed
> the option and recorded the reasoning at `kindSelect`'s doc comment in
> `client/src/view/dm.ts`, which is where the five-clicks observation is written
> down.
>
> This does not weaken anything below. Until sub-project 15 removes it,
> `create_scene` is live and its completeness rule is enforced.

But a scene created with no terrain is a featureless grid: no walls, so
`internal/sight` has nothing to occlude with and everyone sees everything. That
is the hole maps-as-geometry was built to close — *a wall nobody declared is an
invisible barrier* — and an improvised room must not quietly opt out of it.

**So tiles become mandatory, and must hold an entry for every square of
`grid_width × grid_height`.** That is not a new rule; it is `mapdef`'s rule
(`format.go`: "IF DECLARED, it must hold an entry for every square"), and
`docs/map-format.md` states there is no implicit fallback anywhere. Applying it
to the improvised path makes both paths obey one standard, which is what
maps-as-geometry intended when it routed them through one construction site.

A validation change in `create_scene_validate.go`, which already checks tile
kinds against `mapdef`'s vocabulary. The contract does not move.

*CORRECTED 2026-08-31 — the WIRE does not move; the contract's published
DESCRIPTION of itself does.* No field number, type or name changed, so
`check:breaking` had nothing to object to and `tiles` is still
`map<string, TileRef> tiles = 5`. But both places that TELL a caller what the
field means told the caller it was optional, which `e110e9b` made false. The MCP
tool schema said "Optional; omit for a bare grid with no terrain"; the
`CreateScene.tiles` doc comment in `contract/vtt/v1/commands.proto` said
"Optional: omitted entirely, the scene is a bare grid with no terrain" — two
wordings, one claim, both now replaced. In the schema `tiles` also joined the
`required` array (`contract/gen/tools/tools.json`,
`cmd/vtt/tools.json`, `contract/testdata/expected_tools.json`, all generated
from `tools/toolgen`). An agent reads the schema, not the validator, so leaving
those alone would have advertised a field as optional that the gateway refuses
to omit.

The eight scenarios that create scenes need terrain added and their goldens
regenerated.

*CORRECTED 2026-08-31 — eight scenarios, SEVEN golden directories.* Eight of the
nine committed scenarios create a scene and all eight needed terrain
(`denials`, `goblin-fight`, `session-zero`, `shared-control`, `smoke`,
`story-table`, `three-role-exit`, `toy-brawl`), but `goblin-fight` has no golden
directory, so seven sets of goldens were re-derived and re-recorded. The ninth
scenario, `adventure-night`, has goldens and needed no terrain: it brings its
place in through `load_adventure` rather than `create_scene`, and its golden
directory is untouched across the whole arc.

---

## 7. The citation convention

Patrik's ruling, 2026-08-30, currently living only in a gitignored ledger and a
cancelled plan. It lands in `CLAUDE.md`:

> Cite a **name** — a function, a test, a constant, a named arm. For a target
> with no name, place an `[anchor:kebab-name]` at it and cite that string. A
> bare `file.go:123` in prose is out. Mutation-adjudication coordinates are the
> sole exception: `file:line:col` is a mutant's identity, generated by the gate.

**The reason belongs with the rule, because it is what makes it stick.** A stale
line number fails *silently* — it still looks valid while pointing at the wrong
code, and a reader cannot tell without reconstructing history. A deleted anchor
fails *loudly*: grep returns nothing. A wrong answer becomes no answer.

*AMENDED 2026-08-31 — the rule that landed bans THREE shapes, not one.* The
quoted draft above rules out a bare `file.go:123`; `CLAUDE.md` rule 8, landed by
`ac307d5`, rules out two more that fail the same way — still looking valid while
resolving to nothing:

1. A bare `file.go:123`, as drafted.
2. **A path into `.superpowers/`.** That tree is gitignored, so a
   `task-N-brief.md`, `task-N-report.md` or `sdd/progress.md` citation resolves
   only on the machine that wrote it; a fresh clone and CI have neither. A bare
   `task-3-brief.md` does not even say which plan's — measured 2026-09-01, 63
   such citations across 33 committed files, with four sibling plan directories
   each holding a `task-3-brief.md`. `docs/superpowers/plans/` is tracked, so
   naming a plan and a task within it is fine.
3. **"this task" / "this same task".** It names a workflow run that ended, and
   the next reader has no way to find out which one.

The mutation-adjudication exception survives as drafted, with one addition: the
coordinate is re-pointed in the adjudication entry when the file moves. Rule 8
also states its own scope — it binds prose written from here on, and the sweep
over existing citations is explicitly not part of its arrival, which is §2's
non-goal restated where the rule lives.

---

## 8. Testing

**The removal is proved by absence, and absence needs a test that can fail.** A
gate-level check that no `retract`/`Retract` identifier survives outside this
spec and the ADR — a grep with a stated expected count of zero — so a
half-finished removal reds rather than lingers.

*CORRECTED 2026-08-31 — what shipped is not a grep, and could not have been.*
`tools/check-no-retraction.py` (`ac307d5`, extended in `fcf45cf`) MASKS comments
and string literals first and inspects only what is left, which is code
positions; in `.json` it keeps object KEYS and blanks every value, because a key
is the protobuf field name a scenario step or a golden stream is naming. An
expected count of zero over raw text was never available: the word is still
everywhere in this tree on purpose — dated change-records, this spec, the
comments that stop the next reader re-deriving the ruling — measured at 305
occurrences across 61 files after `.json` scanning began, and the only way to
green a naive grep would be to exclude those 61 files, at which point the gate
enforces nothing. An IDENTIFIER is what a reintroduction actually looks like,
and there are 16 of those, all of them the enforcement machinery itself.
`tools/check_no_retraction_test.py` pins the masking in both directions. The
gate has one stated limit rather than a hidden one: inside a Python f-string the
interpolated expression is masked with the literal, so an identifier used only
there is invisible to it.

**The single-pass folds keep their existing suites.** Both `campaign` and
`fold.ts` have coverage that must stay green through the collapse; a fold that
changes shape while its tests keep passing is exactly what those tests are for.

**The two new commands** get the treatment this repo gives every command:
authorization rows in `authzCases`, wire-shape assertions in
`commands.test.ts`, fold arms pinned in both languages, and — for
`remove_actor` — a test that the batch is atomic, and one that proves the
resulting log still folds with the actor gone.

**`create_scene`'s completeness rule** is pinned on the boundary: a scene one
square short of its grid is refused, and the refusal names the missing square.

---

## 9. What could go wrong

**The scenario churn is the bulk of the work, not the removal**, and it is worse
than "regenerate". Two scenario definitions retract; eight create scenes without
full terrain; there are eight golden directories, each holding a `state.json`
HAND-DERIVED from the scenario and a `stream.json` RECORDED from the server.

`cmd/vtt/scenario_goldens_test.go` refuses a shortcut in as many words: "THERE
IS DELIBERATELY NO -update FLAG […] a regenerate-on-demand switch is exactly how
a golden stops being a claim anyone checked." The two files' agreement is
evidence precisely because neither was produced from the other. So every
affected `state.json` is re-derived by hand from the edited scenario, and every
`stream.json` re-recorded from the server, and the two are then checked to
agree. That is the shape of the work, and it is most of this sub-project.

**The `RELEASED` marker could be forgotten.** A gate that does nothing until
someone remembers to switch it on is a gate nobody switches on. Mitigated by the
gate printing, on every pre-release run, that it is reporting rather than
enforcing and naming the file that changes it.

**`remove_actor` against an actor with a character log** (sub-project 14) does
not touch that log. The actor leaves the world; the history stays. Recorded here
so the two sub-projects do not disagree later.

---

## 10. Exit criteria

1. No `retract` identifier survives anywhere in `internal/`, `client/src/`,
   `cmd/`, `contract/` or `scenarios/`, proven by a gate rather than a search.
2. Both folds are single-pass, and their existing suites are green.
3. `remove_token` removes a piece; the log still folds.
4. `remove_actor` removes an actor and its tokens atomically; the log still
   folds; a partial application is impossible.
5. `create_scene` refuses a grid with any undeclared square, and says which.
6. `check:breaking` reports without failing while `contract/RELEASED` is absent,
   and fails when it is present — both directions tested.
7. `CLAUDE.md` carries the citation convention and its reason.
8. `task check` green, both mutation gates included.
