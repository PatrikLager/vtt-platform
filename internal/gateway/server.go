package gateway

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// writeUntilFlushed hands every frame on out to write, in order, until write
// fails or flush is closed — and on flush it DRAINS what is already queued
// before returning.
//
// A free function with an injected write, rather than the closure it was, so
// the drain can be tested at all. It cannot be observed through the socket:
// flush is only ever closed while the connection is being torn down, so on the
// path where the client has gone every write fails anyway, and on the path
// where it has not, the frames are usually already in the OS buffer. Behaviour
// nobody can see is behaviour nobody maintains — this was the one assertion in
// #47 that fault injection found untested, and deleting the drain to make that
// go away would have silently dropped what `for b := range out` gave for free.
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
	// Held separately from adventures because the gateway does no file I/O:
	// cmd/vtt reads the guides and hands them over (ADR-008).
	adventureGuides map[string]string

	// static is the built web client, served at / when non-nil. Optional:
	// `vtt serve` without a bundle still serves the API, and a browser gets
	// an honest 404 rather than a panic. Set via WithStatic, boot time only.
	static fs.FS

	// maps is OPTIONAL server config (maps-as-geometry Task 7, spec §4.3/
	// §4.4): nil/empty is today's behavior for a server with no
	// --maps-dir — GET /api/maps answers 200 with an empty list, the same
	// "empty is not an error" posture handleAdventures already gives (spec
	// §5). Set via WithMaps, BOOT TIME ONLY (mirrors WithAdventures — see
	// its own doc comment for why), keyed by each map's own declared id
	// (Map.ID), not any directory name — cmd/vtt's loadMapsDir refuses a
	// collision there before either map ever reaches here.
	maps map[string]*mapdef.Map

	// packs mirrors maps' own keying but for packs (Pack.ID, set together
	// via WithMaps): used to enrich GET /api/maps with each map's pack name
	// and cell size, so a client can render at the right scale without a
	// second request. loadMapsDir refuses two packs sharing an id before
	// either reaches here, so this map's keys already match packFS's
	// key-for-key.
	packs map[string]*mapdef.Pack

	// packFS is OPTIONAL server config, boot time only, set via
	// WithPackFiles: one fs.FS PER PACK, each rooted AT that pack's own
	// directory (cmd/vtt builds them with os.OpenRoot(dir).FS() over an
	// operator-installed --maps-dir — NOT os.DirFS; see WithPackFiles' own
	// doc comment for why that distinction is load-bearing, not stylistic).
	// GET /api/packs/{pack}/{file} (metadata.go's handlePackFile) serves
	// straight out of the matching entry. A SEPARATE field from packs
	// rather than folding an fs.FS onto *mapdef.Pack itself: a Pack value
	// carries no notion of "where it came from" (mapdef is a pure parser —
	// see its own package doc — and reusable independent of any one
	// directory), so the directory-rooted serving capability has to live
	// here, at the layer that actually owns the filesystem boundary
	// (ADR-008).
	packFS map[string]fs.FS
}

// New constructs a Server over an already-open campaign and identity DB.
// The caller owns both handles' lifecycle (Close them after the Server is
// done serving). No ruleset is loaded — use_ability commands are rejected
// with a clean "no ruleset loaded" error until WithRuleset is called.
func New(c *campaign.Campaign, ids *identity.DB) *Server {
	return &Server{
		campaign: c, ids: ids,
		buffer: gatewayBuffer, noProgress: gatewayNoProgress, writeTimeout: gatewayNoProgress,
		presence:    newPresenceRegistry(),
		encodeFrame: EncodeFrame,
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

// WithMaps configures s to answer GET /api/maps from m/packs, keyed by each
// map's/pack's own declared id (maps-as-geometry Task 7). Both are expected
// already fully loaded and validated (cmd/vtt's loadMapsDir: mapdef.Load,
// mapdef.LoadPack, and a boot-time dry-run mapdef.Compile per map — fail
// loud at boot, spec §4.4); this method does no I/O and no validation of
// its own, mirroring WithAdventures. Returns s for call-site chaining;
// mutates s in place, so it is not safe to call concurrently with s already
// serving traffic. Pack file BYTES are a separate concern — see
// WithPackFiles.
func (s *Server) WithMaps(m map[string]*mapdef.Map, packs map[string]*mapdef.Pack) *Server {
	s.maps = m
	s.packs = packs
	return s
}

// WithPackFiles configures s to answer GET /api/packs/{pack}/{file} from
// fsys, keyed by each pack's own declared id — the SAME keys WithMaps'
// packs argument uses, kept as a separate call because it is a genuinely
// different kind of thing (raw filesystem access for byte serving, not a
// parsed Go value) rather than because the two could disagree in practice.
//
// Each fsys entry MUST be rooted AT that pack's own directory via
// os.OpenRoot(dir).FS() (go1.24+) — NOT os.DirFS. This distinction was
// found missing by review and is the whole reason this doc comment exists:
// fs.FS's contract (fs.ValidPath) refuses any NAME containing a ".."
// element, which stops a request like ".../../../etc/passwd", but it says
// nothing about a file that is, on disk, a symlink pointing outside the
// tree — os.DirFS's own doc comment states plainly that "using DirFS does
// not stop the access any more than using os.Open does" in that case, and
// nothing about the NAME "evil.png" would ever look wrong. A community-
// authored pack (spec §4.2's own trust framing: same trust as guide.md, but
// an installed pack is more likely third-party than a hand-authored guide)
// could ship exactly that. os.Root closes it: "Methods on Root will follow
// symbolic links, but symbolic links may not reference a location outside
// the root" (go doc os.Root). Both mechanisms are proven independently in
// internal/gateway/packfile_internal_test.go, because a fix for one says
// nothing about the other. Boot time only, like WithMaps.
func (s *Server) WithPackFiles(fsys map[string]fs.FS) *Server {
	s.packFS = fsys
	return s
}

// Handler returns the http.Handler routing /healthz and /ws (spec §3).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/ws", s.handleWS)
	// Read-only metadata (metadata.go). Method-qualified patterns, so a POST
	// to a read endpoint is a clean 405 rather than a silent success.
	// The shared join link. POST-only and unauthenticated by construction —
	// see join.go for why its refusals are deliberately indistinguishable.
	mux.HandleFunc("POST /join", s.handleJoin)

	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("GET /api/ruleset", s.handleRuleset)
	mux.HandleFunc("GET /api/ruleset/guide", s.handleRulesetGuide)
	mux.HandleFunc("GET /api/join-link", s.handleJoinLink)
	mux.HandleFunc("GET /api/participants", s.handleParticipants)
	mux.HandleFunc("GET /api/adventures", s.handleAdventures)
	mux.HandleFunc("GET /api/adventures/{id}/guide", s.handleAdventureGuide)
	mux.HandleFunc("GET /api/maps", s.handleMaps)
	mux.HandleFunc("GET /api/packs/{pack}/{file}", s.handlePackFile)

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

	// The seat decides what this connection may receive AND where its
	// subscription starts — for a projected seat those are two different
	// numbers, which is the one thing about this arc that is easy to get
	// wrong (see seat's doc comment).
	sub := newSeat(p, after)
	events, unsubscribe, catchUpHead, err := s.campaign.SubscribeWithNoProgressTimeout(sub.subscribeFrom(after), s.buffer, s.noProgress)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "gateway: subscribe failed")
		return
	}

	outCh := make(chan []byte, s.buffer)
	// flush replaces close(outCh) as the writer's stop signal, and outCh is
	// now never closed at all. Presence sends happen OUTSIDE the registry lock
	// (#47), so the old interlock — deregister under r.mu, then close — cannot
	// hold, and no amount of selecting saves a sender from it: closing a
	// channel while a goroutine is parked sending on it panics that goroutine.
	// A frame written after teardown lands in the buffer and is collected with
	// the connection, which costs one garbage slice and removes the whole
	// "send on closed channel" class that 656079f fixed by ordering and that
	// took three attempts to get right.
	flush := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		write := func(b []byte) bool {
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
			return err == nil
		}
		writeUntilFlushed(outCh, flush, write)
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
	b, err := s.encodeFrame(&vttv1.ServerFrame{
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

	// Presence joins AFTER the catch-up head and BEFORE the snapshot, so the
	// joining client sees itself in its own snapshot (spec §4: a picture of
	// the table, not of everyone else).
	//
	// Deregistration hangs off serve returning, which is what makes BOTH
	// teardown paths one path: a clean quit and a client force-closed by the
	// writer's deadline each unwind through here. The second is the one that
	// gets forgotten, and it is the one that matters — a wedged client that
	// never says goodbye would otherwise sit in the table's list forever.
	pc := &presenceConn{
		participantID: p.ID,
		displayName:   p.Name,
		out:           outCh,
		done:          make(chan struct{}),
	}
	// The snapshot is built AND enqueued inside join's critical section, so no
	// delta can slip between this connection being registered and the snapshot
	// that describes the table it joined. Enqueued after, a DISCONNECTED could
	// overtake it and the joiner would then apply a snapshot still listing the
	// participant it was just told had left — a ghost, permanently, on a
	// client that applies snapshot-then-deltas.
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

	// Safe to reach twice — shutdown below, and this defer as a backstop for
	// the returns that never get there. leave() is idempotent and closes
	// pc.done itself, behind its own membership check, so a second call is a
	// no-op rather than a double close.
	leavePresence := func() {
		if last := s.presence.leave(pc); last {
			s.announceDeparture(pc)
		}
	}
	defer leavePresence()

	var closing atomic.Bool

	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)

		for env := range events {
			// RE-RESOLVE HERE TOO, and for a reason the command loop cannot
			// cover. commandRoles has no spectator row anywhere, so a
			// spectator may issue NO command — the lookup down there never
			// fires for one. And every joiner through the shared link arrives
			// as a spectator. So a revoked stranger who found a leaked link
			// kept watching the whole session: the one thing a spectator does
			// is exactly the thing revocation was not reaching.
			//
			// Delivery is where a watcher meets the server, so delivery is
			// where it bites — on the next thing the table would have shown
			// them, not on a timer and not at their next connect.
			//
			// ErrInvalidToken ONLY. An operational failure must not silently
			// drop an event: losing a frame is worse than a moment's delay in
			// removing somebody, and the very next event catches them anyway.
			if _, err := s.ids.Lookup(pc.participantID); errors.Is(err, identity.ErrInvalidToken) {
				// CLOSED WITH A REASON, not force-closed.
				//
				// There are two revocation paths — this one and the command
				// loop's — and they used to end the connection differently:
				// the command loop sends StatusPolicyViolation with a reason,
				// this one dropped the socket. Which one fired was a race, so
				// a revoked participant was told why SOMETIMES. Measured 5 in
				// 30 runs, and the winner shifts with unrelated timing: the
				// person's client showed "closed" with nothing to explain it.
				//
				// conn.Close rather than shutdown(): shutdown waits on
				// pumpDone and this IS the pump, so calling it here would
				// deadlock. Close only writes the close frame; the read loop
				// then unwinds through its own error path exactly as before,
				// and serve's deferred CloseNow still does the final cleanup.
				// coder/websocket serialises writes internally, so a frame
				// already going out finishes first.
				if !closing.Load() {
					_ = conn.Close(websocket.StatusPolicyViolation,
						"gateway: credential no longer valid")
				}
				return
			}

			// PROJECTED PER RECIPIENT (visibility spec §4), and this is the
			// seam the whole arc turns on. It already existed: this goroutine
			// holds p for the connection's life and already marshaled per
			// connection, so what one seat may see is decided in the one place
			// that was already per-seat.
			//
			// Marshaled per connection, deliberately: each pump encodes
			// straight off its own subscription channel with no shared
			// cache. At table scale (a handful of participants, not
			// thousands of fan-out sockets) a few extra protojson.Marshal
			// calls per event is cheap. Revisit with a real broadcast hub
			// (marshal once, shared bytes) only if client count per
			// campaign ever grows past table scale (say, >10) — and note that
			// a hub can only ever share bytes between seats with the SAME
			// projection, which is the DM and the agent.
			//
			// ONE EVENT IS NOW ZERO, ONE OR SEVERAL FRAMES for a projected
			// seat: an arrival is an ActorAdded plus a TokenPlaced plus a
			// SceneSeen, all stamped with the sequence that caused them. The
			// DM and the agent still get exactly one frame per event, by
			// pointer, unchanged.
			for _, pe := range sub.receive(env) {
				b, err := EncodeFrame(&vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: pe}})
				if err != nil {
					continue // a marshal failure here is a server bug, not this client's fault
				}
				select {
				case outCh <- b:
				case <-writerDone:
					return
				}
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

	// AFTER the pump is running, and that ordering is the whole point.
	//
	// Announcing an arrival walks the registry SERIALLY, waiting up to
	// presenceSendBudget on each connection. Done before the pump started, the
	// news of your arrival sat on the critical path of your own catch-up: N
	// wedged peers cost N x budget, paid by the person joining, who has done
	// nothing wrong. MEASURED: with a 2s budget and one peer whose socket had
	// genuinely backed up, a joiner waited 2.018s for its first event; at the
	// registry, one stalled peer costs 101ms against a 100ms budget, two 202ms,
	// three 302ms. With the production 3s budget and two dead tabs left open
	// somewhere, a new player waits six seconds to see the board.
	//
	// The bound itself is deliberate and unchanged (spec §4.1): a client that
	// is merely BUSY must keep its frame, and an instant drop was tried and was
	// wrong. What moved is WHO WAITS. The announcement is other people's news;
	// the catch-up is the joiner's own reason for connecting.
	//
	// SYNCHRONOUS, not a goroutine, and that is deliberate too. In a goroutine
	// a fast disconnect could let leavePresence broadcast DISCONNECTED before
	// this CONNECTED landed, and a client that re-adds on CONNECTED would keep
	// a ghost for the rest of the session — the same inversion that made
	// announcePromotion take one lock hold instead of three. So the read loop
	// below still waits for this; only the pump no longer does, and a joiner
	// with nothing on screen yet has nothing to send.
	//
	// Only the participant's FIRST connection is an arrival. A second device
	// must not announce someone who is already at the table.
	if firstConnection {
		s.announcePresence(pc)
	}

	// shutdown tears the connection's helper goroutines down in dependency
	// order: mark this as an intentional close (so the pump's post-loop
	// check above is a no-op), stop the subscription (closes `events`),
	// wait for the pump to drain it, THEN signal the writer to flush what is
	// queued and stop, and wait for it to exit.
	//
	// Safe to call after the pump has already force-closed the connection on
	// overflow — but note WHY, because the previous wording ("unsubscribe,
	// CloseNow, and channel-close are all idempotent here") was wrong about the
	// last one and invited the second call that would panic. Closing a channel
	// twice panics. What is actually true is that shutdown runs at most once
	// per serve, and the catch-up-head failure path returns before shutdown
	// exists — so close(flush) has exactly one caller on any given path.
	shutdown := func() {
		// Presence FIRST — but the reason has changed, and the history is
		// worth keeping because it is what makes the current shape defensible.
		//
		// This used to be an ordering against close(outCh): leave took the
		// registry lock, broadcast HELD that same lock while sending, so once
		// leave returned no broadcast could still be holding this connection.
		// Get it backwards and the registry kept handing frames to a channel
		// this function had already closed — "send on closed channel", raised
		// inside a teardown that is often already unwinding, and caught only
		// by net/http's handler recover. The consequence was worse than the
		// crash: map iteration order is random, so a panic mid-broadcast
		// delivered the departure to SOME connections and never to the rest, a
		// permanent ghost at the table, arriving silently. Found by review
		// under -race when `task check` ran no -race anywhere; check:race
		// closes that (2026-08-08), and reinstating the bad ordering was the
		// injection that proved the new gate has teeth.
		//
		// #47 dissolved that interlock, because sends no longer happen under
		// the registry lock at all — so instead of ordering the close, there
		// IS no close (see the flush channel above). What leaving first buys
		// now is smaller and still worth it: the departure is announced before
		// the writer stops, so it goes out on this connection's own socket
		// too, and pc.done is closed early enough that a fan-out already
		// holding this connection abandons it rather than spending the budget.
		//
		// The deferred leave below stays, as a backstop for the return paths
		// that never reach shutdown. Both are safe because leave() is
		// idempotent and closes pc.done behind its own membership check — NOT
		// because of a sync.Once here, which is what this first was and what a
		// reader adding a third call site would otherwise go looking for.
		leavePresence()
		closing.Store(true)
		unsubscribe()
		<-pumpDone
		close(flush)
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

		// RE-RESOLVE, every command. Verify ran once before the upgrade and
		// answered "who is this, and what may they do?" — but the first half
		// is a connection-time fact and the second is a LIVE one. Trusting the
		// cached answer meant a promotion did not bite until the participant
		// reconnected, which would sit on the critical path of everybody who
		// ever joins (they all arrive as spectators), and it meant `vtt revoke`
		// removed nobody: a revoked participant kept playing until they chose
		// to disconnect. Spec §3.2.
		//
		// Measured at 15.5µs against a 40-participant table, on a path that
		// already folds state, appends to SQLite and writes a socket frame.
		//
		// THIS IS HALF OF IT. A spectator issues no commands at all, so the
		// pump above re-resolves on DELIVERY for the same reason. Change one
		// and you almost certainly mean to change the other.
		now, err := s.ids.Lookup(p.ID)
		if errors.Is(err, identity.ErrInvalidToken) {
			// Revoked, or gone. Close rather than refuse-and-continue: their
			// credential is no longer valid, so there is nothing left for this
			// connection to be allowed to do.
			shutdown()
			_ = conn.Close(websocket.StatusPolicyViolation, "gateway: credential no longer valid")
			return
		}

		// ANY OTHER ERROR IS OPERATIONAL, and must not be read as a
		// revocation. Putting a database read on this path also put its
		// failure modes here: a busy file, a corrupt row, a driver error. None
		// of those is a fact about this person's credential, and closing on
		// them tells a player in good standing that theirs is no longer valid
		// — a lie, and one that throws them out of a live table over a
		// transient. identity's own comment records this shared file blocking
		// the full busy_timeout and then failing under another handle's write
		// transaction, so it is measured, not hypothetical.
		//
		// Refuse the command and keep the connection, exactly as handleCommand
		// already does for a campaign that cannot answer. Still fail-closed
		// where it counts: nothing is authorized while we cannot say who is
		// asking.
		var result *vttv1.CommandResult
		if err != nil {
			result = &vttv1.CommandResult{
				RequestId: cmd.GetRequestId(),
				Ok:        false,
				Error:     "gateway: identity unavailable",
			}
		} else {
			result = s.handleCommand(now, cmd)
		}
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
func describeBlockage(why string) string {
	if kind, ok := strings.CutPrefix(why, "scenery: "); ok {
		return "something (a " + kind + ") is in the way"
	}
	if strings.HasPrefix(why, "unknown scene ") {
		return "that destination is not part of any scene this table has created"
	}
	return why
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
				if blocked, why := st.Blocked(tok.SceneID, to.GetX(), to.GetY()); blocked {
					return &vttv1.CommandResult{
						RequestId: requestID, Ok: false,
						Error: "gateway: cannot move there — " + describeBlockage(why),
					}
				}
				// AND YOU MAY ONLY MOVE WHERE YOU CAN SEE (visibility spec
				// exit criterion 7: the goblin's square "cannot be targeted by
				// a player who cannot see it"). This is the second half of
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
				// The cost is real and deliberate: a square out of line of
				// sight cannot be walked onto even when it is remembered
				// terrain (spec §3.2), so a player cannot round a corner in
				// one command. Fail closed (spec §4.4). And this is the
				// PLAYER's branch only — "hard for players, free for DM"
				// (maps-as-geometry spec §6) governs sight exactly as it
				// governs stone.
				if !canSee(viewerFor(p), st, tok.SceneID, to) {
					return &vttv1.CommandResult{
						RequestId: requestID, Ok: false,
						Error: "gateway: cannot move there — you cannot see that square",
					}
				}
			}
		}
	}

	// create_scene's terrain gets the SAME seam and the SAME reasoning as the
	// movement check just above, applied to a different command (whole-
	// branch-review finding C5): checked HERE, not in engine.Apply, because
	// Apply is the fold and by the time an event reaches it the scene is
	// already history — history is not the place to say no.
	//
	// UNLIKE the movement check, this does NOT gate on p.Role. "Hard for
	// players, free for DM" (spec §6) is a rule about MOVEMENT freedom: an
	// author is allowed to stage a creature inside a wall. It is not a rule
	// about FORMAT validity — a tile kind of "banana" is never a legitimate
	// thing for anyone to author, DM or agent included, because the engine
	// (terrain.go) understands exactly three kinds and nothing reads a
	// fourth. So every actor who may issue create_scene is held to the same
	// closed vocabulary Authorize already decided they may use the command
	// at all.
	if cs, ok := cmd.GetCommand().(*vttv1.ClientCommand_CreateScene); ok {
		if err := validateCreateSceneTerrain(cs.CreateScene); err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
	}

	// use_ability/load_adventure/load_map do not become a single Envelope via
	// ToEvent (they each produce a whole ordered batch instead — ruleset.go/
	// adventure.go/map.go); every other command, including remove_condition,
	// still flows through the plain ToEvent -> campaign.Append path below.
	if ua, ok := cmd.GetCommand().(*vttv1.ClientCommand_UseAbility); ok {
		return s.handleUseAbility(requestID, ua.UseAbility, st, p)
	}
	if la, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadAdventure); ok {
		return s.handleLoadAdventure(requestID, la.LoadAdventure, st, p)
	}
	if lm, ok := cmd.GetCommand().(*vttv1.ClientCommand_LoadMap); ok {
		return s.handleLoadMap(requestID, lm.LoadMap, p)
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
