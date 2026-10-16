package campaign

import (
	"fmt"
	"log/slog"
	"sort"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// CharacterEntry is one entry of a character log and the world sequence it
// was written with, 0 for none.
type CharacterEntry struct {
	WorldSeq int64
	Env      *vttv1.Envelope
}

// characterLog is what the campaign holds of one actor's log. Commit one into
// c.logs only once the store holds what it folded (VTT-325).
type characterLog struct {
	fold     *engine.State
	head     int64
	decision *perceive.Projection // nil while the log is paused
	// Give a working copy caused through a three-index slice, or its appends
	// write into the array c.logs holds (fanOutLocked).
	caused []causedEntry
}

// causedEntry is an entry a world event caused, which an anchor may name (VTT-322).
type causedEntry struct{ world, seq int64 }

// CharacterLog returns actorID's character log in its own order; an actor
// that has none reads empty.
func (c *Campaign) CharacterLog(actorID string) ([]CharacterEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return nil, errPoisoned
	}
	stored, err := c.log.ReadCharacterAfter(actorID, 0)
	if err != nil {
		return nil, err
	}
	out := make([]CharacterEntry, len(stored))
	for i, e := range stored {
		out[i] = CharacterEntry{WorldSeq: e.WorldSeq, Env: e.Env}
	}
	return out, nil
}

func eyesOf(id string) perceive.Eyes { return perceive.Eyes{Shoulder: id} }

func isPartyMember(st *engine.State, id string) bool {
	a, ok := st.Actors[id]
	return ok && engine.IsPartyMember(a)
}

// loadCharacterLogsLocked folds every stored log; call it once c.state is the
// world at the log's head (VTT-312, VTT-327).
func (c *Campaign) loadCharacterLogsLocked() error {
	stored, err := c.log.ReadCharacterLogs()
	if err != nil {
		return err
	}
	c.logs = map[string]*characterLog{}
	for id, entries := range stored {
		envs := make([]*vttv1.Envelope, len(entries))
		for i, e := range entries {
			envs[i] = e.Env
		}
		fold, err := foldEvents(envs)
		if err != nil {
			return fmt.Errorf("campaign: the character log of %s does not fold: %w", id, err)
		}
		l := &characterLog{fold: fold, head: envs[len(envs)-1].Sequence}
		for _, e := range entries {
			if e.WorldSeq != 0 {
				l.caused = append(l.caused, causedEntry{world: e.WorldSeq, seq: e.Env.GetSequence()})
			}
		}
		if isPartyMember(c.state, id) {
			l.decision = perceive.Resume(eyesOf(id), fold, c.state)
		}
		c.logs[id] = l
	}
	return nil
}

// fanOutLocked returns the entries envs write to the character logs, the logs
// as they stand after them, and how many frame sets it dropped. It leaves the
// folds and heads in c.logs as they are but advances their decisions: call
// restartDecisionsLocked unless the store then holds the entries (VTT-325).
// Call it only with envs that validated.
func (c *Campaign) fanOutLocked(envs []*vttv1.Envelope) ([]store.Entry, map[string]*characterLog, int, error) {
	logs := make(map[string]*characterLog, len(c.logs))
	for id, l := range c.logs {
		cp := *l
		cp.caused = l.caused[:len(l.caused):len(l.caused)]
		logs[id] = &cp
	}
	var entries []store.Entry
	dropped := 0
	world := c.state.Snapshot()
	for i, env := range envs {
		before := world.Snapshot()
		ev := proto.Clone(env).(*vttv1.Envelope)
		ev.Sequence = c.head + int64(i) + 1
		if err := engine.Apply(world, ev); err != nil {
			return nil, nil, 0, fmt.Errorf("campaign: fan-out diverged from validation: %w", err)
		}
		for _, id := range logIDs(logs, world) {
			l := logs[id]
			if !isPartyMember(world, id) {
				// VTT-316: a log is written only while its actor is a party member.
				if l != nil {
					l.decision = nil
				}
				continue
			}
			var lead []*vttv1.Envelope
			switch {
			case l == nil:
				l = &characterLog{fold: engine.NewState()}
				logs[id] = l
				lead = opening(before)
				l.decision = perceive.New(eyesOf(id), nil)
			case l.decision == nil:
				l.decision = perceive.New(eyesOf(id), nil)
			}
			fold, out, err := trial(l.fold, l.head, lead)
			if err != nil {
				slog.Error("campaign: a character log's opening does not fold; nothing written",
					"actor", id, "world_seq", ev.Sequence, "err", err)
				dropped++
				delete(logs, id)
				continue
			}
			entries = appendEntries(entries, id, -1, out)
			l.fold, l.head = fold, l.head+int64(len(out))

			frames := l.reanchor(l.decision.Event(eyesOf(id), ev, l.fold, world))
			fold, out, err = trial(l.fold, l.head, frames)
			if err != nil {
				// VTT-320: write nothing, and resume the decision from the log, since
				// Event moved it past frames the log never got.
				slog.Error("campaign: a character's frames do not fold onto its log; nothing written",
					"actor", id, "world_seq", ev.Sequence, "err", err)
				dropped++
				l.decision = perceive.Resume(eyesOf(id), l.fold, world)
				continue
			}
			entries = appendEntries(entries, id, i, out)
			for _, e := range out {
				l.caused = append(l.caused, causedEntry{world: ev.Sequence, seq: e.Sequence})
			}
			l.fold, l.head = fold, l.head+int64(len(out))
		}
	}
	return entries, logs, dropped, nil
}

// reanchor rewrites each anchored narration in frames onto the entries of
// this log its world range caused, or makes it unanchored when there are
// none (VTT-322). Rewrite only frames the decision made for this log alone.
func (l *characterLog) reanchor(frames []*vttv1.Envelope) []*vttv1.Envelope {
	for _, f := range frames {
		n := f.GetNarrationAdded()
		if n == nil || (n.AnchorFromSeq == 0 && n.AnchorToSeq == 0) {
			continue
		}
		from := sort.Search(len(l.caused), func(k int) bool { return l.caused[k].world >= n.AnchorFromSeq })
		to := sort.Search(len(l.caused), func(k int) bool { return l.caused[k].world > n.AnchorToSeq })
		if from < to {
			n.AnchorFromSeq, n.AnchorToSeq = l.caused[from].seq, l.caused[to-1].seq
		} else {
			n.AnchorFromSeq, n.AnchorToSeq = 0, 0
		}
	}
	return frames
}

// logIDs are the actors with a log and the party members of world, sorted.
func logIDs(logs map[string]*characterLog, world *engine.State) []string {
	set := map[string]bool{}
	for id := range logs {
		set[id] = true
	}
	for id, a := range world.Actors {
		if engine.IsPartyMember(a) {
			set[id] = true
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// trial folds frames onto a copy of fold, each stamped with its sequence
// after head, and returns the copy, or fold itself when there are none, and
// the stamped frames. Never write to what it returns.
func trial(fold *engine.State, head int64, frames []*vttv1.Envelope) (*engine.State, []*vttv1.Envelope, error) {
	if len(frames) == 0 {
		return fold, nil, nil
	}
	next := fold.Snapshot()
	out := make([]*vttv1.Envelope, len(frames))
	for k, f := range frames {
		e := proto.Clone(f).(*vttv1.Envelope)
		e.Sequence = head + int64(k) + 1
		if err := engine.Apply(next, e); err != nil {
			return nil, nil, err
		}
		out[k] = e
	}
	return next, out, nil
}

// appendEntries hands the store each frame unstamped: it numbers them itself,
// in this order, from the head trial counted from.
func appendEntries(entries []store.Entry, id string, cause int, frames []*vttv1.Envelope) []store.Entry {
	for _, f := range frames {
		e := proto.Clone(f).(*vttv1.Envelope)
		e.Sequence = 0
		entries = append(entries, store.Entry{ActorID: id, Cause: cause, Env: e})
	}
	return entries
}

// opening is the present as before holds it: the open session, the party and
// the public notes (VTT-317).
func opening(before *engine.State) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	if s, ok := openSession(before); ok {
		out = append(out, &vttv1.Envelope{SessionId: s.ID,
			Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: s.Name}}})
	}
	// Perch through no eyes introduces exactly the party (SPEC-016).
	out = append(out, perceive.New(perceive.Eyes{}, nil).Perch(perceive.Eyes{}, engine.NewState(), before, 0)...)
	keys := make([]string, 0, len(before.Notes))
	for k, n := range before.Notes {
		if n.Visibility == vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		n := before.Notes[k]
		out = append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key: k, Title: n.Title, Text: n.Text, Visibility: n.Visibility}}})
	}
	return out
}

// restartDecisionsLocked resumes every active decision from its log and
// c.state, for an append that fanned out and then did not land (VTT-325).
func (c *Campaign) restartDecisionsLocked() {
	for id, l := range c.logs {
		if l.decision != nil {
			l.decision = perceive.Resume(eyesOf(id), l.fold, c.state)
		}
	}
}

func openSession(st *engine.State) (engine.Session, bool) {
	for _, s := range st.Sessions {
		if s.EndSeq == 0 {
			return s, true
		}
	}
	return engine.Session{}, false
}
