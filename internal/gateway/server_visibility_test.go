package gateway_test

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// seedAmbush builds session zero's shape on the fixture's campaign: a 32x32
// grid split by a solid wall, the fixture player's fighter west of it, and a
// Goblin Archer nobody controls standing east of it at (19,8) — the exact
// square seq 20 put tok-fighter on.
//
// EVERY square is tiled, all 1024 of them, and that is not thoroughness — it
// is what makes the sight tests mean anything: internal/sight occludes on
// terrain, so a scene with squares nobody declared would answer questions
// about a room that is half missing. Well inside mapdef.MaxWireTiles (3600).
// (This paragraph cited validateCreateSceneTerrain's every-square refusal as
// the reason until 2026-09-02, when create_scene left the platform. The rule
// itself did not leave with it: mapdef.CheckEverySquarePresent holds every map
// FILE that declares any terrain to the same completeness.)
//
// THE SCENE IS APPENDED, NOT COMMANDED, exactly as seedCellar does since the
// same day: what this fixture owes its callers is a STATE, and engine.Apply is
// the seam a direct append reaches identically.
func (f *gwFixture) seedAmbush(t *testing.T) {
	t.Helper()

	tiles := map[string]*vttv1.TileRef{}
	for y := int32(0); y < 32; y++ {
		for x := int32(0); x < 32; x++ {
			kind := "floor"
			if x == 15 {
				kind = "wall"
			}
			tiles[gridKeyForTest(x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	mustAppend(t, f.campaign, "seed-ambush-scene", &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: "ambush", Name: "Ambush Corridor", GridWidth: 32, GridHeight: 32,
			Tiles: tiles,
			Objects: []*vttv1.SceneObject{
				// EAST OF THE WALL, where the player cannot see it, and it
				// BLOCKS MOVE without blocking sight. Its only job is to be a
				// piece of terrain whose KIND a refusal could name — see
				// TestAPlayerCannotProbeTheDarkWithMoveCommands.
				{
					ObjectId: "crate-1", Kind: "crate", At: &vttv1.GridPosition{X: 20, Y: 20},
					Width: 1, Height: 1, BlocksMove: true,
				},
			},
		}})
	dmConn := f.dial(f.dmToken, gwSeedHead)

	// TWO COMMANDS, which is the whole of what this arc's last task changed:
	// add_actor makes a character and grant_actor_control hands it over, SAYING
	// what it is. Sent over the WIRE, unlike the scene above, which this
	// fixture appends directly since 2026-09-02 — so the seed itself exercises
	// the refusal it would hit if it tried the one-step, which is the only
	// reason these two go through the gateway at all.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-fighter",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-fighter", Name: "Asme",
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed AddActor act-fighter: %s", r.Error)
	}
	// Dial the player onto act-fighter: their default, a1, sees nothing here.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-grant-fighter",
		Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "act-fighter", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed GrantActorControl act-fighter: %s", r.Error)
	}
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-tok-fighter",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-fighter", SceneId: "ambush", ActorId: "act-fighter",
			Position: &vttv1.GridPosition{X: 5, Y: 5},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed PlaceToken tok-fighter: %s", r.Error)
	}

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-goblin",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-goblin-archer", Name: "Goblin Archer",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed AddActor act-goblin-archer: %s", r.Error)
	}
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-tok-goblin",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-goblin-archer", SceneId: "ambush", ActorId: "act-goblin-archer",
			Position: &vttv1.GridPosition{X: 19, Y: 8},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed PlaceToken tok-goblin-archer: %s", r.Error)
	}
}

// gridKeyForTest formats a square the way the wire does (maps-as-geometry spec
// §4.1). Spelled out here rather than imported: engine.gridKey is unexported
// and this package's tests are an EXTERNAL package by design.
func gridKeyForTest(x, y int32) string {
	return strconv.Itoa(int(x)) + "," + strconv.Itoa(int(y))
}

// drainEvents collects every Envelope this connection is given until it has
// been quiet for `quiet`. Results are left in the queue untouched, so a caller
// may still read a CommandResult afterwards.
//
// Wait for quiet, never a count: a character log holds what its character
// perceived, not one entry per world event.
func drainEvents(t *testing.T, conn *websocket.Conn, quiet time.Duration) []*vttv1.Envelope {
	t.Helper()
	q := queueFor(conn)
	var out []*vttv1.Envelope
	for {
		select {
		case e := <-q.events:
			out = append(out, e)
		case <-q.closed:
			t.Fatalf("drainEvents: connection closed: %v", q.err)
		case <-time.After(quiet):
			return out
		}
	}
}

// mentions reports whether any envelope in the stream says needle anywhere at
// all — id, name, or free text.
//
// The WHOLE envelope is searched rather than the fields this projection happens
// to build today. A leak that arrives through a field nobody thought to assert
// on is exactly the shape spec §4.4 warns about, and a test that checks
// GetTokenPlaced().GetTokenId() would sail past a Goblin Archer named in a
// narration.
func mentions(t *testing.T, stream []*vttv1.Envelope, needle string) bool {
	t.Helper()
	for _, e := range stream {
		raw, err := protojson.Marshal(e)
		if err != nil {
			t.Fatalf("marshal envelope seq=%d: %v", e.GetSequence(), err)
		}
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// Compare byte for byte against the log: a redaction that dropped one field
// would survive a count or a spot check.
// VTT-176
func TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	log := f.log(t)
	if len(log) == 0 {
		t.Fatal("the fixture must have appended something to compare against")
	}

	for _, seat := range []struct {
		name  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
	} {
		t.Run(seat.name, func(t *testing.T) {
			stream := drainEvents(t, f.dial(seat.token, 0), 500*time.Millisecond)
			if len(stream) != len(log) {
				t.Fatalf("%s received %d envelopes, want %d — one per log event, no more and no fewer",
					seat.name, len(stream), len(log))
			}
			for i := range log {
				want, err := protojson.Marshal(log[i])
				if err != nil {
					t.Fatal(err)
				}
				got, err := protojson.Marshal(stream[i])
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != string(want) {
					t.Fatalf("%s envelope %d differs from the log:\n got %s\nwant %s",
						seat.name, i, got, want)
				}
			}

			// Resume in the world log: replaying history to a client that already
			// folded it is a duplicate-introduction freeze.
			resume := log[len(log)-1].GetSequence() - 1
			tail := drainEvents(t, f.dial(seat.token, resume), 500*time.Millisecond)
			if len(tail) != 1 {
				t.Fatalf("%s resuming at after=%d received %d envelopes, want exactly 1 — "+
					"a world seat resumes where it asked to", seat.name, resume, len(tail))
			}
			if tail[0].GetSequence() != log[len(log)-1].GetSequence() {
				t.Fatalf("%s resuming at after=%d received sequence %d, want %d",
					seat.name, resume, tail[0].GetSequence(), log[len(log)-1].GetSequence())
			}
		})
	}
}

// log returns the whole campaign log in order, which a world seat's stream is
// compared against.
func (f *gwFixture) log(t *testing.T) []*vttv1.Envelope {
	t.Helper()
	events, unsubscribe, head, err := f.campaign.Subscribe(0, 512)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	out := make([]*vttv1.Envelope, 0, head)
	for int64(len(out)) < head {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatalf("log: subscription closed after %d of %d", len(out), head)
			}
			out = append(out, e)
		case <-time.After(3 * time.Second):
			t.Fatalf("log: only %d of %d events arrived", len(out), head)
		}
	}
	return out
}

// TestAPlayerCannotStepWhereItCannotSeeButTheDMCan is the other half of
// session zero's second clause: the refusal is a rule about PLAYERS, not a
// rule about the square. "Hard for players, free for DM" (maps-as-geometry
// spec §6) governs sight exactly as it governs stone, and a DM staging a
// creature across the map is a legitimate thing to do.
// VTT-350 VTT-158
func TestAPlayerCannotStepWhereItCannotSeeButTheDMCan(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	playerConn := f.dialOn(f.playerToken, 0, "act-fighter")
	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "player-reaches-past-the-wall",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 20, Y: 20},
		}},
	})
	r := readResult(t, playerConn)
	if r.Ok {
		t.Fatal("a player reached a square on the far side of a wall they cannot see through")
	}
	// The refusal names the sight rule, which the player already holds (SPEC-013).
	if !strings.Contains(r.Error, "cannot see") {
		t.Fatalf("refusal = %q, want it to name the sight rule", r.Error)
	}

	dmConn := f.dial(f.dmToken, 0)
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "dm-places-the-fighter-anywhere",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 20, Y: 20},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("the DM must be free of the sight rule as they are free of the wall: %s", r.Error)
	}
}

// VTT-180 VTT-341
func TestASpectatorWithNoPerchReceivesNoBoard(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	dmConn := f.dial(f.dmToken, 0)
	watcher := f.dial(f.spectatorToken, 0)

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "dm-narrates",
		Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{
			Text: "The corridor narrows.",
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("dm narration: %s", r.Error)
	}
	if stream := drainEvents(t, watcher, 500*time.Millisecond); len(stream) != 0 {
		t.Fatalf("a spectator perched on nobody was sent %d events, the first %v", len(stream), stream[0])
	}
}

// --- the spectator perch (spec §3.1.1) ------------------------------------

// seedArmak adds a SECOND party member, east of the wall the ambush fixture
// splits its grid with, so that there are two shoulders to hop BETWEEN.
//
// Armak stands one square east of the Goblin Archer; Asme is on the far side of
// the wall at x=15. That is what makes the hop observable in both directions:
// the archer is invisible from one shoulder and in plain sight from the other,
// and Asme herself disappears when the watcher leaves her.
func (f *gwFixture) seedArmak(t *testing.T) {
	t.Helper()
	dmConn := f.dial(f.dmToken, gwSeedHead)

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-scout",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-scout", Name: "Armak",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed AddActor act-scout: %s", r.Error)
	}
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-grant-scout",
		Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "act-scout", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed GrantActorControl act-scout: %s", r.Error)
	}
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-tok-scout",
		Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-scout", SceneId: "ambush", ActorId: "act-scout",
			Position: &vttv1.GridPosition{X: 20, Y: 8},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed PlaceToken tok-scout: %s", r.Error)
	}
}

// perch sends one SetViewpoint and returns the frames it produced.
//
// The frames are drained WITHOUT anything else happening at the table, which is
// the "whenever" in Patrik's sentence: a hop shows you the new view at once
// rather than at the next event, and at a quiet table the next event may be
// minutes away.
func (f *gwFixture) perch(t *testing.T, conn *websocket.Conn, requestID, actorID string) []*vttv1.Envelope {
	t.Helper()
	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: requestID,
		Command: &vttv1.ClientCommand_SetViewpoint{
			SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID},
		},
	})
	if r := readResult(t, conn); !r.Ok {
		t.Fatalf("perch on %q refused: %s", actorID, r.Error)
	}
	return drainEvents(t, conn, 500*time.Millisecond)
}

// VTT-180 VTT-192 VTT-195 VTT-342
func TestASpectatorHopsFromOneShoulderToAnother(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	f.seedArmak(t)

	watcher := f.dial(f.spectatorToken, 0)
	if roster := drainEvents(t, watcher, 300*time.Millisecond); len(roster) != 0 {
		t.Fatalf("a spectator perched on nobody was sent %d events", len(roster))
	}

	west := f.perch(t, watcher, "perch-asme", "act-fighter")
	if !mentions(t, west, "tok-fighter") {
		t.Fatal("sitting on Asme must show Asme's board, and show it at once")
	}
	if mentions(t, west, "goblin") {
		t.Fatal("the Goblin Archer is behind the wall from Asme's shoulder")
	}

	east := f.perch(t, watcher, "perch-armak", "act-scout")
	if !mentions(t, east, "tok-goblin-archer") {
		t.Fatal("from Armak's shoulder the archer is one square away and in plain sight")
	}
	if mentions(t, east, "tok-fighter") {
		t.Fatal("Asme is behind the wall from Armak's shoulder, and his log never placed her")
	}
	for name, run := range map[string][]*vttv1.Envelope{"Asme's": west, "Armak's": east} {
		for i, e := range run {
			if e.GetSequence() != int64(i+1) {
				t.Fatalf("%s log is sent from entry %d at position %d, want its first entry first", name, e.GetSequence(), i+1)
			}
		}
		if _, err := campaign.FoldPrefix(run); err != nil {
			t.Fatalf("%s log does not fold on its own: %v", name, err)
		}
	}
}

// VTT-364
func TestAPublicNoteReachesAPlayersConnectionAndASecretOneDoesNot(t *testing.T) {
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, gwSeedHead)
	playerConn := f.dial(f.playerToken, gwSeedHead)
	for _, n := range []struct {
		key string
		vis vttv1.NoteVisibility
	}{
		{"tavern", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC},
		{"ambush", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET},
	} {
		sendCommand(t, dmConn, &vttv1.ClientCommand{
			RequestId: "note-" + n.key,
			Command: &vttv1.ClientCommand_UpsertNote{UpsertNote: &vttv1.UpsertNote{
				Key: n.key, Title: n.key, Text: "a note called " + n.key, Visibility: n.vis}},
		})
		if r := readResult(t, dmConn); !r.Ok {
			t.Fatalf("upsert_note %s: %s", n.key, r.Error)
		}
	}
	playerStream := drainEvents(t, playerConn, 500*time.Millisecond)
	if !mentions(t, playerStream, "tavern") {
		t.Errorf("a public note never reached the player's connection; got %d envelopes", len(playerStream))
	}
	if mentions(t, playerStream, "ambush") {
		t.Errorf("a secret note reached the player's connection")
	}
}

// Over the real wire: MayWatch is unit-tested in viewpoint_test.go, and this
// proves the server asks it before moving anybody's eyes.
// VTT-181
func TestASpectatorMayNotPerchOnTheGoblinArcher(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	watcher := f.dial(f.spectatorToken, 0)
	drainEvents(t, watcher, 500*time.Millisecond)

	sendCommand(t, watcher, &vttv1.ClientCommand{
		RequestId: "perch-archer",
		Command: &vttv1.ClientCommand_SetViewpoint{
			SetViewpoint: &vttv1.SetViewpoint{ActorId: "act-goblin-archer"},
		},
	})
	if r := readResult(t, watcher); r.Ok {
		t.Fatal("the server must refuse a perch on an NPC, whatever the client offered")
	}
	// AND NOTHING MOVES. A refusal that still shifted the eyes would leak the
	// ambush and merely say no about it.
	if after := drainEvents(t, watcher, 500*time.Millisecond); len(after) != 0 {
		for _, e := range after {
			raw, _ := protojson.Marshal(e)
			t.Logf("after the refused perch: %s", raw)
		}
		t.Fatal("a refused perch must change nothing on the watcher's board")
	}
}

// Hop while the table is busy: a switch and a live entry must not interleave.
// VTT-189 VTT-342 VTT-367
func TestHoppingWhileTheTableIsBusyKeepsOneOrder(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	f.seedArmak(t)

	w := f.watch(f.spectatorToken, 0, "")
	w.opening()

	dmConn := f.dial(f.dmToken, 0)
	drainEvents(t, dmConn, 300*time.Millisecond)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ys := []int32{5, 6, 7, 6}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			sendCommand(t, dmConn, &vttv1.ClientCommand{
				RequestId: "busy-move",
				Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
					TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 5, Y: ys[i%len(ys)]},
				}},
			})
			if r := readResult(t, dmConn); !r.Ok {
				return
			}
		}
	}()

	shoulders := []string{"act-fighter", "act-scout", "", "act-fighter", ""}
	for i := range 40 {
		if r := w.command(setViewpoint(shoulders[i%len(shoulders)])); !r.GetOk() {
			t.Fatalf("hop %d refused: %s", i, r.GetError())
		}
	}
	close(stop)
	wg.Wait()

	var run []*vttv1.Envelope
	runs := 0
	check := func() {
		for i, e := range run {
			if e.GetSequence() != int64(i+1) {
				t.Fatalf("run %d: entry %d arrived at position %d", runs, e.GetSequence(), i+1)
			}
		}
		if _, err := campaign.FoldPrefix(run); err != nil {
			t.Fatalf("run %d of the watcher's stream does not fold: %v", runs, err)
		}
	}
	for fr := w.next(1500 * time.Millisecond); fr != nil; fr = w.next(1500 * time.Millisecond) {
		switch {
		case fr.GetViewpointChanged() != nil:
			check()
			run, runs = nil, runs+1
		case fr.GetEvent() != nil:
			run = append(run, fr.GetEvent())
		}
	}
	check()
	if runs < 2 {
		t.Fatalf("the watcher was switched %d times, want the hops to land", runs)
	}
}

// A spectator perches and the table plays on: the shoulder's log and its live
// tail fold as one stream.
// VTT-342
func TestAPerchedBoardSurvivesTheTablePlayingOnWithoutIt(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	watcher := f.dial(f.spectatorToken, 0)
	roster := drainEvents(t, watcher, 400*time.Millisecond)
	perch := f.perch(t, watcher, "perch-asme", "act-fighter")

	dmConn := f.dial(f.dmToken, 0)
	drainEvents(t, dmConn, 300*time.Millisecond)

	// The party moves, as it would a moment later.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "after-perch-move",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 5, Y: 6},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("move: %s", r.Error)
	}
	tail := drainEvents(t, watcher, 700*time.Millisecond)

	stream := make([]*vttv1.Envelope, 0, len(roster)+len(perch)+len(tail))
	stream = append(stream, roster...)
	stream = append(stream, perch...)
	stream = append(stream, tail...)

	st, err := campaign.FoldPrefix(stream)
	if err != nil {
		t.Fatalf("the watcher's stream does not fold once the table plays on: %v", err)
	}
	if _, ok := st.Scenes["ambush"]; !ok {
		t.Error("the room the watcher was shown is gone from their own folded stream")
	}
	if _, ok := st.Tokens["tok-fighter"]; !ok {
		t.Error("the shoulder they perched on is gone from their own folded stream")
	}
}

// A spectator dialled at the world's head who then perches is sent the
// shoulder's log from its first entry: a cursor counts in the log it names.
// VTT-342 VTT-349
func TestAPerchOnAConnectionThatResumedAtHeadStillSendsTheBoard(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	head := int64(len(f.log(t)))

	watcher := f.dial(f.spectatorToken, head)
	drainEvents(t, watcher, 400*time.Millisecond)

	got := f.perch(t, watcher, "perch-at-head", "act-fighter")
	if !mentions(t, got, "tok-fighter") {
		t.Fatalf("a perch answered ok must show the board, whatever cursor the connection "+
			"resumed from; got %d frame(s)", len(got))
	}
}

// Where a watcher points their camera is not a fact about the campaign: the log
// does not grow, and a DM at the table hears nothing of it.
// VTT-184 VTT-345
func TestPerchingAppendsNothingToTheLog(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	f.seedArmak(t)

	dmConn := f.dial(f.dmToken, 0)
	drainEvents(t, dmConn, 500*time.Millisecond) // the DM's catch-up is not the subject
	before := len(f.log(t))

	watcher := f.dial(f.spectatorToken, 0)
	f.perch(t, watcher, "perch-asme", "act-fighter")
	f.perch(t, watcher, "perch-armak", "act-scout")
	f.perch(t, watcher, "un-perch", "")

	if after := len(f.log(t)); after != before {
		t.Fatalf("perching appended %d event(s) to the campaign's history", after-before)
	}
	if heard := drainEvents(t, dmConn, 500*time.Millisecond); len(heard) != 0 {
		for _, e := range heard {
			raw, _ := protojson.Marshal(e)
			t.Logf("the DM was told: %s", raw)
		}
		t.Fatal("a perch reaches nobody but the watcher who sent it")
	}
}

// TestAPlayerCannotProbeTheDarkWithMoveCommands closes a leak this arc's own
// wiring created, and it is worth stating plainly because it is the shape the
// whole arc is about: redacting the board turned a REFUSAL MESSAGE into a
// terrain oracle.
//
// engine.Blocked answers for any square in the scene, sight-independent — it
// was written when every player received every tile and so leaked nothing.
// Once SceneCreated is redacted and terrain arrives square by square through
// SceneSeen, a refusal that says "a wall", "a closed door" or "something (a
// crate) is in the way" hands back the contents of the black area one
// move_token at a time. Spec §4.2: "you do not know what is in the black area
// before you enter the black area."
//
// So for a PLAYER the sight question is asked FIRST and every unseen
// destination gets the same answer whatever stands on it. The assertion is
// byte-equality between the refusals, not a substring: a message that varied
// by terrain would still be an oracle even if no single word gave it away.
// VTT-156
func TestAPlayerCannotProbeTheDarkWithMoveCommands(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	playerConn := f.dialOn(f.playerToken, 0, "act-fighter")

	probe := func(id string, x, y int32) string {
		t.Helper()
		sendCommand(t, playerConn, &vttv1.ClientCommand{
			RequestId: id,
			Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
				TokenId: "tok-fighter", To: &vttv1.GridPosition{X: x, Y: y},
			}},
		})
		r := readResult(t, playerConn)
		if r.Ok {
			t.Fatalf("%s: (%d,%d) was accepted; this probe only means something for a REFUSED move", id, x, y)
		}
		return r.Error
	}

	// Four unseen squares, four different things standing on them: blocking
	// scenery, a hidden creature, open floor, and nothing at all because the
	// square is off the grid.
	answers := map[string]string{
		"scenery":  probe("probe-crate", 20, 20),
		"creature": probe("probe-goblin", 19, 8),
		"floor":    probe("probe-empty", 25, 25),
		"offgrid":  probe("probe-offgrid", 40, 40),
	}
	var first, firstName string
	for name, got := range answers {
		if first == "" {
			first, firstName = got, name
			continue
		}
		if got != first {
			t.Fatalf("the refusal distinguishes what is standing on an unseen square, which is the oracle:\n"+
				"  %s -> %q\n  %s -> %q", firstName, first, name, got)
		}
	}
	for _, forbidden := range []string{"wall", "door", "crate", "scenery", "in the way", "grid"} {
		if strings.Contains(strings.ToLower(first), forbidden) {
			t.Fatalf("the refusal for an unseen square names terrain (%q): %q", forbidden, first)
		}
	}

	// THE CONTROL, and it is what stops the fix from being "refuse everything
	// with one word". Terrain the player CAN see is terrain they have already
	// been sent, so its refusal still says what is there.
	if got := probe("probe-visible-wall", 15, 5); !strings.Contains(got, "wall") {
		t.Fatalf("a wall the player can see must still be named: %q", got)
	}
}

// VTT-262 VTT-176
func TestAPlayerIsSentAMoveWithoutItsReasonAndTheDMWithIt(t *testing.T) {
	const why = "the floor gives way under the hero"
	f := newGWFixture(t)
	dmConn := f.dial(f.dmToken, gwSeedHead)
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "r-move",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "t1", To: &vttv1.GridPosition{X: 9, Y: 9}, Reason: proto.String(why),
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("want ok=true moving t1, got error %q", r.Error)
	}

	playerStream := drainEvents(t, f.dial(f.playerToken, 0), 500*time.Millisecond)
	moved := false
	for _, e := range playerStream {
		if e.GetTokenMoved().GetTokenId() == "t1" {
			moved = true
		}
	}
	if !moved {
		t.Fatal("the player was not sent its own token's move")
	}
	if mentions(t, playerStream, why) {
		t.Fatal("the player was sent the move's reason")
	}
	if !mentions(t, drainEvents(t, f.dial(f.dmToken, 0), 500*time.Millisecond), why) {
		t.Fatal("the DM was not sent the move's reason")
	}
}

// Keep the destination a remembered square, not an unseen one: its terrain was
// sent, and SPEC-013 refuses the move anyway. Change this test only with that
// rule.
// VTT-350
func TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	dmConn := f.dial(f.dmToken, 0)
	playerConn := f.dialOn(f.playerToken, 0, "act-fighter")

	// The fighter starts at (5,5) and its SceneSeen carries that square's
	// tile, so this player demonstrably HOLDS that terrain — without this the
	// test below would be pinning "unseen", which is already covered.
	seen := drainEvents(t, playerConn, 400*time.Millisecond)
	if !mentions(t, seen, `"5,5"`) {
		t.Fatal("precondition: the player must have been sent the terrain at (5,5), or this " +
			"test pins UNSEEN rather than REMEMBERED and proves nothing new")
	}

	// The DM carries the fighter across the wall — free of the sight rule as
	// it is free of the stone — so (5,5) is now behind it, remembered and
	// unseen.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "dm-carries-the-fighter-across",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 20, Y: 5},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("the DM must be free of the sight rule: %s", r.Error)
	}
	drainEvents(t, playerConn, 400*time.Millisecond)

	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "step-back-onto-remembered-ground",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 5, Y: 5},
		}},
	})
	r := readResult(t, playerConn)
	if r.Ok {
		t.Fatal("a player stepped onto remembered-but-unseen ground — the sight rule is " +
			"documented as forbidding exactly this, so either the rule or its doc comment moved")
	}
	if !strings.Contains(r.Error, "cannot see") {
		t.Fatalf("refusal = %q, want the sight rule", r.Error)
	}
}

// A client reads to the head it is given (cmd/vtt/state_dump.go's drainToHead):
// a head above the seat's last catch-up entry hangs it, one below truncates it.
// Give every seat the last sequence its own catch-up carries.
// VTT-086
func TestEverySeatCanReachTheCatchUpHeadItIsGiven(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	logHead := f.log(t)[len(f.log(t))-1].GetSequence()

	for _, seat := range []struct {
		name  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		t.Run(seat.name, func(t *testing.T) {
			conn := f.dial(seat.token, 0)
			head := expectCatchUpHead(t, conn)
			// expectCatchUpHead reads the socket directly; everything after it
			// goes through the queue as usual.
			stream := drainEvents(t, conn, 500*time.Millisecond)

			var highest int64
			for _, e := range stream {
				if e.GetSequence() > highest {
					highest = e.GetSequence()
				}
			}
			// EQUAL, not merely reachable, and the difference is a truncated
			// snapshot. A head ABOVE the seat's last catch-up envelope is the
			// hang this test was written for. A head BELOW it is the failure
			// CatchUpHead itself was introduced to prevent: `vtt state dump`
			// stops the moment it sees the number it was given, so a head that
			// undershoots prints a state missing everything after it — and
			// looks complete doing so.
			if highest != head {
				t.Fatalf("%s was told to read to sequence %d and its catch-up ends at %d — "+
					"too high and the client waits for a frame that never comes, too low and it "+
					"prints a truncated snapshot that looks whole", seat.name, head, highest)
			}
		})
	}

	// Give the world seats the world log's own head.
	for _, seat := range []struct {
		name  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
	} {
		if head := expectCatchUpHead(t, f.dial(seat.token, 0)); head != logHead {
			t.Fatalf("%s's catch-up head = %d, want the log's own head %d", seat.name, head, logHead)
		}
	}
}

// VTT-272 VTT-273 VTT-176
func TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause(t *testing.T) {
	f := newRulesetFixture(t, true)
	spectator := f.dial(f.spectatorToken, 0)
	sendCommand(t, spectator, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: "patron"}}})
	if r := readResult(t, spectator); !r.Ok {
		t.Fatalf("perch on patron refused: %s", r.Error)
	}
	patron := f.dial(f.patronToken, 0)
	brawler := f.dial(f.brawlerToken, 0)
	dm := f.dial(f.dmToken, 0)

	sendCommand(t, brawler, fistsCmd("brawler", "patron"))
	if r := readResult(t, brawler); !r.Ok {
		t.Fatalf("fists refused: %s", r.Error)
	}
	sendCommand(t, dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveCondition{
		RemoveCondition: &vttv1.RemoveCondition{ActorId: "patron", ConditionId: "dazed-by-ale"}}})
	if r := readResult(t, dm); !r.Ok {
		t.Fatalf("remove_condition refused: %s", r.Error)
	}

	for name, conn := range map[string]*websocket.Conn{"patron": patron, "brawler": brawler, "spectator": spectator} {
		changes := 0
		for _, e := range drainEvents(t, conn, 500*time.Millisecond) {
			if e.GetParticipantId() != "" || e.GetActorRole() != "" {
				t.Errorf("%s: seq %d names its issuer %q as %q", name, e.GetSequence(), e.GetParticipantId(), e.GetActorRole())
			}
			if e.GetResourceChanged().GetReason() != "" || e.GetConditionApplied().GetSource() != "" ||
				e.GetConditionRemoved().GetReason() != "" {
				t.Errorf("%s: seq %d names its cause: %v", name, e.GetSequence(), e)
			}
			if e.GetEventId() != "" && (e.GetResourceChanged() != nil || e.GetConditionApplied() != nil ||
				e.GetConditionRemoved() != nil) {
				changes++
			}
		}
		if changes != 3 {
			t.Errorf("%s: forwarded %d status changes, want the drink, the condition and its removal", name, changes)
		}
	}

	causes := 0
	for _, e := range drainEvents(t, dm, 500*time.Millisecond) {
		if e.GetSequence() > rfSeedHead && (e.GetParticipantId() == "" || e.GetActorRole() == "") {
			t.Errorf("dm: seq %d lost its issuer", e.GetSequence())
		}
		if e.GetResourceChanged().GetReason() != "" || e.GetConditionApplied().GetSource() != "" ||
			e.GetConditionRemoved().GetReason() != "" {
			causes++
		}
	}
	if causes != 3 {
		t.Errorf("dm: %d status changes carry their cause, want 3", causes)
	}
}
