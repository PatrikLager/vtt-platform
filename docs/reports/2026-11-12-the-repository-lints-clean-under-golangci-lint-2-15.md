# The repository lints clean under golangci-lint 2.15, with no finding silenced — report

Ticket: `docs/superpowers/specs/2026-11-12-the-repository-lints-clean-under-golangci-lint-2-15-design.md`.
Plan: `docs/superpowers/plans/2026-11-12-the-repository-lints-clean-under-golangci-lint-2-15.md`.
Branch `fix/lint-under-golangci-2-15` from main `eb0fa35`. Commits: `59c49ba`
(the code, the ticket, the plan and the register, in one commit because the
pre-commit hook lints the whole tree), `4a13749` (`ci.yml`'s pin and the
lint task's comment). Every code reference below is to `59c49ba`, the last
commit that changes code.

## Done looks like

1. [x] **Clean under 2.15.0.** `golangci-lint config verify` and
   `golangci-lint run ./...`, with golangci-lint 2.15.0 launched through
   `/bin/sh`, report nothing on `59c49ba`; on `eb0fa35` the run reports the
   errcheck findings in `cmd/vtt/serve.go` and `cmd/vtt/harness_boot.go` and
   gosec G703 in `internal/gateway/metadata.go`. The pre-commit hook's lint
   step passed on `59c49ba`'s commit.
2. [x] **No finding silenced.** `git diff eb0fa35 -- .golangci.yml` is
   empty, and the branch adds no `//nolint` and no `#nosec`.
3. [x] **The art route answers as before, but for the answer the rule below
   changes.** `handleArtFile` stats and opens through the art directory's
   `os.Root` and serves with `http.ServeContent`. An installed art file the
   server can stat and cannot open is answered 404 with `gateway: no such
   art` (VTT-682), where net/http answered a refused permission with 403.
   The tests the route's rows name (VTT-121 to VTT-131, VTT-581, and
   VTT-117, which the route shares with the other metadata routes) pass
   unchanged, `TestHandleArtFileRefusesSymlinkEscape` and the tests of
   the content type, `nosniff`, `no-cache` and the 304 among them. VTT-682
   is held by `internal/gateway/metadata_test.go`'s
   `TestAnArtFileTheServerCannotReadIs404` and
   `internal/gateway/qa_art_root_test.go`'s
   `TestQAArtRootAnArtFileTheServerCanStatButNotReadIs404WithTheOneBody`.
   QA's tests, run on `eb0fa35` as well, failed there only on VTT-682, and
   both VTT-682 tests are red on `eb0fa35`.
4. [x] **The servers shut down as before.**
   `TestServeSubprocessExitsCleanlyOnSIGTERM` and
   `TestScenarioLibraryRunsSelfContained` pass unchanged; each goes red when
   its site's `http.ErrServerClosed` filter is removed.
5. [x] **`task check` exits 0.** `task check`, launched through `/bin/sh` on
   `4a13749` with golangci-lint 2.15.0 from `~/go/bin` first on `PATH`,
   exited 0. Its lint step printed `0 issues.`, and the Go mutation gate
   reported no unadjudicated survivor, with mutants NOT EVALUATED and
   counted as killed (below). The client's mutation was not judged: the
   worktree holds no stored report (below).

## What the rules became

- An art file the server can stat and cannot open is answered 404 with the
  route's one body: **VTT-682**. It was dispensed against the register of
  `feat/campaign-data-fits-its-ruleset`, which had committed ids up to
  VTT-681 from the same base, and the row was carried into this branch.
  That branch's own register does not hold it: the dispenser there would
  hand out VTT-682 again, so that branch takes this branch's register, by
  merging it or `main` after it, before it dispenses another id.
- The plan's R2 (a version check in `task lint`) was not drawn: the owner
  answered the plan's open decision with no version named beyond `ci.yml`.

## Phase 4a adjudications

QA was handed SPEC-012's art-route paragraph as it stood, with what the
change does to it, the rows the route holds (VTT-121 to VTT-131, VTT-581)
and VTT-682, and the gateway's constructors. Run against `eb0fa35` as well,
only its VTT-682 test failed. One of its tests,
`TestQAArtRootArtIsAnsweredToEveryRole`, cites VTT-117, a row it was not
handed, and asserts that row's clause on the art.

- An unsatisfiable `Range` is answered by net/http with 416, before and
  after. SPEC-012's "every refusal is 404" is read as the route's own
  refusals; the SPEC-012 update that follows this report says so.
- `HEAD`, `Range`, `If-Range` and multi-range requests are in no sentence
  or row: answered as net/http answers them, unchanged. QA's tests of them
  hold no row; the ids it had given them as the nearest rows were removed,
  since no row states what they assert.
- A symlink inside the art directory to another file inside it is served,
  as before: `os.Root` follows a link that stays inside.
- A path with two segments or a literal `..` gets the mux's own 404 body,
  as before: the one-body sentence is the route's.
- A refusal carries no `Content-Disposition`, as before.
- The sentences QA asked rows for (a refusal's `nosniff` and no
  `Cache-Control`, a FIFO wearing an art name, `Last-Modified` as the
  file's time, the bearer's 401 body, the token never in the URL) predate
  this ticket, which changes none of them.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Task 4 under the owner's answer (no version named): `ci.yml` alone | `4a13749` also rewrites the lint task's comment in `Taskfile.yml` | The comment said `config verify` fetches its schema over the network and that a hidden `--schema` flag reads a local one. From 2.12 the schema is built into the binary, and 2.15.0 refuses `--schema` as an unknown flag, so both sentences were false under the linter the gate now runs (golangci-lint's changelog, v2.12.0: "Embed the JSON schema in the binary"). The retry loop is unchanged: removing it is a gate change of its own. |
| Phase 4a: QA handed SPEC-012's art paragraph as amended | Handed the paragraph as it stood, with what the change does to it, and VTT-682 | The amendment is written after this report, below; the paragraph as amended did not exist when QA ran. |
| The plan's Task 5 (SPEC-012) before Task 6 (the report) | The report first, then SPEC-012 | The `implementation-report` skill and the dev-cycle's Phase 5 put the record after the report it is written from. |

## What could not be established

- What a later linter will report: nothing in the repository checks which
  `golangci-lint` runs, by the owner's choice. A change on the machine
  changes the gate unseen, as it did here.
- Whether the mutants the Go mutation gate reported NOT EVALUATED would be
  killed: they timed out in `internal/perceive`, `internal/sight` and
  `internal/mcp`, in no file this branch changes, and the gate counts them
  as killed.
- The client's mutation on this branch: `task check` found no stored report
  in the worktree and judged nothing. `git diff eb0fa35 -- client` is empty,
  so the full run on `eb0fa35` remains the client's last measurement.

## What was deliberately left out

- Naming the linter's version beyond `ci.yml` (the owner, 2026-11-12).
- The rows QA asked for on sentences of SPEC-012 this ticket does not
  change.
