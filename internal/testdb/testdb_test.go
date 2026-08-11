package testdb_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/testdb"
)

// A fault driver that does not trip makes every test built on it pass
// vacuously — the failure it is supposed to inject never happens, the code
// under test takes its happy path, and the assertion "it reported the error"
// is never reached. So this package's own tests are the load-bearing ones.

var errBoom = errors.New("boom")

func open(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open(testdb.DriverName, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE t (a INTEGER)`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAnArmedFaultFailsTheMatchingStatement(t *testing.T) {
	db := open(t)
	tripped := testdb.Arm("INSERT INTO t", errBoom)
	defer tripped()

	if _, err := db.Exec(`INSERT INTO t (a) VALUES (1)`); !errors.Is(err, errBoom) {
		t.Fatalf("the armed statement returned %v, want boom — nothing was injected, so "+
			"every test built on this driver would exercise its happy path", err)
	}
}

func TestAFaultLeavesOtherStatementsAlone(t *testing.T) {
	// The control. A driver that failed everything would satisfy the test above
	// while making the code under test fail for a reason no caller could ever
	// hit, which is a different kind of nothing.
	db := open(t)
	tripped := testdb.Arm("DELETE FROM t", errBoom)
	defer tripped()

	if _, err := db.Exec(`INSERT INTO t (a) VALUES (1)`); err != nil {
		t.Fatalf("a statement that does not match the fault failed anyway: %v", err)
	}
}

func TestAFaultIsOneShot(t *testing.T) {
	// Otherwise the fault leaks into whatever the code does NEXT — usually its
	// own error handling, or a rollback — and the test observes a failure two
	// steps from the one it armed.
	db := open(t)
	tripped := testdb.Arm("INSERT INTO t", errBoom)
	defer tripped()

	if _, err := db.Exec(`INSERT INTO t (a) VALUES (1)`); !errors.Is(err, errBoom) {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO t (a) VALUES (2)`); err != nil {
		t.Fatalf("the fault fired twice: %v", err)
	}
}

func TestTrippedReportsWhetherTheFaultWasReached(t *testing.T) {
	// The guard against the quietest failure of all: a fault armed on SQL that
	// the code never runs. Without this, such a test asserts nothing and looks
	// exactly like one that works.
	db := open(t)

	tripped := testdb.Arm("INSERT INTO t", errBoom)
	_, _ = db.Exec(`INSERT INTO t (a) VALUES (1)`)
	if !tripped() {
		t.Fatal("tripped() said the fault was never reached, but it fired")
	}

	unreached := testdb.Arm("NO SUCH STATEMENT ANYWHERE", errBoom)
	if _, err := db.Exec(`INSERT INTO t (a) VALUES (2)`); err != nil {
		t.Fatal(err)
	}
	if unreached() {
		t.Fatal("tripped() claimed a fault fired that nothing could have matched")
	}
}

func TestQueriesAreInterceptedToo(t *testing.T) {
	// database/sql routes reads through QueryerContext, a different method from
	// the one Exec uses. Wrapping only one of them leaves half the error arms
	// in the codebase unreachable, silently.
	db := open(t)
	tripped := testdb.Arm("SELECT a FROM t", errBoom)
	defer tripped()

	var a int
	if err := db.QueryRow(`SELECT a FROM t`).Scan(&a); !errors.Is(err, errBoom) {
		t.Fatalf("an armed query returned %v, want boom", err)
	}
}

func TestPragmaStatementsCanBeFaulted(t *testing.T) {
	// PRAGMA is how the identity migration reads a table's shape, and it goes
	// through the query path like any other read. Named separately because it
	// is the specific statement the first user of this package needs to fail.
	db := open(t)
	tripped := testdb.Arm("PRAGMA table_info", errBoom)
	defer tripped()

	if _, err := db.Query(`PRAGMA table_info(t)`); !errors.Is(err, errBoom) {
		t.Fatalf("an armed PRAGMA returned %v, want boom", err)
	}
}

func TestPreparedStatementsAreInterceptedToo(t *testing.T) {
	// database/sql prefers ExecerContext and QueryerContext, so a driver that
	// wrapped only those would look complete — until code that prepares a
	// statement slipped past the fault and the test asserted a failure that
	// never happened. Reached explicitly here, because nothing else does.
	db := open(t)
	tripped := testdb.Arm("INSERT INTO t", errBoom)
	defer tripped()

	if _, err := db.Prepare(`INSERT INTO t (a) VALUES (?)`); !errors.Is(err, errBoom) {
		t.Fatalf("preparing an armed statement returned %v, want boom — a fault can be "+
			"bypassed by preparing", err)
	}
}

func TestUnarmedTrafficPassesThroughUntouched(t *testing.T) {
	// The wrapper must be invisible when nothing is armed: every delegation
	// path — exec, query, prepare — has to reach the real driver and return
	// real results, or the tests built on it are testing the wrapper.
	db := open(t)
	if _, err := db.Exec(`INSERT INTO t (a) VALUES (7)`); err != nil {
		t.Fatal(err)
	}
	var a int
	if err := db.QueryRow(`SELECT a FROM t`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if a != 7 {
		t.Fatalf("read back %d, want 7 — the wrapper is not delegating", a)
	}
	stmt, err := db.Prepare(`SELECT a FROM t WHERE a = ?`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if err := stmt.QueryRow(7).Scan(&a); err != nil || a != 7 {
		t.Fatalf("prepared read gave %d (%v), want 7", a, err)
	}
}

func TestAConnectionThatCannotBeOpenedIsReported(t *testing.T) {
	// The wrapper's own failure arm: it must pass a real open failure through
	// rather than returning a connection that wraps nothing.
	db, err := sql.Open(testdb.DriverName, filepath.Join(t.TempDir(), "no-such-dir", "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err == nil {
		t.Fatal("opening a database under a directory that does not exist succeeded")
	}
}
