# One log per pair of eyes

**Sub-project 14.** The visibility arc was *one log, many views*. This is the
other answer: many logs, one apiece, written when the seeing happened.

**Prerequisite: sub-project 13**, retraction leaving the platform — SATISFIED.
The contract forgot how to retract in `59542e1`, and `campaign.Undo` went in
`133e896`. No `EventsRetracted` survives as live code; what remains in
`internal/campaign` and `internal/harness` are dated obituaries, plus one live
structural guard in those two packages — `internal/campaign/forward_only_test.go`
asserts that no EXPORTED method on `*Campaign` matches undo or retract, exported
being the word that test keeps and that reflection over a concrete type enforces.
`check:no-retraction` reports a `retract` identifier found in the source
extensions it scans, outside the enforcement sites its own EXEMPT list names;
its KNOWN LIMITS record what it does not read, YAML and shell among them, and
`undo` is not its needle.

Which leaves one gap, stated rather than implied: an UNEXPORTED
`func (c *Campaign) undoRange()` is outside the guard, because reflection does
not see it, and outside the gate, because `undo` is not the needle. Nothing
caught it when this was written; the rule binds because it is the rule.
Several of this design's decisions — carrying a world sequence as provenance,
above all — are only safe once nothing can delete by sequence number, and that
is now true rather than assumed.

---

## 1. Why this, and why now

**The reason is structural isolation, and it is worth being precise about what
that means.** Today a player's connection *could* carry the whole campaign; a
filter is what stops it. `internal/gateway/seat.go` says so outright — the
projection "IS NOT THE SECURITY BOUNDARY". Session zero's finding 14 was a
player reading a hidden goblin's position off the board, and the answer we
built is a filter that has to be correct on every event, for every viewer,
forever.

A separate log is correct by construction. There is nothing else on that
connection to leak.

**Why now.** Sub-project 12 chased a client freeze through four fix rounds, a
contract change and roughly a thousand lines before the cause turned out to be
one decision two arcs old: a synthesized introduction — an event that exists
only for one viewer and is not in the log at all — was stamped with a *log*
sequence number, which made it deletable by an operation that has authority
over the log and none over what a person saw. The codebase had already met this
exact failure once, for perch frames, and fixed it there by giving them a number
no retraction could name (`perchSequence`, whose comment records the identical
`"moved unknown token"`).

The lesson generalises past retraction: **a derived, per-viewer artifact must
not borrow an identifier that something else owns.** Give the artifact its own
log and its own sequence, and the whole class is gone.

**And a blind spot found on the way out, which this design closes.** Every
whole-session fold `internal/harness` performs runs on participant 0. The
scenario runner folds `history[sc.Participants[0].Name]` and says so in its own
comment; the soak run folds `soakObserverName`, which is `soakDM`; and `cmd/vtt`'s
golden capture takes `sc.Participants[0].Name`. Participant 0 is `dm` in all nine
files under `scenarios/`. So the machinery that drives a scripted session end to
end has never folded anything but the DM, and a character's catch-up that
degrades over a long session is invisible to it.

That is the bounded claim, and it took three attempts to write one that holds.
Until 2026-09-14 this paragraph said "no test in this repo has ever folded a
projected seat's stream", which was false when it was written. The first
correction said "no SCENARIO", which is false as well. All three of these are
true and were found by looking:

  - the gateway's own tests fold player and spectator seats;
  - `scenarios/goldens/*/projections/` holds player and spectator streams, which
    `walkKeystone` folds at every prefix and `client/test/projection-parity.test.ts`
    folds again through `client/src/fold.ts`;
  - `TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` walks
    `internal/eventgen` over six seeds and builds a DM, an agent and two player
    seats — the DM and agent arms are the identity projection, so the two player
    seats are the projected ones it folds.

The lesson is the one this repository already learned once: a sweeping negative
needs the search that bounds it, and narrowing a false claim by one word just
produces a smaller false claim. What survives here is a statement about
`internal/harness`, because `internal/harness` is what was actually measured.

---

## 2. Non-goals

**Rebuilding a character log.** §3.3 makes them authoritative. Regeneration is
not a feature held back for later; it is refused.

**Merging several characters into one stream.** §3.4 gives a connection exactly
one viewpoint at a time.

**Logs for actors nobody can look through.** A monster accumulates no history,
because no viewpoint will ever read it.

**Changing what "seeing" means.** `internal/sight` is untouched. Only *when* it
runs changes.

**Backfilling a character who joins mid-session.** Their log starts at join,
because they were not there before.

---

## 3. The model

### 3.1 The DM log is the world

There is no separate world log and DM log; they are the same record. It holds
where every character is and what they did, and every secret door, trap, monster
and unopened chest the party has not found. The DM's client folds it directly,
which is already true today — `projected(role)` is false for the DM and the
agent, and stays false.

### 3.2 A character log is what that character experienced

One log per character that can be a viewpoint — which this spec defines as an
actor whose kind makes it a party member, the same distinction `add_actor` and
the adventure format already carry since `d20e9f5`. A monster has no log because
no viewpoint will ever read it, and a monster promoted to a party member starts
a log at that moment, by §3.4's join rule and for the same reason.

The log contains what that character perceived, in the order they perceived it,
each entry carrying that log's own sequence.

Two kinds of entry land in it. A **forwarded event** — a world event the
character perceived, unchanged. And an **introduction** — the first time they
perceive a thing, its existence written down, so the log stands alone.

The introduction is the same synthesis the projection performs today, with the
decisive difference: it is a real entry with its own sequence, authored once,
owned by the log it sits in. Nothing borrows a number.

### 3.3 Authoritative, never regenerated

A character log is the record of what that character experienced. It is written
once and never recomputed, and it is not a function of the world log.

This is a deliberate refusal of a rebuild, and it costs something worth naming:
**a fan-out bug is permanent for the characters it touched.** Nothing can repair
the past because nothing derives it. Two consequences follow and both are
requirements, not advice. The perception decision must be a small pure function
tested hard (§8). And it must **fail closed**: when the answer is unclear, write
nothing. A character who missed a sighting can be told by the DM; a character
who was shown a secret cannot be un-shown, which is the whole premise of this
design.

What it buys is that the DM correcting their own record has no reach into what a
character experienced. Not because we forbid it, but because they are different
logs and nothing regenerates one from the other. The wall was in the wrong place
and the DM moves it; Asme still saw what Asme saw.

### 3.4 One viewpoint per connection, and the perch is how you choose it

Every connection has exactly one viewpoint: the world, for a DM or an agent, or
one character, for everyone else. The stream is that viewpoint's log. One log,
one cursor, always.

A player may own several characters and is active on exactly one at a time,
chosen by clicking its token — the shape RPTool uses, and worth borrowing.
Switching is the same operation as perching: empty, replay that log.

`set_viewpoint` already exists and is spectator-only. It generalises: any seat
selects a viewpoint it is entitled to, a spectator borrows a shoulder, a player
picks among their own. The authorisation table decides who may select what; the
mechanism is one.

---

## 4. Fan-out

### 4.1 Inside `campaign.Append`, in the same transaction

The event lands in the world log and in every perceiving character's log
together, or neither lands.

Atomicity is not tidiness here. Because character logs are authoritative and
never regenerated, a crash between the two writes would leave a permanent hole
in someone's memory that nothing can fill.

Two placements were considered and rejected. **In the gateway**, where the sight
logic already lives: fan-out would then depend on a connection existing, and a
character whose player is offline would accumulate nothing — contradicting the
fact that the character was in the room. **Asynchronously, tailing the world
log**: it decouples the write path and would allow re-deriving after a bug,
which is exactly what §3.3 refuses, and it costs a window where a character's
log is behind the world.

### 4.2 What gets written

For each character that can be a viewpoint, at append time:

1. Ask `internal/sight`, against the world state at that moment, whether this
   character perceives the event.
2. If yes, write any introductions the entry depends on and that this character
   has not already been shown, then the forwarded event.
3. If no, write nothing.

Order within the batch is load-bearing and already known: a scene before the
tokens in it, an actor before its token, a scene before its `SceneSeen`. Both
folds are strict about this and say so in the same words.

### 4.3 Where the memory of "already introduced" lives

In the character's own log. "What has Asme been shown" is the fold of Asme's
log, so the log is its own memory and needs no side table to contradict.

Held in memory per character, the way `campaign` already holds world state, and
rebuilt by folding on open.

---

## 5. Storage

A second table beside `events`:

```sql
CREATE TABLE character_events (
  actor_id    TEXT    NOT NULL,
  seq         INTEGER NOT NULL,
  world_seq   INTEGER NOT NULL,
  event_id    TEXT    NOT NULL,
  occurred_at TEXT    NOT NULL,
  payload     BLOB    NOT NULL,
  PRIMARY KEY (actor_id, seq)
);
```

`seq` is that character's own, from 1, and is the cursor a client resumes from.

`world_seq` is provenance: which world event caused this entry. It is never
folded and never travels on the wire, because a client folds by its own sequence
and has no use for it. It exists so a DM or a debugger can ask what caused an
entry — the question that became unanswerable when introductions borrowed
sequences.

**It is only safe because retraction is gone.** A world sequence carried
alongside a derived entry was precisely the arrangement that let an undo delete
something it did not own. As a column that nothing skips by, it is inert.

---

## 6. The invariant that replaces the keystone

Visibility spec §4.3 asserted
`fold(project(log, viewer)) == visibleState(fold(log), viewer)`. It goes, because
under §3.3 the two sides are not meant to agree: a rebuild would be a different
history, not a check on this one.

What replaces it is smaller and stronger where it matters:

> **Every character log folds cleanly, standalone, at every prefix.**

Nothing in a character's log may reference something that log never introduced.
It needs no world log, no second computation and no correspondence argument —
and it is exactly the property whose violation froze a client in sub-project 12.

---

## 7. What this deletes, and what it does not

**Deleted:** `internal/gateway/project.go`'s read-time projection; the seat's
projector, `pastResume` and the subscribe-from-zero rule; the two-sided
keystone.

`Explored` SURVIVES, and an earlier draft of this section said otherwise while
the next paragraph kept `fold.ts` unchanged — the two cannot both be true.
Terrain memory is still a state field populated by `SceneSeen`, and the client
still renders from it. What goes is computing it per viewer at delivery: the
`SceneSeen` entries are simply in the character's log, and folding them
populates `Explored` exactly as it does today.

**Kept, unchanged:** `internal/sight`. `engine.Apply` remains the only code that
changes state — CLAUDE.md rule 4 holds; one fold now runs over more than one
log. The client renderer. `client/src/fold.ts`, which folds a character log
exactly as it folds today's projected stream.

**Simplified:** reconnect becomes "replay my log from my cursor". The torn-batch
recovery, the resume-at-`seenSeq-1` dance and the shared-sequence batch problem
have nothing left to be about.

---

## 8. Testing

**The per-log invariant is the keystone**, run at every prefix of every
character log a scenario produces, in both languages.

**The perception decision is a pure function and is tested as one**: world state
plus event plus character in, perceive-or-not out. It is the piece §3.3 makes
unrepairable, so it carries the heaviest coverage in the arc, including the
fail-closed direction — an unclear answer writes nothing.

**A leak test that cannot pass vacuously.** §1 records what the harness has and
has not folded: every whole-session fold it performs runs on participant 0.
Every scenario here folds each character's log as well, and the corpus gains a
scenario in which a character is present for some of a session and absent for
the rest.

**Fan-out atomicity** is tested by killing the process between the two writes
and asserting neither landed.

---

## 9. What could go wrong

**A fan-out bug is permanent.** §3.3's accepted cost. Mitigated by the pure
function, the fail-closed rule, and the per-log invariant catching an
incoherent log the moment it is written rather than when a client folds it.

**Write amplification.** One event becomes one world write plus up to N
character writes, inside one transaction. A party is small; a scene load is the
large case and is already a batch.

**A character log grows without bound**, as the world log already does. Same
problem, more copies, and no new answer here.

**The introduction fold on every append** costs a fold per character per event
unless cached. It is cached, and the cache is rebuilt by folding on open — the
same shape `campaign` already uses for world state.

---

## 10. Exit criteria

1. A player's connection never carries an event their character did not
   perceive — demonstrated by the connection having no access to the world log
   at all, not by a filter passing its tests.
2. Every character log folds cleanly standalone, at every prefix, in both
   languages.
3. A character who joins mid-session has a log starting at join, and it folds.
4. A DM correcting the world by appending leaves every character log unchanged.
5. Switching active character replays that character's log and shows only what
   it contains.
6. A spectator perching on a character sees exactly that character's log.
7. `task check` green, both mutation gates included.
8. A cold reader given only this spec can say what a character's log contains
   and what it can never contain.
