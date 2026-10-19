# The window shows what happened, and no log number

## The problem

Every role sees the same window. `client/src/app.ts` calls `renderSpectator`
for every connection, with the player panel and the DM console added to it,
unless a fold failure replaces the window with its reason, and
`renderSpectator` in `client/src/view/spectator.ts` draws two accounts of what
happened. The "Story" section, `renderFeed`, shows text alone. The "Events"
section, `renderTicker`, begins each of its rows with a `span.seq` holding
`#${e.sequence}`: on a player's or a spectator's screen the sequence of the
character log they watch, on the DM's the world's. The ticker's number was a
decision of the client's first design,
`docs/superpowers/specs/2026-07-26-client-design.md` ("event ticker with
sequence numbers"), and this ticket overturns it.

The owner ruled on 2026-10-19, in three answers:

- "i själva 'fönstret' som visar vad som har hänt för spelarna och dm, så
  visas inget 'logg id/nummer' - utan det är bara texten";
- "det enda som gäller är att vi 'döljer' lognummer i fönsteret ... Vi
  behöver ha alla lognummer, internt, så vi kan spåra problem och vad som har
  hänt";
- "Nej en spelare ska inte se världsekvensen - för att en spelare inte ska
  kunna se från ett meddelande i chat fönsteret till ett annat att DM har
  gjort flera saker mellan de meddelanden som dyker upp i chatfönstret".

So the window shows what happened as text, and every log number stays inside
the engine and on the wire, where it is needed to trace a problem. A player's
or a spectator's ticker number is their character log's (SPEC-007), which
numbers what is written to that log rather than the world's events; the
ruling removes it all the same.

These answers narrow the ruling of 2026-10-17 on G11, which
`docs/reports/2026-10-17-a-seat-streams-its-characters-log.md` records as "a
player is not to be told world sequences" ("Exponera inte 'loggsiffran'"), to
the window. Asked whether the world sequence a player's connection does carry
should go too — `CommandResult.sequence`, which names the first world event
a command produced (SPEC-007) and is seen only in the raw frames, such as a
browser's developer tools — the owner answered "Bara fönstret": it stays, as
does every other sequence on the wire.

At sign-off the owner ruled one exception ("Låt det vara"): when the client
cannot fold its log, `app.ts` replaces the window with "Cannot derive state
from the log: " and the fold's reason, and two of `client/src/fold.ts`'s
reasons name a sequence ("session already open at sequence N", "session
ended with none open at sequence N"). That reason stays as it is.

Rule 9, how MapTool does this: its chat window,
`MessagePanel.addMessage`, renders a `TextMessage`'s text, and a `TextMessage`
carries a channel, a target, a source, the text and its transform history,
and no sequence. Every method of its `ServerCommand` returns `void`, so no
reply carries one either. The window here does what MapTool's does; the wire
keeps what MapTool has no equivalent of, a sequence per log.

## Done looks like

1. A new test in `client/test/spectator-view.test.ts` renders a log of
   several events and shows that each ticker row's text is exactly its
   event's `describe` text, newest first. It fails today, because each row
   begins with "#" and the entry's sequence.
2. The tests that state the number today are rewritten to the description
   alone, and pass: in `client/test/spectator-view.test.ts`, "the ticker
   shows sequence numbers, newest first", "each ticker row carries its
   sequence number and its description" and "the feed and the ticker show a
   move's reason" (VTT-263's evidence); in
   `client/test/qa-move-reason.test.ts`, `tickFor`, which finds a row by its
   "#N". "the ticker is bounded — an all-night session must not render
   thousands of rows" passes unchanged.
3. `client/e2e/table.spec.ts`'s "the story feed and ticker record what
   happened" passes and finds a ticker row with no "#". It fails today
   before it reaches the "#": its page opens as a spectator who watches
   nobody, and such a connection is sent no events (SPEC-015's default), so
   the ticker has no rows. That test predates the seat's character log and
   is not run by `task check`. Every other test `task e2e` runs passes.
4. Nothing outside the client's window moves: `git diff --stat main` names no
   file under `contract/`, `internal/` or `cmd/` other than `cmd/vtt/webdist`,
   rebuilt by `task build:client`. `CommandResult.sequence`,
   `Envelope.sequence`, `CatchUpHead` and every stored sequence are as they
   were.
5. `task check:drift` is green after the commit that rebuilds
   `cmd/vtt/webdist`.
6. SPEC-007 says that a client shows no sequence, except in the reason it
   gives for a log it cannot fold, and that a result's `sequence` names the
   first world event its command produced on every connection, a player's
   included, by the owner's rulings of 2026-10-19.
7. `task check` is green on the last code commit.

## Rules this puts on the system

- The window shows no log sequence number, to any role, except in the reason
  it gives for a log it cannot fold (the owner's ruling at sign-off).

That every sequence stays on the wire is not a new rule: the wire carries
them today, and this ticket changes nothing there.

## What it touches

In this order:

1. `client/src/view/spectator.ts` (`renderTicker`) and `client/src/style.css`
   (the `.seq` rule, if nothing else uses it).
2. `client/test/spectator-view.test.ts`, `client/test/qa-move-reason.test.ts`,
   `client/e2e/table.spec.ts`.
3. `cmd/vtt/webdist`, rebuilt by `task build:client`.
4. `docs/requirements.md`, for the rule's row.
5. `docs/specifications/007-the-wire-contract.md`.

One component, the client, and its embedded bundle; the record follows.

## Specifications this moves

docs/specifications/007-the-wire-contract.md

## What could not be established

- That no other view prints a log number. Of the hits `grep -n sequence`
  finds over `client/src/view/*.ts` and `client/src/app.ts`, only
  `renderTicker`'s span draws a number; the rest order or correlate.
  `describe` prints no sequence, and the status line's "sessions: N" counts
  sessions. A fold failure's reason, from `fold.ts` through `app.ts`, is a
  second path, and the exception above. A refusal is drawn as "refused: "
  and its error; of the error strings `grep -rn 'seq %d\|sequence %d'
  --include='*.go' internal cmd | grep -v _test.go` finds, the only ones a
  command's refusal carries are a narration anchor's two
  (`engine.Apply`'s `NarrationAdded` arm), and no control in
  the client sends an anchor, since `addNarration`'s one caller,
  `client/src/view/player.ts`, passes none. A number reached by another
  path, such as a tooltip built elsewhere, was not searched for.
- How long `task check` takes here. Any change to the client's sources or
  tests makes `check:ts-mutation` re-mutate the whole client in its Linux
  container, since the stored report is valid only for unchanged inputs
  (`tools/ts-mutation-inputs.py`).
