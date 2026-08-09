package gateway_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// The shared join link (joining-a-table spec §2, plan J2).
//
// This endpoint is UNAUTHENTICATED BY CONSTRUCTION — that is its whole point —
// so it is the one surface in the product where getting a refusal wrong hands
// something to a stranger. It mints a spectator and returns a token, and it
// does nothing else: no campaign state, no event, no role from the caller.

type joinFixture struct {
	t   *testing.T
	srv *httptest.Server
	ids *identity.DB
}

func newJoinFixture(t *testing.T) *joinFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })

	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(srv.Close)
	return &joinFixture{t: t, srv: srv, ids: ids}
}

type joinReply struct {
	Token         string `json:"token"`
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

// post returns the response, the decoded reply, AND THE RAW BODY.
//
// The raw body is not a convenience. The first version of this fixture
// compared decoded joinReply structs to check that two refusals were
// identical — but http.Error writes PLAIN TEXT, so decoding failed and both
// sides were zero values, identical whatever the bodies actually said. The
// central security property of this endpoint was pinned by nothing, and
// injection proved it: making the closed-door case say so distinctly failed
// no test.
func (f *joinFixture) post(secret, name string) (*http.Response, joinReply, string) {
	f.t.Helper()
	body, err := json.Marshal(map[string]string{"secret": secret, "displayName": name})
	if err != nil {
		f.t.Fatal(err)
	}
	resp, err := http.Post(f.srv.URL+"/join", "application/json", strings.NewReader(string(body)))
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	var out joinReply
	_ = json.Unmarshal(raw, &out)
	return resp, out, string(raw)
}

func TestJoiningThroughAnOpenDoorMintsASpectator(t *testing.T) {
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true); err != nil {
		t.Fatal(err)
	}

	resp, got, _ := f.post(secret, "Kim")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got.Token == "" || got.ParticipantID == "" {
		t.Fatalf("a join must return a usable credential, got %+v", got)
	}
	// SPECTATOR, always. The caller does not choose (spec §2, §5).
	if got.Role != string(identity.RoleSpectator) {
		t.Fatalf("role = %q, want spectator — anyone through the door must be unable to act",
			got.Role)
	}
	if got.Name != "Kim" {
		t.Fatalf("name = %q, want the name they gave", got.Name)
	}

	// And the credential is REAL: it verifies to the participant just minted.
	p, err := f.ids.Verify(got.Token)
	if err != nil {
		t.Fatalf("the returned token must verify: %v", err)
	}
	if p.ID != got.ParticipantID || p.Role != identity.RoleSpectator {
		t.Fatalf("verified %+v, want the spectator that was returned", p)
	}
}

func TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY(t *testing.T) {
	// The security property of this endpoint. A distinguishable refusal tells
	// a prober which half they got right — whether the campaign exists and is
	// merely shut, or whether their guess at the secret was wrong. Verify()
	// already sets this precedent deliberately for tokens.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}

	// Door CLOSED, secret RIGHT.
	closedRight, _, bodyA := f.post(secret, "Kim")
	// Door OPEN, secret WRONG.
	if err := f.ids.SetJoinOpen(true); err != nil {
		t.Fatal(err)
	}
	openWrong, _, bodyB := f.post("not-the-secret", "Kim")

	if closedRight.StatusCode != openWrong.StatusCode {
		t.Fatalf("status differs: closed-door=%d wrong-secret=%d — the difference tells a "+
			"prober which half they got right", closedRight.StatusCode, openWrong.StatusCode)
	}
	// RAW bodies, not decoded structs: these responses are plain text, so a
	// decoded comparison sees two zero values and can never fail.
	if bodyA != bodyB {
		t.Fatalf("body differs:\n  closed door: %q\n  wrong secret: %q\nthat difference "+
			"tells a prober which half they got right", bodyA, bodyB)
	}
	if closedRight.StatusCode == http.StatusOK {
		t.Fatal("neither of those may succeed")
	}
}

func TestAClosedDoorMintsNobody(t *testing.T) {
	// Not just refused — no row. A refusal that still created a participant
	// would make this endpoint a way to fill the table's database.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	_, got, _ := f.post(secret, "Kim")
	if got.Token != "" {
		t.Fatal("a refused join must not return a credential")
	}
	if _, err := f.ids.Verify(got.Token); err == nil {
		t.Fatal("a refused join must not have minted a participant")
	}
}

func TestAnEmptyDisplayNameIsRefused(t *testing.T) {
	// It is what the whole table sees. Blank, or whitespace pretending to be
	// blank, is not a name.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "   ", "\t\n"} {
		resp, _, _ := f.post(secret, name)
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("display name %q was accepted", name)
		}
	}
}

func TestTwoJoinersGetDistinctIdentities(t *testing.T) {
	// The point of minting per person rather than sharing one credential: two
	// people through the same link are two participants, separately revocable,
	// each keeping their own characters.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true); err != nil {
		t.Fatal(err)
	}
	_, a, _ := f.post(secret, "Kim")
	_, b, _ := f.post(secret, "Ada")
	if a.Token == b.Token || a.ParticipantID == b.ParticipantID {
		t.Fatalf("two joiners must get distinct credentials: %+v vs %+v", a, b)
	}
}

func TestRotatingTheLinkRefusesTheOldSecret(t *testing.T) {
	// The property spec §2 calls close to required: a leaked link is closable.
	f := newJoinFixture(t)
	old, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true); err != nil {
		t.Fatal(err)
	}
	if resp, _, _ := f.post(old, "Kim"); resp.StatusCode != http.StatusOK {
		t.Fatal("the old secret should work before rotating")
	}
	fresh, err := f.ids.RotateJoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if resp, _, _ := f.post(old, "Mallory"); resp.StatusCode == http.StatusOK {
		t.Fatal("a rotated link must refuse the OLD secret, or rotating protects nobody")
	}
	if resp, _, _ := f.post(fresh, "Ada"); resp.StatusCode != http.StatusOK {
		t.Fatal("the NEW secret must work, or rotating locks the table out")
	}
}
