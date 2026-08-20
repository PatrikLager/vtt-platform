package gateway_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// --- session zero ---------------------------------------------------------
//
// Every test in this file needs a PLAYER seat. The DM and the agent receive the
// identity projection (spec §3.1, exit criterion 8), so a test that exercises
// only a DM proves nothing about the projection — it would pass unchanged
// against the server that leaked the goblin.

// seedAmbush builds session zero's shape on the fixture's campaign: a 32x32
// grid split by a solid wall, the fixture player's fighter west of it, and a
// Goblin Archer nobody controls standing east of it at (19,8) — the exact
// square seq 20 put tok-fighter on.
//
// EVERY square is tiled, all 1024 of them, and that is not thoroughness — it
// is create_scene's rule. A scene may declare no tiles at all (the tiles-
// optional ruling, 2026-08-13), but a scene that declares SOME is required to
// name them all: validateCreateSceneTerrain answers a partial map with
// `tiles["0,0"] — no tile named for this square`. Well inside
// mapdef.MaxWireTiles (3600).
//
// Seeded as DM COMMANDS over the wire rather than appended to the campaign
// directly, exactly as seedCellar does: the same Append path the rest of the
// suite uses, and one DB handle.
func (f *gwFixture) seedAmbush(t *testing.T) {
	t.Helper()
	dmConn := f.dial(f.dmToken, 4)

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
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-scene",
		Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
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
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed CreateScene ambush: %s", r.Error)
	}

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-fighter",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-fighter", Name: "Asme", ControllerId: f.playerID},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed AddActor act-fighter: %s", r.Error)
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
			Actor: &vttv1.Actor{ActorId: "act-goblin-archer", Name: "Goblin Archer"},
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
// A quiet window rather than a count: the whole point of a projected stream is
// that the number of envelopes a seat receives is NOT the number the log holds,
// so any fixed count would encode today's projection into the test.
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

// TestSessionZeroCannotHappenAgain replays the defect this whole arc exists
// for. Session zero, 2026-08-12: at seq 20 a player put tok-fighter on (19,8),
// the Goblin Archer's exact square on a 32x32 grid, having simply looked at the
// board — "Landing precisely on the archer's hidden square isn't a plausible
// accident."
//
// Exit criterion 7 has three clauses and this test has three assertions, one
// each: the goblin is NOT ON THE WIRE reaching a player, its square CANNOT BE
// TARGETED by a player who cannot see it, and — the control that keeps the
// other two from passing vacuously — the same player still receives its own
// board and can still move on it.
func TestSessionZeroCannotHappenAgain(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	dmConn := f.dial(f.dmToken, 0)
	playerConn := f.dial(f.playerToken, 0)

	dmStream := drainEvents(t, dmConn, 500*time.Millisecond)
	playerStream := drainEvents(t, playerConn, 500*time.Millisecond)

	// The seed reached the wire at all. Without this the first assertion below
	// passes on a server that broadcasts nothing.
	if !mentions(t, dmStream, "goblin") {
		t.Fatalf("the DM must receive the goblin (identity projection, exit criterion 8); "+
			"got %d envelopes and none names it", len(dmStream))
	}
	// The control, and it is load-bearing: a projection that sends a player
	// NOTHING would satisfy the goblin assertion perfectly.
	if !mentions(t, playerStream, "tok-fighter") {
		t.Fatalf("the player must still receive their own token; got %d envelopes and none names it",
			len(playerStream))
	}
	if !mentions(t, playerStream, "ambush") {
		t.Fatalf("the player must still receive the board they stand on; got %d envelopes and none names it",
			len(playerStream))
	}

	// Clause 1: not on the wire.
	if mentions(t, playerStream, "goblin") {
		for _, e := range playerStream {
			raw, _ := protojson.Marshal(e)
			if strings.Contains(strings.ToLower(string(raw)), "goblin") {
				t.Errorf("the goblin reached a player's connection: %s", raw)
			}
		}
		t.Fatalf("session zero: the Goblin Archer is on a player's wire")
	}

	// Clause 2: its square cannot be targeted by a player who cannot see it.
	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "seq-20",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 19, Y: 8},
		}},
	})
	if r := readResult(t, playerConn); r.Ok {
		t.Fatalf("session zero seq 20: a player moved onto the hidden archer's square (19,8)")
	}

	// The control for clause 2. A check that refuses every move would satisfy
	// it and take the game with it.
	sendCommand(t, playerConn, &vttv1.ClientCommand{
		RequestId: "an-ordinary-step",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-fighter", To: &vttv1.GridPosition{X: 6, Y: 5},
		}},
	})
	if r := readResult(t, playerConn); !r.Ok {
		t.Fatalf("a player must still be able to step onto a square they can see: %s", r.Error)
	}
}

// TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection is exit criterion 8,
// and it is the test that has to exist for every other test in this file to
// mean anything: the projection is now in the delivery path of EVERY
// connection, so "the DM sees everything" stopped being a property of the
// server's shape and became a property of one switch.
//
// Compared BYTE FOR BYTE against the log itself — protojson of each envelope,
// in order — rather than by counting frames or spot-checking payload kinds. A
// redaction that dropped one field would survive both of those.
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

			// AND THEIR RESUME IS UNCHANGED TOO, which is a separate claim and
			// was the half a from-zero comparison could not see. A projected
			// seat is subscribed from the beginning whatever `after` it asked
			// for, because its projector has to be fed the log to know what
			// that seat holds. An unprojected seat must NOT be: it needs no
			// such memory, and replaying history to a client that already
			// folded it is the duplicate-introduction freeze from the other
			// direction.
			resume := log[len(log)-1].GetSequence() - 1
			tail := drainEvents(t, f.dial(seat.token, resume), 500*time.Millisecond)
			if len(tail) != 1 {
				t.Fatalf("%s resuming at after=%d received %d envelopes, want exactly 1 — "+
					"an unprojected seat resumes where it asked to", seat.name, resume, len(tail))
			}
			if tail[0].GetSequence() != log[len(log)-1].GetSequence() {
				t.Fatalf("%s resuming at after=%d received sequence %d, want %d",
					seat.name, resume, tail[0].GetSequence(), log[len(log)-1].GetSequence())
			}
		})
	}
}

// log returns the whole campaign log in order, as the source of truth an
// unprojected stream is compared against.
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

// TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway is the seeding
// contract, asserted where it bites: on a real socket, across a real
// reconnect.
//
// A projected seat's projector MUST be fed the log from the beginning and have
// its output discarded up to the resume point — never constructed AT the
// resume point. The difference is invisible in every other test and is exactly
// one leak: a token that was on the player's board when they dropped, and left
// view while they were gone, gets its TokenHidden from the replay of the gap.
// A projector built at the resume point never knew the token was there, so it
// synthesizes no departure and the enemy stays on that player's board for the
// rest of the session — the direction spec §4.4 forbids.
func TestAReconnectingPlayerIsToldWhatLeftViewWhileItWasAway(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	// The goblin starts WEST of the wall, in plain sight of the fighter at
	// (5,5), so the player's board legitimately holds it.
	dmConn := f.dial(f.dmToken, 0)
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "goblin-steps-into-view",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-goblin-archer", To: &vttv1.GridPosition{X: 14, Y: 8},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("moving the goblin into view: %s", r.Error)
	}

	playerConn := f.dial(f.playerToken, 0)
	seen := drainEvents(t, playerConn, 500*time.Millisecond)
	if !mentions(t, seen, "tok-goblin-archer") {
		t.Fatal("precondition: the player must be able to see the goblin at (14,8) — " +
			"nothing below tests anything if it was never on their board")
	}
	// The resume cursor a client would carry: the highest sequence it saw.
	var resume int64
	for _, e := range seen {
		if e.GetSequence() > resume {
			resume = e.GetSequence()
		}
	}
	playerConn.CloseNow()

	// GONE WHILE THEY WERE AWAY: the goblin steps back behind the wall.
	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "goblin-steps-back",
		Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
			TokenId: "tok-goblin-archer", To: &vttv1.GridPosition{X: 19, Y: 8},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("moving the goblin out of view: %s", r.Error)
	}

	back := drainEvents(t, f.dial(f.playerToken, resume), 500*time.Millisecond)
	hidden := false
	for _, e := range back {
		if e.GetTokenHidden().GetTokenId() == "tok-goblin-archer" {
			hidden = true
		}
	}
	if !hidden {
		var got []string
		for _, e := range back {
			raw, _ := protojson.Marshal(e)
			got = append(got, string(raw))
		}
		t.Fatalf("a reconnecting player was never told the goblin left view — it is still on their board.\n"+
			"catch-up from after=%d was:\n%s", resume, strings.Join(got, "\n"))
	}
	// And nothing about where it went.
	if mentions(t, back, "\"x\":19") {
		t.Fatal("the catch-up named the square the goblin retreated to")
	}

	// AND NOTHING IT ALREADY HAD, which is the other half of the seeding
	// contract and the half that has no second chance. The projector is fed
	// the whole log so that it KNOWS what this seat holds; if its output is
	// not discarded up to the resume point the seat is handed its own
	// introductions a second time, and a duplicate ActorAdded is a hard error
	// in both folds — engine.Apply's "actor %q already exists" and
	// client/src/fold.ts's `duplicate actor` — which freezes that client's
	// state permanently, because Session re-folds its whole append-only log
	// on every event that follows.
	for _, e := range back {
		if e.GetSequence() <= resume {
			raw, _ := protojson.Marshal(e)
			t.Fatalf("catch-up from after=%d re-sent something the seat already holds: %s", resume, raw)
		}
	}
}

// TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive is spec §7's PURITY
// property at the wire: "the projection computed live and from catch-up are
// identical. That is what makes a mid-fight reconnect safe."
//
// It is also the test that pins WHICH state the pump projects against. Every
// event has to be judged against the state that event produced — the fold of
// the log so far — and the tempting shortcut is the live projection, which the
// server already has in its hand. That shortcut is invisible on a live
// connection, where head and now are the same thing, and wrong on every
// catch-up, where head is the future. Here the two streams have to match, so
// it cannot hide.
func TestAPlayersCatchUpIsTheSameStreamItWouldHaveSeenLive(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	live := f.dial(f.playerToken, 0)
	drainEvents(t, live, 300*time.Millisecond) // settle the seed

	dmConn := f.dial(f.dmToken, 0)
	for _, step := range []struct {
		id    string
		token string
		x, y  int32
	}{
		// Into view, along the wall, and back out of it — the "partial
		// visibility of a moving token" edge spec §8 says to watch hardest.
		{"g-in", "tok-goblin-archer", 14, 8},
		{"g-along", "tok-goblin-archer", 14, 5},
		{"g-out", "tok-goblin-archer", 19, 8},
		{"f-steps", "tok-fighter", 6, 6},
	} {
		sendCommand(t, dmConn, &vttv1.ClientCommand{
			RequestId: step.id,
			Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
				TokenId: step.token, To: &vttv1.GridPosition{X: step.x, Y: step.y},
			}},
		})
		if r := readResult(t, dmConn); !r.Ok {
			t.Fatalf("%s: %s", step.id, r.Error)
		}
	}

	liveStream := drainEvents(t, live, 500*time.Millisecond)
	catchUp := drainEvents(t, f.dial(f.playerToken, 0), 500*time.Millisecond)

	// The live connection's own seed was drained above, so compare the tail.
	if len(catchUp) < len(liveStream) {
		t.Fatalf("catch-up produced %d envelopes, fewer than the %d seen live", len(catchUp), len(liveStream))
	}
	tail := catchUp[len(catchUp)-len(liveStream):]
	if len(liveStream) == 0 {
		t.Fatal("the scenario must reach this player at all, or this test compares nothing")
	}
	for i := range liveStream {
		want, err := protojson.Marshal(liveStream[i])
		if err != nil {
			t.Fatal(err)
		}
		got, err := protojson.Marshal(tail[i])
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("catch-up envelope %d differs from the one seen live:\n got %s\nwant %s", i, got, want)
		}
	}
}

// TestAPlayerCannotStepWhereItCannotSeeButTheDMCan is the other half of
// session zero's second clause: the refusal is a rule about PLAYERS, not a
// rule about the square. "Hard for players, free for DM" (maps-as-geometry
// spec §6) governs sight exactly as it governs stone, and a DM staging a
// creature across the map is a legitimate thing to do.
func TestAPlayerCannotStepWhereItCannotSeeButTheDMCan(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	playerConn := f.dial(f.playerToken, 0)
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
	// The refusal says WHY, and what it says is a fact the player already
	// holds — see handleCommand's note on why this must not name the occupant.
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

// TestASpectatorWithNoPerchReceivesNoBoard records the behaviour four
// transport tests in this package had to be re-seated around, so that it is
// pinned rather than merely relied upon.
//
// A spectator rides a shoulder (spec §3.1.1), and a connection opens perched
// on NOBODY: SetViewpoint exists, but until this watcher sends one they have no
// eyes and therefore no board. That is the fail-closed direction and the only
// one available — the alternative is a shoulder the server picked for them, and
// a spectator perched on the Goblin Archer would watch the ambush from inside
// it. TestASpectatorHopsFromOneShoulderToAnother is the other half: what
// arrives once they do choose.
//
// They still receive what is addressed to the table rather than drawn on the
// board — narration is the case that matters, because withholding it would
// leave a watcher with no game at all rather than merely no map.
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

	stream := drainEvents(t, watcher, 500*time.Millisecond)
	if mentions(t, stream, "ambush") || mentions(t, stream, "tok-") {
		for _, e := range stream {
			raw, _ := protojson.Marshal(e)
			t.Logf("spectator received: %s", raw)
		}
		t.Fatal("a spectator perched on nobody was given a board")
	}
	if !mentions(t, stream, "The corridor narrows.") {
		t.Fatal("a spectator must still hear the table: narration is addressed, not drawn")
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
	dmConn := f.dial(f.dmToken, 4)

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-ambush-scout",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-scout", Name: "Armak", ControllerId: f.playerID},
		}},
	})
	if r := readResult(t, dmConn); !r.Ok {
		t.Fatalf("seed AddActor act-scout: %s", r.Error)
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

// hides reports whether the stream tells this viewer that tokenID left view.
func hides(stream []*vttv1.Envelope, tokenID string) bool {
	for _, e := range stream {
		if th := e.GetTokenHidden(); th != nil && th.GetTokenId() == tokenID {
			return true
		}
	}
	return false
}

// TestASpectatorHopsFromOneShoulderToAnother is spec §3.1.1 end to end, and it
// is Patrik's own image: "you as a spectator can jump between tokens — like a
// bird hopping from one shoulder to another. You can sit on any of the
// characters, and you can choose to shift to another character's view,
// whenever — but you will only know as much as the party does, not what the DM
// has planned to happen."
//
// Four claims, and the assertions below follow them in order. The watcher is
// offered the party to choose from; sitting on Asme shows Asme's room AND NOT
// the archer behind the wall; hopping to Armak shows the archer and takes Asme
// away, because creatures are pure line of sight; and the terrain of BOTH rooms
// is still theirs at the end, because the bird remembers every shoulder it has
// sat on.
func TestASpectatorHopsFromOneShoulderToAnother(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	f.seedArmak(t)

	watcher := f.dial(f.spectatorToken, 0)

	// WHAT A WATCHER GETS FOR FREE: the roster, and nothing drawn. You cannot
	// choose a shoulder you have not been told about (spec §5 — actors
	// controlled by any player are always known).
	roster := drainEvents(t, watcher, 500*time.Millisecond)
	if !mentions(t, roster, "act-fighter") || !mentions(t, roster, "act-scout") {
		t.Fatal("a watcher must be told which characters the party has, or there is no shoulder to choose")
	}
	if mentions(t, roster, "tok-") || mentions(t, roster, "ambush") {
		t.Fatal("a spectator perched on nobody must have no board at all")
	}

	west := f.perch(t, watcher, "perch-asme", "act-fighter")
	if !mentions(t, west, "tok-fighter") {
		t.Fatal("sitting on Asme must show Asme's board, and show it at once")
	}
	if mentions(t, west, "goblin") {
		t.Fatal("the Goblin Archer is behind the wall from Asme's shoulder: " +
			"a watcher knows as much as the party does, not what the DM has planned")
	}

	east := f.perch(t, watcher, "perch-armak", "act-scout")
	if !mentions(t, east, "tok-goblin-archer") {
		t.Fatal("from Armak's shoulder the archer is one square away and in plain sight")
	}
	if !hides(east, "tok-fighter") {
		t.Fatal("Asme is behind the wall from Armak's shoulder, and creatures are pure " +
			"line of sight: her token must be taken off the watcher's board")
	}

	// THE BIRD REMEMBERS EVERY SHOULDER IT HAS SAT ON (spec §3.1.1/§3.2).
	// Folded with the engine's own fold — the one client/src/fold.ts mirrors —
	// the watcher's stream leaves them holding both rooms. Over an evening that
	// converges on what the party collectively knows, and never on what the DM
	// has planned, because there is no shoulder on the DM's side of the screen.
	stream := make([]*vttv1.Envelope, 0, len(roster)+len(west)+len(east))
	stream = append(stream, roster...)
	stream = append(stream, west...)
	stream = append(stream, east...)

	viewed := engine.NewState()
	for i, e := range stream {
		if err := engine.Apply(viewed, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	explored := viewed.Scenes["ambush"].Explored
	if !explored[gridKeyForTest(5, 5)] {
		t.Error("hopping away from Asme must not un-explore the room she stands in")
	}
	if !explored[gridKeyForTest(20, 8)] {
		t.Error("Armak's room must be explored once the watcher has sat on his shoulder")
	}
}

// TestASpectatorMayNotPerchOnTheGoblinArcher is THE CONSTRAINT THE WHOLE IDEA
// RESTS ON, end to end and over the real wire: MayPerch is unit-tested in
// viewpoint_test.go, and this proves the server actually asks it before moving
// anybody's eyes.
//
// A watcher inside the archer sees the ambush the party is walking into, which
// is session zero with a different seat number.
func TestASpectatorMayNotPerchOnTheGoblinArcher(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)

	watcher := f.dial(f.spectatorToken, 0)
	drainEvents(t, watcher, 500*time.Millisecond) // the roster, which is theirs

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

// TestPerchingAppendsNothingToTheLog is the same shape as handleJoinDoor, whose
// comment says it appends NOTHING, and it is Patrik's ruling: "we do not need
// to log anything about what/where the spectator sees."
//
// Where a watcher points their camera is not a fact about the campaign. Logged,
// it would replay forever, put rows in the story panel, and become RETRACTABLE
// — a DM could undo somebody having looked at Asme.
//
// Two assertions, because "nothing was appended" and "nobody was told" are
// different claims: the log's own length, and a DM sitting at the table who
// must hear nothing at all while the watcher hops.
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
func TestAPlayerCannotProbeTheDarkWithMoveCommands(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	playerConn := f.dial(f.playerToken, 0)

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

// TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee pins the COST of the
// sight rule, which the rule's own doc comment names and no test reached.
//
// The two cases are not the same and only one of them was covered. A square
// never seen and never explored is refused for the obvious reason. A square
// whose terrain this player HAS been sent, and whose client still holds and
// still draws it (spec §3.2, terrain is remembered and creatures are not), is
// refused too — and that is the deliberate restriction, the one a reader of
// "you may only move where you can see" would want to check before agreeing to
// it. If it is ever relaxed to "seen OR explored", this is the test that has to
// change, and it says so.
func TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee(t *testing.T) {
	f := newGWFixture(t)
	f.seedAmbush(t)
	dmConn := f.dial(f.dmToken, 0)
	playerConn := f.dial(f.playerToken, 0)

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

// TestEverySeatCanReachTheCatchUpHeadItIsGiven closes a promise the server
// could no longer keep.
//
// CatchUpHead is the boundary a client reads to: commands.proto says "a client
// that wants a point-in-time snapshot reads until it has seen head_sequence",
// and `vtt state dump` does exactly that (cmd/vtt/state_dump.go's drainToHead).
// The head was the LOG's head, which was the same thing as this seat's head
// while every seat received every event. It stopped being the same thing the
// moment a seat's stream became a projection: the last few log events may be
// entirely withheld from a player, so the number they are told to wait for
// never arrives and the CLI burns its 30-second deadline before failing.
//
// It fails CLOSED, which is the right direction and not a defence — a
// deterministic 30s failure on a shipped command is still a regression. So the
// head a seat is given is now the last sequence THAT SEAT's catch-up carries.
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

	// AND THE UNPROJECTED SEATS STILL GET THE LOG'S OWN HEAD, unchanged. The
	// fix must not quietly turn the DM's boundary into something derived.
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
