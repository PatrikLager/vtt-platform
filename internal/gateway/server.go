package gateway

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// gatewayBuffer is Server.buffer's default (New sets it; see that field's
// doc comment for the test-only override seam). It sizes the hand-off channel
// store.Store.Subscribe hands back, and the per-connection outbound byte
// channel in serve.
//
// It is NOT a limit on how far behind a connection may fall — that reading was
// the bug. The store used to drop any subscriber whose channel filled, which
// an atomic batch larger than this could trigger regardless of how fast the
// client was reading, making this constant a ceiling on adventure size. The
// store now queues per subscriber and drops only on no progress
// (gatewayNoProgress). This is slack for bursty readers, nothing more (see the pump
// goroutine below) so the client observes the disconnect and can reconnect
// with a fresh `after` cursor; the log is always the source of truth for
// whatever was missed. 256 is generous headroom for one connection's
// fan-out lag under normal load.
const gatewayBuffer = 256

// gatewayNoProgress is Server.noProgress's default: how long a connection may
// fail to accept a frame before the server stops waiting on it.
//
// It bounds the store's per-subscriber queue
// (store.SubscriberNoProgressTimeout — kept numerically in step with this
// deliberately; gateway does not import store). Once that budget elapses the
// store closes the subscription, and the pump below force-closes the socket.
//
// Prior art: MapTool (net.rptools.clientserver) bounds its per-connection
// queue not at all and detects a departed client purely with a socket timeout
// — one minute, against a 20s client heartbeat. We have no such deadline on
// conn.Write; see the pump's note on the window that leaves open.
const gatewayNoProgress = 30 * time.Second

// maxWSFrameBytes is the per-connection websocket message read limit,
// pinned explicitly via conn.SetReadLimit in handleWS below (amendment-
// mandated merge-gate fix, review finding: "as" was the only participant-
// writable world-layer field with no cap of its own, its EFFECTIVE bound
// resting silently on coder/websocket's undocumented default read limit —
// nothing in internal/ or cmd/ had ever called SetReadLimit). This value
// matches that library default (github.com/coder/websocket v1.8.15,
// websocket.Conn's own doc comment: "By default, the connection has a
// message read limit of 32768 bytes") byte-for-byte, so pinning it changes
// no observed behavior today — the point is OWNERSHIP: this is now the
// gateway's own stated outer size posture for every inbound command frame,
// not an inherited default that could silently drift wider (or narrower)
// on a future coder/websocket upgrade. Every command's own per-field caps
// (internal/engine/apply.go's maxTextBytes etc.) are stricter than this and
// unaffected by it; this is the layer's outermost wire-frame bound, the one
// thing standing between an oversized frame and Accept ever handing that
// connection's bytes to DecodeCommand at all.
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

	// presence tracks who is connected RIGHT NOW. Wire state, never appended
	// to the log — replaying a campaign must not resurrect a session
	// (spec §4). See presence.go.
	presence *presenceRegistry

	// onServeDone, when set, fires as each connection's serve returns. Nil in
	// production; it exists because "the server tore this connection down" is
	// otherwise unobservable from a client that is deliberately not reading —
	// and not reading is the whole precondition of the case it pins.
	onServeDone func()

	// ruleset/roller are OPTIONAL server config (ruleset-interpreter Task
	// 6): nil ruleset is today's behavior, unchanged — every command this
	// package handled before Task 6 keeps working exactly as before, and a
	// use_ability command gets a clean "no ruleset loaded" CommandResult
	// (ok=false) rather than a connection drop or a crash. Set together via
	// WithRuleset — see that method's doc comment for why roller is always
	// the production crypto.Roller when ruleset is non-nil (never
	// separately configurable at this layer).
	ruleset *rules.Ruleset
	roller  rules.Roller

	// adventures is OPTIONAL server config (adventure-format Task 4): nil/
	// empty is today's behavior — a load_adventure command gets a clean "no
	// adventures available" CommandResult (ok=false) rather than a
	// connection drop or a crash, exactly matching ruleset's own "no
	// ruleset loaded" posture. Set via WithAdventures, BOOT TIME ONLY — the
	// map is never mutated or re-loaded per request; adventure-format spec
	// §7: "All available adventures load+validate at BOOT (fail loud at
	// startup, not at the table)". Keyed by the adventure's own manifest id
	// (adventure.Adventure.ID), not its directory name.
	adventures map[string]*adventure.Adventure

	// adventureGuides is the markdown served by /api/adventures/{id}/guide,
	// keyed by adventure id. Set via WithAdventureGuides, boot time only.
	// Held separately from adventures because the gateway does no file I/O:
	// cmd/vtt reads the guides and hands them over (ADR-008).
	adventureGuides map[string]string

	// static is the built web client, served at / when non-nil. Optional:
	// `vtt serve` without a bundle still serves the API, and a browser gets
	// an honest 404 rather than a panic. Set via WithStatic, boot time only.
	static fs.FS
}

// New constructs a Server over an already-open campaign and identity DB.
// The caller owns both handles' lifecycle (Close them after the Server is
// done serving). No ruleset is loaded — use_ability commands are rejected
// with a clean "no ruleset loaded" error until WithRuleset is called.
func New(c *campaign.Campaign, ids *identity.DB) *Server {
	return &Server{
		campaign: c, ids: ids,
		buffer: gatewayBuffer, noProgress: gatewayNoProgress, writeTimeout: gatewayNoProgress,
		presence: newPresenceRegistry(),
	}
}

// WithRuleset configures s to resolve use_ability commands against rs,
// using the production crypto-seeded Roller (rules.NewCryptoRoller — dice
// are rolled ONCE at Resolve time and recorded onto the resulting
// AbilityUsed event; replay never re-rolls, ruleset-interpreter spec §5
// decision 3). Returns s for call-site chaining (e.g.
// gateway.New(c, ids).WithRuleset(rs)); mutates s in place rather than
// copying, so it is not safe to call concurrently with s already serving
// traffic — callers configure a Server fully before handing it to a
// listener, exactly like New itself.
func (s *Server) WithRuleset(rs *rules.Ruleset) *Server {
	s.ruleset = rs
	s.roller = rules.NewCryptoRoller()
	return s
}

// WithStatic serves fsys (the built web client) at /. Optional — a server
// without it is API-only, which is what `vtt serve` is before the client is
// built and what the harness's own throwaway servers always are.
//
// Takes an fs.FS rather than a directory path because the bundle is EMBEDDED
// in the binary (cmd/vtt/embed.go): go:embed cannot cross package
// directories, so cmd/vtt owns the embed and hands the FS over, the same
// division of labour adventure guides already use.
func (s *Server) WithStatic(fsys fs.FS) *Server {
	s.static = fsys
	return s
}

// WithAdventures configures s to serve advs via load_adventure, keyed by
// each adventure's own id (advs.Adventure.ID — the caller, cmd/vtt's boot
// glue, is responsible for building this map with THAT key, not the
// directory name it loaded from). advs is expected already fully loaded and
// validated (adventure.Load, boot time, fail loud on any error — spec §7);
// this method does no I/O and no validation of its own. Returns s for
// call-site chaining (mirrors WithRuleset); mutates s in place, so it is
// not safe to call concurrently with s already serving traffic.
func (s *Server) WithAdventures(advs map[string]*adventure.Adventure) *Server {
	s.adventures = advs
	return s
}

// Handler returns the http.Handler routing /healthz and /ws (spec §3).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/ws", s.handleWS)
	// Read-only metadata (metadata.go). Method-qualified patterns, so a POST
	// to a read endpoint is a clean 405 rather than a silent success.
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("GET /api/ruleset", s.handleRuleset)
	mux.HandleFunc("GET /api/ruleset/guide", s.handleRulesetGuide)
	mux.HandleFunc("GET /api/adventures", s.handleAdventures)
	mux.HandleFunc("GET /api/adventures/{id}/guide", s.handleAdventureGuide)

	// The client bundle, LAST and at the bare "/" pattern. ServeMux matches
	// the most specific pattern, so the explicit routes above always win — a
	// naive catch-all registered first would serve index.html to the client's
	// own /api fetches, which then fail to parse as JSON with an error that
	// says nothing about routing.
	//
	// Unauthenticated on purpose: the browser must load the app before it has
	// anywhere to type a token. What is public here is the PROGRAM; every
	// route it then calls is authenticated.
	if s.static != nil {
		mux.Handle("/", http.FileServerFS(s.static))
	}
	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleWS resolves both connection parameters from the URL — `token` and
// `after` — BEFORE ever calling websocket.Accept (binding design decision):
// identity.Verify runs against the plain HTTP request, so a bad or revoked
// token gets an ordinary HTTP 401 response and the connection is never
// upgraded. Only a verified request reaches Accept.
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
		return // Accept already wrote the HTTP error response.
	}
	// Own the wire size posture (maxWSFrameBytes's doc comment): pin the
	// read limit explicitly rather than leaving it to coder/websocket's
	// unpinned default.
	conn.SetReadLimit(maxWSFrameBytes)

	s.serve(r.Context(), conn, p, after)
}

func parseAfter(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

// serve runs one connection's full lifecycle: catch-up + live subscription,
// the inbound command loop, and a single writer goroutine that owns every
// write to conn.
//
// Writer choice (documented per the binding constraint): writes are
// serialized through outCh and ONE writer goroutine below, rather than a
// per-connection mutex. Both the command loop (CommandResult replies) and
// the broadcast pump goroutine (live/catch-up Envelopes from `events`) only
// ever hand byte slices to outCh — neither goroutine calls conn.Write
// itself — so two writes can never race on the wire, and the ordering of
// interleaved results/events is whatever order they arrive at outCh.
func (s *Server) serve(ctx context.Context, conn *websocket.Conn, p *identity.Participant, after int64) {
	defer func() { _ = conn.CloseNow() }()
	if s.onServeDone != nil {
		defer s.onServeDone()
	}

	events, cancel, catchUpHead, err := s.campaign.SubscribeWithNoProgressTimeout(after, s.buffer, s.noProgress)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "gateway: subscribe failed")
		return
	}

	outCh := make(chan []byte, s.buffer)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for b := range outCh {
			// Bounded, and load-bearing. A client that stops reading backs
			// the socket up; without a deadline this parks forever while the
			// command loop still waits in conn.Read — a connection that is
			// gone but looks alive, leaking its goroutines and socket.
			//
			// It is not belt-and-braces with the pump's post-loop close
			// below: under the store's no-progress policy a subscriber is
			// only dropped after making ZERO progress, which means this
			// writer is parked, which means the pump is parked handing off to
			// outCh — so by construction the pump is NOT ranging `events`
			// when they close, and that close cannot be observed. This
			// deadline is what unwinds it. shutdown() depends on it too: it
			// waits on pumpDone, which can only come after this returns.
			//
			// MapTool does the same thing and only this thing — a socket
			// timeout, never queue depth (net.rptools.clientserver).
			wctx, wcancel := context.WithTimeout(ctx, s.writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, b)
			wcancel()
			if err != nil {
				return
			}
		}
	}()

	// closing is set (by shutdown, below) immediately before it cancels the
	// subscription as part of a normal, intentional teardown. The pump
	// goroutine checks it once `events` closes so it can tell that apart from
	// the store closing `events` UNILATERALLY — which now means this
	// connection made no progress for its whole no-progress budget, not that
	// it briefly fell behind. See the force-close below the loop.
	// The catch-up head goes out FIRST, before any backlog, so a client knows
	// what it is waiting for before it starts receiving it.
	//
	// Without it a client could not tell catch-up from live: this pump feeds
	// backlog and live broadcast down one channel with no boundary, so
	// `vtt state dump` stopped after 300ms of quiet and called that caught up.
	// A slow moment mid-replay then produced a silently TRUNCATED snapshot.
	// Sent unconditionally, including head 0 for an empty log, so "no frame
	// yet" never has to be interpreted.
	b, err := encodeFrame(&vttv1.ServerFrame{
		Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{HeadSequence: catchUpHead}},
	})
	if err != nil {
		// Fail closed, exactly like the subscribe failure above. Serving a
		// connection that can never announce its head is worse than refusing
		// it: harness.Client.CatchUpHead waits on its context, and `vtt state
		// dump` hands it main.go's signal context, which has NO deadline — so
		// the caller hangs until Ctrl-C instead of getting an error. Tear the
		// writer down first (nothing else has started yet) so it cannot
		// outlive the connection.
		cancel()
		close(outCh)
		<-writerDone
		_ = conn.Close(websocket.StatusInternalError, "gateway: encode catch-up head failed")
		return
	}
	select {
	case outCh <- b:
	case <-ctx.Done():
	}

	// Presence joins AFTER the catch-up head and BEFORE the snapshot, so the
	// joining client sees itself in its own snapshot (spec §4: a picture of
	// the table, not of everyone else).
	//
	// Deregistration hangs off serve returning, which is what makes BOTH
	// teardown paths one path: a clean quit and a client force-closed by the
	// writer's deadline each unwind through here. The second is the one that
	// gets forgotten, and it is the one that matters — a wedged client that
	// never says goodbye would otherwise sit in the table's list forever.
	pc := &presenceConn{participantID: p.ID, displayName: p.Name, out: outCh}
	// The snapshot is built AND enqueued inside join's critical section, so no
	// delta can slip between this connection being registered and the snapshot
	// that describes the table it joined. Enqueued after, a DISCONNECTED could
	// overtake it and the joiner would then apply a snapshot still listing the
	// participant it was just told had left — a ghost, permanently, on a
	// client that applies snapshot-then-deltas.
	firstConnection := s.presence.joinAndSend(pc, func(present []*vttv1.PresenceChanged) []byte {
		b, err := encodeFrame(&vttv1.ServerFrame{
			Frame: &vttv1.ServerFrame_PresenceSnapshot{
				PresenceSnapshot: &vttv1.PresenceSnapshot{Present: present},
			},
		})
		if err != nil {
			return nil
		}
		return b
	})

	leavePresence := func() {
		if last := s.presence.leave(pc); last {
			s.announcePresence(pc, vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED)
		}
	}
	defer leavePresence()

	// Only the participant's FIRST connection is an arrival. A second device
	// must not announce someone who is already at the table.
	if firstConnection {
		s.announcePresence(pc, vttv1.PresenceState_PRESENCE_STATE_CONNECTED)
	}

	var closing atomic.Bool

	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)

		for env := range events {
			// Marshaled per connection, deliberately: each pump encodes
			// straight off its own subscription channel with no shared
			// cache. At table scale (a handful of participants, not
			// thousands of fan-out sockets) a few extra protojson.Marshal
			// calls per event is cheap, and it keeps this goroutine
			// entirely stateless — no retained pointers, nothing to evict,
			// nothing that can leak. Revisit with a real broadcast hub
			// (marshal once, shared bytes) only if client count per
			// campaign ever grows past table scale (say, >10).
			b, err := EncodeFrame(&vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: env}})
			if err != nil {
				continue // a marshal failure here is a server bug, not this client's fault
			}
			select {
			case outCh <- b:
			case <-writerDone:
				return
			}
		}
		// `events` is closed. If shutdown() didn't do it, the store dropped
		// this subscription — this connection will never receive another
		// broadcast, but the command loop below is still blocked in
		// conn.Read, unaware anything happened, and would otherwise sit there
		// looking alive while broadcasts are silently dead forever. Force the
		// connection closed so that Read errors out and drives the normal
		// shutdown() path.
		//
		// Reached when the pump was IDLE at the close — a client that kept up
		// until its subscription ended. A WEDGED one exits through the writer
		// path instead, and needs nothing here: coder/websocket fails the
		// connection when a write errors, so conn.Read below returns and
		// serve unwinds on its own. Verified by injection, not assumed.
		if !closing.Load() {
			_ = conn.CloseNow()
		}
	}()

	// shutdown tears the connection's helper goroutines down in dependency
	// order: mark this as an intentional close (so the pump's post-loop
	// check above is a no-op), stop the subscription (closes `events`),
	// wait for the pump to drain it, THEN close outCh (safe — the pump is
	// guaranteed done, so nothing sends to outCh after this point), and
	// wait for the writer to drain outCh and exit. Safe to call after the
	// pump has already force-closed the connection on overflow: cancel,
	// CloseNow, and channel-close are all idempotent here.
	shutdown := func() {
		// Presence FIRST, before close(outCh). leave takes the registry lock,
		// and broadcast holds that same lock while it sends — so once leave
		// returns, no broadcast can still be holding this connection, and
		// none can acquire it again. Without this ordering the registry keeps
		// handing frames to a channel this function has already closed:
		// "send on closed channel", raised inside a teardown that is often
		// already unwinding, and caught only by net/http's handler recover.
		//
		// The consequence was worse than the crash. Map iteration order is
		// random, so a panic mid-broadcast delivers the departure to SOME
		// connections and never to the rest — a permanent ghost at the table,
		// which is the exact failure presence exists to prevent, arriving
		// silently. Found by review under -race; `task check` does not run
		// the gateway with -race and did not see it.
		//
		// The deferred leave below stays, as an idempotent backstop for the
		// return paths that never reach shutdown.
		leavePresence()
		closing.Store(true)
		cancel()
		<-pumpDone
		close(outCh)
		<-writerDone
	}

	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			shutdown()
			return
		}

		cmd, err := DecodeCommand(raw)
		if err != nil {
			// Malformed frame: close only THIS connection (binding
			// contract) — every other connection is untouched.
			shutdown()
			_ = conn.Close(websocket.StatusPolicyViolation, "gateway: malformed frame")
			return
		}

		result := s.handleCommand(p, cmd)
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

// handleCommand runs the authorize → convert → persist pipeline for one
// inbound ClientCommand (spec §3): authz/validation failures produce an
// ok=false CommandResult and leave the connection open; only a persisted
// event/marker produces ok=true. It never itself closes the connection or
// writes to the wire — the caller (serve) owns transport.
func (s *Server) handleCommand(p *identity.Participant, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	requestID := cmd.GetRequestId()

	st := s.campaign.State()
	if st == nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: "gateway: campaign unavailable"}
	}

	if err := Authorize(p, cmd, st); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// use_ability/load_adventure do not become a single Envelope via ToEvent
	// (they each produce a whole ordered batch instead — ruleset.go/
	// adventure.go); every other command, including remove_condition, still
	// flows through the plain ToEvent -> campaign.Append path below.
	if ua, ok := cmd.GetCommand().(*vttv1.ClientCommand_UseAbility); ok {
		return s.handleUseAbility(requestID, ua.UseAbility, st, p)
	}
	if la, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadAdventure); ok {
		return s.handleLoadAdventure(requestID, la.LoadAdventure, st, p)
	}

	env, err := ToEvent(cmd, p)
	if err != nil {
		var rr *RetractionRange
		if errors.As(err, &rr) {
			return s.handleRetraction(requestID, rr, p)
		}
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// Controller decision (binding, Task 4 flagged concern): backfill
	// TokenMoved.SceneId/From from the state already fetched for Authorize
	// above — the token's CURRENT scene/position, i.e. where it is moving
	// FROM — so the permanent log records that, not just the destination.
	// engine.Apply never reads these fields for TokenMoved (it only reads
	// To — see internal/engine/apply.go), so nothing downstream of Append
	// would supply them; this is the one place in the pipeline that still
	// has both the pre-move state snapshot and the about-to-be-appended
	// envelope in hand.
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

// handleRetraction persists rr via campaign.Undo, which owns constructing
// the EventsRetracted marker itself (ToEvent deliberately never builds one
// — see ErrIsRetraction's doc comment). A fresh marker event id is minted
// here the same way ToEvent mints one for every other event. p is the
// issuing participant: campaign has no identity concept of its own (see
// Undo's doc comment), so the gateway — the one place that has both p and
// the retraction — supplies actor_role/participant_id attribution the same
// way ToEvent does for every other command (spec §4).
func (s *Server) handleRetraction(requestID string, rr *RetractionRange, p *identity.Participant) *vttv1.CommandResult {
	id, err := newEventID()
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	seq, err := s.campaign.Undo(rr.FromSequence, rr.ToSequence, rr.Reason, id, string(p.Role), p.ID)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	// campaign.Undo now returns the marker's own sequence (P6 Task 4
	// pre-step, controller decision — closes the P4 carry-forward), so the
	// result carries it the same way Append's sequence does for every other
	// command; it also remains visible on the broadcast Envelope frame
	// itself, to every connection including this one.
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: seq}
}

// announcePresence tells everyone EXCEPT pc that pc's participant arrived or
// left.
//
// An encode failure is dropped rather than escalated: presence is soft state,
// every client is re-synced by the snapshot it gets on connect, and tearing a
// healthy connection down because someone else's status frame would not
// marshal would turn a cosmetic fault into an outage. That is the opposite of
// the catch-up head, which fails the connection closed — a client that cannot
// learn where catch-up ends cannot function, and one that misses a presence
// blip can.
func (s *Server) announcePresence(pc *presenceConn, state vttv1.PresenceState) {
	b, err := encodeFrame(&vttv1.ServerFrame{
		Frame: &vttv1.ServerFrame_PresenceChanged{
			PresenceChanged: &vttv1.PresenceChanged{
				ParticipantId: pc.participantID,
				DisplayName:   pc.displayName,
				State:         state,
			},
		},
	})
	if err != nil {
		return
	}
	s.presence.broadcast(pc, b)
}
