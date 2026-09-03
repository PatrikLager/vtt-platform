package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// errAdventuresRequireRuleset is composeServer's boot-time flag error when
// adventuresDir is set but rulesetDir is not (adventure-format Task 4,
// binding, mirroring the MCP flag precedent — cmd/vtt/mcp.go requires the
// same pairing for get_adventure_guide): every adventure declares the
// ruleset id it was written for, and Load validates that declaration
// against "the served ruleset" (spec §7, "Load-time validation"). The
// pairing requirement is unchanged; the phrase this used to cite — "the dir
// is for THIS table" — lived in the PLAN, not the spec, and is the half of
// that binding retired on 2026-08-06 when loadAdventuresDir began selecting
// by ruleset rather than refusing to boot. With
// no ruleset configured for serve at all, there is no served ruleset to
// validate against, so the pairing is required rather than silently
// skipping validation.
const errAdventuresRequireRuleset = "vtt serve: --adventures-dir requires --ruleset (adventures load+validate against the served ruleset)"

// composeServer opens the campaign and identity handles for campaignPath —
// a campaign DIRECTORY since Task 4 (2026-09-01-create-scene-leaves §3),
// not a bare log file — and wires them into a gateway.Server's Handler on
// an *http.Server bound to addr (not yet listening — the caller starts it,
// e.g. via ListenAndServe or, for tests that need the assigned port, its
// own net.Listener + Serve).
//
// The returned close func closes both handles (identity first, then
// campaign). CAUTION: srv.Shutdown returning does NOT by itself guarantee
// it is safe to call closeFn. http.Server.Shutdown gracefully closes idle
// listeners and connections and waits for active HTTP handlers to return,
// but it does NOT wait for connections that have been hijacked out of
// HTTP's request/response cycle — which is exactly what every WebSocket
// connection is (coder/websocket hijacks the net.Conn on upgrade). A live
// WS connection can still be reading/writing against campaign/identity
// after Shutdown has returned, so closeFn is only actually safe once every
// gateway connection has itself finished closing.
//
// Today, closeFn's callers are responsible for that guarantee themselves:
// the composeServer e2e test (serve_e2e_test.go) explicitly closes its one
// WS connection before calling Shutdown, so no live connection remains by
// the time closeFn runs. `vtt serve` (serve.go) now has a SIGINT/SIGTERM
// shutdown path (RunE watches cmd.Context().Done()), but it does not close
// this gap either — it calls srv.Shutdown then srv.Close as a best-effort
// bound on wall-clock time, then runs closeFn regardless of whether any WS
// connection is still actually mid-teardown, so a connection racing the
// signal can still observe campaign/identity closing under it. Making this
// safe unconditionally — draining/closing every open gateway connection as
// part of shutdown, rather than trusting the caller (or a timeout) — is a
// ledgered carry-forward (see .superpowers/sdd/progress.md), not solved by
// this comment.
// rulesetDir is OPTIONAL (ruleset-interpreter Task 6, spec §7): "" keeps
// every pre-Task-6 behavior exactly as it was — a nil gateway.Server
// ruleset, use_ability commands rejected with a clean "no ruleset loaded"
// CommandResult. A non-empty rulesetDir is loaded via rules.Load (fails
// loud here, at boot, closing both handles before returning — the same
// fail-loud-at-open posture composeServer already gives a bad
// campaign/identity path) and wired in via gateway.Server.WithRuleset.
//
// adventuresDir is OPTIONAL (adventure-format Task 4, spec §7): "" keeps
// every pre-Task-4 behavior exactly as it was — a nil/empty
// gateway.Server.adventures, load_adventure commands rejected with a clean
// "no adventures available" CommandResult. A non-empty adventuresDir
// REQUIRES a non-empty rulesetDir too (errAdventuresRequireRuleset — every
// adventure declares the ruleset id it was written for, and there is no
// "the served ruleset" to validate it against otherwise); every immediate
// subdirectory of adventuresDir is loaded and validated against the served
// ruleset via loadAdventuresDir (adventures.go) — fail loud here, at boot,
// on ANY single adventure's failure (spec §7: "All available adventures
// load+validate at BOOT... fail loud at startup, not at the table"),
// closing both handles before returning, exactly like a bad rulesetDir
// above. A mismatched adventure (one declaring a different ruleset id than
// rulesetDir) is caught by adventure.Load itself (its own ruleset-id-match
// check) and surfaces as this same boot error.
//
// Maps come from the campaign directory itself (2026-09-01-create-scene-
// leaves Task 5 — "the kernel serves maps, it does not make them"): there
// is no mapsDir parameter and no --maps-dir flag any more, because a map
// belongs to the campaign that uses it (design spec §3), not to a
// server-wide operator flag pointing at a shared store. campaignPath/maps
// ABSENT is not a declaration of anything — a brand-new campaign starts
// with nothing installed (design spec §4, "Install, then load"), and
// treating that as a boot failure would stop `vtt client run`'s
// self-contained throwaway campaign (harness_boot.go) from ever starting —
// so it is treated exactly like the old mapsDir=="" case: a nil/empty
// gateway.Server.maps, GET /api/maps answering 200 with an empty list and
// GET /api/packs/{pack}/{file} always 404ing. campaignPath/maps PRESENT
// (even placed there by nothing more than an empty mkdir) is loaded and
// validated in full via loadMapsDir (maps.go; layout changed by Task 3 of
// the 2026-09-01 create_scene-leaves plan — maps are flat files, packs are
// a sibling tree) — fail loud here, at boot, on any single map's failure,
// an override naming art that is installed and cannot be read, or an
// existing-but-empty maps/ (the same "fail loud, never at the table" posture
// as adventuresDir above), closing both handles before returning. An override
// naming art that is simply NOT installed is not a boot failure since
// 2026-09-02-art-is-a-flat-library Task 3: it degrades that one square and
// warns (spec §4).
//
// campaignPath/art IS CHECKED HERE, unconditionally, before the maps guard —
// see the call site for why "unconditionally" is the whole of it. NO ART
// DIRECTORY IS HANDED TO THE SERVER YET, though, so a map loaded through
// load_map resolves no art at all and every override degrades. Task 4 of that
// plan calls gateway.Server.WithArtDir(campaignPath/art) here and runs
// artlib.Validate once at this point, which subsumes the check below. Until it
// does, this boot walk resolves against campaignPath/art and the request path
// does not — strict at boot and lenient on reload, which is the harmless
// direction of design spec §12's divergence.
//
// The maps DIRECTORY is then handed to the server unconditionally
// (WithMapsDir), present or not, which is what makes install-then-load work
// during a session rather than only across restarts (Task 6 of the same
// plan, design spec §5): on a load_map miss the server probes
// campaignPath/maps/<id>.json through the same mapdef.LoadInstalled this
// boot walk uses. Boot preloading is unchanged — an operator still learns
// about a broken map before anyone connects — and what is added is only the
// map that was not there yet.
func composeServer(campaignPath, addr, rulesetDir, adventuresDir string) (*http.Server, func() error, error) {
	c, err := campaign.Open(campaignPath)
	if err != nil {
		return nil, nil, fmt.Errorf("vtt serve: open campaign: %w", err)
	}

	// identity.Open opens its own SQLite handle on "the same campaign file
	// the store uses" (internal/identity's package comment) — since Task 4
	// (2026-09-01-create-scene-leaves §3) that file is campaign.LogPath's
	// log.db INSIDE campaignPath, not campaignPath itself: campaignPath is
	// now the campaign DIRECTORY, and campaign.Open above has already
	// created it (MkdirAll) by the time this call runs.
	ids, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		_ = c.Close() // best-effort; the compose error below is what matters
		return nil, nil, fmt.Errorf("vtt serve: open identity: %w", err)
	}

	gw := gateway.New(c, ids)
	var rs *rules.Ruleset
	if rulesetDir != "" {
		rs, err = rules.Load(rulesetDir)
		if err != nil {
			_ = ids.Close() // best-effort; the compose error below is what matters
			_ = c.Close()   // best-effort; the compose error below is what matters
			return nil, nil, fmt.Errorf("vtt serve: load ruleset %s: %w", rulesetDir, err)
		}
		gw = gw.WithRuleset(rs)
	}

	if adventuresDir != "" {
		if rs == nil {
			_ = ids.Close() // best-effort; the compose error below is what matters
			_ = c.Close()   // best-effort; the compose error below is what matters
			return nil, nil, errors.New(errAdventuresRequireRuleset)
		}
		advs, err := loadAdventuresDir(adventuresDir, rs)
		if err != nil {
			_ = ids.Close() // best-effort; the compose error below is what matters
			_ = c.Close()   // best-effort; the compose error below is what matters
			return nil, nil, fmt.Errorf("vtt serve: load adventures %s: %w", adventuresDir, err)
		}
		// Guides are read HERE, at boot, not per request: cmd/vtt owns the
		// filesystem (ADR-008), and an unreadable guide should fail loudly at
		// startup rather than becoming a 500 in the middle of a session.
		guides, err := loadAdventureGuides(advs)
		if err != nil {
			_ = ids.Close() // best-effort; the compose error below is what matters
			_ = c.Close()   // best-effort; the compose error below is what matters
			return nil, nil, fmt.Errorf("vtt serve: load adventure guides %s: %w", adventuresDir, err)
		}
		gw = gw.WithAdventures(advs).WithAdventureGuides(guides)
	}

	// BEFORE the maps guard below and OUTSIDE it, for the reason WithMapsDir
	// is outside it: a campaign with art and no map yet is the improvisation
	// case this whole line of work exists for, and a check that only runs when
	// maps/ happens to exist is the boot-order defect of design spec §1 rebuilt
	// in a new directory. An absent art/ passes; art/ present and unopenable
	// stops the boot, where the operator who can fix it is looking
	// (artRootIsOpenable, maps.go, and mapdef.Resolve's own doc comment for the
	// request-time half that degrades instead).
	if err := artRootIsOpenable(filepath.Join(campaignPath, "art")); err != nil {
		_ = ids.Close() // best-effort; the compose error below is what matters
		_ = c.Close()   // best-effort; the compose error below is what matters
		return nil, nil, fmt.Errorf("vtt serve: %w", err)
	}

	// campaignPath/maps ABSENT means nothing has been installed yet (see
	// this function's own doc comment above) — skip loading entirely,
	// exactly like the old mapsDir=="" case. Any OTHER Stat failure
	// (permissions, a plain file sitting where maps/ should be) falls
	// through to loadMapsDir so ITS error surfaces, rather than being
	// silently swallowed here as "no maps".
	mapsDir := filepath.Join(campaignPath, "maps")
	if _, statErr := os.Stat(mapsDir); statErr == nil || !os.IsNotExist(statErr) {
		maps, packs, packFS, err := loadMapsDir(campaignPath)
		if err != nil {
			_ = ids.Close() // best-effort; the compose error below is what matters
			_ = c.Close()   // best-effort; the compose error below is what matters
			return nil, nil, fmt.Errorf("vtt serve: load maps %s: %w", campaignPath, err)
		}
		gw = gw.WithMaps(maps, packs).WithPackFiles(packFS)
	}
	// UNCONDITIONALLY, outside the boot-load guard above: the maps
	// directory is wired whether or not it exists yet, because the case
	// this sub-project exists for is precisely the one where it does not
	// (2026-09-01-create-scene-leaves design spec §4 — a brand-new campaign
	// starts with nothing installed, and the DM authors a place mid-
	// session). Inside the guard, a campaign that booted with no maps/
	// could never find one afterwards, which is the whole feature.
	gw = gw.WithMapsDir(mapsDir)

	// The embedded client, when this binary was built with one. API-only is
	// a valid configuration (the harness boots servers this way), so a
	// missing bundle is not an error.
	if fsys := clientFS(); fsys != nil {
		gw = gw.WithStatic(fsys)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: gw.Handler(),
		// Bound the header-read phase only. A client that opens a connection
		// and dribbles headers forever would otherwise hold a goroutine
		// indefinitely (Slowloris); 10s is generous for a real client and
		// fatal for that attack.
		//
		// Deliberately NOT ReadTimeout or WriteTimeout: those bound the whole
		// request, and every /ws request becomes a long-lived hijacked
		// WebSocket that must outlive any such deadline. ReadHeaderTimeout
		// applies before the upgrade, so it is the one that is safe here.
		ReadHeaderTimeout: 10 * time.Second,
	}
	closeFn := func() error {
		idsErr := ids.Close()
		cErr := c.Close()
		if idsErr != nil {
			return idsErr
		}
		return cErr
	}
	return srv, closeFn, nil
}
