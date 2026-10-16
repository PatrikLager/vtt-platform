# Each party member keeps its own log, written when the event is appended

## The problem

The campaign's log is one table. `internal/store/store.go` creates `events`
and nothing else, and `campaign.Append` and `campaign.AppendBatch` write each
event there alone. What a player or a spectator is sent of an event is decided
after the append, per connection: the gateway's `Projector` hands
`perceive.Projection.Event` the viewer's fold and the world (SPEC-016). So
nothing records what a character perceived. No table holds it, `Campaign`
keeps no state per character, and a character's experience exists only while
some connection projects it.

The per-character-logs design,
`docs/superpowers/specs/2026-08-30-per-character-logs-design.md`, makes that
record. Every party member gets a log of what it perceived (§3.2). The log is
written inside `campaign.Append` and `campaign.AppendBatch`, in the
transaction that writes the world event (§4.1, §4.2). Each entry is numbered
by the log's own sequence (§5), and the log folds standalone at every prefix
(§6).

The owner ruled on 2026-10-15 that a log which opens late opens with the
present: the open session, the party and the public notes, then what the
character experiences. On 2026-10-16 he ruled further:

- An anchored narration is re-anchored onto the entries of that log, not
  dropped.
- A log pauses while its actor is not a party member, and resumes without a
  second opening. A resume hides every token the paused board holds before
  it places what is in view.
- Frames that would not fold onto a log are not written to it.
- The cost of deciding at append time is accepted for now, and measured again
  after the third ticket.

On `main` at `4fe0472`, the tree stops that in four places:

- `.go-arch-lint.yml` lets `campaign` depend on `contract`, `store`, `engine`,
  `campaign` and `eventgen`. It does not let it depend on `perceive`, where
  the decision now lives.
- `internal/store` has no second table, and no write that spans two.
- Every frame `Projection.Event` returns carries the world sequence of its
  cause: `transitions` stamps `Sequence: seq`, and `forwardable` clones the
  envelope.
  `NarrationAdded`'s `anchor_from_seq` and `anchor_to_seq` also name world
  sequences, and `engine.Apply` refuses a narration whose `anchor_to_seq` is
  not before its own sequence. So a frame copied as written into a log with
  its own numbering does not always fold.
- `engine.Apply` refuses a `SessionEnded` when no session is open ("no open
  session to end"), and a `SessionStarted` while one is open ("session already
  open"). `Projection.Perch` introduces no session. So a log that opens or
  resumes holding only what `Perch` gives can fail to fold at the next session
  event.

How MapTool solves this, per CLAUDE.md rule 9, is answered in the section
"Rule 9: how MapTool solves this" of
`docs/superpowers/plans/2026-10-16-each-party-member-keeps-its-own-log.md`.

## Done looks like

1. **A character's log is stored beside the world's.** `internal/store` has a
   table `character_events` next to `events`. A new store test,
   `TestACharacterLogNumbersItsEntriesFromOne`, appends to two actors' logs
   and reads each back. It shows that each log numbers its entries from 1
   without gaps, apart from the other log and from the world's sequence. It
   cannot pass on `main`, which has no such table.
2. **The world event and its entries land together or not at all.** A new
   store test, `TestAFailedCharacterWriteLeavesNoWorldEvent`, makes a
   character insert fail after the world row is written. It shows that the
   world log and every character log are left as they were, and that no
   envelope keeps a stamped sequence.
3. **A failed append changes no character's memory.** A new campaign test,
   `TestAFailedFanOutDoesNotAdvanceACharactersMemory`, makes the store write
   fail for a move that would bring an actor into a party member's sight.
   An event on that actor appended next gives the log nothing, and the same
   move appended again introduces what the failed one would have. Every log
   still folds.
4. **After its opening, a log holds what the decision gives that character.**
   A new test, `TestACharacterLogIsWhatASpectatorPerchedOnItIsSent`, runs
   seeds of `internal/eventgen`, the scenario corpus and histories that
   pause and resume a log. For every party member, it seeds a gateway
   `Projector` for a spectator perched on that actor with the log's entries
   so far, at its opening and again at each resume. At every event after
   which the actor is a party member, the log's entries for that event,
   after any opening or resume entries, match that `Projector`'s frames
   envelope for envelope. Two things may differ:
   the sequence each entry carries, and a `NarrationAdded`'s anchor range
   (item 6). At every other event the log has no entry.
5. **A log opens with the present, pauses, and resumes.** New campaign tests
   cover six cases:
   - `TestANonPartyActorHasNoLog`: an actor that is never a party member has
     an empty log.
   - `TestALateLogOpensWithThePresent`: a party member added while a session
     is open gets a log that opens with that session's `SessionStarted` (same
     session id), the party and the public notes. The log holds no narration
     appended before it opened.
   - `TestALogOpensWhenAGrantMakesAPartyMember`: an actor that a control grant
     turns into a party member gets a log that opens at that grant.
   - `TestALogOpensForAPartyMemberThatHadNone`: a campaign whose world log
     holds a party member and whose `character_events` holds nothing for it
     opens that log at the next append, with the present.
   - `TestALogPausesWhileItsActorIsNotAPartyMember`: a grant sets the kind
     back, events follow, and a second grant makes the actor a party member
     again. The log gets nothing between the two grants and no second
     opening. After it resumes it holds the world's open session, every
     token it holds stands on its square in the world, and it folds. The
     same holds when, during the pause, a non-party actor the paused board
     held is removed and its id is reused for another actor.
   - `TestANewActorUnderAnOldIdContinuesItsLog`: a party member is removed,
     and a different party member is added under its id. The log continues.
     The old actor is withdrawn before the new one is introduced, and the log
     folds.
6. **An anchored narration names its own log's entries.** A new test,
   `TestAnAnchoredNarrationNamesTheEntriesOfItsOwnLog`, checks a narration's
   range in a character log. The range runs from the first to the last of
   that log's entries that the anchored world events caused, leaving out
   opening entries. When none of those events reached the log, the narration
   is unanchored.
7. **Every character log folds standalone at every prefix.** A new test,
   `TestEveryCharacterLogFoldsAtEveryPrefix`, runs `campaign.FoldPrefix` on
   every prefix of every character log, over `internal/eventgen` seeds and
   the corpus. It carries a guard that fails the run unless all of these
   hold:
   - at least two logs are non-empty;
   - at least one log holds an anchored `NarrationAdded`;
   - at least one log holds a `SessionEnded` for a session that was already
     open when that log opened.
8. **Frames that would not fold are not written.** A new campaign test,
   `TestFramesThatWouldNotFoldAreNotWritten`, makes the frames for one
   character fail to fold onto its log. The world event and every other log
   are written, and that log gets nothing for the event. The next event that
   reaches that character is written to its log, and the log folds.
9. **Reopening continues each log and introduces nothing again.** A new
   campaign test, `TestACharacterLogContinuesAfterReopen`, appends, closes
   and reopens the campaign, then appends again. Each log reads back
   unchanged, continues from its last sequence, and receives no introduction
   of anything it already holds. A second new test,
   `TestOpenRefusesACharacterLogThatDoesNotFold`, writes an entry that does
   not fold onto a log of a closed campaign, and `Open` then fails naming
   that actor.
10. **`campaign` may use the decision, and the decision still uses nothing
    more.** `.go-arch-lint.yml` lets `campaign` depend on `perceive`.
    `perceive` keeps its allowance of `contract`, `engine`, `sight` and
    itself.
    `TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight`
    and `check:arch` are green, unchanged.
11. **No connection's stream changes.** `git diff --stat main --
    scenarios/goldens` prints nothing, and
    `TestTheProjectedGoldensAreWhatTheProjectionActuallySends` passes.
12. **The gate holds the new code.** `task check` is green, whole.
    `internal/store` and `internal/campaign` stay in the Go mutation gate, and
    no floor in `tools/coverage-thresholds.txt` goes down.

## Rules this puts on the system

- A world event and every character entry it causes are persisted in one
  transaction, or none of them is.
- An event is written to an actor's log only when the actor is a party member
  in the world after that event. An actor that is never a party member has no
  log.
- A log opens before the frames of the first event after which its actor is
  a party member while its log is empty. It opens with the present as the
  world stood before that event: the open session, the party and the public
  notes. It holds no entry forwarded from an event appended earlier.
- A paused log resumes without a second opening. Its first entries bring it
  to the world's open session, then hide every token its board holds, before
  the decision's frames.
- At an event after which its actor is a party member, a character's log
  receives, after any opening or resume entries, the frames the perception
  decision gives for that character's eyes, given that log's own fold and
  the world after the event, numbered and anchored as its own, unless they
  would not fold onto it. It receives no other entry for that event.
- When those frames would not fold onto the log, the log receives nothing for
  that event, and the world event and every other log are still written.
- Each character log numbers its entries by its own sequence, from 1, without
  gaps.
- An anchored narration in a character log names the entries of that log
  that the anchored world events caused, leaving out the opening. It is
  unanchored when there are none.
- Every character log folds standalone at every prefix.
- After a reopen, a character's memory is the fold of its log, and the next
  entries introduce nothing that log already holds.
- An append that fails changes no character's memory.
- Opening a campaign whose character log does not fold is refused, naming
  the actor.
- No character entry is changed or removed once written.

VTT-311 (what `perceive` imports) and VTT-312 (a decision started over from a
fold sends what one fed from the first event sends) are unchanged, and the
fan-out relies on both.

## What it touches

1. `.go-arch-lint.yml`, the `campaign` component's allowance, first. Nothing
   in `internal/campaign` that imports `perceive` passes `check:arch` before
   it.
2. `internal/store/store.go` and the store's tests: the table, a write that
   spans both tables in one transaction, and a read of one actor's log.
3. `internal/campaign/campaign.go` (`Open`, `rebuildLocked`, `Append`,
   `AppendBatch`), a new fan-out file beside it, and the campaign's tests.
4. `internal/perceive`, only if the log's opening needs something it does not
   export today.
5. `internal/gateway`, tests only, for item 4.
6. `docs/requirements.md`, and `tools/coverage-thresholds.txt`,
   `tools/mutation-equivalents.txt` and `tools/comment-ceilings.txt` as the
   new code needs.

That is more than two components: store, campaign, the arch file and gateway
tests. The owner split the arc into four tickets on 2026-10-15, and this is
the second. Store and campaign cannot be split from each other here, because
item 2's rule spans the store's transaction and the campaign's fold.

## Specifications this moves

docs/specifications/016-the-projection.md
New: the character log, what one holds, when it opens, and how an append writes it

## What could not be established

- **How item 8 forces a fold failure.** No history known today makes the
  decision's frames fail to fold onto a log: none did over 86 logs measured
  during verification. The test needs a way to make it happen, and the plan
  chooses that way.
- **The cost after the third ticket.** The verification measured about 35 ms
  per party member per append on a dense 60x60 map. The owner accepted that
  for now. It has not been measured with the gateway's own projection gone.
