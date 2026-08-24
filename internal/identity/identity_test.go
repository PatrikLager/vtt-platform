package identity_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func openTemp(t *testing.T) (*identity.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d, path
}

func TestCreateInviteVerifyRoundTrip(t *testing.T) {
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("want non-empty token")
	}
	if id == "" {
		t.Fatal("want non-empty id")
	}

	p, err := d.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != id {
		t.Errorf("ID: got %q, want %q", p.ID, id)
	}
	if p.Name != "Lera" {
		t.Errorf("Name: got %q, want %q", p.Name, "Lera")
	}
	if p.Role != identity.RolePlayer {
		t.Errorf("Role: got %q, want %q", p.Role, identity.RolePlayer)
	}
}

// TestTokenNotRecoverableFromDB proves the raw token is not stored anywhere
// retrievable: it opens a second, independent SQLite handle on the same
// file and reads the persisted token_hash directly, asserting it neither
// equals the raw token bytes nor anything other than sha256(token).
func TestTokenNotRecoverableFromDB(t *testing.T) {
	d, path := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	var stored []byte
	if err := raw.QueryRow(`SELECT token_hash FROM participants WHERE id = ?`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(stored, []byte(token)) {
		t.Fatal("stored token_hash equals the raw token bytes — token is recoverable")
	}
	want := sha256.Sum256([]byte(token))
	if !bytes.Equal(stored, want[:]) {
		t.Fatalf("stored token_hash != sha256(token): got %x want %x", stored, want)
	}
}

func TestVerifyRejectsWrongToken(t *testing.T) {
	d, _ := openTemp(t)
	if _, _, err := d.CreateInvite("Lera", identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify("this-is-not-a-real-token"); err == nil {
		t.Fatal("want error for a token that was never issued")
	}
}

func TestRevokedTokenRejectedAfterRevoke(t *testing.T) {
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err != nil {
		t.Fatalf("Verify before revoke: %v", err)
	}
	if err := d.Revoke(id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err == nil {
		t.Fatal("want error for a revoked token")
	}
}

func TestParseRoleAcceptsExactlyTheFourRoles(t *testing.T) {
	cases := []struct {
		in   string
		want identity.Role
	}{
		{"dm", identity.RoleDM},
		{"agent", identity.RoleAgent},
		{"player", identity.RolePlayer},
		{"spectator", identity.RoleSpectator},
	}
	for _, c := range cases {
		got, err := identity.ParseRole(c.in)
		if err != nil {
			t.Errorf("ParseRole(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseRole(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "DM", "Player", "admin", "gm", "npc"} {
		if _, err := identity.ParseRole(bad); err == nil {
			t.Errorf("ParseRole(%q): want error, got nil", bad)
		}
	}
}

func TestTwoInvitesProduceDistinctTokensAndIDs(t *testing.T) {
	d, _ := openTemp(t)
	token1, id1, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	token2, id2, err := d.CreateInvite("Ursus", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}
	if token1 == token2 {
		t.Fatal("want distinct tokens for two invites")
	}
	if id1 == id2 {
		t.Fatal("want distinct ids for two invites")
	}
}

// TestVerifyUsesConstantTimeCompare is the white-box half of the Step 1
// "constant-time-shaped" case (task-3-brief.md, binding DESIGN NOTE): the
// hash lookup itself (SELECT ... WHERE token_hash = ?) is fine as a plain
// indexed comparison because the hash is not secret-timing-sensitive, but
// the final confirmation of a match must be constant-time so no timing
// side channel on token bytes exists. That correctness is exercised
// behaviorally above (round-trip, wrong-token, revoked-token); this test
// enforces the documented implementation contract itself.
func TestVerifyUsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("identity.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "subtle.ConstantTimeCompare") {
		t.Fatal("Verify must confirm the token hash match with subtle.ConstantTimeCompare")
	}
}

// TestCoexistsWithStoreOnSameFile proves identity opens its own SQLite
// handle independent of store.Store and that both handles operate
// correctly against the same campaign file (spec: identity is deliberately
// NOT event-sourced and lives beside, not inside, the event log).
func TestCoexistsWithStoreOnSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")

	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Exercise the identity handle.
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err != nil {
		t.Fatal(err)
	}

	// Exercise the store handle on the SAME file.
	env := &vttv1.Envelope{
		EventId:   "e1",
		SessionId: "sess-1",
		ActorRole: "dm",
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "test"},
		},
	}
	if _, err := s.Append(env); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ReadAfter: got %d events, want 1", len(got))
	}

	// And once more, prove identity is still independently usable.
	if p, err := d.Verify(token); err != nil || p.Name != "Lera" {
		t.Fatalf("Verify after store use: p=%v err=%v", p, err)
	}
}

// --- the join door (joining-a-table T1) -------------------------------------

// TestJoinIsClosedOnAnExistingCampaign is the test this task exists for, and
// the first version of it was VACUOUS — deleting the entire join_access CREATE
// left it green, because its "existing" campaign was built by the NEW schema.
// It stood in for an upgrade that never met an un-upgraded database.
//
// This one builds the OLD schema by hand, through a raw handle, so the table
// genuinely does not exist when identity.Open runs. That is what makes it able
// to fail: Open() applies the schema with CREATE TABLE IF NOT EXISTS, so a new
// COLUMN on participants would never reach an existing campaign — a new TABLE
// does. Closed-by-default is the security property (spec §2), and getting it
// wrong would open joining on exactly the campaigns that already have players.
//
// The JoinSecret() assertion in the middle is load-bearing: without it the test
// passes whether the table was created or not, because JoinOpen() answers false
// down its error path either way.
func TestJoinIsClosedOnAnExistingCampaign(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// The schema as it stood BEFORE this feature: participants only.
	if _, err := raw.Exec(`CREATE TABLE participants (
		id TEXT PRIMARY KEY, display_name TEXT, role TEXT, controls TEXT,
		token_hash BLOB UNIQUE, revoked INTEGER DEFAULT 0);`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(
		`INSERT INTO participants (id, display_name, role, controls, token_hash, revoked)
		 VALUES ('p-old', 'DM', 'dm', '[]', X'00', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	d, err := identity.Open(path)
	if err != nil {
		t.Fatalf("opening a campaign that predates this feature: %v", err)
	}
	defer d.Close()

	if d.JoinOpen() {
		t.Fatal("an existing campaign must come up with the door CLOSED")
	}
	// Proves the table was actually CREATED, not merely absent.
	if _, err := d.JoinSecret(); err != nil {
		t.Fatalf("the migration did not reach an existing campaign: %v", err)
	}
	if d.JoinOpen() {
		t.Fatal("minting the secret on an upgraded campaign must not open the door")
	}
}

func TestJoinIsClosedOnAFreshCampaign(t *testing.T) {
	d, _ := openTemp(t)
	if d.JoinOpen() {
		t.Fatal("a new campaign must come up with the door closed")
	}
}

func TestTheDoorOpensAndClosesAgain(t *testing.T) {
	d, _ := openTemp(t)
	if err := d.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	if !d.JoinOpen() {
		t.Fatal("opening the door must take effect")
	}
	if err := d.SetJoinOpen(false, 0); err != nil {
		t.Fatal(err)
	}
	if d.JoinOpen() {
		t.Fatal("closing it again must take effect — a door that only opens is not a door")
	}
}

func TestTheDoorSurvivesAReopen(t *testing.T) {
	// It is operational state, but it is PERSISTENT operational state: a DM
	// who opens the door and restarts the server has not closed it.
	d, path := openTemp(t)
	if err := d.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if !again.JoinOpen() {
		t.Fatal("the door's state must survive a restart")
	}
}

func TestTheJoinSecretIsStableUntilRotated(t *testing.T) {
	// Stable, because the DM shares it — a secret that changed per call would
	// invalidate the link the moment anyone looked at it.
	d, _ := openTemp(t)
	first, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" {
		t.Fatal("a join secret must exist")
	}
	second, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatal("reading the secret twice must give the same value")
	}
}

func TestRotatingTheSecretInvalidatesTheOldLink(t *testing.T) {
	// The property spec §2 calls close to required: a leaked link must be
	// closable WITHOUT re-inviting anyone already in.
	d, _ := openTemp(t)
	old, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := d.RotateJoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if fresh == old {
		t.Fatal("rotating must produce a different secret, or a leaked link stays valid")
	}
	now, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if now != fresh {
		t.Fatal("after rotating, the secret in use must be the new one")
	}
}

func TestRotatingTheSecretLeavesParticipantsAlone(t *testing.T) {
	// The other half of the same property: rotating closes the door to
	// NEWCOMERS and touches nobody already through it.
	d, _ := openTemp(t)
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.RotateJoinSecret(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err != nil {
		t.Fatalf("rotating the join link must not invalidate an existing participant: %v", err)
	}
}

// TestReadingTheLinkDoesNotOpenTheDoor pins the value the row is CREATED with,
// which nothing above actually reached.
//
// The two closed-by-default tests pass while no join_access row exists at all:
// JoinOpen's query finds nothing, errors, and fails closed. Correct, but it
// means the STORED value was never exercised — injection proved it, flipping
// both the column DEFAULT and the INSERT literal to 1 failed nothing.
//
// This is the shape a DM actually produces: look at the link (which mints the
// row) before deciding to let anyone in. The door must still be shut.
func TestReadingTheLinkDoesNotOpenTheDoor(t *testing.T) {
	d, _ := openTemp(t)
	if _, err := d.JoinSecret(); err != nil {
		t.Fatal(err)
	}
	if d.JoinOpen() {
		t.Fatal("minting the join secret must not admit anybody — reading the link is not " +
			"a decision to open the door")
	}
}

// TestTheDoorRefusesWhenTheDatabaseIsUnusable covers the error paths, and they
// are worth covering rather than merely counting: this is the case where
// failing in the wrong direction is expensive.
//
// A closed handle stands in for "the database cannot be read" generally. Every
// write must report the failure rather than pretend it worked, and JoinOpen
// must answer FALSE — it gates an unauthenticated, row-minting endpoint, so a
// database it cannot read must keep people OUT.
func TestTheDoorRefusesWhenTheDatabaseIsUnusable(t *testing.T) {
	d, _ := openTemp(t)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	if d.JoinOpen() {
		t.Fatal("an unreadable database must answer CLOSED — failing open here admits " +
			"strangers on exactly the fault nobody is watching")
	}
	if _, err := d.JoinSecret(); err == nil {
		t.Fatal("reading the secret from a dead handle must report the failure")
	}
	if _, err := d.RotateJoinSecret(); err == nil {
		t.Fatal("rotating against a dead handle must report the failure, or a DM believes " +
			"a leaked link was closed when it was not")
	}
	if err := d.SetJoinOpen(true, 100); err == nil {
		t.Fatal("opening the door against a dead handle must report the failure")
	}
	if _, err := d.Lookup("p-anyone"); err == nil {
		t.Fatal("a lookup against a dead handle must report the failure, not resolve")
	}
	if err := d.SetRole("p-anyone", identity.RolePlayer); err == nil {
		t.Fatal("promoting against a dead handle must report the failure — a DM console " +
			"that reports success while the database is gone is worse than one that errors")
	}
}

// TestRotatingTheSecretLeavesTheDoorAlone pins the independence of the two
// controls. They are separate decisions and the code claims to keep them
// separate; injection showed nothing was checking.
//
// The closed case is the security-relevant one: a DM who rotates a leaked link
// while the table is shut must not thereby open it. The open case matters too —
// rotating mid-session should not lock out the people still arriving.
func TestRotatingTheSecretLeavesTheDoorAlone(t *testing.T) {
	for _, open := range []bool{false, true} {
		d, _ := openTemp(t)
		if err := d.SetJoinOpen(open, 100); err != nil {
			t.Fatal(err)
		}
		if _, err := d.RotateJoinSecret(); err != nil {
			t.Fatal(err)
		}
		if d.JoinOpen() != open {
			t.Fatalf("rotating the link changed the door from open=%v to open=%v — they are "+
				"separate decisions, and rotating a leaked link must never admit anybody",
				open, d.JoinOpen())
		}
	}
}

// TestTheDoorOpensOnACampaignThatAlreadyHasALink covers SetJoinOpen's CONFLICT
// branch, which nothing reached: every SetJoinOpen(true, 100) in this file ran on a
// database with no row, so `true` only ever exercised the INSERT.
//
// Measured by review: `DO UPDATE SET open = excluded.open` -> `SET open = 0`
// passed the whole package. The failing input is the ordinary one — the DM
// reads the link, THEN opens the door — and the door would never open again
// for the life of that campaign.
func TestTheDoorOpensOnACampaignThatAlreadyHasALink(t *testing.T) {
	d, _ := openTemp(t)
	if _, err := d.JoinSecret(); err != nil { // mints the row, closed
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	if !d.JoinOpen() {
		t.Fatal("opening the door on a campaign that already has a link must work — " +
			"reading the link first is the ordinary order, not an edge case")
	}
}

// TestOpeningTheDoorFirstStillMintsARealSecret pins the secret SetJoinOpen
// writes when it is the call that creates the row.
//
// Measured by review: `VALUES (1, ?, ?)` -> `VALUES (1, ”, ?)` survived the
// whole suite, because every other secret assertion runs on a row minted by
// ensureJoinRow. A DM who opens the door before ever looking at the link would
// get an EMPTY secret — and the join endpoint would then compare an
// attacker-suppliable "" against a stored "".
func TestOpeningTheDoorFirstStillMintsARealSecret(t *testing.T) {
	a, _ := openTemp(t)
	if err := a.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	secret, err := a.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if secret == "" {
		t.Fatal("opening the door must mint a real secret, not an empty one")
	}
	b, _ := openTemp(t)
	if err := b.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	other, err := b.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if other == secret {
		t.Fatal("two campaigns must not share a join secret")
	}
}

// --- promotion (joining-a-table J3) ----------------------------------------

func TestSetRolePromotesTheNamedParticipant(t *testing.T) {
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Kim", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	p, err := d.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != identity.RolePlayer {
		t.Fatalf("role = %q, want player — the same TOKEN must now carry the new role, "+
			"because that is what the connection reads", p.Role)
	}
}

func TestSetRoleLeavesEVERYONEElseAlone(t *testing.T) {
	// A missing WHERE promotes the whole table, and the mutation gate cannot
	// see SQL (#40), so this is guarded by hand or not at all.
	d, _ := openTemp(t)
	_, kim, err := d.CreateInvite("Kim", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	adaToken, _, err := d.CreateInvite("Ada", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(kim, identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	ada, err := d.Verify(adaToken)
	if err != nil {
		t.Fatal(err)
	}
	if ada.Role != identity.RoleSpectator {
		t.Fatalf("Ada became %q — promoting one participant must not promote the table", ada.Role)
	}
}

func TestSetRoleRejectsARoleThatIsNotOneOfTheFour(t *testing.T) {
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Kim", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.Role("superuser")); err == nil {
		t.Fatal("an unknown role must be refused — ParseRole is the whole set, and a row " +
			"carrying anything else is a participant no authz cell describes")
	}
}

func TestSetRoleOnSomeoneWhoDoesNotExistIsAnError(t *testing.T) {
	// Silence here would let the DM console report a successful promotion of
	// somebody who left, and the caller could never tell.
	d, _ := openTemp(t)
	if err := d.SetRole("p-nobody", identity.RolePlayer); err == nil {
		t.Fatal("promoting an unknown participant must report it, not succeed quietly")
	}
}

func TestSetRoleToTheSameRoleIsFine(t *testing.T) {
	// A DM clicking twice is not an error.
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Kim", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatalf("re-promoting to the same role must be a no-op, not an error: %v", err)
	}
}

func TestSetRoleDoesNotDisturbTheCredential(t *testing.T) {
	// The token and the name belong to the person, not the role. A promotion
	// that rewrote the credential would silently log them out.
	//
	// It used to assert that the promotion left `controls` alone too. That
	// column is gone (2026-08-24) and the property it stood for is now
	// structural rather than tested: what a promoted player holds lives in the
	// log, which SetRole's single UPDATE against participants cannot reach.
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Kim", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	p, err := d.Verify(token)
	if err != nil {
		t.Fatalf("the token must still verify after a promotion: %v", err)
	}
	if p.ID != id || p.Name != "Kim" {
		t.Fatalf("promotion changed identity: %+v", p)
	}
	if p.Role != identity.RolePlayer {
		t.Fatalf("the promotion itself did not take: %+v", p)
	}
}

func TestSetRoleOnARevokedParticipantStaysRevoked(t *testing.T) {
	// Promotion must not be a way back in for somebody who was thrown out.
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Mallory", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Revoke(id); err != nil {
		t.Fatal(err)
	}
	_ = d.SetRole(id, identity.RolePlayer)
	if _, err := d.Verify(token); err == nil {
		t.Fatal("promoting a revoked participant must not restore them")
	}
}

// --- live re-resolution (joining-a-table J4, spec §3.2) --------------------

func TestLookupReflectsAPromotionImmediately(t *testing.T) {
	// The property the whole of J4 exists for: authentication is a
	// connection-time fact, authorization is a LIVE one. A promotion must be
	// visible to the very next thing the participant does.
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Kim", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	p, err := d.Lookup(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != identity.RolePlayer {
		t.Fatalf("role = %q, want player — a lookup that returns the OLD role is exactly "+
			"the caching this replaces", p.Role)
	}
}

func TestLookupRefusesARevokedParticipant(t *testing.T) {
	// The serious half. Until this existed, `vtt revoke` removed nobody: the
	// only Verify was at connect, so a revoked participant kept playing until
	// they chose to disconnect. Throwing someone out did nothing without their
	// cooperation.
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Mallory", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Revoke(id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Lookup(id); err == nil {
		t.Fatal("a revoked participant must not resolve — otherwise revocation waits on " +
			"the revoked person's goodwill")
	}
}

func TestLookupRefusesAnUnknownParticipant(t *testing.T) {
	d, _ := openTemp(t)
	if _, err := d.Lookup("p-nobody"); err == nil {
		t.Fatal("an unknown id must not resolve")
	}
}

func TestLookupCarriesTheWholeParticipant(t *testing.T) {
	// Not just the role: Authorize reads ID for ownership checks, so a partial
	// lookup would silently change what authorization sees.
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Kim", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Lookup(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != id || p.Name != "Kim" || p.Role != identity.RolePlayer {
		t.Fatalf("lookup lost identity: %+v", p)
	}
}

func TestLookupRefusesACorruptRow(t *testing.T) {
	// A row whose role cannot be parsed must NOT resolve. It is unreachable
	// through this package's own writers, which is exactly why it is worth
	// asserting: if a row ever became malformed — a hand-edited database, a
	// botched migration, a future writer with a bug — the failure must be a
	// refusal, not a participant with a silently wrong authorization level.
	//
	// This used to be a two-case table; the second case fed unparseable JSON to
	// participants.controls, a column deleted on 2026-08-24. Role is now the
	// only stored field this decodes rather than reads.
	d, path := openTemp(t)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(
		`INSERT INTO participants (id, display_name, role, token_hash, revoked)
		 VALUES ('p-bad', 'Bad', 'superuser', X'01', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if _, err := again.Lookup("p-bad"); err == nil {
		t.Fatal("a corrupt row must be refused, not resolved into a participant whose " +
			"authorization nobody can account for")
	}
}

// TestCheckingTheDoorMintsNothing pins the claim spec §2 rests its whole
// "therefore no rate limiting" argument on: with the door shut the link is
// INERT, so there is nothing for a stranger to hammer.
//
// It was not inert. The join endpoint answered a guess through JoinSecret,
// which mints the row when a campaign has never had one — so an anonymous,
// unauthenticated, REFUSED request performed an INSERT, taking SQLite's write
// lock on a file internal/store writes to inside a transaction on every event
// append. That is the exact hazard ensureJoinRow's read-first path was
// restructured to avoid for the DM console, on the one path a stranger drives.
func TestRotatingBeforeAnythingElseLeavesTheDoorSHUT(t *testing.T) {
	// RotateJoinSecret is an upsert, and its INSERT branch is reached only on a
	// campaign whose join_access row does not exist yet — `vtt join-link rotate`
	// as the very first thing anybody does, or an agent's rotate_join_link.
	// Every other rotation test calls SetJoinOpen or JoinSecret first, so all of
	// them reach DO UPDATE and none of them reaches this.
	//
	// Flip that branch's `open` literal from 0 to 1 and the whole repository
	// stays green — the Go mutation gate cannot mutate SQL text, and the
	// column's DEFAULT 0 is no backstop because both upserts write it
	// explicitly. The two sibling literals carrying this same property ARE
	// pinned; this was the one that was not, and the two statements sit forty
	// lines apart and differ only in that value.
	d, _ := openTemp(t)

	secret, err := d.RotateJoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if d.JoinOpen() {
		t.Fatal("rotating a link on a campaign nobody has opened must not open the door — " +
			"the DM would be handed a live link by an operation that says nothing about " +
			"letting anyone in")
	}
	// And the door is shut in the way that matters: the fresh secret does not
	// get anybody in.
	allowed, err := d.JoinAllows(secret)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("a brand-new secret admitted somebody through a door that was never opened")
	}
}

func TestCheckingTheDoorMintsNothing(t *testing.T) {
	d, path := openTemp(t)

	allowed, err := d.JoinAllows("a stranger's guess")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("a campaign whose door was never opened must refuse everyone")
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRow(`SELECT count(*) FROM join_access`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a refused, anonymous join wrote %d row(s) — the closed door is not inert, "+
			"and spec §2's case against rate limiting depends on it being inert", n)
	}
}

// TestTheDoorNeedsBOTHTheFlagAndTheSecret walks all four cells. Three of them
// refuse, and each refuses for its own reason: a guard that only ever says yes
// is not a guard, and one that says no for the wrong reason is worse.
func TestTheDoorNeedsBOTHTheFlagAndTheSecret(t *testing.T) {
	d, _ := openTemp(t)
	right, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		open   bool
		offer  string
		expect bool
	}{
		{open: true, offer: right, expect: true},
		{open: true, offer: right + "x", expect: false},
		{open: false, offer: right, expect: false},
		{open: false, offer: right + "x", expect: false},
	} {
		if err := d.SetJoinOpen(c.open, 100); err != nil {
			t.Fatal(err)
		}
		got, err := d.JoinAllows(c.offer)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.expect {
			t.Fatalf("door open=%v, correct secret=%v: allowed=%v, want %v",
				c.open, c.offer == right, got, c.expect)
		}
	}
}

// TestAnEmptyStoredSecretAdmitsNobody guards the degenerate compare.
// subtle.ConstantTimeCompare("", "") returns 1, so a blank stored secret plus a
// caller who sends no secret at all is a MATCH — and a request body that simply
// omits the field decodes to "". Unreachable through this package's own
// writers, which is exactly why it earns a test: a hand-edited database or a
// future writer with a bug must fail CLOSED, not admit the world.
func TestAnEmptyStoredSecretAdmitsNobody(t *testing.T) {
	d, path := openTemp(t)
	if err := d.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE join_access SET secret = '' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	for _, offer := range []string{"", "anything"} {
		allowed, err := d.JoinAllows(offer)
		if err != nil {
			t.Fatal(err)
		}
		if allowed {
			t.Fatalf("an empty stored secret must admit nobody, but %q got in", offer)
		}
	}
}

// TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo backs the DM console's
// promote control.
//
// The console cannot answer "who is a spectator?" from presence: presence
// frames carry a display name and a connection state, deliberately, because
// presence is CONNECTION-scoped while a role is campaign-scoped. Putting the
// role in a presence frame would make the answer go stale the moment somebody
// was promoted without reconnecting — which is precisely what J4 made possible.
//
// So the console reads the source of truth (spec §3.1) instead.
func TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo(t *testing.T) {
	d, _ := openTemp(t)
	if _, _, err := d.CreateInvite("Zoe", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	_, dmID, err := d.CreateInvite("Ari", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	_, goneID, err := d.CreateInvite("Mal", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Revoke(goneID); err != nil {
		t.Fatal(err)
	}

	list, err := d.List()
	if err != nil {
		t.Fatal(err)
	}

	// REVOKED PARTICIPANTS ARE OMITTED. They cannot act and cannot connect, so
	// a console listing them offers the DM promote buttons for people who are
	// gone — and worse, makes a revoked name look like somebody still at the
	// table.
	if len(list) != 2 {
		t.Fatalf("got %d participants, want 2 (the revoked one must not be listed): %+v", len(list), list)
	}
	// Sorted by display name, so the list does not reshuffle under the DM's
	// cursor between renders.
	if list[0].Name != "Ari" || list[1].Name != "Zoe" {
		t.Fatalf("want Ari then Zoe, got %q then %q", list[0].Name, list[1].Name)
	}
	if list[0].ID != dmID {
		t.Fatalf("participant id = %q, want %q", list[0].ID, dmID)
	}
	if list[0].Role != identity.RoleDM || list[1].Role != identity.RoleSpectator {
		t.Fatalf("roles = %q, %q; want dm, spectator", list[0].Role, list[1].Role)
	}
}

func TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder(t *testing.T) {
	// DUPLICATE DISPLAY NAMES ARE THIS FEATURE'S ORDINARY TRAFFIC, not an edge
	// case: a shared link lets anybody type any name, and two strangers both
	// answering "Kim" is a Tuesday. Without the id tie-break, SQLite may return
	// them in either order between reads, and the roster reshuffles under the
	// DM's cursor — the exact thing sorting is there to prevent.
	//
	// The names above are all distinct, so `ORDER BY display_name, id` could
	// lose its second column and nothing would notice. Nothing else covers it
	// either: the Go mutation gate cannot mutate SQL text.
	d, _ := openTemp(t)
	var ids []string
	for range 4 {
		_, id, err := d.CreateInvite("Kim", identity.RoleSpectator)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)

	// Read TWICE, and require both the documented order and that it is stable:
	// a single read could match by luck.
	for attempt := range 2 {
		list, err := d.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 4 {
			t.Fatalf("got %d participants, want 4", len(list))
		}
		for i, want := range ids {
			if list[i].ID != want {
				t.Fatalf("attempt %d: position %d is %q, want %q — four people called Kim "+
					"must come back in a fixed order", attempt, i, list[i].ID, want)
			}
		}
	}
}

func TestListingRefusesACorruptRowRatherThanInventingARole(t *testing.T) {
	// Same posture as Lookup: an unparseable role must not become a
	// participant whose authorization nobody can account for. Unreachable
	// through this package's own writers, which is why it earns a test — a
	// hand-edited database must fail loudly, not quietly show somebody as
	// whatever Role("") happens to mean downstream.
	d, path := openTemp(t)
	if _, _, err := d.CreateInvite("Zoe", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE participants SET role = 'overlord'`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := d.List(); err == nil {
		t.Fatal("a stored role that is not a role must be an error, not a listed participant")
	}

	// A second case used to follow, feeding unparseable JSON to
	// participants.controls — the OTHER column List decoded. That column was
	// deleted on 2026-08-24 (it recorded control a second time and granted
	// nothing), so role is the only stored field List can now fail to parse.
	// The repaired row proves the refusal above was about the role and not
	// about the row merely existing.
	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE participants SET role = 'spectator'`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if list, err := d.List(); err != nil || len(list) != 1 {
		t.Fatalf("List after repairing the role = %d participants, %v; want 1, nil", len(list), err)
	}
}

func TestListingRefusesWhenTheTableCannotBeRead(t *testing.T) {
	// An OPERATIONAL failure, distinct from a corrupt row: the storage cannot
	// answer at all. It must surface rather than come back as an empty table,
	// because "nobody is here" is a perfectly ordinary answer and a DM reading
	// it would have no reason to doubt it.
	d, path := openTemp(t)
	if _, _, err := d.CreateInvite("Zoe", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`ALTER TABLE participants RENAME TO participants_elsewhere`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := d.List()
	if err == nil {
		t.Fatalf("unreadable storage must be an error, got %d participant(s)", len(got))
	}
	if got != nil {
		t.Fatal("an error must not also return a list somebody might render")
	}
}

// TestACampaignPredatingTheAdmissionBudgetStillWorks is the migration this
// package had no mechanism for.
//
// The schema comment above join_access records WHY it is a separate table: on
// a campaign that already has `participants`, CREATE TABLE IF NOT EXISTS is a
// no-op, so a new COLUMN there would never appear. A new TABLE dodges that
// because it exists nowhere yet. Adding admitted/admit_limit to join_access
// walks straight back into it — every campaign whose door was ever touched
// already HAS that table, so IF NOT EXISTS skips it and the columns never
// arrive. The failure is silent and total: every join errors on a missing
// column, on real campaigns only, and never on a fresh test database.
//
// So this builds the OLD shape by hand and opens it.
func TestACampaignPredatingTheAdmissionBudgetStillWorks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// The join_access schema EXACTLY as it shipped before the budget.
	if _, err := raw.Exec(`
CREATE TABLE participants (
  id           TEXT PRIMARY KEY,
  display_name TEXT,
  role         TEXT,
  controls     TEXT,
  token_hash   BLOB UNIQUE,
  revoked      INTEGER DEFAULT 0
);
CREATE TABLE join_access (
  id     INTEGER PRIMARY KEY CHECK (id = 1),
  secret TEXT NOT NULL,
  open   INTEGER NOT NULL DEFAULT 0
);
INSERT INTO join_access (id, secret, open) VALUES (1, 'old-secret', 1);`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	db, err := identity.Open(path)
	if err != nil {
		t.Fatalf("opening a campaign that predates the budget: %v", err)
	}
	defer db.Close()

	// It must ADMIT, not merely open: a migration that adds the columns with a
	// budget of zero would satisfy "no error" while locking every existing
	// campaign out of its own join link.
	admitted, err := db.JoinAdmits("old-secret")
	if err != nil {
		t.Fatalf("JoinAdmits on a migrated campaign: %v", err)
	}
	if !admitted {
		t.Fatal("a campaign that predates the budget must still admit through its open door — " +
			"migrating it to a budget of zero would shut a door its DM had left open")
	}
}

// TestOnlyOneJoinerTakesTheLastSlot is the whole point of a cap.
//
// A budget two concurrent joiners can both pass is not a budget, and this is
// the classic shape: read "admitted < limit", both see room, both proceed. The
// read in JoinAdmits is a FAST PATH ONLY — the increment re-states the whole
// condition in its WHERE, so SQLite serialises the two writes and the loser
// matches no row.
//
// Deliberately more goroutines than slots, released together, so the race is
// contended rather than hypothetical.
func TestOnlyOneJoinerTakesTheLastSlot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "race.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 1); err != nil { // ONE slot
		t.Fatal(err)
	}

	// ROUNDS, and the number is measured rather than chosen to look thorough.
	// Review deleted `AND admitted < admit_limit` from the UPDATE — the single
	// clause this test exists for — and a ONE-ROUND version passed five times
	// running: detection was 9 in 40 at one round, 39 in 40 at ten, 40 in 40 at
	// thirty. A cap that lets six racers through a budget of one would have
	// shipped green on four CI runs in five.
	const (
		racers = 16
		rounds = 30
	)
	for round := range rounds {
		if err := d.SetJoinOpen(true, 1); err != nil { // ONE slot, fresh each round
			t.Fatal(err)
		}
		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			granted int
		)
		start := make(chan struct{})
		for range racers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ok, err := d.JoinAdmits(secret)
				if err != nil {
					return
				}
				if ok {
					mu.Lock()
					granted++
					mu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()

		if granted != 1 {
			t.Fatalf("round %d: %d of %d racers were admitted against a budget of 1 — the "+
				"cap is not atomic, so a leaked link is bounded only by how slowly people "+
				"click", round, granted, racers)
		}
	}
}

// TestABudgetIsPerOpeningNotPerCampaign pins what "open the door again" means.
//
// A DM who opens the door twice means twice: the second opening is a fresh
// decision about a fresh set of people, not the remainder of an old one. If
// the count carried over, a campaign would silently run out of admissions
// forever, and the only cure would be a database edit.
func TestABudgetIsPerOpeningNotPerCampaign(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}

	if err := d.SetJoinOpen(true, 1); err != nil {
		t.Fatal(err)
	}
	if ok, _ := d.JoinAdmits(secret); !ok {
		t.Fatal("the first joiner must get the only slot")
	}
	if ok, _ := d.JoinAdmits(secret); ok {
		t.Fatal("a spent budget must refuse")
	}

	if err := d.SetJoinOpen(true, 1); err != nil { // opened again
		t.Fatal(err)
	}
	if ok, _ := d.JoinAdmits(secret); !ok {
		t.Fatal("re-opening the door must restore the budget — otherwise a campaign runs " +
			"out of admissions permanently and only a database edit brings it back")
	}
}

// TestAClosedDoorSpendsNothing keeps spec §2's inertness true for the new
// column too: a refused anonymous request must not write, and "admitted" is
// now a thing a refusal could plausibly touch.
func TestAClosedDoorSpendsNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shut.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 5); err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(false, 5); err != nil {
		t.Fatal(err)
	}

	if ok, _ := d.JoinAdmits(secret); ok {
		t.Fatal("a closed door must admit nobody")
	}
	if ok, _ := d.JoinAdmits("wrong"); ok {
		t.Fatal("a wrong secret must admit nobody")
	}

	// READ THE COUNTER, with NO SetJoinOpen in between.
	//
	// The first version of this re-opened the door and then counted five
	// successful admissions — but SetJoinOpen resets `admitted` to 0
	// unconditionally, so it wiped the very evidence it was about to read.
	// Review injected "every refusal increments admitted" and all three suites
	// stayed green. The shipped failure: a stranger POSTs a wrong secret eight
	// times, the door is exhausted, the whole table is locked out — and each
	// refusal takes SQLite's write lock on the file internal/store appends
	// events to, which is the inertness §2 rests on.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var admitted int
	if err := raw.QueryRow(`SELECT admitted FROM join_access WHERE id = 1`).Scan(&admitted); err != nil {
		t.Fatal(err)
	}
	if admitted != 0 {
		t.Fatalf("two refusals spent %d admissions — a stranger can exhaust the door "+
			"without ever getting through it, locking the table out", admitted)
	}
}

// TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath is the twin of the
// JoinAllows test three functions up, pointed at the function /join actually
// calls. Review deleted the `secret == ""` guard from JoinAdmits and every
// suite stayed green: the existing test exercises JoinAllows, which handleJoin
// no longer uses. ConstantTimeCompare("", "") returns 1 and a request body
// omitting the field decodes to "", so the degenerate row admits the world.
func TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.JoinSecret(); err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 5); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`UPDATE join_access SET secret = '' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	if ok, _ := d.JoinAdmits(""); ok {
		t.Fatal("an empty stored secret admitted an empty candidate — the degenerate row " +
			"admits the world, because ConstantTimeCompare(\"\", \"\") is 1")
	}
}

// TestRotatingAfterASpentBudgetGivesAWorkingLink is the leak remedy, and
// without it the remedy is worse than the leak.
//
// Rotating is what §2 tells a DM to do when a link escapes. Review measured
// the state that left: open with a budget of 2, admit 2, rotate — and the new
// secret admits NOBODY, because rotate replaced the secret and left `admitted`
// spent. `vtt join-link show` says "door: open"; every legitimate player gets
// the byte-identical stranger's 403; nothing on either end says why. That is
// precisely the "cannot be debugged from either end" failure the spec argues
// against for a zero default, reached by a different road.
//
// A new secret is a NEW OPENING. Nobody holding it has spent anything.
func TestRotatingAfterASpentBudgetGivesAWorkingLink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rotate.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 2); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if ok, err := d.JoinAdmits(secret); !ok {
			t.Fatalf("admission %d of 2 refused (%v)", i+1, err)
		}
	}

	fresh, err := d.RotateJoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := d.JoinAdmits(fresh); !ok {
		t.Fatalf("the freshly rotated link admits nobody (%v) — the documented cure for a "+
			"leak hands the DM a door that reads open and refuses everyone", err)
	}
	// And the OLD secret is still locked out, which is the point of rotating.
	if ok, _ := d.JoinAdmits(secret); ok {
		t.Fatal("the old secret still admits — rotating did not close the leak")
	}
}

// TestMigrationSurvivesConcurrentFirstOpens covers the one run where it
// matters: the first open after an upgrade.
//
// The scan and the ALTERs are separate statements, so two processes opening the
// same campaign together both see the columns missing and both add them. Review
// measured 35 failures in 40 trials with four concurrent opens, dying on
// `duplicate column name` — the server refusing to start, or a raw SQL error in
// the DM's terminal, on the one run nobody will connect to the upgrade.
func TestMigrationSurvivesConcurrentFirstOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
CREATE TABLE participants (
  id TEXT PRIMARY KEY, display_name TEXT, role TEXT,
  controls TEXT, token_hash BLOB UNIQUE, revoked INTEGER DEFAULT 0
);
CREATE TABLE join_access (
  id INTEGER PRIMARY KEY CHECK (id = 1), secret TEXT NOT NULL,
  open INTEGER NOT NULL DEFAULT 0
);
INSERT INTO join_access (id, secret, open) VALUES (1, 'old-secret', 1);`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	const openers = 4
	var wg sync.WaitGroup
	errs := make(chan error, openers)
	start := make(chan struct{})
	for range openers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			db, err := identity.Open(path)
			if err != nil {
				errs <- err
				return
			}
			db.Close()
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a concurrent first open failed: %v", err)
	}
}

// TestUpgradingACampaignRemovesTheControlColumnAndKeepsThePeople is the
// deletion of participants.controls, on the databases that already carry it.
//
// The column recorded control a SECOND time and granted nothing: no updater
// ever existed, no grant was ever emitted from it, and the one consumer echoed
// it at /api/me. Leaving it behind would leave the second writer's slot open
// for somebody to start reading again, which is how the concept ends up with
// two authorities a third time.
//
// THE PEOPLE ARE THE OTHER HALF OF THE ASSERTION, and it is not decoration: a
// migration that dropped the whole participants table would satisfy "the
// column is gone" perfectly, and would log out every campaign in existence.
// The row below is written in the pre-deletion shape, by hand, and must still
// resolve by token and by id afterwards.
func TestUpgradingACampaignRemovesTheControlColumnAndKeepsThePeople(t *testing.T) {
	path := filepath.Join(t.TempDir(), "with-controls.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// The participants shape EXACTLY as it shipped while control was recorded
	// twice, carrying a row whose controls list names an actor no grant in any
	// log ever gave them — the lie this deletion is about.
	const token = "an-existing-token"
	hash := sha256.Sum256([]byte(token))
	if _, err := raw.Exec(`
CREATE TABLE participants (
  id           TEXT PRIMARY KEY,
  display_name TEXT,
  role         TEXT,
  controls     TEXT,
  token_hash   BLOB UNIQUE,
  revoked      INTEGER DEFAULT 0
);`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(
		`INSERT INTO participants (id, display_name, role, controls, token_hash, revoked)
		 VALUES ('p-hollis', 'Hollis', 'player', '["act-hollis"]', ?, 0)`, hash[:]); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	d, err := identity.Open(path)
	if err != nil {
		t.Fatalf("opening a campaign that predates the deletion: %v", err)
	}
	defer d.Close()

	p, err := d.Verify(token)
	if err != nil {
		t.Fatalf("an existing participant stopped resolving after the upgrade: %v", err)
	}
	if p.ID != "p-hollis" || p.Name != "Hollis" || p.Role != identity.RolePlayer {
		t.Errorf("the upgrade changed who this is: %+v", p)
	}
	if _, err := d.Lookup("p-hollis"); err != nil {
		t.Errorf("the upgrade lost the participant by id: %v", err)
	}
	list, err := d.List()
	if err != nil || len(list) != 1 {
		t.Errorf("List after the upgrade = %d participants, %v; want 1, nil", len(list), err)
	}

	// And the column itself is gone. Read through a SEPARATE handle so this
	// asks the FILE what shape it has, rather than asking the code that just
	// claimed to change it.
	after, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	rows, err := after.Query(`PRAGMA table_info(participants)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid, name, typ, notnull, dflt, pk any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, fmt.Sprint(name))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(cols, "controls") {
		t.Errorf("participants still carries a controls column after the upgrade (%v) — "+
			"the second record of control outlived the code that read it", cols)
	}
	// The columns that DO carry something are still there, so "gone" cannot be
	// satisfied by a table rebuilt into a different shape.
	for _, want := range []string{"id", "display_name", "role", "token_hash", "revoked"} {
		if !slices.Contains(cols, want) {
			t.Errorf("the upgrade dropped %q as well: %v", want, cols)
		}
	}
}

// TestAReadOnlyCampaignStillCarryingTheControlColumnWillNotOpen pins the price
// of the deletion, deliberately, as a test rather than a footnote.
//
// TestAnAlreadyMigratedReadOnlyCampaignStillOpens says an archived campaign
// stays readable, and it still passes — its fixture is created by THIS code and
// so has no control column. Every campaign created before today does, which
// makes migrationPending answer yes, which takes migrate to BEGIN IMMEDIATE,
// which read-only media cannot give. So the archive opens once it has been
// opened once somewhere writable, and not before.
//
// Written down here because the alternative — leaving a dead column on old
// databases so nothing has to be written — is the option this task explicitly
// rejected, and a cost nobody recorded is a cost the next person reads as a bug.
func TestAReadOnlyCampaignStillCarryingTheControlColumnWillNotOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archived-with-controls.db")

	// CURRENT in every other respect — created by this code, then given back
	// the one column the deletion removes. So the refusal below can only be
	// about `controls`, not about some other missing piece of schema.
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.CreateInvite("Archivist", identity.RoleDM); err != nil {
		t.Fatal(err)
	}
	d.Close()

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`ALTER TABLE participants ADD COLUMN controls TEXT`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	// The DIRECTORY too: SQLite needs to create -wal/-shm beside the file, so a
	// writable directory leaves a path where the write still succeeds.
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o755)
		_ = os.Chmod(path, 0o644)
	})
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("running with rights that make a read-only file writable")
	}

	again, err := identity.Open(path)
	if err == nil {
		again.Close()
		t.Fatal("a read-only campaign that still carries participants.controls opened — " +
			"either the migration silently skipped it, leaving the second record of " +
			"control in place, or it wrote to media that cannot be written")
	}
}

// TestMigratingTwiceIsNotAnError pins idempotency. ALTER TABLE ADD COLUMN is an
// error, not a no-op, on a column that is already there — so a regression in
// the shape scan surfaces as `duplicate column name` on the SECOND open of
// every real campaign, which the migration test alone would never see.
func TestMigratingTwiceIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.db")
	for i := range 3 {
		d, err := identity.Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i+1, err)
		}
		d.Close()
	}
}

// TestJoinBudgetReportsWhatHasBeenSpent is what the DM console and `vtt
// join-link show` render. It had ZERO coverage in this package when it landed:
// the CLI test exercised it through a subprocess, which proves the wiring and
// not the function.
func TestJoinBudgetReportsWhatHasBeenSpent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// A campaign whose door has never been touched has no row at all, and must
	// answer rather than error — the console polls this before anything exists.
	admitted, limit, err := d.JoinBudget()
	if err != nil {
		t.Fatalf("a never-touched campaign errored: %v", err)
	}
	if admitted != 0 || limit != 0 {
		t.Fatalf("a never-touched campaign reports %d/%d, want 0/0", admitted, limit)
	}

	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 3); err != nil {
		t.Fatal(err)
	}
	if admitted, limit, err = d.JoinBudget(); err != nil || admitted != 0 || limit != 3 {
		t.Fatalf("a freshly opened door reports %d/%d (%v), want 0/3", admitted, limit, err)
	}

	if ok, err := d.JoinAdmits(secret); !ok {
		t.Fatal(err)
	}
	if admitted, limit, err = d.JoinBudget(); err != nil || admitted != 1 || limit != 3 {
		t.Fatalf("after one joiner it reports %d/%d (%v), want 1/3 — the count the DM "+
			"reads does not follow the door", admitted, limit, err)
	}

	// Closing resets the spend, so a shut door never reports a stale number.
	if err := d.SetJoinOpen(false, 0); err != nil {
		t.Fatal(err)
	}
	if admitted, _, err = d.JoinBudget(); err != nil || admitted != 0 {
		t.Fatalf("a closed door still reports %d spent (%v)", admitted, err)
	}
}

// TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting covers the error
// arms, and the direction matters more than the coverage: every one of these
// must fail CLOSED. A database that cannot answer must never be able to open a
// door, and JoinAdmits returning (true, err) anywhere would admit a stranger on
// a broken campaign.
func TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 5); err != nil {
		t.Fatal(err)
	}
	// The door is OPEN and the secret is RIGHT — so anything refusing below is
	// refusing because the database is gone, not because the request was bad.
	d.Close()

	if ok, err := d.JoinAdmits(secret); ok || err == nil {
		t.Fatalf("JoinAdmits on a closed database returned (%v, %v) — it must report the "+
			"failure and admit nobody", ok, err)
	}
	if _, _, err := d.JoinBudget(); err == nil {
		t.Fatal("JoinBudget on a closed database reported success")
	}
	if ok, err := d.JoinAllows(secret); ok || err == nil {
		t.Fatalf("JoinAllows on a closed database returned (%v, %v)", ok, err)
	}
	if err := d.SetJoinOpen(true, 5); err == nil {
		t.Fatal("SetJoinOpen on a closed database reported success")
	}
	if _, err := d.RotateJoinSecret(); err == nil {
		t.Fatal("RotateJoinSecret on a closed database reported success")
	}
}

// TestOpeningAnUnreadableCampaignFailsLoudly covers migrate's error arms. A
// campaign that cannot be migrated must not come up half-migrated: every join
// would then fail on a missing column, which reads as "the link is broken"
// rather than "this database could not be upgraded".
func TestOpeningAnUnreadableCampaignFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notadb.db")
	// Not SQLite at all. Open must fail somewhere in schema-or-migrate and say
	// so, rather than returning a handle nothing works against.
	if err := os.WriteFile(path, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d, err := identity.Open(path); err == nil {
		d.Close()
		t.Fatal("opening a file that is not a database succeeded")
	}
}

// TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying covers migrate's
// failure arms with a scenario that actually happens: a campaign file on
// read-only media, or one whose permissions were tightened.
//
// The DIRECTION is the point. A migration that cannot write must not return a
// usable handle, because every join would then fail on a missing column and
// present as "the join link is broken" rather than "this campaign could not be
// upgraded". The BEGIN IMMEDIATE wrapper also means a partial ALTER cannot be
// left behind for the next open to trip over.
func TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readonly.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// The pre-budget shape, so a migration is genuinely required.
	if _, err := raw.Exec(`
CREATE TABLE participants (
  id TEXT PRIMARY KEY, display_name TEXT, role TEXT,
  controls TEXT, token_hash BLOB UNIQUE, revoked INTEGER DEFAULT 0
);
CREATE TABLE join_access (
  id INTEGER PRIMARY KEY CHECK (id = 1), secret TEXT NOT NULL,
  open INTEGER NOT NULL DEFAULT 0
);
INSERT INTO join_access (id, secret, open) VALUES (1, 'old-secret', 1);`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	// The DIRECTORY too: SQLite needs to create -wal/-shm beside the file, so a
	// writable directory leaves a path where the write still succeeds.
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o755)
		_ = os.Chmod(path, 0o644)
	})
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("running with rights that make a read-only file writable")
	}

	d, err := identity.Open(path)
	if err == nil {
		d.Close()
		t.Fatal("opening a campaign that could not be migrated succeeded — every join " +
			"against it would fail on a missing column and read as a broken link")
	}
}

// TestOpeningACurrentCampaignTakesNoWriteLock is the property, tested through
// the consequence rather than by inspecting locks.
//
// migrate runs on EVERY Open, and its first draft wrapped everything in BEGIN
// IMMEDIATE unconditionally — so opening an already-current campaign took
// SQLite's write lock on a file internal/store writes to inside a transaction
// on every event append. ensureJoinRow's own comment records what that costs:
// with another handle holding a write txn, the blocked caller waits the full
// busy_timeout(5000) and then fails SQLITE_BUSY.
//
// So: hold a write transaction open, and assert Open still returns promptly.
func TestOpeningACurrentCampaignTakesNoWriteLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.db")
	d, err := identity.Open(path) // migrates once
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	blocker, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	tx, err := blocker.Begin()
	if err != nil {
		t.Fatal(err)
	}
	// A genuine WRITE, so the transaction actually holds the write lock.
	if _, err := tx.Exec(
		`INSERT INTO join_access (id, secret, open) VALUES (1, 'held', 0)
		 ON CONFLICT(id) DO UPDATE SET secret = excluded.secret`); err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	done := make(chan error, 1)
	go func() {
		second, err := identity.Open(path)
		if err == nil {
			second.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("opening a current campaign behind a write transaction failed: %v — "+
				"migrate is taking the write lock when it has nothing to write", err)
		}
	case <-time.After(2 * time.Second):
		// busy_timeout is 5s, so 2s of silence is already the wrong answer.
		t.Fatal("opening a current campaign BLOCKED behind another handle's write " +
			"transaction — migrate takes the write lock on every open, which is a lock a " +
			"read-only user has no business taking")
	}
}

// TestAnAlreadyMigratedReadOnlyCampaignStillOpens is the other half. A campaign
// on read-only media, or one whose permissions were tightened, must still be
// readable — `vtt state dump` against an archived campaign is the case. The
// unconditional-transaction draft made this impossible: nothing needed writing
// and it took the write lock anyway.
func TestAnAlreadyMigratedReadOnlyCampaignStillOpens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archived.db")
	d, err := identity.Open(path) // creates and migrates
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.JoinSecret(); err != nil {
		t.Fatal(err)
	}
	d.Close()

	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("running with rights that make a read-only file writable")
	}

	again, err := identity.Open(path)
	if err != nil {
		t.Fatalf("an already-migrated read-only campaign would not open: %v — migrate "+
			"writes even when the schema is current", err)
	}
	defer again.Close()
	if !again.JoinOpen() == false {
		t.Fatal("unreachable")
	}
}

// TestJoinAdmitsOnACampaignWithNoDoorRowRefusesWithoutCreatingOne is the
// never-touched case, and it is a security property rather than a coverage
// one: the row does not exist until somebody opens the door or reads the link,
// and an anonymous request must be answered from that absence WITHOUT minting
// anything. Minting here is what the 2026-08-09 amendment was written about.
func TestJoinAdmitsOnACampaignWithNoDoorRowRefusesWithoutCreatingOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "untouched.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if ok, err := d.JoinAdmits("anything"); ok || err != nil {
		t.Fatalf("JoinAdmits on a campaign with no door row returned (%v, %v), want (false, nil)", ok, err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRow(`SELECT count(*) FROM join_access`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("an anonymous refusal created %d door row(s) — the closed door is not "+
			"inert, and §2's case against rate limiting depends on it being inert", n)
	}
}

// TestCreateInviteRefusesARoleThatIsNotOne keeps the four roles the complete
// set. A caller that can invent a role can invent one authz has no cell for,
// and every commandRoles lookup for it would answer "not permitted" — which
// looks like a permissions bug rather than a bad invite.
func TestCreateInviteRefusesARoleThatIsNotOne(t *testing.T) {
	d, _ := openTemp(t)
	if _, _, err := d.CreateInvite("Nobody", identity.Role("overlord")); err == nil {
		t.Fatal("CreateInvite accepted a role that is not one of the four")
	}
}

// TestTheIdentityStoreReportsFailuresRatherThanPretending covers the write
// paths' error arms. Same direction as the join path's: a database that cannot
// answer must say so, never quietly succeed — a silent SetRole would leave a
// promotion the DM believes happened and authz does not.
func TestTheIdentityStoreReportsFailuresRatherThanPretending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, id, err := d.CreateInvite("Ada", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	if err := d.SetRole(id, identity.RoleSpectator); err == nil {
		t.Fatal("SetRole on a closed database reported success")
	}
	if err := d.Revoke(id); err == nil {
		t.Fatal("Revoke on a closed database reported success")
	}
	if _, err := d.JoinSecret(); err == nil {
		t.Fatal("JoinSecret on a closed database reported success")
	}
	if _, err := d.List(); err == nil {
		t.Fatal("List on a closed database reported success")
	}
	if _, _, err := d.CreateInvite("Bo", identity.RoleSpectator); err == nil {
		t.Fatal("CreateInvite on a closed database reported success")
	}
	if _, err := d.Lookup(id); err == nil {
		t.Fatal("Lookup on a closed database reported success")
	}
}

// TestADoorOpenedWithNoStatedBudgetStillAdmits pins the coercion at the layer
// that owns it. The gateway has the same test over the wire, but mutation runs
// PER PACKAGE, so a gateway test cannot kill an identity mutant — and the
// mutation gate duly found `admitLimit <= 0` mutated to `<` surviving here.
//
// A door opened with an explicit 0, or with the absent wire field that decodes
// to one, must not admit nobody: the DM sees "open", every joiner sees the same
// 403 a stranger sees, and nothing on either side distinguishes them.
func TestADoorOpenedWithNoStatedBudgetStillAdmits(t *testing.T) {
	d, _ := openTemp(t)
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 0); err != nil {
		t.Fatal(err)
	}

	if ok, err := d.JoinAdmits(secret); !ok {
		t.Fatalf("a door opened with a budget of 0 admitted nobody (%v) — 0 means "+
			"'unstated', and reading it literally opens a door no one can get through", err)
	}
	_, limit, err := d.JoinBudget()
	if err != nil {
		t.Fatal(err)
	}
	if limit != identity.DefaultAdmitLimit {
		t.Fatalf("an unstated budget became %d, want the default of %d", limit, identity.DefaultAdmitLimit)
	}
}
