# A token carries what a player may see of a creature, and a player is sent no creature's name or statblock: the change

**Ticket:** `docs/superpowers/specs/2026-10-24-a-token-carries-what-a-player-may-see-of-it-design.md`.
An agent that did not write it verified it. The first verdict returned it on
three findings (F1-F3); the writer revised it, and the second verdict passed
it with one gap (H1: hover, the card's click-through and the console are
observed only in a real browser).
**Plan:** `docs/superpowers/plans/2026-10-24-a-token-carries-what-a-player-may-see-of-it.md`,
written by that verifier from a prototype in a separate worktree, and amended
by the reading review of `471c3c6`, the commit that carried both.
**Where it came from:** rulings A-6, K-1 and B-5, with A-2 and A-10, in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`, the fifth of the
nine tickets in the order the owner agreed; and the unstable gateway test the
owner folded into it ("ta ticket 5 och rätta det instabila testet i den").
**The owner's rulings, 2026-10-24:**
- "Inget hp; utfallsraden står": no attributes, resources or module data of a
  creature reach a player, and `outcome_summary` is sent as it is.
- "Ja, ett eget kommando": the DM and the agent rewrite a placed token's four
  texts with their own command.
- "Ja, som i dag": a left click still moves (A-5).
- "En ticket, servern först".
- After the first verdict: "Ja, en kontroll i konsolen"; and on the ability's
  Story line, "man visar att det har blivit träff, men inte 'varför' … bara
  longsword strike hit (dice: 17) typ".
- The plan signed off: "Ja, kör".
- 2026-10-25, "Efter rapport och specar": the whole gate runs once, after this
  report and the specifications are written and reviewed, and its result goes
  into this report before its commit.

**Last code commit:** `eb6d423`, on `1e24277`, `main` at the time. Every code
reference below is to its tree. `a2f7693` and `eb6d423` follow Task 7's
`74ab287`, each for a stop of the gate (Deviations), and the gate's figures
are from `eb6d423`.

## The period, in commits

| Commit | What it holds |
|---|---|
| `471c3c6` | Task 0: the ticket, the plan, VTT-462 to VTT-484, and the withdrawal of VTT-247, 248, 313 and 440 |
| `dea1065` | Task 1: the unstable test's handle opens with `busy_timeout(5000)` |
| `12bf268` | Task 2: the contract's four texts, `DescribeToken`/`TokenDescribed` and `AbilityUsed.outcomes`, the tools, `ToEvent`, `commandRoles`, the MCP lists, the console's Describe group, QA's two `qa_describe_token` files, the bundle |
| `aa25462` | Task 3: both folds keep the texts and apply a rewrite, `foldToDumpJSON`'s escaping, the shared dump fixtures, `eventgen`'s placements and rewrites, the console's prefill, QA's three `qa_token_texts` files, a debt recipe, the bundle |
| `6dc87dd` | Task 4: the adventure and map loaders, the conformance dump, goblin-ambush's texts, adventure-night's golden, `docs/map-format.md` §6, QA's `qa_placement_texts` files |
| `81eaae6` | Task 5: what a character log is sent of a creature and of a token's texts (`internal/perceive/projection.go`), N1-N8, Gw5, the edited tests, the removal-batch fixture, QA's three `qa_creature_shown` files, VTT-485 and VTT-486 |
| `2d53c54` | Task 6: the sight gate for a player's `use_ability`, `perceive.SeesActor`, `rules.UnknownTarget`, `Resolve`'s outcomes, two scenario goldens, QA's three ability files, VTT-487 and VTT-488 |
| `74ab287` | Task 7: `client/src/view/naming.ts`, the card, `describe` over every payload case, the target buttons, the e2e, QA's `client/test/qa-what-a-window-shows.test.ts`, VTT-489 and VTT-490, the bundle |
| `a2f7693` | `client/src/view/naming.ts`'s coverage floor, 100.00 |
| `eb6d423` | The console's describe boxes take the token's text from their row, `draft`'s comment loses its history, the bundle |

## What the code did on `1e24277`

A token was a placement: `PlaceToken`, `TokenPlaced` and `engine.Token` held
an id, a scene, an actor and a position. When a creature's token came into a
character's sight, `Projection.introduce` in `internal/perceive/projection.go`
sent that character's log a copy of the whole actor, its controllers cleared:
its name, attributes, resources and module data. `classify` forwarded every
`ResourceChanged` of an actor the character saw. In the browser, `tokensOnScene`
and `renderDisc` drew the actor's name as the initial and the title, each
resource as a chip, and `describe` wrote "actor Goblin Archer joined", named
other events by token or actor id, and an event it had no wording for by its
payload's name. `renderPlayerPanel` labelled a target button by the actor's
name. A player's `use_ability` was authorized by caster ownership alone, so a
player could aim at an actor no character of theirs saw, and the answer told
them whether the id existed. `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`
renamed a table through its own handle with no `busy_timeout`, and failed
when the server was still reading.

## Done looks like, answered

1. [x] `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` in
   `internal/gateway/server_test.go` opens its handle with
   `?_pragma=busy_timeout(5000)`. `dea1065`'s message records the probe:
   without the timeout the rename failed after 1 ms with `SQLITE_BUSY`, with
   it the rename waited 343 ms for the read and succeeded. `go test
   -count=1000 -run '^TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody$'
   ./internal/gateway/` passed.
2. [~] A token carries `name`, `notes`, `gm_name` and `gm_notes`:
   - the contract: `TestTokenPlacedWithTextEnvelopeRoundTrip`,
     `TestTokenDescribedEnvelopeRoundTrip` and
     `TestDescribeTokenCommandRoundTrip` in `contract/roundtrip_test.go`;
   - the folds: `TestATokenPlacedKeepsItsFourTexts` in
     `internal/engine/apply_test.go`, and fold-unit's "a placed token keeps
     its four texts";
   - the loaders: `TestAPlacementCarriesItsFourTextsIntoTokenPlaced`
     (`internal/adventure/compile_test.go`) and
     `TestAMapPlacementCarriesItsFourTextsIntoTokenPlaced`
     (`internal/mapdef/compile_test.go`);
   - the dump both folds write: `TestTheDumpOfANamedTokenCarriesItsTexts`
     in `cmd/vtt`, and fold-unit's "the dump of a named token matches the
     committed dump, as Go's writeDump does", over
     `contract/testdata/named_token_log.json` and `named_token_dump.json`.

   No bound applies to the texts. What moved: the ticket's "SPEC-018's second
   table names them" is not true at this report's commit. A specification
   follows its report, so the commit after this one adds them to that table,
   where `grep -n 'gm_name' docs/specifications/018-the-fold-bounds-free-text-in-bytes.md`
   finds them.
3. [x] `describe_token` rewrites all four texts:
   - `TestOnlyTheDMAndTheAgentMayDescribeAToken` (`internal/gateway/authz_test.go`)
     refuses a player's and a spectator's;
   - `TestATokenDescribedReplacesAllFourTexts` and fold-unit's "a token's
     description replaces all four texts, an emptied one included" hold the
     folds;
   - `TestADescriptionOfNoTokenIsRefusedAndChangesNothing`, fold-rejections'
     "describing an unknown token is rejected" and the live
     `TestADescribeOfNoTokenIsRefusedAndAppendsNothing` hold the refusal;
   - dm-view's "the console draws a describe row per token of the world, by
     id" and "Describe sends describe_token for its row's token with the four
     fields as typed, empty ones included" hold the console, and
     `COMMAND_SURFACE` declares `describeToken` (`client/test/command-surface.test.ts`).
4. [x] `TestACreatureIsIntroducedWithItsIdModuleKindAndConditionsAlone`
   (`internal/perceive/roster_test.go`) introduces a creature by its id,
   module and kind, its grant and condition behind it, and the party member in
   the same look whole. `TestAPlayerIsSentNoFrameNamingACreatureItSees`
   (`internal/gateway/token_text_test.go`) holds it over a live connection:
   no player frame carries the creature's actor name, `vigor` or the token's
   GM name, and the DM's stream carries all three. Neither compiles on
   `1e24277`, which has no token texts; both were red on `6dc87dd` with them
   laid over it, whose projection is `1e24277`'s plus the arm that withholds
   `TokenDescribed`.
5. [x] `TestACreaturesResourceChangeReachesNoLog`
   (`internal/perceive/status_test.go`) withholds a creature's change, seen
   or not, and forwards the party member's own. It was red on `6dc87dd`, as
   in item 4.
6. [x] `TestATokenArrivesWithItsNameAndNotesAlone` and
   `TestATokensRewriteReachesALogOnlyInSightAndOnlyWhenItsTextChanged`
   (`internal/perceive/board_test.go`): a token arrives with its name and
   notes and no GM text; a GM-only rewrite sends nothing; a name rewrite in
   sight sends one `TokenDescribed` without GM fields; one out of sight sends
   nothing, and the token comes back with its present name. Both were red on
   `6dc87dd`, as in item 4.
7. [x] `TestACreatureInSightIsNotIntroducedAgain` (`internal/perceive/history_test.go`)
   runs twelve events with no `ActorRemoved` or `ActorAdded` of the creature;
   it was green on `6dc87dd`, as in item 4, and guards the new comparison.
   `TestACreatureAGrantMakesAPartyMemberInSightArrivesWhole` and
   `TestAPartyMemberAGrantMakesACreatureInSightIsHeldStripped`
   (`status_test.go`) hold the two kind changes with their exact frame lists;
   both were red on `6dc87dd`. `TestAReshapingKindChangeOutOfSightWaitsForSight` holds the
   unseen case in both directions.
8. [x] In `internal/gateway/server_visibility_test.go`, over a live server
   with `dnd45e-minimal` and a hall split by a wall:
   `TestAPlayersAbilityAtATargetBehindAWallIsRefusedAsAnUnknownTarget`
   answers `rules.UnknownTarget("far").Error()` and leaves the head where it
   was; `TestAnUnseenTargetAndAnAbsentOneAreAnsweredAlikeWhateverTheCommand`
   compares four pairs; `TestAPlayersAbilityAtASeenTargetInRangeResolves`
   resolves; `TestTheDMsAndTheAgentsAbilitiesAreNotJudgedBySight` resolves
   for both behind the wall. The first two were red on `81eaae6` with them
   laid over it and `rules.UnknownTarget` added, which only names `Resolve`'s
   existing answer.
9. [x] `client/test/token-card.test.ts`: "a player's card holds the token's
   name and notes and no GM text", "a DM's card adds what players see and the
   GM notes", "a right-click on a disc is prevented and pins its card, which a
   repaint keeps", "a click on a disc still reaches the board's click
   handler"; app.test.ts's "a right-click pins a disc's card across an event's
   repaint, and a second right-click unpins it". The hover itself is CSS, and
   the e2e "a player's ability reaches the creature they aim it at" hovers
   `tok-cutter`, reads its card, right-clicks it and moves the pointer away.
10. [x] `client/test/naming.test.ts`'s "a player's window names a token by its
    name, else a party member's name, else a creature" and "a DM's window names
    a token by its GM name, else its actor's name, else a creature";
    token-card's "a player's disc shows the naming's initial and title, ? and
    a creature for an unnamed one" and "the target buttons are labelled by the
    window's naming, never by an id"; spectator-view's "a disc is titled by
    name, falling back to `a creature`, never an id"; app.test.ts's "the DM's
    and the agent's windows name a token by its GM name, and a player's by its
    name".
11. [x] spectator-view's "describe renders a real label for every event kind,
    not the fallback" gives an exact line for every payload case and checks it
    covers every case of `EnvelopeSchema`'s oneof; "no Story line is a
    payload's name, an actor or token id, or someone joining" walks the oneof
    by reflection. token-card's "an ability's line names its outcome by the
    window's naming and never the compared total" reads `Lera used Longsword
    Strike on a hunched goblin: hit, rolled 17` from a summary holding "(17 vs
    15)". The e2e reads `Human Fighter used Longsword Strike on a hunched
    goblin: (hit|miss), rolled N` and no row with " vs ". On `1e24277` the line
    was "act-fighter used longsword-strike".
12. [x] `adventures/goblin-ambush` names `tok-cutter` "a hunched goblin" (GM
    name "Goblin Cutter") and `tok-archer` "a goblin with a bow" (GM name
    "Goblin Archer"); `TestConformanceOverAdventuresGlob` checks its hand-edited
    golden, and `TestQAPTGoblinAmbushNamesItsTwoCreatures` its texts. The e2e
    aims at "a hunched goblin", records every frame the player's page
    receives, finds none holding "Goblin Cutter", `act-cutter`'s
    `resourceChanged` or its `resources`, and requires one to be the
    `abilityUsed`. On Task 6's commit, with Task 3's bundle, it and "the story
    feed and ticker record what happened" went red and 18 passed.
13. [x] `TestPlaceTokenOffersTheFourTextsAsOptional`,
    `TestDescribeTokenStatesAllFourAtOnce` and
    `TestUseAbilitySaysAPlayersTargetMustBeSeen` in `tools/toolgen/main_test.go`;
    `TestToolsMatchGolden` against `contract/testdata/expected_tools.json`;
    `check:drift` in the gate below.
14. [x] Each commit lists the tests it edited, and each keeps what it was
    written to hold. The plan named 16 Go tests and 28 client tests from its
    prototype. Beyond them: the exact-envelope and load tests' expectations
    for `internal/adventure`'s `tok-brace` (Task 4), eleven `internal/rules`
    tests that compare a whole `AbilityUsed` (Task 6), and every direct
    caller of `describe` and `tokensOnScene` in the client (Task 7). The e2e's story test reads
    `/^Lera is at \d+,\d+$/`.

## What the rules became

| The ticket's rule | Ids |
|---|---|
| A token carries a name and notes for players and spectators, and GM texts for the DM and the agent alone | VTT-462, VTT-478 |
| Only the DM and the agent may rewrite a placed token's four texts | VTT-463, VTT-479, VTT-480 |
| The DM console offers the rewrite for each placed token | VTT-464 |
| A character log is sent a creature without its name, attributes, resources or module data | VTT-465 |
| A character log is sent no resource change of a creature | VTT-466 |
| A character log is sent a rewrite only in sight and only when the name or notes changed | VTT-467 |
| A player's ability is refused, as an unknown target is, for a target the judging eyes do not see | VTT-472, VTT-482 |
| The browser shows a token's card on hover and right-click, with the GM's texts to the DM | VTT-474, VTT-483 |
| The browser names an actor to a window that is not the DM's by the token's name or a party member's name, never by an id | VTT-475, VTT-484 |
| No Story line is a payload's name or an id | VTT-476, VTT-475, VTT-484 |
| The Story shows an ability's result and roll, never the defense | VTT-477, VTT-473 |

The unstable test puts no rule on the system.

Two rules the plan established beside the ticket's: VTT-468 (a kind change in
sight withdraws and re-introduces the actor, its tokens hidden and placed
again) and VTT-481 (one out of sight waits for sight).

The four withdrawn rows and their successors: VTT-247 by VTT-471, VTT-248 by
VTT-469 and VTT-465, VTT-313 by VTT-470, VTT-440 by VTT-475. No code cites a
withdrawn id: `grep -rn -E 'VTT-(247|248|313|440)([^0-9]|$)' internal cmd client tools`
prints nothing.

Six rules surfaced in Phase 4a earned ids during the work, five of them
sentences QA had to quote and the last a behaviour defect in the plan's rule:
- VTT-485: a character log is introduced to a party member whole;
- VTT-486: a character log is sent a party member's `ResourceChanged` it saw;
- VTT-487: a player's ability is not refused by sight when the eyes see every
  target;
- VTT-488: a player or spectator is sent `outcome_summary` as the log holds it;
- VTT-489: a window names what its fold has lost by the newest log entry that
  named it;
- VTT-490: a DM's or agent's card shows what a player's window names the
  token.

Every row from VTT-462 to VTT-490 names its tests; none is open.

## Phase 4a

One QA agent per task that delivered behaviour, blind to the implementation,
the existing tests and the diff, given the ticket's items, the rows, the
governing specifications and the exported surface (`go doc`, or the client's
`tsc --emitDeclarationOnly` declarations). Task 0 and Task 1 had no behaviour
to derive.

| Task | Tests, as committed | Failures |
|---|---|---|
| 2 | 13 Go in `internal/gateway/qa_describe_token_test.go`, 11 TS in `client/test/qa-describe-token.test.ts` | 2 |
| 3 | 14 Go in `internal/engine/qa_token_texts_test.go`, 6 Go in `internal/gateway/qa_token_texts_test.go`, 22 TS in `client/test/qa-token-texts.test.ts` | 1 |
| 4 | 7 in `internal/mapdef`, 8 in `internal/adventure`, 1 in `internal/adventure/conformance` | 0 |
| 5 | 31 in three `qa_creature_shown_test.go` files (`perceive`, `campaign`, `gateway`) | 0 |
| 6 | 26 in `qa_ability_sight_test.go`, `qa_ability_outcomes_test.go`, `qa_sees_actor_test.go` | 0 |
| 7 | 30 in `client/test/qa-what-a-window-shows.test.ts` | 1 |

### QA adjudications

- Task 2, finding 1: behaviour defect — the console's notes boxes were inputs, against the plan's D16 textareas — fixed with a `textarea` helper in `dm.ts`.
- Task 2, finding 2: spec ambiguity — `place_token`'s `name` doc extends the plan's text with K-1's example and a caution — kept; QA's expectation follows it.
- Task 3, finding 1: behaviour defect, older than the ticket — `foldToDumpJSON` wrote `<`, `>`, `&`, U+2028 and U+2029 raw where `writeDump` escapes them — fixed with `goEscape`; recipe in `docs/verification-debt.md`.
- Task 3: QA's `TestQATTCLIStateDumpOfTheSharedLog` ran only with a hand-built binary, so no gate ran it — removed; the shared fixture holds its cases.
- Task 4: no failure; QA's comments that cited a dated ticket file or a plan decision under `internal/` became plain descriptions.
- Task 5, a reused id under another kind: spec ambiguity — a creature removed unseen and added again as a party member under the same module id waits for sight, since only the module id tells two creatures apart — no code; the specification states it.
- Task 5, the rewrite's ids, the forwarded grant and `outcome_summary`: no change — the rewrite keeps the world's ids as every forwarded event does; N6's and N7's frame lists pin the forwarded grant; `outcome_summary` stands by the owner's ruling.
- Task 5, `perceive.Resume`'s doc: false prose — it said a viewer resumes seeing only when its fold holds every actor in sight "as world does" — now "as shownAs makes it of world".
- Task 6, which refusal comes first and whom it names, and the gate's reach: no change — the gate answers before `Resolve`, naming the first unseen or absent target in command order; a second party member out of the shoulder's sight, and a player with only a creature, are the plan's D11's stated consequences.
- Task 6, a `vs` that rolls dice: no change — it is recorded in `rolls` with the defense inside its total; the window reads `outcomes` alone.
- Task 7, a move's reason: spec ambiguity — the plan's D13 said "with `because` for a reason, as today", naming the helper where it meant the wording `<token> moved to x,y — <reason>` — QA's test asserts the dash line.
- Task 7, the DM's card: behaviour defect in the plan's rule — the plan's D15 said "players see: the token's name or `a creature`", false for a party member's unnamed token — the card shows what a player's window names it (`Namer.shown`), VTT-490.
- Task 7, an ability with no target: no change — it would read "… on ", and `Resolve` refuses one; near-theoretical, small consequence (the owner's ruling of 2026-10-21).
- Task 7, ids that stay in the Story: no change — note keys, adventure ids and condition ids are shown; the plan's D13 narrows "never an id" to actor and token ids.
- Task 7, the app's pin and the agent's chain: QA could not reach `boot`, so two app tests hold the pin across a repaint and each role's naming.

QA's own spec gaps that changed nothing, kept here so they are not asked
again:
- Task 2: who refuses an unknown token is the fold (Task 3); the Describe
  group is not drawn with no token; the place-token form keeps its five
  fields.
- Task 3: the two folds word the unknown-token refusal differently, as they
  do for moved and removed; an empty `token_id` names no token in both.
- Task 4: `encoding/json` matches keys without regard to case, so `"NAME"`
  loads as `name` and a later duplicate wins; near-theoretical, no code and
  no debt entry. `null` loads as empty, a golden's `"notes": ""` equals an
  absent key, and the refusal of an unknown key in an adventure's scene is
  stated in `adventure.Load`'s doc alone.
- Task 7: one target button per token, not per actor; the card's order is
  name, players see, notes, GM notes.

## Phase 4b

One reviewer per task, two passes (Task 1's took one), by command. Every
Must and every Should was fixed in the same change. Five Nits were not taken,
each with its reason in its commit: Task 4's optional note on keys left out
when empty and the `go doc` run-on a blank line would have pushed over its
comment ceiling; Task 5's optional rename; Task 6's two, on the goldens'
masked totals and three QA tests that point at SPEC-016 before it names
`SeesActor`.

| Task | Pass 1 | Pass 2 |
|---|---|---|
| 0 | 2 Must, 13 Should, 12 Nits | 5 Should, 7 Nits |
| 1 | no Must or Should, 2 Nits | — |
| 2 | 2 Must, 5 Should, 3 Nits | 3 Nits |
| 3 | 3 Must, 4 Should, 4 Nits | 2 Must, 4 Nits |
| 4 | 4 Must, 5 Should, 5 Nits | 2 Nits |
| 5 | 2 Must, 4 Should, 5 Nits | 3 Nits |
| 6 | 2 Should, 5 Nits | 1 Nit |
| 7 | 6 Should, 5 Nits | 2 Nits |

The gate's two fixes: `a2f7693`'s one row was checked by command against its
measurement. `eb6d423` had one reviewer, two passes, by command: one Must, `dm.ts`
over its comment ceiling against `main`, and one Nit, a parameter named like
the file's presence flags; then one Should, this report's account of the
comment that went. All three were taken.

The findings that changed code: Task 2's never-forward warning on every GM
field; Task 3's recipe that left `goEscape` unused, its warning on U+2028 and
U+2029, and its 23 over-long comment lines; Task 4's go-arch-lint and
golangci-lint refusals; Task 5's test cited for VTT-466 with no
`ResourceChanged` in it, where adding one did not red it under K11 (below);
Task 6's helper that let a wrong outcome total pass; Task 7's card clipped at
the board's edge and `actor()` re-sorting every token id on every call.

## The deliberate breaks

Each commit's message lists its breaks, what was edited, which tests went red,
and how each red set was measured. Task 1's K1 ran on a probe outside the
tree. Task 2's ran with QA's tests in place, and Task 4's over `go test ./...`
on the final tree, QA's included;
Task 3's first breaks ran before QA's tests existed, on the packages each
touched, and the review re-measured the Go ones over `go test ./...`. Task 5's
and Task 6's ran over `go test ./...` before QA, and QA's tests were measured
under the same breaks by name, the second figure below, while their four rows
named for a requirement were measured over `go test ./...` on the final tree,
QA's included; Task 7's ran over
`bun test client/test` on the final tree, QA's included, its second figure
the count among QA's. Each break was repaired by its inverse edit.

| Break | What was broken | What went red |
|---|---|---|
| K1 | The probe's rename on a handle without the timeout | `SQLITE_BUSY` at once |
| K2 | `place_token`'s `requiredOverride` dropped | `TestPlaceTokenOffersTheFourTextsAsOptional`, among others |
| K3 | `use_ability`'s sentence on seen targets dropped | `TestUseAbilitySaysAPlayersTargetMustBeSeen`, among others |
| K6 | The player admitted to `commandRoles` for `describe_token` | `TestEveryPlayerCommandHasARule`; `TestOnlyTheDMAndTheAgentMayDescribeAToken` with the spectator admitted, or the player with an `unrestricted` rule |
| Task 2's conversion | `ToEvent` blanks `place_token`'s notes and `describe_token`'s GM notes | `TestToEventPlaceTokenProducesTokenPlaced`, `TestToEventDescribeTokenProducesTokenDescribed`, among others |
| Task 2's arm | `classify`'s `TokenDescribed` arm removed | `TestEveryEnvelopePayloadArmHasAnExplicitRuling` |
| Kc1 | The Describe group draws the first token only | 5: two console tests, three of QA's |
| Kc2 | `describeToken` drops `gmNotes` | 6: the builder and send tests, four of QA's |
| Task 2's console | It sends `notes` as `gmNotes` | The send test, among others |
| Task 2's boxes | The notes boxes are inputs again | The row test and QA's textarea test |
| K4 | `Apply` drops `Name` at placement | 7: `TestATokenPlacedKeepsItsFourTexts`, the `cmd/vtt` dump test, five of QA's |
| K4b | The rewrite keeps notes it was sent empty | 4: `TestATokenDescribedReplacesAllFourTexts`, the dump test, two of QA's |
| K4c | The rewrite stores a token it does not hold | 6: the engine and live refusal tests, four of QA's |
| K5 | `fold` drops `GMNotes` at placement | Fold-unit's placement, prototype-name and dump parity tests |
| K5b | The TS rewrite keeps empty notes | The rewrite test and dump parity |
| K7 | The dump writes no `Notes`, or writes an empty one | Dump parity; the second also every scenario golden's fold parity |
| Kc3 | The console's prefill never applies | The prefill test |
| Task 3's escapes | `goEscape` removed, or U+2029 or `&` dropped from its class, or `padStart`'s `"0"` blanked | Dump parity and QA's escape test (for U+2029, dump parity alone) |
| Task 3's generator | A placement named without its id; the rewrite band never drawn; the GM notes fixed | `TestAPlacementIsNamedForItsTokenAndARewriteNamesAPlacedOne`; with the band, also `TestAWalkDrawsEveryKind` and `TestRebuildEqualsLiveProperty` |
| K8 | The adventure compile drops `GmName` | 11, among them `TestCompileValidFixtureExactEnvelopeList` and `TestScenarioGoldenStreamsHaveNotDrifted` |
| K9 | The map loader stops decoding `notes` | 5, among them `TestAMapPlacementCarriesItsFourTextsIntoTokenPlaced` |
| Task 4's dump | The conformance dump drops `GMName`, or `GMNotes` | 4, or 2 |
| K10 | `introduce` clones the actor whole | 33, and 22 of QA's |
| K11 | `classify` forwards every `ResourceChanged` the viewer saw | 5, and 4 of QA's |
| K12 | The synthesized `TokenPlaced` carries `GmName` | 3, and 22 of QA's |
| K12b | It drops `Notes` | 2, and 5 of QA's |
| K13 | A `TokenDescribed` is forwarded whenever held and in sight | 1, and 1 of QA's |
| K14 | `recall` compares with the world's actor whole | 29, and 9 of QA's |
| K15 | `hideTokensOf` returns nothing | 17, and 5 of QA's |
| K16 | A reshaped actor is withdrawn whether seen or not | 2, and 2 of QA's |
| K16b | A reshaped actor in sight is corrected, not re-introduced | 3, and 4 of QA's |
| K17 | `forwardable` keeps `GmNotes` | 1, and 3 of QA's |
| Task 5's VTT-485 | `shownAs` strips every actor | 88 |
| Task 5's VTT-486 | `classify` withholds every `ResourceChanged` | 15 |
| K18 | The sight gate's loop walks no target | 2, and 7 of QA's |
| K19 | The gate refuses only an actor the snapshot holds and does not see | 1, and 2 of QA's |
| K20 | The gate judges every role | 14, and 1 of QA's |
| K21 | An outcome's total takes the compared total | 13, and 7 or 8 of QA's over three runs |
| K22 | The gate answers its own literal | 1, and 7 of QA's |
| Task 6's VTT-487 | The gate refuses every target | 16 |
| Task 6's VTT-488 | `forwardable` clears an `AbilityUsed`'s `outcome_summary` | 3 |
| Kc4 | The naming falls back to the actor id | 23 (10 of QA's) |
| Kc5 | The DM chain puts the token's name first | 15 (9) |
| Kc6 | The card shows GM notes to a player | 3 (2) |
| Kc6b | The card omits the notes | 4 (2) |
| Kc6c | The DM's card omits the GM notes | 2 (1) |
| Kc7 | A right-click does not pin | 4 (2) |
| Kc7b | The `contextmenu` is not prevented | 4 (2) |
| Kc7c | `app.ts` never unpins | 1 (0) |
| Kc8 | `describe`'s `tokenHidden` arm returns its case name | 6 (4) |
| Kc9 | The ability line reads `outcome_summary` | 9 (7) |
| Task 7's VTT-490 | `Namer.shown` follows the window's chain | 3 (1) |
| Task 7's role | `app.ts` passes `dm: false` for every role | 1 (0) |
| Task 7's card side | A card never opens leftward | 1 (0) |

Three things the breaks taught:
- **A leak the projection makes is invisible past the fold.** Under K11 the
  leaked frame does not fold onto a log that holds the creature without
  resources, so the campaign drops that event's frames for the log (VTT-320)
  and nothing reaches the wire. K11 reds the `perceive` tests and the
  campaign tests that see the drop; a live test cannot hold VTT-466 alone.
- **K16 has a direction it cannot reach.** In
  `TestAReshapingKindChangeOutOfSightWaitsForSight` it reds the "creature
  made a party member" direction alone, besides
  `TestAKindChangedOutOfSightIsBroughtToItsPresentOnSight`: an unseen
  creature is not among the actors `transitions` walks, so it waits either
  way.
- **A count from a random roll is not a count.** K21 reddened QA's gateway
  tests only when the server's roll missed, until the helper required each
  outcome's total to be its roll's; even then a roll equal to the defense
  leaves one green, so the count is 7 or 8.

## What the plan weighed, and what the browser showed

**Structured outcomes (the plan's D12).** Reading an ability's result and
roll by parsing `outcome_summary` needed no contract change, but made a
second home for `Resolve`'s format string; a ruleset's display names are free
text no loader bounds, so a "; " or " on " inside one makes the split
ambiguous, and a parse that misses has only the raw text, defense and target
id included, to fall back on. Structured `outcomes` keep one home, since
`Resolve` writes the summary part and the outcome from the same values, and
the defense is in no field the window reads. The plan costed them at two
`Resolve` tests and two re-recorded streams; they took eleven tests and two
hand-edited streams (Deviations).

**A kind change in sight (the plan's E1).** In both directions the frames
are: a `TokenHidden` of each of the actor's tokens on the board, a bare
`ActorRemoved`, the `ActorAdded`, its grants, its conditions, then the
`TokenPlaced` of each token; the forwarded `ActorControlGranted` follows.
Both folds take that grant, which repeats a controller the introduction
granted, since their idempotent return comes after the kind write.
`TestACreatureAGrantMakesAPartyMemberInSightArrivesWhole` and
`TestAPartyMemberAGrantMakesACreatureInSightIsHeldStripped` hold the list
exactly.

**The first browser run (the ticket verdict's H1).** The hover, the card's
click-through and the console are observed only in a real browser. On the
first bundle Task 7 built, `task e2e` passed 20 and skipped 1, and W1
observed two of the three: it hovered `tok-cutter`, read its card as "a
hunched goblin", right-clicked it and moved the pointer away with the card
still shown. No browser run left-clicks through a shown card or clicks the
console's Describe; both are under "What could not be established".

## Rule 9: how MapTool does this

Checked read-only in `~/dev/RPTool/maptool` at `f4b7fef6c`, by the plan.
`Token` carries `name`, `notes`, `gmName` and `gmNotes`; `PointerTool.createHoverNote`
builds the name, the GM name for the GM when it differs, the notes, then the
GM notes; the renderer labels a token by its name and, for the GM, appends the
GM name.
- **Borrowed:** the four fields and their split; the card's order; a rewrite
  that states every field at once, as `EditTokenDialog.commit` does.
- **Not borrowed:** the distribution, since every MapTool client receives the
  GM texts and hides them (rule 9 excludes it); the refusal of an empty name,
  since an unnamed creature reads `a creature` here; "name (gmName)" as the
  DM's label, since the ticket names a creature to the DM by its GM name and
  the card shows both; the marker-only note, since every token has a card.

## What left the code's comments

`internal/eventgen/model.go`'s `Step` doc was 24 lines, and Task 3 had to
touch it, so it was cut to five under SPEC-010's bound. What it said before
`aa25462`, with the rewrite band that commit added:
- The bands are thresholds on one uniform draw, in order: create scene
  [0, 0.05), add actor [0.05, 0.15), place token [0.15, 0.28) when a scene
  and an actor exist, move token [0.28, 0.48) and rewrite a token
  [0.48, 0.52) when a token is on the board, add narration [0.52, 0.60),
  upsert note [0.60, 0.64), delete note [0.64, 0.68), open door [0.68, 0.72),
  close door [0.72, 0.76), grant control [0.76, 0.79), revoke control
  [0.79, 0.82), apply condition [0.82, 0.86), remove condition [0.86, 0.91),
  remove token [0.91, 0.94), remove actor [0.94, 0.96), and the remainder
  starts or ends a session, or adds an actor when one is open.
- A guarded case hands its draw downward rather than redrawing, which is why
  band width alone does not predict a count.
- An extra rng draw moves the whole walk: gating a preference behind one
  extra `rng.Float64()` once took `removeCondition` from one draw per walk to
  zero and red the coverage guard, on logic that was correct.

In `81eaae6`, to keep three test files under their comment ceilings,
`unseen_test.go` lost its note that the scout is a party member because a log
holds no creature's attributes, and N8's warning on its fixture became two
lines.

In `eb6d423`, to keep `client/src/view/dm.ts` under its comment ceiling once the
describe row lost three code lines, the comment on `draft` lost its last
paragraph. It said the lost typing `draft` answers was found by the e2e and
by no unit test, since it existed only when a real event stream ran
alongside a real form. The e2e is `client/e2e/table.spec.ts`'s "what the DM
is typing survives events arriving"; dm-view's "text survives a re-render,
so an incoming event cannot eat what the DM is typing" holds it in
happy-dom now.

## Figures

From the `task check` on `eb6d423`, the gate's third run, launched with this
report and the specifications in the tree, uncommitted:

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp. `internal/perceive` measured 95.7% against
  its 94.5% floor, and `internal/gateway` 93.9% against 93.5%.
- Client coverage: 23 files gated, `naming.ts` among them, each at 100.00%;
  1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - Every package reused its verdict from the second run, on `a2f7693`, whose
    Go code is `eb6d423`'s. That run measured 13 of the 15 afresh and reused
    `internal/artlib`'s and `internal/campaigncfg`'s.
  - 7 timed out and were counted as killed, not measured: 4 in
    `internal/sight`, 1 each in `internal/mcp`, `internal/perceive` and
    `internal/rules`, the counts the ticket 4 report gives for `0aacc59`.
- Client mutation, in its container: Stryker's own "Done in 58 minutes and 31
  seconds". 3203 mutants: 3130 killed, 9 timed out and counted as killed, and
  64 survivors, all adjudicated equivalent.
  - `tools/mutation-equivalents.txt` is unchanged since `a8bb0ef`, on `main`
    before this ticket. `tools/ts-mutation-equivalents.txt` lost its two rows
    on `grid.ts`'s old initial in `74ab287` and gained none.
  - `client/src/view/naming.ts`, new: 85 killed, none survived.
  - `client/src/view/spectator.ts`: 454 killed and 2 survived, against 378
    and 2 on `0aacc59`, by the ticket 4 report.
  - `client/src/view/player.ts`: 247 killed, none survived, against 241.
  - `client/src/app.ts`: 226 killed, none survived, against 222.
  - `client/src/view/dm.ts`: 486 killed and 5 survived, each on a line this
    ticket did not change.
- Race: both runs pass.
- Requirements chain: 490 rows, 309 test files, 16 specifications.
- Comments: 336 files, clean, with 106 comment lines added against `main`.
  Its two notices are the ones `0aacc59` had:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
- New prose, which reads Go and TypeScript only: 8475 added lines across 87
  files, clean.
- Disk: 31.7 GiB free, by the gate's own line, when the Go mutation step
  began.
- Duration: 4583 s, from the launch at 12:50:11 to the exit at 14:06:34. The
  second run took 8034 s, from 09:51:16 to 12:05:10, and stopped at
  `check:ts-mutation`; the first stopped at `check:ts-coverage` (Deviations).

`task e2e`, twice, against the same tree after the gate:

- First run: 19 passed, 1 failed and 1 skipped. The failure is
  `client/e2e/joining.spec.ts`'s "a stranger with one link joins, watches, is
  promoted, and plays": its DM's Start session answered "a session needs a
  name" and no session started, and its trace shows the name box empty when
  `fill` returned. The defect is older than this ticket, and where it went is
  under "What was deliberately left out".
- Second run: 20 passed and 1 skipped, in 25.2 s.
- The skipped one, in both, is "a network drop while the table plays on ends
  in a Reconnect", a `test.skip` in `client/e2e/handover.spec.ts`.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Task 4's file list left out the conformance runner | `tokenPlacedDump` and its format doc gained the four texts | The runner decodes goldens with `DisallowUnknownFields`, so a golden could not carry the texts until its dump knew them |
| adventure-night's golden would not change | Its `stream.json` and `state.json` were edited by hand with the new keys only | It loads goblin-ambush; re-recording would have replaced the recorded dice the hand-derived state rests on |
| The pause battery's probe on the demoted `eye` becomes a condition | It changes the friend's pool | `mark eye` already was a condition; the friend's change still reaches the friend's log in both cases and `eye`'s only while unpaused, so the battery keeps a resource probe |
| "Two Resolve tests" and two re-recorded streams (the plan's D12) | Eleven `internal/rules` tests that compare a whole `AbilityUsed` gained their outcome; door-watch's and toy-brawl's streams were edited by hand | The plan's prototype did not run them; each outcome is the target, word and total its own expected summary names, and the hand edit keeps the recorded dice |
| `tokensOnScene` takes the role (the plan's D20) | It takes the namer | One namer per paint serves the board and the Story |
| The plan's D14: an actor is named by its tokens | By tokens the fold holds and tokens the log named | Otherwise every earlier Story line would read `a creature` once a creature's token left sight |
| The plan's D13: `describe` ends in an exhaustive `never` check | It has no `default` | Its declared return type makes a new case fail the typecheck the same way, with no dead arm for Stryker |
| The plan's D15: "players see:" the token's name or `a creature` | What a player's window names the token | QA showed the plan's rule false for a party member's unnamed token |
| The plan counted three `describe` tests in `qa-move-reason.test.ts` | All eight declarations, 16 cases, pass a namer | `describe` takes the namer, so every direct caller changed |
| The gate as Task 8, before the report | After the report and the specifications | The owner's ruling of 2026-10-25: a Stryker probe in Docker had been stopped for low memory |
| The plan gave `naming.ts` its comment-ceiling row | The gate's first run stopped at `check:ts-coverage`: the file had no coverage floor. `a2f7693` adds it at 100.00, the 46 of 46 lines it measures, and the gate ran again from its start | The plan carried a new file's row in the comment ledger and not in the coverage table, and no hook runs `check:ts-coverage`, so nothing before the whole gate could ask for it |
| Both mutation gates green with no new adjudication on code the plan wrote (Task 8) | The second run stopped at `check:ts-mutation` on one survivor, `""` -> `"Stryker was here!"` in `textarea()`'s `t.value = draft[field] ?? "";`, from `12bf268`. In `eb6d423` both box helpers take the text a box shows with no draft and the row passes each box its token's text, and the gate ran again from its start | The row overwrote every box with no draft after building it, so the helper's fallback was never seen; during the ticket Stryker ran only over Task 7's lines, so the whole gate was the first run to mutate it |

## What could not be established

- **The defense on the wire.** `outcome_summary` carries "(17 vs 15)" to every
  player, by the owner's ruling, and a `vs` that rolls dice would put the
  defense inside a `rolls` total. The window shows neither; the frames do.
- **Ids on the wire.** Every frame about a creature carries its actor and
  token ids, `act-cutter` and `tok-cutter` in goblin-ambush. The window never
  shows them; the ticket left them on the wire.
- **What a creature's conditions tell.** A creature's conditions still reach
  a player, by VTT-243 and the plan's sort. `rulesets/dnd45e-minimal` applies
  `bloodied` and `dying` by threshold, so when a strike hits, a player's Story
  can read "a hunched goblin gained bloodied": a condition can say what hp
  would have.
- **A left click through a shown card, and the console, in a real browser.**
  No e2e left-clicks a cell where a card is shown, so `pointer-events: none`
  is held by no browser run; token-card's happy-dom test "a click on a disc
  still reaches the board's click handler" cannot see CSS. No e2e clicks the
  console's Describe either; dm-view's "Describe sends describe_token for its
  row's token with the four fields as typed, empty ones included" and "the
  console draws a describe row per token of the world, by id" hold it in
  happy-dom.
- **The card at the board's edge in a real browser.** The card now opens
  inward, and happy-dom lays nothing out; W1 sees the card visible, which
  Playwright also says of a clipped one.
- **The third Stryker run.** A run in the gate's container over the review's
  fixes to `naming.ts` and the disc was stopped by the host for low memory.
  The 22 mutants it would have made of those lines were applied by hand and
  all killed; the gate below runs the whole of it.

## What was deliberately left out, and where it went

- Keeping ids or `outcome_summary` off the wire: nowhere, by decision. The
  ticket put ids on the wire out of its scope, and the owner ruled the
  summary stays ("utfallsraden står"); no ticket carries either.
- The defense inside a dice-rolling `vs`: nowhere, since no shipped ruleset
  rolls dice in a `vs`; it is the same question as the summary's.
- A creature's conditions to a player: unchanged by the plan's sort (VTT-243,
  VTT-249).
- An `AbilityUsed` with no target: `Resolve` refuses one; no code.
- The specifications SPEC-016, 007, 013, 018, 014, 021 and 022: the commit
  that follows this report.
- A repaint of the console takes the focus from the box the DM is typing in,
  on `1e24277` as on `eb6d423`; it is the first e2e run's failure (Figures).
  It went to the open debt in `docs/verification-debt.md`, with its probe,
  and is raised to the owner with this report as a candidate for a ticket
  of its own.
