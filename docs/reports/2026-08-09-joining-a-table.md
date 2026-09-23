# Implementation report — joining a table

**Arc:** `docs/superpowers/plans/2026-08-09-joining-a-table.md`
**Spec:** `docs/superpowers/specs/2026-08-08-joining-a-table-design.md`
**Territory:** `/Users/patriklager/dev/vtt-platform/internal/identity/identity.go` (898 lines: 400 Go-comment lines, 445 code lines, 54 blank; 16 comment blocks of ten lines or more)
**Verified against the tree at** `48e3fe2`, 2026-09-19. Nothing was changed; every injection below ran against a `git archive HEAD` copy in a scratch directory.

---

## 1. Vad som byggdes

The DM shares ONE link instead of minting one private invite per person. A campaign gained operational state — a join secret, an open/closed door, and (eleven days later) an admission budget — in a new `join_access` table inside `internal/identity`'s SQLite, deliberately outside the event log so that replaying a campaign cannot reopen a door. An unauthenticated `POST /join` carrying that secret and a display name mints the caller their own participant at role `spectator` through the existing `CreateInvite` path and hands back a token; the DM then promotes them with a `promote_participant` command that updates `participants.role` and appends nothing. **Invites and revocation were not built here** — `CreateInvite`, `Verify`, `Revoke`, `ErrInvalidToken` and the `participants` table all landed on 2026-07-24 in `5d9b51b`, two weeks earlier. What this arc did to them is subtler and larger: it reused `CreateInvite` byte-for-byte so a joiner who reconnects keeps their participant id and their characters, and it made `Revoke` actually do something — until J4 the only `Verify` in the WebSocket path ran once before the upgrade, so a revoked participant kept playing until they chose to disconnect.

## 2. Hur det fungerar i dag

**The door.** `join_access` is a single row pinned by `CHECK (id = 1)`, carrying `secret`, `open`, `admitted` and `admit_limit`. It is a separate TABLE rather than columns on `participants` for one reason that is still load-bearing: `Open()` applies the schema with `CREATE TABLE IF NOT EXISTS`, which is a no-op on a table that already exists, so a new COLUMN would never reach an existing campaign. `admitted`/`admit_limit` are the exception that proves it — they *are* columns, and they needed the package's first real migration (`migrate` → `migrationPending` → `migrateLocked`), which reads first, takes no write lock when there is nothing to do, and wraps the work in `BEGIN IMMEDIATE` on one pinned connection when there is.

**The gate a stranger drives.** `internal/gateway/join.go`'s `handleJoin` is registered at `mux.HandleFunc("POST /join", ...)` (server.go:447) and is the only unauthenticated write path. It caps the body at 4 KiB, validates the display name first and distinctly (`usableDisplayName`: ≤64 *runes*, no control characters, no bidi controls, at least one rune that is not default-ignorable/`Cf`/whitespace), then asks `identity.JoinAdmits(secret)` — one call, one answer. A shut door, a wrong secret and a spent budget are refused with byte-identical 403s.

`JoinAdmits` does one `SELECT` of all four columns, compares the secret in Go with `subtle.ConstantTimeCompare`, decides every refusal there in Go, and only on an expected admission issues `UPDATE join_access SET admitted = admitted + 1 WHERE id = 1 AND open = 1 AND admitted < admit_limit` — whose `WHERE` re-states the whole condition, so racers are serialised by SQLite and the loser matches no row.

**A stale function is still in the file.** `JoinAllows` (identity.go:486) has **no production caller** at HEAD. `grep -rn "JoinAllows"` finds the definition, four call sites in `identity_test.go`, two explanatory comments in `internal/gateway`, and two lines of spec. `handleJoin` calls `JoinAdmits`. Its 22-line doc comment (464–485) argues a live security property for a function that is no longer live.

**Promotion.** `promote_participant` is a `ClientCommand` with an entry in `convert_test.go`'s `notConverted` allowlist. `commandRoles` gates it to dm/agent with no player and no spectator row; `authorizePromotionTarget` bounds the target role to `player` or `spectator` for *every* role including the DM's own; and `handlePromotion` (server.go:1674) additionally refuses to promote anyone whose *current* role is dm or agent — a check that cannot live in `Authorize`, because `Authorize` does no I/O. `identity.SetRole` then changes only `participants.role`, reports `n == 0` as "no participant", and leaves a revoked participant revoked.

**Live authorization.** `identity.Lookup` is the live half. The gateway re-resolves through it in three places: per command in the read loop (server.go:980), per delivered event (`credentialGone`, server.go:1713), and per presence frame (`revoked()`, server.go:1491) — the last resolved *outside* the registry lock, because a `Lookup` inside `broadcast`'s loop would put one SQLite read per connection under the global mutex. Only `ErrInvalidToken` ends a connection; every other error refuses the command and keeps the socket.

**Reach.** `vtt join-link show|open|close|rotate` (with `--admit`), `vtt invite`, `vtt revoke`; `GET /api/join-link`, `GET /api/participants`; a client join view (`client/src/join.ts`, `client/src/view/join.ts`); and — closing a carry-forward the arc recorded as open — MCP tools `get_join_link` and `get_participants` in `internal/mcp/door_tools.go`.

**Gates.** `internal/identity` is first in `check-mutation.py`'s `PACKAGES`. Measured coverage today: **93.7 %** against a floor of 92.1.

**No ADR governs this arc.** `docs/adr/` holds 001–010, all from July 2026, none about identity, authorization or joining. Every rule this arc established lives in the spec and in the comment blocks listed in §6 — which is precisely the gap the report programme exists to close.

## 3. Besluten och varför

**Refusals are decided in Go, from the same read — and this is the whole security argument.**
Spec §2 declines rate limiting, and it declines it on a specific claim: while the door is shut there is no endpoint to hammer, so a shut door leaves the link *inert*. "Inert" is a claim about WRITES, and `internal/identity` shares its SQLite file with `internal/store`, which writes inside a transaction on every event append. So *any* write on the refusal path — even an `UPDATE` matching zero rows — takes the campaign file's write lock, inside a transaction, on the one path an anonymous stranger controls. That is not a tidiness concern; it is the difference between "no endpoint to abuse" and "an endpoint that contends with every event the table appends."

The rejected alternative was to let SQL decide. Comparing the secret in the `WHERE` clause is the obvious shape, and it was refused twice over: SQLite's string comparison is not constant-time, and the secret would have to leave `internal/identity` to be compared anywhere else. So the secret is compared in Go, in constant time, in the package that stores it; the door, the secret and the budget come from ONE `SELECT`; and every refusal returns before the `UPDATE` is reached. The accepted cost is explicit: `JoinAdmits` does *not* re-check the secret in the `UPDATE`, so a `RotateJoinSecret` landing between the `SELECT` and the `UPDATE` lets one in-flight request through on the old secret. A microsecond window, on a rare authorized action, admitting somebody who held the real secret a moment earlier — against a timing side channel on the path a prober actually drives.

**Roles stay in `participants.role`, not in the event log.** `internal/engine`'s `state.go` and `apply.go` contain zero references to `Role`; the fold knows nothing about authorization. Putting a role in the log would drag an identity concern into `engine.State` *and* create a second source of truth for authorization. The rejected alternative was an event, and the argument against it was on the record before it was made: `controller_id` mirroring `controller_ids` had just cost a documented invariant, fault-injection proof on both folds, and a golden scenario before it could be trusted.

**Promotion is a `ClientCommand`, not an HTTP endpoint.** This was genuinely in tension: a `ClientCommand` is structurally *a thing that becomes an event*, and `TestEveryClientCommandConverts` enforces that, while a role change deliberately produces no event. The rejected alternative — an authenticated endpoint beside `/join` — is more honest about what promotion *is*, but it sidesteps `commandRoles`. One authorization surface beats two: §5's matrix is where every "who may do what" answer already lives, and splitting it is how a cell goes missing. The price was an entry in `convert_test.go`'s `notConverted` allowlist, each entry carrying its stated reason.

**Re-resolve per command and per delivery; never drop the socket.** The plan's original T4 said to close the promoted participant's connections so they reconnect. Patrik rejected it — *"What role a participant has and what connection it has should be fully separated"* — and the rejection is the better design for a reason the plan then wrote down: caching "what may this person do" for the life of a socket *is* the defect; a reconnect is not a fix for it, it is a way of paying for it. Two things settle it. Every joiner arrives as a spectator, so reconnect-to-promote would sit on the critical path of everyone who ever joins, making the shared link more cumbersome than the invites it replaces. And socket-dropping cannot cover revocation at all: `vtt revoke` is a *separate process* against the same SQLite file, so there is no in-process event for the gateway to hook.

**A COUNT, not a time window, and no upper bound.** Both bound the blast radius of a leaked link. Patrik's call (2026-08-11): a DM knows how many people they are expecting more reliably than how long those people will take to arrive, and a window that expires mid-arrival is a failure the DM has to diagnose from the other end of a chat. Opening RESETS the count, because carrying it over would let a campaign exhaust its admissions permanently, curable only by editing the database. `DefaultAdmitLimit` is **8**, a number rather than "unlimited" and rather than zero, because protojson omits zero values — an absent `admit_limit` and a deliberate 0 arrive as identical bytes, and of the two readings "admit nobody" is the one nobody can debug from either end. There is no ceiling on the budget: setting it is already an authorized act, so a ceiling would bound only a typo.

**Three refusals, one answer.** A shut door, a wrong secret and a spent budget all return the same status and the same body. The accepted cost is stated rather than hidden: a legitimate Nth+1 player cannot tell why they were turned away and has to ask the DM. The alternative tells a prober which half of their guess was right — whether this campaign exists and is merely shut.

## 4. Vad som visade sig fel

**The closed door was not inert.** Spec §2, as written: *"With the door shut the link is inert, so a leak is harmless outside the window and there is no standing endpoint to hammer."* The first implementation answered a guess through `JoinSecret`, which MINTS the row on a campaign that has never had one — so a refused, anonymous, unauthenticated request performed an INSERT, taking the write lock on the file `internal/store` appends to. Corrected at the J2/J4 review (`6056d20`, 2026-08-09): *"The argument above was sound; the code did not implement it."*

**"A leak is harmless outside the window" was false retrospectively.** The 2026-08-11 amendment: *"`SetJoinOpen(false)` flips a flag and `RotateJoinSecret` replaces a string. NEITHER revokes anything already minted. So 'a leak is harmless outside the window' is true PROSPECTIVELY and false RETROSPECTIVELY."* Every credential minted during an open window is permanent and comes back only through `vtt revoke`, one participant at a time, against a list the DM would first have to notice. This is what produced the admission budget.

**A door opened with an explicit limit of 0 admitted nobody.** Found by the mutation gate (`4b0b8d4`, 2026-08-11): `admitLimit <= 0` → `< 0` survived inside `internal/identity`, because the only test for the coercion lived in `internal/gateway` and *mutation runs per package*. **Re-verified today: killed.** Injecting `admitLimit < 0` fails `TestADoorOpenedWithNoStatedBudgetStillAdmits` and nothing else.

**A refused anonymous request reached for the write lock.** The same run: `admitted >= budget` → `> budget` gave the caller the identical answer (the `UPDATE`'s own `WHERE` still refused) while reaching the write. Nothing could observe it until `internal/testdb` existed, because the observable is not the answer — it is whether a statement was attempted. **Re-verified today: killed.** Injecting `>` fails `TestASpentBudgetRefusesWithoutTouchingTheDatabase`.

**The shut-door refusal still has no fault-injection proof. The reader is right.** `TestASpentBudgetRefusesWithoutTouchingTheDatabase` and `TestAWrongSecretRefusesWithoutTouchingTheDatabase` (both in `fault_internal_test.go`) arm a fault on `"SET admitted = admitted + 1"` and fail if it is reached. The shut-door case is `TestAClosedDoorSpendsNothing` (`identity_test.go:1235`), which reads the counter afterwards but arms nothing — it proves the increment did not *take effect*, not that the `UPDATE` was never *attempted*. Measured: removing `open != 1` from `JoinAdmits`'s Go guard leaves `go test ./internal/identity/` green **and `go test ./...` green across the whole repository**. Same mutant shape as the one the gate caught for the budget, on the same line, on the sibling term — and the operator-based gate cannot express a term deletion from a `||` chain. This is an open gap, not a historical one.

**A measurement in a live comment is now wrong in the safe direction.** The `join_access` schema comment says *"flipping the DEFAULT fails nothing, flipping the INSERT fails two tests"*, and four lines later *"flipping ensureJoinRow's inserted literal fails `TestReadingTheLinkDoesNotOpenTheDoor`"* — one test named where the same block says two. Measured at HEAD: flipping `DEFAULT 0` to `1` fails nothing (claim holds). Flipping `ensureJoinRow`'s inserted literal fails **three**: `TestJoinIsClosedOnAnExistingCampaign`, `TestReadingTheLinkDoesNotOpenTheDoor`, `TestAnAlreadyMigratedReadOnlyCampaignStillOpens`. The block states two numbers for the same injection and neither is current.

**15.5 µs is quoted in four places and no instrument survives.** `identity.go:802`, `server.go:974`, the plan and the spec all carry it. There is **no benchmark anywhere in the repo** — `grep -rn "func Benchmark" --include="*.go"` returns nothing. I wrote one in the scratch copy (40 participants, `Lookup` by id, Apple M2) and measured **11 432 ns/op**. The magnitude reproduces and the argument ("microseconds against milliseconds") stands; the specific figure cannot be reproduced from the tree by anyone, and the number has no owner.

**`retract_events` stopped being promotion's neighbour.** Spec §3.1a originally placed `promote_participant`'s allowlist entry *"beside `use_ability`, `load_adventure` and `retract_events`"*. Corrected in the spec 2026-08-31. Verified: `notConverted` today holds nine entries — `use_ability`, `load_adventure`, `load_map`, `remove_actor`, `promote_participant`, `set_join_door`, `rotate_join_link`, `set_viewpoint` — with `retract_events` gone. The spec's correction is accurate.

**`participants.controls` was a second record of control that granted nothing.** Removed 2026-08-24 (`18b7212`): no updater ever existed, nothing turned it into an `ActorControlGranted`, and its only consumer echoed it at `/api/me` — *"a DM who invited somebody 'controlling Hollis' was told by `/api/me` that they did, while every rule that decides anything said they did not."* The migration built to drop it was itself deleted the same day (`63f2f4d`), because no campaign is in use by anyone, so it protected nothing while charging every existing campaign one writable open. This is not this arc's work, but it rewrote two of this arc's comment blocks and nearly took a joining-a-table test with it: `TestAReadOnlyCampaignStillCarryingTheControlColumnWillNotOpen` was the sole witness for `migrateLocked`'s "budget an already-open door" arm, re-pinned deliberately by `TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign`.

**An authorization hole nobody had named.** `promote_participant` bounded what a promotion may promote TO and said nothing about who may be promoted FROM, so `promote_participant(dm_id, "spectator")` named a permitted role and went through — and agents are authorized to promote, so one agent having a bad day could lock every human out of their own campaign. Found in review of J6, fixed in `handlePromotion`.

**A test pointed at the wrong function and a guard was deleted with every suite green.** `identity_test.go:1286` records it: review deleted the `secret == ""` guard from `JoinAdmits` and everything stayed green, because the existing empty-secret test exercised `JoinAllows`, which `handleJoin` no longer uses. Re-verified: the guard is pinned now — removing `secret == ""` fails `TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath`. The reason the trap existed — a superseded function still compiling and still answering — is unchanged.

**A citation that resolves nowhere.** `Verify`'s doc comment cites *"task-3-brief.md Step 2, binding"*. That file exists only under `.superpowers/`, which is gitignored, and there are now **seven** files with that name, one per sub-project. No clone has it, and no reader with the directory can tell which one is meant.

**One claim I had to reinterpret to check.** The plan's *"`commandRoles` grows 60 → 64 literal cells"* does not count literals in the file — the matrix has never written `false`. It counts rows × 4 roles. Verified at `114b166`: 15 rows before, 16 after. Today the matrix has 22 rows.

**Verified TRUE, against the instinct to assume otherwise.** `migrateLocked`'s comment claims its `columnNames` error arm has never been reached and cannot be through `testdb`, *"measured, not assumed — this arm read `1 0` in the coverage profile."* Re-measured at HEAD: `internal/identity/identity.go:269.16,271.3 1 0`. Still exactly true, thirteen months of commits later.

## 5. Spår

**The arc: `a9613a8..98d1784`, merged as `df7169a`, 2026-08-10.**

How it was established. `git log --oneline --follow -- internal/identity/` gives the package's first commit as `5d9b51b` (2026-07-24) — which is **not** this arc; it is the original invite/revocation work two weeks earlier. The arc's first commit in the package is `12aa38b` ("J1: the join door, closed by default and provably so", 2026-08-09). `git log --merges --oneline --ancestry-path 12aa38b..main | tail -1` gives `df7169a`, whose subject names the PR and the branch — *"Merge pull request #28 from PatrikLager/feat/joining-a-table"* — matching the branch the plan declares. `git rev-parse df7169a^1 df7169a^2` gives `a9613a8` (main side) and `98d1784` (branch tip).

Ten commits, in order: `9b732ab` (spec and plan), `12aa38b` (J1, the door), `db53c4d` (J2, `POST /join`), `114b166` (J3, promotion), `c28cf65` (J4, re-resolution), `6056d20` (J2/J4 review), `90efb19` (J5, client), `fb62da7` (J6, the seams), `8b510c6` (whole-branch must-fix), `98d1784` (whole-branch should-fix).

**Three follow-ups outside that range belong to the arc**, each located the same way (`git log --merges --ancestry-path <commit>..main | tail -1`):

- `ea60267` — PR #28, `fix/a-revoked-participant-is-told-why`, 2026-08-10, commits `02c824b` and `9c21519`. Gateway only; does not touch `internal/identity`.
- `c2e4365` — PR #32, `fix/bound-what-an-open-door-can-mint`, 2026-08-11, commits `0daa81d` and `4b0b8d4`. The admission budget, the package's first migration, and `internal/testdb`. This is the spec's 2026-08-11 amendment.
- `d20e9f5` — 2026-08-24, commits `18b7212` and `63f2f4d`. A different arc (actor-kind), but it rewrote this package's schema comment and `migrate()`.

`.superpowers/` is gitignored and was not used as a source. Sources actually read: the plan, the spec, `docs/adr/` (001–010; none governs this arc), `docs/verification-debt.md`, `tools/mutation-scope.md`, `tools/check-mutation.py`, `tools/coverage-thresholds.txt`, the project memory at `~/.claude/projects/-Users-patriklager-dev-vtt-platform/memory/`, and the commit messages of all fourteen commits above.

## 6. Kommentarsblock denna rapport friar

All in `/Users/patriklager/dev/vtt-platform/internal/identity/identity.go`. For each: what the block narrates (this report now carries it), and **what must stay**, because a previous reader was right that not one block is pure history.

**22–45 — why there is no `controls` column, and why nothing migrates it away.**
STAYS: There is no `controls` column and no statement in this package names one; what a participant controls is `Actor.controller_ids` in the log, read by `gateway/authz.go`'s `controls()` and `eyes()`. Identity is deliberately not event-sourced, which is why control belongs in the log and not here. A campaign file still carrying the column must still open — the column is inert. Do not add a migration to drop it: the shape check makes `migrationPending` answer yes for every such campaign, which takes `migrate` to `BEGIN IMMEDIATE`, which read-only media cannot give.

**89–108 — what `migrate` is for and what it must not grow into.**
STAYS: `migrate` runs on every `Open` and must stay idempotent — `ALTER TABLE ADD COLUMN` is an error, not a no-op, on a column already present. The admission budget must live on the same single row as `open`, because spending an admission has to be ONE conditional `UPDATE` against ONE row to be race-proof. It touches only `join_access`.

**128–142 — why the migration takes `BEGIN IMMEDIATE` on one pinned connection.**
STAYS: the scan and the `ALTER`s are separate statements, so two processes opening the same campaign at the same instant both see the columns missing. `BEGIN IMMEDIATE`, never `db.Begin()`: a DEFERRED transaction takes a read lock and must upgrade it, and `busy_timeout` does not retry a lock upgrade. (The trial counts 35/40, 36, 0/40 are this report's; I could not cheaply re-run them and record them as reported, not reproduced.)

**181–194 — what `tableShape` is, and why the PRAGMA is a literal.**
STAYS: the pragma and the table's label travel as ONE value, so a call site cannot pass a pair that disagrees and mislabel the one message an operator gets. SQLite will not accept a bind parameter in a PRAGMA, so it must stay a literal rather than be interpolated from the name.

**202–217 — one `columnNames` for two call sites, and why the rows handle is closed by `defer`.**
STAYS: `migrateLocked` must close the rows handle BEFORE its `ALTER TABLE` — SQLite will not alter a table with an open cursor on it. `shapeReader` must stay the narrowest interface that serves both `*sql.DB` (before the lock) and `*sql.Conn` (under it). Both call sites must name the table identically.

**252–267 — re-read the shape under the lock, and a known coverage hole.**
STAYS: re-read inside the transaction; a concurrent opener may have completed the migration while this one waited. Wrap `"identity: migrate:"` here, which `migrationPending`'s read does not, or an operator cannot tell which of the two identical reads failed. The error arm is UNCOVERED and cannot be covered through `testdb`: `Arm` is one-shot and matches by substring, and `migrationPending` runs the identical PRAGMA first. **Verified still true at HEAD** (`identity.go:269.16,271.3 1 0`).

**286–296 — repair an open door that arrives without a budget.**
STAYS: the repair must be keyed on the STATE (`open = 1 AND admit_limit = 0`), never on which `ALTER` just ran — a database carrying one column and not the other would otherwise open clean, keep a budget of 0, and refuse every joiner at a door reading "open". The predicate cannot match a legitimate row, because `SetJoinOpen` coerces every budget to at least 1.

**345–358 — why `driverName` is a variable.**
STAYS: `driverName` is `"sqlite"` in production and exists only as a test seam for `internal/testdb`; it is set only by `fault_internal_test.go` and restored by it. Closing a handle is not a substitute — it fails the FIRST statement, leaving every later error arm unreached. This handle is independent of `store.Open` on the same campaign file, and both may be open at once.

**395–408 — `SetJoinOpen` resets the budget and coerces a non-positive limit.**
STAYS: `admitLimit` is ignored when closing. Opening RESETS the count. `admitted` resets on EVERY call including a close, or the next opening's budget arrives already spent. A non-positive `admitLimit` becomes `DefaultAdmitLimit`, never "admit nobody" — protojson omits zero values, so an absent field and a deliberate 0 are the same bytes on the wire.

**464–485 — `JoinAllows` never writes.**
This is the block to look hardest at. **`JoinAllows` has no production caller**; `handleJoin` uses `JoinAdmits`. The 22 lines argue a live security property about a function that is no longer live, and `join_test.go:397` already warns that it "still exists, still compiles, and still answers the same question WITHOUT spending anything". What STAYS is not in this block at all — the rule belongs to `JoinAdmits` (513–543). If the function is kept for its tests, one line saying so, and saying that `handleJoin` does not call it, is what this comment should shrink to. Whether it is kept is Patrik's call, not this report's.

**513–543 — `JoinAdmits`: atomicity, and why every refusal is decided in Go.**
Almost entirely rule, not history; this report frees only the spec-amendment dating around it. STAYS, all of it: the secret is compared HERE, in Go, in constant time, and never leaves this package. REFUSALS are decided here, from the same read — wrong secret, shut door, budget spent — so a refused anonymous request writes NOTHING, because an `UPDATE` matching zero rows still takes SQLite's write lock on the file `internal/store` appends events to. The INCREMENT re-states the door and the budget in its `WHERE`; the read above is a fast path only. It does NOT re-check the secret, so a rotation between the `SELECT` and the `UPDATE` admits one in-flight request on the old secret — accepted deliberately. A `CreateInvite` failure after a `true` return BURNS the slot; do not add a compensating decrement.

**586–601 — `RotateJoinSecret`'s upsert: what it touches and what it must not.**
STAYS: the `DO UPDATE` branch must NOT touch `open` — rotating closes the link to holders of the old secret and says nothing about whether the door is open. The `INSERT` branch writes `open = 0`, and that is not an exception: reaching it means no row existed, and no row already means closed. `admitted` RESETS, or rotating a leaked link after its budget ran out hands the DM a door that reads open and admits nobody. `admit_limit` is NOT touched, or rotating becomes a second way to set a budget.

**662–677 — `SetRole` is the one source of truth for a role.**
STAYS: role lives in `participants.role` beside the token, never in the log — the fold contains no reference to `Role`, so an event would drag an identity concern into `engine.State` and create a second place authorization lives. It changes ONLY the role: token, id and display name belong to the person and survive, or a promotion would log them out. The characters they hold survive because they live in the log, which `SetRole` cannot reach. A revoked participant stays revoked; promotion is not a way back in.

**740–749 — why `Verify`'s hash lookup is a plain indexed equality.**
STAYS: the `WHERE token_hash = ?` equality is safe although SQLite's comparison is not constant-time, because the SHA-256 hash is not secret in a timing-sensitive sense; the confirmation step still wraps the comparison in `subtle.ConstantTimeCompare` to make the contract explicit. GOES with the history — and should go regardless: the `task-3-brief.md` citation resolves only inside gitignored `.superpowers/`, where seven files now carry that name.

**784–804 — `Lookup` is the live half.**
STAYS: `Lookup` resolves a participant as they are NOW. Authentication is a connection-time fact; authorization is a live one, so the gateway re-resolves through here rather than trusting the answer it got at connect. A revoked participant does not resolve, and revoked and unknown share one error — the same posture `Verify` takes. The 15.5 µs measurement should go with the history: no benchmark in the tree produces it (re-measured at ~11.4 µs/op on an ad-hoc bench; the magnitude holds, the figure has no owner).

**833–850 — `List` answers the console's question.**
STAYS: REVOKED PARTICIPANTS ARE OMITTED — they cannot connect and cannot act, and listing them would offer the DM promote controls for people who are gone. Ordering is done in SQL, `display_name` then `id`, so the order is total and two consumers cannot disagree about it. A role must NOT be folded into a presence frame: presence is connection-scoped, a role is campaign-scoped, and live re-resolution is exactly what makes a folded role go stale.

**Not one of the 16, but in the territory and stale: lines 55–73**, the SQL comment inside the `schema` string literal (19 `--` lines; it falls outside the 16 because a Go-comment count treats it as code). It carries the migration story that still matters — `CREATE TABLE IF NOT EXISTS` is a no-op on an existing table, so `join_access` had to be a new TABLE; `id = 1` plus the `CHECK` makes it a single row by construction; the closed-by-default property is carried by `ensureJoinRow`'s explicit `open=0`, not by the column DEFAULT. Its injection numbers are the ones §4 corrects: it says "two tests" in one sentence and names one test in the next, and the measured answer today is three.

---

### One open item, stated plainly

The shut-door refusal has no fault-injection test. Removing `open != 1` from `JoinAdmits`'s Go guard leaves the entire Go suite green, while the two sibling terms on the same line are each pinned by a `testdb` test that arms the `UPDATE` and requires it not to fire. The missing test is the third of a set of three, and the recipe for it is the two siblings with `SetJoinOpen(false, …)` in place of their setup. It belongs in `docs/verification-debt.md` under `test asserts nothing` — the fixture reaches the state, and the assertion cannot fail.

### Protocol note

Per `~/.claude/CLAUDE.md` I invoked `/dev-cycle`. Phase 1 (ground in source; verify by command) and Phase 4b's rule (verify claims BY COMMAND, not by re-reading the code a sentence describes) are what this task is made of and were followed throughout. Phases 2, 3 and 5 have no object here: the work is read-only, produces no diff and lands nothing in git history.
