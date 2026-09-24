package gateway_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func qaPromote(id, role string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		RequestId: "qa-promote-" + role,
		Command: &vttv1.ClientCommand_PromoteParticipant{PromoteParticipant: &vttv1.PromoteParticipant{
			ParticipantId: id, Role: role,
		}},
	}
}

// VTT-036
func TestQAAPromotionLeavesTheLogHeadWhereItWas(t *testing.T) {
	cases := []struct {
		name            string
		issuer          func(f *gwFixture) string
		targetConnected bool
	}{
		{"dm promotes an absent spectator", func(f *gwFixture) string { return f.dmToken }, false},
		{"agent promotes a connected spectator", func(f *gwFixture) string { return f.agentToken }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newGWFixture(t)
			spec, err := f.ids.Verify(f.spectatorToken)
			if err != nil {
				t.Fatalf("fixture: Verify spectator: %v", err)
			}
			if spec.Role != identity.RoleSpectator {
				t.Fatalf("fixture: spectator resolves to role %q", spec.Role)
			}
			if tc.targetConnected {
				target := f.dial(f.spectatorToken, 0)
				expectCatchUpHead(t, target)
				expectPresenceSnapshot(t, target)
			}
			conn := f.dial(tc.issuer(f), 0)
			expectCatchUpHead(t, conn)
			expectPresenceSnapshot(t, conn)

			before := f.head(t)
			if before == 0 {
				t.Fatal("fixture: an empty log; the head comparison would be degenerate")
			}

			sendCommand(t, conn, qaPromote(spec.ID, string(identity.RolePlayer)))
			res := readResult(t, conn)
			if !res.GetOk() {
				t.Fatalf("promotion refused: %q", res.GetError())
			}
			// The promotion really happened: otherwise an unchanged head
			// would be the head of a command that did nothing.
			now, err := f.ids.Lookup(spec.ID)
			if err != nil {
				t.Fatalf("Lookup promoted: %v", err)
			}
			if now.Role != identity.RolePlayer {
				t.Fatalf("after promotion the participant's role is %q, want %q", now.Role, identity.RolePlayer)
			}

			if after := f.head(t); after != before {
				t.Errorf("log head moved %d -> %d across a promotion; VTT-036 says it appends no event", before, after)
			}
		})
	}
}

// VTT-036 control: the same head probe DOES see an appending command from the
// same connection, so its silence above is evidence.
func TestQATheHeadProbeSeesAnAppend(t *testing.T) {
	f := newGWFixture(t)
	conn := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, conn)
	expectPresenceSnapshot(t, conn)
	before := f.head(t)
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "qa-narrate",
		Command:   &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: "The door creaks."}},
	})
	if res := readResult(t, conn); !res.GetOk() {
		t.Fatalf("add_narration refused: %q", res.GetError())
	}
	if after := f.head(t); after != before+1 {
		t.Errorf("head %d -> %d across add_narration, want +1: the probe does not observe appends", before, after)
	}
}
