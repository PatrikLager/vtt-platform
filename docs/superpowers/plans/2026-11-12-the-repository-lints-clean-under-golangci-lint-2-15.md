# Plan: the repository lints clean under golangci-lint 2.15, with no finding silenced

- **Ticket:** `docs/superpowers/specs/2026-11-12-the-repository-lints-clean-under-golangci-lint-2-15-design.md`.
- **Verdict:** passes, with gaps (below). G1 is a false sentence, not an
  unestablished one; it is carried rather than returned because it changes no
  decision here beyond D7, and its writer corrects it before the ticket is
  committed.
  - Verified 2026-11-12 by an agent that did not write the ticket, read-only
    against `eb0fa35` in the worktree `vtt-platform-lint`, with the ticket
    uncommitted.
  - Nothing in the repository was edited but this file. Every probe ran in a
    copy of the tree outside the repository, with golangci-lint 2.15.0 (gosec
    v2.29.0 and errcheck v1.30.0 inside it). `task check`, gremlins and
    Stryker were not run.
- **Base:** `eb0fa35`. **Branch:** `fix/lint-under-golangci-2-15`.
- **Owner's rulings taken in:**
  - 2026-11-12: the repository moves to golangci-lint 2.15 in this small
    ticket, before another branch's gate runs.
  - 2026-10-21: a near-theoretical edge with a small consequence gets no code,
    no test, no debt entry (D1).
  - 2026-10-15: review findings are fixed and committed per task without
    asking; push and merge on the owner's word.
  - Standing: no counts or measured figures in anything written down.
- **Rule 9 (RPTool):** not applicable. The work is the repository's lint
  toolchain and which `net/http` call one handler makes; MapTool has neither.
- **Signed off:** by the owner, 2026-11-12 ("Godkänd"), with O1 answered V0:
  the repository names no linter version beyond `ci.yml`'s (D7). R2 is not
  drawn; Task 4 is `ci.yml` alone.
- **Ids:** R1 is VTT-682. The dispenser ran against the register of the branch
  `feat/campaign-data-fits-its-ruleset`, whose committed rows already reached
  VTT-681 from the same base, so that no id is dispensed twice; the row was
  then carried into this branch's register.

## The verdict, check by check

1. **Every path resolves. By command.** `cmd/vtt/serve.go` (`newServeCmd`),
   `cmd/vtt/harness_boot.go` (`bootSelfContained`), `internal/gateway/metadata.go`
   (`handleArtFile`), `artlib.IsArtFileName`, `TestHandleArtFileRefusesSymlinkEscape`,
   `Taskfile.yml`'s `lint` task, `.golangci.yml`, `go.mod`'s `tool` block
   (`buf`, `gremlins`, `protoc-gen-go`) and `docs/specifications/012-the-read-surface.md`
   all exist as named. Homebrew's binary is gone; `~/go/bin/golangci-lint` is
   the only one on `PATH` under both `/bin/sh` and `/bin/zsh`, and reports
   2.15.0.
2. **"Done" is an observation. By command.** Item 1's "today" half
   reproduces: `config verify` passes and `run ./...` reports exactly the
   errcheck findings on `<-serveErrCh` in `newServeCmd` and in
   `bootSelfContained`'s close function, and gosec G703 on `handleArtFile`'s
   `http.ServeFileFS`. Items 3 and 4 are regression guards that pass today by
   design; their tests exist (G2).
3. **Each rule is breakable. Reading.** "None", with SPEC-012's confinement
   and SPEC-011's shutdown order standing: both are pinned
   (`TestHandleArtFileRefusesSymlinkEscape` and its siblings;
   `TestServeSubprocessExitsCleanlyOnSIGTERM`). Under D2 the ticket's "None"
   is thin (G3): one answer of the route changes and earns R1.
4. **The scope matches the claim. By command, then reading.** `handleArtFile`
   has one caller, the `GET /api/art/{file}` route in `Server.Handler`;
   `newServeCmd`'s shutdown branch is reached only through the binary
   (`cli_test.go`'s in-process `serve` runs end at boot, before it);
   `bootSelfContained` is called by `buildDialer` in `client_run.go`, by
   `client_soak.go`, by `runLibraryScenarioSelfContained` in `library_test.go`
   and by `scenario_goldens_test.go` and `state_dump_switch_test.go`
   (`TestAPlayersDumpNamesItsLogAgainstARealGateway`); `client_run.go`,
   `client_soak.go` and `library_test.go` read its close function's error.
   The ticket's files cover these. It misses
   `.github/workflows/ci.yml` (G1) and, under D2, the test file whose header
   names `http.ServeFileFS` (D6).
5. **No recorded decision is contradicted silently. Reading.** SPEC-012 moves
   and the ticket names it. ADR-010's point 8 ("gremlins is pinned" as a `go
   tool` dependency, because an unpinned tool makes a gate irreproducible)
   bears on O1; the ticket raises it as open, and the recommendation below
   says where it departs from that mechanism. `Taskfile.yml`'s lint comment
   argues from there being no version check ("a version check nobody would
   write"); V2 makes that clause stale (Task 4).
6. **The records the work moves. By command, then reading.** SPEC-012
   resolves and is the record of the art route. Under D2 it moves in the
   sentences that name the serve, and in its Requirements list (D4). SPEC-011 describes the shutdown
   order and no return value; D1 changes no step of that order, so it does not
   move. Searched `docs/specifications/`, `docs/adr/` and `docs/blueprint.md`
   for the lint gate: no record describes it.

## Gaps

- **G1. A false sentence, and a file the scope misses.** The ticket says
  "Nothing in the repository names a version". `.github/workflows/ci.yml`'s
  "Install golangci-lint" step pins v2.11.4, under "Versions pinned to what the
  gate was calibrated against locally". What is true is that nothing the local
  gate runs names one. The writer corrects the sentence and lists `ci.yml`
  under "What it touches" before the ticket is committed. D7 moves the pin.
- **G2. Item 4 names no test.** They are `TestServeSubprocessExitsCleanlyOnSIGTERM`
  (`newServeCmd`'s shutdown, through the real binary) and
  `TestScenarioLibraryRunsSelfContained` (asserts `boot.close()` returns nil
  for every scenario). Each goes red when its site's filter in D1 is removed
  (probe).
- **G3. Item 3 holds for every pinned answer, and not for one unpinned one.**
  Under D2, an art file the server can stat and cannot open is answered 404
  `gateway: no such art` where today it is 403 `403 Forbidden`. That is what
  SPEC-012 already says every refusal is; the change earns R1. If the owner
  takes D2's alternative (c), G3 vanishes and SPEC-012 does not move.
- **G4. The hook refuses more than the ticket says.** `.lefthook.yml`'s `lint`
  command carries no glob (by reading), so it runs on any commit that stages a
  file, and every commit is refused while a finding remains, not only one that
  changes a Go file. D5 follows from it.
- **G5. Item 2's "one rule".** A golangci-lint `//nolint` names linters, never
  rules; a rule can be named only in the reason. Moot under D1 and D2, which
  add no `//nolint`.

## Constraints

- CLAUDE.md rule 2: `.golangci.yml` does not change, and no finding is
  excluded or suppressed. Item 2's `git diff eb0fa35 -- .golangci.yml` is
  empty.
- CLAUDE.md rule 8: citations name functions, tests, records; never a line.
- CLAUDE.md rule 10 and SPEC-010: a comment is a warning or a pointer.
  `tools/check-comments.py` counts only lines that start with `//`, so a
  trailing comment moves no share, and the reading review reads it instead.
  `internal/gateway/metadata.go` sits at its ceiling: Task 2 rewrites its
  comments within the comment lines it has. Run `check-comments` against
  `main` and against `HEAD` after staging, and `--write-ledger` only if it
  asks, last.
- SPEC-012 (the art route), SPEC-011 (shutdown order), SPEC-008 (ids from
  `requirement-id`, never by hand).
- Launch every golangci-lint run, and `task check`, through `/bin/sh -c`.
- A new test goes after a closing brace, so it inherits no neighbour's doc
  comment (`check:doc-owner`). `tools/check-new-prose.py main` before each
  commit. Stage and commit in separate calls, and check what landed.
- The hook is never bypassed.

## Decisions

- **D1. Both errcheck sites read the value; neither discards it.** Forced by
  probe and by source. errcheck's visitor reports every receive statement of
  an error-typed channel, and no setting exempts a receive. The repository's
  idiom for a deliberate discard, `_ = x // reason`, has no receive form under
  2.15: `_ = <-serveErrCh` clears errcheck and staticcheck refuses it as S1005
  (probe, both sites). What remains is `//nolint:errcheck` or reading the
  value, and reading it suppresses nothing. The shape, at both sites:
  `serveErr := <-serveErrCh`, then `if errors.Is(serveErr, http.ErrServerClosed)
  { serveErr = nil }`. In `newServeCmd`'s shutdown branch, `shutdownErr` keeps
  its precedence and the branch returns `serveErr` last; the receive line's
  trailing comment stays as it is. In `bootSelfContained`'s close function,
  `serveErr` joins `firstNonNil` after `shutdownErr`, and the file gains the
  `errors` and `net/http` imports. Every statement runs on the ordinary path.
  What changes: a `ListenAndServe` or `Serve` that had already returned
  something other than `http.ErrServerClosed` is now returned instead of
  lost. That path needs a listener failure racing the signal, or a
  non-temporary `Accept` failure mid-run: near-theoretical with a small
  consequence, so it gets no test (2026-10-21).
- **D2. G703 is fixed by serving through the root, not by a sanitizer or a
  `//nolint`.** `handleArtFile` calls `root.Stat(name)`, refuses a
  non-regular file, opens with `root.Open(name)`, refuses a failed open with
  the route's 404, defers the close, and serves with
  `http.ServeContent(w, r, name, info.ModTime(), f)`. `fsys` and the `io/fs`
  import go. Forced by these probes, each running `golangci-lint run
  ./internal/gateway/...` and the gateway's art tests in a copy of the tree:
  - **(a) A sanitizer.** gosec's path-traversal sanitizers are
    `filepath.Base`, `path.Base`, `filepath.Rel` and the `strconv` parsers.
    A guard is not one: `fs.ValidPath` and `filepath.IsLocal` before the
    call, and `filepath.Clean` on the argument, all leave G703 standing.
    `path.Base(name)` or `filepath.Base(name)` at the sink clears it and the
    tests pass, but it is the identity on every name that reaches the line
    (`artlib.IsArtFileName` admits no separator), so it reads as the guard
    while guarding nothing. Moved up to `PathValue`, it would flatten
    `a/b.png` to `b.png` instead of refusing it, against SPEC-012. Rejected.
  - **(b) `http.ServeContent` over a file opened through the root.** No
    finding: gosec's sinks are the package-level `os` functions, and
    `(*os.Root).Stat` and `(*os.Root).Open` are methods; gosec's own
    `pathtraversal.go` names `os.Root` as the way to confine a path. The
    whole gateway suite and the art tests in `cmd/vtt` pass
    (`TestCampaignsArtServesEndToEnd` among them). Against today's handler,
    probed side by side: a plain GET, HEAD, `Range`, an unsatisfiable
    `Range`, `If-Range`, `If-Modified-Since` on GET and on HEAD, and the
    sidecar answer with the same status, headers and body, the file's own
    `Last-Modified` time aside. The content type
    is unchanged because the handler sets it before either call. The one
    changed answer is G3's. G703 stays live on the handler: an open by path
    in its place is refused by G703 (probe).
  - **(c) `//nolint:gosec // G703: <reason>`**, or gosec's own `// #nosec G703
    -- <reason>`. Both clear the finding, the tests pass, and SPEC-012 does
    not move. Not recommended: `//nolint:gosec` silences every gosec rule on
    that line, `#nosec G703` silences that rule alone, and either is a
    suppression where this repository fixes a false positive at its cause
    (`.golangci.yml`'s header, point 2). It is the acceptable fallback if
    the owner wants SPEC-012 untouched and no Phase 4a; then R1, D3, D4, D6
    and Task 5 drop out, Task 0 keeps only R2 if O1 is V2, and Task 2 is one
    line.
  - **Stat stays before Open.** `root.Open` on a FIFO wearing an art name
    blocks until a writer appears (probe); `root.Stat` refuses it first, as
    `fs.Stat` does today.
- **D3. The unreadable-file answer is pinned.** D2 changes it, so a test
  holds it: `TestAnArtFileTheServerCannotReadIs404` in
  `internal/gateway/metadata_test.go`, after
  `TestAnUnopenableArtRootDegradesAtRequestTime` and in its shape (mode 000,
  skip as root, restore the mode in cleanup). It fails on today's tree with
  403 `403 Forbidden` (probe).
- **D4. SPEC-012 moves.** In the art-route paragraph: `fs.Stat` on
  `root.FS()` becomes `root.Stat` and `root.Open`, with the stat first; the
  content type is set before `http.ServeContent`; `Last-Modified` and the 304
  come from `http.ServeContent`, from the stat's modification time; a file
  the root can stat and not open is answered with the same 404. Its
  Requirements list gains R1's id. Nothing else in the record names the
  serve.
- **D5. Tasks 1 and 2 land in one commit.** Forced by G4: the hook lints the
  whole tree and refuses every commit while a finding remains. Each task's
  diff is still reviewed on its own before that commit.
- **D6. Comments move with the code.** In `metadata.go`, `artContentTypes`'
  comment names `ServeContent` instead of `ServeFileFS`, and the comments in
  `handleArtFile` become one warning in no more comment lines than the ones
  it replaces: stat and open through the root and never by path
  (`TestHandleArtFileRefusesSymlinkEscape`), and keep the stat before the
  open, which blocks on a FIFO. In `artfile_internal_test.go`, the header's
  "http.ServeFileFS guards a dirty path itself" is false under D2 and is
  reworded to `ServeMux` alone. The dated "http.ServeFileFS answered 301"
  passages in the doc comments of
  `TestHandleArtFileRefusesADirectoryWearingAnArtFilename` and
  `TestADirectoryWEARINGAnArtFilenameIs404` stay: they are history and still
  true, and converting them is not this ticket's.
- **D7. `ci.yml` installs 2.15.0, whatever O1 decides.** It is the one place
  the repository names the linter's version (G1). Left at 2.11.4, a
  dispatched CI run lints with the binary this ticket moves off.

## Open decision for the owner

**O1. Does the repository name the linter's version, and where?**

- **V0. Nothing.** The gate is whatever binary is first on `PATH`. This
  ticket exists because that changed without anyone choosing it: 2.15.0
  arrived in `~/go/bin`, a zsh launch picked it, and then Homebrew's 2.11.4
  disappeared. The next `go install` or upgrade does the same. D7 still
  applies.
- **V1. `go.mod`'s `tool` block** (`go get -tool
  github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`; `task lint`
  calls `go tool golangci-lint`, and the hook follows because it calls `task
  lint`). It is ADR-010 point 8's mechanism, the one `buf`, `gremlins` and
  `protoc-gen-go` use. Probed in a copy: `go.sum` takes in the linter's
  whole dependency graph, and minimum version selection raises the
  product's own dependencies. `google.golang.org/protobuf` moves from
  v1.36.11 to v1.36.12, and `golang.org/x/net`, `x/sys`, `x/text`,
  `x/crypto` and `x/tools` move with it. `go tool buf` does not build again
  until `go mod tidy` runs. `go tool protoc-gen-go`, which `buf.gen.yaml`
  calls, then reports v1.36.12, so the contract's committed
  `protoc-gen-go v1.36.11` headers stop matching `task generate:contract`, and
  `check:drift` stays red until the contract is regenerated. Every later
  bump of the linter moves product dependencies again. The first `go tool`
  run after a `go clean -cache` compiles the linter, and the pre-commit hook
  waits for it.
- **V2. A version check in `task lint`.** A Taskfile variable names 2.15.0.
  The lint task's first command compares `golangci-lint version --short` to
  it and refuses with the `go install
  github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0` line. `ci.yml`'s
  install step reads the same variable, or is bumped with a pointer to it.
  The hook needs no change. It costs a short shell block, and an install by
  hand on each machine. No product dependency moves. A different binary
  fails loudly instead of linting differently. It is a gate change, so it
  gets its own commit and review (rule 2). It meets ADR-010 point 8's
  principle, that a gate's tool must be reproducible, by naming and checking
  the version instead of by that point's mechanism, and its commit says so.

**Recommendation: V2.** It closes the failure this ticket came from, and it
does not pull the linter's dependency graph into the product's build or the
contract's generated headers.

## The requirements sort

Accepted:

- **R1.** An installed art file that the server can stat but cannot open is
  answered 404 with the route's one body, `gateway: no such art`. Breaks on
  today's tree, which answers 403 `403 Forbidden`; held by D3's test.
- **R2, only if O1 is V2.** `task lint` refuses to lint when the
  `golangci-lint` on `PATH` is not the version `Taskfile.yml` names, and prints
  the command that installs it. Breaks with a `PATH` shim that reports
  another version. Verified by READING (Phase 3's break, Phase 4b), as the
  register already records for rules no test runs.

Rejected:

- "A serve that ended in an error other than `http.ErrServerClosed` is
  reported at shutdown" (D1): no test can produce it without racing a
  listener failure against a signal; near-theoretical (2026-10-21).
- "A FIFO wearing an art name is refused without being opened": D2 keeps
  today's order and adds nothing to it. SPEC-012's "requires a regular file"
  already covers it in prose, and a row for it would be a different ticket's.
- "The repository lints clean under 2.15.0": that is the gate itself (item 1),
  not a rule the product holds.

## Tasks

0. **The ids.** After sign-off, `requirement-id` allocates R1 (and R2 under
   V2), and the rows go into `docs/requirements.md` naming their evidence.
   Done when the register holds them.
1. **The errcheck sites (D1).** `cmd/vtt/serve.go` (`newServeCmd`'s shutdown
   branch) and `cmd/vtt/harness_boot.go` (`bootSelfContained`'s close
   function). Done when `golangci-lint run ./cmd/vtt/...` reports neither
   errcheck nor staticcheck; `TestServeSubprocessExitsCleanlyOnSIGTERM` and
   `TestScenarioLibraryRunsSelfContained` pass. Phase 3 breaks: remove each
   site's filter, and that site's test goes red; restore the bare receive, and
   errcheck goes red. Phase 4b reviews this diff on its own; it is not
   committed alone (D5).
2. **The art route (D2, D3, D6).** `internal/gateway/metadata.go`
   (`handleArtFile`, `artContentTypes`' comment), `internal/gateway/metadata_test.go`
   (R1's test, first, red on today's tree), `internal/gateway/artfile_internal_test.go`
   (the header sentence). Done when `golangci-lint run ./internal/gateway/...`
   reports nothing; `go test ./internal/gateway` passes, with item 3's tests
   unchanged (`TestHandleArtFileRefusesSymlinkEscape`,
   `TestArtPictureGetsItsRealContentTypeInline`,
   `TestArtSidecarIsOctetStreamAttachment`,
   `TestArtIsSentWithCacheControlSoAnOverwriteReachesTheBrowser` among them);
   and `check-comments` puts no touched file above its ceiling. Phase 3
   breaks: `root.Open(name)` swapped for an open by path, and G703 goes
   red; the failed open answered with anything but the route's 404, and R1's
   test goes red. Then Phase 4a, then 4b.
3. **The commit (D5).** Tasks 1 and 2, QA's passing tests, the ticket (G1
   corrected by its writer) and this plan. Done when `task lint` passes under
   2.15.0 (item 1), `git diff eb0fa35 -- .golangci.yml` is empty and the
   branch adds no `//nolint` (item 2).
4. **The version (O1, D7).** `.github/workflows/ci.yml` under every option.
   Under V2, also `Taskfile.yml`'s lint task, including the comment's "a
   version check nobody would write" clause. Under V1, `go.mod`, `go.sum`,
   the lint task's call, and the regenerated contract instead. Its own
   commit and review. Done (V2) when `task lint` passes with 2.15.0, refuses
   with a `PATH` shim reporting another version and prints the install line,
   and `ci.yml` installs the version the Taskfile names.
5. **SPEC-012 (D4).** After the code it describes. Done when every call the
   art paragraph names is one `handleArtFile` makes, found by grep against
   `metadata.go`, and its Requirements list carries R1's id.
6. **The report.** `docs/reports/2026-11-12-the-repository-lints-clean-under-golangci-lint-2-15.md`,
   per the `implementation-report` skill, against the ticket's "Done looks
   like", with Phase 4a's adjudications under their own heading.
7. **The gate.** `task check` on the final tree, launched through `/bin/sh`
   (item 5). Push and merge on the owner's word.

## Phase 4a and 4b

- **4a runs for Task 2**, because SPEC-012's art paragraph as amended, and
  R1, describe what it delivers. One QA agent, handed that paragraph, R1,
  the rows SPEC-012 lists for the route (VTT-121 to VTT-131 and VTT-581), and
  `go doc ./internal/gateway` for building a `Server` (`New`, `WithArtDir`,
  `Handler`). Never `metadata.go`, the tests or the diff.
- **4a is skipped for Task 1.** No specification sentence describes what it
  delivers: SPEC-011 does not move, and the only behaviour that changes is
  D1's near-theoretical path, which gets no test. The owner signs this skip
  with the plan. It is also skipped for Task 4 (a build-script edit) and
  Tasks 5 and 6 (prose).
- **4b, one reviewer per task, claims checked by command:** S1005 on
  `_ = <-serveErrCh`; G703 cleared by D2 and fired by an open by path; the
  403-to-404 change, and the unchanged HEAD, `Range` and conditional
  answers, against `eb0fa35`; each SPEC-012 sentence D4 rewrites, run again
  against the handler; the comment shares. After the last commit, one
  reviewer reads the seams between the commits.
