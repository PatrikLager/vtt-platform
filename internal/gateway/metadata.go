// Metadata endpoints: the read-only HTTP surface a client needs before it can
// render anything — what ruleset is loaded, what abilities exist, which
// adventures are available, and the markdown guides.
//
// # Auth is a Bearer header, NOT ?token=
//
// The WebSocket route takes its token as a query parameter, and these routes
// deliberately do not follow that precedent. A token in a URL leaks into
// places nobody audits: server access logs, the Referer header on any
// outbound link, proxy logs, browser history, and error strings. This
// codebase already carries the scars — internal/harness/client.go has a
// redactURL regex precisely because a WS URL with a token in it must never be
// printed, and README.md warns about fronting proxies for the same reason.
//
// A header has none of those paths. The WS parameter stays as it is because
// browsers cannot set headers on a WebSocket handshake; HTTP has no such
// excuse, so it does not get the exception.
//
// # Empty is not an error
//
// A server with no ruleset answers /api/ruleset with 200 and empty
// collections rather than 404 (client spec §5: "clean empty responses the UI
// renders honestly"). The client then shows an empty picker instead of an
// error banner, which is the truth: there is nothing to pick. GUIDES are the
// exception — a guide that does not exist is a 404, because an empty string
// would render as a blank document and read as a broken one.
package gateway

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// --- /api/maps (maps-as-geometry Task 7) ----------------------------------
//
// Open to EVERY role, unlike the adventure guide (DM/agent only — DM secrets)
// or the join link (admission control): a map's geometry carries neither. Spec
// §7 states this plainly — "everyone still sees the whole map, no filtering in
// this arc" — so the gate here is simply s.authed, the same shape /api/ruleset
// and the /api/adventures LIST already use.
//
// --- THE RULING THIS FILE'S RAW-BYTE ROUTE WAS BUILT ON, kept because the
// route that inherits it has not been built yet -----------------------------
//
// GET /api/packs/{pack}/{file} served raw bytes out of an operator-installed
// pack directory. 2026-09-02-art-is-a-flat-library Task 7 deleted it with the
// pack, and Task 6 of that plan builds GET /api/art/{file} over the campaign's
// one flat art/ directory — the SAME problem, one directory over: a route
// handing a browser bytes that this process did not author. Everything below
// was decided here, corrected once by review, and is written down rather than
// deleted so the next route does not rediscover it by shipping the hole first.
//
// NEVER LET Content-Type BE INFERRED the way server.go's WithStatic route does
// for the client bundle (http.FileServerFS's extension/sniffing inference).
// Serve under a CLOSED ALLOWLIST of genuine raster tile-art extensions with
// their real Content-Type and X-Content-Type-Options: nosniff; serve anything
// NOT on that list as application/octet-stream with Content-Disposition:
// attachment (still nosniff), so a browser navigating directly to it downloads
// rather than executes it, whatever it turns out to be. Set both headers BEFORE
// calling http.ServeFileFS: net/http's serveContent only infers a type when
// Content-Type is still unset at that point, so setting it first is what
// suppresses the inference rather than racing it.
//
// WHY THIS IS NOT THE SAME CALL AS THE STATIC BUNDLE, even though both are
// "serve a directory of files this process did not author": the static bundle
// is FIRST-PARTY — built by this repo's own `task build:client`, committed,
// embedded into the binary. Art is directory content an OPERATOR installs,
// potentially from a third party (maps-as-geometry spec §4.2 said a pack
// carried the same trust as an adventure's guide.md — but a guide is
// hand-authored markdown a browser never executes, and this kind of route
// serves raw content-type-labelled bytes rather than JSON-wrapped text). An
// installed file that is .html or .js, served with a browser-executable
// Content-Type at a same-origin URL, would let script read this client's own
// Bearer token — it is stored in localStorage (client/src/auth.ts) and sent on
// every /api/* request (client/src/metadata.ts) — and call any authenticated
// route as that participant. Markdown can never do that; same-origin JavaScript
// can. That is the actual difference in KIND review found and this comment
// previously missed: "same trust as guide.md" does not mean "safe to serve as
// browser-executable content", because guide.md was never executable to begin
// with.
//
// SVG IS UNSAFE and belongs OFF the allowlist (forced down the
// attachment/octet-stream path) even though it is nominally an image format an
// author might reach for: an SVG document can embed <script>, so "it has an
// image extension" is not the same claim as "a browser cannot execute anything
// in it" the way it is for PNG/JPEG/GIF/WebP, which carry no script-execution
// surface in any current browser.
//
// ALL OF THAT IS LAYERED ON TOP OF, NOT INSTEAD OF, two other boundaries, and
// each was proven separately because a fix for one says nothing about the
// other:
//
//  1. The filesystem boundary. An fs.FS built via os.OpenRoot(dir).FS()
//     (go1.24+) cannot be walked outside dir, by ".." or by symlink; os.DirFS
//     CANNOT make that claim and its own doc comment says so ("does not stop
//     the access any more than using os.Open does"). This distinction was found
//     missing by review after DirFS shipped first, and internal/artlib already
//     opens every file through os.OpenRoot for the same reason
//     (TestLookupWillNotFollowASymlinkOutOfTheArtDirectory).
//  2. The authentication boundary. Every /api route requires the same Bearer
//     header, and this kind of route is no exception — operator-installed
//     content is trusted about what an AUTHENTICATED caller may read, not about
//     skipping authentication the way /join and the static bundle deliberately
//     do.
//
// A hostile art directory can still make ITS OWN pictures ugly, wrong or
// offensive — that remains the operator's call to vet, as an adventure's
// content is — but it cannot turn into script running in this origin.
//
// ONE THING IS NEW FOR art/ AND HAS NO PACK PRECEDENT: art/ is FLAT (that
// plan's design spec §3.1/§3.3), and os.OpenRoot CONFINES without FLATTENING —
// "art/pack-ish/x.png" is legitimately inside the root and fs.ValidPath rejects
// only "..". net/http's single-segment {file} wildcard not matching across "/"
// is what the pack route relied on; Task 6 must keep that, or check the name,
// and test it directly.

// adventureGuideRoles mirrors the dm/agent shape load_adventure carries
// (authz.go) — adventure guides hold DM secrets (adventure/format.go), so a
// player or spectator reading one would leak the plot, not merely exceed a
// permission.
//
// Deliberately NOT a cell of commandRoles: that table's keys are ClientCommand
// oneof field names and its cell count is asserted literally in authz_test.go.
// HTTP routes are not wire commands, and folding them in would break that
// count for no gain.
var adventureGuideRoles = map[identity.Role]bool{
	identity.RoleDM:    true,
	identity.RoleAgent: true,
}

// joinLinkRoles gates GET /api/join-link (joining-a-table spec §5).
//
// This route is different in kind from everything else behind /api. A player
// may read the ruleset and a spectator may list adventures — those are facts
// about the table. This one hands back a SHARED SECRET that admits ANYBODY who
// holds it, so a spectator who could read it could staff the table with
// strangers, and the spectator default the whole design rests on would be
// decoration.
//
// Same shape as adventureGuideRoles, and NOT a cell of commandRoles for the
// same stated reason: that table's keys are ClientCommand oneof field names
// and its cell count is asserted literally.
var joinLinkRoles = map[identity.Role]bool{
	identity.RoleDM:    true,
	identity.RoleAgent: true,
}

// participantRoles gates GET /api/participants — the table's roster.
//
// A SEPARATE map from joinLinkRoles even though the values match today. "Who
// may read a secret that admits anybody" and "who may see who is at this
// table" are two questions, and one map answering both means widening either
// one silently widens the other.
var participantRoles = map[identity.Role]bool{
	identity.RoleDM:    true,
	identity.RoleAgent: true,
}

// WithAdventureGuides supplies the markdown served by
// /api/adventures/{id}/guide, keyed by adventure id. Boot-time only, like
// WithAdventures: the map is never mutated per request.
//
// Guides are passed in rather than read from disk here because cmd/vtt owns
// the filesystem (ADR-008), and a guide read at request time would also
// mean an unreadable file becomes a 500 in the middle of a session instead
// of a loud failure at boot.
//
// That rule now has exactly one deliberate exception, and this is not it:
// map.go's mapByID probes the campaign's maps/ on a lookup miss, because
// the 2026-09-01-create-scene-leaves design spec §5 assigns that probe to
// the server on purpose — a map authored mid-session has to be loadable
// without a restart, and there is nothing about a guide that needs the
// same.
func (s *Server) WithAdventureGuides(guides map[string]string) *Server {
	s.adventureGuides = guides
	return s
}

// authed verifies the Bearer token and returns the participant, or writes the
// 401 itself and returns nil.
func (s *Server) authed(w http.ResponseWriter, r *http.Request) *identity.Participant {
	// CutPrefix rather than a hand-rolled length test: the manual version
	// carried an off-by-one boundary of its own invention (is a bare
	// "Bearer " with an empty token caught by the length check or by Verify?)
	// that no observable behaviour depended on. Fewer branches, fewer things
	// to get subtly wrong.
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		http.Error(w, "gateway: unauthorized", http.StatusUnauthorized)
		return nil
	}
	p, err := s.ids.Verify(token)
	if err != nil {
		// Deliberately not distinguishing unknown from revoked: telling an
		// unauthenticated caller which one it was is a token-probing oracle.
		http.Error(w, "gateway: unauthorized", http.StatusUnauthorized)
		return nil
	}
	return p
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	// The status line is already written by the time Encode can fail, so there
	// is nothing left to tell the client — the response simply ends short.
	// Discarded explicitly rather than guarded by an `if` that only returns.
	_ = json.NewEncoder(w).Encode(v)
}

// --- /api/me ---------------------------------------------------------------

type meJSON struct {
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

// handleMe tells a client who its token makes it.
//
// Without this the client cannot know its own role or participant id, and
// both are load-bearing: "which actors do I control" is a membership test of
// participantId in Actor.controller_ids, and the role decides which panels
// render at all. Inferring either from the event stream would be guesswork —
// a spectator who has caused no events is indistinguishable from a player who
// has not acted yet.
//
// IT DOES NOT ANSWER WHAT YOU CONTROL, and that is the point (2026-08-24).
// This route used to echo participants.controls, a column no grant ever wrote,
// so it reported control that no rule in the system agreed with. The client
// already asks the right source — client/src/player.ts's controlledActors
// filters the folded st.Actors on controllerIds — so the identity it needs
// from here is the participant id, and control follows from the log.
//
// It reveals nothing the caller did not already prove by holding the token.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := s.authed(w, r)
	if p == nil {
		return
	}
	writeJSON(w, meJSON{
		ParticipantID: p.ID,
		Name:          p.Name,
		Role:          string(p.Role),
	})
}

// --- /api/ruleset ----------------------------------------------------------

type abilityJSON struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Range      int       `json:"range"`
	MaxTargets int       `json:"maxTargets"`
	Usage      usageJSON `json:"usage"`
}

type usageJSON struct {
	Kind     string `json:"kind"` // "atWill" | "resource"
	Resource string `json:"resource,omitempty"`
	Cost     int    `json:"cost,omitempty"`
}

type conditionJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type rulesetJSON struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Abilities  []abilityJSON   `json:"abilities"`
	Conditions []conditionJSON `json:"conditions"`
	Resources  []string        `json:"resources"`
}

func (s *Server) handleRuleset(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	// Every slice starts non-nil: a JSON `null` where the client expects an
	// array turns a "nothing loaded" server into a client-side crash on the
	// first .map().
	out := rulesetJSON{
		Abilities:  []abilityJSON{},
		Conditions: []conditionJSON{},
		Resources:  []string{},
	}
	if s.ruleset != nil {
		rs := s.ruleset
		out.ID, out.Name = rs.ID, rs.Name

		for _, p := range rs.Compiled {
			a := abilityJSON{
				ID:         p.ID,
				Name:       p.Name,
				Range:      p.Targeting.Range,
				MaxTargets: p.Targeting.MaxTargets,
			}
			switch {
			case p.Usage.Limited != nil:
				a.Usage = usageJSON{
					Kind:     "resource",
					Resource: p.Usage.Limited.Resource,
					Cost:     p.Usage.Limited.Cost,
				}
			default:
				a.Usage = usageJSON{Kind: "atWill"}
			}
			out.Abilities = append(out.Abilities, a)
		}
		// Compiled is a map and Go randomizes map iteration, so without this
		// the ability list arrives in a different order on every request and
		// the client's picker reshuffles under the user's cursor.
		slices.SortFunc(out.Abilities, func(a, b abilityJSON) int { return strings.Compare(a.ID, b.ID) })

		for _, c := range rs.Conditions {
			out.Conditions = append(out.Conditions, conditionJSON{
				ID: c.ID, Name: c.Name, Description: c.Description,
			})
		}
		slices.SortFunc(out.Conditions, func(a, b conditionJSON) int { return strings.Compare(a.ID, b.ID) })

		for _, rd := range rs.Resources {
			out.Resources = append(out.Resources, rd.Name)
		}
	}
	writeJSON(w, out)
}

// --- /api/ruleset/guide ----------------------------------------------------

func (s *Server) handleRulesetGuide(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	if s.ruleset == nil || s.ruleset.Guide == "" {
		http.Error(w, "gateway: no ruleset guide available", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"guide": s.ruleset.Guide})
}

// --- /api/adventures -------------------------------------------------------

type adventureJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) handleAdventures(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	// The LIST is open to every role even though the GUIDES are not: a player
	// may see which adventures exist without reading the DM's secrets.
	out := []adventureJSON{}
	for id, adv := range s.adventures {
		out = append(out, adventureJSON{ID: id, Name: adv.Name})
	}
	slices.SortFunc(out, func(a, b adventureJSON) int { return strings.Compare(a.ID, b.ID) })
	writeJSON(w, map[string]any{"adventures": out})
}

// --- /api/adventures/{id}/guide --------------------------------------------

func (s *Server) handleAdventureGuide(w http.ResponseWriter, r *http.Request) {
	p := s.authed(w, r)
	if p == nil {
		return
	}
	if !adventureGuideRoles[p.Role] {
		http.Error(w, "gateway: not authorized", http.StatusForbidden)
		return
	}
	guide, ok := s.adventureGuides[r.PathValue("id")]
	if !ok || guide == "" {
		http.Error(w, "gateway: no guide for that adventure", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"guide": guide})
}

// --- /api/join-link --------------------------------------------------------

type joinLinkJSON struct {
	Open   bool   `json:"open"`
	Secret string `json:"secret"`
	// The budget, because a door has a third state now: open, shut, and open
	// but spent. Without these the console can only say "open" about a link
	// that refuses everyone, and the DM's only way to find out is a player
	// telling them they were turned away — with the same message a stranger
	// gets, so neither of them can tell why.
	Admitted   int `json:"admitted"`
	AdmitLimit int `json:"admitLimit"`
}

// handleJoinLink reports the shared join link and whether the door is open.
//
// The browser cannot read identity's SQLite, so this is the DM console's only
// mirror of both facts. BOTH are returned together on purpose: a console that
// showed the link without the door would have a DM confidently sending out a
// URL that admits nobody, and one that showed the door without the link would
// leave them nothing to send.
//
// The secret is readable BEFORE the door is opened, deliberately. The
// alternative ordering — open first, then look — means the only way to get the
// link is to have the door already standing open while you go and find someone
// to send it to.
func (s *Server) handleJoinLink(w http.ResponseWriter, r *http.Request) {
	p := s.authed(w, r)
	if p == nil {
		return
	}
	if !joinLinkRoles[p.Role] {
		http.Error(w, "gateway: not authorized", http.StatusForbidden)
		return
	}
	secret, err := s.ids.JoinSecret()
	if err != nil {
		http.Error(w, "gateway: join link unavailable", http.StatusInternalServerError)
		return
	}
	admitted, limit, err := s.ids.JoinBudget()
	if err != nil {
		http.Error(w, "gateway: join link unavailable", http.StatusInternalServerError)
		return
	}
	writeJSON(w, joinLinkJSON{
		Open: s.ids.JoinOpen(), Secret: secret,
		Admitted: admitted, AdmitLimit: limit,
	})
}

// --- /api/participants -----------------------------------------------------

type participantJSON struct {
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

// handleParticipants lists everyone who can still act at this table.
//
// This is what the DM console's promote control is built on, and it reads
// identity rather than presence on purpose: presence answers "who is connected
// right now", which is connection-scoped and carries no role, while promotion
// is a question about what somebody is ALLOWED to do. Folding a role into a
// presence frame would go stale the moment somebody was promoted without
// reconnecting — which is exactly what live re-resolution made possible.
//
// It returns names, ids and roles: no token, no hash. The roster is a list of
// people, not of credentials.
func (s *Server) handleParticipants(w http.ResponseWriter, r *http.Request) {
	p := s.authed(w, r)
	if p == nil {
		return
	}
	if !participantRoles[p.Role] {
		http.Error(w, "gateway: not authorized", http.StatusForbidden)
		return
	}
	list, err := s.ids.List()
	if err != nil {
		http.Error(w, "gateway: participants unavailable", http.StatusInternalServerError)
		return
	}
	// Built as a non-nil slice so an empty table serializes as [] rather than
	// null — a client that does list.map() on null gets an exception, and
	// "nobody is here" is a perfectly ordinary state.
	out := make([]participantJSON, 0, len(list))
	for _, q := range list {
		out = append(out, participantJSON{ParticipantID: q.ID, Name: q.Name, Role: string(q.Role)})
	}
	writeJSON(w, out)
}

// --- /api/maps ---------------------------------------------------------

// packRefJSON IS GONE, and its absence is the requirement now. It carried a
// map's pack — id, display name, cell size — on every /api/maps entry, built
// by looking s.packs up under the map's OWN declared pack id. That id was
// mapdef.Map.Pack, and Task 5 of 2026-09-02-art-is-a-flat-library deleted the
// field and made a map file that declares one a refusal (design spec §7). With
// nothing left to key the lookup by, a pack reference cannot be built at all —
// so this is rubble from that deletion rather than Task 6's work brought
// forward, and leaving the JSON key in place would have shipped a field that
// can never again be non-null.
//
// WHAT A CLIENT LOSES, AND IT IS NOT cellPx. An earlier version of this
// comment said the renderer read cellPx from pack.cellPx to draw at the right
// scale, and that was false — corrected in review, 2026-09-04. NOTHING in the
// client has ever read it: client/src/view/spectator.ts's CELL = 44 is the only
// cell size in the renderer, passed to planScene, planFog, planGrid and
// cellFromPoint, and client/src/metadata.ts merely DECLARES the field.
// client/public/std-pack/pack.json's own cell_px is ignored for the same
// reason. So this endpoint dropping cellPx costs the client nothing today, and
// Task 6 is what gives a campaign-level cellPx its first reader rather than
// what restores one. Believing otherwise would let Task 6 ship a server half,
// see no change, and think it had closed a regression that was never open.
//
// THE REAL LOSS IS THE ROUTE, NOT THE NUMBER, and as of Task 7 the route is
// literally gone rather than merely unaddressable. A pack id was the only thing
// this endpoint ever gave a client to fetch art WITH, and
// GET /api/packs/{pack}/{file} was deleted with the pack itself, so the client
// cannot fetch a campaign's art at all until GET /api/art/{file} exists
// (spec §6, Task 6). Nothing shows yet, because no shipped map's art resolves
// and every TileRef.art is empty, so scene-plan.ts's tileImage falls back to a
// "std:<kind>/<material>" key the client's own bundled baseline pack answers.
// The moment Task 8 installs campaigns/example/art/ and TileRef.art starts
// arriving non-empty, tileImage emits "tile:<art>" instead, the ImageMap has no
// such key, and canvas.ts paints drawMissingTile's magenta checkerboard over
// every overridden square. Task 6 lands before Task 8, so the order holds —
// but that is the dependency, and it is between those two tasks rather than
// between this one and either.

type mapMetaJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GridWidth  int32  `json:"gridWidth"`
	GridHeight int32  `json:"gridHeight"`
}

// handleMaps lists every map this server holds (maps-as-geometry Task 7),
// open to every role (this file's own doc comment above explains why). An
// entry is now the map's own identity and geometry and nothing else — see
// packRefJSON's obituary above for why the pack reference it used to carry
// could not survive mapdef.Map.Pack.
//
// The map set is no longer a boot-time constant: a map installed while the
// server runs joins it on its first successful load_map (map.go's mapByID,
// 2026-09-01-create-scene-leaves Task 6), so this listing grows during a
// session and the read below has to be guarded. The entries are copied out
// under the lock and the response is written outside it — a client that
// stops reading must not be able to hold the map set shut against every
// load_map for as long as it likes.
func (s *Server) handleMaps(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	out := []mapMetaJSON{}
	s.mapsMu.RLock()
	for id, m := range s.maps {
		out = append(out, mapMetaJSON{ID: id, Name: m.Name, GridWidth: m.GridW, GridHeight: m.GridH})
	}
	s.mapsMu.RUnlock()
	slices.SortFunc(out, func(a, b mapMetaJSON) int { return strings.Compare(a.ID, b.ID) })
	writeJSON(w, map[string]any{"maps": out})
}
