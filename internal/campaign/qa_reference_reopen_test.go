package campaign_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const (
	qarcParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qarcCreature = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qarcCamp struct {
	t   *testing.T
	dir string
	c   *campaign.Campaign
	n   int
}

func qarcOpenCamp(t *testing.T) *qarcCamp {
	t.Helper()
	k := &qarcCamp{t: t, dir: t.TempDir()}
	k.open()
	t.Cleanup(func() {
		if k.c != nil {
			_ = k.c.Close()
		}
	})
	return k
}

func (k *qarcCamp) open() {
	k.t.Helper()
	c, err := campaign.Open(k.dir)
	if err != nil {
		k.t.Fatalf("open: %v", err)
	}
	k.c = c
}

func (k *qarcCamp) reopen() {
	k.t.Helper()
	if err := k.c.Close(); err != nil {
		k.t.Fatalf("close: %v", err)
	}
	k.c = nil
	k.open()
}

func (k *qarcCamp) append(env *vttv1.Envelope) int64 {
	k.t.Helper()
	k.n++
	env.EventId = fmt.Sprintf("qarc-%d", k.n)
	seq, err := k.c.Append(env)
	if err != nil {
		k.t.Fatalf("append %s: %v", prototext.Format(env), err)
	}
	return seq
}

func (k *qarcCamp) log(actor string) []campaign.CharacterEntry {
	k.t.Helper()
	entries, err := k.c.CharacterLog(actor)
	if err != nil {
		k.t.Fatalf("character log %s: %v", actor, err)
	}
	return entries
}

func (k *qarcCamp) fold(actor string) *engine.State {
	k.t.Helper()
	entries := k.log(actor)
	envs := make([]*vttv1.Envelope, 0, len(entries))
	for _, e := range entries {
		envs = append(envs, e.Env)
	}
	st, err := campaign.FoldPrefix(envs)
	if err != nil {
		k.t.Fatalf("the log of %s does not fold: %v", actor, err)
	}
	return st
}

// VTT-1036.
func (k *qarcCamp) holds() {
	k.t.Helper()
	world := k.c.State()
	for _, owner := range qarcSorted(world.Actors) {
		if !engine.IsPartyMember(world.Actors[owner]) {
			continue
		}
		fold := k.fold(owner)
		eyes := perceive.Eyes{Shoulder: owner}
		for _, id := range qarcSorted(world.Actors) {
			if !perceive.SeesActor(eyes, world, id) {
				continue
			}
			w, h := world.Actors[id], fold.Actors[id]
			if h == nil {
				k.t.Fatalf("the log of %s holds no %s, which is in its sight", owner, id)
			}
			var want map[string]*vttv1.Reference
			if engine.IsPartyMember(w) {
				want = w.GetReferences()
			}
			if !qarcRefsEqual(h.GetReferences(), want) {
				k.t.Fatalf("the log of %s holds %s at %s, the world's form is %s",
					owner, id, qarcRefsText(h.GetReferences()), qarcRefsText(want))
			}
		}
		for _, id := range qarcSorted(fold.Actors) {
			if h := fold.Actors[id]; !engine.IsPartyMember(h) && len(h.GetReferences()) > 0 {
				k.t.Fatalf("the log of %s holds creature %s at %s", owner, id, qarcRefsText(h.GetReferences()))
			}
		}
	}
}

func qarcSorted[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for id := range m {
		ks = append(ks, id)
	}
	sort.Strings(ks)
	return ks
}

func qarcRefs(specs ...string) map[string]*vttv1.Reference {
	out := map[string]*vttv1.Reference{}
	for _, s := range specs {
		id, v, _ := strings.Cut(s, "=")
		out[id] = qarcRef(v)
	}
	return out
}

func qarcRef(spec string) *vttv1.Reference {
	p := strings.Split(spec, "/")
	return &vttv1.Reference{EntryId: p[0], Place: p[1], State: p[2]}
}

func qarcRefsEqual(a, b map[string]*vttv1.Reference) bool {
	if len(a) != len(b) {
		return false
	}
	for id, x := range a {
		y, ok := b[id]
		if !ok || x.GetEntryId() != y.GetEntryId() || x.GetPlace() != y.GetPlace() ||
			x.GetState() != y.GetState() {
			return false
		}
	}
	return true
}

func qarcRefsText(m map[string]*vttv1.Reference) string {
	parts := []string{}
	for _, id := range qarcSorted(m) {
		r := m[id]
		parts = append(parts, fmt.Sprintf("%s=%s/%s/%s", id, r.GetEntryId(), r.GetPlace(), r.GetState()))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func qarcScene(id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 5, GridHeight: 5}}}
}

func qarcAdded(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qarcPlaced(tok, scene, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: scene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y}}}}
}

func qarcRefAdded(actor, id, spec string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceAdded{ReferenceAdded: &vttv1.ReferenceAdded{
		ActorId: actor, ReferenceId: id, Reference: qarcRef(spec)}}}
}

func qarcRefRemoved(actor, id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceRemoved{ReferenceRemoved: &vttv1.ReferenceRemoved{
		ActorId: actor, ReferenceId: id}}}
}

func qarcRefChanged(actor, id, place, state string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceChanged{ReferenceChanged: &vttv1.ReferenceChanged{
		ActorId: actor, ReferenceId: id, Place: place, State: state}}}
}

func (k *qarcCamp) after(skip map[string]int) map[string][]campaign.CharacterEntry {
	k.t.Helper()
	out := map[string][]campaign.CharacterEntry{}
	for owner, n := range skip {
		out[owner] = k.log(owner)[n:]
	}
	return out
}

func (k *qarcCamp) lengths(owners ...string) map[string]int {
	k.t.Helper()
	out := map[string]int{}
	for _, o := range owners {
		out[o] = len(k.log(o))
	}
	return out
}

func qarcEntriesText(entries []campaign.CharacterEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "  world %d: %s\n", e.WorldSeq, prototext.Format(e.Env))
	}
	return b.String()
}

// Keep d alone in far: the tests rely on its seeing none of a, b and e.
func qarcTable(t *testing.T) *qarcCamp {
	t.Helper()
	k := qarcOpenCamp(t)
	k.append(qarcScene("near"))
	k.append(qarcScene("far"))
	k.append(qarcAdded(&vttv1.Actor{ActorId: "a", Name: "alfa", Kind: qarcParty,
		References: qarcRefs("own=echo/mike/lima")}))
	k.append(qarcAdded(&vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		References: qarcRefs("r1=echo/mike/lima", "r2=echo/kilo/oscar")}))
	k.append(qarcAdded(&vttv1.Actor{ActorId: "d", Name: "dora", Kind: qarcParty,
		References: qarcRefs("r1=delta/mike/lima")}))
	k.append(qarcAdded(&vttv1.Actor{ActorId: "e", Name: "echo", Kind: qarcCreature,
		References: qarcRefs("r1=echo/mike/lima")}))
	k.append(qarcPlaced("ta", "near", "a", 0, 0))
	k.append(qarcPlaced("tb", "near", "b", 1, 1))
	k.append(qarcPlaced("te", "near", "e", 2, 2))
	k.append(qarcPlaced("td", "far", "d", 0, 0))
	return k
}

// VTT-1036.
func TestQARCACampaignReopenedAfterReferenceChangesResumesEveryLog(t *testing.T) {
	k := qarcTable(t)
	for _, env := range []*vttv1.Envelope{
		qarcRefAdded("b", "r3", "delta/kilo/lima"),
		qarcRefChanged("b", "r1", "kilo", "lima"),
		qarcRefChanged("b", "r2", "kilo", "lima"),
		qarcRefRemoved("b", "r3"),
		qarcRefRemoved("b", "r2"),
		qarcRefAdded("b", "r2", "delta/mike/oscar"),
		qarcRefAdded("b", "r4", "echo/kilo/lima"),
		qarcRefRemoved("a", "own"),
		qarcRefAdded("a", "Zq", "delta/mike/oscar"),
		qarcRefAdded("d", "r2", "echo/mike/lima"),
		qarcRefChanged("d", "r1", "kilo", "oscar"),
		qarcRefRemoved("d", "r2"),
		qarcRefAdded("e", "r2", "delta/kilo/lima"),
	} {
		k.append(env)
		k.holds()
	}
	k.reopen()
	k.holds()

	steps := []struct {
		env       *vttv1.Envelope
		forwarded []string
	}{
		{qarcRefChanged("b", "r1", "mike", "oscar"), []string{"a", "b"}},
		{qarcRefAdded("d", "r5", "delta/kilo/lima"), []string{"d"}},
		{qarcRefRemoved("a", "Zq"), []string{"a", "b"}},
		{qarcRefAdded("b", "r1x", "echo/mike/lima"), []string{"a", "b"}},
	}
	for i, s := range steps {
		if i == 2 {
			k.reopen()
		}
		before := k.lengths("a", "b", "d")
		k.append(s.env)
		env := s.env
		got := k.after(before)
		for _, owner := range []string{"a", "b", "d"} {
			want := 0
			for _, f := range s.forwarded {
				if f == owner {
					want = 1
				}
			}
			entries := got[owner]
			if len(entries) != want || (want == 1 && entries[0].Env.GetEventId() != env.GetEventId()) {
				t.Fatalf("after the reopen the log of %s is written, for %s:\n%s",
					owner, prototext.Format(env), qarcEntriesText(entries))
			}
		}
		k.holds()
	}
}

// VTT-1068, VTT-1069, VTT-1036.
func TestQARCAfterAReopenAPartyMemberComingIntoSightIsCorrected(t *testing.T) {
	k := qarcTable(t)
	k.append(qarcRefAdded("d", "r2", "echo/kilo/lima"))
	k.append(qarcRefChanged("d", "r1", "mike", "oscar"))
	k.reopen()
	before := k.lengths("a", "b", "d")
	seq := k.append(qarcPlaced("td2", "near", "d", 3, 3))
	got := k.after(before)
	for _, owner := range []string{"a", "b", "d"} {
		refs := 0
		for _, e := range got[owner] {
			switch {
			case e.Env.GetActorAdded() != nil && e.Env.GetActorAdded().GetActor().GetActorId() != "e",
				e.Env.GetActorRemoved() != nil:
				t.Fatalf("the log of %s withdraws or introduces:\n%s", owner, qarcEntriesText(got[owner]))
			case e.Env.GetReferenceAdded() != nil:
				refs++
				if e.Env.GetEventId() != "" || e.WorldSeq != seq {
					t.Fatalf("the log of %s is written a correction that is not bare:\n%s",
						owner, qarcEntriesText(got[owner]))
				}
			}
		}
		if refs == 0 {
			t.Fatalf("the log of %s is brought no reference:\n%s", owner, qarcEntriesText(got[owner]))
		}
	}
	k.holds()
}

type qarcErrors struct {
	mu   sync.Mutex
	msgs []string
}

func (h *qarcErrors) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelError }

func (h *qarcErrors) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	h.mu.Lock()
	h.msgs = append(h.msgs, b.String())
	h.mu.Unlock()
	return nil
}

func (h *qarcErrors) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *qarcErrors) WithGroup(string) slog.Handler { return h }

// VTT-1068, VTT-1035, VTT-1036, VTT-320.
func TestQARCGeneratedWalksKeepEveryLogAtTheWorldsReferences(t *testing.T) {
	errs := &qarcErrors{}
	prev := slog.Default()
	slog.SetDefault(slog.New(errs))
	t.Cleanup(func() { slog.SetDefault(prev) })
	references := 0
	for seed := int64(1); seed <= 12; seed++ {
		k := qarcOpenCamp(t)
		m := eventgen.New()
		rng := rand.New(rand.NewSource(seed))
		for i := 0; i < 300; i++ {
			a := m.Step(rng, i)
			if a.Env == nil {
				continue
			}
			seq, err := k.c.Append(a.Env)
			if a.MustFail {
				if err == nil {
					t.Fatalf("seed %d step %d: %s was not refused", seed, i, a.Kind)
				}
				continue
			}
			if err != nil {
				t.Fatalf("seed %d step %d: %s refused: %v", seed, i, a.Kind, err)
			}
			m.Accepted(a, seq)
			if strings.Contains(strings.ToLower(a.Kind), "reference") {
				references++
			}
			if i%20 == 19 {
				k.holds()
			}
			if i == 150 {
				k.reopen()
				k.holds()
			}
		}
		k.holds()
		k.reopen()
		k.holds()
	}
	if references == 0 {
		t.Fatalf("the walks drew no reference event")
	}
	errs.mu.Lock()
	defer errs.mu.Unlock()
	if len(errs.msgs) != 0 {
		t.Fatalf("the fan-out logged errors:\n%s", strings.Join(errs.msgs, "\n"))
	}
	t.Logf("%d reference events", references)
}
