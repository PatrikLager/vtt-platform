# The joining-a-table arc's rules are in the register, cited by their tests, and a gate holds the chain

## The problem

The rules the joining-a-table arc put on the platform live in prose and nowhere
a check can reach. `docs/superpowers/specs/2026-08-08-joining-a-table-design.md`
states them in its sections 2, 3 and 5; `docs/adr/011-identity-and-authorization.md`
restates them in the present tense; `internal/identity/identity.go` carries them
a third time, in 400 comment lines of 898; `internal/gateway/join.go`,
`internal/gateway/authz.go` and `internal/gateway/server.go` a fourth.
`docs/requirements.md` holds no row. No test under `internal/identity` or
`internal/gateway` cites a requirement id (`grep -rlE '\bVTT-[0-9]{3}\b'
internal` prints nothing), so the test that holds each rule is found by reading
test names, and a rule no test holds is found by nobody. The chain SPEC-008
describes, a row per rule, a test citing the id, a specification naming it,
exists as a record and not as a check: no step of `task check` reads a
citation, and SPEC-008's Status says no ticket carries the gate.

`docs/verification-debt.md`'s entry dated 2026-09-23 records the one rule of the
arc that no test can see: a join refused at a shut door writes nothing. That
entry names its recipe and what closes it.

This ticket gives the arc's second ticket rows to name and a gate to hold them.
The specification of identity and joining, the sort of `identity.go`'s comments
into the record and the code, the removal of `JoinAllows` and the shut-door
test are that second ticket's.

## Done looks like

1. `grep -cE '^\| VTT-[0-9]{3} \|' docs/requirements.md` prints the number of
   rules the sort accepted, each allocated by `requirement-id`, and no row's
   evidence cell is blank or a sentence. Today: 0.
2. For every row whose evidence names a test file, that file carries the bare
   id in a comment on the line above the check that holds the rule, and
   `grep -rlE '\bVTT-[0-9]{3}\b' internal tools` lists exactly those files. Today:
   it lists nothing.
3. `task check` has a step that exits non-zero, naming the offender, when:
   - a test file under `internal/`, `cmd/`, `client/`, `tools/` or `contract/`
     cites `VTT-999` and no row defines it;
   - a file under `docs/specifications/` names `VTT-999` and no row defines it;
   - a row's evidence names a file that does not exist, or one that does not
     carry the row's id;
   - a row's id is not of the shape `VTT-NNN`, two rows share an id, or an
     evidence cell is blank;
   and exits zero on the committed tree. Each refusal is observed by injecting
   its case on the real tree and reverting it. Today: no such step.
4. `tools/check_requirements_chain_test.py` exists, passes with `-q`, and holds
   one red case per refusal in item 3, on fixtures that carry a tag other than
   `VTT`. Today: no such file.
5. `docs/specifications/008-requirement-ids-come-from-the-dispenser.md`'s Status
   names the gate's step and self-test as the implemented half, its example id is
   written as the shape rather than an instance (`grep -cE 'VTT-[0-9]{3}'` on it
   prints 0), and `CLAUDE.md`'s register bullet no longer reads "Nothing checks
   the chain here today". Today: both say the chain is unchecked, and the
   example is an instance.
6. Still true afterwards: `go test ./internal/identity/... ./internal/gateway/...`
   is green; every test a row's evidence names exists as `func <name>(` in the
   file named; `TestARefusedJoinWritesNothingAtAll` and
   `TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY` pass unchanged.

## Rules this puts on the system

The chain, held by the gate:

- An id cited by a test or named by a specification resolves to a row in the
  register.
- A row's evidence names only files that exist and that carry the row's id.
- A row's id is the project tag and a number, and no two rows share one.

The arc's rules, as the pre-ticket and ADR-011 state them, for the sort to
accept or refuse. Each names the check that holds it today, or says none does.

- A join request carrying a secret that does not match the campaign's is
  refused, and the refusal writes nothing.
  (`TestAWrongSecretRefusesWithoutTouchingTheDatabase`)
- A join request arriving when the opening's admission budget is spent is
  refused, and the refusal writes nothing.
  (`TestASpentBudgetRefusesWithoutTouchingTheDatabase`)
- A join request arriving when the door is shut is refused, and the refusal
  writes nothing. (No test can see the write: `docs/verification-debt.md`,
  entry dated 2026-09-23.)
- A shut door, a wrong secret and a spent budget are refused with the same
  status and the same body. (`TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY`,
  `TestTheDoorStopsAdmittingWhenItsBudgetIsSpent`)
- A participant minted through the join link is a spectator, and the joiner
  chooses no role. (`TestJoiningThroughAnOpenDoorMintsASpectator`)
- A refused join creates no participant. (`TestAClosedDoorMintsNobody`)
- A display name is at most 64 runes, carries no control or bidi character,
  has something visible in it, and is refused distinctly from the door.
  (`TestADisplayNameIsBoundedAndPrintable`, `TestAnEmptyDisplayNameIsRefused`)
- A join request body larger than the cap is refused as malformed before it is
  read as a guess. (`TestAnOversizedBodyIsRefusedBeforeItIsRead`)
- A campaign's door is shut until the DM opens it, including a campaign that
  predates the door. (`TestJoinIsClosedOnAFreshCampaign`,
  `TestJoinIsClosedOnAnExistingCampaign`)
- Reading the link and checking the door mint nothing and open nothing.
  (`TestReadingTheLinkDoesNotOpenTheDoor`, `TestARefusedJoinWritesNothingAtAll`;
  `TestCheckingTheDoorMintsNothing` exercises `JoinAllows`, which has no
  production caller.)
- An empty stored secret admits nobody.
  (`TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath`)
- Rotating the link refuses the old secret, admits the new one, and touches no
  participant already through it. (`TestRotatingTheLinkRefusesTheOldSecret`,
  `TestRotatingTheSecretLeavesParticipantsAlone`)
- The admission budget is per opening: opening the door resets the count.
  (`TestABudgetIsPerOpeningNotPerCampaign`)
- A door opened with no stated budget, or a non-positive one, admits
  `DefaultAdmitLimit`. (`TestADoorOpenedWithNoStatedBudgetStillAdmits`)
- Two joiners racing for the last admission: exactly one is admitted.
  (`TestOnlyOneJoinerTakesTheLastSlot`)
- Two joiners through the same link are two participants with distinct
  credentials. (`TestTwoJoinersGetDistinctIdentities`)
- A promotion may make a participant a player or a spectator and nothing else.
  (`TestPromotionMayOnlyTargetPlayerOrSpectator`,
  `TestPromotingToDMIsRefusedOverTheWire`)
- A promotion cannot unmake a dm or an agent. (`TestPromotionCannotUNMAKEADMOrAgent`)
- A spectator cannot promote itself. (`TestASpectatorCannotPromoteItself`)
- Opening or closing the door and rotating the link are DM-and-agent only.
  (`TestAuthorizeTableAllCommandsAllRoles`)
- A promotion changes the role and nothing else about the participant, and a
  revoked participant stays revoked. (`TestSetRoleDoesNotDisturbTheCredential`,
  `TestSetRoleLeavesEVERYONEElseAlone`, `TestSetRoleOnARevokedParticipantStaysRevoked`)
- A promotion takes effect on the promoted participant's next command, without
  a reconnect. (`TestAPromotionBitesWithoutReconnecting`)
- A revoked participant is refused on their next command, their next delivered
  event and their next presence frame, without a reconnect.
  (`TestRevokingRemovesSomebodyWhoIsStillConnected`,
  `TestARevokedSpectatorStopsSeeingTheTable`,
  `TestARevokedWatcherIsNotEvenToldWhoElseArrives`)
- Only an invalid credential ends a connection; any other failure refuses the
  action and keeps the connection.
  (`TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`)
- Revoked participants are not listed.
  (`TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo`)
- A role lives in `participants.role` and nowhere in the event log; a promotion
  appends no event. (No test; `grep -rn Role internal/engine/*.go` prints
  nothing, and no test reads the log head across a promotion.)

## What it touches

In the order they must change:

1. New `tools/check_requirements_chain_test.py` and
   `tools/check-requirements-chain.py`; `Taskfile.yml`. The gate first, because
   it runs on an empty register and is what the rows are held by.
2. `docs/requirements.md`, and the test files that gain a citation:
   `internal/identity/identity_test.go`, `internal/identity/fault_internal_test.go`,
   `internal/gateway/join_test.go`, `internal/gateway/authz_test.go` and
   `internal/gateway/server_test.go`.
   Rows and citations land together, so the gate is green at every commit.
3. `docs/specifications/008-requirement-ids-come-from-the-dispenser.md` and
   `CLAUDE.md`, once the gate exists.

Nine existing files and two new ones, across the register, the tests of two
packages, the tools and the gate. This is already the first half of the arc's
work; the second half is its own ticket.

## Specifications this moves

docs/specifications/008-requirement-ids-come-from-the-dispenser.md

## What could not be established

- Whether the rule that a role never enters the event log is held by anything
  a check can run, or only by a search.
