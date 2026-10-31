package identity_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/identity"

	_ "modernc.org/sqlite"
)

func qaOpen(t *testing.T, limit int) (*identity.DB, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.db")
	db, err := identity.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.SetJoinOpen(true, limit); err != nil {
		t.Fatalf("SetJoinOpen: %v", err)
	}
	secret, err := db.JoinSecret()
	if err != nil {
		t.Fatalf("JoinSecret: %v", err)
	}
	return db, path, secret
}

func qaInvite(t *testing.T, db *identity.DB, name string, role identity.Role) string {
	t.Helper()
	_, id, err := db.CreateInvite(name, role)
	if err != nil {
		t.Fatalf("CreateInvite(%q): %v", name, err)
	}
	return id
}

func qaWantTaken(t *testing.T, db *identity.DB, secret, name string) {
	t.Helper()
	ok, err := db.JoinAdmits(secret, name)
	if ok || !errors.Is(err, identity.ErrNameTaken) {
		t.Errorf("JoinAdmits(%q) = (%v, %v), want (false, ErrNameTaken)", name, ok, err)
	}
}

func qaWantAdmitted(t *testing.T, db *identity.DB, secret, name string) {
	t.Helper()
	ok, err := db.JoinAdmits(secret, name)
	if !ok || err != nil {
		t.Errorf("JoinAdmits(%q) = (%v, %v), want (true, nil)", name, ok, err)
	}
}

// VTT-542 VTT-552
func TestQAAHeldNameIsRefusedIgnoringCase(t *testing.T) {
	db, _, secret := qaOpen(t, 50)
	qaInvite(t, db, "Kim", identity.RolePlayer)
	qaInvite(t, db, "Ünal", identity.RoleSpectator)
	qaInvite(t, db, "ΟΔΥΣΣΕΎΣ", identity.RolePlayer)
	for _, n := range []string{"Kim", "kim", "KIM", "kIm", "ünal", "ÜNAL", "οδυσσεύς", "οδυσσεύσ"} {
		qaWantTaken(t, db, secret, n)
	}
	qaWantAdmitted(t, db, secret, "Lou")
	qaWantAdmitted(t, db, secret, "Kimberly")
}

// VTT-542
func TestQAADMAndAgentNamesAreHeldToo(t *testing.T) {
	db, _, secret := qaOpen(t, 50)
	qaInvite(t, db, "Dungeon", identity.RoleDM)
	qaInvite(t, db, "Robo", identity.RoleAgent)
	qaWantTaken(t, db, secret, "dungeon")
	qaWantTaken(t, db, secret, "ROBO")
	qaWantAdmitted(t, db, secret, "Freeperson")
}

// VTT-543
func TestQACreateInviteRefusesAHeldNameAndMintsNothing(t *testing.T) {
	db, _, _ := qaOpen(t, 5)
	qaInvite(t, db, "Kim", identity.RolePlayer)
	for _, n := range []string{"Kim", "KIM", "kim"} {
		tok, id, err := db.CreateInvite(n, identity.RoleSpectator)
		if !errors.Is(err, identity.ErrNameTaken) || tok != "" || id != "" {
			t.Errorf("CreateInvite(%q) = (%q, %q, %v), want empty and ErrNameTaken", n, tok, id, err)
		}
	}
	qaInvite(t, db, "Lou", identity.RoleSpectator)
	ps, err := db.List()
	if err != nil || len(ps) != 2 {
		t.Fatalf("List = %d participants, %v; want 2 (Kim, Lou)", len(ps), err)
	}
}

// VTT-544
func TestQAARevokedHolderReleasesTheName(t *testing.T) {
	db, _, secret := qaOpen(t, 50)
	id := qaInvite(t, db, "Kim", identity.RolePlayer)
	qaWantTaken(t, db, secret, "kim")
	if err := db.Revoke(id); err != nil {
		t.Fatal(err)
	}
	qaWantAdmitted(t, db, secret, "KIM")
	if _, _, err := db.CreateInvite("kim", identity.RolePlayer); err != nil {
		t.Errorf("CreateInvite over a revoked holder: %v", err)
	}
	qaWantTaken(t, db, secret, "Kim")
}

// VTT-544
func TestQAOneLiveHolderAmongRevokedStillHolds(t *testing.T) {
	db, _, secret := qaOpen(t, 50)
	old := qaInvite(t, db, "Kim", identity.RolePlayer)
	if err := db.Revoke(old); err != nil {
		t.Fatal(err)
	}
	qaInvite(t, db, "kim", identity.RolePlayer)
	qaWantTaken(t, db, secret, "KIM")
	qaWantAdmitted(t, db, secret, "Other")
}

func qaDataVersion(t *testing.T, path string) func() int64 {
	t.Helper()
	w, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = w.Close() })
	return func() int64 {
		var v int64
		if err := w.QueryRow("PRAGMA data_version").Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
}

// VTT-545
func TestQATakenNameRefusalWritesNothing(t *testing.T) {
	db, path, secret := qaOpen(t, 5)
	qaInvite(t, db, "Kim", identity.RolePlayer)
	read := qaDataVersion(t, path)
	v0 := read()
	a0, l0, err := db.JoinBudget()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Kim", "kim", "KIM"} {
		qaWantTaken(t, db, secret, n)
	}
	if v := read(); v != v0 {
		t.Errorf("data_version moved %d -> %d across refusals", v0, v)
	}
	a1, l1, _ := db.JoinBudget()
	if a1 != a0 || l1 != l0 {
		t.Errorf("budget moved (%d,%d) -> (%d,%d) across refusals", a0, l0, a1, l1)
	}
	qaWantAdmitted(t, db, secret, "Lou")
	if v := read(); v == v0 {
		t.Errorf("data_version did not move across an admission: the watcher sees nothing")
	}
	if a2, _, _ := db.JoinBudget(); a2 != a0+1 {
		t.Errorf("spent after one admission = %d, want %d", a2, a0+1)
	}
}

// VTT-009
func TestQAStrangerLearnsNoNames(t *testing.T) {
	db, _, secret := qaOpen(t, 1)
	qaInvite(t, db, "Kim", identity.RolePlayer)
	ok1, e1 := db.JoinAdmits("wrong-secret", "Kim")
	ok2, e2 := db.JoinAdmits("wrong-secret", "Nobody")
	if ok1 || ok2 || errors.Is(e1, identity.ErrNameTaken) || errors.Is(e2, identity.ErrNameTaken) {
		t.Errorf("wrong secret: held=(%v,%v) free=(%v,%v), want plain refusals", ok1, e1, ok2, e2)
	}
	if (e1 == nil) != (e2 == nil) || (e1 != nil && e1.Error() != e2.Error()) {
		t.Errorf("wrong secret differs by name: %v vs %v", e1, e2)
	}
	qaWantAdmitted(t, db, secret, "Lou")
	ok3, e3 := db.JoinAdmits(secret, "Kim")
	ok4, e4 := db.JoinAdmits(secret, "Nobody")
	if ok3 || ok4 || errors.Is(e3, identity.ErrNameTaken) || errors.Is(e4, identity.ErrNameTaken) {
		t.Errorf("spent budget: held=(%v,%v) free=(%v,%v), want plain refusals", ok3, e3, ok4, e4)
	}
	if err := db.SetJoinOpen(false, 5); err != nil {
		t.Fatal(err)
	}
	ok5, e5 := db.JoinAdmits(secret, "Kim")
	if ok5 || errors.Is(e5, identity.ErrNameTaken) {
		t.Errorf("shut door: (%v,%v), want plain refusal", ok5, e5)
	}
}

// VTT-550 VTT-553
func TestQARotationAdmitsTheNewSecretUnderAFreeName(t *testing.T) {
	db, _, secret := qaOpen(t, 1)
	qaInvite(t, db, "Kim", identity.RolePlayer)
	qaWantAdmitted(t, db, secret, "Lou")
	if ok, _ := db.JoinAdmits(secret, "Mia"); ok {
		t.Fatal("spent budget admitted")
	}
	fresh, err := db.RotateJoinSecret()
	if err != nil || fresh == secret {
		t.Fatalf("Rotate = %q, %v", fresh, err)
	}
	if ok, _ := db.JoinAdmits(secret, "Mia"); ok {
		t.Error("old secret admitted after rotation")
	}
	qaWantTaken(t, db, fresh, "KIM")
	qaWantAdmitted(t, db, fresh, "Mia")
}

// VTT-551
func TestQATwoJoinersAreTwoParticipantsWithDistinctCredentials(t *testing.T) {
	db, _, secret := qaOpen(t, 5)
	qaWantAdmitted(t, db, secret, "Ann")
	t1, id1, err := db.CreateInvite("Ann", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	qaWantAdmitted(t, db, secret, "Bob")
	t2, id2, err := db.CreateInvite("Bob", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 || t1 == t2 || t1 == "" || t2 == "" {
		t.Errorf("not distinct: ids %q %q tokens %q %q", id1, id2, t1, t2)
	}
	p, err := db.Verify(t2)
	if err != nil || p.ID != id2 || p.Name != "Bob" {
		t.Errorf("Verify(t2) = %+v, %v", p, err)
	}
}

// VTT-554
func TestQARaceForLastAdmissionAdmitsExactlyOne(t *testing.T) {
	for i := 0; i < 20; i++ {
		db, _, secret := qaOpen(t, 1)
		qaInvite(t, db, "Kim", identity.RolePlayer)
		var wg sync.WaitGroup
		res := make([]bool, 2)
		for j, n := range []string{"Ann", "Bob"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res[j], _ = db.JoinAdmits(secret, n)
			}()
		}
		wg.Wait()
		if res[0] == res[1] {
			t.Fatalf("round %d: results %v, want exactly one admitted", i, res)
		}
	}
}

// VTT-072
func TestQAListOrdersByName(t *testing.T) {
	db, _, _ := qaOpen(t, 5)
	for _, n := range []string{"Charlie", "Alpha", "Bravo"} {
		qaInvite(t, db, n, identity.RolePlayer)
	}
	ps, err := db.List()
	if err != nil || len(ps) != 3 {
		t.Fatalf("List = %d, %v", len(ps), err)
	}
	for i, want := range []string{"Alpha", "Bravo", "Charlie"} {
		if ps[i].Name != want {
			t.Errorf("List[%d] = %q, want %q", i, ps[i].Name, want)
		}
	}
}

// VTT-046
func TestQAClosedStoreAdmitsNobodyNorReportsTaken(t *testing.T) {
	db, _, secret := qaOpen(t, 5)
	qaInvite(t, db, "Kim", identity.RolePlayer)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Kim", "Free"} {
		ok, err := db.JoinAdmits(secret, n)
		if ok || err == nil || errors.Is(err, identity.ErrNameTaken) {
			t.Errorf("JoinAdmits(%q) on closed store = (%v, %v), want (false, non-taken error)", n, ok, err)
		}
	}
}
