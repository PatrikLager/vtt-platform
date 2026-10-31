# A display name belongs to one participant at a time

## The problem

**Two participants may hold one display name, and nothing at the table tells
them apart.** `handleJoin` in `internal/gateway/join.go` trims the name the
joiner typed, refuses it only if `usableDisplayName` rejects it, asks
`identity.JoinAdmits` about the door, and mints a spectator through
`identity.CreateInvite`. `CreateInvite` in `internal/identity/identity.go`
inserts the row as given: the `participants` table declares `display_name
TEXT` with no constraint, and no statement in the package reads the names
already there before inserting. `vtt invite` in `cmd/vtt/invite.go` calls the
same `CreateInvite` with its `--name` flag. So a second "Kim" is admitted,
whether a second person chose the name or the same person joined again from a
browser that had lost their credential.

The four places read for this ticket name a participant and show no id. The
DM's console lists the roster under "Who may do what" one row per participant
with `person.name` and a "Make player" or "Make spectator" button, and offers
every known participant in each character's grant select under the name
`known` holds for them (`renderDMConsole` in `client/src/view/dm.ts`). The
presence strip draws `p.displayName` (`client/src/view/spectator.ts`). An out
of character line is stamped with the speaker's participant name
(`internal/gateway/convert.go` sets `As = p.Name`). Two rows reading "Kim",
two options reading "Kim": the DM promotes one of them, or hands a character
to one of them, and cannot tell which. When one of the two is the stale
credential of somebody who joined again, the character goes to nobody at the
table.

MapTool refuses a second connection under a player name already connected:
`ServerHandshake.handle(ClientInitMsg)` asks `isPlayerConnected`, and the
client shows `Handshake.msg.playerAlreadyConnected`. There the name is the
identity. Here the identity is the participant id, and the owner's ruling of
2026-10-31 is to refuse a name another participant already holds, whether or
not they are connected, and to apply the same rule to `vtt invite`.

## Done looks like

1. A join through the share link, carrying the current secret at an open
   door with budget left, under a display name that a participant who is not
   revoked already holds, is refused, and no participant is created. "Already
   holds" ignores case: `kim` is refused while `Kim` is held, and `åsa` while
   `Åsa` is. A test that names this fails on today's tree, where the join is
   admitted.
2. The join view tells the joiner the name is taken, in words distinct from
   the door's refusal, and the name they typed stays in the box. A client test
   names this.
3. A join refused for a taken name spends no admission: `JoinBudget` reports
   the same spent count before and after it.
4. A taken name reaches nobody without the link: a join under a taken name
   with a wrong secret, at a shut door, or against a spent budget, gets the
   one status and the one body VTT-009 already requires of those three.
5. A name held only by a revoked participant is admitted.
6. `vtt invite --name <n>`, where a participant who is not revoked holds `<n>`
   ignoring case, exits non-zero, prints no token and creates no participant.
   A test names this, and it fails on today's tree.
7. A name no participant who is not revoked holds still joins and still
   invites: the existing tests of `internal/gateway/join_test.go`,
   `cmd/vtt` and `client/test/join.test.ts` pass.
8. VTT-072, "Listing orders by display name then id, a total order", still
   names a passing test.
9. `task check` exits 0 on the branch.

## Rules this puts on the system

- No two participants who are not revoked hold the same display name,
  compared ignoring case, by whichever path the second came in: the share link
  or `vtt invite`.
- A join refused for a taken name spends no admission.
- A join under a taken name is told so only when the door would have admitted
  it; otherwise it gets the door's one answer.
- A join refused for a taken name says so, distinctly from the door's refusal.

## What it touches

In this order, because each layer's refusal is what the next one reports:

1. `internal/identity/identity.go`, and its tests in
   `internal/identity/identity_test.go`: where the names already held are read
   and the second one refused.
2. `internal/gateway/join.go`, and `internal/gateway/join_test.go`: the
   join's answer.
3. `cmd/vtt/invite.go`, and a test in `cmd/vtt`: the invite's refusal.
4. `client/src/join.ts`, `client/src/view/join.ts` and
   `client/test/join.test.ts`: what the joiner reads. Possibly no change if
   the join's answer arrives as the status whose body the view already shows
   (`requestJoin` reads any 400's body, trimmed and without its `gateway: `
   prefix, as the name's refusal).
5. `docs/requirements.md`: the rules above.

Four components. Whether to split them is asked at sign-off; the client half
may come to tests alone.

## Specifications this moves

docs/specifications/009-identity-and-joining.md

## What could not be established

- Two joiners racing under one name at the same moment. Whether the names
  are read and the participant inserted in one statement or transaction is
  the plan's; if not, both may be admitted. How often two people type one
  name in the same instant at a table was not measured.
- Names that differ only in Unicode normalization (a composed `Å` against
  `A` and a combining ring) or in surrounding whitespace. `handleJoin` trims;
  `vtt invite` neither trims nor runs `usableDisplayName`, so `--name "Kim "`
  is stored as typed.
- Duplicates already in a campaign. Nothing is migrated, by the owner's
  standing ruling that nobody uses the product yet; two Kims who joined
  before this lands stay indistinguishable.
- What the DM does with a stale participant. `vtt revoke --id` is the only
  way to revoke one, and the console shows no participant id.
- Which of the 144 `CreateInvite` calls in 43 test files mint two
  participants under one name ignoring case. That decides how much a rule
  inside `CreateInvite` moves, and it is the plan's to count.
