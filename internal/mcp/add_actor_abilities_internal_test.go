package mcp

import (
	"slices"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-683
func TestAddActorArgumentsCarryTheAbilityIdsInTheirOrder(t *testing.T) {
	fd := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor().Fields().ByName("add_actor")
	cases := []struct{ name, args string }{
		{"the JSON names", `{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","abilityIds":["rally","fists","rally"]}}`},
		{"the proto field names", `{"actor":{"actor_id":"a1","kind":"ACTOR_KIND_NON_PARTY","ability_ids":["rally","fists","rally"]}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, err := commandFromArgs(fd, []byte(c.args))
			if err != nil {
				t.Fatalf("commandFromArgs refused %s: %v", c.args, err)
			}
			if got, want := cmd.GetAddActor().GetActor().GetAbilityIds(), []string{"rally", "fists", "rally"}; !slices.Equal(got, want) {
				t.Errorf("the command's actor names %q, want %q", got, want)
			}
		})
	}
}
