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
	ParticipantID string   `json:"participantId"`
	Name          string   `json:"name"`
	Role          string   `json:"role"`
	Controls      []string `json:"controls"`
}

// handleMe tells a client who its token makes it.
//
// Without this the client cannot know its own role or participant id, and
// both are load-bearing: "which actors do I control" is an equality check
// against participantId (Actor.controller_id), and the role decides which
// panels render at all. Inferring either from the event stream would be
// guesswork — a spectator who has caused no events is indistinguishable from
// a player who has not acted yet.
//
// It reveals nothing the caller did not already prove by holding the token.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := s.authed(w, r)
	if p == nil {
		return
	}
	// Non-nil so the client can iterate without a null check; a participant
	// controlling nothing is the common case for a DM or spectator.
	controls := p.Controls
	if controls == nil {
		controls = []string{}
	}
	writeJSON(w, meJSON{
		ParticipantID: p.ID,
		Name:          p.Name,
		Role:          string(p.Role),
		Controls:      controls,
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
