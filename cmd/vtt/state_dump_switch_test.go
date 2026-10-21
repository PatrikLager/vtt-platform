package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

func switchingGateway(t *testing.T, head int64, frames ...*vttv1.ServerFrame) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		all := append([]*vttv1.ServerFrame{{Frame: &vttv1.ServerFrame_CatchUpHead{
			CatchUpHead: &vttv1.CatchUpHead{HeadSequence: head},
		}}}, frames...)
		for _, f := range all {
			raw, err := protojson.Marshal(f)
			if err != nil {
				t.Errorf("switchingGateway: marshal: %v", err)
				return
			}
			if conn.Write(ctx, websocket.MessageText, raw) != nil {
				return
			}
		}
		_, _, _ = conn.Read(ctx)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func switchTo(actorID string) *vttv1.ServerFrame {
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_ViewpointChanged{
		ViewpointChanged: &vttv1.ViewpointChanged{ActorId: actorID},
	}}
}

func sessionAt(seq int64) *vttv1.ServerFrame {
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: &vttv1.Envelope{
		Sequence:  seq,
		SessionId: "s1",
		Payload:   &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}},
	}}}
}

func actorAt(seq int64, actorID string) *vttv1.ServerFrame {
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: &vttv1.Envelope{
		Sequence:  seq,
		SessionId: "s1",
		Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: actorID, Name: actorID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
	}}}
}

func dumpOf(t *testing.T, url string) (fields map[string]json.RawMessage, out string, elapsed time.Duration, err error) {
	t.Helper()
	start := time.Now()
	out, err = runCLI(t, "state", "dump", "--server", url, "--token", "t")
	elapsed = time.Since(start)
	if err != nil {
		return nil, out, elapsed, err
	}
	if jerr := json.Unmarshal([]byte(out), &fields); jerr != nil {
		t.Fatalf("dump printed no JSON object: %v\n%s", jerr, out)
	}
	return fields, out, elapsed, nil
}

func actorsOf(t *testing.T, fields map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var actors map[string]json.RawMessage
	if err := json.Unmarshal(fields["Actors"], &actors); err != nil {
		t.Fatalf("the dump's Actors: %v (%s)", err, fields["Actors"])
	}
	return actors
}

// VTT-423 VTT-424
func TestADumpSwitchedBeforeItsHeadRefusesAndNamesBothLogs(t *testing.T) {
	cases := []struct {
		name   string
		to     string
		named  string
		frames []*vttv1.ServerFrame
	}{
		{"the new log shorter than the head", "a2", "a2's log", []*vttv1.ServerFrame{
			switchTo("a1"), sessionAt(1), actorAt(2, "a1"), switchTo("a2"), sessionAt(1),
		}},
		{"the new log as long as the head", "a2", "a2's log", []*vttv1.ServerFrame{
			switchTo("a1"), sessionAt(1), actorAt(2, "a1"), switchTo("a2"), sessionAt(1), actorAt(2, "a2"), actorAt(3, "a2"),
		}},
		{"a move to nobody", "", "nobody", []*vttv1.ServerFrame{
			switchTo("a1"), sessionAt(1), actorAt(2, "a1"), switchTo(""),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, out, elapsed, err := dumpOf(t, switchingGateway(t, 3, tc.frames...))
			if err == nil {
				t.Fatalf("a dump moved from a1 to %q before head 3 printed:\n%s", tc.to, out)
			}
			if strings.Contains(out, "headSequence") {
				t.Fatalf("a refused dump wrote a state: %q", out)
			}
			if !errors.Is(err, errMovedBeforeHead) {
				t.Fatalf("want errMovedBeforeHead, got %v", err)
			}
			if msg := err.Error(); !strings.Contains(msg, "from a1's log to "+tc.named) {
				t.Fatalf("the refusal does not name a1 and then %s: %q", tc.named, msg)
			}
			if elapsed > 5*time.Second {
				t.Fatalf("the refusal took %s", elapsed)
			}
		})
	}
}

// VTT-425
func TestADumpSwitchedAfterItsHeadPrintsTheLogItHeld(t *testing.T) {
	fields, _, _, err := dumpOf(t, switchingGateway(t, 2,
		switchTo("a1"), sessionAt(1), actorAt(2, "a1"), switchTo("a2"), sessionAt(1), actorAt(2, "a2")))
	if err != nil {
		t.Fatalf("a dump moved after its head refused: %v", err)
	}
	if string(fields["headSequence"]) != "2" || string(fields["log"]) != `"a1"` {
		t.Fatalf("want a1's fold at head 2, got log=%s headSequence=%s", fields["log"], fields["headSequence"])
	}
	if actors := actorsOf(t, fields); len(actors) != 1 || actors["a1"] == nil {
		t.Fatalf("want a1 alone folded, got Actors %s", fields["Actors"])
	}
}

// VTT-426
func TestADumpNamesTheLogItIsTheFoldOf(t *testing.T) {
	player, _, _, err := dumpOf(t, switchingGateway(t, 1, switchTo("a1"), sessionAt(1)))
	if err != nil || string(player["log"]) != `"a1"` {
		t.Fatalf("a player's dump: log=%s err=%v", player["log"], err)
	}
	nobody, _, _, err := dumpOf(t, switchingGateway(t, 0, switchTo("")))
	if err != nil || string(nobody["log"]) != `""` {
		t.Fatalf("a dump on nobody: log=%s err=%v", nobody["log"], err)
	}
	world, _, _, err := dumpOf(t, switchingGateway(t, 1, sessionAt(1)))
	if err != nil {
		t.Fatalf("a world dump: %v", err)
	}
	if _, ok := world["log"]; ok {
		t.Fatalf("a world dump carries a log key: %s", world["log"])
	}
	for k := range player {
		if _, ok := world[k]; !ok && k != "log" {
			t.Fatalf("a world dump lacks %q, which a player's carries", k)
		}
	}
	if len(world) != len(player)-1 {
		t.Fatalf("a world dump has %d keys, a player's %d: the world gained or lost one beside log", len(world), len(player))
	}
}

// VTT-426
func TestAPlayersDumpNamesItsLogAgainstARealGateway(t *testing.T) {
	sc, err := harness.LoadScenario(filepath.Join("../../scenarios", "a-player-switches-and-reconnects.json"))
	if err != nil {
		t.Fatal(err)
	}
	boot, err := bootSelfContained(sc)
	if err != nil {
		t.Fatal(err)
	}
	defer boot.close()
	rep, err := harness.RunScenario(context.Background(), sc, dialerFor(boot.WSURL, boot.Tokens), boot.IDs, io.Discard)
	if err != nil || !rep.Pass {
		t.Fatalf("scenario: err=%v steps=%+v", err, rep)
	}

	out, err := runCLI(t, "state", "dump", "--server", boot.WSURL, "--token", boot.Tokens["player"])
	if err != nil {
		t.Fatalf("a player's dump: %v", err)
	}
	var fields map[string]json.RawMessage
	var log string
	if err := json.Unmarshal([]byte(out), &fields); err != nil || json.Unmarshal(fields["log"], &log) != nil || log != "act-ash" {
		t.Fatalf("a player's dump names log %s, want act-ash, its first party member by id (err %v)", fields["log"], err)
	}
	envs := freshDialOf(t, boot.WSURL, boot.Tokens["player"], log)
	st, err := harness.Fold(envs)
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := writeDump(&want, st, headSequence(envs), &log); err != nil {
		t.Fatal(err)
	}
	if out != want.String() {
		t.Fatalf("a player's dump is not the fold of a fresh dial of %s:\n%s\nwant:\n%s", log, out, want.String())
	}

	world, err := runCLI(t, "state", "dump", "--server", boot.WSURL, "--token", boot.Tokens["dm"])
	if err != nil {
		t.Fatalf("the DM's dump: %v", err)
	}
	if strings.Contains(world, `"log"`) {
		t.Fatalf("the DM's dump names a log:\n%s", world)
	}
}

// VTT-435
func TestADumpToldItsOwnLogAgainFoldsNothingAfterIt(t *testing.T) {
	_, out, _, err := dumpOf(t, switchingGateway(t, 3,
		switchTo("a1"), sessionAt(1), actorAt(2, "a1"), switchTo("a1"), sessionAt(1), actorAt(2, "a1"), actorAt(3, "a1")))
	if !errors.Is(err, errMovedBeforeHead) || strings.Contains(out, "headSequence") {
		t.Fatalf("a dump told a1 again before head 3: err=%v out=%q", err, out)
	}
	fields, _, _, err := dumpOf(t, switchingGateway(t, 2,
		switchTo("a1"), sessionAt(1), actorAt(2, "a1"), switchTo("a1"), sessionAt(1), actorAt(2, "a1"), actorAt(3, "a3")))
	if err != nil || string(fields["headSequence"]) != "2" || string(fields["log"]) != `"a1"` {
		t.Fatalf("a dump told a1 again after head 2: err=%v log=%s headSequence=%s", err, fields["log"], fields["headSequence"])
	}
}
