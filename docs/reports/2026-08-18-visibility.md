# Implementation report — sub-project 11, visibility

Plan: `docs/superpowers/plans/2026-08-18-visibility.md`
Spec: `docs/superpowers/specs/2026-08-18-visibility-design.md`

---

## 1. Vad som byggdes

Session zero (2026-08-12) ended with a player putting `tok-fighter` on (19,8) — the Goblin Archer's exact square on a 32×32 grid — at seq 20, and moving it back at seq 21. Patrik, playing on an iPad, when asked: *"yes, I could see the goblin token on the board — there is no fog/limited view mechanism as far as I can see at least."* Finding 14 named the shape of the fix: the log is the subtle channel, the board is the one that leaks to somebody who is not even trying, and a fix that filters only the log closes the harder hole and leaves the obvious one open.

The arc's answer was **one log, many views**. `internal/store` keeps exactly one append-only log and `engine.Apply` via `campaign.foldEvents` stays the only code that changes state — nothing about visibility enters the fold. What changed is what a *connection* receives. Two pieces were built:

- **`internal/sight`** — a new pure-geometry package. Given an `engine.Scene` and a viewer's square, it answers which squares are visible. Rays run from the viewer's centre to nine sample points on each target square (four corners, four edge midpoints, centre); a tolerance says how many must be reachable. Walls, closed doors and objects carrying `blocks_sight` block; an open door blocks nothing; rotation is ignored. Sight range and tolerance are **inputs** — the platform never decides them. It knows nothing about participants, connections or the wire. 382 lines today, gated by the mutation runner, `.go-arch-lint.yml` and a 99.7 coverage floor **in its first commit** (`a7150c2`, which touched `.go-arch-lint.yml`, `tools/check-mutation.py`, `tools/coverage-thresholds.txt`, `tools/mutation-equivalents.txt` and `tools/mutation-scope.md` alongside the source).
- **The projection in `internal/gateway`** — `project.go` (`Projector`, `Project`, `look`, `classify`, `transitions`), `viewpoint.go` (`MayPerch`, the spectator perch), and the seat wiring in `seat.go`/`server.go`. It filters the event stream per connection at the seam where `serve` already held the participant for the connection's life and already marshalled per connection.

Around those: two projection-only contract messages (`TokenHidden`, `SceneSeen`) and one unlogged command (`SetViewpoint`); fold arms for both in **both** languages plus `Scene.Explored` and `Scene.Visible`; a fog pass and a visible-token seam in the client renderer; and the keystone test in Go and TypeScript over a golden corpus that gained projected player and spectator streams.

Size: **84 files, +17,483 / −482** over the branch.

---

## 2. Hur det fungerar i dag

Verified against the working tree on 2026-09-19 (branch `feat/per-character-logs`, HEAD `48e3fe2`, with sub-project 14's Task 3 uncommitted).

**The seam.** `serve` builds one `seat` per connection (`newSeat`, `seat.go`). `seat.pr` is a `*Projector` for every role **except** DM and agent, where it is `nil` — not "a projector that forwards everything", but nil, so neither the fold nor `sight.VisibleFrom` is ever reached on their path. There is exactly one construction of `ServerFrame_Event` in non-test gateway code (`server.go:1778`), fed by `deliver` from three call sites: catch-up (`server.go:599`), perch (`:737`) and the live pump (`:812`). All three route through `seat.receive`/`seat.perch`, which route through `Projector`.

**Per event.** `seat.receive` appends the envelope to `s.received`, folds that prefix with `campaign.FoldPrefix` — the state *after* this event, not head, because during catch-up head is the future — and calls `Projector.Project(env, world)`. `Project` returns pointer-identical `env` for DM/agent, `nil` for an unknown role (fail closed), and otherwise: `look()` recomputes what the viewer can see, `classify()` returns one of three verdicts (`unrecognised` / `withheld` / `forwarded`) over an exhaustive switch on the `Envelope` oneof with `default: unrecognised`, and `transitions()` synthesizes the difference against the projector's five maps (`scenes`, `actors`, `tokens`, `seen`, `doors`). An `unrecognised` payload emits nothing at all — not even the derived consequences.

**What a viewer gets.** A redacted `SceneCreated` (outline only, empty tiles/objects) when their actor first stands in a scene; a synthesized `ActorAdded` plus per-controller `ActorControlGranted` plus any `ConditionApplied` when an actor becomes knowable; `TokenPlaced` on arrival; `TokenHidden` on departure; `DoorOpened`/`DoorClosed` corrections for visible squares only; and `SceneSeen` carrying the whole current visible set (never a delta) whenever it changes, including an empty one when a scene goes dark. Party members are always in the roster via `engine.IsPartyMember` — the same predicate `MayPerch` and `eyes()` use.

**Key format.** `sight.SquareKey` is `fmt.Sprintf("%d,%d", x, y)`. It was `squareKey`, unexported, at HEAD; the uncommitted change in the tree exports it so `internal/perceive` indexes `VisibleFrom`'s map from *here* rather than from a fifth copy of the format.

**What sub-project 14 replaces.** Its spec §7 is explicit, and the plan's Task 12 names the files. **Deleted:** `internal/gateway/project.go`, `project_test.go`, `project_internal_test.go`, `project_property_test.go`, `keystone_test.go`; the seat's projector, `pastResume`, the subscribe-from-zero rule; and the two-sided keystone `fold(project(log,v)) == visibleState(fold(log),v)`, replaced by a one-sided *every character log folds cleanly, standalone, at every prefix*. `internal/perceive` is the replacement — it is in the tree, staged, and **imported by nothing but its own tests** (verified: `grep -rn "internal/perceive" --include="*.go"` outside the package returns only two mentions inside `sight.go`'s comments). Its own package doc says the move plainly: *"IT IS A MOVE, NOT A NEW IDEA… if one appears to [change], the port is wrong and project.go is right."*

**What survives.** `internal/sight` — spec §7: *"Kept, unchanged: `internal/sight`. Only *when* it runs changes"*, from read time to write time inside `campaign.Append`. `Scene.Explored` and `Scene.Visible` survive (an earlier draft of §7 said otherwise and contradicted its own next paragraph). Both fold arms survive; `client/src/fold.ts` folds a character log exactly as it folds today's projected stream. The client renderer survives. `viewpoint.go`/`MayPerch` survives and is generalised (Task 11): `set_viewpoint` stops being spectator-only and becomes how any seat selects a viewpoint. `engine.IsPartyMember` — moved into `engine` in `48e3fe2` precisely so both sides reach one answer — survives.

**What is about to be lost with the deleted files** is not the rules (they are ported) but the *reasons*: ~723 comment lines in `project.go` and ~311 in `seat.go`, carrying measurements, rejected alternatives and corrections that exist nowhere else. That is what sections 3 and 4 below are for.

---

## 3. Besluten och varför

**Visibility is derived, never folded.** Rejected: putting visibility in `engine.State`. Reason: CLAUDE.md rule 4 — `engine.Apply` is the only writer. `internal/sight` follows `engine.State.Blocked`'s precedent as a derived spatial query.

**The keystone invariant.** Spec §4.3:

> For any log and any viewer, folding that viewer's projection yields exactly the state the server believes that viewer can see.
> `fold(project(log, viewer)) == visibleState(fold(log), viewer)`

Amended 2026-08-22 on Patrik's ruling to run **at every prefix**, not at the end. Rejected: the final-state form as written. Reason: `Explored` is terrain memory — the union of every visible set across history, populated only by `SceneSeen`, which exists only in projections — so the two sides differ on that field *by construction* and no final-state oracle can close the gap. The prefix form is strictly stronger: a leak that appears at one prefix and is covered by the next is invisible to a final-state check. `Explored` leaves the direct comparison and is bounded from above by a **subset** argument: `sceneSeenFor` builds `tiles` only from visible squares, so per message `tiles ⊆ visible`, so nothing can be remembered that was never visible. It is deliberately not pinned from below — a visible square with no terrain is never remembered, because there is nothing to remember.

**The oracle is an independent transcription, not shared code.** Rejected: implementing `visibleState` by calling the projection. Reason: the equation would be a tautology that holds however wrong the projection is. `keystone_test.go` lives in `package gateway_test` and structurally cannot reach `look`, `classify`, `transitions` or `eyes`. Its own comment states the residual hole honestly and measured it: it *does* share `internal/sight`, so a wrong `sight` passes the keystone silently (11 other tests in the package catch it), and a rule mis-transcribed into *both* sides passes too (6 other top-level tests catch it, 5 behaviourally). I verified the first hole directly — see section 5's drift experiment.

**The O(n²) fold shape, chosen deliberately.** `internal/campaign/foldprefix.go` argues with itself at length and lands on keeping it. `FoldPrefix` is `foldEvents` reached from outside the package; `seat.receive` calls it per event, so a log of n events costs O(n²). The comment states the cost rather than hiding it, then states that **the cheap shape is available now and is deliberately not taken**: with retraction gone, nothing is retroactive, so a seat could hold one `engine.State` and advance it with a single `engine.Apply` per event — O(n), in exact agreement. It is refused because that would be a **second event-application loop in the gateway**, which is CLAUDE.md rule 4. Two supporting facts: it is not the dominant term (sight recomputes at 15–176 ms per eye per event against roughly a microsecond per `engine.Apply`), and it is likely moot because sub-project 14 deletes this function's only non-test caller. The comment also records that the *old* reason — retraction made a function-over-a-slice the only possible shape — stopped being true on 2026-08-31, and says so rather than leaving the stale argument standing.

**Nine points, a tolerance, and asymmetry kept rather than fixed.** Patrik, 2026-08-19: *"keep the asymmetry and use the nine points."* Rejected: symmetry (which the spec had asserted). Reasons: one point at the viewer against nine at the target is structurally not a symmetric relation; MapTool has shipped exactly this for two decades; and a symmetric predicate cannot express MapTool's Hill/Pit VBL, which is cover *and* vantage with no coordinate system at all — the thing maps §11.3 said the pillar model could not reach. The counterexample is pinned by test rather than assumed away: *3×3 open floor, one wall at 1,0 — from 0,0 the square 2,1 is NOT visible; from 2,1 the square 0,0 IS.*

**Sight range and tolerance are inputs.** Patrik, 2026-08-18: *"this should not be driven by the engine. It should be input, to the engine."* Rejected: reading a well-known key out of `Actor.Attributes`. Reason: CLAUDE.md rule 5 — no game-system vocabulary in platform code. They are passed as named constants (`sightRangeNotSupplied`, `toleranceNotSupplied`) so the reason travels with the call.

**`Rect` is float64.** Rejected: `int32`, and rejected: an interface with one implementation. Reason: spec §3.5's "squares now, fractional later" — the seam is the *coordinate type*, so a later arc hands narrower rects in and nothing in `sight` changes.

**Exactly one code path introduces an actor.** Rejected: forwarding the raw `ActorAdded` conditionally, e.g. for party members. Reason: two paths collide on every party-member addition, and the cost of a collision is not a glitch — `engine.Apply` refuses a duplicate (`engine: actor %q already exists`), and `Session.ingest` pushes the envelope onto the log *before* folding and never removes it, so every later event re-folds through the same poison. The board stops being true silently for the rest of the session.

**Fail closed on an unrecognised payload.** Rejected: a `default:` that forwards. Reason: `AttackRolled` names a target, `NarrationAdded` may describe a room, `ConditionApplied` names an actor, and a note can say anything. `TestEveryEnvelopePayloadArmHasAnExplicitRuling` walks the descriptor and reds if a new arm lands without a ruling.

**Kind belongs to the actor, and the grant declares it (§5.1).** Three drafts. First: the roster exception keyed on "player-controlled" — the code implemented "has any controller", which is the leak (see section 4). Second: kind fixed on the actor at creation — rejected same day, because *kind is a fact about standing right now*, and a charmed monster becoming a player's to run and back again is a **transition**, which belongs on `ActorControlGranted`. Rejected also: having the compiler stamp every adventure actor as non-party — it would have dropped Hollis Ketch, Mara Voss and the Human Fighter out of every roster the moment they turned a corner, on the only two adventures that exist. *"Infer (wrong), stamp (wrong), or ask (right)."* And **the migration rule was deleted** (`5f0e626`): Patrik, 2026-08-24 — no campaign or ruleset is in use by anyone, so it protected data that does not exist, and the ambiguity it created ("a log written before kind existed" vs "a grant issued today that forgot") was the thing that made the leak reachable. What replaced it is one line: *an absent kind is NOT a party member, always.*

**Perching is not logged.** Patrik: *"we do not need to log anything about what/where the spectator sees."* Rejected: a `set_viewpoint` that appends. Reason: where a spectator points their camera is not a fact about the campaign; it is a view preference like zoom. The cost — a perch does not survive a reconnect — is accepted, and the client re-sends it.

**`perchSequence` is 0.** Rejected: stamping a perch frame with the seat's last folded sequence (which is the rule every *other* synthesized frame follows). Reason, measured while undo still existed: a perch inside an undo range emptied a watcher's board with no message and left the party's next move dangling — `moved unknown token`. The rule generalises past undo: *a derived, per-viewer artifact must not borrow an identifier something else owns.* Sub-project 14 turns this into its own §1.

**Fog is an overlay pass, not a per-op flag.** Rejected: `DrawOp.dim`. Reason: it makes `canvas.ts` — the one layer no test can see — decide what dim means per op kind, and it can be forgotten on one, producing a brightly lit door in a remembered room. `planFog` returns geometry, `shadeFog` fills it, mirroring `planGrid`/`strokeGrid`. Also rejected: RPTool's two fog levels. Reason: unexplored ground is the *absence* of a DrawOp, because the server never sent it; building a heavy fog for it would imply the client holds terrain to conceal.

**`SceneSeen` carries the visible-square set explicitly (field 4).** Patrik's ruling 2026-08-22. Rejected: the client deriving its visible set from the terrain it was sent. Reason: `sight.VisibleFrom` walks the **grid**, not the tile map, so a bare-canvas scene still has visible squares and the server correctly sends the tokens standing on them — then `sceneSeenFor` projected those squares into `Tiles` and dropped every square with no terrain, and the client re-derived a different answer from a lossy proxy and hid the token. *"A token is a FREE OBJECT and needs no terrain to exist."* The token filter stays, but as a consistency check against the server's own set, not an independent decision.

**`perchBox` coalesces: latest wins.** Rejected: a blocking one-slot handoff — measured, a DM's own command times out (>3s) in roughly two runs in three, across three rebuilds. Rejected: a non-blocking FIFO — it clears the stall and keeps every intermediate shoulder, but its depth is a buffer length the client picks. What coalescing stands on is *not* less work: the pump's work is bounded by the pump's speed rather than the sender's.

**A projected seat subscribes from 0 and discards output up to its resume point.** Rejected: constructing a projector at the resume point. Reason, proved rather than asserted: `pr.actors` is genuinely path-dependent — an NPC seen at seq 7 and hidden at seq 8 is still in the roster at 8, while any function of state-at-8 says it is unknown — so such a projector re-introduces the actor and the duplicate `ActorAdded` is the permanent freeze above. `TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed` builds one the forbidden way and *requires* that it corrupts the seat.

**Borrow RPTool's geometry, never its distribution.** A joining MapTool client receives the entire campaign unconditionally, GM layer included, with no per-recipient filtering anywhere in the server; its fog is a client-side render predicate over data the client already holds. The nine points, the tolerance, tokens as free objects and fog as a composited pass all came from there. The distribution model is the exact class of problem this arc exists to close.

---

## 4. Vad som visade sig fel

**The arc's dominant defect class was false prose, not wrong logic.** The project memory written mid-arc says *"twenty false claims, every one in PROSE — comments, reports, spec text — and zero in logic."* The merge commit, written two days later, says **"twenty-five-plus false claims landed here, every one in prose and none in logic, and a fix loop meant to correct four of them ran ten rounds and introduced five more."** The count itself moved. Three consecutive fix rounds whose *entire purpose* was correcting false comments each introduced a new false claim while doing it. The merge message's conclusion: *"A deletion is the only edit that cannot introduce the next false claim."* That sentence is this report's charter.

### Claims later corrected

**The spec asserted symmetry, and the test written for it could not fail.** Spec §3.3.1 today: *"SIGHT IS THEREFORE NOT SYMMETRIC, and this spec previously claimed it was. An earlier draft made symmetry a keystone property — 'if A sees B, B sees A, same ray, same blockers' — which is false under centre-to-many sampling, and the test written for it could not fail because its fixture was an open corridor."* Two of Task 1's five plan-supplied tests were defective: `TestSightIsSymmetric` passed against a **stub**, because two absent map keys agree trivially; and the door test passed for the wrong reason — a wall corner, not the door. Corrected in `8083804` (spec) and `a6ca832` (code and tests).

**"No test in this repo has ever folded a projected seat's stream."** Written into the project memory on 2026-08-30 and into sub-project 14's spec twice — §1 as motivation, §8 as a remedy for a gap already closed. **CORRECTED 2026-09-02.** The narrow finding is true: every whole-session fold in `internal/harness` runs on participant 0, the DM in all nine files under `scenarios/`. The broad claim was false when written: `client/test/projection-parity.test.ts` folds `scenarios/goldens/session-zero/projections/{player,spectator}/state.json`, and `internal/gateway/keystone_test.go` computes `fold(project(log, viewer))` at every prefix of every golden. **Both landed 2026-08-22 in `4109f0d`, eight days before the memory was written.** The spec in the tree now records the whole sequence: *"it took three attempts to write one that holds. Until 2026-09-14 this paragraph said 'no test in this repo has ever folded a projected seat's stream', which was false when it was written. The first correction said 'no SCENARIO', which is false as well."* The reusable half: a true finding about one package was widened into a claim about the repo without enumerating the repo.

**"`load_map` is now the ONLY way a scene comes into existence."** Sub-project 15's Task 8 removed `create_scene` and rewrote every sentence mentioning it — and **replaced one false sentence with another in five newly-written places**. `load_adventure` is still in the contract, still handled, still in the agent's own tool list, and still produces `SceneCreated`; the worst instance was agent-facing prose served by `get_ruleset_guide`, telling the LLM DM a falsehood about a tool it can see. The correct wording was already in the repo, twice, and **one of the two copies is `internal/sight/sight.go`** — *"the map file loader and the adventure loader, which are now the only two"* — which is still the live text at `sight.go:58`. *"'X left, therefore Y is the only one' is a subtraction done without counting what remains. It feels like a correction because it* is *correcting something, which is exactly why it passes self-review."*

**The keystone oracle's own comment told readers to distrust the half that works.** It said *"a bug INSIDE look() would move both sides of the equation together and this test would stay green."* False: two transcriptions of one rule are still two, and four fault injections into `look()`'s body all red the test. The comment now says so, and names the defect: *"THAT REASONING CONFUSED RESEMBLANCE WITH SHARING… The comment was written without running what it asserted."*

**`perchBox` claimed coalescing emits the same frames.** *"This comment claimed the same frames until somebody ran it."* Measured: three rooms, hopping a→b→c one at a time emits 11 frames, going straight to c emits 3, and all 3 are among the 11 — but 4 of the other 8 are rooms passed through, which the queued path leaves on the client's board permanently.

**Two measurements withdrawn, then the withdrawal itself corrected.** `b4cc88e` ("Withdraw two false measurements") and `1f34407` ("Quote the gap, not the percentage, and date the stale count correctly"). The comment had said "12 runs in 12" on a sitting that never reproduced; re-measured at ~64%, and finally replaced by the invariant rather than the rate: *"What survives every sitting is the gap between an arm that fails most of the time and an arm that has never failed once."* The same comment now states plainly *"NO SECOND MACHINE WAS EVER TRIED."*

**The protojson byte figures.** Three commits (`98bf153`, `4ee37b9`, `7b91a33`) over one day. `protojson` deliberately randomises its output — a space after every comma when `detrand.Bool()` is true, seeded from an FNV hash of the running binary. Fourteen binaries differing only in size split seven/seven, exactly 1.00 B per square apart. A round whose whole purpose was correcting unmeasured numbers **withdrew a true number and asserted its format was one "Go does not emit"** — while `internal/mapdef/compile.go` had carried the disagreeing figure since the maps arc.

**A count about counts that arrived stale.** `seat.go`'s `pastResume` comment said mutation adjudications had been re-pointed "four times". The parenthetical that corrects it is the best single artefact the arc produced: *"This line said 'four times' for a while, and that was ALREADY STALE WHEN IT LANDED: `git log -S` puts the sentence at 2fac46e, the branch's 24th commit, and the tally had been five since its 20th… A line that arrives out of date about how often lines go out of date is the argument making itself."* The current text says ten commits.

**`internal/sight`'s guard was justified with a false reason.** Task 1 round 2: the comment said `create_scene` input was unvalidated. False — `create_scene` was validated on every path, and MCP was not a bypass. The guard stayed; only the reason changed, to the one that is actually true: a library cannot see which path produced the `engine.Scene` it was handed, and **log replay genuinely is unchecked**, because `engine.Apply` copies stored objects verbatim. Withdrawn in `c71522a`; §3.5's three-facts split landed in `f633be8` with the title *"Split three facts §3.5 had run together, and both comments then got wrong."*

**The plan's contract field numbers were wrong.** The plan said `token_hidden = 28`, `scene_seen = 29`, after `actor_control_revoked = 27`. The oneof's actual highest field was 29 (`DoorOpened`/`DoorClosed`), so they landed on **30/31**, and `set_viewpoint` — left unnumbered by the brief — took 31. Corrected in `b1b11cc` by verifying rather than trusting.

**The plan's file list was wrong about the client.** It said "Create `client/src/view/visibility.ts` — pure: turns state + explored into what the planner draws." No such file exists. §6.1's amendment of 2026-08-21 replaced it: *"The original §6 said the planner 'takes visible tokens as INPUT' and emits DrawOps 'tagged bright, dimmed, or absent'. Half of that was unbuildable and half was worse than the alternative."* The planner never drew tokens; the seam is `tokensOnScene` in `grid.ts`, and fog became `planFog`/`shadeFog`.

### Defects that escaped

**The roster leak (whole-branch review I1, Important, severity 88).** Spec §5 said the roster exception covers actors *"controlled by any PLAYER"*. `project.go` implemented `len(a.GetControllerIds()) > 0` — **any controller**, over every actor in state, with no visibility test. `controller_ids` holds participant ids and carries no role, and nothing constrains who a grant targets. So one `grant_actor_control` on a hidden monster published it to every player's roster as a whole cloned `Actor` — name, attributes, resources, `module_data`, conditions — while its token stayed correctly hidden, and simultaneously opened `MayPerch` and `eyes()` on it, which §3.1.1 calls *the constraint the whole idea rests on*. Reproduced against a goblin behind a closed door. **The keystone could not catch it:** its oracle transcribed the same predicate, so both sides of the equation agreed while both were wrong. That is the transcription risk the oracle's own comment names, biting in the one place it mattered. It shipped inside the arc and was closed the day after the merge, in sub-project 12 (`d20e9f5`).

**Testimony outlives sight (I2, 85), still open.** Once a player glimpses an NPC, `ResourceChanged`, `ConditionApplied`, `AttackRolled` and `AbilityUsed` naming it keep arriving permanently, in any scene, with no line of sight, because `pr.actors` never forgets. Reproduced: door closed, `TokenHidden` correctly emitted, then `ConditionApplied` on the hidden goblin still delivered. The justification in `project.go` is written **entirely about party members** (*"hearing that the rogue took damage is not the same as being shown where they are standing"*) and then applied to every known actor; §3.2 says the opposite for creatures. Declared out of scope in `docs/superpowers/plans/2026-08-23-actor-kind.md` and recorded in spec §5.1's closing paragraph. **Verified still live in the current tree**: `classify`'s three arms read `pr.actors[...]`, which is only ever emptied when the *world* loses the actor.

**Objects are never fogged (I3, 82), still open.** §6.1 chose the overlay over a per-op `dim` flag because *"the overlay makes that impossible by construction."* It does not — it is keyed to the wrong field. `planFog` keys on `Explored`, which is terrain-keyed (`sceneSeenFor` skips a visible square with no tile), while `planObjects` draws every object in `scene.Objects` unconditionally. So a `blocks_sight` boulder on a tile-less scene stays at full brightness forever, and a multi-square object is fogged only over the fraction of its footprint that was tiled-and-explored. §6.2's own two-different-sources ruling reached the fold and never reached `planFog`. **Verified still live**: `scene-plan.ts`'s `planFog` reads `explored[sq]` only; `planObjects` has no visibility test at all.

**A stale UNRESOLVED block, still in the tree.** `project.go`'s `transitions` doc still declares the torn-batch hazard open — *"UNRESOLVED, AND IT BELONGS TO WHOEVER WIRES THE PUMP (Task 5)"* — and cites `client/src/session.ts:158-160`. It is resolved: `seat.pastResume` filters a projected seat's delivery, `wire.reconnect()` resumes at `seenSeq - 1` and rolls the torn tail out of the log first, and `client/test/session.test.ts` pins the recovery. The whole-branch review found this in August. The citation has also rotted — **`session.ts:158-160` is now inside a presence docstring**; the actual push is `this.log.push(e)` at `session.ts:224`. That comment cost sub-project 12's spec an entire section proposing a fix for a solved problem. It dies with the file.

**The web client could not perch, for three weeks.** The demo gate found it: `viewpoint` appeared exactly once in `client/src`, in a `wire.ts` comment; `setViewpoint` was in no command builder, no control, no URL parameter. Server side built, tested and keystoned; no spectator using the product could reach it. The demo's perch screenshots were honest but the command had to be injected onto the page's own socket before boot. Closed by `feat/perch-ui` (`0200c53`, merged `76e45c2`), which then turned into something else entirely: `check:ts-mutation` had tested **zero mutants since 2026-08-11** behind a green `task check`, for fourteen days.

**A retraction that removed an introduction poisoned a projected seat's fold** — recorded in the merge message as *"design is settled and code is not written."* Sub-project 12 then chased that client freeze through four fix rounds, a contract change and roughly a thousand lines before the cause turned out to be one decision two arcs old: spec §4.2's rule that a synthesized introduction carries its causing *log* sequence. Retraction left the platform entirely in sub-project 13, and §4.2's two paragraphs are kept in the spec marked **AMENDED 2026-08-31**, superseded but verbatim, because the stamping decision they justify is still in force for a plainer reason.

### Measurements that turned out wrong, and one process lesson

Beyond the withdrawn ratios above: the corpus count for the actor-kind guard was first measured at *"8 `actorAdded` events carrying a `controllerId` across 5 golden streams"* and corrected to **12 across 7** — because a shell glob `scenarios/goldens/*/stream.json` does not cross a directory separator, so it never saw `session-zero/projections/player/stream.json` or its spectator twin, *the two nested streams the visibility arc had just added*. Git's pathspec `*` does cross `/`. The measurement that produced the wrong number was itself an instance of the failure the guard exists to prevent.

And: *"THE SIX DEFERRED ITEMS WERE NEVER WRITTEN DOWN AS A LIST. The deferral lives in 8545fca's commit message, which states a COUNT and nothing else… Record the list, not the count."*

---

## 5. Spår

**The range.**

| | |
|---|---|
| Spec | `5793f45` (2026-08-18) *Spec: one log, many views*, plus three same-day amendments before any code: `85a231c`, `52e077c`, `006d714` |
| Plan | `5b644bd` (2026-08-18) *Plan: nine tasks to make a seat see only what it may* |
| Branch | `feat/visibility`, cut off `spec/visibility` (which carried spec and plan); base `5b644bd94163fe49dd3c358385e7b81de3737769` |
| Commit range | `c40a450..44e40ea` — **46 commits**, 2026-08-18 → 2026-08-23 |
| First code | `a7150c2` *The sight test: pure geometry, gated from its first commit* |
| Merge | `35683c9` (2026-08-23) *Merge: one log, many views*; parents `c40a450` (main) and `44e40ea` (branch tip) |
| Immediate follow-on | `d20e9f5` (2026-08-24) *Merge: control is conferred once, by a grant that says what it is* — closes I1 and amends spec §5.1; `76e45c2` (2026-08-25) *Merge feat/perch-ui* — ships the perch control the demo gate found missing |
| Diffstat | 84 files changed, +17,483 / −482 |
| Replaced by | sub-project 14, spec `2026-08-30-per-character-logs-design.md`, plan `2026-09-14-per-character-logs.md`, branch `feat/per-character-logs` (Task 3 uncommitted at the time of writing) |

**How it was established, since there is no mechanical link from plan to commits.** No commit message names the plan, and the merge messages were rewritten so they do not name the branch either. The chain that works from git alone:

1. `git log --oneline --follow -- internal/sight/` → the package's first commit, `a7150c2`.
2. `git log --merges --ancestry-path a7150c2..main | tail -1` → the *first* merge that carried it onto main: `35683c9`.
3. `git rev-parse 35683c9^1 35683c9^2` → `c40a450` (the base, itself the maps-as-geometry merge-gate commit) and `44e40ea` (the branch tip). `git log c40a450..44e40ea` is the arc, 46 commits, and `git merge-base --is-ancestor` confirms both spec and plan commits sit inside it.

Two corroborations. The merge message for `35683c9` describes the nine tasks in the plan's own order. And the gitignored ledger `.superpowers/sdd/2026-08-18-visibility/progress.md` line 3 states it outright — *"Branch: feat/visibility (off spec/visibility, which carries the spec + plan)"* with the base SHA — along with 24 per-task review diffs named `review-<from>..<to>.diff`, which reconstruct the review boundaries exactly. **That ledger is gitignored and does not survive a clone**, which is the argument for this report existing at all: step 1–3 above is the only path that survives, and it works only because the arc happened to create a package of its own.

**The load-bearing check: does anything bind the grid-square key format?**

**No. Nothing binds them.** There is no shared constant, no exported helper the four production copies agree to use, no lint or semgrep rule (`.semgrep/` holds `event-sourcing.yml` and `vocabulary.yml`, neither of which mentions the format), and no test that asserts any two of them agree. The copies at HEAD:

- production Go: `internal/engine/terrain.go:55` (`gridKey`), `internal/mapdef/load.go:227` (`squareKey`), `internal/gateway/project.go:1184` (`squareKey`), `internal/sight/sight.go:382` (`squareKey`, exported to `SquareKey` in the working tree)
- test helpers, each deliberately independent: `internal/gateway/keystone_test.go:335` (`oracleSquareKey`), `internal/gateway/project_test.go:24` (`key`), `internal/gateway/server_visibility_test.go:134` (`gridKeyForTest`), `internal/sight/sight_test.go:553` (`key`), `internal/mapdef/boundary_test.go:43` (`squareKeyForTest`), plus `internal/mapdef/compile_test.go` and three under `cmd/vtt`
- one more in production non-Go: `cmd/vtt/client_soak.go:132`
- TypeScript: `client/src/fold.ts:469` (`doorKey`), with inline `` `${x},${y}` `` in `scene-plan.ts` (×3) and `grid.ts:103`

So it is not four copies plus a test helper — it is **four in Go production code, one more in a Go binary, one exported TS function plus four inline TS sites, and at least seven independent test helpers.**

**What happens if they drift — measured, not reasoned.** I copied the tree to a scratch directory and changed one format at a time to `"%d;%d"`, running the suites. The project tree was not touched.

| Drifted copy | Result |
|---|---|
| `sight.SquareKey` | `internal/sight`'s own suite reds (4 tests: squares that must be visible are not — **fails closed**). `internal/perceive` reds with the **fail-open** direction: `TestQATokenMovedWithheldWhenLineOfSightBlockedByWall` and `…BehindClosedDoor` report `got 2` (Forwarded) where Withheld is required, and `TestClassifyAgreesWithTheProjectionItReplaces` reports `forwarded=true` where the recorded stream says false. **The keystone stays GREEN** — both sides of the equation ask the same broken `sight` and go blind together. What reds instead is `TestTheProjectedGoldensAreWhatTheProjectionActuallySends` (11 subtests) and `TestSessionZeroCannotHappenAgain`. |
| `gateway.squareKey` | Keystone, projected goldens and session-zero all red. `project.go`'s own claim — *"Disagree about the format and NOTHING is ever visible to anyone, which the suite reports immediately and loudly"* — holds, because the oracle keeps its own correct copy. |
| `engine.gridKey` | `internal/engine` reds (5 tests, all door/`Blocked`). `internal/sight` stays **green** — its fixtures build `Tiles` with the test file's own helper, so it is insulated from engine's format entirely. |
| `mapdef.squareKey` | `internal/mapdef` reds broadly (10+ tests). |

The conclusion the tree already half-states: the format is unguarded, but every copy sits under a suite that notices, and the *hand-derived* golden fixtures (`scenarios/goldens/*/projections/*/state.json` — a human wrote 36 squares down from the scene's geometry) are what catch the one case the keystone cannot. The uncommitted comment on `sight.SquareKey` is the first place in the repo that states the two directions correctly and says which one is the leak; it also corrects `project.go`'s copy of the argument, which is scoped to "every visibility answer in this file", true there, and false if generalised. That correction is worth preserving, because `project.go` is about to be deleted and its narrower claim is the one a reader would otherwise carry forward.

---

## 6. Kommentarsblock denna rapport friar

Comment density in the arc's files today: `internal/sight/sight.go` 226/382 lines (59%), `internal/gateway/project.go` 723/1244 (58%), `internal/gateway/seat.go` 311/450 (69%), `internal/gateway/viewpoint.go` 47/71 (66%).

### A. `internal/sight/sight.go` — worth cleaning; the package survives sub-project 14 unchanged

| Lines | What it says |
|---|---|
| 44–66 | Why `Rect` is float64, and the three statements two earlier drafts collapsed in opposite directions (proto type / ingest paths / the fold does not re-validate). |
| 86–90 | Why a degenerate rect is skipped rather than normalised by swapping bounds — the rejected alternative and why. |
| 92–108 | "WHY GUARD AT ALL", including the withdrawn false reason ("an earlier draft of this comment said that and it was false") and the replay argument that replaced it. |
| 184–201 | Sight is deliberately not symmetric: the counterexample, that the spec once claimed the opposite, and the two reasons for keeping rather than fixing (MapTool's 20 years; Hill/Pit VBL). |
| 208–228 | "REDUNDANT as the loop is written, and kept anyway" — the 40,000-scene check, the mutation adjudication, and the falsifiable claim that was written without falsifying it. |
| 242–255 | The early-exit's differential check over 60,000 scenes and the two benchmark rows (43→15 ms sparse, 191→176 ms dense). |
| 356–381 | `SquareKey`'s two drift directions, the 2026-09-17 measurement, and the correction of `project.go`'s narrower claim. *(Currently uncommitted.)* |
| 16–23 | The package doc's provenance for tolerance — the dated Patrik quote and spec reference. |

**Stays** (invariants a reader must obey to change the line correctly): 1–15 and 24 (the package contract: `rangeSquares <= 0` unlimited, `tolerance <= 0` behaves as 1); **36–42** (`MinX <= MaxX` — an inverted rect makes `containsPoint` unsatisfiable and *blinds* its occupant); **69–85** (what blocks, rotation ignored, `Width < 1` skipped to match `covers()`); **110–114** (only squares inside the declared grid, and why — output order and out-of-grid tiles); **149–155** (`Clear`'s open endpoints); **175–182** (tolerance semantics, including that >9 is not clamped); **272–285** (`samplesPerSquare` as a count tolerance is read against, and the epsilon inset — "load-bearing rather than tidy"); **323–324** (the slab test). One line of 184–201 should survive as a pointer: *do not "fix" this into symmetry; spec §7 pins the counterexample.*

### B. `internal/gateway/viewpoint.go` — worth cleaning; the file survives (generalised by sub-project 14 Task 11)

| Lines | What it says |
|---|---|
| 21–31 | That `MayPerch` used to test "has any controller", that a DM granting themselves the Goblin Archer made it perchable, and that Patrik's §5.1 ruling closed it. |

**Stays**: 10–19 (only a party member; `Projector.eyes` refuses the same perch a second time), 32–38 (only a spectator, and why the arm exists even though `commandRoles` already denies it), 40–47 (the two refusals are one string — a watcher could otherwise enumerate the DM's cast by perching on guesses).

### C. FLAGGED — blocks in files sub-project 14 **deletes**. Cleaning these is wasted work.

Plan Task 12 deletes `internal/gateway/project.go`, `project_test.go`, `project_internal_test.go`, `project_property_test.go` and `keystone_test.go` outright. Task 10 removes `seat.pr`, `pastResume`, `catchUp`'s projection branch and the subscribe-from-zero rule from `seat.go`; `perchBox` and `seat.perch` survive in altered form.

`project.go` — roughly 400 of its 723 comment lines are narration this report now holds:

- **41–89** — what the five maps are and why they are not a cache; the two tests; the consequence for whoever wires the pump.
- **127–140** — why range and tolerance are passed as literals rather than read off the actor.
- **197–224** — `perchSequence = 0`: the measured `moved unknown token` collision and why zero outlives undo.
- **227–255** — `reperch` is `transitions` with no cause; "sat on means sat on" and what `perchBox` coalescing costs.
- **284–292** — no memo, what a sound memo key would be, the 15/176 ms measurement.
- **337–350** — the §5.1 history on the party-member arm.
- **391–447** — the shared-sequence consequence, the undo asymmetry that left with retraction, **and the stale UNRESOLVED torn-batch block at 409–430 with its rotted `session.ts:158-160` citation**.
- **451–508** — the obituary for the forgetting loop that left with retraction, plus why nothing replaced it with a loud guard.
- **515–521, 545–573, 586–612, 636–680** — the redaction rationale; why an introduction cannot carry a controller and grants ride behind it; why conditions ride along; why the `SceneSeen` walk is a union and what an empty one means.
- **690–709** — why `visible` is sorted and why the tiles map is stable (protojson sorts; that is the encoder's guarantee, not the type's).
- **740–755** — `objectInSight`'s clamp and int64 comparison.
- **826–837, 849–860, 868–886, 895–913, 961–998, 1012–1044** — the per-arm rulings: narration forwarded and notes withheld and why they differ; one path introduces an actor; `TokenRemoved` withheld because departure is already narrated; the strict-arms block on double application; `ActorRemoved` as the one arm whose forwarding is the only way the fact can arrive.
- **1058–1088** — why `doorTransitions` must exist, with its own 2026-08-21 correction.
- **1175–1184** — `squareKey`'s "fourth copy" argument (superseded by `sight.SquareKey`'s, section 5).
- **1186–1191** — why the three sorters exist.

`seat.go`:

- **14–29** (`viewerFor`: why `Viewpoint` opens empty), **34–52** (`projected()`: "IT IS NOT THE SECURITY BOUNDARY", measured by making the change and watching the suite stay green), **63–89** (why the projector is fed from the beginning; the mutex that was the wrong tool), **100–120** (the `received` slice's obituary — load-bearing while retraction existed, kept now only for rule 4), **308–335** (`pastResume`: no fast path, the ten re-keys, the self-correcting "four times" parenthetical, and why a perch skips the filter), **348–367** (`perch` against the last folded world, not head), **389–412** (`catchUp`: why a seat's head cannot be the log's).
- **132–194** (`perchBox`) sits on the boundary: the mechanism survives sub-project 14, so its comment is worth cleaning — and it is the single largest measurement narrative in the arc (the blocking/FIFO benches, the withdrawn "12 runs in 12", "NO SECOND MACHINE WAS EVER TRIED").

### D. Not listed, because they belong elsewhere

`contract/vtt/v1/events.proto` and `commands.proto` carry the projection-only rationale for `TokenHidden`/`SceneSeen`/`SetViewpoint`, including an in-place record of the 2026-08-31 retraction amendment. Contract comments replicate verbatim into generated files in both languages; they are a published interface, not history, and are out of this report's scope.

---

## Where I could not establish something

- **The branch's push/PR history.** The reflog does not reach back to 2026-08, and no PR number appears in any of the arc's merge messages (unlike the July/early-August work, which merged through numbered GitHub PRs). I can state the branch names only from the gitignored ledger; git alone gives the commit range but not the names.
- **Whether anything downstream depends on `Scene.Visible`/`Explored` being nil rather than empty for the DM.** The keystone asserts it and `state.ts` says a renderer conflating them would blank the DM's board, but I did not trace every consumer.
- **The exact disposition of I2 and I3 after sub-project 14.** Both are confirmed live in the current tree and both are scoped out of the actor-kind plan. `project.go` dies with I2's comment inside it, so whether the *rule* is ported into `internal/perceive` correctly or ported forward as a defect is a question for that arc's review; `perceive.go`'s own header flags the four arms its oracle cannot check, which is the neighbouring risk.
