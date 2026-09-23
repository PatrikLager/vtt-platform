# vtt-platform — Agent Way of Working

LLM-native VTT platform. Event-sourced Go core, thin TS client (future),
protobuf contract. Start here: `docs/superpowers/specs/` (tickets),
`docs/specifications/` (decisions, present tense), `docs/adr/` (frozen
evidence), `README.md`.
## The process this project runs

This project runs the `dev-cycle` package at project scope. How work is planned,
reviewed, tested, broken on purpose, reported and landed lives THERE and is not
restated here: a rule kept in two places drifts, and the copy is always the one
that goes stale.

Three rules left this file when the package arrived, and their numbers are left
empty rather than closed up, because other files cite these by number. **1** was
tests-before-code — the package says it, and adds the deliberate break
afterwards. **6** was review-before-commit. **7** was specs-are-truth. Do not
reuse the numbers.

An improvement to the process itself is a TICKET raised to the process's own
repository, developed there, and inherited here once it lands. This project
never edits it.

## How this project is built and checked

Phase 1 of the cycle requires a project to have written this down. An agent that
guesses at these runs gates that do not run.

- **Build.** `go build ./...` for the Go core, `task build:client` for the TS
  client. There is no single `task build`, and adding one is not this file's
  decision to make.
- **Tests.** `task check` runs them all. They are tiered by what they ARE, not
  by runtime — 1 unit/area, 2 cross-layer, 3 external, 4 whole product — so a
  slow unit test stays in tier 1. `task check:fast` is an inner-loop
  convenience ONLY and never satisfies the gate.
- **The gate.** `task check`, whole, and NO TRIGGER RUNS IT UNPROMPTED: CI
  runs it only when a person dispatches the workflow by hand.
  Three layers, and only the first two are automatic: the pre-commit hook runs
  lint, vet, tier-1 tests, arch, vocabulary, doc-owner, secrets, typecheck and
  the dev-cycle review gate (`review-gate` in `.lefthook.yml`, which refuses a
  commit unless a review record matches the tree being committed or a
  user-approved skip reason is given); pre-push runs tiers 2-3 and the contract
  gates, drift and breaking; everything else — coverage, race, the prose gates
  and both mutation gates — runs when somebody types `task check` and at no
  other time.
  So the cycle's "run the gate locally" is not a courtesy here: half these
  gates run at no other time, and nothing downstream will notice if it is
  skipped.
  Run `task setup` once per clone (JS deps + git hooks); `task check` runs
  `bun test` and node_modules is gitignored, so a fresh clone fails until it
  does. An unhooked clone is not obvious from the inside — if hooks have never
  fired for you, check `.git/hooks/pre-commit` exists.
- **The exported surface, signatures without bodies.** `go doc <package>`.
  Phase 4a hands this to an agent that may not open the source, so it is
  extracted FOR that agent rather than browsed BY it.
- **Where recorded decisions live.** `docs/specifications/`, numbered
  `NNN-<slug>.md`, in the present tense: how one part of the system works now.
  One file per decision — a decision somebody would otherwise re-open.
- **Where tickets are kept.** `docs/superpowers/specs/`, named
  `YYYY-MM-DD-<slug>-design.md`. THE NAME IS HISTORICAL AND THE FOLDER IS NOT
  RENAMED: tracked files, `Taskfile.yml` and `.go-arch-lint.yml` among them,
  reference the `docs/superpowers/` tree by path. What matters is the kind, and
  these are
  tickets — they carry the problem, the non-goals and the exit criteria.
- **Where implementation plans are kept.** `docs/superpowers/plans/`, same
  naming, same reason for the path.
- **Where implementation reports are kept.** `docs/reports/`, one per arc for
  the arcs that have one.
  TWO STRAYS live in `docs/superpowers/reports/`, the mutation audit and the
  enforcement layer; five references point at them and they have not been
  moved.
- **Where the requirements register lives.** `docs/requirements.md`. Its tag is
  `VTT` and its header is `| Id | Requirement | Verified by |`. An id is
  allocated by `requirement-id`, the dev-cycle package's dispenser, on the path
  while the package is installed and otherwise at
  `~/.claude/plugins/cache/patrik-process/dev-cycle/<version>/bin/`, and never
  chosen by hand. How an id is allocated, cited and checked is SPEC-008 in
  `docs/specifications/`. Nothing checks the chain here today.
- **Where the blueprint is.** Nowhere. This project has none, so no
  specification can honestly name the principles it serves. Writing one from a
  single record would invent the principles, which is what a blueprint exists
  to prevent.
- **What `docs/adr/` is now.** Frozen evidence. Each ADR holds the road to a
  decision — its context, the alternatives weighed, the scorecard. The present
  tense moves out to a specification as each one is read; the file itself is
  never rewritten.
- **Where adjudications go.** Mutation survivors:
  `tools/mutation-equivalents.txt` and `tools/ts-mutation-equivalents.txt`.
  Phase 4a's QA adjudications HAVE NO HOME YET — the first QA run here has to
  give them one, and saying so beats sending them somewhere they do not belong.
- **The ONE file where escaped defects go.** `docs/verification-debt.md`, as a
  RECIPE: the exact edit that puts the defect back, which gate should have
  caught it, and why it did not. Recipes are cheap at the moment of escape and
  near-worthless later, because the tree moves. That file also holds known
  coverage gaps — written as a comment a gap explains one test, written there it
  is a claim on future work.

## Non-negotiable rules

These are this project's own. The process has nothing to say about any of them
and would be wrong to.

2. **`task check` is the gate, and it is never weakened to pass it.** A gate
   change is its own reviewed decision, with its own reason. Lowering a threshold, widening an exemption or
   narrowing a scope to get a green run is the one move that cannot be undone by
   the next commit, because nothing afterwards remembers what the gate used to
   catch. Which gate, and how it runs, is in the record above.
3. **Contract evolution is additive only** (ADR-007). Generated code is
   committed; regenerate via `task generate:contract`. Commands are imperative
   (`LoadMap`), events past-tense (`SceneCreated`). That example named
   `CreateScene` until 2026-09-02, when the command left the platform.
   `check:breaking` does NOT enforce this yet, and an agent who believes it
   does will make a breaking change believing it is guarded. Pre-release it
   REPORTS a breaking change and exits 0 (`6c0eb9a`, and ADR-007's own
   amendment block): the switch is `contract/RELEASED`, and the day that file
   exists the same task fails the gate instead. Until then the rule binds
   because it is the rule, not because a gate stops you.
4. **One fold.** `engine.Apply` (via `campaign.foldEvents`) is the only
   code that changes game state. Never add a second event-application loop.
5. **No game-system vocabulary in platform code** (pillar P2/P4; semgrep
   enforces). Rules concepts live in rule-module data (sub-project 5+).
8. **Citations name durable things.** Cite a **name** — a function, a test,
   a constant, a named arm, a commit hash, a dated decision. For a target
   with no name of its own, place an `[anchor:kebab-name]` at it and cite
   that string. Three shapes are out, and all three fail the same way: they
   still LOOK valid while resolving to nothing.
   - A bare `file.go:123`. The line moves and the citation does not.
   - A path into `.superpowers/`. That tree is gitignored, so a
     `task-N-brief.md`, `task-N-report.md` or `sdd/progress.md` citation
     resolves only on the machine that wrote it — a fresh clone and CI have
     none of those files — and a bare `task-3-brief.md` does not say which
     plan's (measured 2026-09-01: 63 such citations across 33 committed
     files, and four sibling plan directories each holding a
     `task-3-brief.md`). `docs/superpowers/plans/` IS tracked, so naming a
     plan and a task within it is fine.
   - "this task" / "this same task". It names a workflow run that ended;
     the next reader has no way to find out which one.
   THERE IS NO EXCEPTION. One stood here until 2026-09-17 and was withdrawn:
   it said mutation-adjudication coordinates were exempt, because
   `file:line:col` is a mutant's identity generated by the gate. The gate does
   generate it; what we STORE need not be it. Every edit above a key moves the
   key, so each entry below is re-pointed by hand or goes stale in silence —
   and the adjudication files carry a long history of exactly that, including
   re-points whose own notes claimed to have re-resolved what they had not.
   Re-keying these on the statement text the gate already computes is agreed
   and not started; until it lands, a mutant coordinate is still how the gate
   NAMES a mutant, so the adjudication files keep writing them and keep paying
   the re-point cost. That is deliberate, it is the one place a bare coordinate
   is still written, and it ends when the re-keying does.
   A stale line number fails SILENTLY — it still looks valid while pointing
   at the wrong code. A deleted anchor fails LOUDLY, because grep returns
   nothing. A wrong answer becomes no answer.
   This binds prose written from here on. Converting the citations already
   in the tree is NOT part of the rule's arrival, and the gate that would
   enforce it — every citation resolves to exactly one anchor, no
   duplicates, no orphans — is available but unbuilt.
9. **Look at RPTool before you design.** Patrik's ruling, 2026-09-05. Before
   brainstorming, planning or implementing anything in an area a virtual
   tabletop already has to solve, the FIRST question is: how does MapTool do
   this (`~/dev/RPTool`), can we borrow the design, or does what they have
   suggest functionality that fits our shape? Answer it in writing — in the
   spec's own prose during brainstorming, and in the plan before a task is
   dispatched — not after the code exists.
   **Why it is a rule and not advice.** It is twenty years of a working
   product sitting on this machine, and we have already paid for skipping it.
   The client hardcoded a 640x480 pane while `planScene`, `planGrid` and
   `planFog` all took `viewW, viewH` as parameters — the seam was built and
   then fed constants, and it was logged as backlog rather than finished.
   (Fixed the same day by 2026-09-02-art-is-a-flat-library Task 6, which is
   why the constants are no longer greppable: `spectator.ts` now measures its
   container and keeps `DEFAULT_PANE_W`/`DEFAULT_PANE_H` only as the fallback
   a headless test sees. The rule is kept in the past tense on purpose — the
   scar is the argument, and a rule whose example has been repaired still has
   to say what went wrong.) MapTool has had zoom, pan and a viewport that follows the window
   since 2005, and its renderer is one line: `gridSize * zoneScale.getScale()`,
   source size times zoom, every frame. We also put `cell_px` on the campaign;
   MapTool puts grid size on the **Zone**, per map, because that is where it
   varies. Both were avoidable by looking first.
   **What to borrow and what not to.** Take the geometry, the coordinate
   model, the unit split between source pixels and screen pixels, the
   vocabulary. Do NOT take the distribution model: every MapTool client
   receives the whole campaign including the GM layer, and visibility is a UI
   filter rather than a boundary — the opposite of this platform's premise
   (ADR-era ruling; see `internal/gateway/seat.go`). Their answer to "what
   does a seat SEE" is wrong for us; their answer to "how does a square map
   onto pixels" is right and proven.
   Record the answer even when it is "MapTool does not solve this" or "their
   answer does not fit, because X" — a checked-and-rejected precedent is worth
   as much as a borrowed one, and it stops the next person re-asking.

## Layout

`contract/` wire constitution (protobuf, vtt.v1) · `internal/store` append-only
log · `internal/engine` state + fold · `internal/campaign` composition, atomic
batch append, poison contract · `internal/identity` invites/roles ·
`internal/gateway` WS API + authz · `cmd/vtt` CLI · `tools/toolgen` MCP tool
definitions · `contract-spike/` frozen ADR-007 evidence (read-only).
`internal/campaign` said "undo" here until 2026-09-01; `campaign.Undo` was
deleted by `133e896` and nothing replaced it, because the log only goes
forward.
