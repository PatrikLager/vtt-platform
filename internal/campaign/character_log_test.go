package campaign_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func openAt(t *testing.T, dir string) *campaign.Campaign {
	t.Helper()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func rawLog(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func execSQL(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func logOf(t *testing.T, c *campaign.Campaign, actorID string) []campaign.CharacterEntry {
	t.Helper()
	got, err := c.CharacterLog(actorID)
	if err != nil {
		t.Fatalf("CharacterLog(%q): %v", actorID, err)
	}
	return got
}

func envsOf(entries []campaign.CharacterEntry) []*vttv1.Envelope {
	out := make([]*vttv1.Envelope, len(entries))
	for i, e := range entries {
		out[i] = e.Env
	}
	return out
}

// kindsOf names each entry's payload, with the id or key it is about.
func kindsOf(entries []campaign.CharacterEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = describe(e.Env)
	}
	return out
}

func describe(env *vttv1.Envelope) string {
	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_SessionStarted:
		return "SessionStarted"
	case *vttv1.Envelope_SessionEnded:
		return "SessionEnded"
	case *vttv1.Envelope_SceneCreated:
		return "SceneCreated " + p.SceneCreated.GetSceneId()
	case *vttv1.Envelope_ActorAdded:
		return "ActorAdded " + p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		return "ActorControlGranted " + p.ActorControlGranted.GetActorId()
	case *vttv1.Envelope_TokenPlaced:
		return "TokenPlaced " + p.TokenPlaced.GetTokenId()
	case *vttv1.Envelope_TokenMoved:
		return "TokenMoved " + p.TokenMoved.GetTokenId()
	case *vttv1.Envelope_TokenHidden:
		return "TokenHidden " + p.TokenHidden.GetTokenId()
	case *vttv1.Envelope_SceneSeen:
		return "SceneSeen " + p.SceneSeen.GetSceneId()
	case *vttv1.Envelope_ConditionApplied:
		return "ConditionApplied " + p.ConditionApplied.GetActorId() + " " + p.ConditionApplied.GetConditionId()
	case *vttv1.Envelope_NoteUpserted:
		return "NoteUpserted " + p.NoteUpserted.GetKey()
	case *vttv1.Envelope_NarrationAdded:
		return "NarrationAdded " + p.NarrationAdded.GetText()
	case *vttv1.Envelope_ActorRemoved:
		return "ActorRemoved " + p.ActorRemoved.GetActorId()
	default:
		return fmt.Sprintf("%T", p)
	}
}

func assertFoldsAtEveryPrefix(t *testing.T, name string, entries []campaign.CharacterEntry) {
	t.Helper()
	envs := envsOf(entries)
	for i := 1; i <= len(envs); i++ {
		if _, err := campaign.FoldPrefix(envs[:i]); err != nil {
			t.Fatalf("%s does not fold at entry %d of %d (%s): %v", name, i, len(envs), describe(envs[i-1]), err)
		}
	}
}

func hasKind(entries []campaign.CharacterEntry, want string) bool {
	for _, e := range entries {
		if describe(e.Env) == want {
			return true
		}
	}
	return false
}

func partyMember(id string) *vttv1.ActorAdded {
	return &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}
}

func nonParty(id string) *vttv1.ActorAdded {
	return &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}
}

func at(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }

// twoRooms is a hall of two rooms on row 1, x 1-2 and x 4-5, parted by a
// closed door at x 3.
func twoRooms() *vttv1.SceneCreated {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[fmt.Sprintf("%d,0", x)] = &vttv1.TileRef{Kind: "wall"}
		tiles[fmt.Sprintf("%d,2", x)] = &vttv1.TileRef{Kind: "wall"}
	}
	tiles["0,1"] = &vttv1.TileRef{Kind: "wall"}
	tiles["6,1"] = &vttv1.TileRef{Kind: "wall"}
	for _, x := range []int32{1, 2, 4, 5} {
		tiles[fmt.Sprintf("%d,1", x)] = &vttv1.TileRef{Kind: "floor"}
	}
	tiles["3,1"] = &vttv1.TileRef{Kind: "door"}
	return &vttv1.SceneCreated{SceneId: "s", Name: "Hall", GridWidth: 7, GridHeight: 3, Tiles: tiles}
}

// hallWithHero puts the party member hero at x 1 and the goblin at gobX.
func hallWithHero(t *testing.T, c *campaign.Campaign, gobX int32) {
	t.Helper()
	must(t, c, cenv(nextID(), twoRooms()))
	must(t, c, cenv(nextID(), partyMember("hero")))
	must(t, c, cenv(nextID(), nonParty("goblin")))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin", Position: at(gobX, 1)}))
}

// VTT-316
func TestANonPartyActorHasNoLog(t *testing.T) {
	c := openTemp(t)
	hallWithHero(t, c, 2)
	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the goblin snarls"}))
	if got := logOf(t, c, "goblin"); len(got) != 0 {
		t.Fatalf("the goblin, never a party member, has a log of %d: %v", len(got), kindsOf(got))
	}
	hero := logOf(t, c, "hero")
	if !hasKind(hero, "ConditionApplied goblin dazed") || !hasKind(hero, "NarrationAdded the goblin snarls") {
		t.Fatalf("the hero beside the goblin did not get the events it saw: %v", kindsOf(hero))
	}
}

// VTT-317 VTT-372
func TestALateLogOpensWithThePresent(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "Night one"}))
	sessionID := c.State().Sessions[0].ID
	must(t, c, cenv(nextID(), partyMember("bard")))
	must(t, c, cenv(nextID(), &vttv1.NoteUpserted{Key: "pub", Title: "Map", Text: "north",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}))
	must(t, c, cenv(nextID(), &vttv1.NoteUpserted{Key: "sec", Title: "Trap", Text: "pit",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "before"}))
	joined := must(t, c, cenv(nextID(), partyMember("asme")))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "after"}))

	got := logOf(t, c, "asme")
	want := []string{"SessionStarted", "ActorAdded bard", "NoteUpserted pub", "ActorAdded asme", "NarrationAdded after"}
	if strings.Join(kindsOf(got), "; ") != strings.Join(want, "; ") {
		t.Fatalf("asme's log = %v, want %v", kindsOf(got), want)
	}
	if id := got[0].Env.GetSessionId(); id != sessionID {
		t.Errorf("the opening's session is %q, want the open session %q", id, sessionID)
	}
	if name := got[0].Env.GetSessionStarted().GetName(); name != "Night one" {
		t.Errorf("the opening's session is named %q", name)
	}
	for i, e := range got {
		if e.Env.GetSequence() != int64(i+1) {
			t.Errorf("entry %d carries sequence %d", i, e.Env.GetSequence())
		}
		if wantWorld := map[bool]int64{true: 0, false: joined}[i < 3]; i < 4 && e.WorldSeq != wantWorld {
			t.Errorf("entry %d (%s) has world sequence %d, want %d", i, describe(e.Env), e.WorldSeq, wantWorld)
		}
	}
	assertFoldsAtEveryPrefix(t, "asme's log", got)
}

// VTT-317
func TestALogOpensWhenAGrantMakesAPartyMember(t *testing.T) {
	c := openTemp(t)
	hallWithHero(t, c, 2)
	if got := logOf(t, c, "goblin"); len(got) != 0 {
		t.Fatalf("the goblin has a log before the grant: %v", kindsOf(got))
	}
	grant := must(t, c, cenv(nextID(), &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}))
	got := logOf(t, c, "goblin")
	if !hasKind(got, "ActorAdded goblin") || !hasKind(got, "ActorControlGranted goblin") || !hasKind(got, "TokenPlaced t-hero") {
		t.Fatalf("the goblin's log does not open with itself, its grant and what it sees: %v", kindsOf(got))
	}
	for _, e := range got {
		if e.WorldSeq != 0 && e.WorldSeq != grant {
			t.Errorf("%s was written at world sequence %d, before the grant at %d", describe(e.Env), e.WorldSeq, grant)
		}
	}
	assertFoldsAtEveryPrefix(t, "the goblin's log", got)
}

// VTT-317
func TestALogOpensForAPartyMemberThatHadNone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	world := []*vttv1.Envelope{
		cenv(nextID(), &vttv1.SessionStarted{Name: "old"}),
		cenv(nextID(), twoRooms()),
		cenv(nextID(), partyMember("hero")),
		cenv(nextID(), nonParty("goblin")),
		cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}),
		cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin", Position: at(2, 1)}),
	}
	world[0].SessionId = "sess-old"
	if _, err := s.AppendBatch(world, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	c := openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
	got := logOf(t, c, "hero")
	if len(got) == 0 || describe(got[0].Env) != "SessionStarted" || got[0].Env.GetSessionId() != "sess-old" {
		t.Fatalf("the hero's log does not open with the open session: %v", kindsOf(got))
	}
	for _, want := range []string{"ActorAdded hero", "SceneCreated s", "TokenPlaced t-gob", "ConditionApplied goblin dazed"} {
		if !hasKind(got, want) {
			t.Errorf("the hero's log has no %s: %v", want, kindsOf(got))
		}
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
	if n := c.DiscardedForTest(); n != 0 {
		t.Errorf("%d frame set(s) dropped opening a log the campaign never wrote", n)
	}
}

// VTT-325
func TestAFailedFanOutDoesNotAdvanceACharactersMemory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	hallWithHero(t, c, 5)
	before := len(logOf(t, c, "hero"))
	if hasKind(logOf(t, c, "hero"), "ActorAdded goblin") {
		t.Fatal("the hero sees the goblin through the closed door; the fixture proves nothing")
	}

	raw := rawLog(t, dir)
	execSQL(t, raw, `CREATE TRIGGER refuse_entries BEFORE INSERT ON character_events
		BEGIN SELECT RAISE(ABORT, 'refused by the test'); END`)
	move := func() *vttv1.Envelope {
		return cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(5, 1), To: at(2, 1)})
	}
	if _, err := c.Append(move()); err == nil || !strings.Contains(err.Error(), "refused by the test") {
		t.Fatalf("the move's append = %v, want the trigger's refusal", err)
	}
	execSQL(t, raw, `DROP TRIGGER refuse_entries`)
	if got := logOf(t, c, "hero"); len(got) != before {
		t.Fatalf("a failed append changed the hero's log from %d to %d entries", before, len(got))
	}

	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
	if got := logOf(t, c, "hero"); len(got) != before {
		t.Fatalf("a condition on the unseen goblin reached the hero: %v", kindsOf(got[before:]))
	}
	must(t, c, move())
	got := logOf(t, c, "hero")
	if !hasKind(got, "ActorAdded goblin") || !hasKind(got, "ConditionApplied goblin dazed") || !hasKind(got, "TokenPlaced t-gob") {
		t.Fatalf("the move appended again did not introduce the goblin with its condition: %v", kindsOf(got[before:]))
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
	if n := c.DiscardedForTest(); n != 0 {
		t.Errorf("%d frame set(s) dropped after the failed append", n)
	}
}

// droppedHero is a campaign whose hero has just had one frame set dropped, one
// that held the goblin's introduction (TestFramesThatWouldNotFoldAreNotWritten).
func droppedHero(t *testing.T) (string, *campaign.Campaign, int) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "Night one"}))
	hallWithHero(t, c, 2)
	must(t, c, cenv(nextID(), partyMember("bard")))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-bard", SceneId: "s", ActorId: "bard", Position: at(1, 1)}))
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendBatch([]*vttv1.Envelope{cenv(nextID(), &vttv1.NarrationAdded{Text: "aside"})}, []store.Entry{
		{ActorID: "hero", Cause: 0, Env: cenv("", &vttv1.TokenHidden{TokenId: "t-gob"})},
		{ActorID: "hero", Cause: 0, Env: cenv("", &vttv1.ActorRemoved{ActorId: "goblin"})},
		{ActorID: "hero", Cause: 0, Env: cenv("", &vttv1.SessionEnded{})},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	c = openAt(t, dir)
	hero := len(logOf(t, c, "hero"))
	must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))
	if got := logOf(t, c, "hero"); len(got) != hero {
		t.Fatalf("the hero's log took frames that do not fold: %v", kindsOf(got[hero:]))
	}
	if n := c.DiscardedForTest(); n != 1 {
		t.Fatalf("%d frame set(s) dropped, want the hero's one", n)
	}
	return dir, c, hero
}

// assertTheGoblinArrives appends a condition on the goblin and requires the
// hero's log to introduce it, fold, and leave the campaign's drop count at
// dropped.
func assertTheGoblinArrives(t *testing.T, c *campaign.Campaign, hero, dropped int) {
	t.Helper()
	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "poisoned"}))
	got := logOf(t, c, "hero")
	if !hasKind(got[hero:], "ActorAdded goblin") || !hasKind(got[hero:], "ConditionApplied goblin poisoned") {
		t.Fatalf("the next event did not reach the hero after the drop: %v", kindsOf(got[hero:]))
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
	if n := c.DiscardedForTest(); n != dropped {
		t.Errorf("the drop repeated: %d frame sets dropped, want %d", n, dropped)
	}
}

// VTT-320
func TestFramesThatWouldNotFoldAreNotWritten(t *testing.T) {
	_, c, hero := droppedHero(t)
	if !hasKind(logOf(t, c, "bard"), "SessionEnded") {
		t.Fatalf("the bard's log lost the event the hero's drop was about: %v", kindsOf(logOf(t, c, "bard")))
	}
	if len(c.State().Sessions) != 1 || c.State().Sessions[0].EndSeq == 0 {
		t.Fatal("the world event did not land")
	}
	// The dropped frames held the goblin's introduction: the next event about it
	// must introduce it again rather than be dropped too.
	assertTheGoblinArrives(t, c, hero, 1)
}

// VTT-325
func TestAFailedAppendAfterADropDoesNotRepeatIt(t *testing.T) {
	dir, c, hero := droppedHero(t)
	raw := rawLog(t, dir)
	execSQL(t, raw, `CREATE TRIGGER refuse_entries BEFORE INSERT ON character_events
		BEGIN SELECT RAISE(ABORT, 'refused by the test'); END`)
	if _, err := c.Append(cenv(nextID(), &vttv1.NarrationAdded{Text: "lost"})); err == nil {
		t.Fatal("the trigger did not refuse the append")
	}
	execSQL(t, raw, `DROP TRIGGER refuse_entries`)
	assertTheGoblinArrives(t, c, hero, 1)
}

func TestAReopenAfterADropDoesNotRepeatIt(t *testing.T) {
	dir, c, hero := droppedHero(t)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	assertTheGoblinArrives(t, openAt(t, dir), hero, 0)
}

// VTT-317
func TestALogOpeningMidBatchIsTheWorldBeforeItsEvent(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), partyMember("bard")))
	if _, err := c.AppendBatch([]*vttv1.Envelope{
		cenv(nextID(), &vttv1.SessionStarted{Name: "Night one"}),
		cenv(nextID(), &vttv1.NoteUpserted{Key: "pub", Title: "Map", Text: "north",
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}),
		cenv(nextID(), partyMember("asme")),
	}); err != nil {
		t.Fatal(err)
	}
	must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))
	got := logOf(t, c, "asme")
	want := "SessionStarted; ActorAdded bard; NoteUpserted pub; ActorAdded asme; SessionEnded"
	if j := strings.Join(kindsOf(got), "; "); j != want {
		t.Fatalf("asme's log = %s, want %s", j, want)
	}
	for _, e := range got[:3] {
		if e.WorldSeq != 0 {
			t.Errorf("opening entry %s carries world sequence %d", describe(e.Env), e.WorldSeq)
		}
	}
	if n := c.DiscardedForTest(); n != 0 {
		t.Errorf("%d frame set(s) dropped", n)
	}
}

// VTT-324 VTT-326
func TestACharacterLogContinuesAfterReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	hallWithHero(t, c, 2)
	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "first night"}))
	kept := logOf(t, c, "hero")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	c = openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(2, 1), To: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "second night"}))
	got := logOf(t, c, "hero")
	if len(got) <= len(kept) {
		t.Fatalf("the hero's log did not grow after the reopen: %v", kindsOf(got))
	}
	for i, e := range kept {
		if !proto.Equal(e.Env, got[i].Env) || e.WorldSeq != got[i].WorldSeq {
			t.Fatalf("entry %d changed across the reopen: %s became %s", i, describe(e.Env), describe(got[i].Env))
		}
	}
	for i, e := range got[len(kept):] {
		if want := int64(len(kept) + i + 1); e.Env.GetSequence() != want {
			t.Errorf("entry %s after the reopen carries sequence %d, want %d", describe(e.Env), e.Env.GetSequence(), want)
		}
		switch e.Env.GetPayload().(type) {
		case *vttv1.Envelope_SceneCreated, *vttv1.Envelope_ActorAdded, *vttv1.Envelope_TokenPlaced, *vttv1.Envelope_SessionStarted:
			t.Errorf("the reopen introduced %s again", describe(e.Env))
		}
	}
	if !hasKind(got[len(kept):], "TokenMoved t-gob") || !hasKind(got[len(kept):], "NarrationAdded second night") {
		t.Errorf("the events after the reopen did not reach the hero: %v", kindsOf(got[len(kept):]))
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
	if n := c.DiscardedForTest(); n != 0 {
		t.Errorf("%d frame set(s) dropped after the reopen", n)
	}
}

// VTT-327
func TestOpenRefusesACharacterLogThatDoesNotFold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	hallWithHero(t, c, 2)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendBatch([]*vttv1.Envelope{cenv(nextID(), &vttv1.NarrationAdded{Text: "aside"})}, []store.Entry{
		{ActorID: "hero", Cause: 0, Env: cenv("", &vttv1.TokenMoved{TokenId: "t-nobody", SceneId: "s", To: at(1, 1)})},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if c, err := campaign.Open(dir); err == nil {
		c.Close()
		t.Fatal("Open accepted a character log that does not fold")
	} else if !strings.Contains(err.Error(), "hero") {
		t.Fatalf("Open's refusal does not name the actor: %v", err)
	}
}

func TestASceneLoadInOneBatchGivesTheLogItsBoardInOrder(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), partyMember("hero")))
	if _, err := c.AppendBatch([]*vttv1.Envelope{
		cenv(nextID(), twoRooms()),
		cenv(nextID(), nonParty("goblin")),
		cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}),
		cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin", Position: at(2, 1)}),
	}); err != nil {
		t.Fatal(err)
	}
	got := logOf(t, c, "hero")
	for _, want := range []string{"SceneCreated s", "ActorAdded goblin", "TokenPlaced t-hero", "TokenPlaced t-gob", "SceneSeen s"} {
		if !hasKind(got, want) {
			t.Errorf("the hero's log has no %s: %v", want, kindsOf(got))
		}
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
}

func TestWhatTheCampaignHoldsOfALogIsTheFoldOfThatLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	must(t, c, cenv(nextID(), partyMember("bard")))
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "Night one"}))
	hallWithHero(t, c, 2)
	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
	must(t, c, cenv(nextID(), &vttv1.NoteUpserted{Key: "pub", Title: "Map", Text: "north",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}))
	if _, err := c.AppendBatch([]*vttv1.Envelope{
		cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(2, 1), To: at(1, 1)}),
		cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "poisoned"}),
		cenv(nextID(), &vttv1.SessionEnded{}),
	}); err != nil {
		t.Fatal(err)
	}
	check := func(when string) {
		t.Helper()
		for _, id := range []string{"bard", "hero"} {
			stored, err := campaign.FoldPrefix(envsOf(logOf(t, c, id)))
			if err != nil {
				t.Fatalf("%s: %s's stored log does not fold: %v", when, id, err)
			}
			// Compare snapshots: a fold from scratch leaves nil maps Snapshot makes.
			if held := c.CharacterFoldForTest(id); !statesEqual(stored.Snapshot(), held) {
				t.Fatalf("%s: the campaign holds %s's log as\n%+v\nbut it folds to\n%+v", when, id, held, stored)
			}
		}
	}
	check("live")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c = openAt(t, dir)
	check("reopened")
}

// VTT-320
func TestADropThatLeftEveryActorHeldKeepsForwardingWhatTheCharacterSees(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "Night one"}))
	hallWithHero(t, c, 2)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendBatch([]*vttv1.Envelope{cenv(nextID(), &vttv1.NarrationAdded{Text: "aside"})}, []store.Entry{
		{ActorID: "hero", Cause: 0, Env: cenv("", &vttv1.SessionEnded{})},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	c = openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))
	if n := c.DiscardedForTest(); n != 1 {
		t.Fatalf("%d frame set(s) dropped, want the hero's SessionEnded", n)
	}
	hero := len(logOf(t, c, "hero"))
	must(t, c, cenv(nextID(), &vttv1.AttackRolled{AttackerId: "hero", TargetId: "goblin", Total: 7}))
	got := logOf(t, c, "hero")
	if !hasKind(got[hero:], "*vttv1.Envelope_AttackRolled") {
		t.Fatalf("the attack between two actors the hero holds was withheld after the drop: %v", kindsOf(got[hero:]))
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
}

// anchoredIn returns the anchor range of the narration text in entries, and
// the range VTT-322 asks for: the first to the last entry whose world event
// lies in [from, to], leaving out the opening, or 0, 0 when none does.
func anchoredIn(t *testing.T, entries []campaign.CharacterEntry, text string, from, to int64) (got, want [2]int64) {
	t.Helper()
	found := false
	for _, e := range entries {
		if n := e.Env.GetNarrationAdded(); n != nil && n.GetText() == text {
			got, found = [2]int64{n.GetAnchorFromSeq(), n.GetAnchorToSeq()}, true
		}
		if e.WorldSeq != 0 && e.WorldSeq >= from && e.WorldSeq <= to {
			if want[0] == 0 {
				want[0] = e.Env.GetSequence()
			}
			want[1] = e.Env.GetSequence()
		}
	}
	if !found {
		t.Fatalf("the log holds no narration %q: %v", text, kindsOf(entries))
	}
	return got, want
}

func assertNoneDropped(t *testing.T, c *campaign.Campaign) {
	t.Helper()
	if n := c.DiscardedForTest(); n != 0 {
		t.Errorf("%d frame set(s) dropped", n)
	}
}

func anchored(text string, from, to int64) *vttv1.NarrationAdded {
	return &vttv1.NarrationAdded{Text: text, AnchorFromSeq: from, AnchorToSeq: to}
}

// VTT-322
func TestAnAnchoredNarrationNamesTheEntriesOfItsOwnLog(t *testing.T) {
	t.Run("a range wholly seen", func(t *testing.T) {
		c := openTemp(t)
		hallWithHero(t, c, 2)
		from := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
		to := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "poisoned"}))
		// A seen event after the range, so its end is not the log's last entry.
		must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "slowed"}))
		must(t, c, cenv(nextID(), anchored("the goblin reels", from, to)))
		hero := logOf(t, c, "hero")
		got, want := anchoredIn(t, hero, "the goblin reels", from, to)
		if want[0] == 0 || want[0] == want[1] || got != want {
			t.Fatalf("anchor %v, want %v (two entries apart)", got, want)
		}
		assertFoldsAtEveryPrefix(t, "the hero's log", hero)
		assertNoneDropped(t, c)
	})
	t.Run("a range with several entries for one world event", func(t *testing.T) {
		c := openTemp(t)
		hallWithHero(t, c, 5)
		door := must(t, c, cenv(nextID(), &vttv1.DoorOpened{SceneId: "s", At: at(3, 1)}))
		must(t, c, cenv(nextID(), anchored("the door swings wide", door, door)))
		hero := logOf(t, c, "hero")
		got, want := anchoredIn(t, hero, "the door swings wide", door, door)
		if want[0] == want[1] || got != want {
			t.Fatalf("anchor %v, want %v (every entry the door gave)", got, want)
		}
		assertFoldsAtEveryPrefix(t, "the hero's log", hero)
		assertNoneDropped(t, c)
	})
	t.Run("a range that starts before the log opened", func(t *testing.T) {
		c := openTemp(t)
		hallWithHero(t, c, 2)
		from := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
		must(t, c, cenv(nextID(), partyMember("asme")))
		to := must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-asme", SceneId: "s", ActorId: "asme", Position: at(2, 1)}))
		must(t, c, cenv(nextID(), anchored("asme arrives", from, to)))
		asme := logOf(t, c, "asme")
		got, want := anchoredIn(t, asme, "asme arrives", from, to)
		if want[0] <= 1 || got != want {
			t.Fatalf("anchor %v, want %v (no opening entry)", got, want)
		}
		assertFoldsAtEveryPrefix(t, "asme's log", asme)
		// The hero is handed the same narration: its range is its own.
		hero := logOf(t, c, "hero")
		if got, want := anchoredIn(t, hero, "asme arrives", from, to); got != want || got == ([2]int64{}) {
			t.Fatalf("the hero's anchor %v, want %v", got, want)
		}
		assertNoneDropped(t, c)
	})
	t.Run("a range in the same batch", func(t *testing.T) {
		c := openTemp(t)
		hallWithHero(t, c, 2)
		first, err := c.AppendBatch([]*vttv1.Envelope{
			cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}),
			cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "poisoned"}),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.AppendBatch([]*vttv1.Envelope{
			cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "slowed"}),
			cenv(nextID(), anchored("the goblin staggers", first, first+2)),
		}); err != nil {
			t.Fatal(err)
		}
		hero := logOf(t, c, "hero")
		got, want := anchoredIn(t, hero, "the goblin staggers", first, first+2)
		if want[1]-want[0] != 2 || got != want {
			t.Fatalf("anchor %v, want %v (across the two batches)", got, want)
		}
		assertFoldsAtEveryPrefix(t, "the hero's log", hero)
		assertNoneDropped(t, c)
	})
	t.Run("a range across a reopen", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "campaign")
		c := openAt(t, dir)
		hallWithHero(t, c, 2)
		from := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		c = openAt(t, dir)
		to := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "poisoned"}))
		must(t, c, cenv(nextID(), anchored("the goblin sags", from, to)))
		hero := logOf(t, c, "hero")
		got, want := anchoredIn(t, hero, "the goblin sags", from, to)
		if want[0] == want[1] || got != want {
			t.Fatalf("anchor %v, want %v (the entry before the reopen included)", got, want)
		}
		assertFoldsAtEveryPrefix(t, "the hero's log", hero)
		assertNoneDropped(t, c)
	})
	t.Run("a range none of whose events reached the log", func(t *testing.T) {
		c := openTemp(t)
		hallWithHero(t, c, 5)
		unseen := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
		// A seen event after the range, so an anchor running to the log's last
		// entry is not mistaken for none.
		must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the hero waits"}))
		must(t, c, cenv(nextID(), anchored("far off, a groan", unseen, unseen)))
		hero := logOf(t, c, "hero")
		if got, _ := anchoredIn(t, hero, "far off, a groan", unseen, unseen); got != [2]int64{} {
			t.Fatalf("anchor %v, want unanchored", got)
		}
		assertFoldsAtEveryPrefix(t, "the hero's log", hero)
		assertNoneDropped(t, c)
	})
}

func grant(actorID string, kind vttv1.ActorKind) *vttv1.ActorControlGranted {
	return &vttv1.ActorControlGranted{ActorId: actorID, ParticipantId: "p-1", Kind: kind}
}

// assertBoardAndSessionAsTheWorld requires actorID's log to hold the world's
// open session, and every token it holds on the world's square.
func assertBoardAndSessionAsTheWorld(t *testing.T, c *campaign.Campaign, actorID string) {
	t.Helper()
	held, world := c.CharacterFoldForTest(actorID), c.State()
	open := func(sessions []engine.Session) string {
		for _, s := range sessions {
			if s.EndSeq == 0 {
				return s.ID
			}
		}
		return ""
	}
	if h, w := open(held.Sessions), open(world.Sessions); h != w {
		t.Errorf("%s's log holds open session %q, the world %q", actorID, h, w)
	}
	for id, tok := range held.Tokens {
		if w, ok := world.Tokens[id]; !ok || w.X != tok.X || w.Y != tok.Y || w.SceneID != tok.SceneID {
			t.Errorf("%s's log holds %s at %d,%d; the world at %+v", actorID, id, tok.X, tok.Y, w)
		}
	}
}

// VTT-316 VTT-318
func TestALogPausesWhileItsActorIsNotAPartyMember(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "moves and a new session", true: "an id reused while paused"}[reuse], func(t *testing.T) {
			c := openTemp(t)
			must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "one"}))
			hallWithHero(t, c, 2)
			pause := must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)))
			held := len(logOf(t, c, "hero"))
			must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "while possessed"}))
			must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))
			must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "two"}))
			if reuse {
				if _, err := c.AppendBatch([]*vttv1.Envelope{
					cenv(nextID(), &vttv1.TokenRemoved{TokenId: "t-gob"}),
					cenv(nextID(), &vttv1.ActorRemoved{ActorId: "goblin"}),
				}); err != nil {
					t.Fatal(err)
				}
				changeling := nonParty("goblin")
				changeling.Actor.ModuleId = "changeling"
				must(t, c, cenv(nextID(), changeling))
				must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-gob-2", SceneId: "s", ActorId: "goblin", Position: at(1, 1)}))
			} else {
				must(t, c, cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(2, 1), To: at(1, 1)}))
			}
			must(t, c, cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s", From: at(1, 1), To: at(2, 1)}))
			if got := logOf(t, c, "hero"); len(got) != held {
				t.Fatalf("the paused log grew: %v", kindsOf(got[held:]))
			}
			resume := must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)))
			got := logOf(t, c, "hero")
			for _, e := range got {
				if e.WorldSeq >= pause && e.WorldSeq < resume {
					t.Errorf("%s was written at world event %d, while the hero was paused", describe(e.Env), e.WorldSeq)
				}
			}
			for _, e := range got[held:] {
				switch d := describe(e.Env); d {
				case "ActorAdded hero", "SceneCreated s", "SessionStarted":
					if d != "SessionStarted" || e.WorldSeq != 0 {
						t.Errorf("the resume wrote %s, a second opening", d)
					}
				}
			}
			assertBoardAndSessionAsTheWorld(t, c, "hero")
			if reuse && c.CharacterFoldForTest("hero").Actors["goblin"].GetModuleId() != "changeling" {
				t.Errorf("the hero's log holds the old goblin after the resume")
			}
			assertFoldsAtEveryPrefix(t, "the hero's log", got)
			assertNoneDropped(t, c)
		})
	}
}

// VTT-318
func TestANewActorUnderAnOldIdContinuesItsLog(t *testing.T) {
	c := openTemp(t)
	hallWithHero(t, c, 2)
	if _, err := c.AppendBatch([]*vttv1.Envelope{
		cenv(nextID(), &vttv1.TokenRemoved{TokenId: "t-hero"}),
		cenv(nextID(), &vttv1.ActorRemoved{ActorId: "hero"}),
	}); err != nil {
		t.Fatal(err)
	}
	held := len(logOf(t, c, "hero"))
	second := partyMember("hero")
	second.Actor.Name = "Second hero"
	must(t, c, cenv(nextID(), second))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero-2", SceneId: "s", ActorId: "hero", Position: at(1, 1)}))
	got := logOf(t, c, "hero")
	if len(got) <= held || got[held].Env.GetSequence() != int64(held+1) {
		t.Fatalf("the log did not continue after entry %d: %v", held, kindsOf(got))
	}
	removed, added := -1, -1
	for i, e := range got[held:] {
		switch describe(e.Env) {
		case "ActorRemoved hero":
			if removed < 0 {
				removed = i
			}
		case "ActorAdded hero":
			added = i
		}
	}
	if removed < 0 || added < removed {
		t.Fatalf("the old hero is not withdrawn before the new one is introduced: %v", kindsOf(got[held:]))
	}
	if name := c.CharacterFoldForTest("hero").Actors["hero"].GetName(); name != "Second hero" {
		t.Errorf("the log holds hero %q", name)
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
	assertNoneDropped(t, c)
}

// VTT-328
func TestAnAnchorAcrossAPauseNamesOnlyWhatCameAfterTheResume(t *testing.T) {
	for _, reopen := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "after a reopen"}[reopen], func(t *testing.T) {
			anchorAcrossAPause(t, reopen)
		})
	}
}

func anchorAcrossAPause(t *testing.T, reopen bool) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	hallWithHero(t, c, 2)
	from := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"}))
	must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)))
	must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "poisoned"}))
	resumed := must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)))
	to := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "slowed"}))
	if reopen {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		c = openAt(t, dir)
	}
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the hero waits"}))
	must(t, c, cenv(nextID(), anchored("the fight was hard", from, to)))
	hero := logOf(t, c, "hero")

	// What the resuming event and those after it caused, never what came before.
	_, all := anchoredIn(t, hero, "the fight was hard", from, to)
	got, want := anchoredIn(t, hero, "the fight was hard", resumed, to)
	if want[0] <= all[0] || got != want {
		t.Fatalf("anchor %v, want %v: from the resume at world event %d, not the dazed entry %d", got, want, resumed, all[0])
	}
	assertFoldsAtEveryPrefix(t, "the hero's log", hero)
	assertNoneDropped(t, c)
}

// VTT-328
func TestAnAnchorEndingAtAResumeThatCausedNothingIsUnanchored(t *testing.T) {
	for _, reopen := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "after a reopen"}[reopen], func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "campaign")
			c := openAt(t, dir)
			must(t, c, cenv(nextID(), partyMember("hero")))
			must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)))
			from := must(t, c, cenv(nextID(), &vttv1.ConditionApplied{ActorId: "hero", ConditionId: "dazed"}))
			must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)))
			resumed := must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)))
			if reopen {
				if err := c.Close(); err != nil {
					t.Fatal(err)
				}
				c = openAt(t, dir)
			}
			must(t, c, cenv(nextID(), anchored("across", from, resumed)))
			hero := logOf(t, c, "hero")
			for _, e := range hero {
				if e.WorldSeq == resumed {
					t.Fatalf("fixture: the resume caused %s, so this proves nothing", describe(e.Env))
				}
			}
			if got, all := anchoredIn(t, hero, "across", from, resumed); got != [2]int64{} || all == ([2]int64{}) {
				t.Fatalf("anchor %v, want unanchored: the range holds %v, all before the pause", got, all)
			}
			assertNoneDropped(t, c)
		})
	}
}
