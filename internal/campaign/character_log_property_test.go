package campaign_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
)

// Keep the seeds giving non-empty and anchored logs, and the corpus logs that
// end a session open at their opening: each guard below judges its own source.
var characterLogSeeds = []int64{1, 2, 3}

const characterLogSteps = 150

// logEnsemble counts what the character logs of a run hold, for the guard.
type logEnsemble struct {
	nonEmpty, anchored, endedOpen int
}

func noteActor(ids map[string]bool, env *vttv1.Envelope) {
	if a := env.GetActorAdded().GetActor(); a != nil {
		ids[a.GetActorId()] = true
	}
}

// check folds every prefix of every character log of the actors ids names.
func (e *logEnsemble) check(t *testing.T, c *campaign.Campaign, ids map[string]bool) {
	t.Helper()
	for id := range ids {
		entries := logOf(t, c, id)
		if len(entries) == 0 {
			continue
		}
		e.nonEmpty++
		assertFoldsAtEveryPrefix(t, id+"'s log", entries)
		openedInSession := false
		for i, en := range entries {
			switch {
			case en.Env.GetNarrationAdded().GetAnchorToSeq() != 0:
				e.anchored++
			case i == 0 && en.Env.GetSessionStarted() != nil && en.WorldSeq == 0:
				openedInSession = true
			case en.Env.GetSessionStarted() != nil:
				openedInSession = false
			case openedInSession && en.Env.GetSessionEnded() != nil:
				e.endedOpen++
				openedInSession = false
			}
		}
	}
	assertNoneDropped(t, c)
}

// corpusStream reads a golden world stream, unstamped for appending again.
func corpusStream(t *testing.T, path string) []*vttv1.Envelope {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := make([]*vttv1.Envelope, len(items))
	for i, item := range items {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(item, env); err != nil {
			t.Fatalf("%s entry %d: %v", path, i, err)
		}
		env.Sequence = 0
		out[i] = env
	}
	return out
}

// VTT-323
func TestEveryCharacterLogFoldsAtEveryPrefix(t *testing.T) {
	var seeds, corpus logEnsemble
	ran := 0
	for _, seed := range characterLogSeeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			ran++
			c := openTemp(t)
			ids := map[string]bool{}
			rng := rand.New(rand.NewSource(seed))
			m := eventgen.New()
			counts := map[string]int{}
			for i := 0; i < characterLogSteps; i++ {
				a := m.Step(rng, i)
				if a.Env != nil {
					noteActor(ids, a.Env)
				}
				applyDrawn(t, c, m, a, i, counts)
			}
			seeds.check(t, c, ids)
		})
	}
	streams, err := filepath.Glob(filepath.Join("..", "..", "scenarios", "goldens", "*", "stream.json"))
	if err != nil || len(streams) == 0 {
		t.Fatalf("no corpus streams found (%v)", err)
	}
	for _, path := range streams {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			ran++
			c := openTemp(t)
			ids := map[string]bool{}
			for _, env := range corpusStream(t, path) {
				noteActor(ids, env)
				must(t, c, env)
			}
			corpus.check(t, c, ids)
		})
	}
	// Judge the guards only on a whole, passing run: a filtered one proves less.
	if ran != len(characterLogSeeds)+len(streams) || t.Failed() {
		return
	}
	if seeds.nonEmpty < 2 || seeds.anchored < 1 {
		t.Errorf("the seeds prove too little: %+v, want 2 non-empty logs and 1 anchored narration", seeds)
	}
	if corpus.nonEmpty < 2 || corpus.endedOpen < 1 {
		t.Errorf("the corpus proves too little: %+v, want 2 non-empty logs and 1 ending a session open at its opening", corpus)
	}
	t.Logf("seeds %+v, corpus %+v", seeds, corpus)
}
