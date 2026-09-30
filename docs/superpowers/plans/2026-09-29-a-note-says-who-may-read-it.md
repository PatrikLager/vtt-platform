# A note says who may read it — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-29-a-note-says-who-may-read-it-design.md`
**Verified:** 2026-09-29, by `verify-ticket`, an agent that did not write the
ticket, against `20c23f3` on `feat/note-visibility` (`main` at the time; the
ticket untracked). Verdict: **Passes with gaps.** The gaps are listed at the
end and travel with this plan. This plan does not edit the ticket; where an
item is thin or a rule is loose, the plan decides around it and says so.

**Goal, in the ticket's words:** a note carries a visibility, public or
DM-only, through every layer: the contract, both folds, the command path (an
`upsert_note` naming none is refused for the DM and the agent alike), the
projection (a public note reaches every player and spectator; a DM-only note,
or one recorded without a visibility, reaches none; a note that stops being
public or is deleted leaves every fold that held it, and no frame names a key a
seat was never sent), adventure notes (public), MCP (`get_state` and the
server's instructions say what the field is) and the client (the DM console
asks, the notes panel marks); SPEC-016, SPEC-007 and SPEC-013 describe it;
every rule the sort accepts has a row cited by a test; `task check` whole is
green.

**The owner's rulings of 2026-09-29, which bound this plan and are not
re-opened.** A note carries a visibility, public or DM-only. One ticket through
every layer. A note recorded without a visibility (old events, goldens, the
enum's zero) is DM-only: fail closed. Notes loaded from an adventure
(`notes/*.json`) are public, and the adventure JSON format does not change. An
`upsert_note` naming no visibility is refused at the gateway for every role
that may issue it, as `add_actor`'s kind is. Narration stays forwarded to all.
The testimony rule for actors is a later ticket.

**MapTool (CLAUDE.md rule 9), the answer this plan records.** MapTool has no
free-standing notes: a token carries two text fields, `notes` and `gmNotes`,
each with a type, and the audience is which field the text is in. It decides
who reads the GM field on the client, from a campaign every client holds:
`Token.getGMNotes` (`model/Token.java`) returns `gmNotes` only when
`MapTool.getPlayer().isGM()` or a trusted macro is running and `""`
otherwise, while `Token.toDto` writes `gmNotes` into the DTO whenever it is
set, and `MapToolServer.addRemoteConnection` sends every joiner
`SetCampaignMsg(campaign.toDto())`. Its vocabulary (`i18n.properties`):
`EditTokenDialog.tab.gmnotes = GM Notes`, `EditTokenDialog.tab.playerNotes =
Player Notes`, `EditTokenDialog.label.visible = Visible to players`,
`EditTokenDialog.label.visible.owner = Visible to Owners Only`, and a GM
preference `Preferences.label.tokens.visible = New tokens visible to
players`. The notes behind this paragraph were written by an Explore agent
against `~/dev/RPTool/maptool` at `f4b7fef6c`; the verifier spot-checked
`getGMNotes`' body, `toDto`'s `gmNotes` write, `addRemoteConnection`'s
`SetCampaignMsg`, and the five i18n keys, by grep. **Borrowed:** the two
audiences, the GM and everyone at the table, decided once per note; and a
visible audience mark on each note in the GM's view (MapTool's GM Notes tab).
**Refused, each with its reason:** the distribution — every client receives
the GM text and a client-side getter blanks it, which is the model rule 9
names; here the projection never sends a DM-only note at all. Two text fields
per record — a note here is one titled record, so one enum lets the server send
or withhold the whole of it. No write enforcement (MapTool's server relays any
token edit) — here SPEC-013's role table and a validator decide. A default
audience (MapTool's "New tokens visible to players") — the owner ruled that a
command states it or is refused, and the console asks rather than pre-fills.
The owner-only third level ("Visible to Owners Only") — out of this ticket; the
enum grows additively and the projection reads every value but public as
DM-only, so a third level fails closed until its ticket. SPEC-007, SPEC-013,
SPEC-016, the register and the code name no other project; this plan and the
report do.

## Verification, check by check

1. **Every path resolves — by command.** Every path the ticket names exists
   as written, save one. `conformance/conformance.go`, listed under
   "Adventures", resolves only as `internal/adventure/conformance/conformance.go`
   (the tree also holds `internal/rules/conformance/conformance.go`); read
   relative to `internal/adventure/`, it is right. Every symbol named is where
   the ticket says: `UpsertNote` and `DeleteNote` in `commands.proto`,
   `NoteUpserted` and `NoteDeleted` in `events.proto`, each a one-line message
   of `key`, `title`, `text` or `key`; `engine.Note` with `Title`, `Text`,
   `UpdatedSeq`; the client `Note`; `classify`'s one arm for both note
   payloads, answering `withheld`; `Project`; `renderNotes`, rendered by
   `renderSpectator` for every seat; `adventure.Compile` emitting
   `NoteUpserted`; `renderDMConsole`; the `upsertNote(key, title, text)`
   builder; toolgen's `upsert_note` entry with `requiredOverride:
   []string{"key", "text"}`; `TestQANoEventPayloadNamesARole`; `ActorKind`
   and `JoinDoor`, each with an `_UNSPECIFIED` zero. SPEC-016's Status names
   "the notes flag" as the first of two tickets not yet written; it names the
   subject, not a path, since the ticket did not exist.
2. **"Done" is an observation — by command where a test exists or can be
   sketched.** The one named test that exists,
   `TestUpsertNoteRequiredOverrideReplacesDerivedList`, asserts `[key text]`
   today, so item 1's schema observation fails today. The rest name tests to
   be written; the verifier sketched the ones that can be sketched in a scratch
   clone and each fails today for the ticket's reason (Measurements, P1 to
   P6): a public note is withheld from a player; a fixture carrying
   `visibility` fails round-trip with `unknown field "visibility"`; a
   visibility-less `upsert_note` is accepted; `act-fighter`'s projected state
   holds `"Notes": {}` (item 6). Item 4's last clause, "a deleted DM-only note
   ... no frame they are sent names its key", is loose for a note that was
   public earlier: the frame that took it off the seat named its key. Row E
   words it for a note never public.
3. **Each rule is breakable — a reading, then probes.** Of the ticket's ten
   rules: the ninth joins two (the console asks; the panel marks) and is split
   J and K; the seventh joins two (a deletion leaves the folds that held the
   note; it reaches no other) and is split G and H; the fifth is loose as above
   and is worded E; the third's forward half is C and its "made DM-only" half
   folds into F; the tenth is VTT-176 and is refused. Every accepted row names
   the break that reds it (D17); the four projection breaks were run against a
   sketch of D6 in a scratch clone (Measurements, P5).
4. **The scope matches the claim — by command, then a reading.** `git grep`
   outside `contract/gen`, `cmd/vtt/webdist`, `docs` and `contract-spike`
   finds `upsert_note`/`UpsertNote`/`upsertNote` in 30 files and a note event
   in 36. A sketch of the refusal (Measurements, P2) reds exactly four tests:
   `TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned`
   (`internal/gateway/server_test.go`), `TestScenarioLibraryRunsSelfContained`
   (through `scenarios/story-table.json`), `TestMCPWorldLayerRoundTrip`
   (`cmd/vtt/mcp_e2e_test.go`) and `TestScenarioGoldenStreamsHaveNotDrifted`;
   `internal/mcp` and `internal/harness` stay green. The ticket declares its
   size (more than two components, more than ten files) and the owner ruled
   one ticket. Forced and not named, each covered by the plan: a new file for
   the validator (D5); `internal/adventure/compile_test.go`
   (`TestCompileValidFixtureExactEnvelopeList` compares whole envelopes);
   `internal/eventgen/model_test.go` (D4's guard); `client/src/state.ts`'s
   header comment, which a tagged field makes false (D2);
   `internal/mcp/server.go`'s "human players can see everything you do",
   which the new note sentence beside it would contradict (D10); the `why` of
   `story-table`'s `act-hero` and `adventure-night`'s `act-fighter`
   `viewer.json` (D11); `describe`'s note label in `client/src/view/spectator.ts`
   if Q3 says yes; `docs/verification-debt.md` if Q7 says yes. None of them
   widens the work past the declared size.
5. **No recorded decision is contradicted silently — a reading.** SPEC-016's
   notes ruling ("withholds ... `NoteUpserted` and `NoteDeleted`, which only
   the DM and the agent may issue") and its Consequence "A player's and a
   spectator's notes panel is empty" are overturned; the ticket names both, and
   SPEC-016's own Status says this ticket rewrites them. SPEC-013 gains a third
   validator and SPEC-007 a third refusal; neither contradicts a sentence.
   SPEC-012's note-key residue stays true as written (D8). The visibility
   ticket of 2026-08-18 (quoted in `TestNarrationReachesAPlayerAndANoteDoesNot`'s
   comment as "a note can say anything (spec §4.4)") and the world-layer design
   are history and are not edited. No ADR decides who reads a note (`grep -il
   note docs/adr/*.md` finds only unrelated uses). The adventure format's
   "initially-REVEALED world notes" becomes true.
6. **The records the work moves are named, and reachable — by command, then a
   reading.** The section lists SPEC-016, SPEC-007 and SPEC-013; all three
   resolve. The reading: no other specification carries a note sentence
   (`grep -il note docs/specifications/*.md` finds SPEC-012's residue,
   SPEC-013's role-table rows for `upsert_note` and `delete_note`, SPEC-014's
   unrelated `ErrNotExist` and SPEC-016; SPEC-007 has none yet). SPEC-012's
   residue does not move (D8). SPEC-007's `Requirements` line ("None allocated
   for this record") changes if row L is accepted, inside a named file. Rows
   A, I, J and K have
   no specification to be named by: the engine's fold, the adventure compile
   and the client have no record (Gap 3, Q10).

## Measurements this plan stands on

All at `20c23f3`, by command, run by the verifier in a scratch clone
(`git clone --no-hardlinks`), never in the working tree. The probes' sketches
(P1 to P6) are the verifier's, not the plan's code; they show the design runs,
and they were discarded.

- `python3 tools/check-requirements-chain.py .` prints `227 rows, 193 test
  files, 10 specifications; every citation resolves and every row's evidence
  holds`. The next id the dispenser would allocate is VTT-228; none is
  allocated before sign-off. `requirement-id` is on the path
  (`~/.claude/plugins/cache/patrik-process/dev-cycle/0.4.0/bin/`).
- `python3 tools/check_mutation_test.py -q` and `python3
  tools/check_ts_mutation_test.py -q` print `OK`.
- **Every file this plan edits under `internal/`, `cmd/` and `client/src`
  sits at its comment ceiling** (`python3 tools/check-comments.py --report`,
  share / ceiling): `apply.go` 43.0/43.0, `state.go` 58.1/58.1, `convert.go`
  36.0/36.0, `server.go` 37.5/37.6, `project.go` 20.4/20.4, `model.go`
  29.5/29.6, `compile.go` 37.5/37.5, `conformance/conformance.go` 29.2/29.2,
  `mcp/read_tools.go` 31.2/31.2, `mcp/server.go` 48.4/48.5, `fold.ts`
  43.4/43.5, `state.ts` 63.9/64.0, `commands.ts` 45.3/45.4, `view/dm.ts`
  46.1/46.1, `view/spectator.ts` 50.2/50.3; `grant_validate.go` 26.3/26.4 and
  `add_actor_validate.go` 25.0/25.0 are the validators' precedent. So a comment
  line added to any of them needs code lines beside it (D20).
- **Mutation keys in the files this plan edits:** `internal/engine apply.go`
  (four; one below the `NoteUpserted` arm), `internal/gateway project.go`
  (three, all below the Projector struct and `classify`), `internal/mcp
  read_tools.go` (one) and `server.go` (two), both below the prose constants
  D10 edits; in `tools/ts-mutation-equivalents.txt`, eighteen in `fold.ts`
  (most below the `noteUpserted` arm), five in `view/dm.ts` (one in `input`,
  four below the notes section) and two in `view/spectator.ts` (in
  `renderPerch`, below `renderNotes`).
- **P1, the contract alone.** `enum NoteVisibility` with
  `NOTE_VISIBILITY_UNSPECIFIED = 0`, `NOTE_VISIBILITY_PUBLIC = 1`,
  `NOTE_VISIBILITY_SECRET = 2` and `NoteVisibility visibility = 4` on both
  messages: `go tool buf lint` clean, `go build ./...` clean, both VTT-038
  tests green; `tools.json` gains `visibility` with `enum:
  [NOTE_VISIBILITY_PUBLIC NOTE_VISIBILITY_SECRET]` (toolgen drops the
  `_UNSPECIFIED` zero) and `required` stays `[key text]`;
  `TestToolsMatchGolden` reds until `expected_tools.json` follows. Renaming the
  second value `NOTE_VISIBILITY_DM_ONLY` reds `TestQANoEventPayloadNamesARole`
  with `event payloads name a role: [vtt.v1.NOTE_VISIBILITY_DM_ONLY]`. A
  contract-only change moves `cmd/vtt/webdist/assets/index.js` (`bunx vite
  build --config client/vite.config.ts`). The generated TypeScript enum is
  `NoteVisibility.UNSPECIFIED`, `.PUBLIC`, `.SECRET`.
- **P2, the refusal.** A validator refusing `NOTE_VISIBILITY_UNSPECIFIED`,
  called in `handleCommand` beside the other two, over `go test -count=1
  ./internal/gateway/ ./internal/mcp/ ./internal/harness/ ./cmd/vtt/`: the four
  reds of check 4 and nothing else. With `visibility` added to the MCP required
  list, `internal/mcp` stays green: its world-layer round trip omits the field
  and nothing refuses it there.
- **P3, today's projection.** A test projecting a `NOTE_VISIBILITY_PUBLIC`
  upsert to a player and a spectator fails on the field-only tree
  (`player: a public note must reach every player and spectator`, and the
  same for the spectator). A test that a note made secret leaves a player's
  fold PASSES on the same tree, because nothing was ever sent: without an
  assertion that the seat held the note first, it is vacuous (D17).
- **P4, the engine field untagged.** `Visibility` on `engine.Note` with no
  json tag makes `TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees`
  red for `adventure-night/dm`, `adventure-night/agent`, `story-table/dm` and
  `story-table/agent`: the dump gains `"Visibility": 0` on every note. With
  `json:",omitempty"` no golden moves (D2).
- **P5, D6 sketched.** With `notes` memory, the two `classify` arms and
  `noteTransitions` as D6 describes, the P3 tests and
  `TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` pass, and
  `TestNarrationReachesAPlayerAndANoteDoesNot` stays green. Four breaks, each
  run and repaired: forwarding every `NoteDeleted` reds
  `TestNarrationReachesAPlayerAndANoteDoesNot` (`deleting a note names the
  note, and must not reach a player either`) and the property test (the seats'
  folds stop early and its vacuity floors trip: `only 0 of 6 seeds ended with
  a player holding a token`); sending nothing when a held note stops being
  public reds the flip test and the property test's seeds 5 and 6; forwarding a
  `NoteUpserted` of any visibility but secret reds the property test (`player-1
  holds note "prop-note-7", which the server does not have`, a deletion the
  memory did not know to forward); never remembering a forwarded note reds
  the flip test and the property test.
- **P6, eventgen drawing by action index** (D4): `internal/eventgen`,
  `internal/campaign` and the gateway's property test, the three places that
  import it, stay green; over the property test's six seeds every seed projects
  a public note to both players, three seeds (2, 5, 6) project a note leaving a
  player because it stopped being public, five project a forwarded deletion,
  and the server ends holding a public note in three seeds (3, 4, 6).
- A round-trip fixture carrying `"visibility": "NOTE_VISIBILITY_SECRET"` fails
  today: `unmarshal note_upserted_envelope.json: proto: (line 11:5): unknown
  field "visibility"`.

## Constraints that bind every task

- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, only
  through `--write-ledger`; mutation keys are re-pointed to where their mutants
  now are, never deleted; a new survivor is killed by a test, and adjudicated
  equivalent only when no test can kill it.
- Rule 3: additive only. One new enum, one new field on each of two messages,
  numbers never reused; `check:breaking` reports (it does not fail until
  `contract/RELEASED` exists), and its report must name no break.
- Rule 4: one fold. `engine.Apply` and `client/src/fold.ts` record the
  visibility and nothing else decides it; the projector's memory is named
  `notes` and never writes `st.Notes` (the semgrep one-fold rule matches
  `$S.Notes[$K] = $V`).
- Rule 5: "public", "secret" and "DM only" are platform vocabulary; no rules
  concept enters.
- Rule 8: this plan, the specifications and the report cite symbols, tests and
  files, never a line number; mutation adjudication keys keep writing
  coordinates, as rule 8 says.
- Rule 9: the answer above is recorded before any task runs.
- Rule 10 and SPEC-010: a comment is a warning, a pointer or an exported
  symbol's one-line doc. `check:comments` refuses a banned term on an added
  line, an added line in a block over six, a file above its ceiling that gained
  a comment line, and a file more than 1.0 below its ceiling until
  `--write-ledger`. A new file with no row is held to 25.0.
- SPEC-008: ids from `requirement-id` only, after sign-off, at the start of
  the task whose commit carries the row (D19).
- The `specification` skill: present tense, one decision per file, every
  "only", "never", "every" with its search named, the old sentence not left
  beside the new.
- The `requirements` skill: one thing, what not how, breakable, named by a
  check or knowingly OPEN; the refusals reported one line each.
- Tests before code; one deliberate break per new check, both directions, as a
  line in the commit (D17).
- `check:drift` passes only on a committed tree (`git status --porcelain --
  contract/gen cmd/vtt/tools.json cmd/vtt/webdist`): a commit that regenerates
  the contract or changes `client/src` carries its rebuilt `cmd/vtt/webdist`,
  and the order is review, commit, gate (D14).
- A reading review's package is `git diff HEAD`, never bare `git diff` (staged
  deletions); nothing is stashed or checked out while a reviewer reads; the
  review settles before any gate starts, and mutation keys move last.
- `git add` and `git commit` in separate calls, and `git show --stat HEAD`
  after every commit, since a blocked call can commit a different set.
- The machine is shared: `uptime` before a gate run; `task check` detached with
  `start_new_session=True`; `git push` given a 600 000 ms timeout and never
  killed under five minutes.
- The ticket is not edited. `docs/adr/`, the visibility ticket, the world-layer
  design, the adventure format design and every report under `docs/reports/`
  are not edited; `adventures/goblin-ambush/guide.md` is left.

## Decisions this plan makes

**D1. The field and its enum.** Forced by VTT-038 (P1: `_DM_ONLY` reds it),
the ruling's two values, and the precedents `ActorKind` and `JoinDoor` ("an
enum rather than a bool", with an `_UNSPECIFIED` zero that a command refuses).
In `events.proto`, beside `NoteUpserted`:

    enum NoteVisibility {
      NOTE_VISIBILITY_UNSPECIFIED = 0;
      NOTE_VISIBILITY_PUBLIC = 1;
      NOTE_VISIBILITY_SECRET = 2;
    }

and `NoteVisibility visibility = 4;` on `UpsertNote` (`commands.proto`, which
imports `events.proto`, as `GrantActorControl.kind` uses `ActorKind`) and on
`NoteUpserted`. One enum for both, so a command and its event cannot disagree
in vocabulary. The proto comment above the enum warns, in the house's
proto-comment manner (SPEC-007 keeps such comments): an enum, not a bool,
because protojson omits a false bool and "secret" would be an absent field;
`_UNSPECIFIED` on a recorded event means DM-only, and on a command it is
refused. The alternatives, one line each: `_DM_ONLY` — refused by VTT-038;
`_GM_ONLY` — escapes VTT-038's word list by spelling, not by meaning;
`_PRIVATE` — private to whom is not said; `_HIDDEN` — the projection's
`TokenHidden` already means "left your view", a different fact;
`_WITHHELD` — names the projection's verdict, not the note; a `bool public` —
protojson omits `false`, the reason `JoinDoor` is an enum. Q1.

**D2. What the folds record.** Forced by the ruling (recorded without is
DM-only), the `ActorKind` precedent ("the fold stores what the log said,
verbatim; readers apply the sentence"), P4, and the block bound. `engine.Note`
gains `Visibility vttv1.NoteVisibility` with the tag `json:",omitempty"`; the
`NoteUpserted` arm of `engine.Apply` stores `nu.Visibility` as sent and refuses
nothing about it; `client/src/state.ts`'s `Note` gains `Visibility:
NoteVisibility`, and `fold.ts`'s `noteUpserted` arm stores `v.visibility`.
`foldToDumpJSON` emits `Visibility` after `UpdatedSeq` only when it is not 0,
the way it already treats an actor's `kind`. So an absent visibility is a
legal recorded state in both folds, the projection and the panel read every
value but `NOTE_VISIBILITY_PUBLIC` as DM-only, and no golden moves until a note
in it carries a visibility; `engine.Scene`'s `Explored` and `Visible` carry the
same tag for the same reason. `state.ts`'s header says Scene, Token, Session,
Note and ActorCondition "are plain Go structs with NO json tags ... zero values
are NOT omitted", already false for Scene and false for Note after this; it is
one block of more than six lines, so it is rewritten whole to six lines or
fewer that say which fields are omitempty (Scene's `Explored` and `Visible`,
Note's `Visibility`) and that Actor and Resource are protobuf types. The
alternative, no tag: `get_state` shows `"Visibility": 0` for a note recorded
without one, and four goldens move in the fold commit for no behaviour (P4).
Q4.

**D3. `ToEvent` carries the command's visibility, in the fold commit.** Forced
by the commit order (D14): the DM console sends a visibility from C3, and a
`ToEvent` that drops it records every console note as DM-only.
`TestToEventUpsertNoteProducesNoteUpserted` asserts the field arrives. The
refusal is not here: `TestEveryClientCommandConverts` requires every command to
convert from an empty payload, the reason SPEC-013 keeps the validators out of
`ToEvent`.

**D4. eventgen draws a visibility without a draw.** Forced by `Step`'s warning
that an extra rng draw moves the whole walk. `upsertNote` sets `Visibility:
noteVisibilities[idx%len(noteVisibilities)]`, where `noteVisibilities` lists
`NOTE_VISIBILITY_UNSPECIFIED`, `NOTE_VISIBILITY_PUBLIC` and
`NOTE_VISIBILITY_SECRET` by name, so the walk is unchanged and a re-upsert of a
key under a different index is a change of visibility (P6: three seeds of six
project one). A guard in `model_test.go`,
`TestAWalkDrawsEveryNoteVisibilityAndAChangeOfOne`, holds that across five
400-action walks every value is drawn and at least one key is upserted public
and later not public. `internal/eventgen` is outside the mutation gate, so the
guard is its only check.

**D5. The refusal.** Forced by the ruling, SPEC-013's "The two validators"
(after `Authorize`, for every role, before anything is written, not wrapping
`ErrUnauthorized`), `ToEvent`'s completeness gate, and the 25.0 default
ceiling for a file with no row. `validateUpsertNote(cmd *vttv1.UpsertNote)
error` lives in a new file, `internal/gateway/note_validate.go`, as the two
validators do, with a doc of at most three lines so the file stays under 25.0.
It refuses `NOTE_VISIBILITY_UNSPECIFIED` and accepts every other value, so the
enum may grow without a note being refused (the `validateGrantActorControl`
precedent, VTT-151). `handleCommand` calls it third, after `validateAddActor`
and before the dispatch, answering ok=false with its text. The message, built
with `%s` of the two values as the kind refusals are:

    gateway: upsert_note: visibility — a note must say who may read it
    (NOTE_VISIBILITY_PUBLIC for every player and spectator,
    NOTE_VISIBILITY_SECRET for the DM and the agent alone); an unstated
    visibility cannot be told from a deliberate one, so it is refused rather
    than guessed

In the same commit toolgen's `upsert_note` override becomes
`requiredOverride: []string{"key", "text", "visibility"}` with a `fieldDocs`
entry for `visibility` in the manner of `add_actor`'s `kind` ("REQUIRED. Who
may read this note. NOTE_VISIBILITY_PUBLIC sends it to every player and
spectator; NOTE_VISIBILITY_SECRET keeps it for the DM and the agent. There is
no default and omitting it is REFUSED. Upserting a public note's key as SECRET
takes it back from every player; upserting a secret one as PUBLIC shows it to
them."). The schema and the refusal land together so no commit says one without
the other.

**D6. The projection: memory, two arms, and a synthesized `NoteDeleted`.**
Forced by: `Project` judges an event against the state after it, where a
deleted note is gone; `classify` reads the memory before `transitions` moves it
("Call it before transitions, which moves pr.tokens and pr.actors past this
event"); both folds refuse a `NoteDeleted` whose key they do not hold; and the
ruling that a DM-only note, and its key, reach no player or spectator.

- `Projector` gains `notes map[string]bool`, the keys of the notes the viewer
  holds, empty from `NewProjector`. It depends only on the log prefix, so every
  projected seat's memory is the same, and a reconnect rebuilds it by the feed
  SPEC-015 already requires.
- `classify` splits its note arm. `NoteUpserted`:
  `passIf(visibility == NOTE_VISIBILITY_PUBLIC)`, so `_SECRET`,
  `_UNSPECIFIED` and any value the enum grows are withheld. `NoteDeleted`:
  `passIf(pr.notes[key])`, forwarded only to a seat that holds the note.
- `transitions` calls a new `noteTransitions(cause, seq, st)` last, after the
  `SceneSeen` walk, mirroring `doorTransitions`' use of its cause and the actor
  forgetting loop. Walking `notes` by sorted key: a key whose note `st` no
  longer holds is forgotten and sends nothing, since `classify` forwarded its
  deletion; a key whose note `st` holds with any visibility but public is
  forgotten and sent a `NoteDeleted` of that key at `seq`. Then the key of a
  causing `NoteUpserted` whose visibility is public is remembered. On a perch
  the cause is nil and nothing changes. It is a function at the end of
  `project.go`, so only the struct field, the constructor line, the two arms
  and the call move the three mutation keys.

**Weighed against a projection-only payload.** The alternative is a new
`NoteHidden { key }` event, `TokenHidden`'s counterpart, sent for BOTH a note
made not public and a deleted one, with `classify` withholding `NoteDeleted` as
it withholds `TokenRemoved` (VTT-205). Sending `NoteHidden` for the one and the
real `NoteDeleted` for the other is refused outright: a player could then tell
"the DM deleted it" from "the DM still holds it privately", which is
information about a DM-only note. Between the two indistinguishable designs:
the synthesized `NoteDeleted` needs no new message, no new arm in either fold
or in `classify`, and no new row for an unreachable log arm; a memory defect
that sends a deletion to a seat without the note is refused by both folds, so
the property test reds at once (P5, the first break) and the keystone would
over any golden holding such a note, where a lenient `NoteHidden` arm modelled
on `TokenHidden`'s would fold silently. Its
cost is that a `NoteDeleted` in a player's stream means "left your view", not
"left the world", the distinction the contract draws for tokens, and that
`describe` labels it `note "<key>" deleted` in the player's feed and ticker.
SPEC-016 states the first as a Consequence; Q3 asks about the second. The
memory is needed in both designs, since either frame names the key. The
recommendation is the synthesized `NoteDeleted`. Q2.

**D7. Adventure notes are public, in `Compile`.** Forced by the ruling and by
"the adventure JSON format does not change". `adventure.Compile` sets
`Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC` on each
`NoteUpserted` it emits; the loader's `Note` and `notes/*.json` are untouched.
`internal/adventure/conformance`'s `noteUpsertedDump` gains `Visibility string
\`json:"visibility"\``, spelled with the wire name and without omitempty, for
the reason `actorAddedDump.Kind` gives (a compiled note with no visibility must
not render as an absent key); its package doc's `note_upserted` line gains the
key (the package doc is exempt from the block bound). The five
`compiled-batch.json` goldens (`adventures/cellar-rats`,
`adventures/goblin-ambush`, and `ok`, `empty-guide` and `golden-mismatch`
under `internal/adventure/conformance/testdata/adventures/`) gain
`"visibility": "NOTE_VISIBILITY_PUBLIC"` by hand first, per the rule in
`conformance.go`'s doc; `golden-mismatch` keeps exactly its one intended
difference, at `envelope[2]`.

**D8. The note-key residue (SPEC-012).** Forced by the ticket's third open
question. A `load_adventure` compiles against the snapshot `authorize` took,
and `campaign.AppendBatch` re-folds under its own lock, so an `upsert_note` on
an adventure's note key landing between the two calls is overwritten. After
this ticket the overwrite also makes the note public, with the adventure's
title and text. Nothing leaks: the DM-only text is in no frame a player is
sent (the batch's `NoteUpserted` carries the adventure's text), and the key is
one the adventure publishes anyway. What changes is only that the DM's private
text leaves the current state (the log keeps it). SPEC-012's sentence ("a note
upserted on the same key between the two calls is overwritten rather than
refused") stays true and is not edited; no code is needed. The debt entry in
`docs/verification-debt.md` can gain one clause saying the overwrite takes the
visibility with it; the ticket does not list that file. Q7.

**D9. The DM console and the notes panel.** Forced by the ruling, the
`kindSelect` precedent (a blank first option keeps "the DM did not answer" a
state, and nothing is sent until an answer), and the block bound. `kindSelect`
becomes `choiceSelect(cls, field, options)`, the actor forms passing their two
kinds and the notes form passing `NOTE_VISIBILITY_PUBLIC` ("public — every
player and spectator") and `NOTE_VISIBILITY_SECRET` ("DM only") after a blank
"who may read it?". The inner "WHICH ONE IS SELECTED" block is untouched; the
doc block above the function names "the two places that must ask" and is more
than six lines, so it is rewritten to six or fewer as a warning (keep the blank
first option; SPEC-013 refuses silence). `noteVisibilityFromWireName` mirrors
`actorKindFromWireName`, answering null for the blank. The Save button refuses
with `Say who may read it: every player and spectator, or the DM alone.` when
the answer is null, and otherwise sends `upsertNote(key, title, text,
visibility)`; the builder in `commands.ts` gains the parameter.
`renderNotes` gives each note whose `Visibility` is not
`NoteVisibility.PUBLIC` the class `secret` and a `DM only` label in its
heading; a player's and a spectator's fold holds only public notes, so their
panel marks nothing. The alternative, a second copy of the select's body as
`visibilitySelect` with a one-line pointer to `kindSelect`, duplicates the one
subtle line the long comment defends. A flip is re-saving the key with the
other choice; a per-note flip control is Gap 1.

**D10. What an agent reads.** Forced by item 8 and by a contradiction.
`getStateDescription` names the note's fourth key: "`Visibility` is a plain
JSON number, 1 for NOTE_VISIBILITY_PUBLIC (every player and spectator holds
the note) and 2 for NOTE_VISIBILITY_SECRET (the DM and you alone); a note
recorded without one has no `Visibility` key and is the DM's and yours alone."
The server `instructions` gain: "Every upsert_note says who may read the note:
NOTE_VISIBILITY_PUBLIC for the whole table, NOTE_VISIBILITY_SECRET for the DM
and you alone." Their first paragraph says "human players can see everything
you do", which the projection already makes false and which the new sentence
would contradict two lines later; it becomes "the DM sees everything you do,
and each player or spectator is sent only what the table lets them see". Q9.
`TestGetStateDescriptionNamesNotesKey` and
`TestServerInstructionsMentionNarrationAsTableMemory` each gain a clause for
`Visibility`/`visibility`. Both constants sit above the mutation keys in their
files, which move.

**D11. Which goldens move, and how each is made.** Forced by the corpus
README's rule (state by hand, stream recorded, projected stream derived) and
by D14's order.

| Golden | Commit | How it is made |
|---|---|---|
| `contract/testdata/note_upserted_envelope.json` (new) | C1 | by hand |
| `contract/testdata/expected_tools.json` | C1, C4 | regenerated toolgen output, read against D1 and D5 |
| `contract/testdata/upsert_note_command.json` | C3 | by hand, `"visibility": "NOTE_VISIBILITY_PUBLIC"` |
| `scenarios/goldens/story-table/state.json` | C4 | by hand from `scenarios/story-table.json` |
| `scenarios/goldens/story-table/stream.json` | C4 | re-recorded from `TestScenarioGoldenStreamsHaveNotDrifted`'s recorded output, after `state.json` |
| `story-table/projections/act-hero/stream.json` | C4, C5 | derived: `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`' emitted stream, read before it is committed |
| `story-table/projections/act-hero/state.json` | C4, C5 | by hand |
| `story-table/projections/act-hero/viewer.json` | C5 | its `why`, by hand |
| the five `compiled-batch.json` | C6 | by hand, first |
| `scenarios/goldens/adventure-night/state.json` | C6 | by hand |
| `scenarios/goldens/adventure-night/stream.json` | C6 | re-recorded |
| `adventure-night/projections/act-fighter/stream.json`, `state.json`, `viewer.json` | C6 | stream derived; state and `why` by hand |

Every other golden holds no note or only an empty `Notes`, and with D2's tag
none moves. `story-table.json` changes in C4, to its final shape (D12), and
C4's projection still withholds notes, so `act-hero`'s stream moves there only
by the sequences after the added step; C5 adds its note frames.
`act-fighter`'s `why` counts its entries ("8 of its 11 entries are
synthesized; the other 3 are forwarded"), which C6's forwarded note makes
false; the sentence is rewritten to name what is forwarded without a count.
Q11. `act-hero`'s `why` says notes "are withheld unconditionally"; C5 rewrites
it to say which of its notes arrive and why.

**D12. Everything that issues `upsert_note` states a visibility.** Forced by
P2. In C4: `TestNoteAndNarrationRejectionSurfacesCleanNotPoisoned`'s valid
upsert gains `NOTE_VISIBILITY_PUBLIC`, so its two rejections stay the fold's
and VTT-161 keeps its evidence; `TestMCPWorldLayerRoundTrip` sends
`"visibility": "NOTE_VISIBILITY_PUBLIC"` and asserts `get_state`'s note carries
`"Visibility": 1`; `scenarios/story-table.json`'s note steps become, in order:
`kobold-den` "Kobold Den" SECRET; `kobold-den` "Kobold Den (cleared)"
PUBLIC; `old-rumor` PUBLIC; **one added step**, `old-rumor` re-upserted
SECRET with the same title and text; `delete_note old-rumor` (ok); the second
delete (denied "not present"); the player's `upsert_note`, now carrying
PUBLIC, still denied "not authorized". So the seat's corpus holds a secret note
made public, a public note made secret and then deleted, which is the case
where a wrong memory breaks a fold, and the one golden where a TypeScript fold
is held to it. Its `noteAt` probe is unchanged. No corpus gate is added for
note steps: every ok-expecting step passes through the gateway's refusal (Gap
4). `internal/mcp/world_layer_e2e_test.go` is not touched (P2: green, its
fake gateway converts by hand). `internal/gateway/authz_test.go`'s
`upsertNoteCmd` is not touched: `Authorize` reads no visibility.

**D13. Specifications move in the commit whose code makes their sentences
true, and each such commit carries the rows its record names.** Forced by the
repo's practice of every sentence true at every commit (the joining plan's D14),
and by Phase 4a: QA derives its tests from the specification, so SPEC-016 must
say what C5 delivers before C5's QA runs, not after the report. The dev-cycle
skill orders the record LAST, after the report; its reason (a record written
first describes a system that does not exist yet) does not reach a sentence
moved with the code that makes it true. After the report, the Phase 5 record
step re-reads the three records against the final tree and commits only what
moved. Q5. The sentences:

- **SPEC-007, in C4.** Under "What a command refuses", after the
  `grant_actor_control` paragraph: "`upsert_note` REQUIRES `visibility`, and
  one that omits it is refused rather than defaulted, for the reason
  `grant_actor_control`'s `kind` is: an omitted enum arrives as
  `NOTE_VISIBILITY_UNSPECIFIED`, and a default to public publishes a note
  nobody chose to publish. Its MCP tool lists `visibility` as required."
  `Requirements`: "None allocated for this record; no ticket carries them
  yet." becomes row L's id.
- **SPEC-013, in C4.** Status adds `internal/gateway/note_validate.go`
  (`validateUpsertNote`) after `add_actor_validate.go`, and
  `note_validate_test.go` to the pinned tests. The command path's list gains
  "4. `validateUpsertNote`, for an `upsert_note`;" and the dispatch,
  `ToEvent`, backfill, `campaign.Append` and result items become 5 to 9.
  "**The two validators.**" becomes "**The three validators.**", and gains:
  "`validateUpsertNote` refuses an `upsert_note` whose `visibility` is
  `NOTE_VISIBILITY_UNSPECIFIED`, with a message naming `visibility`, and
  accepts every other value, so the enum may grow without a note being
  refused. `engine.Apply` accepts a `NoteUpserted` with no visibility, which
  the projection reads as not public (SPEC-016), so this refusal too exists at
  the command boundary alone." Consequences gain "An `upsert_note` states who
  may read the note." `Requirements` gains row B.
- **SPEC-016, in C5.** Status's first paragraph adds `noteTransitions` to the
  symbol list. Its second paragraph keeps only the testimony decision: "One
  decision the owner has taken will change this record, and it is not
  implemented. Only what a viewer sees will give them information ... [the
  existing sentences about it] ... A ticket of its own carries it, and it is
  not written yet; the decision is recorded in
  `docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`.
  Until it lands, every sentence below describes the code, and the ticket that
  lands it rewrites this paragraph and the sentences it names." "What the
  projector remembers" gains "`notes` holds the keys of the notes the viewer
  holds; `noteTransitions` removes a key when `st` no longer holds the note or
  holds it with any visibility but `NOTE_VISIBILITY_PUBLIC`." "What
  `transitions` sends, in order" gains a last sentence: "Last,
  `noteTransitions` walks `notes` by key: a key whose note `st` no longer holds
  is forgotten and sends nothing, since `classify` forwarded its deletion; a
  key whose note `st` holds with any other visibility is forgotten and sent as
  a `NoteDeleted` of that key; and the key of a causing `NoteUpserted` whose
  visibility is `NOTE_VISIBILITY_PUBLIC` is remembered." "How each payload is
  ruled" drops "`NoteUpserted` and `NoteDeleted`, which only the DM and the
  agent may issue (SPEC-013);" from the withheld list and gains: "It forwards
  `NoteUpserted` when its visibility is `NOTE_VISIBILITY_PUBLIC` and withholds
  it for every other value, `NOTE_VISIBILITY_UNSPECIFIED` included, so a note
  recorded without a visibility is DM-only; it forwards `NoteDeleted` when
  `notes` held the key before the event, since a fold that never held the note
  refuses its deletion and a key the viewer was never sent is the DM's." The
  Consequence "A player's and a spectator's notes panel is empty." becomes "A
  player's and a spectator's fold holds exactly the notes whose visibility is
  `NOTE_VISIBILITY_PUBLIC`. A note that stops being public reaches them as a
  `NoteDeleted`, the frame a deletion sends, so they cannot tell the two
  apart." "What this record does not decide" gains "who may issue
  `upsert_note`, and its refusal when it names no visibility, are SPEC-013's".
  `Requirements` gains rows C to H. "Principles served" stays: a public note is
  what "the table is told".
- **SPEC-012 does not move** (D8). SPEC-015, SPEC-011, SPEC-014, SPEC-009 and
  SPEC-010 carry no note sentence.

**D14. Six code commits and a report, in this order.** Forced by: the folds
before anything that sends or reads a visibility; the console before the
refusal, so no commit leaves the DM's Save button refused; the refusal before
the projection, so `story-table.json` reaches its final shape once; the
projection before adventure notes, so `adventure-night`'s goldens move once;
and `check:drift`, which passes only after a commit that regenerates the
contract or touches `client/src` carries its rebuilt `cmd/vtt/webdist`. The
product works at every commit: after C3 the DM chooses and sees; after C4 an
agent must choose; after C5 players see public notes; after C6 adventure notes
are public. The table is under **Commits**. Merging C4 into C5 would halve
`story-table`'s churn and double the reviewed diff; not done.

**D15. Phase 4a per commit.** Forced by the dev-cycle skill: every commit that
delivers behaviour a sentence describes gets one QA agent per `qa-prompt.md`,
given the rows and the whole governing text, never the diff, the source, the
existing tests or the implementer's report. The exported surface is `go doc
<package>` (the project's recorded command); the TypeScript surface has no
recorded command and is extracted with `grep -n '^export'` over the files
named, which the dispatch says. What each receives: C2 — row A, the ticket
(no record describes the fold's note arm), `go doc ./internal/engine`, the
exports of `fold.ts` and `state.ts`; C3 — rows J and K, the ticket, the
exports of `view/dm.ts`, `view/spectator.ts` and `commands.ts`; C4 — rows B
and L, SPEC-013 and SPEC-007 whole as C4 leaves them, `go doc
./internal/gateway`; C5 — rows C to H, SPEC-016 whole as C5 leaves it, `go doc
./internal/gateway`; C6 — row I, the ticket, `go doc ./internal/adventure`. C1
is skipped: nothing reads the field until C2, so there is no behaviour to
derive. The report is skipped (prose). QA writes to its own files
(`qa_note_*_test.go`, `client/test/qa-note-*.test.ts`) and its passing tests
are committed with the task; adjudications go in the report under their own
heading, one line per finding; an escape an earlier gate should have caught
goes to `docs/verification-debt.md` as a recipe.

**D16. Phase 4b per commit.** One reviewer at high effort, briefed to verify by
command: every sentence the commit adds to a specification, a proto comment, a
`why`, a tool description or the instructions, against the code that writes
what it describes; every hand-derived golden, by re-deriving one note's entry;
the deliberate-break lines in the draft message. Findings go to Patrik as
Must/Should/Nit; approved fixes land in the same uncommitted set; then
`review-record.sh --summary`, then the commit. If the reviewer dies on a
model's weekly limit, say so and re-dispatch the same brief on `fable`.

**D17. The deliberate breaks, one per new check, both directions.** Each is an
act in a scratch state, repaired before the commit, and one line of the commit
message. The other direction is the repaired tree's clean run. Measured by the
verifier where marked.

| Commit | Check | The break, and what must red |
|---|---|---|
| C1 | VTT-038 over the new enum | `_SECRET` renamed `_DM_ONLY`: `TestQANoEventPayloadNamesARole` red; `TestNoEventPayloadNamesARole` stays green, since it walks field names, not enum values (measured) |
| C1 | `TestNoteUpsertedEnvelopeRoundTrip` and its TS case | the fixture's value misspelt: both red |
| C2 | row A, Go | the `NoteUpserted` arm drops `Visibility`: `TestANoteRecordsTheVisibilityItsLatestUpsertStated` red |
| C2 | row A, TS | `fold.ts` drops it: the fold-unit case red |
| C2 | the dump's omitempty arm | `foldToDumpJSON` always emits `Visibility`: the fold-dump case red |
| C2 | `ToEvent` carries it | the copy removed: `TestToEventUpsertNoteProducesNoteUpserted` red |
| C2 | D4's guard | a constant visibility: `TestAWalkDrawsEveryNoteVisibilityAndAChangeOfOne` red |
| C3 | row J | the Save guard removed: the "not sent until chosen" case red; the builder sending PUBLIC whatever was chosen: the "carries the choice" case red for SECRET |
| C3 | row K | only `SECRET` marked: the unrecorded-visibility case red; `PUBLIC` marked too: the public-unmarked assertion red |
| C4 | row B | the call removed from `handleCommand`: the wire test red while `note_validate_test.go` stays green; the validator refusing SECRET: the enum-walk accept test red |
| C4 | row L | `"visibility"` dropped from the required list: `TestUpsertNoteRequiredOverrideReplacesDerivedList` red |
| C5 | rows E, H | every `NoteDeleted` forwarded: the never-public and delete-after-secret tests, `TestANoteRecordedWithoutAVisibilityReachesNoPlayer` and the property test red (the property test and the split test's deletion assertion measured on the sketch) |
| C5 | row F | no frame when a held note stops being public: the flip test and the property test red (measured) |
| C5 | row C | a `NoteUpserted` of any visibility but SECRET forwarded: the unrecorded-visibility test and the property test red (the property test measured) |
| C5 | row G | a forwarded note never remembered: the flip test, the public-deletion test and the property test red (the flip and property tests measured) |
| C5 | row D | the `NoteUpserted` arm withholding everything: the public-note test, the wire test and the keystone's notes clause red |
| C5 | the keystone's notes clause | the oracle counting SECRET as public: `story-table/act-hero` red (the other direction of the oracle) |
| C6 | row I | `Compile` omitting `Visibility`: `TestCompileValidFixtureExactEnvelopeList`, `TestConformanceOverAdventuresGlob` and the `act-fighter` projected golden red |

**D18. Mutation keys and the ledger, last in each commit.** Forced by the
measured keys and ceilings. After the review settles: `--report` on the
commit's files; `--write-ledger` where a share fell more than 1.0 under its
row (never raising one); each moved key re-pointed by reading the token its
mutator rewrites at the new line and column, never by offset; `python3
tools/check_mutation_test.py -q` and `python3 tools/check_ts_mutation_test.py
-q` print `OK`. Expected: C2 `fold.ts` below the `noteUpserted` arm if the
arm or the dump line grows; C3 `view/dm.ts` (all five) and
`view/spectator.ts` (two); C4 `mcp/read_tools.go` and `mcp/server.go`; C5
`project.go` (three). A new survivor in new code is killed by a test before the
commit; the whole mutation gates run in D14's gate runs.

**D19. Rows are dispensed per task, after sign-off.** Forced by SPEC-008 and
the recorder's whole-tree fingerprint. At the start of each task, `requirement-id
"<the row>"` for that commit's rows (the dispenser writes `**OPEN — no test
yet**`); the test cites the id on its own `// VTT-NNN` line (Go) or in its
file (TS) as it is written; the evidence cell is filled when the test lands, in
the same commit, so no OPEN row reaches `main` unless the sort says so. VTT-216's
evidence is re-pointed in C5 (the sort's last table).

**D20. The comment budget.** Forced by the measured ceilings. A comment line
added to a file at its ceiling needs about (1 / share − 1) code lines added with
it: two for `apply.go` at 43.0, four for `project.go` at 20.4. So the plan
adds warnings only where an editor needs one and code pays for it:
`Projector.notes` (one line: "Forget a key only in noteTransitions, after
classify has ruled on the event"), `noteTransitions`' doc (one line naming
SPEC-016), `validateUpsertNote`'s doc in the new file (at most three lines:
refuse UNSPECIFIED only; never wrap `ErrUnauthorized`; never move it into the
fold). `engine.Apply`'s arm gets none (row A's test holds it). `--report`
before every commit decides, not this estimate.

## The sort

Rows are lettered; nothing is an id before sign-off. Every accepted row is new
work and starts `**OPEN — no test yet**` when dispensed, until its test lands in
the same commit. "Test" names the check that will cite the id; "(new)" means
written first in that commit.

| # | Rule (what) | Proposed | The observation that goes red | Test | Commit, record |
|---|---|---|---|---|---|
| A | A note in the folded state carries the visibility its latest `NoteUpserted` stated, and none when that event stated none. | accept | a `NoteUpserted` of each value folds to a note carrying it, in `engine.Apply` and in `client/src/fold.ts` alike | `internal/engine/apply_test.go#TestANoteRecordsTheVisibilityItsLatestUpsertStated` (new), `client/test/fold-unit.test.ts` "a note records the visibility its latest upsert stated, none included" (new); from C4 `client/test/fold-parity.test.ts` over `story-table` | C2; no record (Gap 3) |
| B | An `upsert_note` that states no visibility is refused, for the DM and the agent alike, with a message naming `visibility`; one stating any visibility the contract offers is accepted. | accept, VTT-151's shape | a visibility-less `upsert_note` from a DM or an agent connection answered ok=true, or a stated one answered ok=false | `internal/gateway/note_validate_test.go#TestAnUpsertNoteWithNoVisibilityIsRefused`, `#TestEveryNoteVisibilityTheContractOffersIsAccepted` (new, internal); `internal/gateway/server_test.go#TestAnUpsertNoteNamingNoVisibilityIsRefusedForTheDMAndTheAgent` (new, wire; also cites VTT-162) | C4; SPEC-013 |
| C | A note recorded without a visibility is sent to no player or spectator. | accept, the owner's fail-closed ruling | a `NoteUpserted` with `_UNSPECIFIED` produces a frame naming the note for a player or a spectator | `internal/gateway/project_test.go#TestANoteRecordedWithoutAVisibilityReachesNoPlayer` (the note half of `TestNarrationReachesAPlayerAndANoteDoesNot`, split out, spectator added); `project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer` (clause: every note a seat holds is public on the server) | C5; SPEC-016 |
| D | A public note reaches every player and spectator, whatever they see. | accept | a player with an actor, a player with none, a spectator perched and one not: any of them is not sent a public `NoteUpserted` | `project_test.go#TestAPublicNoteReachesEveryPlayerAndSpectator` (new; red on today's projection, P3); `server_visibility_test.go#TestAPublicNoteReachesAPlayersConnectionAndASecretOneDoesNot` (new, wire, catches a `ToEvent` that drops the field); `keystone_test.go#TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` (notes clause) | C5; SPEC-016 |
| E | A note never recorded public is named in no frame a player or spectator is sent. | accept, the ticket's fifth rule worded for a note never public | a secret upsert, a secret update or the deletion of a never-public note yields any frame carrying its key | `project_test.go#TestANoteNeverPublicIsNeverNamedToAPlayer` (new); the property test (clause: every note key in a frame sent to a seat was public at or before that event) | C5; SPEC-016 |
| F | A note that stops being public leaves the fold of every player and spectator that held it. | accept, the ticket's sixth rule with "recorded without" folded in | after a public upsert then a secret (or unrecorded) upsert of the key, a seat's fold still holds the note; the test first asserts it held it | `project_test.go#TestANoteMadeSecretLeavesEveryFoldThatHeldIt` (new; also cites VTT-221 for the frame's sequence); the property test (clause, checked after every event: a seat's notes equal the server's public notes) | C5; SPEC-016 |
| G | A deleted note leaves the fold of every player and spectator that held it. | accept, the seventh rule's first half | after a public upsert and its deletion, a seat's fold still holds the note | `project_test.go#TestADeletedPublicNoteLeavesEveryFoldThatHeldIt` (new); the property test | C5; SPEC-016 |
| H | A note's deletion is sent only to a player or spectator whose fold holds the note. | accept, the seventh rule's second half | public, then secret, then deleted: the seat is sent a second `NoteDeleted` and its fold refuses it | `project_test.go#TestADeletionAfterANoteWasMadeSecretReachesNoPlayer` (new); the keystone over `story-table/act-hero` | C5; SPEC-016 |
| I | A note loaded from an adventure is public. | accept | a compiled `NoteUpserted` without `NOTE_VISIBILITY_PUBLIC`; `act-fighter`'s projected seat without `ravine-trail-warning` | `internal/adventure/compile_test.go#TestCompileValidFixtureExactEnvelopeList` (want updated first); `internal/adventure/conformance/conformance_test.go#TestConformanceOverAdventuresGlob`; `internal/gateway/keystone_test.go#TestTheProjectedGoldensAreWhatTheProjectionActuallySends` | C6; no record (Gap 3) |
| J | The DM console sends no `upsert_note` until the DM has chosen who may read it, and sends the choice. | accept, the ninth rule's first half | Save with key and text and no choice sends a command; a chosen value is not the one sent | `client/test/dm-view.test.ts` "a note is not sent until who may read it is chosen", "a saved note carries the visibility chosen" (new) | C3; no record (Gap 3) |
| K | The notes panel marks every note that is not public as DM only. | accept, the ninth rule's second half | a SECRET or an unrecorded note shown unmarked, or a PUBLIC note marked | `client/test/spectator-view.test.ts` "the notes panel marks every note that is not public as DM only" (new) | C3; no record (Gap 3) |
| L | The `upsert_note` tool's schema lists `visibility` as required. | accept, the ticket's first item; toolgen's own reason (an LLM omits what the schema does not require) | the generated schema's `required` for `upsert_note` without `visibility` | `tools/toolgen/main_test.go#TestUpsertNoteRequiredOverrideReplacesDerivedList` (want updated first) | C4; SPEC-007 |

**Refused, one line each:**

1. "The DM and the agent are sent every note event unchanged" — VTT-176 holds it
   (`TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`), as the ticket says.
2. "A refused `upsert_note` appends nothing" — VTT-162's rule; B's wire test
   cites VTT-162 as well.
3. "A frame a note change produces carries its event's sequence" — VTT-221;
   F's test cites it.
4. "Every note frame a seat is sent folds" — VTT-224; the property test
   already cites it.
5. "A player's and a spectator's panel lists the public notes" — follows from
   D and `renderNotes`' existing listing ("notes render with title and body").
6. "A command's visibility reaches its event" — how D is achieved; D's wire
   test observes it.
7. "`get_state` and the instructions say what visibility is" — prose for an
   agent: the two existing string tests gain a clause (D10) and Phase 4b reads
   the sentences; a string check cannot tell a true description from a false
   one, and neither test carries a row today.
8. "The adventure format does not change" — a constraint on the work, with
   nothing in the system to observe.
9. "No visibility value names a role" — VTT-038 holds it (P1).
10. "The contract change is additive" — rule 3; `check:breaking` reports only.
11. "A load in the residue window overwrites a DM-only note with a public
    one" — a known defect in the debt file, not a rule (D8).
12. "A perch sends nothing about notes" — no table consequence: a re-sent
    public note folds as an upsert of the same content.
13. "Narration stays forwarded to all" — VTT-216, unchanged.
14. "The console's select starts blank" — how J is achieved.
15. "A player cannot tell a deletion from a note made secret" — a Consequence
    of D6, stated in SPEC-016; E and F are the rules it protects.

**Existing rows that move:**

- **VTT-216** — its evidence names
  `internal/gateway/project_test.go#TestNarrationReachesAPlayerAndANoteDoesNot`.
  C5 splits that test: its narration half becomes `TestNarrationReachesAPlayer`
  and keeps `// VTT-216`; its note half becomes row C's test. The evidence cell
  is re-pointed to `#TestNarrationReachesAPlayer` in C5.
- **VTT-161** — evidence unchanged; its test's valid upsert gains a visibility
  in C4 (D12).
- **VTT-225, VTT-224, VTT-212** — evidence unchanged; the keystone and the
  property test gain note clauses in C5, and their row text already covers
  them.
- **VTT-217, VTT-219** — evidence unchanged;
  `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`' goldens move in
  C4, C5 and C6.
- **VTT-038, VTT-151, VTT-153, VTT-176** — unchanged; relied on.

## Tasks, in dependency order

Each task is one commit (D14) and runs Phase 3, 4a, 4b and 5 for it. "Test
first" names what is written before the code and how it fails on the tree the
task starts from.

### Task 0 — Instruments and baselines

**Files:** none in the repository.

`uptime`; `python3 tools/check-requirements-chain.py .`; `python3
tools/check-comments.py --report` filtered to the files above; both mutation
self-tests; `which requirement-id`; `git status --short` (only the ticket and
this plan untracked). Keep the outputs.

**Done when:** they match Measurements (227 rows, the fifteen shares, `OK`
twice, the dispenser found).

### Task 1 — The contract (C1)

**Files:** `contract/vtt/v1/events.proto` (the enum, its comment, the field),
`contract/vtt/v1/commands.proto` (the field); regenerated
`contract/gen/go/vtt/v1/`, `contract/gen/ts/vtt/v1/`,
`contract/gen/tools/tools.json` and `cmd/vtt/tools.json` (`task
generate:contract`); `contract/testdata/expected_tools.json`; new
`contract/testdata/note_upserted_envelope.json`; `contract/roundtrip_test.go`;
`contract/events.test.ts`; `cmd/vtt/webdist` (`task build:client`).

**Rows:** none.

**Test first:** `TestNoteUpsertedEnvelopeRoundTrip` and the fixture's case in
`contract/events.test.ts` — red today, `unknown field "visibility"` (measured).

**Do:** D1. The tool schema gains the property and its two values; its
`required` does not change until C4.

**Done when:** `go tool buf lint` clean; `task check:breaking` names no break;
`go test ./contract/ ./tools/toolgen/ ./internal/engine/ ./internal/gateway/
-run 'RoundTrip|ToolsMatchGolden|NoEventPayloadNamesARole'` green; `bun test
contract/` green; the `tools.json` diff is the one property; Phase 4b recorded;
committed with D17's two break lines; `task check:drift` green after the
commit.

### Task 2 — The folds (C2)

**Files:** `internal/engine/state.go`, `apply.go`, `apply_test.go`;
`internal/gateway/convert.go`, `convert_test.go`; `internal/eventgen/model.go`,
`model_test.go`; `internal/campaign/states_equal_notes_test.go`,
`internal/harness/states_equal_notes_internal_test.go` (a case differing only
in `Visibility`); `client/src/state.ts`, `client/src/fold.ts`;
`client/test/fold-unit.test.ts`, `fold-dump.test.ts`, and the note literals in
`client/test/spectator-view.test.ts` (typecheck); `cmd/vtt/webdist`;
`docs/requirements.md` (row A); the ledger and keys as D18 finds.

**Test first:** `TestANoteRecordsTheVisibilityItsLatestUpsertStated` (PUBLIC,
SECRET and UNSPECIFIED each recorded; a later upsert replaces it) — does not
compile on the C1 tree (`engine.Note` has no `Visibility`); the fold-unit case
(red: `Visibility` undefined); the fold-dump case "a note's visibility is
dumped when stated and omitted when not" (red); the convert test's new
assertion (red: `ToEvent` drops it); D4's guard (red: every draw is
UNSPECIFIED).

**Do:** D2, D3, D4.

**Done when:** `go test ./internal/engine/ ./internal/gateway/
./internal/eventgen/ ./internal/campaign/ ./internal/harness/` green; `bun
test` and `task client:typecheck` green; `git diff --stat -- scenarios
adventures` prints nothing; the chain checker prints 228 rows; Phase 4a's QA
tests pass and are committed; Phase 4b recorded; D18; committed; `check:drift`
green after.

### Task 3 — The DM console and the notes panel (C3)

**Files:** `client/src/commands.ts`, `client/src/view/dm.ts`,
`client/src/view/spectator.ts`; `client/test/commands.test.ts`,
`dm-view.test.ts` (the trim test chooses a visibility; the key-and-text guard
stays first, so "each guard refuses with its own exact wording" keeps its note
case and gains the new message), `spectator-view.test.ts`;
`contract/testdata/upsert_note_command.json`; `cmd/vtt/webdist`;
`docs/requirements.md` (J, K); `tools/ts-mutation-equivalents.txt` (D18).

**Test first:** "a note is not sent until who may read it is chosen" (red:
Save sends today); "a saved note carries the visibility chosen", for each
value (red: no select); "the notes panel marks every note that is not public as
DM only" (red: no mark); "upsertNote matches the committed fixture" with the
fixture's visibility (red: the builder has no parameter).

**Do:** D9; and Q3's answer on `describe`'s note label, if yes.

**Done when:** `bun test`, typecheck and lint green; `bunx vite build` run and
`cmd/vtt/webdist` staged; the chain checker prints 230 rows; QA and review
recorded; D18; committed; `check:drift` green after; `task check` whole per
Q6.

### Task 4 — The command path (C4)

**Files:** new `internal/gateway/note_validate.go` and
`note_validate_test.go`; `internal/gateway/server.go`; `server_test.go`;
`tools/toolgen/main.go`, `main_test.go`; regenerated
`contract/gen/tools/tools.json`, `cmd/vtt/tools.json`;
`contract/testdata/expected_tools.json`; `internal/mcp/read_tools.go`,
`server.go`, `read_tools_test.go`, `server_test.go`;
`cmd/vtt/mcp_e2e_test.go`; `scenarios/story-table.json` and D11's C4 goldens;
`docs/specifications/007-the-wire-contract.md`, `013-authorization.md`;
`docs/requirements.md` (B, L); `tools/mutation-equivalents.txt` (the three
`internal/mcp` keys); the ledger (the new file's row).

**Test first:** `note_validate_test.go` (does not compile: no
`validateUpsertNote`); the wire test (red: the DM's and the agent's
visibility-less upserts are accepted); the toolgen test wanting `[key text
visibility]` (red); the two MCP string clauses (red).

**Do:** D5, D10, D12, and D13's SPEC-007 and SPEC-013 sentences; the
`NoteVisibility` comment in `events.proto` gains "An UpsertNote naming
NOTE_VISIBILITY_UNSPECIFIED is refused.", regenerated with the rest. The
`story-table` goldens per D11: `state.json` by hand, then the stream
re-recorded, then `act-hero`'s stream copied from the emitted output and read,
its `state.json` by hand.

**Done when:** `go test ./internal/gateway/ ./internal/mcp/ ./internal/harness/
./cmd/vtt/ ./tools/toolgen/` green (the four P2 reds green again);
`TestFoldGoldenCorpus` and `client/test/fold-parity.test.ts` green over
`story-table`; the chain checker prints 232 rows and 10 specifications;
SPEC-013's list numbered 1 to 9; QA and review recorded; D18;
committed.

### Task 5 — The projection (C5)

**Files:** `internal/gateway/project.go`; `project_test.go`;
`server_visibility_test.go`; `project_property_test.go`; `keystone_test.go`;
`scenarios/goldens/story-table/projections/act-hero/stream.json`, `state.json`,
`viewer.json`; `docs/specifications/016-the-projection.md`;
`docs/requirements.md` (C to H; VTT-216's evidence);
`tools/mutation-equivalents.txt` (the three `project.go` keys); the ledger.

**Test first:** the five new project tests and the split that gives row C its
test (D's is red on the C4 tree, P3; F's and H's first assert the seat held
the note, or they are green on the C4 tree for nothing sent, P3); the wire test
(red); the keystone's notes clause — at every prefix a player's or spectator's
folded notes equal the world's notes whose visibility is
`NOTE_VISIBILITY_PUBLIC`, entry for entry, derived from the ruling and not from
`classify` — red over `story-table/act-hero`, where `kobold-den` is public and
withheld; the property clauses — after every
event a seat's notes, applied as the frames arrive, equal the server's public
notes, and every note key in a frame was public at or before that event — red,
with two per-seed floors below P6's counts (a player is sent a public note in at
least five seeds of six; a note leaving because it stopped being public in at
least two).

**Do:** D6; D13's SPEC-016 sentences; `act-hero`'s stream and state per D11
and its `why`.

**Done when:** `go test ./internal/gateway/ ./cmd/vtt/` green;
`client/test/projection-parity.test.ts` green over `act-hero`; the chain
checker prints 238 rows and VTT-216 names `TestNarrationReachesAPlayer`;
`grep -n 'notes panel is empty' docs/specifications/016-the-projection.md`
prints nothing; QA and review recorded; D18; committed.

### Task 6 — Adventure notes are public (C6)

**Files:** `internal/adventure/compile.go`, `compile_test.go`;
`internal/adventure/conformance/conformance.go`; the five
`compiled-batch.json`; `scenarios/goldens/adventure-night/state.json`,
`stream.json`, `projections/act-fighter/stream.json`, `state.json`,
`viewer.json`; `docs/requirements.md` (I).

**Test first:** `TestCompileValidFixtureExactEnvelopeList` with
`NOTE_VISIBILITY_PUBLIC` in its want (red); the five goldens by hand (red:
`TestConformanceOverAdventuresGlob`, `TestRunValidAdventure`); `act-fighter`'s
state by hand, holding `ravine-trail-warning` (red: the projected-goldens
test).

**Do:** D7; the `adventure-night` goldens per D11.

**Done when:** `go test ./internal/adventure/... ./internal/gateway/
./internal/harness/ ./cmd/vtt/` and `bun test` green; `grep -c
NOTE_VISIBILITY_PUBLIC` prints 1 for each of the five goldens; the chain
checker prints 239 rows; QA and review recorded; committed.

### Task 7 — The whole gate

**Files:** none changed unless it finds something.

`uptime`; `task check` detached with `start_new_session=True`, once after C3
and once after C6 per Q6, each after its review settled and its keys moved.

**Done when:** each run exits 0 with every step printing its own completion
line (`check:comments`, `check:requirements-chain`, `check:drift`,
`check:breaking`'s report, `check:mutation`, the TS mutation gate, coverage,
race, `lint`). A failure is fixed in a new commit through the same cycle.

### Task 8 — The report (C7), then the records, then push

**Files:** new `docs/reports/2026-09-29-a-note-says-who-may-read-it.md`.

Per the `implementation-report` skill, against the ticket's nine items in its
numbering, naming C6 as the last code commit: the rows with their ids and
tests, the fifteen refusals, the QA adjudications under their own heading, the
break lines, the rule-9 answer, the goldens and how each was made, the gaps
below as found or closed, and the sign-off answers. Its own reading review and
commit; no QA. Then the Phase 5 record step: re-read SPEC-007, SPEC-013 and
SPEC-016 against the final tree; commit only what moved. Fetch, check the
branch against `origin/main`, merge if behind, push.

**Done when:** `git log --oneline main..` lists C1 to C7 (and a record commit
only if one was needed); `git status --short` prints nothing.

## Commits

| Commit | Carries | Gate steps before; after |
|---|---|---|
| C1 | enum and fields, generated Go/TS/tools, `expected_tools.json`, the envelope fixture and both round trips, `webdist` | the task's tests and the pre-commit hook; `check:drift` after |
| C2 | the folds: `engine.Note`, the `NoteUpserted` arm, `ToEvent`, eventgen and its guard, the equality cases, `state.ts`, `fold.ts` and its dump, their tests, `webdist`, row A | hook; `check:drift` after |
| C3 | the console and the panel: `commands.ts`, `dm.ts`, `spectator.ts`, their tests, the command fixture, `webdist`, rows J K | hook; `check:drift` after; `task check` whole (Q6) |
| C4 | the command path: the validator and its call, toolgen, both `tools.json`, `expected_tools.json`, the MCP prose and tests, `server_test.go`, `mcp_e2e_test.go`, `story-table.json` and its C4 goldens, SPEC-007, SPEC-013, rows B L | hook |
| C5 | the projection: `project.go`, the project, wire, property and keystone tests, `act-hero`'s goldens, SPEC-016, rows C to H, VTT-216's evidence | hook |
| C6 | adventure notes: `compile.go`, `compile_test.go`, `conformance.go`, the five batches, `adventure-night`'s goldens, row I | hook; `task check` whole |
| C7 | the implementation report | hook |

Every commit also carries the ledger and keys D18 finds for it. Push after C7:
pre-push runs tiers 2 and 3, `check:drift` and `check:breaking`, about three
minutes.

## Gaps that travel with this plan

1. **A note's visibility is changed by re-saving it whole.** The console has
   no per-note "make public / make DM only" control; the DM retypes the key and
   text. An agent flips one in one call.
2. **The skipped handover test** (`client/e2e/handover.spec.ts`, "a network
   drop while the table plays on ends in a Reconnect") fills the note form and
   saves; un-skipped, it must choose a visibility first. Out of this ticket, as
   the ticket says.
3. **Rows A, I, J and K are named by no specification**: the engine's fold,
   the adventure compile and the client have no record. The chain allows it;
   the `requirements` skill's "a specification names the requirements it
   carries" does not hold for them. Q10.
4. **No corpus gate holds note steps.** The gateway's refusal holds every
   ok-expecting step; a denied step could carry the refused shape, as
   `TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence` forbids for
   actors. `story-table`'s one denied upsert is given a visibility anyway.
5. **The keystone never sees `_UNSPECIFIED`**: the corpus goes through the
   gateway, which now refuses it. Row C is held by its unit test and the
   eventgen walks.
6. **The residue grows** (D8): a load can replace a DM-only note on an
   adventure's key with the adventure's public note. No text leaks.
7. **`get_state` prints the visibility as a number**, as it prints an actor's
   kind; the description names the numbers.
8. **A player's feed says `note "<key>" deleted`** when a note is made DM only,
   unless Q3 changes the label.
9. **Two of the ticket's sentences are loose** (check 2, check 3): item 4's
   clause about a deleted DM-only note, and the fifth rule. Row E is the
   plan's wording.
10. **Every edited file is at its ceiling** (D20): a warning comment costs code
    lines beside it, and the ledger falls where code outgrows comments.

## Questions for sign-off

1. **The values: `NOTE_VISIBILITY_PUBLIC` and `NOTE_VISIBILITY_SECRET`?**
   Recommend yes (D1): `_DM_ONLY` reds VTT-038, `_GM_ONLY` dodges it by
   spelling, `_PRIVATE` does not say to whom, `_HIDDEN` collides with the
   projection's `TokenHidden`. The console labels say "public" and "DM only".
2. **A note leaving a player's view as a synthesized `NoteDeleted`, or a new
   projection-only `NoteHidden` sent for both a withdrawal and a deletion?**
   Recommend the synthesized `NoteDeleted` (D6): information-equivalent, no new
   message or fold arm, and a memory defect is refused loudly by both folds.
   Sending `NoteHidden` for a withdrawal and `NoteDeleted` for a deletion is
   refused either way: it would tell a player the DM still holds the note.
3. **`describe`'s label for `noteDeleted`: keep `note "<key>" deleted`, or say
   something true for a deletion and a withdrawal alike?** Recommend `note
   "<key>" withdrawn`, in C3 (a `spectator.ts` string and its pinned case in
   "describe renders a real label for every event kind"); the ticket does not
   name the change.
4. **`engine.Note.Visibility` tagged `json:",omitempty"`?** Recommend yes
   (D2): no golden moves until a note carries a visibility, the way `Scene`'s
   `Explored` and `Visible` were added; an agent reads a missing key as
   "recorded without one", which the description says.
5. **Specification sentences in the commit whose code makes them true, rather
   than one record commit after the report?** Recommend yes (D13): every
   sentence stays true at every commit, and C5's QA derives from a SPEC-016
   that already says what C5 delivers. The Phase 5 record step still runs after
   the report, and commits only what moved.
6. **`task check` whole twice, after C3 and after C6, or once after C6?**
   Recommend twice: the TypeScript mutation gate and the drift gate settle on
   the client while its three commits are the newest, and the Go side is
   judged whole at the end. Each run is over an hour.
7. **One clause in the residue's debt entry, and SPEC-012 untouched?**
   Recommend both (D8); the ticket does not list `docs/verification-debt.md`,
   so the clause needs the yes.
8. **`story-table.json` gains one step (old-rumor re-upserted secret before its
   deletion)?** Recommend yes (D12): it is the case where a wrong memory breaks
   a fold, and the only golden where the TypeScript fold is held to it.
9. **Correct the MCP instructions' "human players can see everything you do"
   in C4?** Recommend yes (D10): the note sentence added two lines below would
   contradict it.
10. **Accept rows A, I, J and K with no specification naming them?** Recommend
    yes, with Gap 3 carried to the report: each is breakable and cited by a
    test, and writing a record for the engine, the adventure compile or the
    client is not this ticket.
11. **Rewrite `act-fighter`'s `why` without its counts?** Recommend yes (D11):
    C6 makes "8 of its 11" false, and a count rots by addition where a name of
    what is forwarded does not.
12. **The console's select blank by default, the DM answering each time?**
    Recommend yes (D9), following `kindSelect`: a pre-filled answer cannot be
    told from a DM who never looked. The alternative, pre-selecting "DM only",
    fails closed but makes every public note an extra click the DM may forget.
