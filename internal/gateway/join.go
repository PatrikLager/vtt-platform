package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// maxJoinBody caps the request. The endpoint is unauthenticated, so it is the
// one place where an anonymous caller chooses how many bytes we read.
const maxJoinBody = 4 << 10

// maxDisplayNameRunes bounds the one string an UNAUTHENTICATED caller gets to
// put on everybody else's screen. Counted in RUNES, not bytes, so the bound is
// the same for a name written in Swedish, Japanese or emoji — a byte cap would
// quietly give non-ASCII names a third of the room.
const maxDisplayNameRunes = 64

// usableDisplayName reports whether name is fit to broadcast to the table.
//
// The joiner chooses this without authenticating, and it then travels in every
// PresenceSnapshot and PresenceChanged frame to every client. The client
// renders through textContent so this is not an XSS boundary — but "not XSS"
// is not the same as "bounded", and the frames also reach logs, the CLI and
// the MCP surface, none of which escape anything.
//
// Three classes are refused, each because it lets a stranger decide something
// about someone else's display rather than about their own name:
//
//   - EMPTY, including whitespace pretending not to be. It is what the whole
//     table sees; blank is not a name.
//   - CONTROL characters — a newline forges a second line, an ANSI escape
//     recolours a terminal, a NUL truncates whatever is not Go.
//   - BIDI controls, which are not "control characters" by Unicode's
//     definition but do the same job: U+202E renders the rest of a line
//     right-to-left, so a name can be made to read as somebody else's.
//   - Names with NOTHING VISIBLE IN THEM. A run of zero-width spaces passes
//     every rule above and survives TrimSpace, then renders as an empty gap
//     in everyone's participant list.
//
// Everything else is allowed on purpose. A bound that only accepted ASCII
// would lock out most of the people who might sit at a table.
func usableDisplayName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxDisplayNameRunes {
		return false
	}
	visible := false
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return false
		}
		// DEFAULT-IGNORABLE code points are the ones that render as nothing,
		// and Unicode's own categories will not tell you which those are:
		// U+3164 HANGUL FILLER is a LETTER (category Lo) and draws a blank —
		// which is precisely why it is the classic invisible-name trick. Cf
		// and whitespace round out the set. A name made only of these passes
		// every rule above and survives TrimSpace, then appears in everyone's
		// participant list as an empty gap: somebody at the table with no name.
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
	if !usableDisplayName(name) {
		http.Error(w, "gateway: a display name is required, up to 64 ordinary characters",
			http.StatusBadRequest)
		return
	}

	// ONE refusal, and identity owns both halves of the question — the door
	// and the secret come from a single read there, the comparison is
	// constant-time, and NOTHING IS WRITTEN. That last part is why this is not
	// the two calls it used to be: JoinSecret mints the row on a campaign that
	// has never had one, so a refused anonymous request performed an INSERT.
	// See identity.JoinAllows.
	//
	// An error is refused the same way rather than reported: a database that
	// cannot answer must not be able to open a door, and telling an anonymous
	// caller that this campaign's identity store is unwell is not information
	// they have any business having.
	// JoinAdmits, not JoinAllows: it SPENDS an admission from this opening's
	// budget as it answers, atomically, so two joiners racing for the last slot
	// cannot both get through (spec §2, amended 2026-08-11). A budget checked
	// separately from being spent is not a budget.
	allowed, err := s.ids.JoinAdmits(req.Secret)
	if err != nil || !allowed {
		// Deliberately the same status and text for every refusal here —
		// now covering THREE reasons, not two: a shut door, a wrong secret,
		// and an exhausted budget. A joiner turned away by a spent budget
		// learns nothing a prober could not, which is the point (spec §5); the
		// cost is that a legitimate Nth+1 player cannot tell why either, and
		// has to ask the DM. That cost is real and is recorded in the spec.
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
