package gateway

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/artlib"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Keep the HTTP gates out of commandRoles: its keys are the ClientCommand
// oneof's fields, which TestEveryClientCommandHasRoleCells walks (SPEC-012).
var adventureGuideRoles = map[identity.Role]bool{
	identity.RoleDM:    true,
	identity.RoleAgent: true,
}

// Keep this the join link's own gate (SPEC-009).
var joinLinkRoles = map[identity.Role]bool{
	identity.RoleDM:    true,
	identity.RoleAgent: true,
}

// Keep this a separate map from joinLinkRoles: widening one must not widen
// the other (SPEC-009).
var participantRoles = map[identity.Role]bool{
	identity.RoleDM:    true,
	identity.RoleAgent: true,
}

// WithAdventureGuides supplies the markdown /api/adventures/{id}/guide serves,
// keyed by adventure id (SPEC-012).
func (s *Server) WithAdventureGuides(guides map[string]string) *Server {
	s.adventureGuides = guides
	return s
}

func (s *Server) authed(w http.ResponseWriter, r *http.Request) *identity.Participant {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		http.Error(w, "gateway: unauthorized", http.StatusUnauthorized)
		return nil
	}
	p, err := s.ids.Verify(token)
	if err != nil {
		// Answer an unknown and a revoked token alike: telling them apart is a
		// token-probing oracle (SPEC-009).
		http.Error(w, "gateway: unauthorized", http.StatusUnauthorized)
		return nil
	}
	return p
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	// Discard the encode error: the status line is already written.
	_ = json.NewEncoder(w).Encode(v)
}

type meJSON struct {
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := s.authed(w, r)
	if p == nil {
		return
	}
	// Do not answer control here: the log's ActorControlGranted is the only
	// authority (SPEC-012).
	writeJSON(w, meJSON{
		ParticipantID: p.ID,
		Name:          p.Name,
		Role:          string(p.Role),
	})
}

type abilityJSON struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Range      int       `json:"range"`
	MaxTargets int       `json:"maxTargets"`
	Usage      usageJSON `json:"usage"`
}

type usageJSON struct {
	Kind     string `json:"kind"`
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
	// Keep every slice non-nil: a JSON null where the client expects an array
	// crashes its first .map() (SPEC-012).
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
		// Keep the sort: Compiled is a map, and an unsorted list reshuffles the
		// picker on every request (SPEC-012).
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

type adventureJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) handleAdventures(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	// Keep the list open to every role and the guide gated: a guide holds the
	// DM's secrets (SPEC-012).
	out := []adventureJSON{}
	for id, adv := range s.adventures {
		out = append(out, adventureJSON{ID: id, Name: adv.Name})
	}
	slices.SortFunc(out, func(a, b adventureJSON) int { return strings.Compare(a.ID, b.ID) })
	writeJSON(w, map[string]any{"adventures": out})
}

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

type joinLinkJSON struct {
	Open       bool   `json:"open"`
	Secret     string `json:"secret"`
	Admitted   int    `json:"admitted"`
	AdmitLimit int    `json:"admitLimit"`
}

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

type participantJSON struct {
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

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
	// Keep the slice non-nil: an empty table is [] to a client, never null.
	out := make([]participantJSON, 0, len(list))
	for _, q := range list {
		out = append(out, participantJSON{ParticipantID: q.ID, Name: q.Name, Role: string(q.Role)})
	}
	writeJSON(w, out)
}

// DefaultCellPx is the cell size a server reports when nothing set one.
// Keep it equal to campaigncfg.DefaultCellPx without importing that package
// (TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber, SPEC-012).
const DefaultCellPx int32 = 64

type mapMetaJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GridWidth  int32  `json:"gridWidth"`
	GridHeight int32  `json:"gridHeight"`
	CellPx     int32  `json:"cellPx"`
}

func (s *Server) handleMaps(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	out := []mapMetaJSON{}
	// Copy the entries out under mapsMu and write outside it: a client that
	// stops reading must not hold the map set shut against load_map.
	s.mapsMu.RLock()
	for id, m := range s.maps {
		// Do not read the zero check as a guard: zero is a map that declared
		// nothing (SPEC-012).
		cellPx := m.CellPx
		if cellPx == 0 {
			cellPx = s.cellPx
		}
		out = append(out, mapMetaJSON{
			ID: id, Name: m.Name, GridWidth: m.GridW, GridHeight: m.GridH, CellPx: cellPx,
		})
	}
	s.mapsMu.RUnlock()
	slices.SortFunc(out, func(a, b mapMetaJSON) int { return strings.Compare(a.ID, b.ID) })
	writeJSON(w, map[string]any{"maps": out, "cellPx": s.cellPx})
}

// Keep this allowlist closed and set the type before ServeFileFS, or net/http
// infers one (SPEC-012).
var artContentTypes = map[string]string{
	".png": "image/png",
}

// Check the name with artlib.IsArtFileName before opening anything: the {file}
// pattern does not stop an encoded slash, and os.OpenRoot confines without
// flattening (SPEC-012).
func (s *Server) handleArtFile(w http.ResponseWriter, r *http.Request) {
	if s.authed(w, r) == nil {
		return
	}
	name := r.PathValue("file")
	if s.artDir == "" || !artlib.IsArtFileName(name) {
		http.Error(w, "gateway: no such art", http.StatusNotFound)
		return
	}
	root, err := os.OpenRoot(s.artDir)
	if err != nil {
		// Answer an absent and an unopenable root alike, and name no path: the body
		// reaches every seat (SPEC-012).
		http.Error(w, "gateway: no such art", http.StatusNotFound)
		return
	}
	defer root.Close()

	// Stat on root.FS(), never on the path: os.DirFS follows a symlink out and
	// would mask the confinement (TestHandleArtFileRefusesSymlinkEscape).
	fsys := root.FS()
	if info, err := fs.Stat(fsys, name); err != nil || !info.Mode().IsRegular() {
		http.Error(w, "gateway: no such art", http.StatusNotFound)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Keep no-cache: without it a browser reuses month-old art for days
	// (SPEC-012).
	w.Header().Set("Cache-Control", "no-cache")
	if ct, ok := artContentTypes[filepath.Ext(name)]; ok {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	}
	// Serve off root.FS(), for the reason the Stat above gives.
	http.ServeFileFS(w, r, fsys, name)
}
