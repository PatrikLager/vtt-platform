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

// Server is the WebSocket and HTTP gateway over one open Campaign and
// identity DB (SPEC-011).
type Server struct {
	campaign *campaign.Campaign
	ids      *identity.DB

	// Override only from this package's tests: New sets gatewayBuffer (SPEC-011).
	buffer int

	// Override only from this package's tests; zero means
	// store.SubscriberNoProgressTimeout (SPEC-011).
	noProgress time.Duration

	// Keep writeTimeout apart from noProgress: with one knob the write bound
	// fires first, and TestAWedgedConnectionIsTornDownAndOthersKeepServing
	// stops reaching the store's drop (SPEC-011).
	writeTimeout time.Duration

	// Override both only from this package's tests: New sets
	// gatewayPingInterval and gatewayPingTimeout (SPEC-011).
	pingInterval time.Duration
	pingTimeout  time.Duration

	// See EncodeFrame (SPEC-011).
	encodeFrame func(*vttv1.ServerFrame) ([]byte, error)

	// Never append presence to the log: who is online is not campaign history
	// (SPEC-007, SPEC-011).
	presence *presenceRegistry

	// Leave nil in production. TestAClientThatStopsReadingEntirelyIsTornDown
	// sets it to see a teardown its own client cannot.
	onServeDone func()

	// See WithRuleset (SPEC-012).
	ruleset *rules.Ruleset
	roller  rules.Roller

	// See WithAdventures (SPEC-012).
	adventures map[string]*adventure.Adventure

	// See WithAdventureGuides (SPEC-012).
	adventureGuides map[string]string

	// See WithStatic (SPEC-011).
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
// Close both after it stops serving: the caller owns them (SPEC-011).
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

	// Keep the backfill here: engine.Apply refuses a TokenMoved with no SceneId
	// and never reads From, and nothing after this point holds both the pre-move
	// token and the envelope (VTT-160).
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

// announcePresence tells every other connection that pc's participant arrived
// (SPEC-011). Drop the frame on an encode failure, never the connection:
// presence is repaired by the next snapshot.
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
	// Pass revoked(): presence does not run through the pump, so this is where a
	// revoked participant stops hearing it (SPEC-009, VTT-032).
	s.presence.broadcast(pc, b, s.revoked())
}

// announceDeparture announces pc's participant gone only while no connection
// of theirs remains (SPEC-011, announceIfAbsent).
func (s *Server) announceDeparture(pc *presenceConn) {
	// Resolve revoked here, before announceIfAbsent, even for a departure it
	// suppresses: an identity read inside it holds the registry's locks
	// (SPEC-011).
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

// revoked answers the connected participants whose Lookup is ErrInvalidToken
// (SPEC-009). Never call it under the registry's locks (SPEC-011).
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

// announcePromotion re-announces a promoted participant to every connection,
// theirs included (SPEC-009, VTT-047).
func (s *Server) announcePromotion(participantID string) {
	// Resolve, encode and send inside announceIfPresent, never as separate
	// steps: a connection that unwinds between them leaves a ghost (SPEC-011).
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

// handleRemoveActor appends the batch SPEC-017 states. Check nothing here: the
// fold refuses an unknown actor and a token placed after st was read, and a
// refused batch appends nothing.
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

	// Stamp the four fields the batch leaves zero: store.AppendBatch requires an
	// EventId (SPEC-017).
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

// handleJoinDoor applies set_join_door (SPEC-009).
func (s *Server) handleJoinDoor(requestID string, req *vttv1.SetJoinDoor) *vttv1.CommandResult {
	var open bool
	switch req.GetDoor() {
	case vttv1.JoinDoor_JOIN_DOOR_OPEN:
		open = true
	case vttv1.JoinDoor_JOIN_DOOR_CLOSED:
		open = false
	default:
		// Refuse, never default: guessing open admits strangers and guessing
		// closed locks the table out (SPEC-009, JoinDoor).
		return &vttv1.CommandResult{
			RequestId: requestID,
			Ok:        false,
			Error:     "gateway: set_join_door must say open or closed",
		}
	}
	// Pass the limit through as it came: SetJoinOpen owns the default (SPEC-009).
	if err := s.ids.SetJoinOpen(open, int(req.GetAdmitLimit())); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true}
}

// handleRotateJoinLink applies rotate_join_link (SPEC-009).
func (s *Server) handleRotateJoinLink(requestID string) *vttv1.CommandResult {
	if _, err := s.ids.RotateJoinSecret(); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	// Never return the secret here: a result travels the channel every
	// participant's frames use, and the DM reads it from GET /api/join-link
	// (SPEC-009).
	return &vttv1.CommandResult{RequestId: requestID, Ok: true}
}

// handlePromotion applies promote_participant (SPEC-009).
func (s *Server) handlePromotion(requestID string, req *vttv1.PromoteParticipant) *vttv1.CommandResult {
	// Refuse a dm or agent target here: Authorize does no I/O and cannot read the
	// target's role (SPEC-009, VTT-026).
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

// credentialGone reports whether Lookup answers ErrInvalidToken for the
// participant (SPEC-009). Match nothing else: an operational failure must not
// drop a frame (VTT-033).
func (s *Server) credentialGone(participantID string) bool {
	_, err := s.ids.Lookup(participantID)
	return errors.Is(err, identity.ErrInvalidToken)
}

// Keep these two apart: deliver closes with a reason for errEncodeFrame alone
// (SPEC-011).
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
