package gateway

import vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"

// Test hooks for the two hand-written lists that authorization depends on.
//
// They are exported only under `go test` (this file's _test.go suffix keeps it
// out of the built package), and they exist so authz_test.go — which lives in
// package gateway_test, deliberately, so it exercises the same surface a real
// caller sees — can reflect over the ClientCommand oneof and check that
// neither list has fallen behind it.
//
// The alternative was moving that gate into an internal test, which would have
// let it drift from the external suite that pins everything else about
// Authorize.

// CommandNameForTest exposes commandName's mapping from a oneof arm to the
// string commandRoles is keyed by.
func CommandNameForTest(cmd *vttv1.ClientCommand) string { return commandName(cmd) }

// HasRoleCellsForTest reports whether any role at all may issue name. A
// command missing from commandRoles is denied to everyone.
func HasRoleCellsForTest(name string) bool {
	// len, not comma-ok. A `"foo": {}` entry EXISTS while permitting nobody, so
	// the comma-ok form certified a command no role may issue — the very state
	// this gate was added to catch.
	return len(commandRoles[name]) > 0
}
