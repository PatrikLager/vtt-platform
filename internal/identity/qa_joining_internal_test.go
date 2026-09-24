package identity

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/testdb"

	_ "modernc.org/sqlite"
)

// errQAFault is the error every armed statement fails with. A refusal that
// reached an armed write would either surface it or swallow it; tripped()
// reports the reach either way.
var errQAFault = errors.New("qa: an armed write statement was reached")

// writeVerbs are the SQL statements that write. Both cases, because Arm
// matches a substring and the implementation's casing is not ours to know.
var writeVerbs = []string{
	"UPDATE", "update",
	"INSERT", "insert",
	"DELETE", "delete",
	"REPLACE", "replace",
}

// qaFaultDB opens a campaign file through the fault-injecting driver.
func qaFaultDB(t *testing.T) (*DB, string) {
	t.Helper()
	prev := driverName
	driverName = testdb.DriverName
	t.Cleanup(func() { driverName = prev })
	path := filepath.Join(t.TempDir(), "x.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

// dataVersion reads SQLite's PRAGMA data_version on one pinned connection of
// an independent handle: it changes when ANOTHER connection commits a write,
// so it observes a write without knowing its SQL.
type dataVersion struct {
	t    *testing.T
	conn *sql.Conn
}

func qaWatch(t *testing.T, path string) *dataVersion {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open watcher: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("watcher conn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &dataVersion{t: t, conn: conn}
}

func (w *dataVersion) read() int64 {
	w.t.Helper()
	var v int64
	if err := w.conn.QueryRowContext(context.Background(), "PRAGMA data_version").Scan(&v); err != nil {
		w.t.Fatalf("PRAGMA data_version: %v", err)
	}
	return v
}

// refuseUnderEachArm calls join once per write verb, with only that verb
// armed, and returns the verbs whose armed statement was reached. One arm at
// a time: arming a second statement before the first is spent makes every
// earlier arm report tripped, because testdb.Arm overwrites the one global
// arm, so arms are never stacked here.
func refuseUnderEachArm(t *testing.T, join func() (bool, error)) (reached []string) {
	t.Helper()
	for _, v := range writeVerbs {
		tripped := testdb.Arm(v, errQAFault)
		ok, err := join()
		if tripped() {
			reached = append(reached, v)
		}
		if ok {
			t.Errorf("with %q armed: JoinAdmits = true, want a refusal", v)
		}
		if err != nil && errors.Is(err, errQAFault) {
			t.Errorf("with %q armed: JoinAdmits surfaced the armed write fault: %v", v, err)
		}
	}
	return reached
}

// VTT-008
func TestQAAShutDoorRefusalReachesNoWriteStatement(t *testing.T) {
	cases := []struct {
		name      string
		candidate func(secret string) string
	}{
		{"the current secret", func(s string) string { return s }},
		{"a wrong secret", func(string) string { return "not-the-secret" }},
		{"an empty secret", func(string) string { return "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, path := qaFaultDB(t)
			secret, err := db.JoinSecret()
			if err != nil {
				t.Fatalf("JoinSecret: %v", err)
			}
			if secret == "" {
				t.Fatal("fixture: JoinSecret returned an empty secret")
			}
			// Opened with budget, then shut: the door term is the only thing
			// that can refuse the current secret here.
			if err := db.SetJoinOpen(true, 5); err != nil {
				t.Fatalf("SetJoinOpen(true): %v", err)
			}
			if err := db.SetJoinOpen(false, 0); err != nil {
				t.Fatalf("SetJoinOpen(false): %v", err)
			}
			watch := qaWatch(t, path)
			before := watch.read()

			candidate := tc.candidate(secret)
			hit := refuseUnderEachArm(t, func() (bool, error) { return db.JoinAdmits(candidate) })
			if len(hit) != 0 {
				t.Errorf("a shut-door refusal reached write statements %v; VTT-008 says it writes nothing", hit)
			}
			if after := watch.read(); after != before {
				t.Errorf("data_version moved %d -> %d across a shut-door refusal: something committed a write", before, after)
			}
		})
	}
}

// VTT-008 (a campaign whose door was never touched reads as shut; SPEC-009
// "How it works": "a campaign with no door row is refused without creating one")
func TestQAAnUntouchedDoorRefusalReachesNoWriteStatement(t *testing.T) {
	db, path := qaFaultDB(t)
	watch := qaWatch(t, path)
	before := watch.read()

	hit := refuseUnderEachArm(t, func() (bool, error) { return db.JoinAdmits("anything") })
	if len(hit) != 0 {
		t.Errorf("an untouched-door refusal reached write statements %v", hit)
	}
	if after := watch.read(); after != before {
		t.Errorf("data_version moved %d -> %d across a refusal on an untouched door", before, after)
	}
	if admitted, limit, err := db.JoinBudget(); err != nil || admitted != 0 || limit != 0 {
		t.Errorf("JoinBudget after refusal = (%d, %d, %v), want (0, 0, nil)", admitted, limit, err)
	}
}

// VTT-008 control: the same arms and the same watcher DO see the write an
// admission makes, so their silence above is evidence and not blindness.
func TestQAAnOpenDoorAdmissionIsSeenByTheSameProbes(t *testing.T) {
	db, path := qaFaultDB(t)
	secret, err := db.JoinSecret()
	if err != nil {
		t.Fatalf("JoinSecret: %v", err)
	}
	if err := db.SetJoinOpen(true, 5); err != nil {
		t.Fatalf("SetJoinOpen: %v", err)
	}

	// Probe 1: an armed UPDATE (either case) catches the admission's write.
	var hit []string
	for _, v := range []string{"UPDATE", "update"} {
		tripped := testdb.Arm(v, errQAFault)
		ok, _ := db.JoinAdmits(secret)
		if tripped() {
			hit = append(hit, v)
			if ok {
				t.Errorf("JoinAdmits returned true although its armed %q failed", v)
			}
		}
	}
	if len(hit) == 0 {
		t.Fatalf("an open-door admission reached no armed UPDATE: the arms observe nothing")
	}

	// Probe 2: the data_version watcher sees an admission commit.
	watch := qaWatch(t, path)
	before := watch.read()
	ok, err := db.JoinAdmits(secret)
	if err != nil || !ok {
		t.Fatalf("JoinAdmits at an open door with budget = (%v, %v), want (true, nil)", ok, err)
	}
	if after := watch.read(); after == before {
		t.Errorf("data_version did not move across an admission: the watcher observes nothing")
	}
}
