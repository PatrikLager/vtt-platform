package gateway_test

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaPRPlainScene  = "qapr-vault"
	qaPRPlainScout  = "qapr-scout"
	qaPRPlainTok    = "tok-qapr-scout"
	qaPRPlainUnseen = "You cannot move there: you cannot see that square."
	qaPRPlainLine   = "This character has no name. Ask the DM for one, or speak out of character with /ooc."
	qaPRPlainWhisp  = "This character has no name. Ask the DM for one with /ooc, then whisper."
)

var qaPRPlainPackage = regexp.MustCompile(`^[a-z]+: `)

// Keep row 1 of the vault as the tests that build it read it, x=0..9: crate,
// scout, floor, closed door | floor, wall, closed door, statue, floor, floor.
func qaPRPlainVault(t *testing.T, f *gwFixture, crateKind string) {
	t.Helper()
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 10; x++ {
		tiles[fmt.Sprintf("%d,0", x)] = &vttv1.TileRef{Kind: "wall"}
		tiles[fmt.Sprintf("%d,1", x)] = &vttv1.TileRef{Kind: "floor"}
		tiles[fmt.Sprintf("%d,2", x)] = &vttv1.TileRef{Kind: "wall"}
	}
	tiles["3,1"].Kind = "door"
	tiles["5,1"].Kind = "wall"
	tiles["6,1"].Kind = "door"
	at := func(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }
	mustAppend(t, f.campaign, "qapr-vault-scene", &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: qaPRPlainScene, Name: "vault", GridWidth: 10, GridHeight: 3, Tiles: tiles,
		Objects: []*vttv1.SceneObject{
			{ObjectId: "qapr-crate", Kind: crateKind, At: at(0, 1), Width: 1, Height: 1, BlocksMove: true},
			{ObjectId: "qapr-statue", Kind: "statue", At: at(7, 1), Width: 1, Height: 1, BlocksMove: true},
		},
	}})
	mustAppend(t, f.campaign, "qapr-vault-actor", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: qaPRPlainScout, Name: "Scout"}}})
	mustAppend(t, f.campaign, "qapr-vault-grant", &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: qaPRPlainScout, ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustAppend(t, f.campaign, "qapr-vault-token", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: qaPRPlainTok, SceneId: qaPRPlainScene, ActorId: qaPRPlainScout, Position: at(1, 1), Name: "Scout"}})
}

func qaPRPlainOpen(t *testing.T, f *gwFixture, token, viewpoint string) *websocket.Conn {
	t.Helper()
	var conn *websocket.Conn
	if viewpoint == "" {
		conn = f.dial(token, f.head(t))
	} else {
		conn = f.dialOn(token, f.head(t), viewpoint)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	drainEvents(t, conn, 200*time.Millisecond)
	return conn
}

func qaPRPlainMove(tok string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: fmt.Sprintf("qapr-mv-%s-%d-%d", tok, x, y), Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: tok, To: &vttv1.GridPosition{X: x, Y: y}}}}
}

func qaPRPlainSay(actorID string, ooc bool) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "qapr-say-" + actorID, Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: "hello there", ActorId: actorID, OutOfCharacter: ooc}}}
}

func qaPRPlainWhisper(from string, to ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "qapr-wh-" + from, Command: &vttv1.ClientCommand_AddWhisper{
		AddWhisper: &vttv1.AddWhisper{Text: "psst", FromActorId: from, ToActorIds: to}}}
}

func qaPRPlainRefused(t *testing.T, f *gwFixture, conn *websocket.Conn, cmd *vttv1.ClientCommand) string {
	t.Helper()
	before := f.head(t)
	sendCommand(t, conn, cmd)
	res := readResult(t, conn)
	if res.GetOk() {
		t.Fatalf("%s: accepted at sequence %d, want a refusal", cmd.GetRequestId(), res.GetSequence())
	}
	if after := f.head(t); after != before {
		t.Errorf("%s: refused but the head moved %d -> %d", cmd.GetRequestId(), before, after)
	}
	return res.GetError()
}

func qaPRPlainAccepted(t *testing.T, f *gwFixture, conn *websocket.Conn, cmd *vttv1.ClientCommand) {
	t.Helper()
	before := f.head(t)
	sendCommand(t, conn, cmd)
	res := readResult(t, conn)
	if !res.GetOk() {
		t.Fatalf("%s: refused %q, want it accepted", cmd.GetRequestId(), res.GetError())
	}
	if after := f.head(t); after == before {
		t.Errorf("%s: accepted but nothing was appended", cmd.GetRequestId())
	}
}

func qaPRPlainWant(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// VTT-601 VTT-156 VTT-162
func TestQAPRPlainAnUnseenSquareIsOneTextWhateverStandsOnIt(t *testing.T) {
	f := newGWFixture(t)
	qaPRPlainVault(t, f, "crate")
	conn := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	for _, sq := range []struct {
		what string
		x, y int32
	}{{"floor", 4, 1}, {"wall", 5, 1}, {"closed door", 6, 1}, {"scenery", 7, 1}, {"far floor", 9, 1}} {
		got := qaPRPlainRefused(t, f, conn, qaPRPlainMove(qaPRPlainTok, sq.x, sq.y))
		qaPRPlainWant(t, "unseen "+sq.what, got, qaPRPlainUnseen)
	}
	qaPRPlainAccepted(t, f, conn, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

// VTT-602 VTT-157 VTT-162
func TestQAPRPlainASeenObstructionIsNamedInPlainWords(t *testing.T) {
	f := newGWFixture(t)
	qaPRPlainVault(t, f, "crate")
	conn := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	for _, sq := range []struct {
		x, y int32
		want string
	}{
		{1, 0, "You cannot move there: a wall."},
		{1, 2, "You cannot move there: a wall."},
		{3, 1, "You cannot move there: a closed door."},
		{0, 1, "You cannot move there: something (a crate) is in the way."},
	} {
		got := qaPRPlainRefused(t, f, conn, qaPRPlainMove(qaPRPlainTok, sq.x, sq.y))
		qaPRPlainWant(t, fmt.Sprintf("seen (%d,%d)", sq.x, sq.y), got, sq.want)
	}
	qaPRPlainAccepted(t, f, conn, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

// VTT-602
func TestQAPRPlainALongSceneryKindIsClippedInsideThePlainWords(t *testing.T) {
	f := newGWFixture(t)
	kind := strings.Repeat("abcdefghij", 6)
	qaPRPlainVault(t, f, kind)
	conn := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	got := qaPRPlainRefused(t, f, conn, qaPRPlainMove(qaPRPlainTok, 0, 1))
	qaPRPlainWant(t, "long kind", got, "You cannot move there: something (a "+kind[:40]+"…) is in the way.")
	qaPRPlainAccepted(t, f, conn, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

// VTT-602 VTT-157
func TestQAPRPlainTheCellarBoulderIsNamedInPlainWords(t *testing.T) {
	f := newGWFixture(t)
	f.seedCellarWithSceneryKind(t, "barrel")
	conn := qaPRPlainOpen(t, f, f.playerToken, "act-fighter")
	qaPRPlainWant(t, "barrel", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-fighter", 1, 1)),
		"You cannot move there: something (a barrel) is in the way.")
	qaPRPlainWant(t, "wall", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-fighter", 2, 2)),
		"You cannot move there: a wall.")
	qaPRPlainAccepted(t, f, conn, qaPRPlainSay("act-fighter", false))
}

// VTT-601 VTT-453
func TestQAPRPlainTheMoveIsJudgedByTheEyesTheConnectionShows(t *testing.T) {
	f := newGWFixture(t)
	qaPRPlainVault(t, f, "crate")
	hero := qaPRPlainOpen(t, f, f.playerToken, "a1")
	qaPRPlainWant(t, "judged by Hero", qaPRPlainRefused(t, f, hero, qaPRPlainMove(qaPRPlainTok, 2, 1)), qaPRPlainUnseen)
	qaPRPlainWant(t, "judged by Hero, wall", qaPRPlainRefused(t, f, hero, qaPRPlainMove(qaPRPlainTok, 1, 0)), qaPRPlainUnseen)
	scout := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	qaPRPlainAccepted(t, f, scout, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

// VTT-601 VTT-602
func TestQAPRPlainACreatureTokenIsJudgedThroughTheShownCharacter(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	qaPRPlainVault(t, f, "crate")
	mustAppend(t, f.campaign, "qapr-goblin-token", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-qapr-goblin", SceneId: qaPRPlainScene, ActorId: "a3", Position: &vttv1.GridPosition{X: 2, Y: 1}}})
	conn := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	qaPRPlainWant(t, "goblin onto door", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-qapr-goblin", 3, 1)),
		"You cannot move there: a closed door.")
	qaPRPlainWant(t, "goblin onto wall", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-qapr-goblin", 2, 0)),
		"You cannot move there: a wall.")
	qaPRPlainWant(t, "goblin past door", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-qapr-goblin", 4, 1)), qaPRPlainUnseen)
	qaPRPlainAccepted(t, f, qaPRPlainOpen(t, f, f.dmToken, ""), &vttv1.ClientCommand{RequestId: "qapr-open", Command: &vttv1.ClientCommand_OpenDoor{
		OpenDoor: &vttv1.OpenDoor{SceneId: qaPRPlainScene, At: &vttv1.GridPosition{X: 3, Y: 1}}}})
	qaPRPlainAccepted(t, f, conn, qaPRPlainMove("tok-qapr-goblin", 3, 1))
}

// VTT-601 VTT-370
func TestQAPRPlainAPlayerWhoRunsOnlyACreatureIsToldTheyCannotSee(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	qaPRPlainVault(t, f, "crate")
	ivo := qaPRPlainIvo(t, f)
	qaPRPlainAccepted(t, f, qaPRPlainOpen(t, f, f.dmToken, ""), &vttv1.ClientCommand{RequestId: "qapr-revoke", Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{ActorId: "a2", ParticipantId: ivo}}})
	mustAppend(t, f.campaign, "qapr-beast", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "qapr-beast", Name: "Beast"}}})
	mustAppend(t, f.campaign, "qapr-beast-grant", &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: "qapr-beast", ParticipantId: ivo, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})
	mustAppend(t, f.campaign, "qapr-beast-token", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-qapr-beast", SceneId: qaPRPlainScene, ActorId: "qapr-beast", Position: &vttv1.GridPosition{X: 8, Y: 1}}})
	conn := qaPRPlainOpen(t, f, f.otherPlayerToken, "")
	qaPRPlainWant(t, "eyeless onto floor", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-qapr-beast", 9, 1)), qaPRPlainUnseen)
	qaPRPlainWant(t, "eyeless onto statue", qaPRPlainRefused(t, f, conn, qaPRPlainMove("tok-qapr-beast", 7, 1)), qaPRPlainUnseen)
	lera := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	qaPRPlainAccepted(t, f, lera, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

func qaPRPlainIvo(t *testing.T, f *gwFixture) string {
	t.Helper()
	tobbe := f.campaign.State().Actors["a2"]
	if tobbe == nil || len(tobbe.GetControllerIds()) != 1 {
		t.Fatalf("whisperTable's a2 has controllers %v, want Ivo alone", tobbe.GetControllerIds())
	}
	return tobbe.GetControllerIds()[0]
}

func TestQAPRPlainAMoveRefusedBeforeTheGateKeepsItsOldWords(t *testing.T) {
	f := newGWFixture(t)
	qaPRPlainVault(t, f, "crate")
	ivo := qaPRPlainOpen(t, f, f.otherPlayerToken, "")
	spec := qaPRPlainOpen(t, f, f.spectatorToken, "")
	for who, conn := range map[string]*websocket.Conn{"ivo": ivo, "spectator": spec} {
		for _, to := range [][2]int32{{2, 1}, {5, 1}, {1, 0}} {
			got := qaPRPlainRefused(t, f, conn, qaPRPlainMove(qaPRPlainTok, to[0], to[1]))
			if !strings.HasPrefix(got, "gateway: not authorized") {
				t.Errorf("%s moving Lera's token to %v: got %q, want the old gateway words", who, to, got)
			}
		}
	}
	lera := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	qaPRPlainAccepted(t, f, lera, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

// VTT-158 VTT-379
func TestQAPRPlainTheDMAndTheAgentMoveWhereAPlayerIsRefused(t *testing.T) {
	f := newGWFixture(t)
	qaPRPlainVault(t, f, "crate")
	agent := qaPRPlainOpen(t, f, f.agentToken, "")
	qaPRPlainAccepted(t, f, agent, qaPRPlainMove(qaPRPlainTok, 5, 1))
	dm := qaPRPlainOpen(t, f, f.dmToken, "")
	qaPRPlainAccepted(t, f, dm, qaPRPlainMove(qaPRPlainTok, 0, 1))
}

// VTT-601 VTT-156
func TestQAPRPlainASquareOffTheGridIsRefusedInPlainWords(t *testing.T) {
	f := newGWFixture(t)
	qaPRPlainVault(t, f, "crate")
	conn := qaPRPlainOpen(t, f, f.playerToken, qaPRPlainScout)
	for _, to := range [][2]int32{{-1, 1}, {10, 1}, {1, 3}, {1, -1}} {
		got := qaPRPlainRefused(t, f, conn, qaPRPlainMove(qaPRPlainTok, to[0], to[1]))
		qaPRPlainWant(t, fmt.Sprintf("off the grid %v", to), got, qaPRPlainUnseen)
	}
	qaPRPlainAccepted(t, f, conn, qaPRPlainMove(qaPRPlainTok, 2, 1))
}

// VTT-603 VTT-495 VTT-588 VTT-162
func TestQAPRPlainANamelessCharacterIsToldWhyOnTheWire(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	conn := qaPRPlainOpen(t, f, f.playerToken, "")
	qaPRPlainWant(t, "line from a4", qaPRPlainRefused(t, f, conn, qaPRPlainSay("a4", false)), qaPRPlainLine)
	qaPRPlainWant(t, "whisper from a4", qaPRPlainRefused(t, f, conn, qaPRPlainWhisper("a4")), qaPRPlainWhisp)
	qaPRPlainAccepted(t, f, conn, qaPRPlainSay("a1", false))
	qaPRPlainAccepted(t, f, conn, qaPRPlainWhisper("a1"))
}

// VTT-603
func TestQAPRPlainAuthorizeWrapsErrUnauthorizedForANamelessCharacter(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	lera := &identity.Participant{ID: f.playerID, Name: "Lera", Role: identity.RolePlayer}
	st := f.campaign.State()
	for _, c := range []struct {
		cmd  *vttv1.ClientCommand
		want string
	}{{qaPRPlainSay("a4", false), qaPRPlainLine}, {qaPRPlainWhisper("a4"), qaPRPlainWhisp}} {
		err := gateway.Authorize(lera, c.cmd, st)
		if err == nil {
			t.Fatalf("%s: Authorize accepted", c.cmd.GetRequestId())
		}
		if !errors.Is(err, gateway.ErrUnauthorized) {
			t.Errorf("%s: %q does not wrap ErrUnauthorized", c.cmd.GetRequestId(), err)
		}
		qaPRPlainWant(t, c.cmd.GetRequestId(), err.Error(), c.want)
		if !strings.HasPrefix(err.Error(), "This character has no name.") || qaPRPlainPackage.MatchString(err.Error()) ||
			strings.Contains(err.Error(), "gateway") {
			t.Errorf("%s: %q names a package or misses the opening", c.cmd.GetRequestId(), err)
		}
	}
	for _, ok := range []*vttv1.ClientCommand{qaPRPlainSay("a1", false), qaPRPlainWhisper("a1")} {
		if err := gateway.Authorize(lera, ok, st); err != nil {
			t.Errorf("%s: control refused: %v", ok.GetRequestId(), err)
		}
	}
}

// VTT-506 VTT-589 VTT-603
func TestQAPRPlainTheNamelessTextIsToldOnlyOfTheSpeakersOwnPartyMember(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	ivo := qaPRPlainIvo(t, f)
	mustAppend(t, f.campaign, "qapr-a5", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "qapr-a5"}}})
	mustAppend(t, f.campaign, "qapr-a5-grant", &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: "qapr-a5", ParticipantId: ivo, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustAppend(t, f.campaign, "qapr-a6", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "qapr-a6"}}})
	mustAppend(t, f.campaign, "qapr-a6-grant", &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: "qapr-a6", ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})
	conn := qaPRPlainOpen(t, f, f.playerToken, "")
	others := []string{"qapr-absent", "a2", "a3", "qapr-a5", "qapr-a6"}
	qaPRPlainOneText(t, "line", others, func(id string) string { return qaPRPlainRefused(t, f, conn, qaPRPlainSay(id, false)) })
	qaPRPlainOneText(t, "whisper", append([]string{""}, others...), func(id string) string {
		return qaPRPlainRefused(t, f, conn, qaPRPlainWhisper(id))
	})
	qaPRPlainAccepted(t, f, conn, qaPRPlainWhisper("a1"))
}

func qaPRPlainOneText(t *testing.T, what string, ids []string, refuse func(string) string) {
	t.Helper()
	first := refuse(ids[0])
	if first == qaPRPlainLine || first == qaPRPlainWhisp || strings.HasPrefix(first, "This character has no name.") {
		t.Errorf("%s as %q: got the nameless text %q", what, ids[0], first)
	}
	for _, id := range ids[1:] {
		if got := refuse(id); got != first {
			t.Errorf("%s as %q: got %q, want the one text %q", what, id, got, first)
		}
	}
}

// VTT-603 VTT-495 VTT-588
func TestQAPRPlainRefusalsOutsideTheNamelessCaseKeepTheirOldWords(t *testing.T) {
	f := newGWFixture(t)
	f.whisperTable(t)
	lera := qaPRPlainOpen(t, f, f.playerToken, "")
	dm := qaPRPlainOpen(t, f, f.dmToken, "")
	for _, c := range []struct {
		what, prefix string
		conn         *websocket.Conn
		cmd          *vttv1.ClientCommand
	}{
		{"player ooc naming a4", "gateway: not authorized", lera, qaPRPlainSay("a4", true)},
		{"player whisper to a1 from a4", "gateway: not authorized: a player whispers to the DM alone", lera, qaPRPlainWhisper("a4", "a1")},
		{"dm in character as a4", "gateway: not authorized", dm, qaPRPlainSay("a4", false)},
		{"dm whisper from a4", "gateway: not authorized", dm, qaPRPlainWhisper("a4", "a1")},
	} {
		got := qaPRPlainRefused(t, f, c.conn, c.cmd)
		if !strings.HasPrefix(got, c.prefix) {
			t.Errorf("%s: got %q, want it to begin %q", c.what, got, c.prefix)
		}
	}
	qaPRPlainAccepted(t, f, lera, qaPRPlainSay("", true))
}
