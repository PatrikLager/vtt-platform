package identity_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer, []string{"act-lera"})
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
	if len(p.Controls) != 1 || p.Controls[0] != "act-lera" {
		t.Errorf("Controls: got %v, want [act-lera]", p.Controls)
	}
}

// TestTokenNotRecoverableFromDB proves the raw token is not stored anywhere
// retrievable: it opens a second, independent SQLite handle on the same
// file and reads the persisted token_hash directly, asserting it neither
// equals the raw token bytes nor anything other than sha256(token).
func TestTokenNotRecoverableFromDB(t *testing.T) {
	d, path := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
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
	if _, _, err := d.CreateInvite("Lera", identity.RolePlayer, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify("this-is-not-a-real-token"); err == nil {
		t.Fatal("want error for a token that was never issued")
	}
}

func TestRevokedTokenRejectedAfterRevoke(t *testing.T) {
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
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
	token1, id1, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}
	token2, id2, err := d.CreateInvite("Ursus", identity.RoleAgent, nil)
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
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer, []string{"act-lera"})
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
	if err := d.SetJoinOpen(true); err != nil {
		t.Fatal(err)
	}
	if !d.JoinOpen() {
		t.Fatal("opening the door must take effect")
	}
	if err := d.SetJoinOpen(false); err != nil {
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
	if err := d.SetJoinOpen(true); err != nil {
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
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
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
	if err := d.SetJoinOpen(true); err == nil {
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
		if err := d.SetJoinOpen(open); err != nil {
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
// branch, which nothing reached: every SetJoinOpen(true) in this file ran on a
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
	if err := d.SetJoinOpen(true); err != nil {
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
	if err := a.SetJoinOpen(true); err != nil {
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
	if err := b.SetJoinOpen(true); err != nil {
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
	token, id, err := d.CreateInvite("Kim", identity.RoleSpectator, nil)
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
	_, kim, err := d.CreateInvite("Kim", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}
	adaToken, _, err := d.CreateInvite("Ada", identity.RoleSpectator, nil)
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
	_, id, err := d.CreateInvite("Kim", identity.RoleSpectator, nil)
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
	_, id, err := d.CreateInvite("Kim", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatalf("re-promoting to the same role must be a no-op, not an error: %v", err)
	}
}

func TestSetRoleDoesNotDisturbTheCredential(t *testing.T) {
	// The token and the controls belong to the person, not the role. A
	// promotion that rewrote either would silently log them out or strip what
	// they hold.
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Kim", identity.RoleSpectator, []string{"act-warden"})
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
	if len(p.Controls) != 1 || p.Controls[0] != "act-warden" {
		t.Fatalf("promotion changed controls: %v", p.Controls)
	}
}

func TestSetRoleOnARevokedParticipantStaysRevoked(t *testing.T) {
	// Promotion must not be a way back in for somebody who was thrown out.
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Mallory", identity.RoleSpectator, nil)
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
	_, id, err := d.CreateInvite("Kim", identity.RoleSpectator, nil)
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
	_, id, err := d.CreateInvite("Mallory", identity.RolePlayer, nil)
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
	// Not just the role: Authorize reads ID for ownership checks and Controls
	// is part of the identity story, so a partial lookup would silently change
	// what authorization sees.
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Kim", identity.RolePlayer, []string{"act-warden"})
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
	if len(p.Controls) != 1 || p.Controls[0] != "act-warden" {
		t.Fatalf("lookup lost controls: %v", p.Controls)
	}
}

func TestLookupRefusesACorruptRow(t *testing.T) {
	// A row whose role or controls cannot be parsed must NOT resolve. These
	// are unreachable through this package's own writers, which is exactly why
	// they are worth asserting: if a row ever became malformed — a hand-edited
	// database, a botched migration, a future writer with a bug — the failure
	// must be a refusal, not a participant with a silently wrong authorization
	// level.
	for _, tc := range []struct{ name, role, controls string }{
		{"unparseable role", "superuser", "[]"},
		{"unparseable controls", "player", "not json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, path := openTemp(t)
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			raw, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := raw.Exec(
				`INSERT INTO participants (id, display_name, role, controls, token_hash, revoked)
				 VALUES ('p-bad', 'Bad', ?, ?, X'01', 0)`, tc.role, tc.controls); err != nil {
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
		})
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
		if err := d.SetJoinOpen(c.open); err != nil {
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
	if err := d.SetJoinOpen(true); err != nil {
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
	if _, _, err := d.CreateInvite("Zoe", identity.RoleSpectator, nil); err != nil {
		t.Fatal(err)
	}
	_, dmID, err := d.CreateInvite("Ari", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, goneID, err := d.CreateInvite("Mal", identity.RolePlayer, nil)
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
		_, id, err := d.CreateInvite("Kim", identity.RoleSpectator, nil)
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
	if _, _, err := d.CreateInvite("Zoe", identity.RoleSpectator, nil); err != nil {
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

	// The same posture for the OTHER column this decodes. Controls is JSON in
	// a text column, so it can be malformed independently of the role, and a
	// participant listed with silently-empty controls would read to a DM as
	// somebody holding nothing.
	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE participants SET role = 'spectator', controls = '{not json'`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.List(); err == nil {
		t.Fatal("controls that are not JSON must be an error, not a participant holding nothing")
	}
}

func TestListingRefusesWhenTheTableCannotBeRead(t *testing.T) {
	// An OPERATIONAL failure, distinct from a corrupt row: the storage cannot
	// answer at all. It must surface rather than come back as an empty table,
	// because "nobody is here" is a perfectly ordinary answer and a DM reading
	// it would have no reason to doubt it.
	d, path := openTemp(t)
	if _, _, err := d.CreateInvite("Zoe", identity.RoleSpectator, nil); err != nil {
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
