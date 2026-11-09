# Plan: a sheet holds only values its ruleset declares and none written empty

- **Ticket:** `docs/superpowers/specs/2026-11-08-campaign-data-fits-its-ruleset-design.md`
  (ADR-012, Build order step 1, its second ticket; blueprint principle 26),
  as rewritten and amended on 2026-11-08.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-08 by an agent that did not write the ticket, read-only
    against `eb0fa35` on `feat/campaign-data-fits-its-ruleset`, with the
    ticket untracked. A first plan for the ticket's earlier text was
    replaced by this one; the owner had struck its forms and defaults, and
    nothing of it was signed.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and gremlins were not run. Probes ran from scratch
    directories outside the tree (P2 to P4).
- **To be committed as:** `docs/superpowers/plans/2026-11-08-campaign-data-fits-its-ruleset.md`.
- **Base:** `eb0fa35`. **Branch:** `feat/campaign-data-fits-its-ruleset`.
- **Owner's rulings taken in:**
  - 2026-11-08: a sheet is unique to its character; no form, no default, no
    check that a sheet holds every property; keeping a campaign's sheets in
    step with its ruleset is the DM's; refused is a value the ruleset does
    not have, and a value that has none, written as `null` or as an empty
    string; `0` is a value; at boot, a campaign that does not fit is refused
    (principle 26 as written). His earlier rulings on forms and defaults are
    withdrawn.
  - 2026-11-02: plain words are for the player's refusals from the panel;
    the DM console's and the agent's refusals keep their form (D2).
  - 2026-10-21: a near-theoretical edge with a small consequence gets no code,
    no test, no debt entry (D4).
  - 2026-09-23: a fact has one home (D1).
  - 2026-09-04: nobody uses the product; no migration. Principle 5 still
    binds every campaign log already written: it opens, and is served with
    no ruleset.
  - Standing: no counts or measured figures in anything written down.
- **Signed off:** by the owner, 2026-11-08 ("Godkänd"), with OD-1 answered
  A: a `current` that has no value is refused where it is written. The
  ticket's item 3 is narrowed to the MCP tool and the console's raw paste.
- **Ids:** allocated at Task 0, after sign-off: R1 VTT-669, R3 VTT-670,
  R4 VTT-671, R5 VTT-672, R6 VTT-673, R7 VTT-674, R2 VTT-675, R8 VTT-676.
  R2 was refused at the sort and R8 not drawn; both were allocated after
  the reading review of Task 0: a check that treats no ruleset as one
  declaring nothing passes R1 and R6 and refuses every key, so each needs
  its own row. R1 to R8 are this plan's labels.

## The verdict, check by check

Checks 2, 4 and 6 and the gaps G1 and G2 read the ticket before its item 3
was narrowed to the MCP tool and the console's raw paste. The narrowed
ticket names `client/src/commands.ts`, `cmd/vtt/webdist`, SPEC-022 and
SPEC-007, and places nothing in `internal/gateway/codec.go`.

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol it names is where it says: `validateAddActor`,
   `handleCommand`, `engine.Apply`'s `ActorAdded` arm, `checkStatblockKey`,
   `actorJSON`, `resourceValJSON`, `loadActors`, `DecodeCommand`,
   `commandFromArgs`, `composeServer`, `tools/toolgen/main.go`, both
   `tools.json` copies, SPEC-024 and SPEC-013, and MapTool's
   `CampaignProperties.tokenTypeMap` and `Token.getPropertyDefault`.
2. **"Done" is an observation. By command for item 3, by reading for 1 and
   4, and on the writer's probes for 2.** Each item names what will be true
   and the tests that show it. The writer's `protojson` probes were rerun
   (P2) and hold; its `encoding/json` probes, and that an empty string is
   refused on every path, were taken as given. Items 1 and 4's "today" halves
   rest on reading: `validateAddActor` reads no key, and `composeServer`
   reads no actor. No named test exists yet. Item 3's last sentence cannot
   hold together with its WebSocket half where the ticket places it: G1.
3. **Each rule is breakable. By reading.** Each rule line maps to R1 to R8,
   each with what turns it red.
4. **The scope matches the claim. By command, then reading.** The callers:
   `validateAddActor` and `checkStatblockKey` each have one; `composeServer`
   is called by `newServeCmd`, by `bootSelfContained` in
   `cmd/vtt/harness_boot.go` for `vtt client run`, and by tests. The
   searches under "The existing tests and data that change" find no test or
   data the new refusals hit.
   Under OD-1's recommendation the work touches `client/src/commands.ts`,
   which the ticket does not name, and not `internal/gateway/codec.go`,
   which it does: G2.
5. **No recorded decision is contradicted silently. By reading.** The
   ticket carries the check SPEC-024's Consequences hands it; VTT-617 and
   VTT-618 stand word for word (D1). `adventure.Load` refusing a `null` or a
   missing `current` tightens a loader, not a log: no tracked adventure is
   refused by it (P5), and principle 5 binds logs. The owner withdrew his
   own forms and defaults; this plan overturns nothing recorded.
6. **The records the work moves are named. By command, then reading.**
   SPEC-024 and SPEC-013 are named and resolve. Missing, depending on OD-1:
   SPEC-022 if the console's paste is checked (option A), SPEC-007 if the
   contract gives `current` presence (option B). G2.

**Split: not recommended.** The ticket is one rule, refused where values
enter and at boot, and is now small.

## The gaps this plan inherits

- **G1. Item 3's WebSocket half cannot hold where the ticket places it.**
  The ticket says a pre-decode read in `DecodeCommand` or a contract change
  each makes item 3 hold. The pre-decode read does not. Both of the
  gateway's clients write a `current` of `0` by leaving it out (P3, P4):
  the browser through protobuf-es `toJson` in `client/src/wire.ts`, and the
  MCP server through `harness.Client.SendCommand`'s `protojson.Marshal`.
  So in a frame, "left out" is every client's `0`, and refusing it refuses
  the owner's own case, a resource spent to nothing. A `null` never reaches
  a frame at all: both clients' decoders read a `null` current as `0` before
  they encode (P2, P4). And a refusal naming the actor and the key cannot be
  `DecodeCommand`'s: a decode error closes the connection as a malformed
  frame (its caller in `internal/gateway/server.go`, SPEC-011). OD-1 chooses
  between checking where the value is written (A) and giving the field
  presence (B); under A the writer narrows item 3's WebSocket half to the
  console's raw paste.
- **G2. The files and records follow OD-1.** Under A:
  `client/src/commands.ts` (`parseActorJSON`), `client/test/commands.test.ts`,
  `cmd/vtt/webdist` and SPEC-022 join, and `internal/gateway/codec.go`
  leaves. Under B: `contract/vtt/v1/events.proto` (`Resource`), every
  generated file, SPEC-007, and every reader of a resource's current (D4).
- **G3. Item 4 says "naming each actor and the key" without saying whether
  every key or the first.** D5 settles it.

## Measurements this plan stands on

- **P1.** `test -e` over every path the ticket names, `grep` for each
  symbol. All resolve.
- **P2.** `protojson.Unmarshal` into a `ClientCommand`, from a scratch module
  that replaces the repository's module with this worktree: `{}` and
  `{"current":0}` decode alike; `"brawn": null` in `attributes` is refused
  (`invalid value for int32 field value: null`); a resource written `null`
  is refused (`unexpected token null`); `{"current": null, "max": 3}` is
  accepted with current `0`. protojson accepts `add_actor` and `actor_id`
  beside `addActor` and `actorId`; `actor`, `resources` and `current` are
  spelled alike in both.
- **P3.** `protojson.Marshal` of an `add_actor` whose resources are current
  `0` maximum `0`, and current `0` maximum `2`, writes `{}` and `{"max":2}`.
  `harness.Client.SendCommand`, which `internal/mcp/server.go` sends through,
  marshals that way, so `scenarios/toy-brawl.json`'s `drink`, written with
  `"current": 0`, reaches the gateway without it.
- **P4.** protobuf-es, run with bun against `contract/gen/ts`: `toJson`
  writes `{"max":4}` and `{}` for the same resources; `fromJson` of an actor,
  as `parseActorJSON` calls it, accepts `{"current": null, "max": 3}` as
  current `0` and refuses a `null` map value.
- **P5.** Every tracked `actors/*.json` under `adventures/` and the
  adventure testdata, read with Python: none writes `null`, and every
  resource gives `current`. The alpha adventure under `~/dev/vtt-alpha`,
  outside the repository, likewise, and its keys are dnd45e-minimal's.
- **P6.** The MCP SDK's `Server.AddTool`, which `internal/mcp` registers
  every tool with, leaves validating arguments against the input schema to
  the caller. `add_actor`'s generated schema requires both `current` and
  `max` in a resource; the server accepts a resource with no `max`.
- **P7.** `campaign.Open` takes an exclusive, non-blocking `flock`, so a
  second open fails while the first is held.

## Constraints that bind every task

- **CLAUDE.md rules:** 2 (no gate moves); 3 (additive contract; OD-1's
  option B is not additive, D4); 4 (the fold changes nothing); 5 (no
  game-system word in Go tests either); 8 and 10 (names, not line numbers;
  a comment is a warning or a pointer, and one this change makes false is
  rewritten, `checkStatblockKey`'s doc and the `loadActors` comments that
  name it first); 9 (answered below).
- **Principle 6.** Decisions are made where the record is kept. A check in
  a tool or a window is an aid at the input, never the only thing standing
  between a participant and an outcome an unaltered tool could not reach
  (D4).
- **The project's mechanical gates**, as
  `docs/superpowers/plans/2026-11-05-a-ruleset-names-its-properties.md`
  lists them under its "Constraints that bind every task": `check:new-prose`,
  `check:comments` after staging, the requirements chain, `check:doc-owner`,
  the coverage floors, the Go mutation gate over `tools/check-mutation.py`'s
  `PACKAGES` (which hold `internal/rules`, `internal/adventure`,
  `internal/gateway` and `internal/mcp`), a client change committed with its
  `cmd/vtt/webdist`, stage and commit in separate calls.

## Rule 9: how MapTool does this

In MapTool at RPTool `63862e4`:
- `Token.setProperty` stores any key with any value, and nothing in it
  refuses a key its token's property type does not list, or an empty value.
  A missing value reads as the type's default (`Token.getEvaluatedProperty`,
  `Token.getPropertyDefault`).
- **Borrowed:** nothing. **Not borrowed:** property types and defaults, which
  the owner struck; keeping a key the type does not list, which principle 26
  refuses; storing an empty value, which the owner refuses; untyped text
  values, where `0` and an empty string are both just strings, while here a
  value is an integer and `0` is a value.

## Decisions

1. **D1. One key check, in `internal/rules`, for its callers.** A new
   exported method on `rules.Ruleset` takes an actor's attribute keys and
   resource keys and answers the first fault, attributes before resources,
   each map in byte order of its keys, with its field (`attributes`,
   `resources`) and key, in VTT-617's and VTT-618's words. That order is
   what VTT-669 and VTT-673 call the first key that does not fit.
   `adventure.Load` wraps it with `fieldErr`, the gateway (D2) and the boot
   (D5) with the actor's id. `checkStatblockKey` is retired into it.
   - **Forced by** the 2026-09-23 ruling, and by SPEC-024, which keeps where
     a key is checked in `internal/rules`; `.go-arch-lint.yml` already lets
     `gateway`, `adventure` and `cmd` use `rules`.
   - **Forced by** determinism: `loadActors` ranges over a Go map today, so
     which of its bad keys is named varies, and tests compare the text.
2. **D2. `add_actor` reads its keys when a ruleset is served.** In
   `handleCommand`, after `validateAddActor`, for an actor with an id, D1's
   check runs; a refusal reads `gateway: add_actor: actor "<id>": ` and the
   fault. With no ruleset, nothing is read.
   - **Forced by** SPEC-013's place for a validator (every role, before
     anything is written, never in `Authorize`, `ToEvent` or the fold); by
     VTT-154, which leaves an actor with no id to the fold's refusal; by the
     owner's 2026-11-02 ruling on the DM's and the agent's refusals.
3. **D3. `adventure.Load` sees what was written.** `actorJSON` decodes a
   number as a pointer and a resource as a pointer whose `current` and `max`
   are pointers, so `null` and a missing field are seen. Refused, naming the
   file, the field and the key: `attributes.<key>` written `null`;
   `resources.<key>` written `null`; `resources.<key>.current` written `null`
   or left out, `{}` among them. A `max` left out or `null` is no maximum,
   as today. `0` loads. An empty string stays `encoding/json`'s refusal.
   - **Forced by** item 2 and the owner's "0 is a value"; the field names
     follow the existing `resources.<key>.current` refusal of a current over
     its maximum.
4. **D4. How `add_actor` tells a `current` that has no value from `0`: OD-1.**
   G1 forces the choice: the gateway cannot read it from a frame. The
   options, with their costs:
   - **A, recommended: read it where it is written.**
     - `internal/mcp`: before `protojson` decodes a tool's arguments in
       `commandFromArgs`, walk them by the command's descriptor and refuse
       any `vtt.v1.Resource` object whose `current` is `null` or left out, as
       a tool error naming the `actor_id` of the enclosing `vtt.v1.Actor`
       and the resource's key. The read is generic, keyed by message names
       and not by the tool's name, so `handlerFor`'s warning that the
       handler has no per-command switch stays true. It runs on arbitrary
       bytes before `protojson`'s depth limit, so it keeps to the bound
       `commandFromArgs`' comment states ("WHAT KEEPS THIS BOUNDED IS THE
       CONTRACT"), never descends into `module_data`'s `Struct`, and stays
       inside what `FuzzToolArgumentsBecomeTheCommandTheyName` drives.
     - `client/src/commands.ts`: `parseActorJSON` reads the pasted object the
       same way before `fromJson`, and reports what it finds after its
       controller and kind refusals, keeping the server's order its comment
       states.
     - Nothing new at the gateway. A frame from another writer whose
       `current` is left out or `null` reads as `0`. One such writer ships in
       the binary: `vtt client run`'s scenarios, which `runCommandStep` in
       `internal/harness/engine.go` decodes with `protojson` and sends through
       `harness.Client`; no participant plays through it. **Forced by** the
       2026-10-21 ruling, and the outcome is a value its writer could have
       written and sees on the sheet. Principle 6 holds: an unaltered tool
       reaches the same `0` by writing it.
     - **Cost:** a reader in `internal/mcp`, another in the client, each with
       tests; `cmd/vtt/webdist`; SPEC-022. Item 3's WebSocket half is
       narrowed by the ticket's writer to the console's raw paste.
   - **B: give `Resource.current` presence** (proto3 `optional`). The gateway,
     where the record is kept, refuses an absent `current` on `add_actor`
     for every client, since both encoders write a set optional `0`.
     - **Cost:** a presence change on an existing field is not an addition
       (rule 3), so it needs its own recorded decision under ADR-007. Go's
       `Resource.Current` becomes a pointer in every literal and reader, in
       `internal/engine`, `internal/rules` and its conformance suite,
       `internal/adventure`, `internal/perceive`, `internal/campaign`,
       `internal/gateway`, `internal/harness`, `internal/eventgen` and their
       tests; TypeScript's `current` becomes optional in `client/src/fold.ts`,
       `player.ts`, `view/grid.ts`, `view/spectator.ts` and their tests;
       `vtt state dump` prints a set `0`, so every scenario golden holding a
       zero current changes, with the client fold's dump beside it. A log
       written before decodes with `current` absent, which only `add_actor`
       reads, so it still opens.
5. **D5. `vtt serve` refuses a campaign that does not fit.** In
   `composeServer`, after `rules.Load` and before the adventures, D1's check
   runs over `campaign.Campaign.Actors`; any fault refuses the boot, naming
   every actor that does not fit, sorted by id, each with its first fault,
   after closing the identity store and the campaign. With no ruleset,
   nothing is read.
   - **Forced by** item 4 and principle 26. Naming the first fault per actor
     is a preference: it says which actors do not fit, and no command
     changes which keys a sheet holds.
   - The campaign's hold is observed: after the refusal, the test opens the
     campaign directory again, which fails while a handle holds it (P7).
6. **D6. The tool says what the server does.** `tools/toolgen/main.go`:
   `add_actor`'s `attributes` and `resources` docs say their keys are the
   served ruleset's properties (`get_ruleset`), a number under `attributes`
   and a resource under `resources`, that a resource gives its `current`,
   `0` included, and may leave out `max` for no maximum, and that a key the
   ruleset does not declare, or one under the other map, refuses the whole
   command; the bound sentence that
   `TestTheToolsStateTheFoldsBoundOnAnActorsModuleAndKeys` requires stays.
   `vtt.v1.Resource` gains a `requiredOverride` of `current` alone, so the
   schema stops requiring `max` (P6). Both `tools.json` copies are
   regenerated.
   - **Forced by** principle 34: today's docs say "omit unless a rule module
     instructs otherwise", and the schema calls `max` required, which the
     server does not refuse.

## Open decisions for the owner

- **OD-1. How `add_actor` refuses a `current` written as `null` or left out.**
  **Recommended: A**, checked where it is written, in the agent's tool and
  the DM's raw paste. The gateway cannot tell it from `0` without a contract
  change, and that change reaches every reader of a resource in both
  languages and every golden, for a case the owner's own "we cannot close
  every hole" covers. Under A the ticket's writer narrows item 3's WebSocket
  half to the console's raw paste before Task 0. **B** makes the rule hold
  for any writer at the gateway, at D4's cost.
  - **Answered A** by the owner, 2026-11-08.

## Requirements

Allocated at Task 0 with `requirement-id`, after the `requirements` sort.

| R | Item | Rule | Red under |
|---|---|---|---|
| R1 | 1 | With a ruleset served, `add_actor` refuses an undeclared key, or a value under the other type's map, naming the actor, the map and the key | the actor appended |
| R2 | 1 | With no ruleset served, `add_actor` reads no key against a ruleset | refused |
| R3 | 2 | `adventure.Load` refuses a `null` value and a resource whose `current` is `null` or left out; `0` loads | loads as `0`, or `0` refused |
| R4 | 3 | The MCP tool refuses a resource whose `current` is `null` or left out; `0` passes | the command sent, or `0` refused |
| R5 | 3 | The console's raw paste refuses the same | a command built, or `0` refused |
| R6 | 4 | `vtt serve` refuses a campaign holding an undeclared key or a value under the other map, naming each actor and key, and leaves nothing open | served, or a second open fails |
| R7 | D6 | `add_actor`'s schema requires a resource's `current` and not its `max` | either listed wrongly |
| R8 | 4 | With no ruleset, `vtt serve` refuses no campaign for its actors' keys | refused |

Ids, allocated at Task 0: R1 VTT-669, R2 VTT-675, R3 VTT-670, R4 VTT-671,
R5 VTT-672, R6 VTT-673, R7 VTT-674, R8 VTT-676 (R2 and R8 after the reading
review, above). That a refused boot leaves the campaign closed is an
observation in R6's test, not a row.

VTT-617 and VTT-618 keep their rows and their tests.

## The existing tests and data that change

By search; each task reruns its own first.
- **No test or data holds a key its ruleset does not declare** where a
  ruleset is served: `scenarios/door-watch.json`, `toy-brawl.json` and
  `goblin-fight.json`; the adventures and their testdata, which VTT-617
  already holds; `internal/gateway`'s `server_visibility_test.go`,
  `qa_issuer_test.go`, `qa_id_bound_test.go`, `qa_cmd_id_bound_test.go`,
  `qa_ruleset_bound_test.go`;
  `cmd/vtt/mcp_ruleset_e2e_test.go`; the Playwright suite, whose actors hold
  no keys.
- **No adventure writes `null` or leaves out `current`** (P5). The test in
  `internal/adventure/load_test.go` that loads a resource with a current
  and no maximum, asserting "0 meaning unlimited", stays green.
- **No MCP or client test sends a resource without `current`.**
- **`checkStatblockKey`'s tests keep their texts:**
  `internal/adventure/statblock_type_test.go`,
  `qa_statblock_property_test.go`, `TestLoadInvalidFixtures`'
  `undeclared-attribute` and `undeclared-resource` rows.
- **`rules.Resolve`'s refusals and their tests are unchanged.**

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:** the owner answers OD-1; the ticket's writer narrows item 3;
  run the `requirements` sort over R1 to R8; allocate ids; commit the
  ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and the chain passes.

### Task 1: the key check and the adventure (R3; D1, D3; item 2)

- **Files:** a new method in `internal/rules` and its tests;
  `internal/adventure/load.go` (`actorJSON`, `resourceValJSON`,
  `loadActors`; `checkStatblockKey` retired); new tests in
  `internal/adventure`.
- **True when:** the method names the same first fault whichever map order
  Go draws; `adventure.Load` refuses each case of D3 naming the file, the
  field and the key (each loads as `0` on `eb0fa35`, by the writer's
  `encoding/json` probe); `0` and a missing `max` load; VTT-617's and
  VTT-618's tests pass unchanged; `TestConformanceOverAdventuresGlob` passes.

### Task 2: `add_actor` (R1, R2, R4, R5, R7; D2, D4, D6; items 1 and 3)

- **Files:** `internal/gateway/server.go` (`handleCommand`),
  `internal/gateway/add_actor_validate.go`; `internal/mcp/server.go`
  (`commandFromArgs`); `client/src/commands.ts` (`parseActorJSON`) and
  `cmd/vtt/webdist`; `tools/toolgen/main.go`, both `tools.json` copies,
  `tools/toolgen/main_test.go`; new tests in `internal/gateway`,
  `internal/mcp` and `client/test/commands.test.ts`.
- **True when:** with a ruleset served, an undeclared key and a value under
  the other map are refused naming the actor, the map and the key, and with
  none served both are accepted (each accepted on `eb0fa35`); the MCP tool
  refuses a resource whose `current` is `null` or left out and passes `0`,
  with or without a ruleset (each accepted on `eb0fa35`, P2); the console's
  paste does the same; the schema requires `current` and not `max`.

### Task 3: the boot (R6, R8; D5; item 4)

- **Files:** `cmd/vtt/serve_compose.go` (`composeServer`, a new function
  beside it); a new test in `cmd/vtt` that appends `ActorAdded` envelopes
  through `campaign.Open` and `campaign.Campaign.Append`, closes, and calls
  `composeServer`.
- **True when:** a campaign holding actors with undeclared keys is refused,
  naming each actor and its key, sorted, and opens again afterwards; with
  no ruleset it is served; a campaign whose sheets hold only declared keys
  is served.

### Task 4: the gate

- **Do:** `python3 tools/check-comments.py main` and `--write-ledger` once if
  a ceiling can come down; check free space and the load; run `task check`
  whole on the last code commit, detached, output to a file, requiring its
  completion line; run `task e2e`. Tell the owner which commit the client's
  stored mutation report measured.
- **True when:** both are green (item 5).

### Task 5: the report

`docs/reports/2026-11-08-campaign-data-fits-its-ruleset.md` with the
`implementation-report` skill, and its reading review: each "Done looks
like" item with its observation, R1 to R8 and what turned each red, Rule 9,
G1 to G3 and OD-1's answer, Phase 4a's adjudications under their own
heading, the gate.

### Task 6: the specifications

With the `specification` skill, from the report:
- **SPEC-024:** the key check's home and its callers; the adventure's `null`
  and missing `current`; the boot's refusal; where an empty `current` is and
  is not refused, and why (G1); its Consequences' `add_actor` sentence made
  present.
- **SPEC-013:** the key check after `validateAddActor`.
- **SPEC-022:** the console's paste refuses a `current` that has no value.
- **SPEC-007:** `vtt.v1.Resource`'s override, which makes `max` not
  required in the generated schema.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the surfaces
as signatures and names, never the source: `go doc ./internal/rules` (the
new method), `go doc ./internal/adventure` (`Load`), the `add_actor` tool's
schema in `contract/gen/tools/tools.json`, and `parseActorJSON`.
Tell it: never run `task check`, `task e2e`, Stryker or gremlins; its files
are new `qa_` files; semgrep bans game words in Go tests too; known and not
findings: a sheet lacking a declared property is accepted, a resource with
no `max` is accepted, `rules.Resolve`'s refusals keep their words, and a
frame from a writer other than the browser and the MCP tool whose
`current` is left out or `null` reads as `0`; the tree is hashed before and
after it.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "whichever way") names the search that bounds it, D1's
callers and P5 above all; check each comment under `internal/`, `cmd/` and
`client/src` is a warning or a pointer; check the tool docs against what the
server and the MCP tool now refuse; check that
`git diff main -- contract ':!contract/gen/tools'` prints nothing.
