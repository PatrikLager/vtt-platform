# A campaign has one writer

## The problem

`campaign.Open` (`internal/campaign/campaign.go`) creates the campaign
directory if it is missing, opens its log through `store.Open`
(`internal/store/store.go`, SQLite with a 5-second `busy_timeout`), and folds
the whole log into an in-memory state; `Append` and `AppendBatch` fold each
new envelope against that state before they persist it. Nothing stops a
second `Campaign` on the same directory, in the same process or another:
`grep -rn -i 'flock\|lockfile\|O_EXCL'` over `internal/store`,
`internal/campaign` and `cmd/vtt` prints nothing. Two `Campaign`s each accept
an append the other would refuse, because each folds against its own state;
when the two conflict (two `SceneCreated` for one scene id), the log stops
folding, a projected seat is sent no further event, and the next
`campaign.Open` refuses the log, so the campaign no longer boots.
`docs/verification-debt.md` carries this as open debt with that recipe.
`vtt serve` opens a `Campaign` through `composeServer`
(`cmd/vtt/serve_compose.go`) and holds it while it serves; `vtt invite`,
`vtt revoke` and `vtt join-link` (`cmd/vtt/invite.go`, `revoke.go`,
`joinlink.go`) each call `campaign.Open` too, though they append no event:
they open it so the directory exists before `identity.Open` opens the
participants table in the same SQLite file (`campaign.LogPath`), and close it
on return. A DM who runs `vtt invite` while `vtt serve` runs therefore opens a
second `Campaign` on a served directory today, harmlessly, since it appends
nothing.

## Done looks like

1. While a `Campaign` is open on a directory, a second `campaign.Open` of that
   directory is refused with an error that names the directory and says
   another writer holds it, whether the first is in the same process or
   another: named tests under `internal/campaign/` fail on today's tree and
   pass after.
2. Closing the first `Campaign` lets the next `campaign.Open` succeed, and so
   does the death of the process that held it, killed without closing: named
   tests observe both.
3. `vtt invite`, `vtt revoke`, `vtt join-link` and `vtt art install` still work against a
   directory a running `vtt serve` holds, and against one nothing holds: a
   named test under `cmd/vtt/` runs each while a `Campaign` is open on the
   directory and passes before and after.
4. `vtt serve` on a directory another `vtt serve` holds exits with the
   refusal and serves nothing.
5. `docs/verification-debt.md`'s entry is closed by the test that holds item
   1, and `task check` whole is green.
6. A seat sent an event whose fold fails withholds it and every event after
   it (VTT-191): a named internal test under `internal/gateway/` observes it
   (the owner's ruling at sign-off), so the entry's other half closes too.

## Rules this puts on the system

- At most one `Campaign` is open on a campaign directory at a time.
- A second open of a held directory is refused, and says why.
- The hold ends when its holder closes or its process dies.
- A command that appends no event does not need the hold.

## What it touches

1. `internal/campaign/campaign.go` (`Open`, `Close`) and its tests
2. `cmd/vtt/invite.go`, `revoke.go`, `joinlink.go` and `serve_compose.go`
   (`composeServer`), and their tests
3. Any test that opens two `Campaign`s on one directory without closing the
   first, which the plan lists
4. A specification of the campaign directory's writer, new;
   `docs/specifications/015-the-seat-and-the-perch.md`, whose sentence that
   nothing locks a campaign directory becomes false; `README.md`;
   `docs/requirements.md`; `docs/verification-debt.md`;
   `tools/comment-ceilings.txt`
5. `internal/gateway/viewpoint_internal_test.go`, for VTT-191

Two components, the campaign package first, then the CLI that opens it.

## Specifications this moves

New: a campaign directory has one writer — what holds it, what a second open
is told, and which commands need the hold.
docs/specifications/015-the-seat-and-the-perch.md

## What could not be established

- The mechanism: an advisory `flock` on a file in the directory, an exclusive
  lock through SQLite, or a lock file created exclusively. Which one releases
  on a crash, which conflicts within one process as well as across processes,
  and which leaves `identity.Open` on the same SQLite file working, is the
  plan's to measure.
- Whether the identity commands stop opening a `Campaign` at all (a separate
  way to make the directory exist) or open it without the hold.
- Whether the harness (`cmd/vtt/harness_boot.go`) and the other `cmd/vtt`
  commands that touch a campaign directory need the hold or must not take it.
- Whether VTT-191, a seat withholding an event whose fold fails, gets the
  internal test the same debt entry names; it needs no second `Campaign` and
  is not this ticket's unless the plan says so.
