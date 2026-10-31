package identity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/testdb"
)

// Do not run these tests in parallel: driverName and the armed fault are both
// process-global. Do not swap the fault driver for a closed handle: that fails
// the first statement and leaves every later arm dark.

var errDBDown = errors.New("testdb: injected failure")

func withFaultDriver(t *testing.T) {
	t.Helper()
	prev := driverName
	driverName = testdb.DriverName
	t.Cleanup(func() { driverName = prev })
}

// Keep this pre-budget: the migration tests need something to migrate.
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
	// Drop the columns rather than hand-write the old schema: the rest of the file
	// must be exactly what this code produces.
	for _, col := range []string{"admitted", "admit_limit"} {
		if _, err := d.db.Exec(`ALTER TABLE join_access DROP COLUMN ` + col); err != nil {
			t.Fatalf("dropping %s: %v", col, err)
		}
	}
	d.Close()
	return path
}

// Arms the one fault on the migration's door-repair UPDATE, under the lock.
// VTT-060
func TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign(t *testing.T) {
	// Keep this test: swallowing the repair's error leaves an open door budgeted
	// at 0, which admits nobody and says nothing.
	withFaultDriver(t)
	// Keep it pre-budget, or no migration is pending and the repair never runs.
	path := preBudgetCampaign(t)

	// Keep the armed text the UPDATE's own: testdb matches by substring, and the
	// one fault must reach the statement under the lock.
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

// VTT-060
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

// VTT-060
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

// VTT-060
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

// VTT-060
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

// VTT-060
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

// VTT-045
func TestAnAdmissionThatCannotBeSpentIsNotGranted(t *testing.T) {
	// Keep this test: returning true here admits a participant whose admission
	// was never recorded, so the budget never moves.
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
	ok, err := d.JoinAdmits(secret, "Newcomer")
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

// VTT-046
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
	ok, err := d.JoinAdmits(secret, "Newcomer")
	if !tripped() {
		t.Fatal("the fault was never reached — this test proved nothing")
	}
	if ok || err == nil {
		t.Fatalf("JoinAdmits returned (%v, %v) when it could not read the door — a "+
			"database that cannot answer must never be able to open one", ok, err)
	}
}

// VTT-071
func TestOpeningWithAnUnusableDriverIsReported(t *testing.T) {
	// Do not arm a fault here: sql.Open fails before any statement runs.
	prev := driverName
	driverName = "no-such-driver-anywhere"
	t.Cleanup(func() { driverName = prev })

	if d, err := Open(filepath.Join(t.TempDir(), "x.db")); err == nil {
		d.Close()
		t.Fatal("Open succeeded against a driver that is not registered")
	}
}

// VTT-063
func TestPreBudgetFixtureReallyDropsTheColumns(t *testing.T) {
	// Keep this control: if DROP COLUMN quietly stopped working, every test on
	// preBudgetCampaign would arm a fault against a migration with nothing to do.
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
	admitted, limit, err := d.JoinBudget()
	if err != nil {
		t.Fatal(err)
	}
	if limit != DefaultAdmitLimit || admitted != 0 {
		t.Fatalf("the migrated fixture reports %d/%d, want 0/%d — the door was open, so "+
			"the repair should have budgeted it", admitted, limit, DefaultAdmitLimit)
	}
}

// VTT-006
func TestASpentBudgetRefusesWithoutTouchingTheDatabase(t *testing.T) {
	// Keep the fault on the UPDATE and require it NOT to fire: `admitted >= budget`
	// mutated to `>` answers the same while reaching for the write lock.
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
	if ok, err := d.JoinAdmits(secret, "Newcomer"); !ok {
		t.Fatalf("the one admission was refused: %v", err)
	}

	reached := testdb.Arm("SET admitted = admitted + 1", errDBDown)
	ok, err := d.JoinAdmits(secret, "Newcomer")
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

// VTT-005
func TestAWrongSecretRefusesWithoutTouchingTheDatabase(t *testing.T) {
	// Keep this separate from the spent-budget case: a guard covering one
	// refusal says nothing about the other.
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
	ok, err := d.JoinAdmits("not-the-secret", "Newcomer")
	if reached() {
		t.Fatal("a wrong secret reached the UPDATE — an anonymous guess must not be able " +
			"to take the campaign file's write lock")
	}
	if ok || err != nil {
		t.Fatalf("a wrong secret answered (%v, %v), want (false, nil)", ok, err)
	}
}

// VTT-008
func TestAShutDoorRefusesWithoutTouchingTheDatabase(t *testing.T) {
	// Keep the secret right and the budget untouched, so the door is the only
	// term refusing: a guard that lost it reaches the write.
	withFaultDriver(t)
	path := filepath.Join(t.TempDir(), "shut.db")
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
	if err := d.SetJoinOpen(false, 5); err != nil {
		t.Fatal(err)
	}

	reached := testdb.Arm("SET admitted = admitted + 1", errDBDown)
	ok, err := d.JoinAdmits(secret, "Newcomer")
	if reached() {
		t.Fatal("a shut door reached the UPDATE — a refused request must not take " +
			"SQLite's write lock on the file internal/store appends events to")
	}
	if ok || err != nil {
		t.Fatalf("a shut door answered (%v, %v), want (false, nil)", ok, err)
	}
}

const namesRead = "SELECT display_name FROM participants WHERE revoked = 0"

func kimHeldAtAnOpenDoor(t *testing.T) (*DB, string, string) {
	t.Helper()
	d, path := qaFaultDB(t)
	if _, _, err := d.CreateInvite("Kim", RoleSpectator); err != nil {
		t.Fatal(err)
	}
	secret, err := d.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetJoinOpen(true, 5); err != nil {
		t.Fatal(err)
	}
	return d, path, secret
}

// VTT-545
func TestATakenNameAtTheDoorReachesNoWrite(t *testing.T) {
	d, path, secret := kimHeldAtAnOpenDoor(t)
	watch := qaWatch(t, path)
	before := watch.read()
	if hit := refuseUnderEachArm(t, func() (bool, error) { return d.JoinAdmits(secret, "kim") }); len(hit) != 0 {
		t.Fatalf("a join under a taken name reached %v", hit)
	}
	ok, err := d.JoinAdmits(secret, "kim")
	if ok || !errors.Is(err, ErrNameTaken) {
		t.Fatalf("JoinAdmits(kim) = (%v, %v), want (false, ErrNameTaken)", ok, err)
	}
	if after := watch.read(); after != before {
		t.Fatalf("data_version moved %d -> %d: a refused name wrote", before, after)
	}
	if admitted, _, err := d.JoinBudget(); err != nil || admitted != 0 {
		t.Fatalf("JoinBudget after a refused name: spent %d (%v), want 0", admitted, err)
	}
}

// VTT-009
func TestADoorThatRefusesNeverReadsTheNames(t *testing.T) {
	d, _, secret := kimHeldAtAnOpenDoor(t)
	for _, c := range []struct {
		label string
		set   func()
		offer string
	}{
		{"a wrong secret", func() {}, "not-the-secret"},
		{"a shut door", func() { _ = d.SetJoinOpen(false, 0) }, secret},
		{"a spent budget", func() {
			_ = d.SetJoinOpen(true, 1)
			if ok, err := d.JoinAdmits(secret, "Ann"); !ok || err != nil {
				t.Fatalf("fixture: spending the one admission = (%v, %v)", ok, err)
			}
		}, secret},
	} {
		c.set()
		tripped := testdb.Arm(namesRead, errDBDown)
		ok, err := d.JoinAdmits(c.offer, "kim")
		if tripped() || ok || err != nil {
			t.Fatalf("%s under a taken name: (%v, %v), names read reached; want (false, nil) unread",
				c.label, ok, err)
		}
	}
	if err := d.SetJoinOpen(true, 5); err != nil {
		t.Fatal(err)
	}
	tripped := testdb.Arm(namesRead, errDBDown)
	_, _ = d.JoinAdmits(secret, "kim")
	if !tripped() {
		t.Fatal("control: an open door with the current secret never read the names, so the refusals prove nothing")
	}
}

// VTT-046 VTT-070
func TestANamesReadThatFailsAdmitsAndMintsNobody(t *testing.T) {
	d, _, secret := kimHeldAtAnOpenDoor(t)
	tripped := testdb.Arm(namesRead, errDBDown)
	ok, err := d.JoinAdmits(secret, "Zed")
	if !tripped() {
		t.Fatal("the names read was never reached — this test proved nothing")
	}
	if ok || err == nil || errors.Is(err, ErrNameTaken) {
		t.Fatalf("JoinAdmits over a failed names read = (%v, %v), want (false, a read error)", ok, err)
	}
	if admitted, _, err := d.JoinBudget(); err != nil || admitted != 0 {
		t.Fatalf("JoinBudget after a failed names read: spent %d (%v), want 0", admitted, err)
	}

	tripped = testdb.Arm(namesRead, errDBDown)
	token, _, err := d.CreateInvite("Zed", RolePlayer)
	if !tripped() {
		t.Fatal("CreateInvite never read the names — this test proved nothing")
	}
	if err == nil || token != "" {
		t.Fatalf("CreateInvite over a failed names read = (%q, %v), want an error and no token", token, err)
	}
	if list, err := d.List(); err != nil || len(list) != 1 {
		t.Fatalf("List after a failed invite: %d participants (%v), want 1", len(list), err)
	}
}
