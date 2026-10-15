package gateway_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

type resumeStep struct {
	env   *vttv1.Envelope
	perch string
}

type resumeHistory struct {
	name   string
	viewer gateway.Viewer
	steps  []resumeStep
	every  int
	oracle bool
	// Resume at these steps too, whatever every says.
	also map[int]bool
}

func resumeEyes(v gateway.Viewer) perceive.Eyes {
	if v.Role == identity.RolePlayer {
		return perceive.Eyes{Controller: v.ParticipantID}
	}
	return perceive.Eyes{Shoulder: v.Viewpoint}
}

func resumeFold(t *testing.T, frames []*vttv1.Envelope) *engine.State {
	t.Helper()
	st, err := campaign.FoldPrefix(frames)
	if err != nil {
		t.Fatalf("a viewer's frames do not fold: %v", err)
	}
	return st
}

// resumeCompare is VTT-312's two sides; the plan's D13 samples the eventgen part.
func resumeCompare(t *testing.T, h resumeHistory) (resumes, sighted, compared int) {
	t.Helper()
	var log []*vttv1.Envelope
	worlds := make([]*engine.State, len(h.steps))
	var last *engine.State
	for i, s := range h.steps {
		if s.env == nil {
			worlds[i] = last
			continue
		}
		log = append(log, s.env)
		var err error
		if last, err = campaign.FoldPrefix(log); err != nil {
			t.Fatalf("%s: the world does not fold at step %d: %v", h.name, i, err)
		}
		worlds[i] = last
	}
	pr := gateway.NewProjector(h.viewer)
	sent := make([][]*vttv1.Envelope, len(h.steps))
	viewpoints := make([]string, len(h.steps)+1)
	viewpoints[0] = h.viewer.Viewpoint
	for i, s := range h.steps {
		viewpoints[i+1] = viewpoints[i]
		if s.env == nil {
			sent[i] = pr.ReperchForTest(s.perch, worlds[i])
			viewpoints[i+1] = s.perch
			continue
		}
		sent[i] = pr.Project(s.env, worlds[i])
	}
	for k := 1; k < len(h.steps); k++ {
		if (k-1)%h.every != 0 && !h.also[k] {
			continue
		}
		var shown []*vttv1.Envelope
		for _, f := range sent[:k] {
			shown = append(shown, f...)
		}
		v := h.viewer
		v.Viewpoint = viewpoints[k]
		resumed := perceive.New(resumeEyes(v), worlds[k-1])
		resumes++
		if resumeHasEyes(worlds[k-1], v) {
			sighted++
		}
		for i := k; i < len(h.steps); i++ {
			var got []*vttv1.Envelope
			if s := h.steps[i]; s.env == nil {
				v.Viewpoint = s.perch
				got = resumed.Perch(resumeEyes(v), resumeFold(t, shown), worlds[i], 0)
			} else {
				got = resumed.Event(resumeEyes(v), s.env, resumeFold(t, shown), worlds[i])
			}
			if len(got) != len(sent[i]) {
				t.Fatalf("%s: started over at %d, step %d sent %d frames, fed from the first %d",
					h.name, k, i, len(got), len(sent[i]))
			}
			for j := range got {
				if !proto.Equal(got[j], sent[i][j]) {
					t.Fatalf("%s: started over at %d, step %d frame %d is %v, fed from the first %v",
						h.name, k, i, j, got[j], sent[i][j])
				}
			}
			compared += len(got)
			shown = append(shown, got...)
		}
	}
	return resumes, sighted, compared
}

// resumeHasEyes reports whether v sees through any actor of st, so that a
// resume point exercises what the viewer saw and not an empty board.
func resumeHasEyes(st *engine.State, v gateway.Viewer) bool {
	if st == nil {
		return false
	}
	if v.Role == identity.RolePlayer {
		for _, a := range st.Actors {
			if slices.Contains(a.GetControllerIds(), v.ParticipantID) {
				return true
			}
		}
		return false
	}
	a, ok := st.Actors[v.Viewpoint]
	return ok && engine.IsPartyMember(a)
}

// resumeParty is the party members st holds, sorted.
func resumeParty(st *engine.State) []string {
	var out []string
	for id, a := range st.Actors {
		if engine.IsPartyMember(a) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func resumeEventgenLog(t *testing.T, seed int64) []*vttv1.Envelope {
	t.Helper()
	m := eventgen.New()
	rng := rand.New(rand.NewSource(seed))
	server := engine.NewState()
	var log []*vttv1.Envelope
	var seq int64
	for i := 0; i < 250; i++ {
		a := m.Step(rng, i)
		if a.Env == nil {
			continue
		}
		probe := proto.Clone(a.Env).(*vttv1.Envelope)
		probe.Sequence = seq + 1
		if err := engine.Apply(server.Snapshot(), probe); err != nil || a.MustFail {
			continue
		}
		seq++
		a.Env.Sequence = seq
		a.Env.ParticipantId = "prop-issuer"
		if err := engine.Apply(server, a.Env); err != nil {
			t.Fatal(err)
		}
		m.Accepted(a, seq)
		log = append(log, proto.Clone(a.Env).(*vttv1.Envelope))
	}
	return log
}

func resumeHistories(t *testing.T) []resumeHistory {
	t.Helper()
	var out []resumeHistory
	events := func(log []*vttv1.Envelope) []resumeStep {
		steps := make([]resumeStep, len(log))
		for i, e := range log {
			steps[i] = resumeStep{env: e}
		}
		return steps
	}
	corpus := map[string]keystoneGolden{}
	for _, g := range keystoneCorpus(t) {
		corpus[g.name] = g
	}
	seatDirs, err := filepath.Glob("../../scenarios/goldens/*/projections/*/viewer.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range seatDirs {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var seat projectedSeat
		if err := json.Unmarshal(raw, &seat); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		// Select on the declared viewer, never the directory name: an oracle seat
		// is a spectator perched on a party member.
		if seat.Role != string(identity.RoleSpectator) || seat.Viewpoint == "" {
			continue
		}
		golden := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
		out = append(out, resumeHistory{name: golden + "/" + seat.Seat,
			viewer: gateway.Viewer{ParticipantID: seat.ParticipantID, Role: identity.RoleSpectator, Viewpoint: seat.Viewpoint},
			steps:  events(corpus[golden].log), every: 1, oracle: true})
	}
	for _, h := range historyFolds() {
		var log []*vttv1.Envelope
		for i, p := range h.events {
			log = append(log, historyEnvelope(int64(i+1), p))
		}
		out = append(out, resumeHistory{name: "history/" + h.name,
			viewer: gateway.Viewer{ParticipantID: "w", Role: identity.RoleSpectator, Viewpoint: "hero"},
			steps:  events(log), every: 1})
	}
	for seed := int64(1); seed <= 6; seed++ {
		log := resumeEventgenLog(t, seed)
		world, err := campaign.FoldPrefix(log)
		if err != nil {
			t.Fatal(err)
		}
		// Seat the players that control something by the end, and a spectator on
		// the first party member, so every seat has eyes for part of the log.
		var players []string
		for _, a := range world.Actors {
			for _, c := range a.GetControllerIds() {
				if !slices.Contains(players, c) {
					players = append(players, c)
				}
			}
		}
		slices.Sort(players)
		for _, p := range players[:min(2, len(players))] {
			out = append(out, resumeHistory{name: fmt.Sprintf("seed=%d/%s", seed, p),
				viewer: gateway.Viewer{ParticipantID: p, Role: identity.RolePlayer}, steps: events(log), every: 10})
		}
		if party := resumeParty(world); len(party) > 0 {
			out = append(out, resumeHistory{name: fmt.Sprintf("seed=%d/spectator", seed),
				viewer: gateway.Viewer{ParticipantID: "w", Role: identity.RoleSpectator, Viewpoint: party[0]},
				steps:  events(log), every: 10})
		}
		var perching []resumeStep
		also := map[int]bool{}
		hop := 0
		for i, e := range log {
			perching = append(perching, resumeStep{env: e})
			if i%30 != 29 {
				continue
			}
			now, err := campaign.FoldPrefix(log[:i+1])
			if err != nil {
				t.Fatal(err)
			}
			if party := resumeParty(now); len(party) > 0 {
				also[len(perching)], also[len(perching)+1] = true, true
				perching = append(perching, resumeStep{perch: party[hop%len(party)]})
				hop++
			}
		}
		out = append(out, resumeHistory{name: fmt.Sprintf("seed=%d/percher", seed),
			viewer: gateway.Viewer{ParticipantID: "w", Role: identity.RoleSpectator},
			steps:  perching, every: 10, also: also})
	}
	return out
}

// VTT-312
func TestADecisionStartedOverFromWhatItsViewerHoldsSendsWhatOneFedFromTheStartSends(t *testing.T) {
	start := time.Now()
	histories := resumeHistories(t)
	oracles := 0
	for _, h := range histories {
		resumes, sighted, compared := resumeCompare(t, h)
		if sighted == 0 || compared == 0 {
			t.Errorf("%s: %d resume points, %d with eyes, and %d frames compared; the history proves nothing",
				h.name, resumes, sighted, compared)
		}
		if h.oracle {
			oracles++
		}
	}
	if oracles == 0 {
		t.Fatal("no oracle seat was selected from scenarios/goldens")
	}
	t.Logf("%d histories, %d oracle seats, in %s", len(histories), oracles, time.Since(start).Round(time.Millisecond))
}
