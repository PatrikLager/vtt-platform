package gateway_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

	// path is the campaign file, so a test can count ROWS. The endpoint's
	// refusal properties are about what it did NOT create, and a decoded
	// reply cannot witness that — see post's comment for how that went wrong.
	path string
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
	return &joinFixture{t: t, srv: srv, ids: ids, path: path}
}

// count returns the number of rows in table, straight from the file.
func (f *joinFixture) count(table string) int {
	f.t.Helper()
	raw, err := sql.Open("sqlite", f.path)
	if err != nil {
		f.t.Fatal(err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
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
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
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
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
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
	// Not just refused — NO ROW. A refusal that still created a participant
	// would make this unauthenticated endpoint a way for any stranger to fill
	// the table's database.
	//
	// THE FIRST VERSION OF THIS TEST COULD NOT FAIL, and it is the same defect
	// post's comment describes one screen above. It asserted on the DECODED
	// reply, but http.Error writes plain text: got.Token was always "", so the
	// first assertion was a tautology and the second was Verify(""), which
	// errors unconditionally. Injection proved it — minting a participant on
	// every refused join failed nothing in this package OR in identity. The
	// rows are the only honest witness to a claim about what was not created.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	before := f.count("participants")
	resp, _, _ := f.post(secret, "Kim")
	if resp.StatusCode == http.StatusOK {
		t.Fatal("a closed door must refuse even the correct secret")
	}
	if after := f.count("participants"); after != before {
		t.Fatalf("a refused join minted %d participant(s) anyway", after-before)
	}
}

func TestARefusedJoinWritesNothingAtAll(t *testing.T) {
	// Spec §2 rests its case against rate limiting entirely on the closed door
	// leaving "no standing endpoint to hammer" — the link is INERT. That is a
	// claim about writes, not just about credentials, and it has to be checked
	// HERE rather than only in identity: identity.JoinAllows can be perfectly
	// read-only while this handler still reaches the minting path. That seam
	// is precisely the shape this plan was built around.
	//
	// Nothing below asks for the secret first: JoinSecret() MINTS the row, so
	// a fixture that warms it up cannot see this.
	f := newJoinFixture(t)
	if n := f.count("join_access"); n != 0 {
		t.Fatalf("fixture is not fresh: %d join_access row(s) before any request", n)
	}
	resp, _, _ := f.post("a stranger's guess", "Kim")
	if resp.StatusCode == http.StatusOK {
		t.Fatal("a campaign whose door was never opened must refuse")
	}
	if n := f.count("join_access"); n != 0 {
		t.Fatalf("an anonymous, REFUSED request wrote %d row(s) — it takes SQLite's write "+
			"lock on the file internal/store appends every event to, on the one path a "+
			"stranger controls", n)
	}
}

func TestAnOversizedBodyIsRefusedBeforeItIsRead(t *testing.T) {
	// The cap on the ONE surface an anonymous stranger controls, and it was
	// exercised by nothing: raise maxJoinBody to a gigabyte and everything
	// still passed.
	//
	// The oversize goes in the SECRET, not the display name. An 8KB name is
	// refused 400 by the rune bound either way, so that shape would pass
	// whether the cap existed or not — the wrong-reason trap, in the test
	// written to close a gap.
	f := newJoinFixture(t)
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	before := f.count("participants")

	resp, _, body := f.post(strings.Repeat("x", 8<<10), "Kim")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for a body past the cap", resp.StatusCode, http.StatusBadRequest)
	}
	// Refused as MALFORMED, not as a door problem: the request never got far
	// enough to be a guess at the secret, and reporting it as one would tell a
	// prober their oversized body was merely the wrong password.
	if !strings.Contains(body, "malformed") {
		t.Fatalf("an oversized body must be refused as malformed, got %q", strings.TrimSpace(body))
	}
	if after := f.count("participants"); after != before {
		t.Fatal("an oversized request minted a participant")
	}
}

func TestAnEmptyDisplayNameIsRefused(t *testing.T) {
	// It is what the whole table sees. Blank, or whitespace pretending to be
	// blank, is not a name.
	//
	// Refused DISTINCTLY, and asserted as such: this is the joiner's own
	// mistake and saying so leaks nothing about the door or the secret. The
	// earlier version asserted only "not 200", so collapsing this into the
	// shared refusal failed nothing — and that would tell somebody who simply
	// forgot to type their name that the link is closed.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "   ", "\t\n"} {
		resp, _, body := f.post(secret, name)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("display name %q: status %d, want %d", name, resp.StatusCode, http.StatusBadRequest)
		}
		if strings.Contains(body, "not accepting anyone") {
			t.Fatalf("display name %q was refused with the DOOR's message (%q) — a joiner who "+
				"forgot their name would be told the link is closed", name, strings.TrimSpace(body))
		}
	}
}

func TestADisplayNameIsBoundedAndPrintable(t *testing.T) {
	// An UNAUTHENTICATED caller chooses this string, and every client at the
	// table then renders it in every presence frame. Length and control
	// characters are two different ways for a stranger to decide what everyone
	// else's screen does — the client escapes with textContent, so this is not
	// XSS, but "not XSS" is not the same as "bounded".
	//
	// Refused distinctly, like the blank name and for the same reason: it is
	// the joiner's own input and telling them says nothing about the door.
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		strings.Repeat("a", 65),             // bounded only by the body cap
		strings.Repeat("\u00e5", 200),       // and bounded by RUNES, not bytes
		"Kim\u001b[31m",                     // an ANSI escape, for anything that logs it
		"Kim\nDM: everyone roll initiative", // a newline, to forge a second line
		"Kim\u202emiK",                      // a BIDI override, so the name reads as somebody else
		"\u200b\u200b\u200b",                // zero-width spaces: passes every other rule, shows nothing
		"\ufeff",                            // a byte-order mark, the same shape
		"\u3164",                            // HANGUL FILLER, the classic invisible-name trick
		"Kim\u0000",                         // a NUL, for anything that is not Go
	} {
		resp, _, _ := f.post(secret, name)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("display name %q: status %d, want %d", name, resp.StatusCode, http.StatusBadRequest)
		}
	}
	// And an ordinary name with non-ASCII in it still gets in — a bound that
	// only accepts ASCII would lock out most of the people who might play.
	resp, got, _ := f.post(secret, "Åsa Örn")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a perfectly ordinary name was refused: %d", resp.StatusCode)
	}
	if got.Name != "Åsa Örn" {
		t.Fatalf("display name came back as %q", got.Name)
	}

	// THE BOUNDARY ITSELF, from both sides. 65 above is refused; exactly the
	// cap must be ACCEPTED, or the limit is quietly 63 and the only person who
	// finds out is the one whose name is that long.
	//
	// The mutation gate caught this gap: > became >= and every test still
	// passed, because nothing sat on the boundary. 64 is written literally
	// rather than read from the constant — this is the external test package,
	// and a test that recomputes the bound from the same expression it is
	// checking cannot catch the bound being wrong.
	resp, _, _ = f.post(secret, strings.Repeat("n", 64))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a name of exactly the documented 64-rune limit was refused (%d) — the limit "+
			"is off by one", resp.StatusCode)
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
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
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
	if err := f.ids.SetJoinOpen(true, 100); err != nil {
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

// TestTheDoorStopsAdmittingWhenItsBudgetIsSpent is the END-TO-END wiring, and
// it exists because the registry-level tests cannot see it.
//
// Every identity test injects its own budget and calls JoinAdmits directly. If
// handleJoin kept calling JoinAllows — which still exists, still compiles, and
// still answers the same question WITHOUT spending anything — the cap would be
// perfect in internal/identity and absent from the product. That is the exact
// shape a review caught in this session's previous change: a feature dead in
// production behind a fully green suite.
func TestTheDoorStopsAdmittingWhenItsBudgetIsSpent(t *testing.T) {
	f := newJoinFixture(t)
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true, 2); err != nil {
		t.Fatal(err)
	}

	for i := range 2 {
		resp, _, body := f.post(secret, fmt.Sprintf("Player %d", i))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("join %d of 2 returned %d (%q) — the budget was spent early",
				i+1, resp.StatusCode, body)
		}
	}

	spent, _, spentBody := f.post(secret, "One Too Many")
	if spent.StatusCode != http.StatusForbidden {
		t.Fatalf("the third join returned %d, want 403 — an open door mints without limit, "+
			"so a leaked link is bounded only by how fast anyone clicks", spent.StatusCode)
	}

	// The SAME answer as a wrong secret, byte for byte. A prober must not learn
	// which of the three refusals they hit (spec §5) — and the raw body is
	// compared, not the decoded struct, because http.Error writes plain text
	// and two failed decodes are identical whatever the bodies said.
	wrong, _, wrongBody := f.post("not-the-secret", "Prober")
	if wrong.StatusCode != spent.StatusCode || wrongBody != spentBody {
		t.Fatalf("a spent budget answers %d %q and a wrong secret answers %d %q — the "+
			"difference tells a prober the door is real and merely full",
			spent.StatusCode, spentBody, wrong.StatusCode, wrongBody)
	}

	// And it minted exactly the two it admitted.
	if n := f.count("participants"); n != 2 {
		t.Fatalf("%d participants exist, want 2 — the refused join minted one anyway", n)
	}
}
