package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Keep the cap: /join is unauthenticated, so its caller chooses how much is
// read (SPEC-009).
const maxJoinBody = 4 << 10

// Count runes, never bytes: a byte cap gives a non-ASCII name as little as a
// quarter of the room (VTT-012).
const maxDisplayNameRunes = 64

// usableDisplayName holds the display-name rule SPEC-009 states (VTT-012).
func usableDisplayName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxDisplayNameRunes {
		return false
	}
	visible := false
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return false
		}
		// Keep Other_Default_Ignorable_Code_Point: U+3164 HANGUL FILLER is a
		// letter and draws nothing (VTT-012).
		switch {
		case unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r):
		case unicode.Is(unicode.Cf, r):
		case unicode.IsSpace(r):
		default:
			visible = true
		}
	}
	return visible
}

type joinRequest struct {
	Secret      string `json:"secret"`
	DisplayName string `json:"displayName"`
}

type joinResponse struct {
	Token         string `json:"token"`
	ParticipantID string `json:"participantId"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

// handleJoin is the POST /join SPEC-009 states. Mint a spectator and nothing
// else: whoever holds a shared link may only watch (VTT-010).
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJoinBody)).Decode(&req); err != nil {
		http.Error(w, "gateway: malformed join request", http.StatusBadRequest)
		return
	}

	// Refuse the name first, and distinctly: it says nothing about the door
	// (VTT-013).
	name := strings.TrimSpace(req.DisplayName)
	if !usableDisplayName(name) {
		http.Error(w, "gateway: a display name is required, up to 64 ordinary characters",
			http.StatusBadRequest)
		return
	}

	// Ask JoinAdmits alone, and refuse its error as a refusal: a database that
	// cannot answer must not open the door (SPEC-009, VTT-046).
	allowed, err := s.ids.JoinAdmits(req.Secret)
	if err != nil || !allowed {
		// Answer every refusal with this status and body: a difference tells a
		// prober which half it got right (VTT-009).
		http.Error(w, "gateway: this link is not accepting anyone", http.StatusForbidden)
		return
	}

	token, id, err := s.ids.CreateInvite(name, identity.RoleSpectator)
	if err != nil {
		http.Error(w, "gateway: could not join", http.StatusInternalServerError)
		return
	}
	writeJSON(w, joinResponse{
		Token:         token,
		ParticipantID: id,
		Name:          name,
		Role:          string(identity.RoleSpectator),
	})
}
