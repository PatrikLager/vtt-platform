package gateway

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/artlib"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep the drain on flush: it is what `for b := range out` gave for free, and
// nothing on the socket can observe its loss (SPEC-011).
func writeUntilFlushed(out <-chan []byte, flush <-chan struct{}, write func([]byte) bool) {
	for {
		select {
		case b := <-out:
			if !write(b) {
				return
			}
		case <-flush:
			for {
				select {
				case b := <-out:
					if !write(b) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

// Keep it slack for a bursty reader, never a limit on how far behind a connection
// may fall: the store drops a subscriber only on no progress (SPEC-011).
const gatewayBuffer = 256

// Keep it numerically in step with store.SubscriberNoProgressTimeout: gateway
// does not import store, and New gives writeTimeout the same figure (SPEC-011).
const gatewayNoProgress = 30 * time.Second

// Keep the read limit pinned in handleWS: the library's default is the same figure
// today and may move; every per-field cap in internal/engine is stricter (SPEC-011).
const maxWSFrameBytes = 32768

// Server is the WebSocket/HTTP gateway (spec §3, §7.9): it wires the pure
// core in this package (Authorize, ToEvent, EncodeFrame, DecodeCommand) to
// a real transport over one already-open Campaign and identity DB.
type Server struct {
	campaign *campaign.Campaign
	ids      *identity.DB

	// buffer defaults to gatewayBuffer; New sets it. It is unexported and
	// only overridden by this package's own internal tests (see
	// server_internal_test.go, precedent: campaign's poison_internal_test.go)
	// to make the overflow-closes-the-socket behavior deterministically
	// testable without appending gatewayBuffer+ events.
	buffer int

	// noProgress is how long a connection may fail to consume a waiting
	// envelope before the store cuts its subscription loose — after which
	// serveWS force-closes the socket rather than leaving a zombie. Zero means
	// store.SubscriberNoProgressTimeout. Unexported and settable like buffer,
	// because the gateway's own tests need a budget shorter than a wall-clock
	// half-minute.
	noProgress time.Duration

	// writeTimeout bounds a single conn.Write. SEPARATE from noProgress on
	// purpose: they are different policies at different layers, and conflating
	// them makes the socket path untestable. noProgress governs the store's
	// queue ("is this subscriber consuming?"); writeTimeout governs the socket
	// ("can this client still take bytes?"). With one knob the store always
	// drops first, so the write path can never be exercised in isolation —
	// which is exactly how its absence went unnoticed.
	writeTimeout time.Duration

	// pingInterval and pingTimeout govern the keepalive (keepalive.go): how
	// often an otherwise-silent connection is pinged, and how long the pong may
	// take before the peer is judged gone. Defaults are gatewayPingInterval and
	// gatewayPingTimeout; unexported and overridden only by this package's own
	// tests, exactly like buffer and noProgress above, because a suite cannot
	// wait twenty wall-clock seconds per assertion.
	pingInterval time.Duration
	pingTimeout  time.Duration

	// encodeFrame is EncodeFrame behind a per-Server seam, so a test can force
	// an encode failure without reaching across into another Server's
	// connections. See codec.go for why this is not a package global.
	encodeFrame func(*vttv1.ServerFrame) ([]byte, error)

	// presence tracks who is connected RIGHT NOW. Wire state, never appended
	// to the log — replaying a campaign must not resurrect a session
	// (spec §4). See presence.go.
	presence *presenceRegistry

	// onServeDone, when set, fires as each connection's serve returns. Nil in
	// production; it exists because "the server tore this connection down" is
	// otherwise unobservable from a client that is deliberately not reading —
	// and not reading is the whole precondition of the case it pins.
	onServeDone func()

	// See WithRuleset (SPEC-012).
	ruleset *rules.Ruleset
	roller  rules.Roller

	// See WithAdventures (SPEC-012).
	adventures map[string]*adventure.Adventure

	// See WithAdventureGuides (SPEC-012).
	adventureGuides map[string]string

	// static is the built web client, served at / when non-nil. Optional:
	// `vtt serve` without a bundle still serves the API, and a browser gets
	// an honest 404 rather than a panic. Set via WithStatic, boot time only.
	static fs.FS

	// Take mapsMu for every access once s serves; see WithMaps and mapByID
	// (SPEC-014).
	maps map[string]*mapdef.Map

	// See WithMapsDir (SPEC-014).
	mapsDir string

	// See WithArtDir (SPEC-014).
	artDir string

	// See WithCellPx (SPEC-012).
	cellPx int32

	// Guard maps with this and nothing else: mapsDir, artDir and cellPx are
	// written only before s serves (SPEC-014).
	mapsMu sync.RWMutex
}

// New constructs a Server over an already-open campaign and identity DB.
// The caller owns both handles' lifecycle (Close them after the Server is
// done serving). No ruleset is loaded — use_ability commands are rejected
// with a clean "no ruleset loaded" error until WithRuleset is called.
func New(c *campaign.Campaign, ids *identity.DB) *Server {
	return &Server{
		campaign: c, ids: ids,
		buffer: gatewayBuffer, noProgress: gatewayNoProgress, writeTimeout: gatewayNoProgress,
		pingInterval: gatewayPingInterval, pingTimeout: gatewayPingTimeout,
		presence:    newPresenceRegistry(),
		encodeFrame: EncodeFrame,
		cellPx:      DefaultCellPx,
	}
}

// WithRuleset configures s to resolve use_ability commands against rs with a
// crypto-seeded roller (SPEC-012). Do not call a With* method on a serving
// Server: they write without a lock.
func (s *Server) WithRuleset(rs *rules.Ruleset) *Server {
	s.ruleset = rs
	s.roller = rules.NewCryptoRoller()
	return s
}

// WithStatic serves fsys, the built web client, at /; without it the server is
// API-only. Take an fs.FS, not a path: cmd/vtt embeds the bundle, and go:embed
// cannot cross package directories (SPEC-011).
func (s *Server) WithStatic(fsys fs.FS) *Server {
	s.static = fsys
	return s
}

// WithAdventures configures s to serve advs via load_adventure, keyed by each
// adventure's own id (SPEC-012).
func (s *Server) WithAdventures(advs map[string]*adventure.Adventure) *Server {
	s.adventures = advs
	return s
}

// WithMaps sets the maps s starts with, keyed by each map's own id and
// validated by the caller (SPEC-014).
func (s *Server) WithMaps(m map[string]*mapdef.Map) *Server {
	s.maps = m
	return s
}

// WithMapsDir names the campaign's maps directory, where a load_map looks for a
// map s does not hold (SPEC-014).
func (s *Server) WithMapsDir(dir string) *Server {
	s.mapsDir = dir
	return s
}

// WithArtDir names the campaign's art directory, which every map load resolves
// art against (SPEC-014).
func (s *Server) WithArtDir(dir string) *Server {
	s.artDir = dir
	return s
}

// WithCellPx sets how many pixels one grid square of this campaign occupies
// (SPEC-012).
func (s *Server) WithCellPx(px int32) *Server {
	s.cellPx = px
	return s
}

// Handler returns the http.Handler for every route the gateway serves (SPEC-011).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/ws", s.handleWS)
	// Keep the patterns method-qualified: a POST to a read route must be a 405.
	mux.HandleFunc("POST /join", s.handleJoin)

	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("GET /api/ruleset", s.handleRuleset)
	mux.HandleFunc("GET /api/ruleset/guide", s.handleRulesetGuide)
	mux.HandleFunc("GET /api/join-link", s.handleJoinLink)
	mux.HandleFunc("GET /api/participants", s.handleParticipants)
	mux.HandleFunc("GET /api/adventures", s.handleAdventures)
	mux.HandleFunc("GET /api/adventures/{id}/guide", s.handleAdventureGuide)
	mux.HandleFunc("GET /api/maps", s.handleMaps)
	// Keep {file} single-segment: net/http's wildcard does not match across "/", and
	// art/ is flat (handleArtFile).
	mux.HandleFunc("GET /api/art/{file}", s.handleArtFile)

	// Keep the bundle last and at the bare "/": the most specific pattern wins, so no
	// API route falls into it. Unauthenticated on purpose: the program is public and
	// every route it calls is not (SPEC-011).
	if s.static != nil {
		mux.Handle("/", http.FileServerFS(s.static))
	}
	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleWS must verify the token before websocket.Accept: a refused credential
// gets an ordinary 401 and is never upgraded (SPEC-011).
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	after, err := parseAfter(r.URL.Query().Get("after"))
	if err != nil {
		http.Error(w, "gateway: invalid after parameter", http.StatusBadRequest)
		return
	}

	p, err := s.ids.Verify(r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "gateway: unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept has written the HTTP error; add nothing.
	}
	// Pin the read limit: the library's default is unpinned (SPEC-011).
	conn.SetReadLimit(maxWSFrameBytes)

	s.serve(r.Context(), conn, p, after)
}

func parseAfter(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

// SPEC-011 holds this lifecycle. Keep every write on outCh: the read loop, the
// pump, the presence registry and the head frame hand bytes to one writer.
func (s *Server) serve(ctx context.Context, conn *websocket.Conn, p *identity.Participant, after int64) {
	defer func() { _ = conn.CloseNow() }()
	if s.onServeDone != nil {
		defer s.onServeDone()
	}

	// Keep the seat deciding both what this connection receives and where the
	// subscription starts: for a projected seat they differ (seat.go).
	sub := newSeat(p, after)
	events, unsubscribe, catchUpHead, err := s.campaign.SubscribeWithNoProgressTimeout(sub.subscribeFrom(after), s.buffer, s.noProgress)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "gateway: subscribe failed")
		return
	}

	outCh := make(chan []byte, s.buffer)
	// Never close outCh: presence sends outside the registry lock, and closing a
	// channel under a parked sender panics that goroutine. flush is the writer's
	// stop signal (SPEC-011).
	flush := make(chan struct{})
	writerDone := make(chan struct{})
	// Keep activity declared here: the pinger reads it from a third goroutine
	// (keepalive.go).
	var activity writeActivity
	go func() {
		defer close(writerDone)
		write := stampedWrite(&activity, func(b []byte) bool {
			// Keep this write bounded: a client that stops reading otherwise parks the
			// writer, the pump and the read loop forever, and shutdown waits on the pump,
			// which waits on this (SPEC-011).
			wctx, wcancel := context.WithTimeout(ctx, s.writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, b)
			wcancel()
			return err == nil
		})
		writeUntilFlushed(outCh, flush, write)
	}()

	// Project the backlog first, so the head below is a sequence this seat can
	// reach (seat.catchUp).
	backlog, catchUpHead := sub.catchUp(ctx, events, catchUpHead)

	// Queue the head first, before any backlog, and unconditionally, head 0 included
	// (SPEC-007, SPEC-011).
	b, err := s.encodeFrame(&vttv1.ServerFrame{
		Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{HeadSequence: catchUpHead}},
	})
	if err != nil {
		// Fail closed: a connection that cannot announce its head hangs a caller with no
		// deadline (`vtt state dump`). Stop the writer first so it cannot outlive the
		// connection.
		unsubscribe()
		close(flush)
		<-writerDone
		_ = conn.Close(websocket.StatusInternalError, "gateway: encode catch-up head failed")
		return
	}
	select {
	case outCh <- b:
	case <-ctx.Done():
	}

	// Join presence after the head and before the pump, so the joiner is in its
	// own snapshot (SPEC-011).
	pc := &presenceConn{
		participantID: p.ID,
		displayName:   p.Name,
		out:           outCh,
		done:          make(chan struct{}),
	}
	// Keep the snapshot built and queued inside joinAndSend's critical section:
	// enqueued after, a delta can overtake it and leave a ghost.
	firstConnection := s.presence.joinAndSend(pc, func(present []*vttv1.PresenceChanged) []byte {
		b, err := s.encodeFrame(&vttv1.ServerFrame{
			Frame: &vttv1.ServerFrame_PresenceSnapshot{
				PresenceSnapshot: &vttv1.PresenceSnapshot{Present: present},
			},
		})
		if err != nil {
			return nil
		}
		return b
	})

	// Safe to reach twice: leave is idempotent and closes pc.done behind its own
	// membership check.
	leavePresence := func() {
		if last := s.presence.leave(pc); last {
			s.announceDeparture(pc)
		}
	}
	defer leavePresence()

	// closing tells the pump a closed events was shutdown's doing, not the
	// store's no-progress drop; set it before unsubscribing.
	var closing atomic.Bool

	// Keep the pump the only producer of this connection's envelopes: a perch queued
	// from the command goroutine reorders batches (handleSetViewpoint, perchBox).
	perches := newPerchBox()

	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)

		// Keep env outside the loop: the perches arm does not assign it.
		var env *vttv1.Envelope

		// Send the projected backlog from here, behind the same revocation check and
		// writer as everything else. Do not guard the check on len(backlog): every DM
		// connection has an empty one, and the mutation gate cannot see the guard.
		if s.credentialGone(pc.participantID) {
			if !closing.Load() {
				_ = conn.Close(websocket.StatusPolicyViolation,
					"gateway: credential no longer valid")
			}
			return
		}
		if !deliver(s.encodeFrame, backlog, outCh, writerDone, conn, &closing) {
			return
		}

		// Keep projecting and sending in this one goroutine: the order envelopes leave
		// in must be the order the projector's memory changed in (SPEC-011).
	pumping:
		for {
			select {
			case <-perches.wake:
				// Apply the perch here, not only send it: sub.perch moves the eyes and computes
				// what they newly see, beside sub.receive.
				actorID, ok := perches.take()
				if ok && !deliver(s.encodeFrame, sub.perch(actorID), outCh, writerDone, conn, &closing) {
					return
				}
				continue
			case e, ok := <-events:
				if !ok {
					break pumping
				}
				env = e
			}
			// Re-resolve on every delivery: a spectator issues no command, so this is where
			// revocation reaches one (SPEC-009, VTT-032). ErrInvalidToken only: an
			// operational failure must not drop an event.
			if s.credentialGone(pc.participantID) {
				// Close with a reason, as the command loop does, or which path fires decides
				// whether the peer is told why. conn.Close, not shutdown: shutdown waits on
				// this pump.
				if !closing.Load() {
					_ = conn.Close(websocket.StatusPolicyViolation,
						"gateway: credential no longer valid")
				}
				return
			}

			// Marshal per connection, in this goroutine: a seat's projection is per
			// recipient, and one event can be zero, one or several frames (SPEC-011). Do
			// not share bytes across seats with different projections.
			if !deliver(s.encodeFrame, sub.receive(env), outCh, writerDone, conn, &closing) {
				return
			}
		}
		// events closed without shutdown: the store dropped this subscription. Force the
		// socket closed so conn.Read fails and the read loop runs shutdown; a wedged
		// writer exits by its own path (SPEC-011).
		if !closing.Load() {
			_ = conn.CloseNow()
		}
	}()

	// Announce after the pump is running, and synchronously: before it, N wedged
	// peers cost the joiner N budgets; in a goroutine, a fast disconnect lets
	// DISCONNECTED overtake CONNECTED (SPEC-011).
	if firstConnection {
		s.announcePresence(pc)
	}

	// Keep this order: leave presence, mark closing, unsubscribe, drain the pump,
	// flush the writer. close(flush) has one caller per path; closing a channel
	// twice panics.
	shutdown := func() {
		// Leave first: pc.done lets a fan-out holding this connection abandon it
		// before the writer stops (SPEC-011).
		leavePresence()
		closing.Store(true)
		unsubscribe()
		<-pumpDone
		close(flush)
		<-writerDone
	}

	// Start the pinger after the hand-rolled teardowns above: a CloseNow from here
	// would pre-empt their reasoned Close (keepalive.go).
	stopPing := make(chan struct{})
	defer close(stopPing)
	go pingUntilStopped(ctx, s.pingInterval, s.pingTimeout, stopPing,
		activity.busy, conn.Ping, func() { _ = conn.CloseNow() })

	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			shutdown()
			return
		}

		cmd, err := DecodeCommand(raw)
		if err != nil {
			// Close only this connection, with a reason (SPEC-011).
			shutdown()
			_ = conn.Close(websocket.StatusPolicyViolation, "gateway: malformed frame")
			return
		}

		// Re-resolve on every command: authorization is live, a connection-time answer
		// is not (SPEC-009, VTT-031). The pump re-resolves on delivery for the same
		// reason; change both or neither.
		now, err := s.ids.Lookup(p.ID)
		if errors.Is(err, identity.ErrInvalidToken) {
			// Revoked or gone: close, there is nothing left this connection may do (VTT-032).
			shutdown()
			_ = conn.Close(websocket.StatusPolicyViolation, "gateway: credential no longer valid")
			return
		}

		// Any other lookup error is operational: refuse the command and keep the
		// connection (VTT-033). A dead writer, seen on the result's writerDone arm,
		// costs the connection.
		result := s.answerCommand(now, err, cmd, perches)
		b, err := EncodeFrame(&vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Result{Result: result}})
		if err != nil {
			continue
		}
		select {
		case outCh <- b:
		case <-writerDone:
			shutdown()
			return
		}
	}
}

// Keep the kind clipped: it is the one author-written string this function
// renders, and a Blocked reason that interpolates a map file's text needs the
// same clip (SPEC-013).
func describeBlockage(why string) string {
	if kind, ok := strings.CutPrefix(why, "scenery: "); ok {
		return "something (a " + artlib.Clip(kind, artlib.MaxFragment) + ") is in the way"
	}
	if strings.HasPrefix(why, "unknown scene ") {
		return "that destination is not part of any scene this table has created"
	}
	return why
}

// Never enqueue a frame here: the pump is the only producer of a connection's
// envelopes (SPEC-011).
func (s *Server) answerCommand(p *identity.Participant, lookupErr error, cmd *vttv1.ClientCommand,
	perches *perchBox) *vttv1.CommandResult {
	switch sv, isPerch := cmd.GetCommand().(*vttv1.ClientCommand_SetViewpoint); {
	case lookupErr != nil:
		// Refuse this command and keep the connection (SPEC-009, VTT-033).
		return &vttv1.CommandResult{
			RequestId: cmd.GetRequestId(),
			Ok:        false,
			Error:     "gateway: identity unavailable",
		}

	case isPerch:
		// Keep the perch out of handleCommand: it appends nothing and its effect is
		// this connection's alone (SPEC-013).
		return s.handleSetViewpoint(p, cmd, sv.SetViewpoint, perches)

	default:
		return s.handleCommand(p, cmd)
	}
}

// Refuse on a nil state: authorizing against no world decides from nothing
// (SPEC-013).
func (s *Server) authorize(p *identity.Participant, cmd *vttv1.ClientCommand) (*engine.State, *vttv1.CommandResult) {
	requestID := cmd.GetRequestId()
	st := s.campaign.State()
	if st == nil {
		return nil, &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: "gateway: campaign unavailable"}
	}
	if err := Authorize(p, cmd, st); err != nil {
		return nil, &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return st, nil
}

// Hand the perch to the pump and apply nothing here: a frame emitted from this
// goroutine lands inside a projected batch and the watcher's fold stops
// (SPEC-011, SPEC-013).
func (s *Server) handleSetViewpoint(p *identity.Participant, cmd *vttv1.ClientCommand,
	req *vttv1.SetViewpoint, perches *perchBox) *vttv1.CommandResult {
	if _, refusal := s.authorize(p, cmd); refusal != nil {
		return refusal
	}
	perches.set(req.GetActorId())
	return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true}
}

// handleCommand runs the command path SPEC-013 states, in its order. Never
// close the connection or write to the wire here: serve owns transport
// (SPEC-011).
func (s *Server) handleCommand(p *identity.Participant, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	requestID := cmd.GetRequestId()

	st, refusal := s.authorize(p, cmd)
	if refusal != nil {
		return refusal
	}

	// Keep this gate the player's alone: the DM and the agent author the world
	// (SPEC-013).
	if p.Role == identity.RolePlayer {
		if mt, ok := cmd.GetCommand().(*vttv1.ClientCommand_MoveToken); ok {
			if tok, known := st.Tokens[mt.MoveToken.GetTokenId()]; known {
				to := mt.MoveToken.GetTo()
				// Ask sight FIRST and answer with one string: a refusal that varies with
				// the occupant is a terrain oracle
				// (TestAPlayerCannotProbeTheDarkWithMoveCommands).
				if !canSee(viewerFor(p), st, tok.SceneID, to) {
					return &vttv1.CommandResult{
						RequestId: requestID, Ok: false,
						Error: "gateway: cannot move there — you cannot see that square",
					}
				}
				// Name the obstruction only here, after sight: this player has been sent
				// this square's terrain (SPEC-013).
				if blocked, why := st.Blocked(tok.SceneID, to.GetX(), to.GetY()); blocked {
					return &vttv1.CommandResult{
						RequestId: requestID, Ok: false,
						Error: "gateway: cannot move there — " + describeBlockage(why),
					}
				}
			}
		}
	}

	// Keep the validators here, for every role, before anything is written: not
	// in Authorize, since they refuse a form and not an issuer; not in ToEvent
	// (TestEveryClientCommandConverts); not in the fold (SPEC-013).
	if g, ok := cmd.GetCommand().(*vttv1.ClientCommand_GrantActorControl); ok {
		if err := validateGrantActorControl(g.GrantActorControl); err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
	}

	if aa, ok := cmd.GetCommand().(*vttv1.ClientCommand_AddActor); ok {
		if err := validateAddActor(aa.AddActor); err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
	}

	if un, ok := cmd.GetCommand().(*vttv1.ClientCommand_UpsertNote); ok {
		if err := validateUpsertNote(un.UpsertNote); err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
	}

	// Dispatch these four before ToEvent: each appends a batch, never one envelope
	// (SPEC-012, SPEC-013).
	if ua, ok := cmd.GetCommand().(*vttv1.ClientCommand_UseAbility); ok {
		return s.handleUseAbility(requestID, ua.UseAbility, st, p)
	}
	if la, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadAdventure); ok {
		return s.handleLoadAdventure(requestID, la.LoadAdventure, st, p)
	}
	if lm, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadMap); ok {
		return s.handleLoadMap(requestID, lm.LoadMap, p)
	}
	if ra, ok := cmd.GetCommand().(*vttv1.ClientCommand_RemoveActor); ok {
		return s.handleRemoveActor(requestID, ra.RemoveActor, st, p)
	}
	// Answer these three without ToEvent: they append nothing (SPEC-007).
	if pp, ok := cmd.GetCommand().(*vttv1.ClientCommand_PromoteParticipant); ok {
		return s.handlePromotion(requestID, pp.PromoteParticipant)
	}
	if d, ok := cmd.GetCommand().(*vttv1.ClientCommand_SetJoinDoor); ok {
		return s.handleJoinDoor(requestID, d.SetJoinDoor)
	}
	if _, ok := cmd.GetCommand().(*vttv1.ClientCommand_RotateJoinLink); ok {
		return s.handleRotateJoinLink(requestID)
	}

	env, err := ToEvent(cmd, p)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// Keep the backfill here: engine.Apply never reads SceneId or From from a
	// TokenMoved, and nothing after this point holds both the pre-move token and
	// the envelope (VTT-160).
	if tm, ok := env.Payload.(*vttv1.Envelope_TokenMoved); ok {
		if tok, ok := st.Tokens[tm.TokenMoved.GetTokenId()]; ok {
			tm.TokenMoved.SceneId = tok.SceneID
			tm.TokenMoved.From = &vttv1.GridPosition{X: tok.X, Y: tok.Y}
		}
	}

	seq, err := s.campaign.Append(env)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: seq}
}

// announcePresence tells everyone EXCEPT pc that pc's participant ARRIVED.
//
// Arrivals only, since #55. Departures go through announceDeparture, which
// re-checks absence at send time — a departure is only news if they are still
// gone, and a reconnect can land between leave() deciding "that was the last
// connection" and the announcement leaving. An arrival needs no such re-check:
// the connection announcing it is registered and is the reason the news is
// true, so there is nothing that could have undone it in between.
//
// An encode failure is dropped rather than escalated: presence is soft state,
// every client is re-synced by the snapshot it gets on connect, and tearing a
// healthy connection down because someone else's status frame would not
// marshal would turn a cosmetic fault into an outage. That is the opposite of
// the catch-up head, which fails the connection closed — a client that cannot
// learn where catch-up ends cannot function, and one that misses a presence
// blip can.
func (s *Server) announcePresence(pc *presenceConn) {
	b, err := s.encodeFrame(&vttv1.ServerFrame{
		Frame: &vttv1.ServerFrame_PresenceChanged{
			PresenceChanged: &vttv1.PresenceChanged{
				ParticipantId: pc.participantID,
				DisplayName:   pc.displayName,
				State:         vttv1.PresenceState_PRESENCE_STATE_CONNECTED,
			},
		},
	})
	if err != nil {
		return
	}
	// Revoked participants are denied this frame. Presence is the ONE delivery
	// path that does not run through the pump, so without this a revoked
	// stranger who came in on a leaked link went on watching the guest list
	// arrive and leave until the table next appended an event (spec §3.2).
	s.presence.broadcast(pc, b, s.revoked())
}

// announceDeparture tells the table pc's participant has gone — unless they
// have already come back.
//
// leave() decides "that was their last connection" and this is a SEPARATE step,
// so a reconnect landing in between would otherwise make the table's last word
// about a PRESENT participant "DISCONNECTED", and tell their fresh connection
// itself gone (broadcast excludes by connection pointer, so a participant's own
// other connection is a legitimate target). Measured at 1 inversion in 20,000
// rounds before the re-check existed; it persists until a snapshot, and spec
// §3.4 makes reconnection manual, so it stays wrong until the player acts on a
// problem they cannot see. See presenceRegistry.announceIfAbsent (#55).
func (s *Server) announceDeparture(pc *presenceConn) {
	// revoked() resolved HERE, before the fan-out is held, for two reasons and
	// the second is the binding one. It reads identity once per connected
	// participant, which inside the walk would queue every other announcement
	// behind a pile of SQLite reads. And it CANNOT be deferred until after the
	// absence check to save that work on a suppressed departure: the check and
	// the target selection have to happen under ONE hold of the registry lock,
	// or membership can change between them and the suppression this exists for
	// stops holding. So a suppressed departure pays for a deny set it does not
	// use, deliberately.
	deny := s.revoked()
	s.presence.announceIfAbsent(pc.participantID, deny, func() []byte {
		b, err := s.encodeFrame(&vttv1.ServerFrame{
			Frame: &vttv1.ServerFrame_PresenceChanged{
				PresenceChanged: &vttv1.PresenceChanged{
					ParticipantId: pc.participantID,
					DisplayName:   pc.displayName,
					State:         vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED,
				},
			},
		})
		if err != nil {
			return nil
		}
		return b
	})
}

// revoked resolves every connected participant and returns those whose
// credential no longer stands.
//
// Resolved HERE rather than inside the registry, and that placement is the
// point: a Lookup inside broadcast's loop would put one SQLite read per
// connection under the registry's global mutex — the fan-out stall
// presenceSendBudget exists to prevent, reintroduced on the path that fans out.
//
// Only ErrInvalidToken denies. An operational failure is not a fact about
// anybody's credential, and dropping presence frames on a busy database would
// make a transient look like the whole table walking out.
//
// The cost is one lookup per connected participant per presence frame, and
// presence frames are rare — somebody joins, somebody leaves — unlike events.
// nil when nobody is revoked, which is the ordinary case and allocates nothing.
func (s *Server) revoked() map[string]bool {
	var out map[string]bool
	for _, id := range s.presence.participantIDs() {
		if _, err := s.ids.Lookup(id); errors.Is(err, identity.ErrInvalidToken) {
			if out == nil {
				out = make(map[string]bool, 1)
			}
			out[id] = true
		}
	}
	return out
}

// announcePromotion re-announces a promoted participant to the whole table,
// their own connections included.
//
// The frame carries no NEW presence information — they were already connected
// and still are. It exists as a NUDGE, and it closes the half of promotion
// that live re-resolution does not reach: the server now lets a promoted
// spectator act on their existing socket, but their own browser read its role
// once at connect (/api/me) and nothing ever told it that role moved. So they
// could act and their client offered them nothing to act with — the server
// said yes to a screen with no controls on it.
//
// Sent to everyone rather than just to them, because a stale role is a stale
// role: the DM's console lists roles too.
//
// Found by the e2e. No unit test could see it — every layer was correct, and
// what was wrong was a browser's idea of itself.
func (s *Server) announcePromotion(participantID string) {
	// Resolve, encode and send as ONE UNINTERRUPTIBLE fan-out — see
	// announceIfPresent, which holds fanOut across all three. Doing it in three
	// separable steps let the participant's last connection unwind between the
	// resolve and the send, so the table saw DISCONNECTED then CONNECTED and
	// kept a ghost in its list for the rest of the session.
	s.presence.announceIfPresent(participantID, func(name string) []byte {
		b, err := s.encodeFrame(&vttv1.ServerFrame{
			Frame: &vttv1.ServerFrame_PresenceChanged{
				PresenceChanged: &vttv1.PresenceChanged{
					ParticipantId: participantID,
					DisplayName:   name,
					State:         vttv1.PresenceState_PRESENCE_STATE_CONNECTED,
				},
			},
		})
		if err != nil {
			return nil
		}
		return b
	})
}

// handleRemoveActor takes an actor out of the world AND the pieces it had on
// the board, as one ordered batch (retraction-leaves spec §5.2, Task 9).
//
// THE CASCADE IS CORRECTNESS, NOT CONVENIENCE, and it is the whole reason this
// command is not a plain ToEvent conversion. engine.Apply's TokenPlaced arm
// and client/src/fold.ts's tokenPlaced arm both refuse a token whose actor
// they do not know, in almost the same words — so an ActorRemoved that left
// this actor's tokens standing would leave a world whose own introductions no
// longer fold, and through client/src/session.ts's
// re-fold-the-whole-log-on-every-event that is a permanent client freeze. The
// batch is therefore one TokenRemoved per token of this actor, in token-id
// order, and then the ActorRemoved, last.
//
// ONE campaign.AppendBatch, which is what makes it atomic: the batch is
// validated by folding the whole of it before any of it is persisted, so a
// rejection anywhere appends nothing. handleLoadMap (map.go) is the precedent
// for the shape, down to this function's own stamping loop.
//
// SORTED BY TOKEN ID, not by map iteration order. Go randomises the latter, and
// the log is permanent: two identical tables would otherwise record the same
// removal in different orders, and no test could assert either.
//
// NOTHING IS CHECKED HERE. An unknown actor id produces a one-event batch that
// campaign.AppendBatch rejects with the fold's own "removed unknown actor"
// wording — the same division of labour remove_token has, where engine.Apply
// owns the unknown-subject rejection and this layer owns the shape. And the
// snapshot this reads can go stale between here and the append, which the fold
// also owns: its ActorRemoved arm refuses an actor whose tokens still stand, so
// a place_token that lands in between costs a clean rejection rather than an
// orphaned token.
//
// CONTROL GRANTS NEED NO EVENT: controller_ids is a field on the Actor, so
// whoever held this actor stops holding it because the actor is gone.
func (s *Server) handleRemoveActor(requestID string, cmd *vttv1.RemoveActor,
	st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	var tokenIDs []string
	for id, tok := range st.Tokens {
		if tok.ActorID == cmd.GetActorId() {
			tokenIDs = append(tokenIDs, id)
		}
	}
	sort.Strings(tokenIDs)

	envs := make([]*vttv1.Envelope, 0, len(tokenIDs)+1)
	for _, id := range tokenIDs {
		envs = append(envs, &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{
			TokenRemoved: &vttv1.TokenRemoved{TokenId: id}}})
	}
	envs = append(envs, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{
		ActorRemoved: &vttv1.ActorRemoved{ActorId: cmd.GetActorId()}}})

	// Stamped here rather than by ToEvent, which this command does not use —
	// the same loop handleLoadMap runs over the envelopes mapdef.Compile
	// returns, and for the same reason: a batch has no single envelope for
	// ToEvent to build.
	now := timestamppb.Now()
	for _, env := range envs {
		id, err := newEventID()
		if err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
		env.EventId = id
		env.ParticipantId = p.ID
		env.ActorRole = string(p.Role)
		env.OccurredAt = now
	}

	firstSeq, err := s.campaign.AppendBatch(envs)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq}
}

// handleJoinDoor opens or closes the shared join link (joining-a-table §2).
//
// Authorize has already bounded WHO may issue this (dm/agent, authz.go), so
// this applies it. It appends NOTHING: the door is operational state, like
// presence, and replaying a campaign must never reopen a door somebody closed.
func (s *Server) handleJoinDoor(requestID string, req *vttv1.SetJoinDoor) *vttv1.CommandResult {
	var open bool
	switch req.GetDoor() {
	case vttv1.JoinDoor_JOIN_DOOR_OPEN:
		open = true
	case vttv1.JoinDoor_JOIN_DOOR_CLOSED:
		open = false
	default:
		// REFUSED, not defaulted, and this is why the contract carries an enum
		// rather than a bool: protojson omits zero values, so `bool open`
		// would put CLOSED on the wire as an absent field and make a sender
		// that forgot to set it indistinguishable from one asking to shut the
		// door. Both guesses are bad in their own direction — guess open and a
		// bug admits strangers, guess closed and a bug locks the table out
		// mid-session — so neither is made.
		return &vttv1.CommandResult{
			RequestId: requestID,
			Ok:        false,
			Error:     "gateway: set_join_door must say open or closed",
		}
	}
	// The budget travels straight through. A non-positive value — which is
	// what an absent field decodes to, since protojson omits zero values —
	// becomes DefaultAdmitLimit inside SetJoinOpen rather than being guessed at
	// here, so the CLI and the wire cannot drift into two different defaults.
	if err := s.ids.SetJoinOpen(open, int(req.GetAdmitLimit())); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true}
}

// handleRotateJoinLink mints a new join secret, closing a LEAKED link to
// newcomers without touching anybody already through it.
func (s *Server) handleRotateJoinLink(requestID string) *vttv1.CommandResult {
	if _, err := s.ids.RotateJoinSecret(); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	// The new secret is deliberately NOT returned here. CommandResult carries
	// no payload, and adding one to smuggle a credential back would put a
	// shared secret on the channel every participant's frames travel. The DM
	// reads it from GET /api/join-link instead, which is authenticated and
	// dm/agent only.
	return &vttv1.CommandResult{RequestId: requestID, Ok: true}
}

// handlePromotion applies an authorized role change.
//
// Authorize has already bounded WHO may issue this and WHAT role it may name
// (gateway/authz.go: dm/agent only, targeting player or spectator only), so
// this applies it and reports what identity said. It deliberately appends
// nothing: the whole point of keeping role identity-side is that there is one
// source of truth, and writing an event beside it would create a second.
func (s *Server) handlePromotion(requestID string, req *vttv1.PromoteParticipant) *vttv1.CommandResult {
	// A PROMOTION MAY NOT UNMAKE A DM OR AN AGENT.
	//
	// Authorize bounds what a promotion may promote TO (authz.go: player or
	// spectator only, spec §3.1a). It cannot bound who may be promoted FROM,
	// because that is a fact about the target's CURRENT row and Authorize does
	// no I/O — so the check lives here, where the lookup is.
	//
	// Without it, promote_participant(dm_id, "spectator") names a permitted
	// role and goes through. Nobody left at the table could undo it: promotion
	// cannot reach dm by design, so it would take host access and `vtt invite`.
	// Agents are authorized to promote, which is the sharp end — one agent
	// having a bad day could lock every human out of their own campaign.
	target, err := s.ids.Lookup(req.GetParticipantId())
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	if target.Role == identity.RoleDM || target.Role == identity.RoleAgent {
		return &vttv1.CommandResult{
			RequestId: requestID,
			Ok:        false,
			Error:     "gateway: not authorized: a dm or agent cannot be demoted by a promotion",
		}
	}
	if err := s.ids.SetRole(req.GetParticipantId(), identity.Role(req.GetRole())); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	s.announcePromotion(req.GetParticipantId())
	return &vttv1.CommandResult{RequestId: requestID, Ok: true}
}

// credentialGone reports whether this participant's credential has been
// revoked since the connection was accepted.
//
// ErrInvalidToken ONLY, and that narrowness is the point: an operational
// failure must not silently drop an event or a connection. Losing a frame is
// worse than a moment's delay in removing somebody, and the very next event
// asks again.
func (s *Server) credentialGone(participantID string) bool {
	_, err := s.ids.Lookup(participantID)
	return errors.Is(err, identity.ErrInvalidToken)
}

// The two ways enqueueEvents can stop short, as sentinels so the caller can
// tell them apart without matching a sentence: one is this server's bug and
// deserves a close frame saying so, the other is the connection already going
// away underneath it.
var (
	errEncodeFrame = errors.New("gateway: encode event frame")
	errWriterGone  = errors.New("gateway: writer stopped")
)

// Keep closing a pointer: it is serve's own atomic, and vet refuses a copy.
func deliver(encode func(*vttv1.ServerFrame) ([]byte, error), envs []*vttv1.Envelope,
	outCh chan<- []byte, writerDone <-chan struct{}, conn *websocket.Conn, closing *atomic.Bool) bool {
	err := enqueueEvents(encode, envs, outCh, writerDone)
	if err == nil {
		return true
	}
	// errWriterGone needs no close: serve's deferred CloseNow disposes of it.
	if errors.Is(err, errEncodeFrame) && !closing.Load() {
		_ = conn.Close(websocket.StatusInternalError, "gateway: encode failed")
	}
	return false
}

// Stop at the first failure, never skip: a projected batch's order is
// load-bearing for the client's fold (SPEC-011). Keep the encoder injected, or
// the error arm is unreachable by wiring.
func enqueueEvents(encode func(*vttv1.ServerFrame) ([]byte, error), envs []*vttv1.Envelope, outCh chan<- []byte, writerDone <-chan struct{}) error {
	for _, pe := range envs {
		b, err := encode(&vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: pe}})
		if err != nil {
			return fmt.Errorf("%w: sequence %d: %w", errEncodeFrame, pe.GetSequence(), err)
		}
		select {
		case outCh <- b:
		case <-writerDone:
			return errWriterGone
		}
	}
	return nil
}
