package gateway_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
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
// A spectator rides a shoulder (spec §3.1.1) and SetViewpoint — the command
// that chooses one — is Task 6's. Until it exists a spectator perches on
// nobody and therefore has no eyes, which is the fail-closed direction: the
// alternative is a default perch the server picked, and a spectator perched on
// the Goblin Archer would watch the ambush from inside it.
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
