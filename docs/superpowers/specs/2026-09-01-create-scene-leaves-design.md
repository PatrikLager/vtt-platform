# The kernel serves maps, it does not make them

**Sub-project 15.** `create_scene` leaves the platform. A campaign owns its maps,
a map is installed by putting a file in a directory, and `load_map` brings it
into play.

**Follows sub-project 13** (`2026-08-30-retraction-leaves-design.md`), which
removed retraction and — in requiring `create_scene` to declare complete terrain
— exposed what `create_scene` is.

---

## 1. Why this

**Patrik, 2026-09-01:**

> There should not be an ability within the kernel of the platform to create
> maps. We might in the future add a map editor, that both the Human DM and LLM
> DM can use to improvise with. But the command when the new/added map is created
> should simply be — `load_map`, not `create_scene`.

**The argument is about shape, not about features.** `create_scene` is a
ONE-SHOT command. Terrain authoring is ITERATIVE. Put iterative work behind a
one-shot command and every mistake is permanent: declare a wall, then decide it
wants a door, and there is nothing to edit. Allowing any terrain variety at all
therefore obliges an editing surface — change terrain, remove terrain, move a
thing, delete a thing — and that is a map editor. A map editor is its own tool,
and it does not belong in an event-sourced kernel.

**The same conclusion had already arrived as a defect.** Sub-project 13's
whole-branch review found that the DM console's `wall` fill produced a room no
player could ever enter, in five clicks: nothing edits a scene once
`SceneCreated` has landed, the scene id cannot be reused so `load_map` cannot
repair it, and `internal/engine`'s terrain check blocks entry to every wall
square.

Worse — and this is deliberate design rather than an oversight — the blocked
check runs on the player branch only. Maps-as-geometry's rule is "hard for
players, free for DM", and it governs stone exactly as it governs sight, so the
DM's own movement is untouched. The DM therefore walks the room they have just
ruined and cannot experience the problem they created. A footgun whose author is
structurally unable to feel it is the worst shape available.

The `wall` option was removed in `fcf45cf`; this sub-project removes the command
that made it possible.

**Nothing is lost that the platform should have had.** A place still comes into
existence, still improvised, still mid-session — through a file and a
`load_map`. What goes is the platform's ability to be *told* to invent one.

---

## 2. Non-goals

**The map editor.** It may exist later, for both the human DM and the LLM DM.
This sub-project makes its output loadable; it does not build it.

**An upload endpoint.** Installing a map is a filesystem act. Nothing about a map
travels over the wire, which is why the 32 KiB frame ceiling that constrained
`create_scene` does not exist here.

**Migrating existing campaigns.** The platform has never been used for real.
A campaign that is a bare log file is refused with a message, not silently
adopted — §3 says why.

**Per-square editing of a loaded scene.** That is the editing surface this
sub-project exists to keep out of the kernel.

---

## 3. A campaign becomes a directory

Today `campaign.Open` takes a path and hands it to `store.Open`: a campaign is
one log file, and maps are a server-wide `--maps-dir` the operator points at,
loaded and validated once at boot. That arrangement cannot express "this campaign
has these maps".

```
campaigns/goblin-ambush/
  log.db                     # what campaign.Open takes today
  maps/
    cellar.json
    dungeon-level-2.json
  packs/
    cellar-basics/
      pack.json
      masonry_1.png
```

**Maps are flat files with human-readable names.** A DM runs the campaign and
will not remember an identifier; `ls maps/` must tell them what they have.
Packs stay directories because they carry images beside their manifest.

**Packs live inside the campaign rather than in a shared store.** The trade is
duplication against portability, and the campaign directory being copyable,
zippable and handable to someone is worth more than deduplicating a tileset.
MapTool settled the same question the same way — a `.cmpgn` embeds its own
assets — and it has held for two decades. If duplication ever bites, the answer
is a tool that installs a pack into a campaign, not a shared directory that makes
a campaign incomplete on its own.

**`--maps-dir` goes.** An operator-level flag serving whatever campaign happens
to be running stops making sense once maps belong to a campaign. `GET /api/maps`
answers from the open campaign's own `maps/`.

**A bare log file is refused, not adopted.** `campaign.Open` takes a directory.
Pointing it at a lone `log.db` earns a message telling you to put it in one.
Silently treating it as a campaign with no maps is the implicit fallback this
platform keeps ruling against — the same reasoning that made terrain mandatory
and that gives the console's selects a blank first option.

---

## 4. Install, then load

**Two acts, always.** This is the model, not a workaround for a missing feature:

1. **Install** — a map file appears in the campaign's `maps/`.
2. **Load** — `load_map{map_id}` brings it into play.

Boot-time is not a special case; it is step 1 having happened early. The two
scenarios this serves:

- **Ordinary play.** The campaign ships with `dungeon-level-1.json` and
  `dungeon-level-2.json`. The party finishes level 1 and the DM loads level 2. No
  restart, no reconnect.
- **Improvisation.** The players go somewhere unanticipated. The DM authors a map
  outside the platform, following the map JSON contract, writes it into `maps/`,
  and loads it.

**Install is outside the platform.** The DM, a script, or a future editor writes
the file by whatever means. The platform receives nothing over any wire, so there
is no frame ceiling, no upload endpoint, no new command, and no authorization
question. Moving this step out of the kernel is the whole point of the
sub-project.

**Load is unchanged.** `LoadMap { string map_id = 1; }` already says exactly what
is needed. The contract does not move.

---

## 5. Lookup on demand

The server holds its maps in memory, populated at boot. On a miss, it probes
`maps/<id>.json` before refusing — a single path, not a directory walk, because
§6 makes the filename the id.

Sequence on a miss: probe → parse → check `format_version` → compile against the
map's pack → cache → emit the batch. Every failure is a readable `CommandResult`:

- `map "level-2" not found in this campaign`
- `map "level-2" declares format 3; this server understands 1 and 2`
- the `mapdef` compile error verbatim, including the every-square rule

**Boot preload stays exactly as it is.** Everything present at startup is still
walked and validated, so an operator still learns about a broken map before
anyone connects. On-demand covers only what arrives later; the existing property
is preserved rather than traded away.

**No watcher.** A filesystem watch would need debouncing, would inherit
platform-specific behaviour, and would hold state that can drift from the disk.
Lookup on demand adds no state and no surface: the directory is the truth at the
moment you ask. A half-copied file fails to compile, the DM reads the error, and
tries again.

**Concurrency.** Two `load_map`s racing on the same new id must not both compile
and cache it. The lookup takes the lock the map set already lives behind.

---

## 6. Identity: filename, id, and name

Three strings, and conflating any two of them is a defect:

| | | |
|---|---|---|
| **filename** | `maps/dungeon-level-2.json` | what a person types and reads |
| **`id`** | `"id": "dungeon-level-2"` | the scene's identity in the log, forever |
| **`name`** | `"The Sunken Cellar"` | the display label |

**The `id` field stays, and it is not redundant.** `mapdef.Compile` sets
`SceneId` from the map's `ID`, so the map's id becomes the scene id in
`SceneCreated` and in every `TokenPlaced` that follows. It lives in the log
permanently — every move, every sighting, every door in that place references it.

**Filename and id must match, and a mismatch is refused**, naming both. That
refusal is the reason to keep the field. Without it, renaming `cellar.json` to
`sunken-cellar.json` and loading it produces a SECOND scene for the same place,
silently, while the original remains in the world. With it, the rename earns an
error that says exactly what is wrong. Fail loud, not silent.

**Uniqueness is the filesystem's job.** MapTool needs an opaque GUID because zone
names are not unique and a rename must not break the references a campaign holds.
A directory cannot hold two `cellar.json`, so the guarantee MapTool buys with a
UUID we get for free — and we spend it on a name a human can remember.

**`name` changes freely.** It is the display label; nothing references it.

---

## 7. Format versions, on maps and on packs separately

Both artifacts carry `format_version`. Each declares the format *it* is written
in, and the server checks each against what it understands, naming the file that
is wrong.

**Separately, because they change independently.** A pack is shared: one tileset
may back many maps. If the map format moves and a single shared number moved with
it, every pack on disk would either need rewriting to carry a version that says
nothing about its own contents, or the number would silently only ever describe
maps — which is the state this exists to leave. With separate versions a pack
stays valid and untouched while backing maps of either version.

**Decided now because it is cheap now.** The format is pre-release and has no
readers outside this repo. MapTool's persistence carries decades of migration
logic baked into its model classes; its lesson, stated plainly, is to plan for
format evolution from day one rather than retrofit it.

---

## 8. There is no `isVisible`, and that is a real difference

MapTool has three concepts where this design has two: a zone exists in the
campaign, a zone `isVisible` to players, and `enforceZone` moves everyone's view
onto it.

**It needs the middle one because its distribution is all-or-nothing.** Every
connected client receives every zone in full — `Campaign.toDto()` adds all zones
with no visibility check, and the server sends the whole campaign to each new
client. Visibility is a client-side rendering filter, so a modified client or
anyone reading the traffic sees every hidden map.

**Ours is derived.** A scene exists in the world; whether a player perceives it
falls out of where their character is and what `internal/sight` says they can
see. There is nothing to declare and nothing to filter, so loading a map for a
place the party has not reached leaks nothing.

Recorded because it is the one place this platform is ahead of the prior art, and
because a later reader comparing the two should know the omission is deliberate.

---

## 9. What leaves, and what stays

**Leaves:** `CreateScene` and its oneof arm; the gateway handler, its
`commandRoles` row and `commandName` arm, and `create_scene_validate.go` entire;
the client builder, the console's Create-scene group with its fill selector and
size cap, and the `COMMAND_SURFACE` row; the toolgen entry and both generated
`tools.json`.

**Stays:** `SceneCreated`, the EVENT, emitted by `mapdef.Compile`. And the
every-square-declared rule, which already lives in `mapdef` — sub-project 13 put
it there rather than in the command, which is why removing the command costs the
rule nothing.

**`create_scene` goes LAST**, after install-then-load works. Sub-project 13 made
the opposite mistake once: the DM's undo controls were deleted before a
forward-only way to remove a token existed, leaving an interval in which the DM
could do neither. Removal follows its replacement.

---

## 10. The corpus is the bulk of the work

Eight scenario files issue eleven `createScene` steps. Each switches to an
authored map file and a `load_map`. Seven golden directories are re-derived —
`goblin-fight` has no golden, and `adventure-night` loads an adventure rather
than creating a scene.

`state.json` is hand-derived from the scenario definition and `stream.json` is
recorded from the server; their agreement is evidence only because neither was
produced from the other, and `cmd/vtt/scenario_goldens_test.go` has no `-update`
flag by deliberate decision. That constraint holds here.

**The plumbing exists.** `composeServer` already takes a maps directory and
scenarios currently pass an empty string.

**Two things should improve.** No scenario exercises `load_map` today, so the
corpus gains coverage of what becomes the only way a place exists. And the map
format is richer than `create_scene`'s — a named tile against a pack, rather than
a bare kind — so fixtures can carry real walls and doors instead of uniform
floor.

---

## 11. Testing

**The three new behaviours**, each pinned on the boundary:

- a map installed after boot is found and loaded with no restart;
- a `format_version` the server does not understand is refused, naming the file
  and both versions;
- a filename/id mismatch is refused, rather than producing a second scene.

**The removal is proved by absence with a gate**, in the shape sub-project 13
used: no `create_scene` identifier survives, checked in code positions rather
than by grepping prose, so the dated records explaining why it left are kept.

**The corpus conversion is proved by the existing golden gates.** They already
fail on any drift between hand-derived state and recorded stream.

---

## 12. What could go wrong

**A DM renames a map file and loses the correspondence.** The refusal in §6
catches the case that matters — rename then load — but a rename of a file already
loaded leaves a scene in the world whose id names no file. Nothing breaks; the
log is self-contained. It is a reason to say in the map format's documentation
that the id is the scene's name forever.

**The improvisation path is only as good as the authoring.** Until a map editor
exists, "author a map outside the platform" means writing JSON by hand or with an
LLM's help. That is a real cost, and it is the cost the ruling accepts: a
platform that cannot author is better than one that authors badly and cannot
edit.

**Boot validation and on-demand validation can diverge.** Two code paths reach
`mapdef` and they must agree, or a map that boots cleanly could be refused on
reload. They should share one function rather than two similar ones.

**Nothing can create a place if the campaign directory is read-only.** An
operator running from a read-only mount loses improvisation entirely. Worth a
clear error rather than a confusing one.

---

## 13. Exit criteria

1. No `create_scene` identifier survives in `internal/`, `client/src/`, `cmd/`,
   `contract/` or `scenarios/`, proven by a gate rather than a search.
2. A campaign is a directory; `campaign.Open` refuses a bare log file with a
   message that says what to do.
3. A map written into a running campaign's `maps/` is loadable without a restart.
4. A `format_version` the server does not understand is refused by name, for both
   maps and packs.
5. A filename/id mismatch is refused and names both.
6. All eight affected scenarios load maps; seven goldens re-derived by hand and
   agreeing with their recorded streams.
7. `SceneCreated` still carries complete terrain, by `mapdef`'s rule, unchanged.
8. `task check` green from cold, both mutation gates included, every gated
   package's fingerprint recomputed against the tree being merged.
