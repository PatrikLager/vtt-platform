# SPEC-021: Each party member's experience is a log of its own, written when the event is appended

## Status

Accepted. Implemented by `internal/campaign/fanout.go`
(`CharacterEntry`, `characterLog`, `causedEntry`, `CharacterLog`, `eyesOf`,
`isPartyMember`, `loadCharacterLogsLocked`, `fanOutLocked`, `logIDs`,
`trial`, `appendEntries`, `reanchor`, `record`, `resume`, `resumeScan`,
`resumesOf`, `observe`, `restartDecisionsLocked`, `opening`, `openSession`)
and `internal/campaign/campaign.go` (`rebuildLocked`, `foldEvents`,
`Campaign.appendLocked`, `Campaign.SubscribeCharacter`, `Campaign.Close`),
over `internal/store/store.go`'s `character_events` table, `Entry`,
`CharacterEntry`, `AppendBatch`, `ReadCharacterAfter`, `readCharacterLocked`,
`ReadCharacterLogs`, `Store.chars` and `Close`,
`internal/store/subscribe.go`'s `subscriber` (`enqueue`, `pump`),
`SubscribeCharacter`, `NotifyCharacter`, `startSubscriber` and `enqueueAll`,
and over `perceive`'s `New`, `Resume`, `Event` and `Perch` (SPEC-016).
Pinned by `internal/store/character_log_test.go`,
`character_subscribe_test.go`, `qa_character_subscribe_test.go` and
`store_failure_test.go`,
`internal/campaign/character_log_test.go`,
`character_log_property_test.go`, `qa_character_log_test.go`,
`qa_pause_resume_test.go`, `property_test.go`,
`character_subscribe_test.go`, `qa_character_subscribe_test.go` and
`poison_internal_test.go`, by `internal/perceive/resume_from_fold_test.go`,
and by `internal/gateway/character_log_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The one
this record would cite: what a character perceived is recorded once, when it
happened, and nothing the world does afterwards reaches back into it.

## How it works

**What a character log is.** A party member (`engine.IsPartyMember`) gets a
log of its own in `character_events`, beside the world's `events`, from the
first event after which it is one; a campaign written before the table
existed opens each party member's log at its next append. Each entry carries
the log's own sequence, from 1, without gaps (VTT-321), and `world_seq`, the
world event at whose append it was written, or 0 for an entry no event
caused: an opening's or a resume's. `event_id` is not unique there. An entry
forwarded from the world keeps the world event's `event_id`, `session_id`
and `occurred_at`, so one world event seen by four characters is four
entries with one id. Every other entry carries none of the three, except
the `SessionStarted` of an opening or a resume, which carries the open
session's id. `Campaign.CharacterLog` reads one log from the store at every
call, so what it returns is the caller's.

**When a log is written.** `Campaign.appendLocked` validates the batch
(SPEC-015), then `fanOutLocked` folds each envelope onto its own copy of the
world and walks, by id, every actor that has a log or is a party member after
the envelope. Nothing is written to an actor that is not a party member after
the event (VTT-316). For the others it writes, in order, any opening or
resume entries, then the frames `perceive.Projection.Event` gives through the
eyes `Eyes{Shoulder: id}`, handed the log's own fold after those entries and
the world after the envelope (VTT-319). The world rows and every entry go to
the store in one `AppendBatch`, one transaction, or none of them does
(VTT-315). The working copies of the logs are committed only once the store
holds the entries; a failed write leaves them as they were and resumes every
active decision from its log and the world it is in
(`restartDecisionsLocked`, VTT-325).

**The opening.** A log opens before the frames of the first event after which
its actor is a party member while its log is empty (VTT-317). It opens with
the present as the world stood before that event: the open session's
`SessionStarted`, with that session's id and name; then the party, as
`Perch` through no eyes introduces it (SPEC-016); then a `NoteUpserted` for
each public note, by key. Nothing appended earlier is forwarded into it. Its
decision starts from nothing seen (`perceive.New(eyes, nil)`).

**A pause and a resume.** A log whose actor is not a party member after an
event, through a grant of another kind or the actor's removal, is paused: it
keeps what it has and is written nothing. When the actor is a party member
again, the log resumes without a second opening (VTT-318). Its first entries
end the session it holds open if the world, before the resuming event, holds
another or none, and start the world's open session, with its id and name, if
the log holds none or another; sessions that opened and closed during the
pause are not in the log. Then a `TokenHidden` for every token the log holds,
by id. Its decision starts from nothing seen, so the frames for the resuming
event place what it sees on its present square and correct each held actor
in sight. A new actor added under an old id continues that id's log, and is
withdrawn and introduced afresh, or corrected, as VTT-313 says.

**Frames that would not fold.** Each actor's frames are folded first onto a
copy of its log, each stamped with the sequence it will have. A set that does
not fold is not written, whole, while the world event and every other log
land; the decision then resumes from the log and the world after the event
(`perceive.Resume`), and `slog.Error` names the actor and the world sequence
(VTT-320).

**An anchored narration.** A `NarrationAdded` names its anchor in world
sequences. In a log, `reanchor` rewrites it onto the first and the last entry
of that log that a world event in the range caused, leaving out entries no
event caused; when none did, the narration is unanchored (VTT-322). When the
range spans a resume of the log, it names only what events at or after the
latest resume in the range caused, the resuming event's own frames included
(VTT-328). A resume's boundary is the world event that resumed the log. The
fan-out records it as it happens, and `Open` finds it with `resumeScan`, an
observer handed to the world's one fold, which records each event after
which a stored log's actor turns party member, the first included. A
reopened log therefore holds more resumes than the fan-out recorded, all at
or before its first caused entry, where none moves an anchor.

**The memory, and Open.** What the campaign holds of a log is its fold, its
last sequence, the decision that writes it (nil while paused), and the
caused entries and resumes the anchors are rewritten against. `Open` reads
every stored log, folds it, and refuses a log that does not fold, naming the
actor (VTT-327). An active log's decision resumes as
`perceive.Resume(eyes, fold, world)` (SPEC-016), so a reopen introduces
nothing the log holds (VTT-324). No entry is changed or removed once written
(VTT-326): `grep -rln 'character_events' --include='*.go' . | grep -v
_test.go` prints only `internal/store/store.go`, whose statements on the
table are its `CREATE TABLE`, the next-sequence query and the `INSERT` in
`Store.appendLocked`, and the two reads; none updates or deletes. Every log
folds standalone at every prefix (VTT-323).

**Following a log.** `Campaign.SubscribeCharacter(actorID, afterSeq,
buffer, noProgress)` follows one actor's log from a cursor in that log's own
sequence, as `SubscribeWithNoProgressTimeout` follows the world's, and
refuses while the campaign is poisoned (VTT-334). It holds the campaign's
mutex across `store.Store.SubscribeCharacter`, which reads the entries after
the cursor and registers the subscriber under one hold of the store's lock,
in a registry per actor beside the world's (`Store.chars`), so a
subscriber's dedupe compares the sequences of one log only (VTT-330). It
delivers the history, then each later entry, once and in order (VTT-329).
Its catch-up head is the last entry it queued before it returned, or the
cursor when it queued none (VTT-331); the cursor is taken unclamped, as the
world's subscription takes it. Any actor id can be followed, one with no
entries included. `appendLocked` delivers each entry it wrote through
`NotifyCharacter`, in the fan-out's order, after the world's envelopes and
after the live apply; a refused, failed or poisoning append delivers none
(VTT-332), and the mutex keeps a catch-up out of the window between the
store's commit and the live apply (VTT-333). An entry is accepted once the
subscriber's hand-off channel takes it; a subscription that leaves a waiting
entry unaccepted for its no-progress budget is closed, alone (VTT-336,
VTT-337), and each subscription has a budget of its own. Closing the
campaign closes the store, which ends every character subscription
(VTT-335). The world's subscription and this one share `startSubscriber` and
`enqueueAll`, which ignores an envelope of sequence 0.

**What this record does not decide.** What the decision gives an event is
SPEC-016's. The world's append, its validation and its poison contract are
`internal/campaign`'s, and one writer per campaign directory is SPEC-019's.
No connection reads or follows a character log (`grep -rn
'CharacterLog(\|ReadCharacter\|SubscribeCharacter' --include='*.go' . | grep
-v _test.go` prints only `internal/campaign` and `internal/store`): what a
player or a spectator is sent is the projection's (SPEC-016) until the second
half of the per-character-logs arc's third ticket
(`docs/superpowers/specs/2026-10-16-a-character-log-has-its-own-subscription-design.md`,
"The problem") moves a seat onto its character's log. Who may follow which
log is SPEC-013's, then.

## Consequences

Whoever works on the campaign, the store or a reader of character logs is
bound by these:

- A character log is never rebuilt from the world. A fan-out defect is
  permanent for the logs it touched, so the decision fails closed: when its
  frames would not fold, the character misses them.
- After a drop, the next event that lands corrects what came into or left
  sight, the visible squares, the doors in sight and the status of each actor
  in sight. A token that stays in sight keeps the square the log last held
  until its next forwarded move, nothing resends a note or a narration, and
  the next event can drop again while what the log lacks stays in sight.
- A session the log holds open after a drop, which the world has ended, makes
  the log miss the next `SessionStarted`: its fold keeps the old session open
  through the new one's events, though each of them carries the new
  session's id.
- A paused log misses every narration and every public note made during the
  pause, and every session that opened and closed in it.
- Entries of one log share `event_id`s with the world and with other logs;
  nothing may treat `character_events.event_id` as unique.
- An entry's `world_seq` is provenance and the anchors' key, never folded and
  never sent on the wire.
- Every append runs the decision once per party member and envelope, under
  the campaign's mutex, on top of the gateway's projection until the second
  half of the third ticket deletes it; the owner accepted that cost on
  2026-10-16 for now, to be measured
  again after the third ticket.
- A character subscription waits for an append in progress, its fan-out
  included, and an append, `State` and `CharacterLog` wait for a
  subscription's catch-up, read under the campaign's mutex.
- The store keeps a registry key, until it closes, for every actor id it
  subscribed and every actor id `NotifyCharacter` was called with; a call
  after `Close` makes one again.
- The mutation gate does not reach the registry per actor: one registry
  shared by every log is no mutant it makes, so VTT-330 is held by the tests
  in its row and nothing else.

## Requirements

VTT-315, VTT-316, VTT-317, VTT-318, VTT-319, VTT-320, VTT-321, VTT-322,
VTT-323, VTT-324, VTT-325, VTT-326, VTT-327, VTT-328, VTT-329, VTT-330,
VTT-331, VTT-332, VTT-333, VTT-334, VTT-335, VTT-336, VTT-337.
