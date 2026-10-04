package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaBoundScene = "scn-qa-bound"
	qaBoundActor = "act-qa-bound"
	qaBoundToken = "tok-qa-bound"
)

func qaBoundSeed() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		{EventId: "evt-qa-bound-1", ActorRole: "dm", Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: qaBoundScene, Name: "QA scene", GridWidth: 10, GridHeight: 10,
		}}},
		{EventId: "evt-qa-bound-2", ActorRole: "dm", Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: qaBoundActor, Name: "QA actor", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}}}},
		{EventId: "evt-qa-bound-3", ActorRole: "dm", Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: qaBoundToken, SceneId: qaBoundScene, ActorId: qaBoundActor, Position: &vttv1.GridPosition{X: 1, Y: 1},
		}}},
	}
}

func qaBoundState(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	for _, env := range qaBoundSeed() {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("seed %s: %v", env.GetEventId(), err)
		}
	}
	return st
}

func qaBoundMove(tokenID string, to *vttv1.GridPosition, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "evt-qa-bound-move", ActorRole: "dm", Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tokenID, SceneId: qaBoundScene, From: &vttv1.GridPosition{X: 1, Y: 1}, To: to, Reason: reason,
	}}}
}

func qaBoundTo() *vttv1.GridPosition { return &vttv1.GridPosition{X: 2, Y: 2} }

func qaBoundApplyMove(t *testing.T, tokenID string, to *vttv1.GridPosition, reason string) (*engine.State, error) {
	t.Helper()
	st := qaBoundState(t)
	return st, engine.Apply(st, qaBoundMove(tokenID, to, reason))
}

type qaBoundCase struct {
	name   string
	text   string
	nbytes int
}

func qaBoundAtLimit256() []qaBoundCase {
	return []qaBoundCase{
		{"ascii 256", strings.Repeat("a", 256), 256},
		{"two-byte x128", strings.Repeat("é", 128), 256},
		{"three-byte x85 plus ascii", strings.Repeat("€", 85) + "a", 256},
		{"four-byte x64", strings.Repeat("😀", 64), 256},
	}
}

func qaBoundOver256() []qaBoundCase {
	return []qaBoundCase{
		{"ascii 257", strings.Repeat("a", 257), 257},
		{"two-byte x129 is 129 characters", strings.Repeat("é", 129), 258},
		{"three-byte x86 is 86 characters", strings.Repeat("€", 86), 258},
		{"four-byte x65 is 65 characters", strings.Repeat("😀", 65), 260},
		{"ascii 255 plus one two-byte", strings.Repeat("a", 255) + "é", 257},
	}
}

// VTT-264
func TestQAMoveReasonBoundGoFoldAcceptsAReasonOfExactly256Bytes(t *testing.T) {
	for _, tc := range qaBoundAtLimit256() {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.text) != tc.nbytes {
				t.Fatalf("fixture: %d bytes, want %d", len(tc.text), tc.nbytes)
			}
			st, err := qaBoundApplyMove(t, qaBoundToken, qaBoundTo(), tc.text)
			if err != nil {
				t.Fatalf("Apply refused a %d-byte reason: %v", tc.nbytes, err)
			}
			tok := st.Tokens[qaBoundToken]
			if tok.X != 2 || tok.Y != 2 {
				t.Fatalf("token at (%d,%d) after an accepted move, want (2,2)", tok.X, tok.Y)
			}
		})
	}
}

// VTT-264
func TestQAMoveReasonBoundGoFoldRefusesAReasonOver256Bytes(t *testing.T) {
	for _, tc := range qaBoundOver256() {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.text) != tc.nbytes {
				t.Fatalf("fixture: %d bytes, want %d", len(tc.text), tc.nbytes)
			}
			st, err := qaBoundApplyMove(t, qaBoundToken, qaBoundTo(), tc.text)
			if err == nil {
				t.Fatalf("Apply accepted a %d-byte reason", tc.nbytes)
			}
			want := "engine: move reason must be at most 256 bytes, got " + strconv.Itoa(tc.nbytes)
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err.Error(), want)
			}
			tok := st.Tokens[qaBoundToken]
			if tok.X != 1 || tok.Y != 1 {
				t.Fatalf("token at (%d,%d) after a refused move, want (1,1)", tok.X, tok.Y)
			}
		})
	}
}

// SPEC-018 "How it works", table row `TokenMoved.reason` | 256 | yes.
func TestQAMoveReasonBoundGoFoldAcceptsAnEmptyReason(t *testing.T) {
	st, err := qaBoundApplyMove(t, qaBoundToken, qaBoundTo(), "")
	if err != nil {
		t.Fatalf("Apply refused an empty reason: %v", err)
	}
	if tok := st.Tokens[qaBoundToken]; tok.X != 2 || tok.Y != 2 {
		t.Fatalf("token at (%d,%d), want (2,2)", tok.X, tok.Y)
	}
}

// SPEC-018 "How it works": "A `TokenMoved` is checked for a known token, then
// for a destination at all, then its reason."
func TestQAMoveReasonBoundGoFoldChecksTokenThenDestinationThenReason(t *testing.T) {
	long := strings.Repeat("a", 257)
	reasonErr := "engine: move reason must be at most 256 bytes, got 257"

	_, ctlReason := qaBoundApplyMove(t, qaBoundToken, qaBoundTo(), long)
	if ctlReason == nil || ctlReason.Error() != reasonErr {
		t.Fatalf("control: known token, valid destination, long reason = %v, want %q", ctlReason, reasonErr)
	}

	_, ctlToken := qaBoundApplyMove(t, "tok-qa-bound-absent", qaBoundTo(), "")
	if ctlToken == nil {
		t.Fatal("control: an unknown token with a valid destination was accepted")
	}
	_, ctlDest := qaBoundApplyMove(t, qaBoundToken, nil, "")
	if ctlDest == nil {
		t.Fatal("control: a missing destination with an empty reason was accepted")
	}

	t.Run("unknown token, missing destination, long reason answers the token", func(t *testing.T) {
		_, err := qaBoundApplyMove(t, "tok-qa-bound-absent", nil, long)
		if err == nil || err.Error() != ctlToken.Error() {
			t.Fatalf("error = %v, want the token refusal %q", err, ctlToken.Error())
		}
	})
	t.Run("unknown token, valid destination, long reason answers the token", func(t *testing.T) {
		_, err := qaBoundApplyMove(t, "tok-qa-bound-absent", qaBoundTo(), long)
		if err == nil || err.Error() != ctlToken.Error() {
			t.Fatalf("error = %v, want the token refusal %q", err, ctlToken.Error())
		}
	})
	t.Run("known token, missing destination, long reason answers the destination", func(t *testing.T) {
		_, err := qaBoundApplyMove(t, qaBoundToken, nil, long)
		if err == nil || err.Error() != ctlDest.Error() {
			t.Fatalf("error = %v, want the destination refusal %q", err, ctlDest.Error())
		}
		if strings.Contains(err.Error(), "reason") {
			t.Fatalf("error %q names the reason; the destination is checked first", err)
		}
	})
}

func qaBoundNarration(text, as string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "evt-qa-bound-narr", ActorRole: "dm", Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: text, As: as,
	}}}
}

func qaBoundNote(key, title, text string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "evt-qa-bound-note", ActorRole: "dm", Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: title, Text: text, Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
	}}}
}

// VTT-266
func TestQAMoveReasonBoundGoFoldBoundsTheNarrationSpeaker(t *testing.T) {
	for _, tc := range append(qaBoundAtLimit256(), qaBoundCase{"empty", "", 0}) {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			if err := engine.Apply(qaBoundState(t), qaBoundNarration("Something happens.", tc.text)); err != nil {
				t.Fatalf("Apply refused a %d-byte speaker: %v", tc.nbytes, err)
			}
		})
	}
	for _, tc := range qaBoundOver256() {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			err := engine.Apply(qaBoundState(t), qaBoundNarration("Something happens.", tc.text))
			if err == nil {
				t.Fatalf("Apply accepted a %d-byte speaker", tc.nbytes)
			}
			qaBoundNamesBoundAndLength(t, err, 256, tc.nbytes)
		})
	}
}

func qaBoundNamesBoundAndLength(t *testing.T, err error, bound, got int) {
	t.Helper()
	msg := err.Error()
	if !strings.Contains(msg, strconv.Itoa(bound)) {
		t.Errorf("error %q does not name the bound %d", msg, bound)
	}
	if !strings.Contains(msg, "got "+strconv.Itoa(got)) {
		t.Errorf("error %q does not name the length %d", msg, got)
	}
}

// SPEC-018 "How it works": the table's NoteUpserted.key, NoteUpserted.title,
// NoteUpserted.text and NarrationAdded.text rows.
func TestQAMoveReasonBoundGoFoldBoundsTheOtherFreeTextFields(t *testing.T) {
	type field struct {
		name     string
		bound    int
		mayEmpty bool
		env      func(s string) *vttv1.Envelope
	}
	fields := []field{
		{"note key", 128, false, func(s string) *vttv1.Envelope { return qaBoundNote(s, "Title", "Body.") }},
		{"note title", 256, true, func(s string) *vttv1.Envelope { return qaBoundNote("k", s, "Body.") }},
		{"note text", 8192, false, func(s string) *vttv1.Envelope { return qaBoundNote("k", "Title", s) }},
		{"narration text", 8192, false, func(s string) *vttv1.Envelope { return qaBoundNarration(s, "") }},
	}
	for _, f := range fields {
		t.Run(f.name+" at the bound", func(t *testing.T) {
			if err := engine.Apply(qaBoundState(t), f.env(strings.Repeat("a", f.bound))); err != nil {
				t.Fatalf("refused %d bytes: %v", f.bound, err)
			}
		})
		t.Run(f.name+" at the bound in two-byte characters", func(t *testing.T) {
			if err := engine.Apply(qaBoundState(t), f.env(strings.Repeat("é", f.bound/2))); err != nil {
				t.Fatalf("refused %d bytes: %v", f.bound, err)
			}
		})
		t.Run(f.name+" one over", func(t *testing.T) {
			err := engine.Apply(qaBoundState(t), f.env(strings.Repeat("a", f.bound+1)))
			if err == nil {
				t.Fatalf("accepted %d bytes", f.bound+1)
			}
			qaBoundNamesBoundAndLength(t, err, f.bound, f.bound+1)
		})
		t.Run(f.name+" one character over in two-byte characters", func(t *testing.T) {
			err := engine.Apply(qaBoundState(t), f.env(strings.Repeat("é", f.bound/2+1)))
			if err == nil {
				t.Fatalf("accepted %d bytes", f.bound+2)
			}
			qaBoundNamesBoundAndLength(t, err, f.bound, f.bound+2)
		})
		t.Run(f.name+" empty", func(t *testing.T) {
			err := engine.Apply(qaBoundState(t), f.env(""))
			if f.mayEmpty && err != nil {
				t.Fatalf("refused an empty value the table allows: %v", err)
			}
			if !f.mayEmpty {
				if err == nil {
					t.Fatal("accepted an empty value the table forbids")
				}
				qaBoundNamesBoundAndLength(t, err, f.bound, 0)
			}
		})
	}
	t.Run("the quoted key refusal", func(t *testing.T) {
		err := engine.Apply(qaBoundState(t), qaBoundNote("", "Title", "Body."))
		want := "engine: note key must be 1-128 bytes, got 0"
		if err == nil || err.Error() != want {
			t.Fatalf("error = %v, want %q", err, want)
		}
	})
}

// SPEC-018 "How it works": the table's row order, which fold.ts's checkLen
// calls repeat.
func TestQAMoveReasonBoundGoFoldNamesTheFirstFieldInTableOrder(t *testing.T) {
	over := func(n int) string { return strings.Repeat("a", n+1) }
	cases := []struct {
		name, want string
		env        *vttv1.Envelope
	}{
		{"key, title and text all out of bounds", "key", qaBoundNote("", over(256), "")},
		{"title and text out of bounds", "title", qaBoundNote("k", over(256), "")},
		{"narration text and speaker out of bounds", "text", qaBoundNarration("", over(256))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Apply(qaBoundState(t), tc.env)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name the %s", err, tc.want)
			}
		})
	}
}

// VTT-265
func TestQAMoveReasonBoundTheToolDescribesTheReasonAndTheFoldsBound(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []struct {
		Name        string `json:"name"`
		InputSchema struct {
			Properties map[string]struct {
				Description string `json:"description"`
				Type        string `json:"type"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools {
		if tool.Name != "move_token" {
			continue
		}
		found = true
		prop, ok := tool.InputSchema.Properties["reason"]
		if !ok {
			t.Fatal("move_token has no reason property")
		}
		for _, r := range tool.InputSchema.Required {
			if r == "reason" {
				t.Error("reason is listed as required")
			}
		}
		d := prop.Description
		low := strings.ToLower(d)
		if !strings.Contains(low, "optional") {
			t.Errorf("description %q does not say the reason is optional", d)
		}
		if !strings.Contains(d, "DM") || !strings.Contains(low, "agent") {
			t.Errorf("description %q does not name the DM and the agent as its readers", d)
		}
		if !strings.Contains(low, "only") && !strings.Contains(low, "alone") {
			t.Errorf("description %q does not say the DM and the agent alone read it", d)
		}
		bound := qaBoundLargestAcceptedReason(t)
		if !strings.Contains(d, strconv.Itoa(bound)+" bytes") {
			t.Errorf("description %q does not state the fold's measured bound of %d bytes", d, bound)
		}
		if !strings.Contains(d, "UTF-8") {
			t.Errorf("description %q does not say the bytes are UTF-8", d)
		}
	}
	if !found {
		t.Fatal("no move_token tool in the generated manifest")
	}
}

func qaBoundLargestAcceptedReason(t *testing.T) int {
	t.Helper()
	lo, hi := 0, 1<<16
	if _, err := qaBoundApplyMove(t, qaBoundToken, qaBoundTo(), strings.Repeat("a", hi)); err == nil {
		t.Fatalf("the fold accepts a %d-byte reason", hi)
	}
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if _, err := qaBoundApplyMove(t, qaBoundToken, qaBoundTo(), strings.Repeat("a", mid)); err == nil {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

type qaBoundConn struct {
	conn   *websocket.Conn
	frames chan *vttv1.ServerFrame
	errMu  sync.Mutex
	err    error
}

func qaBoundDial(t *testing.T, ctx context.Context, base, token string) *qaBoundConn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(base, "http") + "/ws?after=0&token=" + url.QueryEscape(token)
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(1 << 22)
	qc := &qaBoundConn{conn: conn, frames: make(chan *vttv1.ServerFrame, 1024)}
	go func() {
		defer close(qc.frames)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				qc.errMu.Lock()
				qc.err = err
				qc.errMu.Unlock()
				return
			}
			f := &vttv1.ServerFrame{}
			if err := protojson.Unmarshal(data, f); err != nil {
				qc.errMu.Lock()
				qc.err = err
				qc.errMu.Unlock()
				return
			}
			qc.frames <- f
		}
	}()
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })
	return qc
}

func (qc *qaBoundConn) readErr() error {
	qc.errMu.Lock()
	defer qc.errMu.Unlock()
	return qc.err
}

func (qc *qaBoundConn) send(t *testing.T, ctx context.Context, cmd *vttv1.ClientCommand) {
	t.Helper()
	data, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := qc.conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func (qc *qaBoundConn) next(t *testing.T, timeout time.Duration, want func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case f, ok := <-qc.frames:
			if !ok {
				t.Fatalf("connection ended while waiting: %v", qc.readErr())
			}
			if want(f) {
				return f
			}
		case <-deadline:
			t.Fatalf("no matching frame within %s", timeout)
		}
	}
}

func (qc *qaBoundConn) result(t *testing.T, requestID string) *vttv1.CommandResult {
	t.Helper()
	return qc.next(t, 10*time.Second, func(f *vttv1.ServerFrame) bool {
		return f.GetResult() != nil && f.GetResult().GetRequestId() == requestID
	}).GetResult()
}

func qaBoundMoveCmd(requestID string, reason *string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: qaBoundToken, To: qaBoundTo(), Reason: reason,
	}}}
}

// VTT-161 VTT-162 VTT-264
func TestQAMoveReasonBoundWireRefusalReachesTheIssuerAndAppendsNothing(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var head int64
	for _, env := range qaBoundSeed() {
		env.OccurredAt = timestamppb.Now()
		env.ParticipantId = "p-qa-seed"
		seq, err := c.Append(env)
		if err != nil {
			t.Fatalf("seed %s: %v", env.GetEventId(), err)
		}
		head = seq
	}
	agentTok, _, err := ids.CreateInvite("QA agent", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}
	dmTok, _, err := ids.CreateInvite("QA dm", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	observer := qaBoundDial(t, ctx, srv.URL, dmTok)
	ch := observer.next(t, 10*time.Second, func(f *vttv1.ServerFrame) bool { return f.GetCatchUpHead() != nil })
	if got := ch.GetCatchUpHead().GetHeadSequence(); got != head {
		t.Fatalf("observer catch-up head = %d, want %d", got, head)
	}
	issuer := qaBoundDial(t, ctx, srv.URL, agentTok)
	issuer.next(t, 10*time.Second, func(f *vttv1.ServerFrame) bool { return f.GetCatchUpHead() != nil })

	refused := []struct {
		id, reason, want string
	}{
		{"req-qa-ascii-257", strings.Repeat("a", 257), "engine: move reason must be at most 256 bytes, got 257"},
		{"req-qa-two-byte-129", strings.Repeat("é", 129), "engine: move reason must be at most 256 bytes, got 258"},
		{"req-qa-four-byte-65", strings.Repeat("😀", 65), "engine: move reason must be at most 256 bytes, got 260"},
	}
	for _, r := range refused {
		issuer.send(t, ctx, qaBoundMoveCmd(r.id, proto.String(r.reason)))
		res := issuer.result(t, r.id)
		if res.GetOk() {
			t.Fatalf("%s: ok=true for a %d-byte reason", r.id, len(r.reason))
		}
		if res.GetError() != r.want {
			t.Fatalf("%s: error = %q, want the fold's text %q", r.id, res.GetError(), r.want)
		}
	}

	accepted := strings.Repeat("é", 128)
	issuer.send(t, ctx, qaBoundMoveCmd("req-qa-256", proto.String(accepted)))
	res := issuer.result(t, "req-qa-256")
	if !res.GetOk() {
		t.Fatalf("a 256-byte reason on the same connection was refused: %q", res.GetError())
	}
	if res.GetSequence() != head+1 {
		t.Fatalf("accepted move got sequence %d, want %d: a refused command appended", res.GetSequence(), head+1)
	}

	first := observer.next(t, 10*time.Second, func(f *vttv1.ServerFrame) bool {
		if f.GetResult() != nil {
			t.Fatalf("observer was sent a result for %q", f.GetResult().GetRequestId())
		}
		return f.GetEvent() != nil && f.GetEvent().GetSequence() > head
	}).GetEvent()
	if first.GetSequence() != head+1 {
		t.Fatalf("observer's first new event is sequence %d, want %d", first.GetSequence(), head+1)
	}
	if got := first.GetTokenMoved().GetReason(); got != accepted {
		t.Fatalf("observer's first new event carries reason %q, want the accepted 256-byte one", got)
	}
	if tok := c.State().Tokens[qaBoundToken]; tok.X != 2 || tok.Y != 2 {
		t.Fatalf("token at (%d,%d), want (2,2)", tok.X, tok.Y)
	}
	if err := issuer.readErr(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("issuer connection ended: %v", err)
	}
}
