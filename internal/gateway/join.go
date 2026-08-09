package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// maxJoinBody caps the request. The endpoint is unauthenticated, so it is the
// one place where an anonymous caller chooses how many bytes we read.
const maxJoinBody = 4 << 10

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

// handleJoin mints a participant from the shared join link (spec §2).
//
// UNAUTHENTICATED BY CONSTRUCTION — that is the point of a link you can share —
// which makes it the one surface where a wrong refusal hands something to a
// stranger. Three properties hold it together:
//
//   - It mints a SPECTATOR and nothing else. The caller does not choose a role,
//     so the worst a stranger with the link can do is watch. Promotion is a
//     separate, authorized decision.
//   - A CLOSED DOOR and a WRONG SECRET are refused IDENTICALLY: same status,
//     same body. Distinguishing them tells a prober which half they got right —
//     whether this campaign exists and is merely shut, or whether their guess
//     was wrong. identity.Verify sets the same precedent for tokens, and for
//     the same reason.
//   - It reuses CreateInvite unchanged, which is what keeps the presence
//     design's §3.1a true: a joiner who reconnects presents the same token,
//     resolves to the same participant id, and gets their characters back.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req joinRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJoinBody)).Decode(&req); err != nil {
		http.Error(w, "gateway: malformed join request", http.StatusBadRequest)
		return
	}

	// The name is checked FIRST and reported distinctly, because it is the
	// joiner's own mistake and telling them is not a leak: it says nothing
	// about the door or the secret.
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		http.Error(w, "gateway: a display name is required", http.StatusBadRequest)
		return
	}

	// Both conditions, then ONE refusal. Evaluated without short-circuiting on
	// the door, so a closed campaign and a wrong secret cost the same work as
	// well as returning the same answer.
	secret, err := s.ids.JoinSecret()
	okSecret := err == nil &&
		subtle.ConstantTimeCompare([]byte(secret), []byte(req.Secret)) == 1
	if !s.ids.JoinOpen() || !okSecret {
		// Deliberately the same status and text for every refusal above.
		http.Error(w, "gateway: this link is not accepting anyone", http.StatusForbidden)
		return
	}

	token, id, err := s.ids.CreateInvite(name, identity.RoleSpectator, nil)
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
