package gateway

import (
	"database/sql"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// VTT-894 VTT-926
func TestAGrantIsRefusedWhenIdentityCannotReadAParticipant(t *testing.T) {
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = camp.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	raw, err := sql.Open("sqlite", campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	s := New(camp, ids)
	mint := func(name string, role identity.Role) string {
		_, id, err := ids.CreateInvite(name, role)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	dm, broken := mint("Dot", identity.RoleDM), mint("Bram", identity.RoleSpectator)
	if _, err := raw.Exec(`UPDATE participants SET role = 'nobody' WHERE id = ?`, broken); err != nil {
		t.Fatal(err)
	}
	st := engine.NewState()
	st.Actors["pm-1"] = &vttv1.Actor{ActorId: "pm-1", Name: "Ash", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		ControllerIds: []string{broken}}
	grant := func(to string, kind vttv1.ActorKind) *vttv1.ClientCommand {
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "pm-1", ParticipantId: to, Kind: kind}}}
	}
	for _, c := range []struct {
		name string
		cmd  *vttv1.ClientCommand
	}{
		{"the grantee unreadable", grant(broken, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)},
		{"a holder unreadable", grant(dm, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)},
	} {
		res := s.granteeRefusal("r", c.cmd, st)
		if res == nil || res.GetOk() || !strings.Contains(res.GetError(), "stored role invalid") {
			t.Errorf("%s: answered %v, want identity's refusal naming the stored role", c.name, res)
		}
	}
}
