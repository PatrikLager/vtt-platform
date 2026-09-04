package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// --- fixture ---------------------------------------------------------------

type metaFixture struct {
	t   *testing.T
	srv *httptest.Server
	ids *identity.DB

	dmToken, agentToken, playerToken, spectatorToken string
}

// newMetaFixture builds a server with a ruleset, two adventures and their
// guides — the fully-loaded shape the metadata routes describe. withContent
// false gives the opposite: a bare server, for the "nothing loaded" cases the
// spec says must answer honestly rather than error.
func newMetaFixture(t *testing.T, withContent bool) *metaFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	ids, err := identity.Open(campaign.LogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })

	mint := func(name string, role identity.Role) string {
		tok, _, err := ids.CreateInvite(name, role)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	f := &metaFixture{
		t:              t,
		ids:            ids,
		dmToken:        mint("DM", identity.RoleDM),
		agentToken:     mint("Agent", identity.RoleAgent),
		playerToken:    mint("Lera", identity.RolePlayer),
		spectatorToken: mint("Watcher", identity.RoleSpectator),
	}

	srv := gateway.New(c, ids)
	if withContent {
		rs := loadDnd45eMinimal(t)
		loaded := loadGoblinAmbush(t, rs)

		// A second entry so ORDER is observable. It is hand-built rather than
		// loaded, because every committed adventure belongs to a different
		// ruleset (cellar-rats is tavern-brawl's) and adventure.Load rightly
		// refuses to attach one to the wrong ruleset. These handlers read only
		// ID and Name, so a literal is a faithful stand-in and keeps the test
		// about the HANDLER rather than about adventure.Load, which has its
		// own tests. "aaa-" sorts before "goblin-ambush" on purpose: with the
		// map's iteration order randomized, an unsorted handler fails this.
		advs := map[string]*adventure.Adventure{
			loaded.ID:    loaded,
			"aaa-second": {ID: "aaa-second", Name: "Second Adventure"},
		}
		guides := map[string]string{
			loaded.ID:    "# guide for " + loaded.ID,
			"aaa-second": "# guide for aaa-second",
		}
		srv = srv.WithRuleset(rs).WithAdventures(advs).WithAdventureGuides(guides)
	}

	f.srv = httptest.NewServer(srv.Handler())
	t.Cleanup(f.srv.Close)
	return f
}

// get issues an authenticated GET. Auth is a Bearer HEADER, not ?token= —
// see metadata.go for why the WS precedent is deliberately not followed.
func (f *metaFixture) get(path, token string) (int, []byte) {
	f.t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.srv.URL+path, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, body
}

// --- the role table --------------------------------------------------------

// TestMetadataAdventureGuideRoleTable is written out BY HAND, one cell per
// role, exactly as authz_test.go's binding note requires: a table derived
// from adventureGuideRoles would assert only that the map equals itself, and
// would keep passing if someone widened the map.
//
// Adventure guides hold DM secrets (adventure/format.go). A player or
// spectator reading one is not a permissions nit — it is the table's whole
// reason to exist.
func TestMetadataAdventureGuideRoleTable(t *testing.T) {
	f := newMetaFixture(t, true)

	cases := []struct {
		role  string
		token string
		want  int
	}{
		{"dm", f.dmToken, http.StatusOK},
		{"agent", f.agentToken, http.StatusOK},
		{"player", f.playerToken, http.StatusForbidden},
		{"spectator", f.spectatorToken, http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			code, body := f.get("/api/adventures/goblin-ambush/guide", tc.token)
			if code != tc.want {
				t.Fatalf("%s: status = %d, want %d (body %s)", tc.role, code, tc.want, body)
			}
			if tc.want == http.StatusOK {
				var got struct {
					Guide string `json:"guide"`
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("decode: %v (body %s)", err, body)
				}
				if got.Guide == "" {
					t.Error("guide body is empty")
				}
			} else if len(body) > 0 && json.Valid(body) {
				var leak map[string]any
				_ = json.Unmarshal(body, &leak)
				if _, present := leak["guide"]; present {
					t.Errorf("%s was denied but the response still carried a guide field", tc.role)
				}
			}
		})
	}
}

// --- auth ------------------------------------------------------------------

func TestMetadataRejectsBadMissingAndRevokedTokens(t *testing.T) {
	f := newMetaFixture(t, true)

	t.Run("missing Authorization header", func(t *testing.T) {
		if code, _ := f.get("/api/ruleset", ""); code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", code)
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		if code, _ := f.get("/api/ruleset", "not-a-real-token"); code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", code)
		}
	})

	t.Run("revoked token", func(t *testing.T) {
		tok, id, err := f.ids.CreateInvite("Doomed", identity.RoleDM)
		if err != nil {
			t.Fatal(err)
		}
		if code, _ := f.get("/api/ruleset", tok); code != http.StatusOK {
			t.Fatalf("pre-revocation status = %d, want 200", code)
		}
		if err := f.ids.Revoke(id); err != nil {
			t.Fatal(err)
		}
		if code, _ := f.get("/api/ruleset", tok); code != http.StatusUnauthorized {
			t.Fatalf("post-revocation status = %d, want 401", code)
		}
	})
}

// --- empty-but-honest ------------------------------------------------------

// TestMetadataEmptyCollectionsWithNothingLoaded pins spec §5's "clean empty
// responses the UI renders honestly": a server with no ruleset answers 200
// with empty collections rather than 404 or 500, so the client shows an empty
// picker instead of an error banner.
func TestMetadataEmptyCollectionsWithNothingLoaded(t *testing.T) {
	f := newMetaFixture(t, false)

	code, body := f.get("/api/ruleset", f.dmToken)
	if code != http.StatusOK {
		t.Fatalf("/api/ruleset status = %d, want 200 (body %s)", code, body)
	}
	var rs struct {
		ID        string `json:"id"`
		Abilities []any  `json:"abilities"`
		Resources []any  `json:"resources"`
	}
	if err := json.Unmarshal(body, &rs); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if rs.Abilities == nil {
		t.Error("abilities must be [] and never null — the client iterates it directly")
	}
	if rs.Resources == nil {
		t.Error("resources must be [] and never null")
	}

	code, body = f.get("/api/adventures", f.dmToken)
	if code != http.StatusOK {
		t.Fatalf("/api/adventures status = %d, want 200", code)
	}
	var advs struct {
		Adventures []any `json:"adventures"`
	}
	if err := json.Unmarshal(body, &advs); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if advs.Adventures == nil {
		t.Error("adventures must be [] and never null")
	}

	// Guides are the exception: nothing to serve is a 404, not an empty body.
	if code, _ := f.get("/api/ruleset/guide", f.dmToken); code != http.StatusNotFound {
		t.Errorf("/api/ruleset/guide with no ruleset: status = %d, want 404", code)
	}
	if code, _ := f.get("/api/adventures/nope/guide", f.dmToken); code != http.StatusNotFound {
		t.Errorf("unknown adventure guide: status = %d, want 404", code)
	}
}

// --- content ---------------------------------------------------------------

// TestMetadataRulesetAbilitiesAreSortedById pins determinism at the boundary.
// Ruleset.Compiled is a Go map, and Go randomizes map iteration, so an
// unsorted handler returns a different ability order on every request — the
// client's picker would reshuffle under the user's cursor between polls.
func TestMetadataRulesetAbilitiesAreSortedById(t *testing.T) {
	f := newMetaFixture(t, true)

	var first []string
	for i := 0; i < 5; i++ {
		code, body := f.get("/api/ruleset", f.dmToken)
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		var got struct {
			Abilities []struct {
				ID string `json:"id"`
			} `json:"abilities"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		ids := make([]string, len(got.Abilities))
		for j, a := range got.Abilities {
			ids[j] = a.ID
		}
		if len(ids) < 2 {
			t.Fatalf("fixture ruleset has %d abilities; need at least 2 to detect order", len(ids))
		}
		for j := 1; j < len(ids); j++ {
			if ids[j-1] > ids[j] {
				t.Fatalf("abilities not sorted by id: %v", ids)
			}
		}
		if i == 0 {
			first = ids
			continue
		}
		if fmt.Sprint(ids) != fmt.Sprint(first) {
			t.Fatalf("ability order differs between requests:\n %v\n %v", first, ids)
		}
	}
}

// TestMetadataAdventuresListedForEveryRole pins that the LIST is public to
// all four roles even though the GUIDES are not — a player must be able to
// see which adventures exist without reading the DM's secrets.
func TestMetadataAdventuresListedForEveryRole(t *testing.T) {
	f := newMetaFixture(t, true)

	for _, tc := range []struct {
		role  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		code, body := f.get("/api/adventures", tc.token)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", tc.role, code)
		}
		var got struct {
			Adventures []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"adventures"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: decode: %v", tc.role, err)
		}
		if len(got.Adventures) != 2 {
			t.Fatalf("%s: got %d adventures, want 2", tc.role, len(got.Adventures))
		}
		for i := 1; i < len(got.Adventures); i++ {
			if got.Adventures[i-1].ID > got.Adventures[i].ID {
				t.Errorf("%s: adventures not sorted by id: %+v", tc.role, got.Adventures)
			}
		}
	}
}

// TestMetadataRulesetGuideServedForEveryRole is the ruleset guide's happy path;
// unlike an adventure guide it carries no DM secrets and is open to all four
// roles (the LLM affordance every client may read).
func TestMetadataRulesetGuideServedForEveryRole(t *testing.T) {
	f := newMetaFixture(t, true)

	for _, tc := range []struct {
		role  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		code, body := f.get("/api/ruleset/guide", tc.token)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200 (body %s)", tc.role, code, body)
		}
		var got struct {
			Guide string `json:"guide"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: decode: %v", tc.role, err)
		}
		if got.Guide == "" {
			t.Errorf("%s: guide is empty", tc.role)
		}
	}
}

// TestMetadataRulesetShapeMatchesTheContract pins the response fields the
// client is written against, including usage — the picker greys out an
// ability whose resource is spent, and cannot do that without cost.
func TestMetadataRulesetShapeMatchesTheContract(t *testing.T) {
	f := newMetaFixture(t, true)

	code, body := f.get("/api/ruleset", f.dmToken)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	var got struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Abilities []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Range      int    `json:"range"`
			MaxTargets int    `json:"maxTargets"`
			Usage      struct {
				Kind     string `json:"kind"`
				Resource string `json:"resource"`
				Cost     int    `json:"cost"`
			} `json:"usage"`
		} `json:"abilities"`
		Conditions []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"conditions"`
		Resources []string `json:"resources"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if got.ID == "" || got.Name == "" {
		t.Error("ruleset id/name must be populated")
	}
	if len(got.Abilities) == 0 {
		t.Fatal("no abilities returned")
	}
	for _, a := range got.Abilities {
		if a.ID == "" {
			t.Error("ability with empty id")
		}
		switch a.Usage.Kind {
		case "atWill":
		case "resource":
			if a.Usage.Resource == "" {
				t.Errorf("ability %q: resource usage with no resource named", a.ID)
			}
		default:
			t.Errorf("ability %q: usage kind %q is neither atWill nor resource", a.ID, a.Usage.Kind)
		}
	}
	if len(got.Resources) == 0 {
		t.Error("ruleset declares resources; the list must not be empty")
	}

	// Conditions are sorted too, and for the same reason abilities are:
	// rs.Conditions is a map, so an unsorted handler reorders the list on
	// every request. Nothing asserted this until a surviving mutant showed
	// that reversing the condition comparator changed no test's outcome.
	if len(got.Conditions) < 2 {
		t.Fatalf("fixture ruleset has %d conditions; need at least 2 to detect order",
			len(got.Conditions))
	}
	for i := 1; i < len(got.Conditions); i++ {
		if got.Conditions[i-1].ID > got.Conditions[i].ID {
			t.Errorf("conditions not sorted by id: %+v", got.Conditions)
			break
		}
	}
}

// TestMetadataMeIdentifiesTheCaller pins /api/me, which T7's player UI cannot
// work without: "which actors do I control" is an equality check against
// participantId, and the role decides which panels exist at all. Inferring
// either from the event stream would be guesswork — a spectator who has
// caused no events looks exactly like a player who has not acted yet.
func TestMetadataMeIdentifiesTheCaller(t *testing.T) {
	f := newMetaFixture(t, true)

	for _, tc := range []struct {
		role  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		code, body := f.get("/api/me", tc.token)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", tc.role, code)
		}
		var got struct {
			ParticipantID string `json:"participantId"`
			Name          string `json:"name"`
			Role          string `json:"role"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: decode: %v (body %s)", tc.role, err, body)
		}
		if got.Role != tc.role {
			t.Errorf("role = %q, want %q", got.Role, tc.role)
		}
		if got.ParticipantID == "" {
			t.Errorf("%s: participantId is empty; the client cannot match controllerIds without it", tc.role)
		}
		// The display name, asserted since the controls block that used to sit
		// here went: it is the OTHER field this route carries, and without an
		// assertion on it the handler could drop it and stay green.
		if got.Name == "" {
			t.Errorf("%s: name is empty; presence labels and the DM roster both render it", tc.role)
		}
	}

	if code, _ := f.get("/api/me", "garbage"); code != http.StatusUnauthorized {
		t.Errorf("/api/me with a bad token: status = %d, want 401", code)
	}
}

// TestMeSaysWhoYouAreAndNeverWhatYouControl is the deletion, pinned.
//
// Control is a fact about the LOG: Actor.controller_ids, written by
// ActorControlGranted and read by authz.go's controls() and by eyes()'s player
// arm. (The party roster and MayPerch used to read it too and no longer do —
// 80dfa0e on this branch moved them onto Actor.kind, and since the migration
// arm was deleted on 2026-08-24 isPartyMember does not touch controller_ids at
// all.) /api/me used to answer
// control a second time from a SQLite column nothing ever updated, so the
// answer was a plausible-looking lie: a DM who invited somebody "controlling
// Hollis" was told by this route that they controlled Hollis, while every rule
// that decides anything said they did not.
//
// The assertion is on the KEY, not on its value, and that is deliberate. An
// empty list is the shape a wrong answer takes when nobody has been granted
// anything yet, so a test that accepted `"controls": []` would pass against
// exactly the code this deletes. Decoding into a map is what makes absence
// observable at all — a struct field would read the same for "absent" and
// "present and empty".
func TestMeSaysWhoYouAreAndNeverWhatYouControl(t *testing.T) {
	f := newMetaFixture(t, true)

	for _, tc := range []struct {
		role  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		code, body := f.get("/api/me", tc.token)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", tc.role, code)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: decode: %v (body %s)", tc.role, err, body)
		}
		if v, ok := got["controls"]; ok {
			t.Errorf("%s: /api/me still answers what you control (%v) — the only "+
				"authority on that is Actor.controller_ids in the log, and a second "+
				"answer here is a claim no grant backs", tc.role, v)
		}
		// The route still has to do its own job, or "no controls key" would be
		// satisfied by a route that answered nothing at all.
		if got["role"] != tc.role {
			t.Errorf("%s: role = %v, want %q", tc.role, got["role"], tc.role)
		}
		if got["participantId"] == "" || got["participantId"] == nil {
			t.Errorf("%s: participantId is missing; the client cannot match "+
				"controllerIds without it", tc.role)
		}
	}
}

// TestJoinLinkIsDMOnlyAndLiteralPerRole guards a route that hands out a
// SHARED SECRET.
//
// Everything else behind /api is readable by whoever holds a credential — a
// player may read the ruleset, a spectator may list adventures. This one is
// different in kind: the secret it returns admits ANYBODY who has it, so a
// spectator who could read it could hand the table to strangers, and the
// spectator default (spec §2) would be decoration.
//
// Written as four literal rows rather than derived from joinLinkRoles, for the
// reason authz_test.go's binding note gives: a table built from the map would
// assert only that the map equals itself, and would keep passing if somebody
// widened it.
func TestJoinLinkIsDMOnlyAndLiteralPerRole(t *testing.T) {
	f := newMetaFixture(t, true)

	for _, tc := range []struct {
		role  string
		token string
		want  int
	}{
		{"dm", f.dmToken, http.StatusOK},
		{"agent", f.agentToken, http.StatusOK},
		{"player", f.playerToken, http.StatusForbidden},
		{"spectator", f.spectatorToken, http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			code, body := f.get("/api/join-link", tc.token)
			if code != tc.want {
				t.Fatalf("%s: status = %d, want %d (body %s)", tc.role, code, tc.want, body)
			}
			if tc.want != http.StatusOK {
				// And the refusal does not carry the SECRET ITSELF. Searching
				// the body for the word "secret" tests the JSON field name, not
				// the value — it fires on a missing `return` after http.Error
				// (worth keeping) but not on a refusal that leaked the value by
				// any other shape.
				live, err := f.ids.JoinSecret()
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(body), live) {
					t.Fatalf("%s: a refusal carried the live join secret: %s", tc.role, body)
				}
				if strings.Contains(string(body), "secret") {
					t.Fatalf("%s: a refusal must not carry the link: %s", tc.role, body)
				}
			}
		})
	}
}

func TestJoinLinkReportsTheDoorAndTheSecret(t *testing.T) {
	// The DM console cannot read identity's SQLite, so this route is the only
	// way the browser learns whether the door is open or what to share. Both
	// halves are asserted: a console that showed the link but not the door
	// would have a DM confidently sending out a link that admits nobody.
	f := newMetaFixture(t, true)

	code, body := f.get("/api/join-link", f.dmToken)
	if code != http.StatusOK {
		t.Fatalf("status = %d: %s", code, body)
	}
	var got struct {
		Open   bool   `json:"open"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if got.Open {
		t.Fatal("a campaign must report its door CLOSED until somebody opens it")
	}
	if got.Secret == "" {
		t.Fatal("the DM must be able to see the link before opening the door — otherwise " +
			"the only order of operations is open-then-look, which is a window with the " +
			"door open and nobody told where to go")
	}

	if err := f.ids.SetJoinOpen(true, 100); err != nil {
		t.Fatal(err)
	}
	_, body = f.get("/api/join-link", f.dmToken)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Open {
		t.Fatal("opening the door must be visible here — this is the console's only mirror")
	}
}

// TestParticipantsIsDMOnlyAndLiteralPerRole gates the roster.
//
// Reusing joinLinkRoles would be wrong even though the values match today: who
// may see a shared SECRET and who may see the table's roster are two
// questions, and one map answering both means widening either widens the
// other. The literal rows below are the whole point (authz_test.go's binding
// note).
func TestParticipantsIsDMOnlyAndLiteralPerRole(t *testing.T) {
	f := newMetaFixture(t, true)

	for _, tc := range []struct {
		role  string
		token string
		want  int
	}{
		{"dm", f.dmToken, http.StatusOK},
		{"agent", f.agentToken, http.StatusOK},
		{"player", f.playerToken, http.StatusForbidden},
		{"spectator", f.spectatorToken, http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			code, body := f.get("/api/participants", tc.token)
			if code != tc.want {
				t.Fatalf("%s: status = %d, want %d (body %s)", tc.role, code, tc.want, body)
			}
		})
	}
}

func TestParticipantsNamesEveryoneAndTheirRole(t *testing.T) {
	// What the promote control is built on: the DM has to be able to see WHO
	// is only watching. A roster without roles would make promotion a guess.
	//
	// And it must never carry a token or a hash — the roster is a list of
	// people, not of credentials.
	f := newMetaFixture(t, true)

	code, body := f.get("/api/participants", f.dmToken)
	if code != http.StatusOK {
		t.Fatalf("status = %d: %s", code, body)
	}
	var got []struct {
		ParticipantID string `json:"participantId"`
		Name          string `json:"name"`
		Role          string `json:"role"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}

	roles := map[string]string{}
	for _, p := range got {
		if p.ParticipantID == "" {
			t.Fatalf("a participant with no id cannot be promoted: %+v", p)
		}
		roles[p.Name] = p.Role
	}
	for name, want := range map[string]string{"DM": "dm", "Agent": "agent", "Lera": "player", "Watcher": "spectator"} {
		if roles[name] != want {
			t.Fatalf("%s has role %q, want %q (roles: %v)", name, roles[name], want, roles)
		}
	}
	if strings.Contains(string(body), "token") || strings.Contains(string(body), "hash") {
		t.Fatalf("the roster must not carry credentials: %s", body)
	}
}

// --- maps (maps-as-geometry Task 7) -----------------------------------
//
// SIX TESTS AND A PACK DIRECTORY STOOD HERE, all of them about
// GET /api/packs/{pack}/{file}, which 2026-09-02-art-is-a-flat-library Task 7
// deleted with the pack. Each pinned a property, and every one of those
// properties belongs to GET /api/art/{file} the day Task 6 of that plan builds
// it — the ruling itself is written down in metadata.go's own doc section so it
// is not carried only by tests that no longer exist:
//
//   - TestPackImagesAreServedAndUnknownOnesAre404: a real file 200s, an unknown
//     id 404s, and a traversal over the real round trip does not escape.
//   - TestPackFileUnknownWithinKnownPackIs404: a known directory, a file it
//     does not contain — without it, a handler that served a listing or always
//     200'd would pass the two cases above.
//   - TestPackFileAllowlistedExtensionGetsItsRealContentType and
//     TestPackFileUnrecognizedExtensionIsOctetStreamAttachment: the closed
//     allowlist and the octet-stream/attachment fallback, both with nosniff.
//   - TestPackFileSVGIsNotServedAsImage: the one deliberate exclusion, because
//     an SVG can embed <script> and a same-origin script can read this client's
//     Bearer token out of localStorage.
//   - TestPackFilesRequireAuth: the Bearer gate. This one IS replaced, by
//     TestNoPackRouteIsServed below, which is only able to tell a deleted route
//     from a live one BECAUSE that gate answered 401 before anything else.
//   - TestPackFilesReadableByEveryRole: spec §7's role breadth. The /api/maps
//     half of that survives in TestMapsListedForEveryRole below.
//
// Two more went with internal/gateway/packfile_internal_test.go, which was the
// whole file: the ".." refusal isolated from ServeMux's own redirect, and the
// symlink escape that only os.OpenRoot (never os.DirFS) stops. internal/artlib
// still pins the symlink half at the LOOKUP layer
// (TestLookupWillNotFollowASymlinkOutOfTheArtDirectory); nothing pins it at a
// ROUTE, because there is no route serving bytes any more.
//
// NOTHING REGRESSES BY DELETING THEM — the surface they guarded is gone, and
// no client can fetch campaign art at all until Task 6. What WOULD regress is
// Task 6 shipping that route without re-deriving this list.

// mapsFixture is deliberately separate from metaFixture: /api/maps needs a
// server holding real maps, which metaFixture's adventures/ruleset setup has
// no reason to carry.
type mapsFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken, agentToken, playerToken, spectatorToken string
}

// newGatewayWithMaps builds a server holding one map ("shrine") and four
// tokens, one per role. It built a REAL pack directory beside it until
// 2026-09-02-art-is-a-flat-library Task 7; nothing is wired for art now,
// because nothing serves art bytes until Task 6 of that plan.
//
// NO STATIC BUNDLE IS WIRED, and TestNoPackRouteIsServed depends on that:
// server.go registers http.FileServerFS at "/" only when one is present, so
// with none, a path no route carries reaches nothing at all and ServeMux
// answers 404 by itself.
func newGatewayWithMaps(t *testing.T) *mapsFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	ids, err := identity.Open(campaign.LogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })

	mint := func(name string, role identity.Role) string {
		tok, _, err := ids.CreateInvite(name, role)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	f := &mapsFixture{
		t:              t,
		dmToken:        mint("DM", identity.RoleDM),
		agentToken:     mint("Agent", identity.RoleAgent),
		playerToken:    mint("Lera", identity.RolePlayer),
		spectatorToken: mint("Watcher", identity.RoleSpectator),
	}

	maps := map[string]*mapdef.Map{
		"shrine": {ID: "shrine", Name: "Obsidian Shrine", GridW: 3, GridH: 3},
	}
	srv := gateway.New(c, ids).WithMaps(maps)
	f.srv = httptest.NewServer(srv.Handler())
	t.Cleanup(f.srv.Close)
	return f
}

// mapsFixture.get is gone, with the pack tests that were its only callers
// (2026-09-02-art-is-a-flat-library Task 7). It issued a GET as the fixture's
// DM, for tests that were not ABOUT roles; everything left here is about roles
// or about a route's absence, and both name their token at the call site.

func (f *mapsFixture) getAs(path, token string) (int, []byte) {
	f.t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.srv.URL+path, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, body
}

// TestNoPackRouteIsServed asserts an ABSENCE, so it was written before the
// removal and failed until 2026-09-02-art-is-a-flat-library Task 7 landed it —
// the same shape client/test/command-surface.test.ts uses for create_scene and
// for retraction. What it does now is keep the route from coming back.
//
// THE ABSENCE IS ONLY OBSERVABLE WITHOUT A TOKEN, and that is the whole design
// of this test. Authenticated, a route that does not exist and a route whose
// pack id is unknown both answer 404, so an authenticated probe would have
// passed for as long as the route existed. Unauthenticated, the two differ:
// handlePackFile called s.authed FIRST and answered 401 before looking at any
// pack (TestPackFilesRequireAuth pinned exactly that, and is the test this one
// replaces), while net/http's ServeMux answers 404 for a pattern it does not
// carry. A 401 here means the route is still registered.
//
// The fixture wires no static bundle, so there is no "/" catch-all to answer
// instead — see newGatewayWithMaps.
//
// WHAT THIS TEST CANNOT SEE, and it is the more dangerous of the two
// resurrections (review, 2026-09-04). Its entire signal is
// 404-rather-than-401, and that signal exists ONLY because handlePackFile
// gated on s.authed before touching anything. A route brought back WITHOUT an
// auth gate answers 404 for an unknown pack id exactly as an absent route
// does, and this test passes. So it catches the route returning in the shape
// it left in; it does not catch the route returning in a worse one, and it is
// not a general guard against pack code reappearing in this package.
//
// tools/check-no-pack.py (Task 9 of 2026-09-02-art-is-a-flat-library) is the
// instrument for that — it reads code positions across the whole tree — and
// internal/mapdef's own TestNoPackTypeOrLoaderRemainsInThisPackage is the
// package-scoped version of the same idea. This test is deliberately NOT
// grown into either: what it is for is the one property those two cannot
// assert, which is what a running server actually answers.
func TestNoPackRouteIsServed(t *testing.T) {
	f := newGatewayWithMaps(t)
	if code, body := f.getAs("/api/packs/mossy-keep/planks_03.png", ""); code != http.StatusNotFound {
		t.Fatalf("unauthenticated GET /api/packs/{pack}/{file} = %d (body %s), want 404 — "+
			"401 means the route is still registered and only its auth check answered",
			code, body)
	}
}

// TestMapsListedForEveryRole pins /api/maps' shape and its role breadth
// (spec §7, "everyone still sees the whole map. No filtering in this arc" —
// unlike an adventure guide, DM/agent only, or the join link, admission
// control, a map's geometry carries neither secret): id, name and grid
// dimensions, AND THE ABSENCE of a pack reference.
//
// It said the entry carried "the pack's own name/cellPx a client needs to draw
// at the right scale without a second request" — and that sentence was wrong in
// both halves by the time it was read. The entry carries no pack at all since
// Task 5 of 2026-09-02-art-is-a-flat-library deleted mapdef.Map.Pack, which is
// what the second half of this test now asserts; and no client ever read
// cellPx to draw with — client/src/view/spectator.ts's CELL = 44 is the only
// cell size in the renderer, and metadata.ts merely declared the field. Task 6
// of that plan is what gives a campaign-level cellPx its first reader.
func TestMapsListedForEveryRole(t *testing.T) {
	f := newGatewayWithMaps(t)
	for _, tc := range []struct {
		role  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		code, body := f.getAs("/api/maps", tc.token)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200 (body %s)", tc.role, code, body)
		}
		var got struct {
			Maps []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				GridWidth  int    `json:"gridWidth"`
				GridHeight int    `json:"gridHeight"`
			} `json:"maps"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: decode: %v (body %s)", tc.role, err, body)
		}
		if len(got.Maps) != 1 {
			t.Fatalf("%s: got %d maps, want 1", tc.role, len(got.Maps))
		}
		m := got.Maps[0]
		if m.ID != "shrine" || m.Name != "Obsidian Shrine" || m.GridWidth != 3 || m.GridHeight != 3 {
			t.Errorf("%s: map = %+v, want shrine/Obsidian Shrine/3x3", tc.role, m)
		}

		// THE PACK REFERENCE IS GONE, and its absence is what this half now
		// pins — it used to assert mossy-keep/Mossy Keep/64 rode along on
		// every entry. A map has no pack to name since Task 5 of
		// 2026-09-02-art-is-a-flat-library deleted mapdef.Map.Pack, so a
		// "pack" key here could only be a leftover claiming an association
		// nothing can establish. Asserted on the RAW object rather than
		// through a typed decode, because a struct with no Pack field would
		// pass whether the server sent one or not — encoding/json discards
		// what it has nowhere to put, so the typed shape above cannot tell
		// the deletion from a fixture that happens not to exercise it.
		var raw struct {
			Maps []map[string]any `json:"maps"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("%s: decode raw: %v (body %s)", tc.role, err, body)
		}
		if _, present := raw.Maps[0]["pack"]; present {
			t.Errorf("%s: entry carries a pack reference (%v); no map declares a pack any more",
				tc.role, raw.Maps[0]["pack"])
		}
	}
}

// TestMapsEmptyCollectionWithNothingLoaded mirrors
// TestMetadataEmptyCollectionsWithNothingLoaded's "empty is not an error"
// posture (spec §5): a server booted for a campaign whose maps/ is absent,
// or which has no maps installed yet (2026-09-01-create-scene-leaves Task
// 5), answers 200 with an empty list, not a 404 or a 500.
func TestMapsEmptyCollectionWithNothingLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(campaign.LogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/maps", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Maps []any `json:"maps"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if got.Maps == nil {
		t.Error("maps must be [] and never null — the client iterates it directly")
	}
}
