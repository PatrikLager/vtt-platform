package gateway

import (
	"errors"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func roleActors() map[string]*vttv1.Actor {
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	return map[string]*vttv1.Actor{
		"a-ash":   {ActorId: "a-ash", Name: "Ash", Kind: party, ControllerIds: []string{"p-1"}},
		"b-bram":  {ActorId: "b-bram", Name: "Bram", Kind: party, ControllerIds: []string{"p-1", "s-1", "d-1", "g-1", "x-1", "e-1"}},
		"c-cole":  {ActorId: "c-cole", Name: "Cole", Kind: party},
		"d-empty": {ActorId: "d-empty", Name: "Wisp", Kind: party, ControllerIds: []string{""}},
	}
}

// VTT-891 VTT-893
func TestRoleInJudgesAPlayerOrASpectatorByTheActorsAndNoOtherRole(t *testing.T) {
	actors := roleActors()
	for _, c := range []struct {
		what string
		p    identity.Participant
		want identity.Role
	}{
		{"a spectator who shares b-bram", identity.Participant{ID: "s-1", Name: "Robin", Role: identity.RoleSpectator}, identity.RolePlayer},
		{"a spectator who controls nothing", identity.Participant{ID: "s-2", Name: "Sol", Role: identity.RoleSpectator}, identity.RoleSpectator},
		{"a player who controls a-ash and b-bram", identity.Participant{ID: "p-1", Name: "Lera", Role: identity.RolePlayer}, identity.RolePlayer},
		{"a player who controls nothing", identity.Participant{ID: "p-9", Name: "Ivo", Role: identity.RolePlayer}, identity.RoleSpectator},
		{"an empty id beside an empty controller", identity.Participant{ID: "", Name: "Nobody", Role: identity.RoleSpectator}, identity.RoleSpectator},
		{"the DM, who shares b-bram", identity.Participant{ID: "d-1", Name: "Dee", Role: identity.RoleDM}, identity.RoleDM},
		{"the agent, who shares b-bram", identity.Participant{ID: "g-1", Name: "Gus", Role: identity.RoleAgent}, identity.RoleAgent},
		{"a role that is none of the four", identity.Participant{ID: "x-1", Name: "Max", Role: identity.Role("gm")}, identity.Role("gm")},
		{"an empty role", identity.Participant{ID: "e-1", Name: "Eve", Role: ""}, ""},
	} {
		p := c.p
		got := roleIn(&p, actors)
		if got.Role != c.want {
			t.Errorf("%s: judged %q, want %q", c.what, got.Role, c.want)
		}
		if got.ID != c.p.ID || got.Name != c.p.Name {
			t.Errorf("%s: judged as %+v, want the same participant %+v", c.what, got, c.p)
		}
		if p.Role != c.p.Role {
			t.Errorf("%s: the participant handed in now holds %q, want identity's %q untouched", c.what, p.Role, c.p.Role)
		}
	}
}

// VTT-891
func TestAuthorizeJudgesASpectatorWhoControlsAnActorAsAPlayer(t *testing.T) {
	st := engine.NewState()
	st.Actors = roleActors()
	st.Tokens["t-bram"] = engine.Token{ID: "t-bram", SceneID: "scn", ActorID: "b-bram"}
	move := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: "t-bram", To: &vttv1.GridPosition{X: 1, Y: 1}},
	}}
	robin := &identity.Participant{ID: "s-1", Name: "Robin", Role: identity.RoleSpectator}
	if err := Authorize(robin, move, st); err != nil {
		t.Fatalf("a spectator identity holds, who shares b-bram, was refused a move of its token: %v", err)
	}
	sol := &identity.Participant{ID: "s-2", Name: "Sol", Role: identity.RoleSpectator}
	err := Authorize(sol, move, st)
	if want := `gateway: not authorized: role "spectator" may not issue "move_token"`; !errors.Is(err, ErrUnauthorized) || err.Error() != want {
		t.Fatalf("a spectator who controls nothing moving t-bram: err = %v, want %q", err, want)
	}
}

// VTT-893
func TestMayWatchJudgesASpectatorWhoControlsAnActorAsAPlayer(t *testing.T) {
	actors := roleActors()
	robin := &identity.Participant{ID: "s-1", Name: "Robin", Role: identity.RoleSpectator}
	if err := MayWatch(robin, "b-bram", actors); err != nil {
		t.Errorf("a spectator who shares b-bram may not watch it: %v", err)
	}
	for _, id := range []string{"a-ash", "c-cole", ""} {
		if err := MayWatch(robin, id, actors); err == nil {
			t.Errorf("a spectator who shares b-bram may watch %q, a player's set names b-bram alone", id)
		}
	}
	if got := defaultViewpoint(robin, actors); got != "b-bram" {
		t.Errorf("a spectator who shares b-bram defaults to %q, want b-bram", got)
	}
	sol := &identity.Participant{ID: "s-2", Name: "Sol", Role: identity.RoleSpectator}
	for _, id := range []string{"a-ash", "c-cole", ""} {
		if err := MayWatch(sol, id, actors); err != nil {
			t.Errorf("a spectator who controls nothing may not watch %q: %v", id, err)
		}
	}
	if got := defaultViewpoint(sol, actors); got != "" {
		t.Errorf("a spectator who controls nothing defaults to %q, want nobody", got)
	}
}
