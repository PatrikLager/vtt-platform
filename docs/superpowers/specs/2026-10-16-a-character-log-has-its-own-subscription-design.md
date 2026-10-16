# A character log can be subscribed to, from its own cursor

## The problem

Every party member's log is written when the event is appended (SPEC-021),
and nothing can follow one as it grows. `internal/store/subscribe.go` serves
one subscription, to the world's `events`: `SubscribeWithNoProgressTimeout`
reads the history after a cursor under the store lock, queues it, registers a
`subscriber` in `s.subs`, and `notifyLocked` queues every later event for
every registered subscriber. `campaign.Campaign` exposes that one
subscription (`Subscribe`, `SubscribeWithNoProgressTimeout`) and notifies
only world events, after the live apply (`appendLocked`). A character log can
only be read whole, by `Campaign.CharacterLog`, which reads the store at
every call. So a reader that wants one character's log as it grows has no
cursor to resume from and no way to be told of a new entry.

The owner ruled on 2026-10-16 that the third ticket of the per-character-logs
arc is split in two. This is the first half: a character log gets a
subscription of its own, in the store and the campaign, and nothing in the
gateway changes. The second half moves a seat onto it, generalises
`set_viewpoint`, announces a switch with a new server frame, moves the move
gate to the active character's eyes, and deletes the projection.

A subscriber's queue keys its dedupe on `env.Sequence <= sub.lastSeq`, and a
character log numbers its entries by its own sequence (VTT-321). One registry
for world events and character entries would compare numbers from different
logs, so an entry and a world event could each be dropped as already queued.

How MapTool solves this, per CLAUDE.md rule 9: the world's subscriber already
follows MapTool's per-connection queue, an unbounded queue drained by its own
sender with liveness judged by time rather than depth
(`store.SubscriberNoProgressTimeout`'s comment names
`net.rptools.clientserver`). MapTool has no per-character stream at all, so a
character subscription has no precedent there beyond that queue, which it
reuses.

## Done looks like

1. **A character's log streams from its own cursor.** A new store test,
   `TestACharacterSubscriptionDeliversItsLogFromTheCursorThenLive`, writes
   entries to two actors' logs, subscribes to one after a cursor, and reads
   that log's entries after the cursor, then each entry appended later, each
   once and in order, with the catch-up head the last entry queued before the
   subscription returned. It cannot compile on `main`, which has no such
   subscription.
2. **A subscription carries its own log and nothing else.** A new store test,
   `TestACharacterSubscriptionCarriesNoOtherLog`, subscribes to one actor's
   log and to the world, appends world events and entries for several actors
   whose sequences coincide, and shows each subscription received exactly its
   own log and the world subscription exactly the world.
3. **A subscription racing an append misses nothing and repeats nothing.** A
   new store test, `TestACharacterSubscriptionRacingAnAppendSeesEachEntryOnce`,
   subscribes while entries are being appended and notified, and shows every
   entry after the cursor arriving once.
4. **A log that does not exist yet can be subscribed to.** A new store test,
   `TestACharacterSubscriptionWaitsForALogThatHasNoEntries`, subscribes to an
   actor with no entries, gets the cursor back as the catch-up head and
   nothing else, and then receives the log's first entries when they are
   written.
5. **The campaign delivers what the fan-out writes.** A new campaign test,
   `TestACharacterSubscriptionCarriesWhatTheFanOutWrites`, subscribes to a
   party member's log, appends world events and a batch, and shows the
   subscription delivering exactly what `CharacterLog` then reads, in order.
   A failed append delivers nothing (`TestAFailedAppendNotifiesNoCharacter`).
6. **The campaign's other guards cover it.** A poisoned campaign refuses a
   character subscription, and closing the campaign ends every character
   subscription, so its channel closes
   (`TestAPoisonedCampaignRefusesACharacterSubscription`,
   `TestClosingTheCampaignEndsCharacterSubscriptions`).
7. **A stalled subscriber is let go as the world's is.** A new store test,
   `TestAStalledCharacterSubscriberIsDroppedAfterTheNoProgressTimeout`, shows a
   character subscription that is not read closes after its no-progress
   budget, while another reader of the same log keeps receiving.
8. **No connection changes.** `git diff --stat main -- internal/gateway
   scenarios` prints nothing.
9. **The gate holds the new code.** `task check` is green, whole, with
   `internal/store` and `internal/campaign` in the Go mutation gate and no
   floor in `tools/coverage-thresholds.txt` lowered.

## Rules this puts on the system

- A character log subscription delivers every entry of that log after its
  cursor, history then live, each once and in order.
- A character log subscription delivers no entry of another log and no world
  event.
- A character log subscription's catch-up head is the last entry it queued
  before it returned, or its cursor when there was none.
- An entry reaches a subscriber only after the append that wrote it is
  persisted and applied, and a failed append reaches none.
- A poisoned campaign refuses a character subscription, and closing the
  campaign ends every one.
- A character subscription that stops reading is dropped after its
  no-progress budget, and no other reader is affected.

## What it touches

1. `internal/store/subscribe.go`, `internal/store/store.go` (the `Store`
   struct and `Close`) and the store's tests: a registry per actor, a
   subscription to one actor's log, a notify for one actor's entry, and
   `Close` ending them.
2. `internal/campaign/campaign.go` and `internal/campaign/fanout.go`, and the
   campaign's tests: `SubscribeCharacter`, and the notify of each entry after
   the live apply.
3. `docs/requirements.md`, and `tools/coverage-thresholds.txt`,
   `tools/mutation-equivalents.txt` and `tools/comment-ceilings.txt` as the
   new code needs.

## Specifications this moves

docs/specifications/021-each-party-member-keeps-its-own-log.md

## What could not be established

- **Whether a world subscriber and a character subscriber should share one
  liveness budget per connection.** The second half gives one connection one
  subscription at a time, so this half gives each subscription its own, as
  the world's has. Not measured.
