# SPEC-019: A campaign directory has one writer

## Status

Accepted. Implemented by `internal/campaign/campaign.go` (`EnsureDir`,
`ErrHeld`, `Open`, `takeHold` and `Close`) and by `cmd/vtt/invite.go`,
`revoke.go` and `joinlink.go`'s `withIdentity`, which open no `Campaign`.
Pinned by `internal/campaign/writer_hold_test.go`,
`internal/campaign/qa_writer_hold_test.go`, `cmd/vtt/writer_hold_test.go` and
`cmd/vtt/qa_writer_hold_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: every event is judged against the whole log before it, so only one
judge may append at a time.

## How it works

**The hold is an exclusive `flock` on the campaign directory.**
`campaign.Open` first calls `EnsureDir`, which refuses a path that is a plain
file and creates a missing directory with mode `0o750`. It then takes the
hold: `takeHold` opens the directory itself with `syscall.Open(dir,
O_RDONLY|O_CLOEXEC, 0)` and calls `syscall.Flock` on that descriptor with
`LOCK_EX|LOCK_NB`. Only then does `Open` open the log through `store.Open`
and fold it, so an opener that is refused never opens the log. Nothing is
created in the directory for the hold.

**A second opener is refused at once.** While one `Campaign` holds a
directory, an `Open` of it, in the same process or another, returns without
waiting an error that wraps `campaign.ErrHeld` and names the directory:
`campaign: another writer holds the campaign directory <dir>; a campaign has
one writer at a time, so stop the vtt serve or close the Campaign that has it
open`. `vtt serve` prints it after `vtt serve: open campaign: `, exits 1 and
listens on nothing. Any other failure to open the directory or lock it is
returned as `campaign: take the writer hold on <dir>: <err>`, and that `Open`
fails too.

**When the hold ends.** `Close` closes the log and then the directory's
descriptor; a second `Close` returns nil. An `Open` that fails after taking
the hold, in `store.Open` or in the fold, releases it before it returns. The
kernel releases it when the holding process exits or is killed. `O_CLOEXEC`
keeps the descriptor out of every process the holder starts, so a child that
outlives `Close` holds nothing.

**Who takes it.** Every `Campaign`, and outside the tests only
`composeServer` opens one (`grep -rn 'campaign\.Open('` over `cmd`,
`internal` and `tools`, test files aside): for `vtt serve`, and for `vtt
client run` and `vtt client soak` in self-contained mode, given neither
`--server` nor `--tokens`, through `bootSelfContained` on a fresh temporary
directory.

**Who does not.** `vtt invite`, `vtt revoke` and `vtt join-link` call
`campaign.EnsureDir` and then `identity.Open(campaign.LogPath(dir))`, so they
work beside a `vtt serve` on the same directory. `vtt art install` writes
into `art/` and opens no `Campaign`, and `mintInvites`, which
`bootSelfContained` runs, opens identity alone. Identity's handle on `log.db`
is its own (SPEC-009) and takes no hold.

## Consequences

- The hold binds `campaign.Open` and nothing else. A process that writes
  `log.db` without a `Campaign` is not stopped, and the log it leaves may not
  fold; a seat fed it withholds from the first event that does not fold
  (SPEC-015), and the next `Open` refuses the log.
- The hold is unix only: `syscall.Flock` does not exist for `GOOS=windows`,
  so `internal/campaign` does not build there.
- A filesystem on which `flock` fails with anything but `EWOULDBLOCK`
  refuses every `Open` of a campaign on it.
- `invite`, `revoke` and `join-link` neither read nor fold the log. A
  directory one of them creates holds identity's tables and no `events` table
  until a `Campaign` opens it, and they work on a campaign whose log does not
  fold.
- A test that opens a directory again must first close the `Campaign` that
  holds it.

## Requirements

VTT-267, VTT-268, VTT-269, VTT-270, VTT-271.
