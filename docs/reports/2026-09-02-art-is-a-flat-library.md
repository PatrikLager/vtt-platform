# Implementation report — sub-project 16: art is a flat library

**Plan:** `docs/superpowers/plans/2026-09-02-art-is-a-flat-library.md`
**Spec:** `docs/superpowers/specs/2026-09-02-art-is-a-flat-library-design.md`
**Merge:** `b674abb`, 2026-09-08
**Written:** 2026-09-19, against the tree at `48e3fe2` (branch `feat/per-character-logs`, whose uncommitted work was not touched)

Every claim below was checked by running a command. Where a comment's claim turned out false, it is in section 4, not section 2. Where I could not establish something, I say so.

---

## 1. Vad som byggdes

The arc deleted the pack — a directory that owned a set of art and lent it to maps that declared its name — and replaced it with one flat `campaign/art/` directory in which the filename stem *is* the art id. A new self-contained package, `internal/artlib`, resolves an id to a `Piece` by reading `<dir>/<id>.json` and `<dir>/<id>.png`; nothing declares an id, nothing registers, and uniqueness is the filesystem's, because two files cannot share a name in one directory. `mapdef.Resolve` stopped taking a `*Pack` and started taking an art directory, and stopped refusing a map whose art it cannot find — it drops that one square to the kind and material `m.Tiles` already gave it and warns. The warning needed somewhere to go, so `CommandResult.warnings` was added: the arc's one contract change. Along the way the pack route `GET /api/packs/{pack}/{file}` was deleted and `GET /api/art/{file}` built in its place, `vtt art install` was written, `cell_px` was rehomed out of the pack into `campaign/campaign.json` (and later per map), an adventure's embedded `tiles/pack.json` became its own flat `<adventure>/art/` read by the same `artlib`, and the DM console was fixed to read control from the roster rather than from presence. 21 commits on `feat/art-is-a-flat-library`, 142 files, +18,297 / −3,005.

---

## 2. Hur det fungerar i dag

Verified against the current tree: `go build ./...` clean; `go test ./internal/artlib/... ./internal/mapdef/... ./internal/campaigncfg/... -count=1` all pass; `internal/artlib` at 99.4 % statement coverage against a 99.0 floor in `tools/coverage-thresholds.txt:33`.

**Resolution.** `artlib.Open(dir)` takes one `os.ReadDir` snapshot of the art directory into two maps: exact entry names, and a lowercase→real fold index built only for entries that are not already lowercase. `Library.Lookup(id)` then (a) checks `isArtID(id)` — lowercase ASCII letters and digits joined by single interior hyphens, at most 250 bytes; (b) checks the snapshot for `<id>.json` and `<id>.png` by **exact** name; (c) opens the directory through `os.OpenRoot` and reads. Every name below the id gate — the sidecar, the picture, a door's `open` and `closed` — is checked against the same snapshot before the filesystem is trusted (`Library.has`, `statPicture`). A name that differs from the request only in case produces a `*CaseMismatch` error wrapping **both** `ErrCaseMismatch` and `ErrNotFound`, so existing `errors.Is(err, ErrNotFound)` arms keep firing unchanged.

**The one snapshot per load.** `mapdef.BuildSceneCreated` calls `artlib.Open` once (`internal/mapdef/compile.go:199`) and threads the `*Library` through every square and object, so a 3600-square map pays one directory scan, not 3600. `mapdef.Resolve` (`internal/mapdef/resolve.go:106`) is the single-square wrapper and opens its own library — its doc says explicitly that a many-square load must not call it. The package-level `artlib.Lookup(dir, id)` is `Open(dir).Lookup(id)`; its only production caller today is `cmd/vtt/art.go:216`, one id per installed file.

**Degrading.** An unresolvable art name costs one square, never the map: the square keeps the kind and material `m.Tiles` declared and loses only its particular picture, and a warning is emitted. Warnings are collapsed **once per art name**, not per square (`warningTally` in `internal/mapdef/compile.go`); the kind-mismatch warning is the exception and keeps its square keys, capped, because that is the one warning whose remedy needs to know which square.

**The one refusal left.** A sidecar declaring a `format_version` **later** than `artlib.FormatVersion` (1) wraps `ErrFormatVersion` and refuses the map. Everything else a sidecar can get wrong — a missing brace, a truncated copy, an unknown field, a `format_version` that is absent, that is not a version number, or that is *below* 1 — is a broken file and degrades. The version is read in a lenient pre-pass (`declaredFormat`, a `json.RawMessage`) before the strict `DisallowUnknownFields` decode, because a real v2 sidecar carries v2 fields and a single strict decode would report `unknown field "variants"` and never mention the version.

**Boot.** `composeServer` (`cmd/vtt/serve_compose.go:260-318`) runs `artRootIsOpenable(artDir)` — an unopenable art root **refuses the boot**, because that is every piece failing rather than one — then `artlib.Validate(artDir)`, which walks the directory once and collects *every* problem (subdirectory, symlink, a filename no map could spell, a sidecar that does not resolve), logs them all via `slog.Warn`, **and starts the server anyway**. `WithArtDir` then hands the gateway a path; nothing is read at boot. Art installed or overwritten mid-session takes effect at the next `load_map`, with no restart.

**The route.** `GET /api/art/{file}` → `Server.handleArtFile` (`internal/gateway/metadata.go:689`): auth, then `artlib.IsArtFileName(name)` — the same rule `Lookup` resolves by, admitting exactly `<id>.png` and `<id>.json` — then `os.OpenRoot(s.artDir)`, then `fs.Stat` on `root.FS()` requiring a regular file, then `ServeFileFS` off the same `fs.FS`. `X-Content-Type-Options: nosniff` and `Cache-Control: no-cache`; an unknown extension cannot occur because the name check runs first. The encoded-slash form is tested directly (`internal/gateway/metadata_test.go:1210`, plus `internal/gateway/artfile_internal_test.go`).

**Install.** `vtt art install <path>...` refuses a directory, refuses a non-regular file, refuses a name failing `IsArtFileName`, refuses an existing stem without `--force`, copies with a `.vtt-replaced` backup, then runs `artlib.Lookup` over `art/` **as it now is** for every installed stem and rolls the whole batch back if any fails — so a malformed sidecar is refused where the operator is holding the file rather than at the table.

**cell_px.** `internal/campaigncfg` reads optional `campaign/campaign.json`; `DefaultCellPx = 64` when the file or the field is absent; bounded 8..1024; a map may declare its own `cell_px` which overrides the campaign default. `campaigns/example/campaign.json` holds `{"format_version": 1, "cell_px": 64}`.

**Adventures.** `Adventure.ArtDir` is `<adventure>/art`, resolved by the same `artlib` rooted at the bundle. `loadEmbeddedPack` and `Adventure.Pack` are gone; a bundle still shipping `tiles/pack.json` loads and the directory is simply not read. **The resolving half is all that is built** — see section 4.

**Contract.** `CommandResult.warnings = 5` (`contract/vtt/v1/commands.proto:302`), a repeated string, additive per ADR-007. `client/src/app.ts:421` renders `res.warnings.join("; ")` into the toast on a successful command.

**The example campaign** is 13 files in `campaigns/example/art/`: `masonry-1`, `earth-1`, `flagstone-1` (sidecar + picture each), the `cellar-door` sidecar with its two pictures and no `cellar-door.png`, and four sidecar-less object pictures (`barrel`, `brazier`, `crate-wood`, `pillar-stone`). No `packs/` directory survives anywhere. Two `pack.json` files do survive and are legitimate: `client/public/std-pack/pack.json` and its build copy `cmd/vtt/webdist/std-pack/pack.json` — the client's *standard baseline* art, which kept its name and is exempted by word in `check-no-pack.py`.

---

## 3. Besluten och varför

**1. Packs leave; art is one flat directory; the filename is the id.** *(Patrik, 2026-09-02, verbatim: "we do not allow underlying folders. You want a different masonry_1, then you have to call it something else. very simple." And: "each 'art' is unique and should be able to be used by any map.")*
Rejected: keeping packs as namespaces. `mapdef.Map.Pack` was a single string and every `overrides` value resolved against that one pack, so a map could use art from exactly one source; wanting a wall from one and a tree from another forced copying. Worse than the duplication is what the constraint rewards — a bespoke pack per map, turning a distribution unit into a per-map bundle. Also rejected: a manifest or registry listing what is installed (nothing declares an id, so nothing can disagree with the filename), and a duplicate check anywhere (the filesystem is the check).
Cost accepted: renaming a file breaks every map that names it.

**2. Content-addressed ids (MapTool's answer) rejected.** MapTool's asset cache is also one flat directory with a picture and a sidecar keyed by stem — `AssetManager.getAssetCacheFile` is `cacheDir/<id>`, `getAssetInfoFile` is `cacheDir/<id>.info` — read 2026-09-05 under CLAUDE.md rule 9. The one divergence is deliberate: theirs is the MD5 of the picture's bytes. Content-addressing buys de-duplication and rename-safety free, and costs what this platform cannot pay — a map file here is JSON a DM or an LLM writes by hand, and `"overrides": {"0,1": "masonry-1"}` is a sentence either can read where a 32-character digest is not. It would also orphan every reference the moment somebody retouched a picture.

**3. `Lookup` versus `Library.Lookup` — the performance decision. Reproduced; the numbers hold.**
The design's founding rule was "no scan on the hot path": knowing the id IS knowing the path, and a `ReadDir` would be the boot-time load the design exists to delete, just moved. On 2026-09-08 (`f04c63b`) that was reversed, because APFS is case-insensitive and `Lookup(dir, "masonry-1")` against a `Masonry-1.png` returned a `Piece` naming a file Linux does not have. The accounting error named in the commit: *"the ReadDir is per LOAD, not per lookup."*
The cost landed on `Validate`, which had been calling the package-level `Lookup` per sidecar — once that took a snapshot, that loop became quadratic. `internal/artlib/artlib.go:804` and again at `:1208` claim **773 ms for 800 pieces against 25 ms before**, and `f04c63b`'s message adds *"Back to 27ms."*

**I reproduced all three numbers**, on go1.26.4 darwin/arm64, APFS, in an isolated copy of the package in the scratchpad (nothing in the repo was touched). 800 pieces = 1600 files, best of 5 after a warm-up:

| variant | best | mean |
|---|---|---|
| pre-snapshot `Validate` (`0268a69`, direct-open `Lookup`) | 22.11 ms | 28.05 ms |
| current `Validate`, shared `Library` | **22.96 ms** | 24.60 ms |
| `Validate` calling the package-level `Lookup` per sidecar | **793.46 ms** | 828.72 ms |

And it is genuinely quadratic — doubling n roughly quadruples the time: 200 → 50.5 ms, 400 → 208.2 ms, 800 → 760.5 ms, 1600 → 3248.4 ms.
**Verdict: the comment's 773 ms / 25 ms is accurate and still reproducible.** The three-line difference (793 vs 773, 22.96 vs 27) is machine noise. The decision that follows from it is sound and is still the shape of the code: `Lookup` is the single-shot form for a caller with one id (today: `vtt art install`, one per file); every caller with many ids Opens once.
Rejected alternative: keeping the scan-free `Lookup` and living with the case-folding residual. The package doc had accepted exactly that — see section 4.

**4. Unresolvable art degrades one square; it does not refuse the map.** *(Patrik's rulings, 2026-09-03 and 2026-09-04.)*
Rejected: refusing. Measured, twice, and both times the refusal took the whole platform down rather than one square. A picture with no sidecar named as tile art: `composeServer` turns any map-load error into a refusal to start, so one PNG without its JSON meant nobody could play. And a corrupt sidecar named by one committed map stopped the server booting, exit status 1, with every other map fine — *"and the boot log said 'starting anyway' immediately before it did not."*

**5. One thing still refuses, and it is `>` not `!=`.** A `format_version` **later** than this server's refuses the map, because that is a fact about the server rather than about the file, and the remedy is a newer server. Degrading it would turn a v2 art set into hundreds of plain squares reading "my art is broken" when the diagnosis is "this server is too old" — one refusal naming both versions says that; ninety warnings do not.
Rejected: `!= FormatVersion`, which shipped until Task 8 and gave a typo'd `"format_version": 0` the refusal reserved for the future.

**6. `Validate` reports every problem and the server boots anyway.** *(Patrik's severity ruling, 2026-09-03.)*
Rejected: refusing the boot (a campaign with two hundred good pieces and one `Masonry-1.png` copied off a Windows box must not be held hostage), and returning at the first finding (an operator with three mistakes would pay three boots). `errors.Join` keeps `errors.Is` reaching each one, and `vtt art install` reads it as the single refusal it always was.
The one thing that *does* refuse the boot is an art root that cannot be opened — that is every piece failing, not one.

**7. An adventure keeps its art self-contained.** *(Controller's ruling, 2026-09-03, from a pre-flight finding neither spec nor plan had.)*
Rejected: installing an adventure's art into `campaign/art/` on load, which collides with the flat namespace by construction — two adventures shipping `masonry-1` would fight over a name the DM never chose. Two roots, one mechanism, and `artlib` stays the only art code in the tree.

**8. Delete the pack early — Task 5, not Task 7.** *(Patrik, 2026-09-04.)*
Rejected: the phased removal copied from `create_scene`'s precedent. The plan had copied the shape without checking whether the situations matched: `create_scene` was a contract command with live clients, a pack is an internal Go type with no external consumer. *"The test is not 'is this a removal?' but 'who is standing on it while I take it away?'"*

**9. No migration route at all.** *(Patrik, 2026-09-06.)* Three refusals carrying migration instructions had been built for an audience of zero. They were deleted, along with the five tests pinning their wording and the spec paragraph arguing about compatibility. A map declaring `pack` is still refused — by `DisallowUnknownFields`, which was doing the whole job the special cases only decorated.
The lesson recorded: rule 8 asked who is standing on the old thing; it never asked who is arriving at the new one, and the answer there is nobody too.

**10. One warning per art NAME, not per square.** Per-square was built on Patrik's instruction and withdrawn by him on seeing that one square can carry many things — objects key on their anchor, so four crates stacked on one square render that coordinate four times. The bound that justifies the collapse is **per adventure**: `adventure.Compile` concatenates every scene's warnings onto one `CommandResult` and nothing caps the scene count.

**11. The route's guard is an explicit name check, not the route pattern.** See section 4 — this reverses a claim made three times.

**12. `if`/`else if` rather than `switch` in the guards.** Not style. Go's cover tool starts a counted block at a `case` *body*, so gremlins scores a case *condition* as NOT COVERED and never runs it. Rewriting `isArtID`'s loop took the package from 36 killed / 1 lived / 14 not covered to 46 / 0 / 5 with no test added.

**13. A door may name the same picture for both states — permitted, deliberately.** Rejected: an `Open != Closed` check, because what an author gets wrong is that the two pictures *look* the same and what `==` sees is that they are *spelled* the same. `cp cellar-door-closed.png cellar-door-open.png` is one command and passes such a check. Also rejected: a warning, which would carry the same defect one register quieter.

**14. `Clip` and `BoundErr` live in `artlib` rather than in a package of their own,** because `mapdef` already imports `artlib` and the alternative was a new package with its own architecture entry, coverage floor and mutation-gate registration — gate work, which is paused.

---

## 4. Vad som visade sig fel

### 4.1 The case-folding residual — a defect that was written down as accepted, then closed

**The original claim**, `internal/artlib/artlib.go` as of `0268a69`, lines 17–23:

> // The DISK half is closed by Validate, which refuses a name no map could ever
> // spell — but only when Validate runs. Between two runs, a Masonry-1.png
> // dropped in by hand still resolves for "masonry-1" on macOS and hands the
> // renderer a filename Linux does not have. Closing that inside Lookup needs a
> // ReadDir, which is the boot-time load this design exists to delete, so the
> // residual stands: it is the same cp-after-boot window spec §5 already accepts,
> // and `vtt art install` is what catches it.

**The correction** (`f04c63b`, 2026-09-08):

> This was not unknown. The package doc recorded it as an ACCEPTED RESIDUAL: closing it "needs a ReadDir, which is the boot-time load this design exists to delete", so the gap stood, with `vtt art install` named as what catches it. What that accounting missed is that the ReadDir is per LOAD, not per lookup.

The consequence while it stood: a campaign authored on a Mac drew correctly there and lost those squares to plain terrain on Linux and in CI, **silently**, because from the server's side the piece resolved. The fix's own first version was also wrong and was caught in review by measurement — it gated only the *id*, leaving every read below it (the sidecar, the picture the sidecar names, a door's two pictures) going back through the case-folding filesystem; a miscased sidecar could return a **door where the map had a wall**, and the shipped `cellar-door` shape was one of the cases that slipped through.

### 4.2 The path-traversal guard: nine unmeasured mutants behind 100.0 % coverage

`internal/artlib`'s first mutation run reported **14 NOT COVERED out of 51 mutants against a suite at 100.0 % statement coverage**, and **nine of the fourteen sat on the character comparisons inside `isArtID`** — the package's traversal guard, the one function most worth measuring. Go's cover tool begins a counted block at a `case` *body*, so a mutant positioned on a case *expression* falls outside every counted block and gremlins skips it without running anything, while the line number — computed from the bodies — reads 100 %.

The correction, `tools/mutation-scope.md:523`:

> The general rule: **line coverage does not predict mutant coverage**, and a package can sit at 100% while a quarter of its mutants are never evaluated. When a guard matters, write it as `if`/`else` and let the gate measure it.

And `c3ef4ac`'s closing line: *"A green mutation gate bounds the mutators it has, not the defects that exist."*

Two things worth keeping beside it. First, the same review found that **the traversal guard could be deleted with the whole suite still green** — the test asserted only that an error came back, and in a fresh `TempDir` every escaped path is missing, so `Lookup` errored either way and the test could not tell a refusal from a miss. The sibling test in `mapdef` had already solved this by planting a loadable file at the escape target; the id list had been copied across and the discipline had not. Second, a fifth NOT COVERED mutant sat on `Validate`'s symlink refusal — the package's other security guard — *"until review pointed out that the remedy below was being recommended and not applied."*

### 4.3 The route that accepted an encoded slash

**The original claim**, plan `afe3f00`, and repeated in a dispatch and to Patrik:

> **And one requirement has no pack precedent to copy.** The pack route was saved from serving a nested file only by net/http's single-segment `{file}` wildcard; nobody had to think about it, because a pack WAS a directory. `art/` is flat, so this is now a rule the route must enforce rather than a shape it inherits.

**The correction**, same file, `390879a`, 2026-09-05:

> **Measured 2026-09-05 and false:** the wildcard rejects the literal form (`/api/art/a/x.png` → 404) and passes the ENCODED one — `/api/art/pack-ish%2Fx.png` arrives at the handler with `PathValue("file") == "pack-ish/x.png"`, because the mux decodes `%2F` into the value after matching. So the pattern's independent contribution against a determined request is **zero**, and the pack route had the identical hole; it was never protected by its shape.

Two compounding facts: `os.Root` **confines but does not flatten**, so `art/pack-ish/x.png` is legitimately inside the root and a root alone would serve it; and `fs.ValidPath` rejects only `..`. The guard is therefore the explicit name check, `artlib.IsArtFileName`, and nothing else. Twenty-seven payloads — double-encoding, overlong UTF-8, unicode slash lookalikes, null bytes, symlinks wearing legal art names — were run against the new route and leak nothing. Also recorded, and general: *"A test using the literal `/a/b` shape passes against a defenceless handler, because the mux answers 404 before the handler runs — the test proves the mux's behaviour, not the guard's."*

### 4.4 Prose that was false when written, in the arc's own code

The repo calls false prose its dominant defect class and this arc is the case for it. The instances, each measured:

- **"each broken piece is refused when a map names it"** — written into five Go files and true of one arm out of five. It rotted in the subtlest way available: nobody edited it. Patrik's boot ruling made it false silently. Measurement found two arms that *render* with nothing objecting anywhere. (Review finding F1, 2026-09-03.)
- **`packRefJSON`'s obituary claimed the renderer read `cellPx` to draw at the right scale.** It never did — the client's only cell size was a hardcoded `CELL = 44`, and two files declared the field while nothing read it.
- **`warningTally`'s doc said the un-deduplicated form put 6840 bytes on the cellar map and "roughly 270 KB" at `MaxWireTiles`.** The 6840 does not reproduce — those 96 warnings measure 4917, and at the honest rate a map at `MaxWireTiles` lands *under* the 200 KiB read limit. The sentence justifying the type described a threshold no map ever crosses, and the figure had been copied into four comments. It is an invariant now, not a number.
- **The proto comment described behaviour the platform did not yet have** and would have propagated verbatim into generated Go and TypeScript.
- **The spec's §8 and exit criterion 5 still described the retired `!=` `format_version` rule** in the section tasks write their tests from, three commits after the guard became `>`.

### 4.5 Defects that escaped the merge

Four landed within 48 hours of `b674abb`, in this arc's own code:

- **`24ae4a0` (2026-09-09), the art bug one layer up.** `mapdef.LoadInstalled` built its path from the id — `file := id + ".json"` — and opened it, so on APFS `Cellar.json` resolved for the id `"cellar"` and the declared-id check passed too. *"Found by walking the tests sub-project 16 deleted."* The obituary the arc left behind for `TestLoadMapsDirRefusesDuplicateMapIds` justified the removal with the same sentence that justified the art uniqueness rule, and it had the same hole. *"The deleted test was not the loss; the argument left behind was."*
- **`91ab275` (2026-09-09), a campaign's own files reaching a client whole.** A 20 KB value in a sidecar's `kind` produced 20,191 bytes of warnings; `handleLoadMap` has already committed and broadcast the scene by the time the frame is built, so every other seat's board changed while the issuer's socket closed on "message too big". `artlib.Clip` and `BoundErr` were added. **The commit's own first draft then re-broke it**: two clips in series make both mutants unkillable, that was read as redundancy, and deleting the `mapdef` one let an override value of any length through — measured at 20,041 bytes. *"A surviving mutant means no test drives the path; look for the missing test before concluding the guard is spare."*
- **`2b5878d` and `2cfed2c` (2026-09-09/10)** closed two more unbounded render sites in `resolve.go`, and `2b5878d` took **five review rounds, with the code right after the second** — the other three rounds were the prose written about it.

### 4.6 Still wrong in the tree today

Three I verified myself, all live:

**(a) An adventure's art is fetchable by no client.** `internal/adventure/format.go:58-72` says it plainly: *"WHAT IS BUILT HERE IS THE RESOLVING HALF ONLY… The only route serving art bytes is GET /api/art/{file}, whose handleArtFile opens the CAMPAIGN's art directory, so a piece resolved from here is fetchable by no client — it resolves with no warning and every browser draws the square plain, which §4 of the design spec calls worse than the refusal it replaced."* Nothing ships in that state (neither adventure declares an override or an object), but the pre-flight ruling of 2026-09-03 is half-delivered.

**(b) Two comments in the tree give different counts for the same finding.** `internal/artlib/artlib.go:1131-1132` says the retired claim *"was written in four places and is false in three of the five arms"*. `cmd/vtt/serve_compose.go:279-280` says *"that is false in four of the five arms (review finding F1, widened by Patrik's ruling of 2026-09-04)"*. `78ff462`'s own commit message says *"Five places said…"*, and the spec amendment in that same commit says *"a sentence that five Go files had already retired"*. The `serve_compose` text explains the difference and therefore makes `artlib.go`'s number the stale one — it carries the pre-2026-09-04 count beside a bullet list that reflects post-ruling behaviour. I could not establish whether "four places" or "five places" is right; the literal sentence was never in the tree verbatim, so no grep settles it.

**(c) `artlib.go`'s 74-line comment at 369–442 is the doc for `clip`, and `clip` is at line 486.** There is no blank line before 434, so `go doc` attaches the whole run to `const ( MaxFragment; MaxMessage )` — I ran it: the published documentation for two exported constants begins *"clip bounds a fragment of author-controlled sidecar text…"*, and `clip` itself has no doc. The same fusion — a new opening paragraph appended without the old one being removed — appears at 781–790 (`Lookup`, two openings), 1058–1065 (`statPicture`) and 1205 (`entryProblem`).

---

## 5. Spår

**The range.** `afe3f00` (exclusive) .. `f04c63b` (inclusive), 21 non-merge commits, merged as `b674abb` on 2026-09-08.

| | |
|---|---|
| Spec | `docs/superpowers/specs/2026-09-02-art-is-a-flat-library-design.md`, committed `1fec646` "Art is a library, not a claim", 2026-09-02, on `main` |
| Plan | `docs/superpowers/plans/2026-09-02-art-is-a-flat-library.md`, committed `afe3f00` "Plan: art is a flat library", 2026-09-02, on `main` |
| Branch | `feat/art-is-a-flat-library`, cut from `afe3f00` on 2026-09-03, merged 2026-09-08, since deleted |
| Merge | `b674abb`, parents `afe3f00` (main) and `f04c63b` (branch tip) |
| Size | 142 files, +18,297 / −3,005 |
| Follow-ons | `24ae4a0` (`fix/map-filename-case`, 09-09), `91ab275` (`feat/bound-author-bytes`, 09-09), `2b5878d`, `2cfed2c` (`fix/bound-author-controlled-bytes`, 09-10), `caad475` (09-10) — all correcting this arc's code, all outside the merge |

The 21 commits, oldest first:
`255551a` the pre-flight scan · `c3ef4ac` the art library (Task 1) · `e3e449d` warnings reach the client (Task 2) · `37f715a` mutation-key repair · `6c0f02c` Resolve degrades (Task 3) · `2b59f72` the cap was four · `78ff462` the no-subfolders rule is enforced (Task 4) · `bfd6ebb` a map naming a pack is refused (Task 5) · `8a28f34` the pack leaves (Task 7, run fifth) · `a049e4d` a broken file is not a server that is too old (Task 4b) · `390879a` art gets a route (Task 6) · `035248e` the demo campaign gets its art back (Task 8) · `3a99a26` `check:no-pack` (Task 9) · `66ef637` no landing pad · `16a58dd` the threshold never crossed · `2d428a6` the console asked presence · `d91ebbd`, `6d567df`, `02070bc`, `0268a69` review rounds · `f04c63b` an id resolves only to a file named exactly that.

**How I established it, and what is not mechanical.**
1. `git log -1 --format=%P b674abb` gave the two parents. The mainline parent `afe3f00` turns out to *be* the plan commit, so the range is exactly "everything after the plan landed".
2. `git log --oneline --no-merges afe3f00..f04c63b` gave the 21 commits.
3. **The branch name is not in the merge.** The subject is "Merge sub-project 16: art is a flat library" — no `Merge branch 'x'`. I recovered `feat/art-is-a-flat-library` from `git reflog show main` (`b674abb main@{2026-09-08}: merge feat/art-is-a-flat-library`) and the HEAD reflog, which also dates the checkout at 2026-09-03. That is local reflog only; it would not survive a fresh clone.
4. **There is no mechanical link from plan to commits.** The plan prescribes a commit message at each task's final step; only **two of the four** it spells out appear verbatim in the log (`bfd6ebb`, `8a28f34`). Task 6's planned "A grid is uniform, so its cell size belongs to the campaign" shipped as "Art gets a route, and the board gets the window"; Task 8's planned "The example campaign keeps its art in one place" shipped as "The demo campaign gets its art back". The plan's task *numbers* are traceable only through commit **bodies** and through code comments that name `art-is-a-flat-library Task N` in prose — which is itself part of why this report exists.
5. The ordering deviates from the plan on purpose: Task 7 ran fifth (`8a28f34`), on Patrik's 2026-09-04 ruling to delete the old solution first.
6. `docs/superpowers/specs/.../2026-09-02-...-design.md` has been edited by eleven commits, five of them **after** the merge — so the spec is not a snapshot of what this arc delivered and must not be read as one.

---

## 6. Kommentarsblock denna rapport friar

**First, the measurement checks out, and it is about `artlib.go` alone.** Counted with a Go-aware lexer (four states: normal, interpreted string, raw string, block comment) so a `//` inside a string literal is not miscounted; a naive first-token rule gives identical numbers, and the package contains no `/* */` comments at all.

| file | total | comment | code | blank | comment share | blocks ≥10 |
|---|---|---|---|---|---|---|
| `artlib.go` | 1263 | **844** | **372** | 47 | **69.4 %** | **22** |
| `artlib_test.go` | 1883 | 568 | 1213 | 102 | 31.9 % | 19 |
| `artlib_internal_test.go` | 40 | 10 | 26 | 4 | 27.8 % | 1 |
| combined | 3186 | 1422 | 1611 | 153 | 46.9 % | 42 |

844 / 372 / 22 / 69 % match `artlib.go` exactly. The package as a whole is 46.9 %.

**The honest finding before the list: not one of the 42 blocks is pure history.** Every one either states an invariant outright or wraps one in narration. So no block on this list can be deleted whole. What this report frees is the *narration inside* each block — the dated measurement, the earlier wording, the review-round number, the ruling's provenance — leaving the rule. Nineteen of `artlib.go`'s 22 blocks are in that state; three are invariant-only and are **not** on this list at all.

### `internal/artlib/artlib.go` — blocks this report's content now holds

| lines | len | attached to | what the narration is about |
|---|---|---|---|
| 1–117 | 117 | `package artlib` | the whole flat-art design, the retired accepted residual (16–39), a MapTool source read dated 2026-09-05 (48–74), and "THE LAST BULLET USED TO SAY THE OPPOSITE" (110–117) |
| 176–198 | 23 | `var ErrArtDirUnreadable` | the 2026-09-03 two-callers ruling and the Task 3 round-1 path disclosure |
| 201–238 | 38 | `var ErrFormatVersion` | that the rule was `!=` until Task 8 and what a typo'd `0` cost |
| 241–252 | 12 | `type Piece` | that exit criterion 6 read "is refused" until Patrik's 2026-09-03 ruling |
| 263–284 | 22 | `type declaredFormat` | review findings F3 and F4 of 2026-09-05 |
| 369–442 | 74 | (`clip`; godoc attaches it to the `MaxFragment`/`MaxMessage` const) | the bounds inventory, what is STILL UNBOUNDED, "THREE ENTRIES LEFT THIS LIST", and the mutation-gate argument for two `ToValidUTF8` passes |
| 488–521 | 34 | `func bareCause` | that it cost three review rounds on Task 3 and which phase each round missed |
| 537–553 | 17 | `func isArtID` | that round 1 carried a dead `id != ""` and a first draft brought it back |
| 600–623 | 24 | `func IsArtFileName` | the one sentence about the pack route being "saved" by the wildcard |
| 628–656 | 29 | `type Library` | the measured APFS/HFS case-folding bug and Patrik's 2026-09-03 reporting ruling |
| 693–706 | 14 | inside `Open`'s entry loop | the `!ok \|\| name < prev` comparator that was removed because both mutants were unkillable |
| 781–790 | 10 | `func Lookup` | that calling it in a loop is what made `Validate` quadratic |
| 795–808 | 14 | `func lookupIn` | "this paragraph said the opposite until then" + the 773 ms / 25 ms measurement |
| 859–895 | 37 | `func pieceFromSidecar` | the measured v2 fixture and review finding F2 of 2026-09-05 |
| 928–944 | 17 | inside `pieceFromSidecar` | that a single `!=` arm shipped until Task 8, plus the campaigncfg / `mapJSON.Pack` aside |
| 997–1039 | 43 | inside `pieceFromSidecar` | Task 8 review finding F4 and the 15-line MapTool `MD5Key` digression (1012–1018) |
| 1101–1168 | 68 | `func Validate` | that Task 1 returned at the first finding and Task 4 changed it; review finding F1 and its per-arm table; an open question dated 2026-09-03 |
| 1195–1209 | 15 | `func entryProblem` | review finding F4 of 2026-09-03 and the 25 ms / 773 ms measurement again |

**Not on this list — these stay, in full:**
- **348–362**, `unsupportedFormat`: the only place `ErrFormatVersion` is wrapped, and why a second construction site is a second thing that can forget the sentinel.
- **949–958**, the below-version arm: why it takes no sentinel, and what whoever bumps `FormatVersion` must decide.
- **1212–1225**, `entryProblem`'s symlink refusal: `Type()` is the entry's own type; the link rule runs before the is-it-art filter; `if` rather than `switch` because gremlins scores a case condition NOT COVERED.

And inside every block on the list, the invariant sentences stay. The ones a reader must obey to change the line, by block: *"EVERY FILE IS OPENED THROUGH os.OpenRoot, never a plain filepath.Join"* and the four-sentinel table (1–117); *"NEITHER THIS ERROR NOR ANYTHING IT WRAPS NAMES A PATH"* (176–198); *"'LATER', NOT 'DIFFERENT'"* and *"THE SENTINEL IS THE INTERFACE"* (201–238); *"HasSidecar … exists because Kind cannot carry that fact"* (241–252); *"a Go zero value cannot carry PRESENCE"* (263–284); *"ONE BOUND PER PATH"* and *"A surviving mutant means no test drives the path"* (369–442); *"EVERY OS ERROR … GOES THROUGH HERE, and the rule is per SYSCALL PHASE"* with its three-phase table (488–521); *"the EMPTY STRING … needs no clause"* (537–553); *"os.OpenRoot CONFINES WITHOUT FLATTENING"* and *"TWO EXTENSIONS AND NO MORE"* (600–623); *"The snapshot is deliberately not refreshed"* (628–656); *"FIRST WINS … because os.ReadDir returns entries SORTED BY FILENAME"* (693–706); *"A caller resolving MANY ids must Open once"* (781–790); *"install and a map load still run the SAME resolution"* (795–808); *"IT READS THE VERSION IN A PASS OF ITS OWN"* and *"IT IS A json.Decoder AND NOT json.Unmarshal"* (859–895); *"The boundary is killable in both directions"* (928–944); *"A door has TWO pictures and no third"* (997–1039); *"IT REPORTS EVERY PROBLEM IT FINDS, NOT THE FIRST"* and *"An absent art/ passes"* (1101–1168); *"ONE ENTRY CONTRIBUTES AT MOST ONE PROBLEM, AND THAT IS STRUCTURAL HERE"* (1195–1209).

**Size of the prize.** I did not measure the history/invariant split line by line across all eighteen. What is measured: the five largest blocks (117, 74, 68, 43, 38 = **340 lines, 40 % of `artlib.go`'s comment**) each carry three to five short rules buried in dated narrative, and reduced to their rules would fit in roughly 40 lines.

**The test files hold 20 more blocks ≥10 lines** (19 in `artlib_test.go`, 1 in `artlib_internal_test.go`), and the same pattern: 15 of the 19 are mixed. This report covers the history in at least these — `artlib_test.go` 286–309 (the 2026-09-03 ruling recap), 634–660 (a `-1` row that sat there until Task 8), 689–709 (review finding F4 and its two corrections), 803–823 (the 2026-09-05 fault-injection proof), 871–887 (review finding F2), 908–918 (Patrik's 2026-09-04 ruling), 1103–1120 and 1171–1187 (the Task 3 path-disclosure rounds), 1291–1310 (that `campaigns/example` had no art until Task 8), 1386–1410 (the whole case-folding narrative), 1433–1442 (what the first exact-name rule missed), 1617–1627 (that a fixture said "the longest a real one gets" while using 122), 1722–1735 and 1814–1833 (the 2026-09-09 clip measurements).

---

## Svar på gate-frågan: does `check:no-pack` and its two siblings still protect anything this report would not?

**Cost, measured.** 3,710 lines exactly — `check-no-pack.py` 724 + `check-no-create-scene.py` 546 + `check-no-retraction.py` 484, plus their three test files at 853 + 683 + 420. Plus 131 lines of Taskfile `desc` prose. Roughly **900 of the 1,754 script lines are three near-copies of the same comment/string masker** (the scripts say so themselves). They carry 117 tests of their own, all green, all inside `task check` (`Taskfile.yml:47-49`) and therefore inside CI; none is in `check:fast`. All three pass on the current tree, in under three seconds total.

**Catches, searched four ways: zero.** Six commits in the repo's 496 touch the three scripts, and they are the three births plus one scope extension and two docstring corrections. No exemption has **ever** been added to any of the three after it was written; the only post-birth allow-list change (`66ef637`) *removed* one. No commit message anywhere records a gate firing. And replaying today's scanner over every commit since each gate landed — 60 commits for `no-retraction`, 45 for `no-create-scene`, 29 for `no-pack` — produced **0 failures**. On the day each was written they found nothing live either: `check-no-create-scene` shipped with an empty `EXEMPT`, and `check-no-pack`'s 87 code positions were all in the two legitimate families.

**What they would and would not cover if deleted.**

1. **Go code: redundant.** Every banned symbol is deleted, so a *reference* to one fails `go build` with an undefined-symbol error. I checked every surviving occurrence in every `.go` file: **all of them are comments.** The gate's one unique job is catching a *fresh declaration* under a banned name, which compiles fine — but that is exactly what `internal/mapdef/load_test.go:257` `TestNoPackTypeOrLoaderRemainsInThisPackage` already does for the package where it matters, in eight spellings.
2. **Comments and Markdown: the gates do not cover these at all** — and this is the premise the cleanup needs corrected. The scripts **mask comments and string literals before reading**, and `.md` is not in `SOURCE_SUFFIXES`. The repo's own `6ad7320` (2026-09-14) states it: *"None of the gates read Markdown, so none of them would have caught any of it."* The ~20 Go-comment references to the six deleted symbols, and the 327 occurrences of the word in Markdown, are invisible to `check:no-pack` today and would remain invisible either way. **This is precisely the territory this report covers and the gate never did.**
3. **On-disk JSON: already held by the loader.** `mapdef.decodeStrict` uses `DisallowUnknownFields`, so a map declaring `"pack"` — or `"package"`, or anything else — is refused by name without the loader knowing any of those words. `66ef637` deleted the special-cased fields for exactly that reason. `TestAMapDeclaringAPackOrAPackageIsStillRefused` is the guard. The gate's `.json` arm reads object keys only and is the weaker of the two.
4. **The blind spot is the most likely failure.** `check-no-pack`'s needle is `[Pp]ack(?!age)|PACK(?!AGE)`, carving out the English word `package` — which was unavoidable, since 923 of the 1,857 hits were Go's own `package` keyword. The consequence, which the script documents and which was **verified by injection, exit 0**: `ArtPackage`, `LoadPackage`, `handlePackageFile`, `PackageTile` all escape. That is the most idiomatic rename anyone reaching for a new distribution unit would pick.

**My answer.** `check:no-create-scene` and `check:no-retraction` protect a *contract command* and an *event shape* respectively, and their strongest instruments are elsewhere anyway (`contract/events.test.ts`'s `/retract/i` over the **generated descriptors**, and `client/test/command-surface.test.ts`) — those read generated code the source gate never sees, and they cost nothing to keep. `check:no-pack` is the one this report displaces. What it uniquely covers is a fresh Go or TypeScript **declaration** under one of six names in a package the narrow Go test does not reach — against which it has never fired, and which its own carve-out lets through under the likeliest spelling. What it does **not** cover is the thing actually still in the tree: six deleted symbols named in about twenty Go comments, a plan and a spec that teach the word 93 and 55 times, and two of the three biggest holders of the forbidden word being the gate and its own test file.

If the gates go, I would keep `internal/mapdef/load_test.go:257`, `internal/gateway/metadata_test.go:861`, `contract/events.test.ts`'s descriptor read and `client/test/command-surface.test.ts` — they are cheap, they read generated artifacts, and together they hold the half that reaches a table. And if `check:no-pack` is kept, its Taskfile `desc` is stale: it still quotes "87 code positions, 13 files, 23 words" where the script says 80 / 11 / 18.

---

## What I could not establish

- Whether the retired claim was written in **four** places or **five** — the literal sentence never existed verbatim, so no search settles it; the commit message, the spec amendment and the `artlib.go` comment give two different answers.
- Whether the merge message's "16 e2e specs pass, four of which did not before" and "both mutation gates at zero unadjudicated survivors" still hold: the tree is dirty with unrelated work and a mutation run takes ~40 minutes, so I did not attempt either. I verified the cheaper claims instead — `go build ./...` clean, `internal/artlib` / `internal/mapdef` / `internal/campaigncfg` green, `internal/artlib` at 99.4 % against a 99.0 floor.
- The original wording of any claim that was corrected *within* a single commit before it was committed. Only what reached the tree is recoverable.

*Nothing in the repository was modified. The benchmark ran against copies of `artlib.go` (current and at `0268a69`) in the session scratchpad, in a standalone module, not against the working tree.*


---

## Addendum, samma dag: rättade räkningar

ADDENDUM to the "art is a flat library" implementation report I just handed back — two corrections and one new finding, all in the dead-reference counts. Everything else in the report stands.

1. CORRECTION to my count of dead-symbol references. I wrote "six distinct symbols, 11 production Go comment sites". The symbol count is right; the line count was low, because my grep pattern missed the `Map.Pack` and `s.packs` spellings.

   - "Six" holds as a count of SYMBOLS, not lines: `mapdef.Pack`, `PackTile`, `LoadPack`, `ErrPackNotLoaded`, `WithPackFiles`, `packRefJSON` are the six named in production (non-test) Go comments. Seven if you count `Server.packs`, which appears once spelled `s.packs` at internal/gateway/metadata.go:525.
   - As LINES, production Go comments naming a dead symbol number 17, not 11: cmd/vtt/harness_boot.go:126; cmd/vtt/maps.go:22,31; internal/gateway/metadata.go:523,525,526,588,589,665; internal/gateway/server.go:340,341; internal/mapdef/format.go:22; internal/mapdef/installed.go:10; internal/mapdef/load.go:403; internal/mapdef/resolve.go:95; tools/genmappack/main.go:175; tools/genmappack/std_pack.go:37.
   - Eleven more sit in Go tests (one of which is live code, not a comment: the `banned := []string{...}` absence list at internal/mapdef/load_test.go:287), about forty in the plan and spec, and three in the proto plus its two generated copies. Zero live code anywhere — unchanged.

2. NEW, and it strengthens the gate answer in the report's final section: ten dead-symbol references sit in TypeScript comments and are watched by NOTHING.

   client/src/app.ts:227; client/src/metadata.ts:54,56; client/src/view/art-assets.ts:8; client/src/view/pack-assets.ts:15,49,62; client/test/metadata.test.ts:90,91; client/test/pack-assets.test.ts:14.

   They are outside `check:no-pack` (it masks comments before reading), outside the compiler (they are comments), and outside `tools/check-citations.py` — whose own KNOWN LIMITS say "GO COMMENTS ONLY. Markdown, TypeScript and Python prose are not read." check-citations.py is the instrument that protects the obituary genre while catching fabrications, so this is the one place in the tree where a stale reference has no reader at all. It is squarely in the territory an arc report covers and a word-absence gate never did.

3. Minor, and consistent with what I already wrote: `internal/mapdef/load.go` has no named "pack" arm left — the refusal is `decodeStrict`'s `DisallowUnknownFields`, and the message is Go's own `json: unknown field "pack"`. Nothing in the tree spells the word in order to refuse it; only the test spells it, to assert the refusal. I stated this in decision 9; flagging it again because it is the sharpest single argument that `check:no-pack` guards a door the loader already closed.
