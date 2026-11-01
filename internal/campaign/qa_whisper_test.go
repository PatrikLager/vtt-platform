package campaign_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

var qaWSeq atomic.Int64

func qaWID() string { return fmt.Sprintf("qaw-%d", qaWSeq.Add(1)) }

func qaWEnv(env *vttv1.Envelope) *vttv1.Envelope {
	env.EventId = qaWID()
	env.SessionId = "qaw-session"
	return env
}

func qaWActor(id, name string, kind vttv1.ActorKind) *vttv1.Envelope {
	return qaWEnv(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: name, Kind: kind},
	}}})
}

func qaWWhisper(text, from string, to ...string) *vttv1.Envelope {
	return qaWEnv(&vttv1.Envelope{
		ActorRole: "dm", ParticipantId: "qaw-dm",
		Payload: &vttv1.Envelope_WhisperAdded{WhisperAdded: &vttv1.WhisperAdded{
			Text: text, ToActorIds: to, FromActorId: from,
		}},
	})
}

func qaWNarration(text string) *vttv1.Envelope {
	return qaWEnv(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}})
}

func qaWTable(t *testing.T) *campaign.Campaign {
	t.Helper()
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	start := qaWEnv(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "night"}}})
	for _, env := range []*vttv1.Envelope{
		start,
		qaWActor("tobbe", "Tobbe", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaWActor("anna", "Anna", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaWActor("bruno", "Bruno", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaWActor("goblin", "Goblin", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
	} {
		if _, err := c.Append(env); err != nil {
			t.Fatalf("setup append %v: %v", env, err)
		}
	}
	return c
}

func qaWWorld(t *testing.T, c *campaign.Campaign) []*vttv1.Envelope {
	t.Helper()
	ch, stop, head, err := c.Subscribe(0, 1024)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer stop()
	var out []*vttv1.Envelope
	for head > 0 {
		select {
		case env := <-ch:
			out = append(out, env)
			if env.GetSequence() >= head {
				return out
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("world catch-up stalled at %d of %d", len(out), head)
		}
	}
	return out
}

func qaWByID(envs []*vttv1.Envelope, id string) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, e := range envs {
		if e.GetEventId() == id {
			out = append(out, e)
		}
	}
	return out
}

func qaWLog(t *testing.T, c *campaign.Campaign, actor string) []*vttv1.Envelope {
	t.Helper()
	entries, err := c.CharacterLog(actor)
	if err != nil {
		t.Fatalf("character log %s: %v", actor, err)
	}
	out := make([]*vttv1.Envelope, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Env)
	}
	return out
}

func qaWWhispersIn(envs []*vttv1.Envelope) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, e := range envs {
		if e.GetWhisperAdded() != nil {
			out = append(out, e)
		}
	}
	return out
}

func qaWAppendOK(t *testing.T, c *campaign.Campaign, env *vttv1.Envelope) *vttv1.Envelope {
	t.Helper()
	if _, err := c.Append(env); err != nil {
		t.Fatalf("append whisper: %v", err)
	}
	got := qaWByID(qaWWorld(t, c), env.GetEventId())
	if len(got) != 1 {
		t.Fatalf("world log holds %d copies of %s, want 1", len(got), env.GetEventId())
	}
	return got[0]
}

func qaWExpectOnly(t *testing.T, c *campaign.Campaign, world *vttv1.Envelope, wantTo map[string][]string, wantFrom string) {
	t.Helper()
	for _, actor := range []string{"tobbe", "anna", "bruno", "goblin"} {
		log := qaWLog(t, c, actor)
		copies := qaWByID(log, world.GetEventId())
		to, want := wantTo[actor]
		if !want {
			if len(copies) != 0 || len(qaWWhispersIn(log)) != 0 {
				t.Errorf("%s's log holds the whisper (%d copies, %d whispers), want none: %v", actor, len(copies), len(qaWWhispersIn(log)), copies)
			}
			continue
		}
		if len(copies) != 1 {
			t.Errorf("%s's log holds %d copies of the whisper, want 1", actor, len(copies))
			continue
		}
		cp := copies[0]
		if cp.GetParticipantId() != "" || cp.GetActorRole() != "" {
			t.Errorf("%s's copy carries participant %q role %q, want both empty", actor, cp.GetParticipantId(), cp.GetActorRole())
		}
		w := cp.GetWhisperAdded()
		if strings.Join(w.GetToActorIds(), ",") != strings.Join(to, ",") || w.GetFromActorId() != wantFrom {
			t.Errorf("%s's copy names to=%v from=%q, want to=%v from=%q", actor, w.GetToActorIds(), w.GetFromActorId(), to, wantFrom)
		}
		if w.GetText() != world.GetWhisperAdded().GetText() {
			t.Errorf("%s's copy reads %q, want %q", actor, w.GetText(), world.GetWhisperAdded().GetText())
		}
		trimmed := proto.Clone(world).(*vttv1.Envelope)
		trimmed.ParticipantId, trimmed.ActorRole, trimmed.Sequence = "", "", 0
		trimmed.GetWhisperAdded().ToActorIds = to
		got := proto.Clone(cp).(*vttv1.Envelope)
		got.Sequence = 0
		if !proto.Equal(trimmed, got) {
			t.Errorf("%s's copy differs from the world event beyond what is cleared:\n got %v\nwant %v", actor, got, trimmed)
		}
	}
}

// VTT-582
func TestQAWAWhisperToOnePartyMemberIsWrittenOnlyIntoItsLog(t *testing.T) {
	c := qaWTable(t)
	world := qaWAppendOK(t, c, qaWWhisper("An arrow came from the east.", "", "tobbe"))
	if got := world.GetWhisperAdded().GetToActorIds(); len(got) != 1 || got[0] != "tobbe" {
		t.Fatalf("world copy names %v, want [tobbe]", got)
	}
	qaWExpectOnly(t, c, world, map[string][]string{"tobbe": {"tobbe"}}, "")
}

// VTT-582, VTT-584
func TestQAWAWhisperToTwoPartyMembersIsCutToEachLogsOwnCharacter(t *testing.T) {
	c := qaWTable(t)
	world := qaWAppendOK(t, c, qaWWhisper("You both smell smoke.", "", "tobbe", "anna"))
	if got := strings.Join(world.GetWhisperAdded().GetToActorIds(), ","); got != "tobbe,anna" {
		t.Errorf("world copy names %q, want tobbe,anna uncut", got)
	}
	qaWExpectOnly(t, c, world, map[string][]string{"tobbe": {"tobbe"}, "anna": {"anna"}}, "")
}

// VTT-584
func TestQAWAWhisperNamingACreatureToo(t *testing.T) {
	c := qaWTable(t)
	world := qaWAppendOK(t, c, qaWWhisper("The goblin winks at you.", "", "goblin", "bruno"))
	qaWExpectOnly(t, c, world, map[string][]string{"bruno": {"bruno"}}, "")
}

// VTT-583
func TestQAWAWhisperFromAPartyMemberIsWrittenOnlyIntoItsLog(t *testing.T) {
	c := qaWTable(t)
	world := qaWAppendOK(t, c, qaWWhisper("I pocket the ring.", "anna"))
	if world.GetWhisperAdded().GetFromActorId() != "anna" {
		t.Fatalf("world copy names from=%q, want anna", world.GetWhisperAdded().GetFromActorId())
	}
	qaWExpectOnly(t, c, world, map[string][]string{"anna": nil}, "anna")
}

// VTT-582, VTT-583, VTT-584
func TestQAWABatchOfWhispersReachesEachAddresseeAlone(t *testing.T) {
	c := qaWTable(t)
	a := qaWWhisper("to tobbe", "", "tobbe")
	b := qaWWhisper("from bruno", "bruno")
	if _, err := c.AppendBatch([]*vttv1.Envelope{a, b}); err != nil {
		t.Fatalf("batch: %v", err)
	}
	want := map[string]map[string]int{
		"tobbe": {a.EventId: 1, b.EventId: 0},
		"anna":  {a.EventId: 0, b.EventId: 0},
		"bruno": {a.EventId: 0, b.EventId: 1},
	}
	for actor, ids := range want {
		log := qaWLog(t, c, actor)
		for id, n := range ids {
			if got := len(qaWByID(log, id)); got != n {
				t.Errorf("%s's log holds %d copies of %s, want %d", actor, got, id, n)
			}
		}
		if got, n := len(qaWWhispersIn(log)), ids[a.EventId]+ids[b.EventId]; got != n {
			t.Errorf("%s's log holds %d whispers, want %d", actor, got, n)
		}
	}
	if got := qaWByID(qaWLog(t, c, "tobbe"), a.EventId); len(got) == 1 && strings.Join(got[0].GetWhisperAdded().GetToActorIds(), ",") != "tobbe" {
		t.Errorf("tobbe's copy names %v", got[0].GetWhisperAdded().GetToActorIds())
	}
}

// VTT-362
func TestQAWNarrationStillReachesEveryLogBesideWhispers(t *testing.T) {
	c := qaWTable(t)
	qaWAppendOK(t, c, qaWWhisper("psst", "", "tobbe"))
	n := qaWNarration("The torches gutter.")
	if _, err := c.Append(n); err != nil {
		t.Fatalf("narration: %v", err)
	}
	for _, actor := range []string{"tobbe", "anna", "bruno"} {
		if got := qaWByID(qaWLog(t, c, actor), n.EventId); len(got) != 1 {
			t.Errorf("%s's log holds %d copies of the narration, want 1", actor, len(got))
		}
	}
}

// VTT-582, VTT-583
func TestQAWLogsHoldingWhispersFoldAgainOnReopen(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range []*vttv1.Envelope{
		qaWEnv(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{}}}),
		qaWActor("tobbe", "Tobbe", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaWActor("anna", "", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaWWhisper("one", "", "tobbe", "anna"),
		qaWWhisper("two", "anna"),
	} {
		if _, err := c.Append(env); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	before := map[string]int{"tobbe": len(qaWWhispersIn(qaWLog(t, c, "tobbe"))), "anna": len(qaWWhispersIn(qaWLog(t, c, "anna")))}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c2, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("reopen refused a campaign holding whispers: %v", err)
	}
	defer func() { _ = c2.Close() }()
	if before["tobbe"] != 1 || before["anna"] != 2 {
		t.Errorf("whispers per log = %v, want tobbe 1, anna 2", before)
	}
	if got := len(qaWWhispersIn(qaWLog(t, c2, "anna"))); got != before["anna"] {
		t.Errorf("anna's log holds %d whispers after reopen, want %d", got, before["anna"])
	}
}

func qaWHead(t *testing.T, c *campaign.Campaign) int {
	t.Helper()
	return len(qaWWorld(t, c))
}

func qaWBoundCases() []struct {
	name string
	text string
	ok   bool
} {
	return []struct {
		name string
		text string
		ok   bool
	}{
		{"empty", "", false},
		{"8192 ascii", strings.Repeat("a", 8192), true},
		{"8193 ascii", strings.Repeat("a", 8193), false},
		{"8192 bytes of four-byte runes", strings.Repeat("𝄞", 2048), true},
		{"8192 runes, 8193 bytes", strings.Repeat("a", 8191) + "é", false},
	}
}

// VTT-585
func TestQAWTheGoFoldBoundsAWhispersTextInBytes(t *testing.T) {
	for _, shape := range []struct {
		name string
		mk   func(string) *vttv1.Envelope
	}{
		{"to", func(s string) *vttv1.Envelope { return qaWWhisper(s, "", "tobbe") }},
		{"from", func(s string) *vttv1.Envelope { return qaWWhisper(s, "tobbe") }},
	} {
		for _, tc := range qaWBoundCases() {
			t.Run(shape.name+"/"+tc.name, func(t *testing.T) {
				c := qaWTable(t)
				head := qaWHead(t, c)
				_, err := c.Append(shape.mk(tc.text))
				if tc.ok && err != nil {
					t.Fatalf("append refused a whisper of %d bytes: %v", len(tc.text), err)
				}
				if !tc.ok {
					if err == nil {
						t.Fatalf("append accepted a whisper of %d bytes", len(tc.text))
					}
					if after := qaWHead(t, c); after != head {
						t.Errorf("a refused whisper changed the world log from %d to %d events", head, after)
					}
				}
				st := engine.NewState()
				_ = engine.Apply(st, qaWActor("tobbe", "Tobbe", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
				if err := engine.Apply(st, shape.mk(tc.text)); (err == nil) != tc.ok {
					t.Errorf("engine.Apply on %d bytes: err=%v, want ok=%v", len(tc.text), err, tc.ok)
				}
			})
		}
	}
}

// VTT-586
func TestQAWTheGoFoldRefusesAWhisperNamingAnUnfoldedActor(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  func() *vttv1.Envelope
	}{
		{"to an unknown actor", func() *vttv1.Envelope { return qaWWhisper("hi", "", "nobody") }},
		{"to a known and an unknown actor", func() *vttv1.Envelope { return qaWWhisper("hi", "", "tobbe", "nobody") }},
		{"from an unknown actor", func() *vttv1.Envelope { return qaWWhisper("hi", "nobody") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qaWTable(t)
			head := qaWHead(t, c)
			if _, err := c.Append(tc.env()); err == nil {
				t.Errorf("campaign.Append accepted a whisper %s", tc.name)
			}
			if after := qaWHead(t, c); after != head {
				t.Errorf("a refused whisper changed the world log from %d to %d events", head, after)
			}
			prefix := append(qaWWorld(t, c), tc.env())
			if _, err := campaign.FoldPrefix(prefix); err == nil {
				t.Errorf("FoldPrefix accepted a log holding a whisper %s", tc.name)
			}
			batch := []*vttv1.Envelope{qaWWhisper("fine", "", "anna"), tc.env()}
			if _, err := c.AppendBatch(batch); err == nil {
				t.Errorf("AppendBatch accepted a batch holding a whisper %s", tc.name)
			}
			if got := qaWByID(qaWLog(t, c, "anna"), batch[0].EventId); len(got) != 0 {
				t.Errorf("a refused batch still wrote anna's whisper into her log")
			}
		})
	}
}

// VTT-586
func TestQAWTheGoFoldRefusesAWhisperToARemovedActor(t *testing.T) {
	c := qaWTable(t)
	rm := qaWEnv(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "goblin"}}})
	if _, err := c.Append(rm); err != nil {
		t.Fatalf("remove goblin: %v", err)
	}
	if _, err := c.Append(qaWWhisper("hi", "", "goblin")); err == nil {
		t.Errorf("campaign.Append accepted a whisper to an actor the fold no longer holds")
	}
}
