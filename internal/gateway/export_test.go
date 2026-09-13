package gateway

import (
	"sort"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Test hooks for the three hand-written lists that authorization depends on:
// commandName's switch, commandRoles, and playerRules.
//
// They are exported only under `go test` (this file's _test.go suffix keeps it
// out of the built package), and they exist so authz_test.go — which lives in
// package gateway_test, deliberately, so it exercises the same surface a real
// caller sees — can check that none of them has fallen behind. Two mechanisms,
// and they are not the same: commandName and commandRoles are checked by
// REFLECTING over the ClientCommand oneof, while playerRules is checked by
// DIFFING it against commandRoles, because the question there is not "does the
// contract have this command" but "did anyone decide what a player may do with
// it".
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

// PlayerCommandsForTest lists every command commandRoles lets a PLAYER issue.
func PlayerCommandsForTest() []string {
	var out []string
	for name, roles := range commandRoles {
		if roles[identity.RolePlayer] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// PlayerRuleCommandsForTest lists every command playerRules decides.
func PlayerRuleCommandsForTest() []string {
	var out []string
	for name := range playerRules {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// AuthorizeUndecidedForTest runs the player half against a name that has a role
// cell and no rule, which is the state the fall-through used to permit. It
// cannot be reached through Authorize with the tables as they stand — every
// player cell has a rule, which is the point — so the default arm would
// otherwise be a guard nobody drives.
func AuthorizeUndecidedForTest(p *identity.Participant, name string) error {
	return authorizePlayer(p, nil, nil, name)
}
