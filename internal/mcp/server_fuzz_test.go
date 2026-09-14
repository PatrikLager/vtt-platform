package mcp

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// commandFields is every arm of the ClientCommand oneof, in descriptor order.
// It is read from the contract rather than listed, so a command added tomorrow
// is fuzzed tomorrow — the same reflection internal/gateway's
// TestEveryClientCommandHasRoleCells uses, for the same reason.
func commandFields() []protoreflect.FieldDescriptor {
	oneof := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor().Oneofs().ByName("command")
	out := make([]protoreflect.FieldDescriptor, 0, oneof.Fields().Len())
	for i := range oneof.Fields().Len() {
		out = append(out, oneof.Fields().Get(i))
	}
	return out
}

// FuzzToolArgumentsBecomeTheCommandTheyName drives the MCP surface's trust
// boundary: an agent hands over raw JSON, and commandFromArgs turns it into a
// ClientCommand by unmarshalling into a dynamically-resolved message type and
// setting it into a oneof by reflection.
//
// NOT PANICS. A fuzz target that only asserts "did not crash" would pass with
// the boundary returning the wrong command every time. The property is that the
// command coming out is the one the tool NAMED:
//
//   - exactly one oneof arm is populated, never zero and never two;
//   - that arm is the field descriptor the handler was built for.
//
// WHY THAT IS THE INVARIANT THAT MATTERS: internal/gateway decides
// authorization by reading which arm is populated. commandRoles is keyed by the
// name derived from it, and authorizePlayer's precondition is that its name
// argument matches the command it is judging. A tool whose arguments could
// produce a different arm than its own descriptor would be judged by another
// command's rule — a player's move_token rule applied to an add_actor, or the
// reverse. internal/mcp may not import internal/gateway (.go-arch-lint.yml), so
// this states the same guarantee on the side that produces it.
//
// The round-trip is checked too: a command that cannot be marshalled back is
// one the gateway could not be sent — internal/harness's client really does
// protojson.Marshal it onto the wire. That assertion has NO fault-injection
// proof and review could not falsify it either: the one asymmetry that would
// fire it, a Value holding ±Inf, is refused by the decoder first. It is a
// tripwire against a protobuf-go upgrade or a contract change, not a live
// guard, and labelling it honestly is what rule 1 asks when a proof is absent.
func FuzzToolArgumentsBecomeTheCommandTheyName(f *testing.F) {
	fields := commandFields()

	// Seeds are shapes, not coverage. Each is a thing a hostile or confused
	// caller really sends; the fuzzer's job is the space between them.
	seeds := []string{
		// well-formed, and the empty object the boundary substitutes for absent
		`{}`, ``, `   `,
		// valid-looking arguments for several commands
		`{"tokenId":"t1","to":{"x":1,"y":2}}`,
		`{"actorId":"a1"}`,
		`{"sceneId":"s","at":{"x":0,"y":1}}`,
		`{"text":"a line of narration"}`,
		// shapes protojson must refuse rather than guess at
		`{"unknownField":1}`, `{"tokenId":123}`,
		// AND ONE IT ACCEPTS, which is the sharper seed of the two groups and
		// was filed under "must refuse" until review measured it: a null for any
		// field that is not a google.protobuf.Value is DISCARDED and decoding
		// continues, so this yields a MoveTokenRequest with an empty token_id
		// rather than an error. The gateway validates ids; this seam does not.
		`{"tokenId":null}`,
		`[]`, `[1,2,3]`, `null`, `true`, `"a string"`, `42`,
		// duplicate keys, where JSON itself is ambiguous
		`{"tokenId":"a","tokenId":"b"}`,
		// numbers at and past the edges of the types they land in
		`{"to":{"x":2147483647,"y":-2147483648}}`,
		`{"to":{"x":2147483648,"y":0}}`,
		`{"to":{"x":1e308,"y":0}}`,
		`{"to":{"x":"1","y":"2"}}`,
		// a oneof arm named INSIDE the arguments. These are refused as unknown
		// fields, because protojson is decoding into the ARM's own message and
		// MoveTokenRequest has no "command" field — so this is not a shape that
		// could break the invariant even in principle, which an earlier comment
		// here claimed. Kept because it is what a caller confusing the envelope
		// for the payload actually sends.
		`{"command":{"addActor":{}}}`, `{"moveToken":{"tokenId":"t"}}`,
		// strings that have cost this repo elsewhere: long, and invalid UTF-8
		`{"actorId":"` + longRun(300) + `"}`,
		"{\"actorId\":\"\xff\xfe\"}",
		// structurally hostile
		`{`, `}`, `{"a":`, `{"a":{"b":{"c":{"d":{}}}}}`,
		`{"actorId":"\ud800"}`,
	}
	// EVERY ARM, not just the first and last. Seeding only two indices left 20
	// of 22 commands unreached by anything the gate runs — `task check` runs the
	// seeds, never -fuzz — and the arm that mattered most was among them:
	// add_actor is the only command whose message reaches a recursive
	// well-known type (Actor.module_data is a google.protobuf.Struct), so it is
	// the one place decode depth is bounded by a runtime limit rather than by
	// the schema. The full 22x30 matrix runs in well under a second.
	for i := range fields {
		for _, s := range seeds {
			f.Add(i, s)
		}
	}

	f.Fuzz(func(t *testing.T, idx int, raw string) {
		if len(fields) == 0 {
			t.Fatal("ClientCommand has no command oneof arms")
		}
		// Any int the fuzzer produces selects a real arm, so no input is wasted
		// on an out-of-range index.
		fd := fields[((idx%len(fields))+len(fields))%len(fields)]

		cmd, err := commandFromArgs(fd, []byte(raw))
		if err != nil {
			if cmd != nil {
				t.Fatalf("commandFromArgs(%s, %q) returned both a command and an error: %v",
					fd.Name(), raw, err)
			}
			return
		}
		if cmd == nil {
			t.Fatalf("commandFromArgs(%s, %q) returned (nil, nil)", fd.Name(), raw)
		}

		var populated []protoreflect.FieldDescriptor
		cmd.ProtoReflect().Range(func(f protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			if f.ContainingOneof() != nil {
				populated = append(populated, f)
			}
			return true
		})
		if len(populated) != 1 {
			t.Fatalf("commandFromArgs(%s, %q) populated %d oneof arms, want exactly 1 — "+
				"authorization reads which arm is set to decide what this command IS",
				fd.Name(), raw, len(populated))
		}
		if populated[0].FullName() != fd.FullName() {
			t.Fatalf("commandFromArgs(%s, %q) produced a %s — the arguments chose a different "+
				"command than the tool that accepted them, so the gateway would judge it by "+
				"another command's rule", fd.Name(), raw, populated[0].FullName())
		}
		if _, err := protojson.Marshal(cmd); err != nil {
			t.Fatalf("commandFromArgs(%s, %q) built a command that cannot be marshalled: %v — "+
				"it could never be sent, which is a different failure than being refused",
				fd.Name(), raw, err)
		}
	})
}

func longRun(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// TestAToolCalledWithNoArgumentsGetsAnEmptyObject pins the one behaviour of the
// trust boundary that the fuzz target above cannot see.
//
// commandFromArgs substitutes "{}" for absent arguments, because a tool whose
// fields are all optional is legitimately called with nothing — end_session
// takes none at all. Delete that substitution and protojson refuses the empty
// input, so the tool stops working; the fuzz target stays green throughout,
// because a refusal is a legal outcome there and it simply returns early.
//
// Measured: removing the substitution failed no test in this package.
func TestAToolCalledWithNoArgumentsGetsAnEmptyObject(t *testing.T) {
	var noArgTool protoreflect.FieldDescriptor
	for _, fd := range commandFields() {
		if fd.Message().Fields().Len() == 0 {
			noArgTool = fd
			break
		}
	}
	if noArgTool == nil {
		t.Skip("no command in the contract takes zero fields any more")
	}
	for _, args := range [][]byte{nil, {}} {
		cmd, err := commandFromArgs(noArgTool, args)
		if err != nil {
			t.Fatalf("commandFromArgs(%s, %#v): %v — a tool that takes no fields must be "+
				"callable with no arguments", noArgTool.Name(), args, err)
		}
		if cmd == nil {
			t.Fatalf("commandFromArgs(%s, %#v) returned no command and no error",
				noArgTool.Name(), args)
		}
	}
}
