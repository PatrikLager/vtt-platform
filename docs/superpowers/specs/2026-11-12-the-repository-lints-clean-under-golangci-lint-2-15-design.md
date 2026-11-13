# The repository lints clean under golangci-lint 2.15, with no finding silenced

## The problem

**The linter the gate runs has changed under the repository.** `task lint`,
a step of `task check`, and the pre-commit hook's lint step run
`golangci-lint config verify` and then `golangci-lint run ./...` with the
`golangci-lint` first on `PATH`; `.golangci.yml` is validated against the
schema of whatever binary runs (`Taskfile.yml`'s lint task explains why the
schema is fetched for the binary's own major.minor). The one place that pins
a version is `.github/workflows/ci.yml`'s "Install golangci-lint" step,
which installs v2.11.4, and CI is not run. The local gate has passed with
golangci-lint 2.11.4, installed by Homebrew. That installation is gone from
the machine; the one left is 2.15.0, in `~/go/bin`.

**Under 2.15.0, `main` (`eb0fa35`) has these findings.** `config verify`
passes, and `run ./...` reports:
- `errcheck` on `<-serveErrCh` in `cmd/vtt/serve.go`, in `newServeCmd`'s
  shutdown branch, which receives `ListenAndServe`'s return after
  `Shutdown` and `Close` and discards it;
- `errcheck` on `<-serveErrCh` in `cmd/vtt/harness_boot.go`, in the close
  function `bootSelfContained` returns, which receives `Serve`'s return
  after `Close` and discards it;
- `gosec` G703 ("Path traversal via taint analysis") on
  `http.ServeFileFS(w, r, fsys, name)` in `internal/gateway/metadata.go`'s
  `handleArtFile`, where `name` is the request's `{file}` path value. The
  handler serves only a name `artlib.IsArtFileName` accepts, from
  `root.FS()` of an `os.Root` opened on the art directory, which refuses a
  name that leaves the directory (`TestHandleArtFileRefusesSymlinkEscape`);
  the analysis does not see either.

So `task check` exits 1 at `lint` on any branch, and the pre-commit hook
refuses a commit that changes a Go file.

`.golangci.yml`'s exclusions are scoped and each carries its reason; the
repository's `//nolint` lines each name their linter and why. An exclusion
that cannot be justified in one sentence is not kept.

## Done looks like

1. `golangci-lint config verify` and `golangci-lint run ./...`, with
   golangci-lint 2.15.0, report nothing on the branch. On `eb0fa35` the run
   reports the findings above.
2. No finding is silenced by turning a linter off or widening an exclusion:
   `git diff eb0fa35 -- .golangci.yml` adds no exclusion, and any `//nolint`
   the branch adds names one linter and one rule and says in one sentence
   why the finding does not hold.
3. The art route answers as before, but for the answer the rule below
   changes: the tests that pin it pass unchanged,
   `TestHandleArtFileRefusesSymlinkEscape` among them, and so do the tests
   of the content type, `nosniff`, `no-cache` and the conditional request
   SPEC-012 describes.
4. `vtt serve` and `vtt client run` shut down as before: the tests of
   `newServeCmd`'s shutdown and of `bootSelfContained`'s close pass
   unchanged.
5. `task check` exits 0 on the branch, run with golangci-lint 2.15.0.

## Rules this puts on the system

One, under the plan's D2: an installed art file that the server can stat
and cannot open is answered 404 with the art route's one body, where
net/http answered it itself. The rules these places already hold stand: the
art route's confinement (SPEC-012) and the shutdown order (SPEC-011).

## What it touches

- `cmd/vtt/serve.go` (`newServeCmd`).
- `cmd/vtt/harness_boot.go` (`bootSelfContained`).
- `internal/gateway/metadata.go` (`handleArtFile`).
- `.github/workflows/ci.yml` ("Install golangci-lint").
- Whatever else records the linter's version, if the plan names one.

## Specifications this moves

- docs/specifications/012-the-read-surface.md

## What could not be established

- Whether gosec's taint analysis accepts any check in `handleArtFile` as
  clearing `name`, or whether G703 needs a `//nolint` with its reason, or a
  serve that takes no path (`http.ServeContent` over a file opened through
  the `os.Root`). The last changes what SPEC-012 says of the route.
- Whether the repository should name the linter's version, so that a change
  on the machine cannot change the gate unseen, and where: `go.mod`'s `tool`
  block, which already names `buf` and `gremlins`, or a version check in the
  lint task. That is the owner's to decide.
- What else 2.15 changes that this run cannot show: the run reports every
  finding (`.golangci.yml` sets no limit on their number), so these are all
  it has.
