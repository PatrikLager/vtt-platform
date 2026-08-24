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
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// --- /api/maps and /api/packs/{pack}/{file} (maps-as-geometry Task 7) -----
//
// Both routes are open to EVERY role, unlike the adventure guide (DM/agent
// only — DM secrets) or the join link (admission control): a map's geometry
// and a pack's art carry neither. Spec §7 states this plainly — "everyone
// still sees the whole map, no filtering in this arc" — so the gate here is
// simply s.authed, the same shape /api/ruleset and the /api/adventures
// LIST already use.
//
// CONTENT TYPE AND TRUST BOUNDARY (decided here, not left implicit — and
// corrected once already by review, see the note below on what changed).
//
// handlePackFile does NOT let Content-Type be inferred from the file the
// way server.go's WithStatic route does for the client bundle
// (http.FileServerFS's extension/sniffing inference). Instead it serves
// under a CLOSED ALLOWLIST of genuine tile-art extensions
// (packFileContentTypes below) with their real Content-Type and
// X-Content-Type-Options: nosniff; anything not on that list is served as
// application/octet-stream with Content-Disposition: attachment (still
// nosniff), so a browser navigating directly to it downloads rather than
// executes it, whatever it turns out to be.
//
// WHY THIS IS NOT THE SAME CALL AS THE STATIC BUNDLE, even though both are
// "serve a directory of files this process did not author": the static
// bundle is FIRST-PARTY — built by this repo's own `task build:client`,
// committed, embedded into the binary. A pack is directory content an
// OPERATOR installs, potentially from a third party (spec §4.2 states a
// pack carries the same trust as an adventure's guide.md — but a guide is
// hand-authored markdown a browser never executes; a pack is more likely to
// be community content, and unlike a guide, this route serves it as raw,
// content-type-labelled bytes rather than JSON-wrapped text). An
// operator-installed pack containing an .html or .js file, served with a
// browser-executable Content-Type at a same-origin URL, would let script
// read this client's own Bearer token — it is stored in localStorage
// (client/src/auth.ts) and sent on every /api/* request (client/src/
// metadata.ts) — and call any authenticated route as that participant.
// Markdown can never do that; same-origin JavaScript can. That is the
// actual difference in KIND review found and this comment previously
// missed: "same trust as guide.md" does not mean "safe to serve as
// browser-executable content," because guide.md was never executable to
// begin with.
//
// SVG is deliberately treated as UNSAFE and left OFF the allowlist (forced
// down the attachment/octet-stream path) even though it is nominally an
// image format some pack authors might reach for: an SVG document can
// embed <script>, so "it has an image extension" is not the same claim as
// "a browser cannot execute anything in it" the way it is for PNG/JPEG/GIF/
// WebP, which carry no script-execution surface in any current browser.
//
// This is layered ON TOP of, not instead of, the filesystem-level boundary
// (an fs.FS built via os.OpenRoot cannot be walked outside the directory it
// was rooted at, by construction or by symlink — see WithPackFiles' doc
// comment) and the authentication boundary (every pack/map route requires
// the same Bearer header every other /api route does). A hostile pack
// directory can still make ITS OWN images ugly, wrong, or offensive — that
// remains the operator's call to vet, same as an adventure's content — but
// it can no longer turn into script running in this origin.

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
// Guides are passed in rather than read from disk here because the gateway
// does no file I/O — cmd/vtt owns the filesystem (ADR-008), and a guide read
// at request time would also mean an unreadable file becomes a 500 in the
// middle of a session instead of a loud failure at boot.
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

type packRefJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	CellPx int32  `json:"cellPx"`
}

type mapMetaJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GridWidth  int32  `json:"gridWidth"`
	GridHeight int32  `json:"gridHeight"`
	// Pack is a pointer, omitted entirely for a map that names no pack
	// (mapdef.Map.Pack "" is legal — a map using only standard tiles) rather
	// than a zero-valued packRefJSON, which would read as "a pack named
	// empty-string" instead of "no pack".
	Pack *packRefJSON `json:"pack,omitempty"`
}

// handleMaps lists every boot-loaded standalone map (maps-as-geometry Task
// 7), open to every role (this file's own doc comment above explains why).
// Each entry's Pack is looked up from s.packs by the map's OWN declared
// Pack id — enriching the listing with the pack's display name and cell
// size so a client can render at the right scale without a second request;
// nil if the map declares no pack, or (should not happen past boot
// validation, but handled rather than assumed) the id is not one of s.packs.
func (s *Server) handleMaps(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	out := []mapMetaJSON{}
	for id, m := range s.maps {
		item := mapMetaJSON{ID: id, Name: m.Name, GridWidth: m.GridW, GridHeight: m.GridH}
		if p, ok := s.packs[m.Pack]; ok {
			item.Pack = &packRefJSON{ID: p.ID, Name: p.Name, CellPx: p.CellPx}
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b mapMetaJSON) int { return strings.Compare(a.ID, b.ID) })
	writeJSON(w, map[string]any{"maps": out})
}

// --- /api/packs/{pack}/{file} -------------------------------------------

// handlePackFile serves one raw file out of a pack's own directory
// (maps-as-geometry Task 7), open to every role. Escape is refused by the
// fs.FS s.packFS[pack] IS, not by anything this handler checks itself — two
// independent mechanisms, both load-bearing (packfile_internal_test.go
// proves each separately, because a fix for one says nothing about the
// other):
//
//  1. A literal ".." in the requested name: fs.FS's own contract
//     (fs.ValidPath) refuses any name containing a ".." element, so
//     http.ServeFileFS's underlying fsys.Open call rejects it regardless of
//     what this handler does or forgets to do. (http.ServeFileFS also has
//     its own separate precaution against a dirty r.URL.Path, and
//     net/http's ServeMux redirects a dirty path before routing even gets
//     here — both incidental, neither one is what this handler depends on.)
//  2. A file that is, on disk, a symlink pointing OUTSIDE the pack
//     directory — no ".." anywhere in the request, so mechanism 1 does not
//     apply. This one is NOT fs.FS's contract in general (os.DirFS,
//     otherwise a valid fs.FS, explicitly does not stop it — see its own
//     doc comment) — it depends on s.packFS[pack] specifically being built
//     from os.OpenRoot(dir).FS(), which cmd/vtt does (see WithPackFiles'
//     doc comment for the full reasoning; this was found missing by
//     review, after DirFS shipped first).
//
// An unknown pack id 404s before ever touching the filesystem, rather than
// serving a directory listing or leaking which packs exist through a
// different status code.
//
// Content-Type is NOT inferred (this section's own package-doc comment
// explains why): the extension of the requested name is looked up against
// packFileContentTypes, a closed allowlist. A hit gets served with its real
// Content-Type, inline. A miss — including pack.json itself, which is
// structured data a client fetches programmatically rather than tile art a
// browser renders, and including .svg, deliberately excluded from the
// allowlist — gets application/octet-stream and Content-Disposition:
// attachment, so a browser navigating to it downloads rather than executes
// whatever it turns out to be. X-Content-Type-Options: nosniff is set on
// EVERY response from this handler, allowlisted or not, so a browser never
// second-guesses either header.
//
// Content-Type/Content-Disposition are set BEFORE calling http.ServeFileFS
// deliberately: net/http's serveContent only infers a type when the
// response's Content-Type header is still unset at that point, so setting
// it here first is what suppresses ServeFileFS's own inference rather than
// racing it.
func (s *Server) handlePackFile(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	fsys, ok := s.packFS[r.PathValue("pack")]
	if !ok {
		http.Error(w, "gateway: unknown pack", http.StatusNotFound)
		return
	}

	name := r.PathValue("file")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if ct, ok := packFileContentTypes[strings.ToLower(filepath.Ext(name))]; ok {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(name)))
	}
	http.ServeFileFS(w, r, fsys, name)
}

// packFileContentTypes is the closed allowlist handlePackFile serves WITH
// their real Content-Type — genuine tile-art raster formats only, all of
// them carrying no script-execution surface in any current browser. .svg
// is deliberately NOT here (this section's own package-doc comment
// explains why) despite nominally being an image format: it can embed
// <script>, so it is routed down the octet-stream/attachment path with
// everything else this map does not name.
var packFileContentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}
