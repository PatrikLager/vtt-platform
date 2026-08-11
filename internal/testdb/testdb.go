// Package testdb is a SQLite driver that can be told to fail one statement.
//
// TEST-ONLY. Nothing in cmd/ or the server imports it, so it never reaches a
// shipped binary — but it is a normal package rather than a _test.go file
// because more than one package needs it, and Go cannot import test files.
//
// WHY IT EXISTS. Every store this repo has is a SQLite file, and every method
// on one ends in `if err != nil { return fmt.Errorf(...) }`. Those arms decide
// what happens when a database fails mid-operation — whether a refused join
// admits anyway, whether a half-applied migration returns a usable handle,
// whether a promotion the DM believes happened silently did not. They are the
// arms that matter most and the only ones no test could reach: a real SQLite
// file does not fail on demand, so the choice was between leaving them
// untested and pretending a closed-database handle exercises the same paths.
// It does not — closing a handle fails the FIRST statement, so everything past
// it stays dark.
//
// Patrik's call, 2026-08-11, over lowering internal/identity's coverage floor
// to accommodate 15 such arms added by the admission budget.
package testdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // the driver being wrapped
)

// DriverName is what to hand sql.Open to get a fault-injecting connection.
const DriverName = "sqlite-fault"

var (
	mu    sync.Mutex
	armed *fault
)

type fault struct {
	match string
	err   error
}

// Arm makes the next statement whose SQL CONTAINS match fail with err.
//
// One-shot: it disarms itself when it trips, so a fault cannot leak into the
// statement after the one under test. Matching on the SQL rather than counting
// statements is deliberate — a count is a promise about every statement the
// driver happens to run first, including the schema and any PRAGMA, and it
// breaks the moment an unrelated query is added anywhere upstream.
//
// Returns a function that disarms and reports whether it tripped. A test that
// arms a fault nothing reaches is a test that proved nothing, and this is how
// it finds out.
func Arm(match string, err error) (tripped func() bool) {
	mu.Lock()
	defer mu.Unlock()
	f := &fault{match: match, err: err}
	armed = f
	return func() bool {
		mu.Lock()
		defer mu.Unlock()
		fired := armed != f // it disarmed itself by firing
		armed = nil
		return fired
	}
}

// trip reports the armed error if q matches, disarming as it does.
func trip(q string) error {
	mu.Lock()
	defer mu.Unlock()
	if armed == nil || !strings.Contains(q, armed.match) {
		return nil
	}
	err := armed.err
	armed = nil
	return err
}

func init() {
	// The wrapped driver is obtained from a throwaway handle rather than
	// constructed: modernc registers its driver value privately, and this is
	// the only way to reach it without depending on an internal symbol.
	probe, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic("testdb: cannot reach the sqlite driver: " + err.Error())
	}
	base := probe.Driver()
	_ = probe.Close()
	sql.Register(DriverName, &faultDriver{base: base})
}

type faultDriver struct{ base driver.Driver }

func (d *faultDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &faultConn{Conn: c}, nil
}

// faultConn intercepts the context-aware paths, which is what database/sql
// prefers when the driver offers them — and modernc's does. The Prepare path
// is wrapped too, because a statement reached through it would otherwise slip
// past the fault entirely and the test would pass for the wrong reason.
type faultConn struct{ driver.Conn }

func (c *faultConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := trip(q); err != nil {
		return nil, err
	}
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return e.ExecContext(ctx, q, args)
}

func (c *faultConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := trip(q); err != nil {
		return nil, err
	}
	qr, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return qr.QueryContext(ctx, q, args)
}

func (c *faultConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	if err := trip(q); err != nil {
		return nil, err
	}
	p, ok := c.Conn.(driver.ConnPrepareContext)
	if !ok {
		// Every driver.Conn has Prepare; only the context-aware form is
		// optional. modernc offers both, so this arm is unreached today and
		// exists so a future swap of the wrapped driver cannot silently lose
		// the interception.
		return c.Prepare(q)
	}
	return p.PrepareContext(ctx, q)
}
