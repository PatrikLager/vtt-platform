package main

import (
	"errors"
	"fmt"
	"net/http"
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

// composeServer opens the campaign and identity handles for campaignPath
// and wires them into a gateway.Server's Handler on an *http.Server bound
// to addr (not yet listening — the caller starts it, e.g. via
// ListenAndServe or, for tests that need the assigned port, its own
// net.Listener + Serve).
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
// mapsDir is OPTIONAL (maps-as-geometry Task 7, design spec §4.4): ""
// keeps every pre-Task-7 behavior exactly as it was — a nil/empty
// gateway.Server.maps, GET /api/maps answering 200 with an empty list and
// GET /api/packs/{pack}/{file} always 404ing. Unlike adventuresDir, a
// non-empty mapsDir needs no rulesetDir — a standalone map carries no
// ruleset reference (mapdef.Map has none; only adventure.Adventure does).
// Every immediate subdirectory of mapsDir is loaded and validated via
// loadMapsDir (maps.go) — fail loud here, at boot, on any single map's
// failure or an override that does not resolve against its own pack (the
// same "fail loud, never at the table" posture as adventuresDir above),
// closing both handles before returning.
func composeServer(campaignPath, addr, rulesetDir, adventuresDir, mapsDir string) (*http.Server, func() error, error) {
	c, err := campaign.Open(campaignPath)
	if err != nil {
		return nil, nil, fmt.Errorf("vtt serve: open campaign: %w", err)
	}

	ids, err := identity.Open(campaignPath)
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

	if mapsDir != "" {
		maps, packs, packFS, err := loadMapsDir(mapsDir)
		if err != nil {
			_ = ids.Close() // best-effort; the compose error below is what matters
			_ = c.Close()   // best-effort; the compose error below is what matters
			return nil, nil, fmt.Errorf("vtt serve: load maps %s: %w", mapsDir, err)
		}
		gw = gw.WithMaps(maps, packs).WithPackFiles(packFS)
	}

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
