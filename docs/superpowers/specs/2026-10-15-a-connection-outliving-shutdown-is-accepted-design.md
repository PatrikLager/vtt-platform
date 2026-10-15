# A connection outliving vtt serve's shutdown is accepted

## The problem

`vtt serve`'s `RunE` in `cmd/vtt/serve.go`, on SIGINT or SIGTERM, calls
`srv.Shutdown` under `serveShutdownTimeout`, then `srv.Close`, and returns; its
deferred `closeFn`, which `composeServer` in `cmd/vtt/serve_compose.go`
returns, closes the identity handle and then the campaign.
`http.Server.Shutdown` does not wait for hijacked connections, which every
WebSocket connection is, so a connection still open when `RunE` returns runs
against those handles as they close. `Campaign.Close` takes the campaign's
lock, which `Append` and `AppendBatch` hold for the whole append, so an append
already under way finishes before the log closes. A command the connection
reads after the identity handle closes fails `Lookup` with an error that is not
`identity.ErrInvalidToken`, and `answerCommand` refuses it with "gateway:
identity unavailable" (VTT-033). A command that passed `Lookup` before then
and appends after `Campaign.Close` gets the error that `Begin` in
`store.Append` or `store.AppendBatch` returns on a closed database, and the
gateway answers it with `ok` false. An identity write that passed `Lookup` and
runs after the identity handle closes fails the same way. `main` returns once
`RunE` has, and the process exits with the connection in it.

`docs/verification-debt.md` carries the gap under Open debt, in the entry
beginning "A WebSocket connection can outlive `Shutdown`", as work to be done,
and SPEC-011's paragraph "Process shutdown does not wait for connections" ends
by sending the reader there. The owner ruled on 2026-10-15 that the work is not
done: the gap opens only while the server is stopping with a client connected,
and nothing a connection sends in it is half-written: it lands whole, is
refused, or is cut off by the exit before it reaches the log. Nothing in the
tree records that ruling, so the entry reads as open work.

Two comments in `cmd/vtt/serve.go` point at text that does not say what they
point at it for. The block in `newServeCmd`'s shutdown branch sends the reader
to `composeServer`'s doc comment for the hijacked connections, and that doc
comment does not mention them. `serveShutdownTimeout`'s doc comment sends the reader to that block for
why `Shutdown` alone cannot be trusted to finish, which the block does not
say.

## Done looks like

1. The entry in `docs/verification-debt.md` beginning "A WebSocket connection
   can outlive `Shutdown`" ends with a paragraph that closes it by the owner's
   ruling of 2026-10-15 and names the ruling's ground, including that the
   ground is a reading of the code and not a test. `grep -c '^\*\*Closed
   2026-10-15' docs/verification-debt.md` prints 2: this paragraph and the one
   closing the `load_adventure` entry. The paragraph's content is held by Phase
   4b's reading.
2. SPEC-011's paragraph "Process shutdown does not wait for connections" says
   the gap is accepted by that ruling, and no longer says it is recorded in the
   debt file as open work; its Consequences no longer wait for the debt to
   close. `grep -n -e 'gap is recorded in' -e 'until the debt above is closed'
   docs/specifications/011-the-connection.md` prints nothing, where on `main`
   it prints two lines.
3. No code changes but comments. `git diff --stat main` lists
   `cmd/vtt/serve.go`, `tools/comment-ceilings.txt` and files under `docs/`,
   and `git diff -U0 main -- cmd/vtt/serve.go | grep -E '^[+-][^+-]' | grep
   -vE '^[+-][[:space:]]*//'` prints nothing. The two comments that point at
   `docs/verification-debt.md`, on the deferred `closeFn` in `newServeCmd` and
   above `closeFn` in `composeServer`, stay: the entry they point at stays in
   the file and still states the gap, and the warning in `composeServer` still
   binds a caller whose process goes on running after `closeFn`.
4. Neither comment in `cmd/vtt/serve.go` named under The problem points at a
   comment that does not say what it is pointed at for. `grep -n -e
   "composeServer's doc comment" -e "RunE's own comment" cmd/vtt/serve.go`
   prints nothing, where on `main` it prints two lines, and `python3
   tools/check-comments.py main` ends clean.
5. `task check` whole is green.

## Rules this puts on the system

None. The ruling decides which work is not done; no observation of the running
code goes red when it is broken.

## What it touches

1. `docs/verification-debt.md`
2. `docs/specifications/011-the-connection.md`
3. `cmd/vtt/serve.go`, comments only
4. `tools/comment-ceilings.txt`, the row for `cmd/vtt/serve.go`

## Specifications this moves

docs/specifications/011-the-connection.md

## What could not be established

- What a connection does in the gap, by observation. No test holds a
  connection open across `Shutdown`; the ground above is a reading of
  `closeFn`, `Campaign.Close`, `store.Close`, `Lookup` and `answerCommand`.
- Whether `vtt client run` and `vtt client soak` in self-contained mode meet
  the gap. `bootSelfContained` in `cmd/vtt/harness_boot.go` returns a close
  that calls `srv.Close` and then `composeServer`'s `closeFn`; whether every
  harness connection has closed by then was not checked, and the debt entry
  does not name that path.
