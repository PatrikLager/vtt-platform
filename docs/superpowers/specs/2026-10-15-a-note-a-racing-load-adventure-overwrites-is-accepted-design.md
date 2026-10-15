# A note a racing load_adventure overwrites is accepted

## The problem

`handleCommand` in `internal/gateway/server.go` runs `authorize`, which takes
`st := s.campaign.State()`, and hands that snapshot to `handleLoadAdventure` in
`internal/gateway/adventure.go`. `handleLoadAdventure` runs
`adventure.Compile(adv, st)`, whose `checkCollisions` refuses an adventure that
declares a note key the snapshot holds, and then calls `campaign.AppendBatch`,
which folds the batch through `engine.Apply` under the campaign's lock. The
fold's `NoteUpserted` arm overwrites `st.Notes[nu.Key]`, whatever it held.
An `upsert_note` that another connection of the DM or the agent appends
between the two calls, on a key the adventure declares, is overwritten by the
adventure's note, which `Compile` makes `NOTE_VISIBILITY_PUBLIC`. One
connection cannot do this to itself: its read loop answers a command before it
reads the next (SPEC-013).

`docs/verification-debt.md` carries the overwrite under Open debt, in the entry
beginning "A `load_adventure` whose note key collides with a note upserted
after its snapshot", as work to be done, and SPEC-012's paragraph "The
note-key residue" ends by sending the reader there. The owner ruled on
2026-10-15 that the work is not done: the overwrite needs two connections that
may write notes acting within the same moment, on a key the adventure also
declares, and a table has either a human or an agent as its DM, which the
gateway does not enforce. Nothing in the tree records that ruling, so the entry
reads as open work.

The ruling departs, in this one case, from
`docs/superpowers/specs/2026-07-26-adventure-format-design.md`, whose
collisions are "checked against the live snapshot before the batch —
rejection, not overwrite". VTT-133 stays as it is:
`TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned`'s doc comment
already leaves the note-key race to the debt entry.

## Done looks like

1. The entry in `docs/verification-debt.md` beginning "A `load_adventure`
   whose note key collides" ends with a paragraph that closes it by the
   owner's ruling of 2026-10-15, names the ruling's ground, and says what
   closing it would need should a table ever need it. `grep -c '^\*\*Closed
   2026-10-15' docs/verification-debt.md` prints 1. The paragraph's content is
   held by Phase 4b's reading.
2. SPEC-012's paragraph "The note-key residue" says the overwrite is accepted
   by that ruling, and no longer says the debt file carries it under Open
   debt. `grep -n 'carries it under Open debt'
   docs/specifications/012-the-read-surface.md` prints nothing.
3. No code changes. `git diff --stat main` lists only files under `docs/`.
   The two comments that point at `docs/verification-debt.md`, above the
   `adventure.Compile` call in `handleLoadAdventure` and in the doc comment of
   `TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned` in
   `internal/gateway/adventure_test.go`, stay: the entry they point at stays in
   the file and still states the upsert.
4. `task check` whole is green.

## Rules this puts on the system

None. The ruling decides which work is not done; no observation of the running
code goes red when it is broken.

## What it touches

1. `docs/verification-debt.md`
2. `docs/specifications/012-the-read-surface.md`

## Specifications this moves

docs/specifications/012-the-read-surface.md

## What could not be established

- Whether a table has met the overwrite. No campaign log on this machine was
  searched for an `upsert_note` and a `load_adventure` batch on one key.
- Whether a plan is kept. A plan for the fix was verified and measured on
  2026-10-15 in a copy of `8217272` and is not committed; the closing paragraph
  keeps the one fact from it that a later fix would need.
