# Requirements

project: VTT

A requirement is a rule the platform must obey, written once and cited by
whatever proves it. An id is allocated by `requirement-id` and never chosen by
hand; the evidence cell is never blank. How ids are allocated, cited and
checked: `docs/specifications/008-requirement-ids-come-from-the-dispenser.md`.

| Id | Requirement | Verified by |
|---|---|---|
| VTT-001 | An id cited by a test or named by a specification resolves to a row in the register. | tools/check_requirements_chain_test.py#test_a_test_citing_an_id_no_row_defines_is_refused, tools/check_requirements_chain_test.py#test_a_specification_naming_an_id_no_row_defines_is_refused |
| VTT-002 | A row's evidence names only files that exist, carry the row's id, and hold the check named. | tools/check_requirements_chain_test.py#test_evidence_naming_a_missing_file_is_refused, tools/check_requirements_chain_test.py#test_evidence_naming_a_file_that_does_not_carry_the_id_is_refused, tools/check_requirements_chain_test.py#test_evidence_naming_a_check_that_is_not_in_the_file_is_refused |
| VTT-003 | A row's id is the project tag and a number. | tools/check_requirements_chain_test.py#test_a_row_id_of_another_shape_is_refused, tools/check_requirements_chain_test.py#test_a_row_numbered_zero_is_refused |
| VTT-004 | No two rows share an id. | tools/check_requirements_chain_test.py#test_two_rows_sharing_an_id_are_refused |
| VTT-005 | A join request carrying a secret that does not match the campaign's is refused, and the refusal writes nothing. | internal/identity/fault_internal_test.go#TestAWrongSecretRefusesWithoutTouchingTheDatabase |
| VTT-006 | A join request arriving when the opening's admission budget is spent is refused, and the refusal writes nothing. | internal/identity/fault_internal_test.go#TestASpentBudgetRefusesWithoutTouchingTheDatabase |
| VTT-007 | A join request arriving when the door is shut is refused. | internal/gateway/join_test.go#TestAClosedDoorMintsNobody, internal/identity/identity_test.go#TestAClosedDoorSpendsNothing |
| VTT-008 | A join request refused at a shut door writes nothing. | internal/identity/fault_internal_test.go#TestAShutDoorRefusesWithoutTouchingTheDatabase |
| VTT-009 | A shut door, a wrong secret and a spent budget are refused with the same status and the same body. | internal/gateway/join_test.go#TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY, internal/gateway/join_test.go#TestTheDoorStopsAdmittingWhenItsBudgetIsSpent |
| VTT-010 | A participant minted through the join link is a spectator. | internal/gateway/join_test.go#TestJoiningThroughAnOpenDoorMintsASpectator |
| VTT-011 | A refused join creates no participant. | internal/gateway/join_test.go#TestAClosedDoorMintsNobody, internal/gateway/join_test.go#TestTheDoorStopsAdmittingWhenItsBudgetIsSpent, internal/gateway/join_test.go#TestAnOversizedBodyIsRefusedBeforeItIsRead |
| VTT-012 | A display name that is empty, longer than the bound, or carries a control, bidi or invisible-only content is refused, and an ordinary non-ASCII name is admitted. | internal/gateway/join_test.go#TestADisplayNameIsBoundedAndPrintable, internal/gateway/join_test.go#TestAnEmptyDisplayNameIsRefused |
| VTT-013 | A refused display name is refused distinctly from the door's refusal. | internal/gateway/join_test.go#TestAnEmptyDisplayNameIsRefused |
| VTT-014 | A join request body larger than the cap is refused as malformed before it is read as a guess. | internal/gateway/join_test.go#TestAnOversizedBodyIsRefusedBeforeItIsRead |
| VTT-015 | A campaign's door is shut until the DM opens it, including a campaign that predates the door. | internal/identity/identity_test.go#TestJoinIsClosedOnAFreshCampaign, internal/identity/identity_test.go#TestJoinIsClosedOnAnExistingCampaign |
| VTT-016 | Reading the link does not open the door. | internal/identity/identity_test.go#TestReadingTheLinkDoesNotOpenTheDoor |
| VTT-017 | A join request on a campaign whose door has never been touched creates no door row. | internal/gateway/join_test.go#TestARefusedJoinWritesNothingAtAll |
| VTT-018 | An empty stored secret admits nobody. | internal/identity/identity_test.go#TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath |
| VTT-019 | Rotating the link refuses the old secret and admits the new one. | internal/gateway/join_test.go#TestRotatingTheLinkRefusesTheOldSecret |
| VTT-020 | Rotating the link touches no participant already through it. | internal/identity/identity_test.go#TestRotatingTheSecretLeavesParticipantsAlone, internal/gateway/server_test.go#TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse |
| VTT-021 | The admission budget is per opening: opening the door resets the count. | internal/identity/identity_test.go#TestABudgetIsPerOpeningNotPerCampaign |
| VTT-022 | A door opened with no stated budget, or a non-positive one, admits the default budget. | internal/identity/identity_test.go#TestADoorOpenedWithNoStatedBudgetStillAdmits |
| VTT-023 | Two joiners racing for the last admission: exactly one is admitted. | internal/identity/identity_test.go#TestOnlyOneJoinerTakesTheLastSlot |
| VTT-024 | Two joiners through the same link are two participants with distinct credentials. | internal/gateway/join_test.go#TestTwoJoinersGetDistinctIdentities |
| VTT-025 | A promotion may make a participant a player or a spectator and nothing else. | internal/gateway/authz_test.go#TestPromotionMayOnlyTargetPlayerOrSpectator, internal/gateway/server_test.go#TestPromotingToDMIsRefusedOverTheWire, internal/gateway/authz_test.go#TestAnAgentMayNotPromoteAnyoneToDMOrAgent |
| VTT-026 | A promotion cannot unmake a dm or an agent. | internal/gateway/server_test.go#TestPromotionCannotUNMAKEADMOrAgent |
| VTT-027 | A spectator cannot promote itself. | internal/gateway/authz_test.go#TestASpectatorCannotPromoteItself |
| VTT-028 | Opening or closing the door, rotating the link and promoting a participant are DM-and-agent only. | internal/gateway/authz_test.go#TestAuthorizeTableAllCommandsAllRoles |
| VTT-029 | A promotion changes the role and nothing else about the participant. | internal/identity/identity_test.go#TestSetRoleDoesNotDisturbTheCredential |
| VTT-030 | A revoked participant stays revoked through a promotion. | internal/identity/identity_test.go#TestSetRoleOnARevokedParticipantStaysRevoked |
| VTT-031 | A promotion takes effect on the promoted participant's next command, without a reconnect. | internal/gateway/server_test.go#TestAPromotionBitesWithoutReconnecting |
| VTT-032 | A revoked participant is refused on their next command, their next delivered event and their next presence frame, without a reconnect. | internal/gateway/server_test.go#TestRevokingRemovesSomebodyWhoIsStillConnected, internal/gateway/server_test.go#TestARevokedSpectatorStopsSeeingTheTable, internal/gateway/server_test.go#TestARevokedWatcherIsNotEvenToldWhoElseArrives |
| VTT-033 | An identity store that cannot answer refuses the command, keeps the connection, and delivery continues. | internal/gateway/server_test.go#TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody |
| VTT-034 | Revoked participants are not listed. | internal/identity/identity_test.go#TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo |
| VTT-035 | WITHDRAWN 2026-09-24: No event carries a role. False as worded: `Envelope.actor_role` is a role field on every Envelope. Succeeded by VTT-038 and VTT-039. | **READING — verify-ticket check 5 of 2026-09-24; withdrawn, held by nothing** |
| VTT-036 | A promotion appends no event. | internal/gateway/server_test.go#TestAPromotionAppendsNoEvent |
| VTT-037 | Promoting one participant changes no other participant's role. | internal/identity/identity_test.go#TestSetRoleLeavesEVERYONEElseAlone |
| VTT-038 | No event payload names a role. | internal/gateway/authz_test.go#TestNoEventPayloadNamesARole, internal/engine/qa_role_test.go#TestQANoEventPayloadNamesARole |
| VTT-039 | Folding an event yields the same state whatever role its envelope carries. | internal/engine/role_test.go#TestTheFoldIgnoresTheEnvelopesRole, internal/engine/qa_role_test.go#TestQAFoldingIgnoresTheEnvelopesRole |
| VTT-040 | Reading the join secret does not change it. | internal/identity/identity_test.go#TestTheJoinSecretIsStableUntilRotated |
| VTT-041 | A participant's credential is stored as its hash. | internal/identity/identity_test.go#TestTokenNotRecoverableFromDB |
| VTT-042 | A join carrying the current secret at an open door with budget remaining is admitted. | internal/identity/identity_test.go#TestTheDoorNeedsBOTHTheFlagAndTheSecret |
| VTT-043 | Rotating the link leaves the door open or shut as it was. | internal/identity/identity_test.go#TestRotatingTheSecretLeavesTheDoorAlone, internal/identity/identity_test.go#TestRotatingBeforeAnythingElseLeavesTheDoorSHUT, internal/gateway/server_test.go#TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse |
| VTT-044 | At an open door, a link rotated after its budget is spent admits a holder of the new secret. | internal/identity/identity_test.go#TestRotatingAfterASpentBudgetGivesAWorkingLink |
| VTT-045 | An admission whose spend cannot be recorded is not granted. | internal/identity/fault_internal_test.go#TestAnAdmissionThatCannotBeSpentIsNotGranted |
| VTT-046 | A door whose database cannot answer admits nobody. | internal/identity/fault_internal_test.go#TestTheDoorStateReadFailingIsNotAnAdmission, internal/identity/identity_test.go#TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting |
| VTT-047 | A promotion is announced on the promoted participant's own connection. | internal/gateway/server_test.go#TestAPromotionIsAnnouncedToThePromotedPersonThemselves |
| VTT-048 | A door whose database cannot answer reads as shut. | internal/identity/identity_test.go#TestTheDoorRefusesWhenTheDatabaseIsUnusable |
| VTT-049 | Rotating the link against a database that cannot answer reports the failure. | internal/identity/identity_test.go#TestTheDoorRefusesWhenTheDatabaseIsUnusable, internal/identity/identity_test.go#TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting |
| VTT-050 | An added comment line carries none of the banned terms the comment gate names. | **OPEN — no test yet** |
| VTT-051 | A comment in code is an imperative warning, a pointer to a record, or the one-line doc sentence of an exported symbol. | **READING — Phase 4b** |
| VTT-052 | A comment block longer than the bound is refused when a change adds a line to it, the package doc excepted. | **OPEN — no test yet** |
| VTT-053 | A change that adds a comment line to a file leaves that file at or under its ceiling. | **OPEN — no test yet** |
| VTT-054 | A ledger row is never raised above the base's. | **OPEN — no test yet** |
| VTT-055 | A share more than the band under its ceiling is refused until the ceiling is lowered. | **OPEN — no test yet** |
| VTT-056 | Every ledger row names a file that exists. | **OPEN — no test yet** |
| VTT-057 | A file with no row is held to the default ceiling. | **OPEN — no test yet** |
| VTT-058 | A run of the comment gate that scans nothing, has no base, or finds no ledger fails. | **OPEN — no test yet** |
