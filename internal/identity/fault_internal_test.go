package identity

import (
	"errors"
	"os"
	"path/filepath"
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

// A withControlColumnCampaign fixture and FOUR tests over it used to sit here,
// arming faults on the migration that dropped participants.controls: the DROP
// itself, the participants PRAGMA before the lock, the same PRAGMA under it,
// and a no-fault control proving the fixture's column really went. That
// migration was removed on 2026-08-24 — no campaign is in use, so it guarded no
// data while charging every existing campaign one writable open — and none of
// those statements exists any more. Run unchanged against the removal, all four
// failed: three on "the fault was never reached", the fourth because the column
// is still selectable afterwards. There is nothing left for them to arm.
//
// The under-lock coverage they carried between them was the PARTICIPANTS shape
// read's error arm, and it went with the code it guarded — so it cost nothing
// that survives. Not to be confused with migrateLocked's join_access re-read,
// which has an error arm of its own that was ALREADY uncovered at HEAD (profile
// `287.16,289.3 1 0`) and has never been reachable through testdb at all, for
// the reason stated at that arm.
//
// What this change DID cost on a surviving line came from identity_test.go
// rather than from here, and it belonged to the joining-a-table arc:
// TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign below re-pins it.

func TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign(t *testing.T) {
	// The door repair is the migration's one DATA write, and it is the arm that
	// lost its witness on 2026-08-24: the only test that reached it was
	// TestAReadOnlyCampaignStillCarryingTheControlColumnWillNotOpen, deleted
	// with the controls migration, which hit it by accident rather than on
	// purpose. On read-only media BEGIN IMMEDIATE does not fail (SQLite defers
	// the write lock) and that fixture already had both budget columns, so this
	// UPDATE was the first statement that actually tried to write. Measured:
	// identity.go's arm read `1 1` at HEAD and `1 0` after the deletion.
	//
	// It belongs to the joining-a-table arc, not to this one, which is why it is
	// re-pinned deliberately instead of being left to the coverage floor's
	// slack. Swallowing this error leaves an open door budgeted at 0, and 0
	// admits nobody — the DM shares a link that reads open and turns everyone
	// away, with no error anywhere to say why.
	withFaultDriver(t)
	// A PRE-BUDGET campaign, so migrationPending answers yes on the missing
	// column BEFORE its own budget SELECT, leaving the single armed fault
	// unspent for the UPDATE under the lock.
	path := preBudgetCampaign(t)

	// The UPDATE's own prefix. `WHERE open = 1 AND admit_limit = 0` would match
	// migrationPending's SELECT first and spend the fault before the lock.
	tripped := testdb.Arm("UPDATE join_access SET admit_limit", errDBDown)
	d, err := Open(path)
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if err == nil {
		d.Close()
		t.Fatal("a migration that could not budget an already-open door opened anyway — " +
			"the door still reads open with a budget of 0, so it refuses every joiner " +
			"the DM sent the link to, and nothing reported the failed write")
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
	// The fixture above is load-bearing for FIVE tests: if DROP COLUMN quietly
	// stopped working, they would all arm faults against a migration that had
	// nothing to do, and all pass while proving nothing. (Four until
	// 2026-08-24; the budget-repair test added that day made it five. The count
	// is written out because it is the kind of number that rots silently — it
	// already read four while five tests used the fixture.)
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
