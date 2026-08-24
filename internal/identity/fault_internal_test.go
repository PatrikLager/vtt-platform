package identity

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/testdb"
)

// These are the arms that decide what happens when the database fails
// MID-OPERATION, and until internal/testdb existed nothing could reach them.
//
// Closing a handle is not a substitute and the difference is the whole point:
// a closed handle fails the FIRST statement, so a migration's BEGIN, its ALTER,
// its COMMIT, and JoinAdmits' UPDATE all stay dark behind the SELECT that
// failed ahead of them. Those later arms are exactly the interesting ones —
// they are where a half-applied migration or an admission spent against a
// failed write would come from.
//
// INTERNAL (package identity), because driverName is unexported. Each test
// restores it, and none of these may run in parallel: the driver name and the
// armed fault are both process-global.

var errDBDown = errors.New("testdb: injected failure")

// withFaultDriver points Open at the fault-injecting wrapper for one test.
func withFaultDriver(t *testing.T) {
	t.Helper()
	prev := driverName
	driverName = testdb.DriverName
	t.Cleanup(func() { driverName = prev })
}

// preBudgetCampaign writes a campaign in the shape that shipped BEFORE the
// admission budget, so opening it genuinely requires a migration.
func preBudgetCampaign(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.JoinSecret(); err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 4); err != nil {
		t.Fatal(err)
	}
	// Drop the budget columns back off. SQLite can drop a column, and this is
	// closer to the real thing than hand-writing the old schema: the rest of
	// the database is exactly what this code produces.
	for _, col := range []string{"admitted", "admit_limit"} {
		if _, err := d.db.Exec(`ALTER TABLE join_access DROP COLUMN ` + col); err != nil {
			t.Fatalf("dropping %s: %v", col, err)
		}
	}
	d.Close()
	return path
}

// withControlColumnCampaign writes a campaign in the shape that shipped BEFORE
// participants.controls was deleted, so opening it genuinely requires the drop.
//
// Same construction as preBudgetCampaign and for the same reason: the rest of
// the database is exactly what this code produces, so the fixture cannot drift
// away from the real thing in some other column.
func withControlColumnCampaign(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "controls.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`ALTER TABLE participants ADD COLUMN controls TEXT`); err != nil {
		t.Fatalf("re-adding the control column: %v", err)
	}
	// CONFIRMED HERE, not in the tests over it. "The column is not selectable"
	// is the shape every assertion downstream takes, and it reads identically
	// for a column that was dropped and for one that was never added — so a
	// fixture that quietly stopped working would make those tests pass rather
	// than fail. This is the one place the difference is observable.
	rows, err := d.db.Query(`SELECT controls FROM participants`)
	if err != nil {
		t.Fatalf("the fixture did not add a control column: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	d.Close()
	return path
}

func TestAMigrationThatCannotDropTheControlColumnRefusesTheCampaign(t *testing.T) {
	// Refusing rather than opening anyway, and the reason differs from the
	// budget's. A missing column breaks the next statement loudly; a column
	// that FAILED to go on stays readable, so a handle returned here would be a
	// campaign that still carries the second record of control while the code
	// believes it removed it — the exact state this deletion exists to end.
	withFaultDriver(t)
	path := withControlColumnCampaign(t)

	tripped := testdb.Arm("DROP COLUMN controls", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a migration that could not drop participants.controls opened anyway, " +
			"leaving a campaign whose schema still records control twice")
	}
}

func TestAMigrationThatCannotReadTheParticipantShapeRefusesTheCampaign(t *testing.T) {
	// The participants PRAGMA specifically. `PRAGMA table_info` alone matches
	// the join_access read first and would prove only what its own test
	// already does, so this arms the participants query by name.
	//
	// This is migrationPending's read, BEFORE any lock — asserted by the
	// absence of the "migrate:" wrap, which only migrateLocked adds. Without
	// that check this test and its sibling below say the same thing and neither
	// notices when one of them stops reaching the arm it names.
	withFaultDriver(t)
	path := withControlColumnCampaign(t)

	tripped := testdb.Arm("PRAGMA table_info(participants)", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a campaign whose participants shape could not be read opened anyway — " +
			"the migration decided the column was gone from an answer it never got")
	}
	if strings.Contains(err.Error(), "identity: migrate:") {
		t.Fatalf("this failed UNDER THE LOCK (%v) — the pre-check read is what this test "+
			"names, and it is no longer the one that trips", err)
	}
}

func TestTheParticipantShapeFailingUnderTheLockRefusesTheCampaign(t *testing.T) {
	// The SAME read, in the other place it happens. migrateLocked re-reads the
	// shape inside the transaction because a concurrent opener may have
	// finished the whole migration while this one waited for the write lock,
	// and that re-read has its own failure arm — one the test above cannot
	// reach, because migrationPending's read comes first and swallows the
	// single armed fault.
	//
	// A PRE-BUDGET campaign is what separates them: its missing admitted column
	// makes migrationPending answer "yes" before it ever asks about
	// participants, so the fault survives to the read under the lock. That
	// separation is an ORDERING assumption about another function, so the
	// "migrate:" wrap is checked rather than trusted — reorder migrationPending's
	// two reads and this test says so instead of passing on the wrong arm.
	withFaultDriver(t)
	path := preBudgetCampaign(t)

	tripped := testdb.Arm("PRAGMA table_info(participants)", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a migration whose second shape read failed opened anyway, so whether the " +
			"control column is gone was decided from an answer nobody got")
	}
	if !strings.Contains(err.Error(), "identity: migrate:") {
		t.Fatalf("the fault tripped BEFORE the lock (%v) — this test names migrateLocked's "+
			"re-read, and migrationPending's read is swallowing it instead", err)
	}
}

func TestTheControlColumnFixtureMigratesCleanlyWhenNothingFails(t *testing.T) {
	// The same guard TestPreBudgetFixtureReallyDropsTheColumns puts on its
	// fixture, and the POSITIVE path over it: with no fault armed, opening this
	// campaign must leave the column gone. Without this the fixture could stop
	// adding the column and the two fault tests would still fail loudly on
	// !tripped(), but nothing would say the drop itself works against a
	// database this code produced — identity_test.go's hand-written old-shape
	// table is the only other witness, and one construction agreeing with
	// itself is not two.
	withFaultDriver(t)
	path := withControlColumnCampaign(t)

	d, err := Open(path)
	if err != nil {
		t.Fatalf("the fixture produced a campaign that will not migrate: %v", err)
	}
	defer d.Close()

	if _, err := d.db.Query(`SELECT controls FROM participants`); err == nil {
		t.Fatal("participants.controls is still selectable after the migration — the " +
			"fixture added no column, or the drop did not run")
	}
}

func TestAMigrationThatCannotStartRefusesTheCampaign(t *testing.T) {
	withFaultDriver(t)
	path := preBudgetCampaign(t)

	tripped := testdb.Arm("BEGIN IMMEDIATE", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a campaign whose migration could not start opened anyway — every join " +
			"against it fails on a missing column and reads as a broken link")
	}
}

func TestAMigrationThatCannotCommitRefusesTheCampaign(t *testing.T) {
	withFaultDriver(t)
	path := preBudgetCampaign(t)

	tripped := testdb.Arm("COMMIT", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a migration that could not commit returned a usable handle")
	}
}

func TestAMigrationThatCannotAddAColumnRefusesTheCampaign(t *testing.T) {
	withFaultDriver(t)
	path := preBudgetCampaign(t)

	tripped := testdb.Arm("ADD COLUMN admit_limit", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a migration that could not add a column opened anyway, leaving a campaign " +
			"with half the budget schema")
	}
}

func TestAMigrationThatCannotReadTheShapeRefusesTheCampaign(t *testing.T) {
	withFaultDriver(t)
	path := preBudgetCampaign(t)

	tripped := testdb.Arm("PRAGMA table_info", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a campaign whose schema could not be read opened anyway — the migration " +
			"decided there was nothing to do from an answer it never got")
	}
}

func TestAMigrationThatCannotReadTheBudgetStateRefusesTheCampaign(t *testing.T) {
	withFaultDriver(t)
	path := filepath.Join(t.TempDir(), "current.db")
	d, err := Open(path) // already current, so migrationPending takes its read path
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	tripped := testdb.Arm("WHERE open = 1 AND admit_limit = 0", errDBDown)
	again, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		again.Close()
		t.Fatal("a campaign whose budget state could not be read opened anyway — an open " +
			"door with no budget would then never be repaired, and refuse everyone")
	}
}

func TestAnAdmissionThatCannotBeSpentIsNotGranted(t *testing.T) {
	// The arm that matters most in this file. The SELECT says there is room;
	// the UPDATE that spends the slot fails. Returning true here would admit a
	// participant whose admission was never recorded — so the budget would not
	// move, and the door would mint without limit exactly as it did before #42.
	withFaultDriver(t)
	path := filepath.Join(t.TempDir(), "spend.db")
	d, err := Open(path)
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

	tripped := testdb.Arm("SET admitted = admitted + 1", errDBDown)
	ok, err := d.JoinAdmits(secret)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if ok {
		t.Fatal("a joiner was admitted on a write that failed — the admission is not " +
			"recorded, so the budget never moves and the door mints without limit")
	}
	if err == nil {
		t.Fatal("a failed spend was reported as a plain refusal, so a broken database is " +
			"indistinguishable from a shut door in the logs")
	}
}

func TestTheDoorStateReadFailingIsNotAnAdmission(t *testing.T) {
	withFaultDriver(t)
	path := filepath.Join(t.TempDir(), "read.db")
	d, err := Open(path)
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

	tripped := testdb.Arm("SELECT secret, open, admitted, admit_limit", errDBDown)
	ok, err := d.JoinAdmits(secret)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if ok || err == nil {
		t.Fatalf("JoinAdmits returned (%v, %v) when it could not read the door — a "+
			"database that cannot answer must never be able to open one", ok, err)
	}
}

func TestOpeningWithAnUnusableDriverIsReported(t *testing.T) {
	// Open's own error arm, which nothing else reaches: sql.Open fails before
	// any statement runs, so no fault can be armed for it.
	prev := driverName
	driverName = "no-such-driver-anywhere"
	t.Cleanup(func() { driverName = prev })

	if d, err := Open(filepath.Join(t.TempDir(), "x.db")); err == nil {
		d.Close()
		t.Fatal("Open succeeded against a driver that is not registered")
	}
}

func TestPreBudgetFixtureReallyDropsTheColumns(t *testing.T) {
	// The fixture above is load-bearing for four tests: if DROP COLUMN quietly
	// stopped working, they would all arm faults against a migration that had
	// nothing to do, and all pass while proving nothing.
	withFaultDriver(t)
	path := preBudgetCampaign(t)

	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		t.Fatalf("fixture wrote no database: %v", err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatalf("the fixture produced a campaign that will not migrate: %v", err)
	}
	defer d.Close()
	// It migrated, which means there WAS something to migrate.
	admitted, limit, err := d.JoinBudget()
	if err != nil {
		t.Fatal(err)
	}
	if limit != DefaultAdmitLimit || admitted != 0 {
		t.Fatalf("the migrated fixture reports %d/%d, want 0/%d — the door was open, so "+
			"the repair should have budgeted it", admitted, limit, DefaultAdmitLimit)
	}
}

func TestASpentBudgetRefusesWithoutTouchingTheDatabase(t *testing.T) {
	// The inertness property, and until internal/testdb existed there was no
	// way to observe it. Spec §2's case against rate limiting is that a
	// refused, anonymous, unauthenticated request performs NO WRITE — because
	// even an UPDATE matching zero rows takes SQLite's write lock on the file
	// internal/store appends events to, inside a transaction, on the one path a
	// stranger controls.
	//
	// The mutation gate found it: `admitted >= budget` mutated to `>` gives the
	// caller the identical answer (the UPDATE's own WHERE still refuses) while
	// reaching for the write. Nothing could tell the difference, so nothing did.
	// Here the fault is armed on the UPDATE and must NOT fire.
	withFaultDriver(t)
	path := filepath.Join(t.TempDir(), "spent.db")
	d, err := Open(path)
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
	if ok, err := d.JoinAdmits(secret); !ok {
		t.Fatalf("the one admission was refused: %v", err)
	}

	reached := testdb.Arm("SET admitted = admitted + 1", errDBDown)
	ok, err := d.JoinAdmits(secret)
	if reached() {
		t.Fatal("a refusal against a spent budget reached the UPDATE — it takes SQLite's " +
			"write lock on the campaign file, so a stranger hammering a spent door " +
			"contends with every event append, which is the whole of §2's case against " +
			"rate limiting")
	}
	if ok || err != nil {
		t.Fatalf("a spent budget answered (%v, %v), want (false, nil)", ok, err)
	}
}

func TestAWrongSecretRefusesWithoutTouchingTheDatabase(t *testing.T) {
	// The same property on the path a prober actually uses. Separate from the
	// spent-budget case because they refuse for different reasons and a guard
	// covering one says nothing about the other.
	withFaultDriver(t)
	path := filepath.Join(t.TempDir(), "wrong.db")
	d, err := Open(path)
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

	reached := testdb.Arm("SET admitted = admitted + 1", errDBDown)
	ok, err := d.JoinAdmits("not-the-secret")
	if reached() {
		t.Fatal("a wrong secret reached the UPDATE — an anonymous guess must not be able " +
			"to take the campaign file's write lock")
	}
	if ok || err != nil {
		t.Fatalf("a wrong secret answered (%v, %v), want (false, nil)", ok, err)
	}
}
