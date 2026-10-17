package campaign_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

func noteDeletedOf(entries []campaign.CharacterEntry, key string) *vttv1.Envelope {
	for _, e := range entries {
		if e.Env.GetNoteDeleted().GetKey() == key {
			return e.Env
		}
	}
	return nil
}

// VTT-361 VTT-362 VTT-363 VTT-364 VTT-365 VTT-219 VTT-239
func TestEveryActiveLogIsToldWhatTheTableIsTold(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), twoRooms()))
	must(t, c, cenv(nextID(), partyMember("hero")))
	must(t, c, cenv(nextID(), partyMember("scout")))
	sage := partyMember("sage")
	sage.Actor.Resources = map[string]*vttv1.Resource{"pool": {Current: 2, Max: 2}}
	must(t, c, cenv(nextID(), sage))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-scout", SceneId: "s", ActorId: "scout", Position: at(5, 1)}))

	public := vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC
	for _, payload := range []any{
		&vttv1.SessionStarted{Name: "Night one"},
		&vttv1.NarrationAdded{Text: "the wind howls"},
		&vttv1.NoteUpserted{Key: "pub-a", Title: "Map", Text: "north", Visibility: public},
		&vttv1.NoteUpserted{Key: "pub-b", Title: "Rumour", Text: "a bell", Visibility: public},
		&vttv1.NoteUpserted{Key: "pub-a", Title: "Map", Text: "north",
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET},
		&vttv1.NoteDeleted{Key: "pub-b"},
		&vttv1.AdventureLoaded{AdventureId: "ambush", Name: "Ambush"},
		&vttv1.ResourceChanged{ActorId: "sage", Resource: "pool", Delta: -1, NewValue: 1},
		&vttv1.SessionEnded{},
	} {
		must(t, c, cenv(nextID(), payload))
	}

	for _, id := range []string{"hero", "scout", "sage"} {
		log := logOf(t, c, id)
		for _, want := range []string{
			"ActorAdded hero", "ActorAdded scout", "ActorAdded sage",
			"SessionStarted", "NarrationAdded the wind howls",
			"NoteUpserted pub-a", "NoteUpserted pub-b", "SessionEnded",
		} {
			if !hasKind(log, want) {
				t.Errorf("%s's log lacks %q: %v", id, want, kindsOf(log))
			}
		}
		for _, e := range log {
			if e.Env.GetAdventureLoaded() != nil {
				t.Errorf("%s's log holds the adventure load: %v", id, kindsOf(log))
			}
		}

		secret, deleted := noteDeletedOf(log, "pub-a"), noteDeletedOf(log, "pub-b")
		if secret == nil || deleted == nil {
			t.Errorf("%s's log lacks a deletion of each note it held: %v", id, kindsOf(log))
			continue
		}
		a, b := proto.Clone(secret).(*vttv1.Envelope), proto.Clone(deleted).(*vttv1.Envelope)
		a.Sequence, b.Sequence = 0, 0
		a.GetNoteDeleted().Key, b.GetNoteDeleted().Key = "", ""
		if !proto.Equal(a, b) {
			t.Errorf("%s's log tells a note made secret from one deleted:\n secret:  %v\n deleted: %v", id, secret, deleted)
		}
	}

	resource := func(id string) bool {
		for _, e := range logOf(t, c, id) {
			if rc := e.Env.GetResourceChanged(); rc.GetActorId() == "sage" && rc.GetNewValue() == 1 {
				return true
			}
		}
		return false
	}
	if !resource("sage") {
		t.Errorf("the tokenless sage's own resource change did not reach its log: %v", kindsOf(logOf(t, c, "sage")))
	}
}
