package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/artlib"
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
// so it loads to an empty map set and GET /api/maps answers 200 with an empty
// list. campaignPath/maps PRESENT (even placed there by nothing more than an
// empty mkdir) is loaded and validated in full — fail loud here, at boot, on
// any single map's failure, an override naming art written for a
// format_version this server does not understand, or an existing-but-empty
// maps/ (the same "fail loud, never at the table" posture as adventuresDir
// above), closing both handles before returning. An override naming art that
// is simply NOT installed is not a boot failure since
// 2026-09-02-art-is-a-flat-library Task 3, and one naming art that is
// installed and CANNOT BE READ stopped being one at Task 4b: each degrades
// that one square and warns (spec §4). Until 4b that second case was
// measurably fatal — one corrupt sidecar named by one committed map, exit
// status 1, every other map fine.
//
// loadMapsDir (maps.go) makes all of those calls, and since Task 7 of that
// plan it is called UNCONDITIONALLY — the os.Stat(campaignPath/maps) guard
// that used to stand in front of it is gone with the sibling packs/ tree, the
// pack set and GET /api/packs/{pack}/{file}. Nothing serves art bytes until
// that plan's Task 6 builds GET /api/art/{file}.
//
// campaignPath/art IS CHECKED AND WIRED HERE, unconditionally. THE THREE STEPS
// HAVE THREE DIFFERENT SEVERITIES and that is the substance of it
// (2026-09-02-art-is-a-flat-library Task 4):
//
//   - artRootIsOpenable REFUSES the boot. art/ present and unopenable is every
//     piece failing at once, and an operator at a terminal can chmod it.
//   - artlib.Validate REPORTS and starts anyway (Patrik's ruling, 2026-09-03,
//     which overrides that task's own brief). It finds files no map has named —
//     a subdirectory, a symlink, a name no map could spell, a sidecar with no
//     picture — and a table must not lose its server over one of them. A
//     finding is not a prediction that the piece will be refused at the table:
//     see artlib.Validate's doc comment for what each arm actually does, which
//     ranges from refusing the map to rendering with nothing objecting. It does
//     NOT subsume the check above, which this comment claimed until Task 4
//     found out otherwise: a reported problem does not stop a boot, and an
//     unopenable root must.
//   - WithArtDir hands over the path and reads nothing. Every override and
//     object art name resolves against it at load_map time (design spec §3.6),
//     so art installed or overwritten mid-session takes effect on the next load
//     with no restart.
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

	// EVERYTHING ABOUT art/ HAPPENS HERE, unconditionally: a campaign with art
	// and no map yet is the improvisation case this whole line of work exists
	// for, and anything that only runs when maps/ happens to exist is the
	// boot-order defect of design spec §1 rebuilt in a new directory. There is
	// no os.Stat(mapsDir) guard left to be outside of — Task 7 deleted it once
	// loadMapsDir stopped failing on an absent maps/ — but the rule it forced
	// these three steps to obey is the rule regardless of what the code around
	// them looks like on any given day, and putting any of them behind a
	// condition on some OTHER directory is caught by a test (cmd/vtt's
	// TestABrokenArtDirectoryStopsTheBootWhetherOrNotMapsExists,
	// TestEveryArtProblemIsReportedAtBootAndTheServerStartsAnyway, and
	// TestArtInstalledAfterBootIsFoundWithoutARestart — each of which has a
	// case that boots with no maps/ at all).
	artDir := filepath.Join(campaignPath, "art")

	// An absent art/ passes; art/ present and unopenable stops the boot, where
	// the operator who can fix it is looking (artRootIsOpenable, maps.go, and
	// mapdef.Resolve's own doc comment for the request-time half that degrades
	// instead). This is about the ROOT — every piece failing at once — and is
	// the one art condition that is fatal here.
	if err := artRootIsOpenable(artDir); err != nil {
		_ = ids.Close() // best-effort; the compose error below is what matters
		_ = c.Close()   // best-effort; the compose error below is what matters
		return nil, nil, fmt.Errorf("vtt serve: %w", err)
	}

	// THE WALK REPORTS AND DOES NOT REFUSE (Patrik's severity ruling,
	// 2026-09-03, which overrides this task's own plan text). artlib.Validate
	// finds what no map load can ever see — a subdirectory, a symlink, a
	// filename no map could spell, a sidecar with no picture — and every one of
	// those is a file nothing has named yet. A campaign with two hundred good
	// pieces and one Masonry-1.png copied off a Windows box must not fail to
	// boot over it, and an operator with five mistakes must not pay five boots
	// to hear about them: Validate collects, this reports the lot, and the
	// server starts.
	//
	// THIS REPORT IS THE SIGNAL, AND FOR TWO ARMS IT IS THE ONLY ONE. This
	// comment said "nothing malformed can render regardless — artlib.Lookup
	// refuses each broken piece individually when a map names it" until
	// 2026-09-03, and that is false in four of the five arms (review finding
	// F1, widened by Patrik's ruling of 2026-09-04): a sidecar that cannot be
	// parsed, an orphan sidecar or a subdirectory all draw the square PLAIN,
	// and a relative symlink or a wrong-cased filename RENDERS with nothing
	// objecting. Only a declared format_version this server does not understand
	// still refuses the map. artlib.Validate's own doc comment carries the
	// measured per-arm table; do not restate it here,
	// because a second copy is a second thing to rot. What the ruling buys is
	// that one hand-copied file does not cost a table its server — not that the
	// loader will catch everything this walk names. `vtt art install` (Task 6)
	// still refuses outright, because there the operator is holding the file.
	//
	// slog, because this is the "the platform continues and here is what is
	// wrong" shape internal/campaign and internal/gateway already use it for,
	// and because composeServer has no other channel: its return values are a
	// server or a refusal, and this is neither. The problems go in the MESSAGE
	// rather than an attribute so they print one per line instead of as one
	// escaped string.
	if problems := artlib.Validate(artDir); problems != nil {
		slog.Warn(fmt.Sprintf(
			"vtt serve: art dir %s has problems; the server is starting anyway. Do not "+
				"assume a map load will stop them: some refuse the map, some draw the "+
				"square plain, and a symlink or a wrong-cased name renders regardless:\n%v",
			artDir, problems))
	}

	// UNCONDITIONALLY, and nothing is read: WithArtDir hands over a PATH, and
	// every override and object art name is resolved against it when a map is
	// loaded (design spec §3.6). That is what makes art installed or overwritten
	// mid-session take effect on the next load_map with no restart — and what
	// deletes the boot-time art load whose ordering was the original defect.
	// dir need not exist: a campaign that has installed no art is ordinary, and
	// each unresolved reference costs one warning rather than the map (§4).
	gw = gw.WithArtDir(artDir)

	// UNCONDITIONALLY, with no guard on campaignPath/maps existing: loadMapsDir
	// answers "nothing installed yet" for an absent maps/ itself, and answers a
	// boot error for every other read failure — permissions, a plain file
	// sitting where maps/ belongs, an existing-but-empty maps/ (art-is-a-flat-
	// library Task 7; see loadMapsDir's own doc comment).
	//
	// THE GUARD THAT USED TO BE HERE was an os.Stat(mapsDir), and its only
	// reason was that loadMapsDir failed on a missing directory. It was also
	// this sub-project's own defect in miniature — a check on one directory
	// deciding whether another one gets loaded (design spec §1) — and it is why
	// the three art steps above had to be written OUTSIDE it and pinned by
	// tests that boot with no maps/ at all. With the walk tolerant, there is
	// nothing left for it to decide.
	mapsDir := filepath.Join(campaignPath, "maps")
	maps, err := loadMapsDir(campaignPath)
	if err != nil {
		_ = ids.Close() // best-effort; the compose error below is what matters
		_ = c.Close()   // best-effort; the compose error below is what matters
		return nil, nil, fmt.Errorf("vtt serve: load maps %s: %w", campaignPath, err)
	}
	gw = gw.WithMaps(maps)
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
