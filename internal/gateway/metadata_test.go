package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
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

	ids, err := identity.Open(path)
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

// TestMetadataRulesetGuideServedWhenLoaded is the ruleset guide's happy path;
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

// --- maps and packs (maps-as-geometry Task 7) -------------------------

// mapsFixture is deliberately separate from metaFixture: these routes need
// a REAL pack directory on disk (fs.FS-backed byte serving, traversal
// defence and content-type inference are the whole point under test), which
// metaFixture's adventures/ruleset setup has no reason to carry.
type mapsFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken, agentToken, playerToken, spectatorToken string
}

// newGatewayWithPack builds a server with one map ("shrine") and one REAL
// pack directory ("mossy-keep") — loaded through mapdef.LoadPack, not a
// hand-built literal, since the loader (not this test) owns what a valid
// Pack value looks like — containing pack.json plus one real file,
// planks_03.png, so a request for it has actual bytes to return.
func newGatewayWithPack(t *testing.T) *mapsFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	ids, err := identity.Open(path)
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

	packDir := filepath.Join(t.TempDir(), "tiles")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "pack.json"), []byte(`{
		"id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3", "file":"planks_03.png",
		           "kind":"floor", "material":"wood"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "planks_03.png"),
		[]byte("stand-in bytes; a real Content-Type is looked up from the allowlist by extension, not from this content"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An SVG file, deliberately: proves the allowlist EXCLUDES it (this
	// file's own doc comment on TestPackFileSVGIsNotServedAsImage) rather
	// than the fixture simply never exercising the case.
	if err := os.WriteFile(filepath.Join(packDir, "icon.svg"),
		[]byte("<svg><script>alert(1)</script></svg>"), 0o644); err != nil {
		t.Fatal(err)
	}

	pack, err := mapdef.LoadPack(packDir)
	if err != nil {
		t.Fatal(err)
	}

	maps := map[string]*mapdef.Map{
		"shrine": {ID: "shrine", Name: "Obsidian Shrine", GridW: 3, GridH: 3, Pack: "mossy-keep"},
	}
	packs := map[string]*mapdef.Pack{"mossy-keep": pack}
	// os.OpenRoot, matching production (cmd/vtt/maps.go) — NOT os.DirFS; see
	// WithPackFiles' doc comment for why that distinction is load-bearing
	// (a symlink escape, found by review, that os.DirFS does not stop).
	root, err := os.OpenRoot(packDir)
	if err != nil {
		t.Fatal(err)
	}
	packFS := map[string]fs.FS{"mossy-keep": root.FS()}

	srv := gateway.New(c, ids).WithMaps(maps, packs).WithPackFiles(packFS)
	f.srv = httptest.NewServer(srv.Handler())
	t.Cleanup(f.srv.Close)
	return f
}

// get issues a GET authenticated as the fixture's DM (every maps/packs
// route is open to every role per spec §7's "everyone still sees the whole
// map" — the role-table tests below cover that breadth explicitly; this
// helper exists for the tests that are not ABOUT roles).
func (f *mapsFixture) get(path string) (int, []byte) {
	return f.getAs(path, f.dmToken)
}

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

// TestPackImagesAreServedAndUnknownOnesAre404 is task-7-brief.md's own RED
// test: a real pack file 200s, an unknown pack 404s, and a traversal
// attempt over the real HTTP round trip does not escape the pack directory.
//
// HONEST NOTE on what this assertion actually proves (found while fault-
// injecting, recorded in task-7-report.md): a literal ".." in the URL never
// reaches handlePackFile at all here — net/http's own ServeMux redirects
// any request whose path contains a ".." element to the CLEANED path
// (verified directly: this exact request 307s to /api/etc/passwd, which
// matches no route and 404s) BEFORE pattern matching ever runs. So this
// test is real and worth keeping — a caller must still not observe a 200 —
// but it does not, by itself, exercise handlePackFile's OWN fs.FS defence;
// it is caught one layer up, by framework behaviour this package does not
// own or control. TestHandlePackFileRefusesTraversalEvenWithAPathValueSetDirectly
// (packfile_internal_test.go) is the assertion that actually isolates and
// proves the fs.FS-level defence the brief asked for, by handing
// handlePackFile a traversal string directly, bypassing ServeMux's own
// path-cleaning entirely.
func TestPackImagesAreServedAndUnknownOnesAre404(t *testing.T) {
	f := newGatewayWithPack(t)
	if code, body := f.get("/api/packs/mossy-keep/planks_03.png"); code != 200 {
		t.Fatalf("pack image returned %d: %s", code, body)
	}
	if code, _ := f.get("/api/packs/mossy-keep/../../etc/passwd"); code == 200 {
		t.Fatal("path traversal escaped the pack directory")
	}
	if code, _ := f.get("/api/packs/no-such-pack/x.png"); code != 404 {
		t.Fatalf("unknown pack returned %d, want 404", code)
	}
}

// TestPackFileUnknownWithinKnownPackIs404 covers the adjacent case the
// brief's own test does not: a KNOWN pack, but a file name that pack does
// not contain. Without this, a bug that served the whole packDir listing
// (or always returned 200) on any request under a valid pack id would slip
// past the "known pack, known file" and "unknown pack" cases alone.
func TestPackFileUnknownWithinKnownPackIs404(t *testing.T) {
	f := newGatewayWithPack(t)
	if code, body := f.get("/api/packs/mossy-keep/does-not-exist.png"); code != http.StatusNotFound {
		t.Fatalf("unknown file in a known pack: status = %d, want 404 (body %s)", code, body)
	}
}

// getFull issues an authenticated (DM) GET and returns the full response
// for header inspection; the caller closes the body.
func (f *mapsFixture) getFull(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+f.dmToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// TestPackFileAllowlistedExtensionGetsItsRealContentType pins this task's
// content-type decision (metadata.go's own package-doc section explains the
// reasoning): a .png is on the closed tile-art allowlist, so it is served
// as image/png, inline (no Content-Disposition), with nosniff set. This
// used to be plain extension INFERENCE (http.ServeFileFS/ServeContent, the
// same mechanism WithStatic uses for the client bundle) — review found that
// insufficient for third-party pack content and this is the corrected
// behaviour: an allowlist, not inference, even though the OUTCOME for a
// .png is unchanged.
func TestPackFileAllowlistedExtensionGetsItsRealContentType(t *testing.T) {
	f := newGatewayWithPack(t)
	resp := f.getFull(t, "/api/packs/mossy-keep/planks_03.png")
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := resp.Header.Get("Content-Disposition"); got != "" {
		t.Errorf("Content-Disposition = %q, want empty (allowlisted content serves inline)", got)
	}
}

// TestPackFileUnrecognizedExtensionIsOctetStreamAttachment pins the
// fallback half of the allowlist: pack.json is real, legitimate content
// this route serves (it sits in the same pack directory as the images,
// and a client fetching it programmatically does not care about
// Content-Type or Content-Disposition), but it is not TILE ART, so it gets
// application/octet-stream + an attachment disposition rather than any
// inference — proving the fallback is not merely theoretical, since a real,
// always-present file exercises it.
func TestPackFileUnrecognizedExtensionIsOctetStreamAttachment(t *testing.T) {
	f := newGatewayWithPack(t)
	resp := f.getFull(t, "/api/packs/mossy-keep/pack.json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want it to contain \"attachment\"", cd)
	}
}

// TestPackFileSVGIsNotServedAsImage pins the one deliberate exclusion this
// task's review specifically asked to be explicit about: SVG can embed
// <script>, so it does NOT get image/svg+xml or inline serving despite
// nominally being an image format — it is routed down the SAME
// octet-stream/attachment fallback as any other unrecognised extension.
// The fixture's icon.svg literally contains a <script> tag, so this test
// also proves the response never claims to be inline-renderable image
// content that a browser might choose to display anyway.
func TestPackFileSVGIsNotServedAsImage(t *testing.T) {
	f := newGatewayWithPack(t)
	resp := f.getFull(t, "/api/packs/mossy-keep/icon.svg")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "image/svg+xml" || strings.HasPrefix(ct, "image/") {
		t.Errorf("Content-Type = %q, want NOT an image/* type for .svg", ct)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want it to contain \"attachment\"", cd)
	}
}

// TestPackFilesRequireAuth pins that pack files sit behind the SAME Bearer-
// header gate every other /api route does (metadata.go's package doc:
// "every route it then calls is authenticated") — an installed pack is
// operator-trusted content (spec §4.2, "same trust as guide.md"), but that
// trust is about what an AUTHENTICATED caller may read, not about skipping
// authentication the way /join and the static bundle deliberately do.
func TestPackFilesRequireAuth(t *testing.T) {
	f := newGatewayWithPack(t)
	if code, _ := f.getAs("/api/packs/mossy-keep/planks_03.png", ""); code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", code)
	}
	if code, _ := f.getAs("/api/packs/mossy-keep/planks_03.png", "garbage"); code != http.StatusUnauthorized {
		t.Fatalf("bad token: status = %d, want 401", code)
	}
}

// TestPackFilesReadableByEveryRole pins spec §7's "everyone still sees the
// whole map. No filtering in this arc" — unlike an adventure guide (DM/agent
// only, DM secrets) or the join link (admission control), pack art carries
// neither, so every role that can authenticate at all can read it.
func TestPackFilesReadableByEveryRole(t *testing.T) {
	f := newGatewayWithPack(t)
	for _, tc := range []struct {
		role  string
		token string
	}{
		{"dm", f.dmToken},
		{"agent", f.agentToken},
		{"player", f.playerToken},
		{"spectator", f.spectatorToken},
	} {
		if code, body := f.getAs("/api/packs/mossy-keep/planks_03.png", tc.token); code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 (body %s)", tc.role, code, body)
		}
	}
}

// TestMapsListedForEveryRole pins /api/maps' shape and its role breadth
// (same reasoning as TestPackFilesReadableByEveryRole): id, name, grid
// dimensions, and the pack's own name/cellPx a client needs to draw at the
// right scale without a second request.
func TestMapsListedForEveryRole(t *testing.T) {
	f := newGatewayWithPack(t)
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
				Pack       *struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					CellPx int    `json:"cellPx"`
				} `json:"pack"`
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
		if m.Pack == nil || m.Pack.ID != "mossy-keep" || m.Pack.Name != "Mossy Keep" || m.Pack.CellPx != 64 {
			t.Errorf("%s: pack = %+v, want mossy-keep/Mossy Keep/64", tc.role, m.Pack)
		}
	}
}

// TestMapsEmptyCollectionWithNothingLoaded mirrors
// TestMetadataEmptyCollectionsWithNothingLoaded's "empty is not an error"
// posture (spec §5): a server booted without --maps-dir answers 200 with an
// empty list, not a 404 or a 500.
func TestMapsEmptyCollectionWithNothingLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(path)
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
