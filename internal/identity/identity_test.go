package identity_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
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

// VTT-075
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

// Reads token_hash through a second raw handle on the same file.
// VTT-041
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

// VTT-074
func TestVerifyRejectsWrongToken(t *testing.T) {
	d, _ := openTemp(t)
	if _, _, err := d.CreateInvite("Lera", identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify("this-is-not-a-real-token"); err == nil {
		t.Fatal("want error for a token that was never issued")
	}
}

// VTT-074
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

// VTT-066
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

// VTT-024
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

func TestVerifyUsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("identity.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "subtle.ConstantTimeCompare") {
		t.Fatal("Verify must confirm the token hash match with subtle.ConstantTimeCompare")
	}
}

// VTT-076
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

	token, _, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err != nil {
		t.Fatal(err)
	}

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

	if p, err := d.Verify(token); err != nil || p.Name != "Lera" {
		t.Fatalf("Verify after store use: p=%v err=%v", p, err)
	}
}

// Builds the pre-door schema by hand, so Open really creates join_access.
// VTT-015
func TestJoinIsClosedOnAnExistingCampaign(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the old shape: participants only, no join_access.
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
	// Keep this query: without it the test passes whether the table was created or not.
	if _, err := d.JoinSecret(); err != nil {
		t.Fatalf("the migration did not reach an existing campaign: %v", err)
	}
	if d.JoinOpen() {
		t.Fatal("minting the secret on an upgraded campaign must not open the door")
	}
}

// VTT-015
func TestJoinIsClosedOnAFreshCampaign(t *testing.T) {
	d, _ := openTemp(t)
	if d.JoinOpen() {
		t.Fatal("a new campaign must come up with the door closed")
	}
}

// VTT-064
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

// VTT-064
func TestTheDoorSurvivesAReopen(t *testing.T) {
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

// VTT-040
func TestTheJoinSecretIsStableUntilRotated(t *testing.T) {
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

// VTT-019
func TestRotatingTheSecretInvalidatesTheOldLink(t *testing.T) {
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

// VTT-020
func TestRotatingTheSecretLeavesParticipantsAlone(t *testing.T) {
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

// VTT-016
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

// VTT-048 VTT-049 VTT-070
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

// VTT-043
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

// VTT-065
func TestTheDoorOpensOnACampaignThatAlreadyHasALink(t *testing.T) {
	d, _ := openTemp(t)
	if _, err := d.JoinSecret(); err != nil { // mints the row, closed
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	// Keep this test: it holds SetJoinOpen's CONFLICT branch, SQL text the
	// mutation gate cannot mutate.
	if !d.JoinOpen() {
		t.Fatal("opening the door on a campaign that already has a link must work — " +
			"reading the link first is the ordinary order, not an edge case")
	}
}

// VTT-065
func TestOpeningTheDoorFirstStillMintsARealSecret(t *testing.T) {
	a, _ := openTemp(t)
	if err := a.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	secret, err := a.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	// Keep this test: it holds the secret SetJoinOpen's INSERT branch mints, SQL
	// text the mutation gate cannot mutate.
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

// VTT-029
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

// VTT-037
func TestSetRoleLeavesEVERYONEElseAlone(t *testing.T) {
	// Keep this test: a missing WHERE promotes the whole table, and the mutation
	// gate cannot see SQL.
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

// VTT-066
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

// VTT-067
func TestSetRoleOnSomeoneWhoDoesNotExistIsAnError(t *testing.T) {
	d, _ := openTemp(t)
	if err := d.SetRole("p-nobody", identity.RolePlayer); err == nil {
		t.Fatal("promoting an unknown participant must report it, not succeed quietly")
	}
}

// VTT-068
func TestSetRoleToTheSameRoleIsFine(t *testing.T) {
	d, _ := openTemp(t)
	_, id, err := d.CreateInvite("Kim", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatalf("re-promoting to the same role must be a no-op, not an error: %v", err)
	}
}

// VTT-029
func TestSetRoleDoesNotDisturbTheCredential(t *testing.T) {
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

// VTT-030
func TestSetRoleOnARevokedParticipantStaysRevoked(t *testing.T) {
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

// VTT-029
func TestLookupReflectsAPromotionImmediately(t *testing.T) {
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

// VTT-074
func TestLookupRefusesARevokedParticipant(t *testing.T) {
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

// VTT-074
func TestLookupRefusesAnUnknownParticipant(t *testing.T) {
	d, _ := openTemp(t)
	if _, err := d.Lookup("p-nobody"); err == nil {
		t.Fatal("an unknown id must not resolve")
	}
}

// VTT-075
func TestLookupCarriesTheWholeParticipant(t *testing.T) {
	// Assert ID too: Authorize reads it for ownership, so a partial lookup would
	// change what authorization sees.
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

// VTT-069
func TestLookupRefusesACorruptRow(t *testing.T) {
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

// VTT-043
func TestRotatingBeforeAnythingElseLeavesTheDoorSHUT(t *testing.T) {
	// Keep this test: flipping the INSERT branch's `open` literal to 1 stays green
	// everywhere else, since the mutation gate cannot mutate SQL text.
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
	allowed, err := d.JoinAdmits(secret)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("a brand-new secret admitted somebody through a door that was never opened")
	}
}

// Walks all four cells; only the open-door, right-secret cell admits.
// VTT-042
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
		got, err := d.JoinAdmits(c.offer)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.expect {
			t.Fatalf("door open=%v, correct secret=%v: allowed=%v, want %v",
				c.open, c.offer == right, got, c.expect)
		}
	}
}

// VTT-034 VTT-072
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

	if len(list) != 2 {
		t.Fatalf("got %d participants, want 2 (the revoked one must not be listed): %+v", len(list), list)
	}
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

// VTT-072
func TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder(t *testing.T) {
	// Keep the duplicate names and the id tie-break: `ORDER BY display_name, id`
	// could lose its second column and no other test would notice.
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

	// Read twice: a single read could match the order by luck.
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

// VTT-069
func TestListingRefusesACorruptRowRatherThanInventingARole(t *testing.T) {
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

// VTT-070
func TestListingRefusesWhenTheTableCannotBeRead(t *testing.T) {
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

// VTT-063
func TestACampaignPredatingTheAdmissionBudgetStillWorks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the pre-budget shape exactly: the migration must have something to do.
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

	// Assert an admission, not just no error: columns added with a budget of zero
	// would lock every existing campaign out of its link.
	admitted, err := db.JoinAdmits("old-secret")
	if err != nil {
		t.Fatalf("JoinAdmits on a migrated campaign: %v", err)
	}
	if !admitted {
		t.Fatal("a campaign that predates the budget must still admit through its open door — " +
			"migrating it to a budget of zero would shut a door its DM had left open")
	}
}

// More goroutines than slots, released together, so the race is contended.
// VTT-023
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

	// Keep rounds high: one round misses the race most of the time.
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

// VTT-021
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

// Reads `admitted` through a raw handle after the refusals.
// VTT-007
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

	// Read the counter with NO SetJoinOpen in between: it resets admitted to 0.
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

// VTT-018
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

// VTT-044 VTT-019
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
	if ok, _ := d.JoinAdmits(secret); ok {
		t.Fatal("the old secret still admits — rotating did not close the leak")
	}
}

// VTT-061
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

// Opens the same fresh file three times.
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

// VTT-073
func TestJoinBudgetReportsWhatHasBeenSpent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Require zeros for no row, not an error: the console polls before anything exists.
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

	if err := d.SetJoinOpen(false, 0); err != nil {
		t.Fatal(err)
	}
	if admitted, _, err = d.JoinBudget(); err != nil || admitted != 0 {
		t.Fatalf("a closed door still reports %d spent (%v)", admitted, err)
	}
}

// VTT-046 VTT-049 VTT-070
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
	// Keep the door open and the secret right: only the dead database can refuse below.
	d.Close()

	if ok, err := d.JoinAdmits(secret); ok || err == nil {
		t.Fatalf("JoinAdmits on a closed database returned (%v, %v) — it must report the "+
			"failure and admit nobody", ok, err)
	}
	if _, _, err := d.JoinBudget(); err == nil {
		t.Fatal("JoinBudget on a closed database reported success")
	}
	if err := d.SetJoinOpen(true, 5); err == nil {
		t.Fatal("SetJoinOpen on a closed database reported success")
	}
	if _, err := d.RotateJoinSecret(); err == nil {
		t.Fatal("RotateJoinSecret on a closed database reported success")
	}
}

// VTT-071
func TestOpeningAnUnreadableCampaignFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notadb.db")
	if err := os.WriteFile(path, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d, err := identity.Open(path); err == nil {
		d.Close()
		t.Fatal("opening a file that is not a database succeeded")
	}
}

// VTT-060
func TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readonly.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the pre-budget shape: a migration must be genuinely required.
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

// VTT-062
func TestOpeningACurrentCampaignTakesNoWriteLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.db")
	d, err := identity.Open(path)
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
	// Keep a real write in the transaction: a deferred BEGIN alone takes no lock.
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
		// Keep this under busy_timeout(5000): 2s of silence is already the wrong answer.
		t.Fatal("opening a current campaign BLOCKED behind another handle's write " +
			"transaction — migrate takes the write lock on every open, which is a lock a " +
			"read-only user has no business taking")
	}
}

// VTT-062
func TestAnAlreadyMigratedReadOnlyCampaignStillOpens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archived.db")
	d, err := identity.Open(path)
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

// VTT-017
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

// VTT-066
func TestCreateInviteRefusesARoleThatIsNotOne(t *testing.T) {
	d, _ := openTemp(t)
	if _, _, err := d.CreateInvite("Nobody", identity.Role("overlord")); err == nil {
		t.Fatal("CreateInvite accepted a role that is not one of the four")
	}
}

// VTT-070
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

// VTT-022
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

	// Keep the negative case: only it holds the guard's `<= 0` against `== 0`.
	if err := d.SetJoinOpen(true, -1); err != nil {
		t.Fatal(err)
	}
	if ok, err := d.JoinAdmits(secret); !ok {
		t.Fatalf("a door opened with a budget of -1 admitted nobody (%v)", err)
	}
	if _, limit, err := d.JoinBudget(); err != nil || limit != identity.DefaultAdmitLimit {
		t.Fatalf("a negative budget became %d (%v), want the default of %d", limit, err, identity.DefaultAdmitLimit)
	}
}
