package mcp

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-671
func TestAddActorArgumentsRefuseACurrentWithNoValue(t *testing.T) {
	fd := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor().Fields().ByName("add_actor")
	if fd == nil {
		t.Fatal("ClientCommand has no add_actor arm")
	}
	cases := []struct{ name, args, want string }{
		{"a current written null",
			`{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","resources":{"focus":{"current":null,"max":3}}}}`,
			`actor "a1": resources.focus.current`},
		{"a current left out",
			`{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","attributes":{"brace":1},"resources":{"focus":{"max":3}}}}`,
			`actor "a1": resources.focus.current`},
		{"a resource written empty",
			`{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","resources":{"focus":{}}}}`,
			`actor "a1": resources.focus.current`},
		{"the proto field names",
			`{"actor":{"actor_id":"a1","kind":"ACTOR_KIND_NON_PARTY","resources":{"focus":{"max":3}}}}`,
			`actor "a1": resources.focus.current`},
		{"the first resource in byte order of the keys",
			`{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","resources":{"zz":{"max":1},"aa":{"max":1},"mm":{"current":1}}}}`,
			`actor "a1": resources.aa.current`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, err := commandFromArgs(fd, []byte(c.args))
			if err == nil {
				t.Fatalf("commandFromArgs accepted %s as %v", c.args, cmd)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), c.want)
			}
		})
	}
}

// VTT-671 VTT-679
func TestAddActorArgumentsPassACurrentOfZero(t *testing.T) {
	fd := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor().Fields().ByName("add_actor")
	cases := []struct{ name, args string }{
		{"a current of 0", `{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","attributes":{"brace":1},"resources":{"focus":{"current":0,"max":3}}}}`},
		{"a current and no maximum", `{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","resources":{"focus":{"current":4}}}}`},
		{"a maximum written null", `{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","resources":{"focus":{"current":4,"max":null}}}}`},
		{"no resources", `{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY"}}`},
		{"module data is the module's, never read as a resource",
			`{"actor":{"actorId":"a1","kind":"ACTOR_KIND_NON_PARTY","moduleData":{"resources":{"focus":{"max":3}}}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, err := commandFromArgs(fd, []byte(c.args))
			if err != nil {
				t.Fatalf("commandFromArgs refused %s: %v", c.args, err)
			}
			if cmd.GetAddActor().GetActor().GetActorId() != "a1" {
				t.Errorf("command = %v, want the actor a1", cmd)
			}
		})
	}
}
