package gateway_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

type qaEnv struct {
	t      *testing.T
	dir    string
	ids    *identity.DB
	srv    *httptest.Server
	secret string
}

func qaSetup(t *testing.T, limit int) *qaEnv {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() { srv.Close(); ids.Close(); c.Close() })
	sec, err := ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := ids.SetJoinOpen(true, limit); err != nil {
		t.Fatal(err)
	}
	return &qaEnv{t: t, dir: dir, ids: ids, srv: srv, secret: sec}
}

type qaReply struct {
	status int
	body   string
	Token  string `json:"token"`
	ID     string `json:"participantId"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

func (e *qaEnv) join(secret, name string) qaReply {
	e.t.Helper()
	r := e.post(secret, name)
	if r.status == 0 {
		e.t.Fatal(r.body)
	}
	return r
}

func (e *qaEnv) post(secret, name string) qaReply {
	b, _ := json.Marshal(map[string]string{"secret": secret, "displayName": name})
	resp, err := http.Post(e.srv.URL+"/join", "application/json", bytes.NewReader(b))
	if err != nil {
		return qaReply{body: err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	r := qaReply{status: resp.StatusCode, body: string(raw)}
	if resp.StatusCode == http.StatusOK {
		_ = json.Unmarshal(raw, &r)
	}
	return r
}

func (e *qaEnv) rows() int {
	e.t.Helper()
	db, err := sql.Open("sqlite", campaign.LogPath(e.dir))
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM participants").Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *qaEnv) hold(name string, role identity.Role) string {
	e.t.Helper()
	_, id, err := e.ids.CreateInvite(name, role)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *qaEnv) admitted() int {
	e.t.Helper()
	a, _, err := e.ids.JoinBudget()
	if err != nil {
		e.t.Fatal(err)
	}
	return a
}

func qaWant(t *testing.T, what string, r qaReply, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: status %d want %d, body %q", what, r.status, status, r.body)
	}
}

// VTT-542 VTT-546 VTT-552
func TestQAHeldNameIsRefusedIgnoringCaseForEveryRole(t *testing.T) {
	e := qaSetup(t, 50)
	e.hold("Ægir Ünal", identity.RoleDM)
	e.hold("Agent Smith", identity.RoleAgent)
	e.hold("Pip", identity.RolePlayer)
	e.hold("Quill", identity.RoleSpectator)
	for _, name := range []string{"ægir ünal", "ÆGIR ÜNAL", "AGENT smith", "pIP", "qUILL", "Pip"} {
		r := e.join(e.secret, name)
		qaWant(t, name, r, 400)
		if !strings.Contains(r.body, "taken") {
			t.Fatalf("%q: body does not say taken: %q", name, r.body)
		}
	}
	r := e.join(e.secret, "Fresh Face")
	qaWant(t, "free", r, 200)
	if r.Role != "spectator" || r.Token == "" || r.ID == "" {
		t.Fatalf("admission reply %+v", r)
	}
}

// VTT-542
func TestQAAJoinedNameHoldsItselfAgainstLaterJoiner(t *testing.T) {
	e := qaSetup(t, 50)
	qaWant(t, "first", e.join(e.secret, "Zed"), 200)
	qaWant(t, "second", e.join(e.secret, "zED"), 400)
	qaWant(t, "other", e.join(e.secret, "Zeda"), 200)
}

// VTT-545 VTT-011
func TestQATakenNameWritesNothing(t *testing.T) {
	e := qaSetup(t, 50)
	e.hold("Held", identity.RolePlayer)
	rows, spent := e.rows(), e.admitted()
	qaWant(t, "taken", e.join(e.secret, "hELD"), 400)
	if e.rows() != rows || e.admitted() != spent {
		t.Fatalf("rows %d->%d admitted %d->%d", rows, e.rows(), spent, e.admitted())
	}
	qaWant(t, "free", e.join(e.secret, "Free"), 200)
	if e.rows() != rows+1 || e.admitted() != spent+1 {
		t.Fatalf("free join: rows %d admitted %d", e.rows(), e.admitted())
	}
}

// VTT-009 VTT-013 VTT-546
func TestQADoorRefusalsLeakNoNames(t *testing.T) {
	e := qaSetup(t, 1)
	e.hold("Held", identity.RolePlayer)
	nameRefusal := e.join(e.secret, "held")
	qaWant(t, "taken", nameRefusal, 400)
	if !strings.Contains(nameRefusal.body, "taken") {
		t.Fatalf("name refusal %q does not say the name is taken", nameRefusal.body)
	}

	wrongHeld := e.join("not-the-secret", "held")
	wrongFree := e.join("not-the-secret", "Nobody")
	qaWant(t, "wrong/held", wrongHeld, 403)
	qaWant(t, "wrong/free", wrongFree, 403)
	if wrongHeld.body != wrongFree.body || wrongHeld.body == nameRefusal.body {
		t.Fatalf("bodies: %q %q name %q", wrongHeld.body, wrongFree.body, nameRefusal.body)
	}

	emptyHeld := e.join("", "held")
	qaWant(t, "empty secret", emptyHeld, 403)
	if emptyHeld.body != wrongFree.body {
		t.Fatalf("empty-secret body %q", emptyHeld.body)
	}

	if err := e.ids.SetJoinOpen(false, 1); err != nil {
		t.Fatal(err)
	}
	shutHeld, shutFree := e.join(e.secret, "held"), e.join(e.secret, "Nobody")
	qaWant(t, "shut/held", shutHeld, 403)
	qaWant(t, "shut/free", shutFree, 403)
	if shutHeld.body != wrongFree.body || shutFree.body != wrongFree.body {
		t.Fatalf("shut bodies %q %q", shutHeld.body, shutFree.body)
	}

	if err := e.ids.SetJoinOpen(true, 1); err != nil {
		t.Fatal(err)
	}
	qaWant(t, "spend", e.join(e.secret, "Spender"), 200)
	spentHeld, spentFree := e.join(e.secret, "held"), e.join(e.secret, "Nobody")
	qaWant(t, "spent/held", spentHeld, 403)
	qaWant(t, "spent/free", spentFree, 403)
	if spentHeld.body != wrongFree.body || spentFree.body != wrongFree.body {
		t.Fatalf("spent bodies %q %q", spentHeld.body, spentFree.body)
	}
}

// VTT-544 VTT-542
func TestQARevokedHolderDoesNotHoldTheName(t *testing.T) {
	e := qaSetup(t, 50)
	gone := e.hold("Ørjan", identity.RolePlayer)
	e.hold("Both", identity.RolePlayer)
	qaWant(t, "before revoke", e.join(e.secret, "ørjan"), 400)
	if err := e.ids.Revoke(gone); err != nil {
		t.Fatal(err)
	}
	r := e.join(e.secret, "ørjan")
	qaWant(t, "after revoke", r, 200)
	qaWant(t, "still held", e.join(e.secret, "BOTH"), 400)
}

// VTT-544
func TestQAOnlyRevokedHoldersLeaveTheNameFree(t *testing.T) {
	e := qaSetup(t, 50)
	if _, id, err := e.ids.CreateInvite("Solo", identity.RolePlayer); err != nil {
		t.Fatal(err)
	} else if err := e.ids.Revoke(id); err != nil {
		t.Fatal(err)
	}
	if _, id, err := e.ids.CreateInvite("SOLO", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	} else if err := e.ids.Revoke(id); err != nil {
		t.Fatal(err)
	}
	qaWant(t, "free", e.join(e.secret, "solo"), 200)
	qaWant(t, "now held", e.join(e.secret, "Solo"), 400)
}

// VTT-555 VTT-542
func TestQATrimBeforeJudgedAndStored(t *testing.T) {
	e := qaSetup(t, 50)
	e.hold("Alice", identity.RolePlayer)
	qaWant(t, "padded held", e.join(e.secret, " \t aLICE \n"), 400)
	r := e.join(e.secret, "  Bob \t")
	qaWant(t, "padded free", r, 200)
	if r.Name != "Bob" {
		t.Fatalf("reply name %q", r.Name)
	}
	ps, err := e.ids.List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range ps {
		if p.ID == r.ID {
			found = true
			if p.Name != "Bob" {
				t.Fatalf("stored name %q", p.Name)
			}
		}
	}
	if !found {
		t.Fatal("joined participant not listed")
	}
	qaWant(t, "trimmed now held", e.join(e.secret, "bob"), 400)
}

// VTT-549 VTT-552
func TestQAUsableDisplayNameBounds(t *testing.T) {
	e := qaSetup(t, 50)
	for _, n := range []string{"", "   ", "\t\n", "ctl\x07char", "nul\x00", "rtl\u202eevil", "\u200b", "\u200b\u200b"} {
		r := e.join(e.secret, n)
		qaWant(t, "unusable "+n, r, 400)
	}
	qaWant(t, "65 runes", e.join(e.secret, strings.Repeat("é", 65)), 400)
	qaWant(t, "64 runes", e.join(e.secret, strings.Repeat("é", 64)), 200)
	qaWant(t, "non-ascii", e.join(e.secret, "Łukasz Ωmega 龍"), 200)
	qaWant(t, "non-ascii again", e.join(e.secret, "łukasz ωmega 龍"), 400)
	if got := e.rows(); got != 2 {
		t.Fatalf("rows %d want 2", got)
	}
}

// VTT-550
func TestQARotationRefusesOldAdmitsNew(t *testing.T) {
	e := qaSetup(t, 50)
	e.hold("Held", identity.RolePlayer)
	old := e.secret
	qaWant(t, "old ok", e.join(old, "Before"), 200)
	next, err := e.ids.RotateJoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if next == old {
		t.Fatal("secret unchanged")
	}
	qaWant(t, "old refused", e.join(old, "After"), 403)
	qaWant(t, "new held", e.join(next, "HELD"), 400)
	qaWant(t, "new free", e.join(next, "After"), 200)
}

// VTT-551
func TestQATwoJoinersAreTwoParticipants(t *testing.T) {
	e := qaSetup(t, 50)
	a, b := e.join(e.secret, "Ada"), e.join(e.secret, "Adam")
	qaWant(t, "a", a, 200)
	qaWant(t, "b", b, 200)
	if a.ID == b.ID || a.Token == b.Token || a.Token == "" || b.Token == "" {
		t.Fatalf("not distinct: %+v %+v", a, b)
	}
	if a.Role != "spectator" || b.Role != "spectator" {
		t.Fatalf("roles %q %q", a.Role, b.Role)
	}
	p, err := e.ids.Verify(a.Token)
	if err != nil || p.ID != a.ID || p.Name != "Ada" {
		t.Fatalf("verify %+v %v", p, err)
	}
}

// VTT-553
func TestQARotatedLinkAfterSpentBudgetAdmitsNewHolder(t *testing.T) {
	e := qaSetup(t, 1)
	qaWant(t, "spend", e.join(e.secret, "One"), 200)
	qaWant(t, "spent", e.join(e.secret, "Two"), 403)
	next, err := e.ids.RotateJoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	qaWant(t, "old", e.join(e.secret, "Two"), 403)
	qaWant(t, "taken", e.join(next, "one"), 400)
	qaWant(t, "new", e.join(next, "Two"), 200)
}

// VTT-554
func TestQARaceForLastAdmissionAdmitsExactlyOne(t *testing.T) {
	for i := 0; i < 5; i++ {
		e := qaSetup(t, 1)
		var wg sync.WaitGroup
		out := make([]qaReply, 2)
		for j, n := range []string{"Racer A", "Racer B"} {
			wg.Add(1)
			go func() { defer wg.Done(); out[j] = e.post(e.secret, n) }()
		}
		wg.Wait()
		ok := 0
		for _, r := range out {
			if r.status == 200 {
				ok++
			} else if r.status != 403 {
				t.Fatalf("loser status %d %q", r.status, r.body)
			}
		}
		if ok != 1 || e.rows() != 1 {
			t.Fatalf("round %d: admitted %d rows %d", i, ok, e.rows())
		}
	}
}
