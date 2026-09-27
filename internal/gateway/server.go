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
	// Held separately from adventures because cmd/vtt owns the filesystem
	// (ADR-008): it reads the guides and hands them over, so an unreadable
	// one fails loudly at boot rather than becoming a 500 mid-session. The
	// rule has one deliberate exception since 2026-09-01-create-scene-leaves
	// Task 6 — mapsDir below — and a guide is not it; see mapByID (map.go)
	// and WithAdventureGuides (metadata.go) for the whole reasoning.
	adventureGuides map[string]string

	// static is the built web client, served at / when non-nil. Optional:
	// `vtt serve` without a bundle still serves the API, and a browser gets
	// an honest 404 rather than a panic. Set via WithStatic, boot time only.
	static fs.FS

	// maps is OPTIONAL server config (maps-as-geometry Task 7, spec §4.3/
	// §4.4): nil/empty is today's behavior for a campaign whose maps/ is
	// absent, or which has no maps installed yet (2026-09-01-create-scene-
	// leaves Task 5 — maps come from the campaign directory itself, not a
	// --maps-dir flag) — GET /api/maps answers 200 with an empty list, the
	// same "empty is not an error" posture handleAdventures already gives
	// (spec §5). Keyed by each map's own declared id (Map.ID), not any
	// directory name — cmd/vtt's loadMapsDir refuses a collision there
	// before either map ever reaches here.
	//
	// NO LONGER BOOT TIME ONLY as of 2026-09-01-create-scene-leaves Task 6:
	// WithMaps still fills it before the server serves anything, but a map
	// installed into the campaign's maps/ during a session joins it on its
	// first successful load_map (map.go's mapByID). Every access ONCE THE
	// SERVER IS SERVING therefore goes through mapsMu below; WithMaps'
	// own write does not, and its doc comment says why.
	maps map[string]*mapdef.Map

	// mapsDir is the campaign's own maps/ directory, set via WithMapsDir:
	// where map.go's mapByID looks when the set above does not hold an id
	// (2026-09-01-create-scene-leaves design spec §5). Empty means this
	// server cannot look anything up on disk, which is every pre-Task-6
	// behaviour unchanged — see mapByID's own doc comment.
	mapsDir string

	// artDir is the campaign's flat art/ directory, set via WithArtDir: the
	// root every override and every object art name resolves against, read
	// when a map is loaded rather than once at boot
	// (2026-09-02-art-is-a-flat-library design spec §3.6). Empty means no art
	// resolves, which is not an error — every override then degrades to its
	// base tile and warns (that spec's §4), and the map still loads.
	//
	// Read without a lock for the same reason mapsDir is: a configuration
	// call sets it before the server serves anything. NOTHING IS CACHED behind
	// it — the directory is read as it is at the moment it is asked, which is
	// what makes art installed or overwritten during a session take effect on
	// the next load_map with no restart, and what a boot-time pack load could
	// never do.
	artDir string

	// cellPx is how many pixels one grid square of this campaign's art
	// occupies, reported on GET /api/maps and set via WithCellPx. New fills it
	// with DefaultCellPx, so a Server nobody configured still reports the
	// documented number rather than a zero that would read as "no grid".
	//
	// Read without a lock for the same reason mapsDir and artDir are: a
	// configuration call sets it before the server serves anything.
	cellPx int32

	// mapsMu guards maps, and only maps. mapsDir, artDir and cellPx are set
	// once by a With* call before the server serves anything and never written
	// again, so they are read without it (each says so at its own field above).
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

// WithStatic serves fsys, the built web client, at /; without it the server is
// API-only. Take an fs.FS, not a path: cmd/vtt embeds the bundle, and go:embed
// cannot cross package directories (SPEC-011).
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

// WithMaps configures s to answer GET /api/maps from m, keyed by each map's
// own declared id (maps-as-geometry Task 7). m is expected already fully
// loaded and validated (cmd/vtt's loadMapsDir, via mapdef.LoadInstalled — fail
// loud at boot, spec §4.4); this method does no I/O and no validation of its
// own, mirroring WithAdventures.
//
// IT TOOK A SECOND ARGUMENT, a pack set, until 2026-09-02-art-is-a-flat-library
// Task 7. That set enriched every /api/maps entry with the map's own declared
// pack until Task 5 deleted mapdef.Map.Pack and left nothing to key the lookup
// by; Task 7 took the set, the fs.FS beside it (WithPackFiles) and mapdef.Pack
// itself. Art is read from a directory at map-load time now — see WithArtDir.
//
// Returns s for call-site chaining; mutates s in place WITHOUT taking mapsMu,
// so it is not safe to call concurrently with s already serving traffic — the
// map set gains entries during a session (WithMapsDir below), but never
// through this method.
func (s *Server) WithMaps(m map[string]*mapdef.Map) *Server {
	s.maps = m
	return s
}

// WithMapsDir tells s where this campaign keeps its maps, so that a map
// installed while the server is running is loadable without a restart
// (2026-09-01-create-scene-leaves design spec §4/§5, and the sub-project's
// own reason to exist: create_scene left the platform, and what replaces
// improvisation is authoring a map outside the platform, writing it into
// the campaign's maps/, and loading it). dir is the campaign's maps/
// directory itself; it need not exist — a brand-new campaign has no maps
// directory at all, and one that appears later is found on the next lookup,
// because the probe reads the directory as it is at the moment it is asked
// rather than holding any state about it.
//
// A PATH rather than an fs.FS: the point of the probe
// is that it runs mapdef.LoadInstalled, the SAME function cmd/vtt's boot
// walk runs (design spec §12 — a map that boots cleanly must not be refused
// on reload), and that function works in ordinary paths because the boot
// walk does. An fs.FS here would have needed a second, bytes-shaped entry
// into mapdef and a second error vocabulary, which is the divergence itself
// wearing the costume of a safety measure. The escape an fs.FS would have
// closed is closed instead where the untrusted id enters:
// mapdef.LoadInstalled refuses any id that is not one plain filename,
// before it joins anything.
//
// Boot time only as a CONFIGURATION call, like every other With* method:
// mutates s in place, so it is not safe to call concurrently with s already
// serving traffic.
func (s *Server) WithMapsDir(dir string) *Server {
	s.mapsDir = dir
	return s
}

// WithArtDir tells s where this campaign keeps its art, so that every map it
// loads resolves overrides and object art against that one flat directory
// (2026-09-02-art-is-a-flat-library design spec §3). dir need not exist: a
// campaign that has installed no art is ordinary, its maps still load, and
// each unresolved reference costs one warning rather than the map (§4).
//
// A PATH rather than an fs.FS, for the reason WithMapsDir gives above and one
// more: internal/artlib opens every file through os.OpenRoot, so the symlink
// escape an fs.FS would be reached for is already closed underneath, at the
// syscall rather than at a name check.
//
// NOTHING IS READ HERE AND NOTHING IS CACHED. That is the whole point: the
// boot-order defect this sub-project removes existed because art was loaded
// once, at startup, in an order another directory's loading depended on.
// There is no boot-time art load to get wrong any more.
//
// This method landed in Task 3 rather than Task 4, where the plan scheduled
// it: Task 3 moved art resolution off the pack Task 7 later deleted, and
// without somewhere for
// the gateway to resolve FROM, map_test.go's assertion that an override's art
// reaches the wire had to be weakened for one task and remembered back. A
// weakened assertion that nobody restores fails silently; an interface that
// arrives one task early fails loudly, at the next implementer's first
// compile. Task 4 then did what it owned: cmd/vtt's composeServer calls this
// with campaignPath/art, unconditionally and outside its maps guard, having
// first run artlib.Validate over that directory and reported (not refused) what
// it found.
//
// Boot time only as a CONFIGURATION call, like every other With* method:
// mutates s in place, so it is not safe to call concurrently with s already
// serving traffic.
func (s *Server) WithArtDir(dir string) *Server {
	s.artDir = dir
	return s
}

// WithCellPx tells s how many pixels one grid square of this campaign's art
// occupies — campaign.json's cell_px, read by cmd/vtt (ADR-008: cmd owns the
// filesystem) through internal/campaigncfg and handed over as a number, which
// is why this package takes an int32 and never a path.
//
// It was a PACK field until 2026-09-02-art-is-a-flat-library, served to the
// client as pack.cellPx on every /api/maps entry. Design spec §6 rehomes it to
// the campaign because a grid is uniform: art pieces at differing native
// resolutions on the same board is a rendering problem, not a capability, and
// one number per campaign says so.
//
// Boot time only as a CONFIGURATION call, like every other With* method:
// mutates s in place, so it is not safe to call concurrently with s already
// serving traffic.
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

// describeBlockage turns engine.State.Blocked's reason into a clause a
// player reading a CommandResult.Error can act on. This is the FIRST place a
// Blocked answer reaches a human rather than a source reader or a test
// failure (task-5 review finding) — terrain.go's own reasons are written for
// the latter, and read inconsistently as a result: "a wall" and "a closed
// door" already pass for plain English, but "scenery: <kind>" is a
// colon-joined debug tag, and `unknown scene %q` is a raw Go-quoted literal
// naming an internal scene id nobody at the table chose. Only those two get
// rewritten; the rest pass through unchanged rather than being forced
// through a "sentence-ifier" that would just be paraphrasing prose that
// already reads fine. engine/terrain.go stays untouched (Task 6 does not
// touch internal/engine) — this list has to be kept in sync by hand if
// Blocked's reasons ever change, which is the cost of the fix living on the
// consuming side instead.
//
// THE KIND IS BOUNDED HERE, and it is the only author-written string this
// function RENDERS — the unknown-scene arm receives one too and discards it.
// A map file's objects[].kind is free text: mapdef.Load checks that object's
// footprint and its art and never looks at kind, so terrain.go's
// "scenery: " + o.Kind carried whatever the file said, one-for-one, into what
// a blocked player reads.
//
// Measured 2026-09-10 before the clip, and the two pairs are DIFFERENT SHAPES,
// which an earlier draft of this comment ran together: as a Blocked reason, a
// 4-byte kind gave 32 bytes and a 5000-byte one gave 5028. Over the wire, where
// answerCommand prefixes "gateway: cannot move there — ", the test's own two
// shapes — 100 and 5000 — gave 159 and 5059. A 4-byte kind over the wire is 63,
// and a reader re-deriving the sentence as written would get that and conclude
// the numbers were invented.
//
// It is an ERROR rather than a warning, which is why it was not bounded
// alongside the art warnings and why the reasoning belongs here rather than
// being inferred: one move_token yields one refusal, so it never
// scene-qualifies and never accumulates the way adventure.Compile's warnings
// do. The cost was a player bumping into a crate and reading a wall of text,
// not a socket closing.
//
// THE PASSTHROUGH ARM IS NOT BOUNDED and does not need to be TODAY: every
// other reason terrain.go returns is a literal it wrote itself ("a wall", "a
// closed door", "outside the grid"), and the unknown-scene arm is rewritten to
// a constant above.
// A reason that ever interpolates something a map file wrote needs the same
// treatment, and will not get it by sitting in that arm.
// RULE 9, ANSWERED: MapTool has nothing to borrow here, and the reason is
// structural rather than an oversight. Its movement blocking is geometric —
// ZoneWalker and VBL decide passability and the client simply will not path
// into the cell — so no server-to-client message names the obstruction, and
// there is no sentence to bound. It could not have one worth copying anyway:
// every MapTool client receives the whole campaign, which is the distribution
// model CLAUDE.md says explicitly not to take, so it has no per-message budget
// to protect in the first place.
func describeBlockage(why string) string {
	if kind, ok := strings.CutPrefix(why, "scenery: "); ok {
		return "something (a " + artlib.Clip(kind, artlib.MaxFragment) + ") is in the way"
	}
	if strings.HasPrefix(why, "unknown scene ") {
		return "that destination is not part of any scene this table has created"
	}
	return why
}

// answerCommand produces the CommandResult for one decoded command.
//
// IT WRITES NO ENVELOPE, and that restriction is the whole of the C1 fix: this
// runs on the COMMAND goroutine, and the pump is the only producer of a
// connection's envelopes. A perch that needs frames sent asks the pump for them
// through `perches` rather than enqueueing its own — see handleSetViewpoint.
//
// lookupErr is the identity re-resolution's OPERATIONAL failure, never a
// revocation: serve has already dealt with that one and does not reach here.
// See its comment on why a busy database must not be read as "your credential
// is no longer valid".
//
// A separate function rather than a switch inline in serve, and the reason is
// mechanical rather than aesthetic: written inline, the perch branch put serve
// at gocyclo 31 against a limit of 30 and the lint gate refused it (MEASURED —
// that is the failure this extraction answers, not an estimate of what it
// would have been). Raising the threshold would have been weakening a gate to
// pass it (CLAUDE.md rule 2); the split it forced runs along a seam that was
// already there.
func (s *Server) answerCommand(p *identity.Participant, lookupErr error, cmd *vttv1.ClientCommand,
	perches *perchBox) *vttv1.CommandResult {
	switch sv, isPerch := cmd.GetCommand().(*vttv1.ClientCommand_SetViewpoint); {
	case lookupErr != nil:
		// Refuse the command and keep the connection, exactly as authorize
		// already does for a campaign that cannot answer. Still fail-closed
		// where it counts: nothing is authorized while we cannot say who is
		// asking.
		return &vttv1.CommandResult{
			RequestId: cmd.GetRequestId(),
			Ok:        false,
			Error:     "gateway: identity unavailable",
		}

	case isPerch:
		// THE ONE COMMAND ANSWERED OUTSIDE handleCommand, and the reason is the
		// one handleCommand's own doc comment gives for owning no transport: a
		// perch appends NOTHING to the log and changes nothing anyone else can
		// observe. Its whole effect is on THIS connection's seat and THIS
		// connection's wire. handleCommand runs authorize → convert → persist,
		// and a perch is none of those three.
		return s.handleSetViewpoint(p, cmd, sv.SetViewpoint, perches)

	default:
		return s.handleCommand(p, cmd)
	}
}

// authorize is the preamble EVERY inbound command shares, in one place because
// it has two callers: fetch the world the command is judged against, and ask
// the one authorization function (spec §4). It returns either that state, or
// the CommandResult that refuses the command — never both, and never neither.
//
// The nil-state arm is not a formality. campaign.State() answers nil for a
// campaign that cannot be read, and authorizing against no world at all would
// be deciding who may do what with nothing to decide it from; fail closed
// (spec §4.4) and tell the caller so, keeping the connection.
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

// handleSetViewpoint authorizes a perch and HANDS IT TO THE PUMP.
//
// IT APPENDS NOTHING, and that is a ruling rather than an omission — the same
// one handleJoinDoor's doc comment makes for the shared door. Patrik: "we do
// not need to log anything about what/where the spectator sees." The log is
// the campaign's history, and where a watcher points their camera is not a
// fact about the campaign; it is a view preference, like zoom. Logged, it
// would replay forever and add story-panel noise — and the log only goes
// forward, so it would be there for good.
//
// The cost of that ruling is the perch not surviving a reconnect (spec
// §3.1.1), because it lives on the connection like the catch-up point does.
// The client re-sends it after redialling.
//
// IT DOES NOT APPLY THE PERCH ITSELF, and that division is C1's fix rather
// than a preference. This function runs on the command goroutine; the seat's
// projector and this connection's wire belong to the PUMP. When both goroutines
// enqueued, two batches computed in one order reached the socket in the other:
// a pump frame landed INSIDE a perch batch, and a TokenMoved computed before a
// TokenHidden was delivered after it, so the watcher's own stream stopped
// folding — permanently, on the browser client. Handing the shoulder over and
// letting the pump do both the projecting and the sending makes that
// unrepresentable rather than unlikely: one goroutine mutates the projector's
// memory and emits the frames that describe the mutation, so emission order IS
// mutation order.
//
// WHAT ok MEANS HERE, precisely, because the honest answer is narrower than the
// usual one: the shoulder has been RECORDED and this connection's pump will
// move to it. Not "the frames are on the wire" — they are computed a moment
// later, by the pump. Handing over never blocks (perchBox.set), so a spectator
// hopping quickly cannot stall their own command loop behind a projection, and
// a hop that is superseded before the pump reaches it is simply skipped.
//
// The refusal path is Authorize's, which is where MayPerch enforces the one
// rule this command has: a perch may only target a PARTY MEMBER — what the
// actor IS, never who currently controls it (visibility spec §5.1).
func (s *Server) handleSetViewpoint(p *identity.Participant, cmd *vttv1.ClientCommand,
	req *vttv1.SetViewpoint, perches *perchBox) *vttv1.CommandResult {
	if _, refusal := s.authorize(p, cmd); refusal != nil {
		return refusal
	}
	perches.set(req.GetActorId())
	return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true}
}

// handleCommand runs the authorize → convert → persist pipeline for one
// inbound ClientCommand (spec §3): authz/validation failures produce an
// ok=false CommandResult and leave the connection open; only a persisted
// event/marker produces ok=true. It never itself closes the connection or
// writes to the wire — the caller (serve) owns transport.
//
// set_viewpoint never reaches here: it persists nothing and its whole effect
// is on one connection, so serve answers it directly (handleSetViewpoint).
func (s *Server) handleCommand(p *identity.Participant, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	requestID := cmd.GetRequestId()

	st, refusal := s.authorize(p, cmd)
	if refusal != nil {
		return refusal
	}

	// The map constrains PLAYERS; the DM and the agent author the world and
	// are free of it (maps-as-geometry spec §6, Patrik: "hard for players,
	// free for DM"). Staging a creature inside stone is a legitimate thing
	// for a DM to do.
	//
	// Checked HERE, not in engine.Apply: Apply is the FOLD — by the time an
	// event reaches it the move is already history, and history is not the
	// place to say no. This is the seam where a command is still a request,
	// the last point a refusal can mean "you may not" rather than "this
	// never happened".
	if p.Role == identity.RolePlayer {
		if mt, ok := cmd.GetCommand().(*vttv1.ClientCommand_MoveToken); ok {
			if tok, known := st.Tokens[mt.MoveToken.GetTokenId()]; known {
				to := mt.MoveToken.GetTo()
				// YOU MAY ONLY MOVE WHERE YOU CAN SEE (visibility spec exit
				// criterion 7: the goblin's square "cannot be targeted by a
				// player who cannot see it"). This is the second half of
				// session zero. Filtering the wire stops a player LOOKING at a
				// hidden creature; without this they could still land on it,
				// because move_token validates a destination and never a path
				// — the whole grid was one command away.
				//
				// PHRASED AS "CAN YOU SEE THE SQUARE", NOT "IS SOMETHING
				// STANDING THERE", and the difference is the whole design. A
				// refusal that depends on the occupant is an ORACLE: a player
				// could sweep the map with move commands and read the hidden
				// creatures off the refusals, which is session zero again with
				// more typing. This refusal depends on nothing the player does
				// not already hold — their own visible set is exactly what
				// SceneSeen just told them — so it leaks nothing, in the
				// strong sense that they can compute it themselves.
				//
				// FIRST, AND THE ORDER IS THE WHOLE POINT. It ran second for
				// one commit, so that every pre-existing refusal kept its exact
				// wording, and that was a leak of its own: engine.Blocked
				// answers for ANY square in the scene, sight-independent — it
				// was written when a player received every tile, so it gave
				// nothing away. Against a REDACTED board it hands back walls,
				// closed doors and scenery kinds ("something (a crate) is in
				// the way") for terrain SceneSeen has never sent, one
				// move_token at a time, defeating spec §4.2 through an error
				// string. TestAPlayerCannotProbeTheDarkWithMoveCommands sweeps
				// unseen squares of four different kinds and requires the
				// refusals to be byte-identical.
				//
				// The cost is real and deliberate: a square out of line of
				// sight cannot be walked onto even when it is remembered
				// terrain (spec §3.2), so a player cannot round a corner in
				// one command. Fail closed (spec §4.4). And this is the
				// PLAYER's branch only — "hard for players, free for DM"
				// (maps-as-geometry spec §6) governs sight exactly as it
				// governs stone, so the DM's refusals are untouched, wording
				// and ordering both.
				if !canSee(viewerFor(p), st, tok.SceneID, to) {
					return &vttv1.CommandResult{
						RequestId: requestID, Ok: false,
						Error: "gateway: cannot move there — you cannot see that square",
					}
				}
				// AND ONLY THEN WHAT IS ON IT. Reached only for a square this
				// player can see, which is a square whose terrain they have
				// already been sent — so naming it tells them nothing new and
				// keeps the refusal useful.
				if blocked, why := st.Blocked(tok.SceneID, to.GetX(), to.GetY()); blocked {
					return &vttv1.CommandResult{
						RequestId: requestID, Ok: false,
						Error: "gateway: cannot move there — " + describeBlockage(why),
					}
				}
			}
		}
	}

	// grant_actor_control's kind gets the SAME seam and the SAME reasoning as
	// the movement check above, and for the second time the same argument:
	// engine.Apply is the fold, and by the time an event reaches it the grant
	// is already history — history is not the place to say no. (This paragraph
	// named create_scene's terrain check as the seam directly above it until
	// 2026-09-02, when create_scene left the platform; add_actor's own check
	// below still makes three call sites of the pattern, not two.)
	//
	// It is HERE rather than in Authorize because it is not a rule about who:
	// the DM and the agent are both entitled to hand a character over, and
	// neither may do it without saying what they are handing over. And it is
	// here rather than in ToEvent because ToEvent's own completeness gate
	// (TestEveryClientCommandConverts) requires every command to convert from
	// an EMPTY payload — that gate exists because grant_actor_control once
	// shipped advertised and dead, so narrowing it for this command in
	// particular would be trading one silent hole for another.
	if g, ok := cmd.GetCommand().(*vttv1.ClientCommand_GrantActorControl); ok {
		if err := validateGrantActorControl(g.GrantActorControl); err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
	}

	// add_actor gets the SAME seam and, for the third time, the same argument:
	// engine.Apply is the fold, and by the time an ActorAdded reaches it the
	// actor is already history.
	//
	// TWO RULES BEHIND ONE CALL, and they answer the fold question OPPOSITELY.
	// The CONTROLLER rule also lives in the fold, which refuses the same shape
	// outright — there is no history to protect, so it can — and this seam only
	// adds the answer: a refusal naming grant_actor_control, before anything is
	// written, instead of a poisoned append. The KIND rule (actor-kind Task 7)
	// lives HERE AND NOWHERE ELSE, because an absent kind is a legal state on a
	// recorded event ("not a party member") and a fold that refused it would be
	// refusing something the contract defines. See validateAddActor, which
	// argues both at length.
	if aa, ok := cmd.GetCommand().(*vttv1.ClientCommand_AddActor); ok {
		if err := validateAddActor(aa.AddActor); err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
	}

	// use_ability/load_adventure/load_map/remove_actor do not become a single
	// Envelope via ToEvent (they each produce a whole ordered batch instead —
	// ruleset.go/adventure.go/map.go and handleRemoveActor in this file);
	// every other command, including remove_condition
	// and remove_token (retraction-leaves Task 8 — no board-position seam
	// needed here, since it is DM/agent only and engine.Apply's own
	// unknown-token guard is the entire validation story), still flows
	// through the plain ToEvent -> campaign.Append path below.
	if ua, ok := cmd.GetCommand().(*vttv1.ClientCommand_UseAbility); ok {
		return s.handleUseAbility(requestID, ua.UseAbility, st, p)
	}
	if la, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadAdventure); ok {
		return s.handleLoadAdventure(requestID, la.LoadAdventure, st, p)
	}
	if lm, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadMap); ok {
		return s.handleLoadMap(requestID, lm.LoadMap, p)
	}
	// remove_actor is the fourth batch command and the only one in this arc:
	// it emits a TokenRemoved per token of the actor and then the
	// ActorRemoved, as ONE ordered batch (handleRemoveActor, in this file).
	if ra, ok := cmd.GetCommand().(*vttv1.ClientCommand_RemoveActor); ok {
		return s.handleRemoveActor(requestID, ra.RemoveActor, st, p)
	}
	// promote_participant produces NO EVENT AT ALL, unlike the two above which
	// produce a batch. A role lives in participants.role beside the token —
	// one source of truth, never in the log (joining-a-table spec §3.1). It is
	// the only command that changes identity rather than campaign state, which
	// is why ToEvent's completeness gate names it on its allowlist.
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
