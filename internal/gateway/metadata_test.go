package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
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
		tok, _, err := ids.CreateInvite(name, role, nil)
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
		tok, id, err := f.ids.CreateInvite("Doomed", identity.RoleDM, nil)
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
