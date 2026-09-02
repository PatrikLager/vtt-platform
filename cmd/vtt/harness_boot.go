// harness_boot.go is the self-contained boot glue for `vtt client run`
// (task-3-brief.md): it composes a real gateway server (composeServer,
// serve_compose.go) on a throwaway temp campaign, mints one invite token
// per scenario participant directly via identity, and hands back only
// PLAIN STRINGS (a ws:// URL and a name→token map) — never the
// *http.Server, *campaign.Campaign, or *identity.DB it built them from.
// That boundary is deliberate and load-bearing: internal/harness's own
// package comment (client.go) documents the P1 rule that the harness core
// may act only through the wire, the same way a live `--server`/`--tokens`
// run does — self-contained mode must not become a back door that hands
// the harness a server object just because the process happens to own
// both ends.
package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/harness"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// resolveRulesetDir resolves a scenario's bare ruleset id (Scenario.Ruleset,
// e.g. "tavern-brawl") to a loadable directory: rulesets/<id>, relative to
// the REPOSITORY ROOT (ruleset-interpreter Task 6 binding: "self-contained
// boot loads rulesets/<id> relative to repo root"). Resolution rule,
// documented here as the one place it is implemented: walk upward from the
// current working directory until a go.mod file is found (findRepoRoot) —
// this makes bootSelfContained work identically no matter what the
// process's cwd happens to be: the repo root itself (the expected `vtt
// client run` invocation), any subdirectory of it, or a Go test binary's
// own package directory (`go test` sets cwd to the package source dir,
// e.g. cmd/vtt — the SAME mechanism library_test.go's
// TestScenarioLibraryRunsSelfContained relies on when it runs
// scenarios/toy-brawl.json through this exact function). All of those land
// on the SAME rulesets/ directory this repo commits at its root.
func resolveRulesetDir(id string) (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "rulesets", id)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("ruleset %q not found (looked for a directory at %s)", id, dir)
	}
	return dir, nil
}

// resolveAdventuresDir resolves a scenario's relative Adventures dir path
// (e.g. "adventures") to an absolute directory relative to the REPOSITORY
// ROOT (adventure-format Task 4 binding: "self-contained boot passes it to
// serve-compose", mirroring resolveRulesetDir's own repo-root-relative
// resolution above). Unlike resolveRulesetDir, rel is already the directory
// itself (Scenario.Adventures' own doc comment) — no further joining under
// a fixed parent happens beyond the repo-root prefix.
func resolveAdventuresDir(rel string) (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, rel)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("adventures dir %q not found (looked for a directory at %s)", rel, dir)
	}
	return dir, nil
}

// resolveMapsDir resolves a scenario's relative Maps dir path (e.g.
// "scenarios/maps") to an absolute directory relative to the REPOSITORY
// ROOT, exactly as resolveAdventuresDir above resolves its own — same
// findRepoRoot walk, same "rel is already the directory" rule, same
// fail-loud stat. What differs is what the caller then DOES with it: an
// adventures directory is handed to composeServer, which reads it where it
// lies; a maps directory is COPIED INTO the campaign by installMaps below,
// because --maps-dir no longer exists and a map belongs to the campaign
// that uses it (2026-09-01-create-scene-leaves Task 5).
func resolveMapsDir(rel string) (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, rel)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("maps dir %q not found (looked for a directory at %s)", rel, dir)
	}
	return dir, nil
}

// installMaps copies every *.json in srcDir into campaignPath/maps, which is
// where composeServer looks and where internal/gateway's mapByID probes on a
// lookup miss (2026-09-01-create-scene-leaves Tasks 5 and 6). It is the
// scenario runner performing the INSTALL half of "install, then load"
// (design spec §4) on the scenario's behalf: a scenario file names maps by
// id, and an id only means something once the file is in the campaign.
//
// COPIED, NOT SYMLINKED, and the reason is the model rather than tidiness: a
// map belongs to the campaign that uses it (design spec §3), and a link is
// not ownership. A campaign whose maps/ pointed at this repository's own
// committed corpus would be one write away from editing the corpus, and
// bootSelfContained's closeFn deletes the campaign directory afterwards —
// os.RemoveAll would unlink the symlink rather than its target, but a
// throwaway directory that a teardown deletes is the wrong place to keep a
// door into tracked files. A copy is also what an operator does, and these
// files are small.
//
// It creates campaignPath/maps even when srcDir holds no .json at all, which
// composeServer then treats as a boot ERROR ("maps dir ... contains no
// maps"). That is deliberate: a scenario that declares a maps directory has
// said it needs maps, and an empty one is a mistake to report rather than a
// campaign to start.
//
// NON-RECURSIVE, matching the flat maps/ layout Task 3 established (one
// standalone map per file, named by its own id). A packs/ tree is NOT
// installed, because no scenario map declares a pack — the corpus uses only
// the standard tile vocabulary, for which mapdef needs none. A corpus map
// that named a pack and resolved anything against it — a tile override, or an
// object's art — would fail loudly at boot (mapdef.ErrPackNotLoaded, surfaced
// through composeServer's own boot load, since mapdef.LoadInstalled dry-runs
// Compile). One that named a pack and resolved nothing against it would load
// unchanged, because Compile never consults the pack in that case. Neither is
// silent breakage, so packs are left out until a scenario genuinely needs art.
func installMaps(srcDir, campaignPath string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read maps dir %s: %w", srcDir, err)
	}
	dstDir := filepath.Join(campaignPath, "maps")
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dstDir, err)
	}

	// BOTH ends go through os.Root (go1.24+; this repo is on go1.26) rather
	// than filepath.Join of a directory entry's own name — the same primitive
	// and the same reasoning maps.go already applies to a pack directory:
	// "Methods on Root will follow symbolic links, but symbolic links may not
	// reference a location outside the root" (go doc os.Root). A name that is
	// not a single path element, or an entry that is a symlink pointing out of
	// the corpus, cannot make either half of this copy touch a file outside
	// the two directories named here. That is worth having even though srcDir
	// is this repository's own committed corpus: what makes it safe today is a
	// fact about the corpus, and Root makes it a fact about the code.
	src, err := os.OpenRoot(srcDir)
	if err != nil {
		return fmt.Errorf("open maps dir %s: %w", srcDir, err)
	}
	defer src.Close()
	dst, err := os.OpenRoot(dstDir)
	if err != nil {
		return fmt.Errorf("open %s: %w", dstDir, err)
	}
	defer dst.Close()

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := src.ReadFile(e.Name())
		if err != nil {
			return fmt.Errorf("read map %s/%s: %w", srcDir, e.Name(), err)
		}
		if err := dst.WriteFile(e.Name(), data, 0o600); err != nil {
			return fmt.Errorf("install map %s: %w", e.Name(), err)
		}
	}
	return nil
}

// findRepoRoot walks upward from the current working directory until it
// finds a directory containing go.mod, returning that directory. Returns
// an error if it reaches the filesystem root without finding one.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("find repo root: no go.mod found above %s", dir)
		}
		dir = parent
	}
}

// bootResult is bootSelfContained's output: everything `vtt client run`
// needs to drive the scenario, expressed only as strings plus a teardown
// func — the strings (WSURL, Tokens, IDs) are exactly what a live
// `--server`/`--tokens` invocation would have supplied instead (IDs via
// tokens.json's additive "ids" field), so the two modes converge on the
// same harness.Dialer + ids shape (client_run.go's dialerFor + RunScenario
// call).
type bootResult struct {
	WSURL  string
	Tokens map[string]string // participant name -> invite token
	// IDs is participant name -> the real, server-assigned
	// identity.Participant.ID behind that invite (P6 Task 4 fix round) —
	// harness.RunScenario's ids parameter, resolving a scenario's
	// {{id:<name>}} placeholder (e.g. a GrantActorControl's participant_id that must
	// equal a player's own identity) automatically in self-contained mode.
	IDs map[string]string
	// close stops the server, closes the campaign/identity handles, and
	// removes the temp campaign dir. Always non-nil on a successful
	// bootSelfContained; the caller must call it exactly once, after the
	// scenario run has finished closing every harness Conn (mirrors
	// serve_compose.go's composeServer doc comment on why closeFn is only
	// safe once no live WS connection remains).
	close func() error
}

// bootSelfContained starts an in-process gateway server on a fresh temp
// campaign directory, mints one invite token per sc.Participants (name and role
// taken straight from the scenario — an invite carries nothing else since
// 2026-08-24, when the `controls` key that used to ride along with them was
// deleted for granting nothing), and returns a bootResult ready for
// dialerFor(boot.WSURL, boot.Tokens).
func bootSelfContained(sc *harness.Scenario) (*bootResult, error) {
	dir, err := os.MkdirTemp("", "vtt-harness-run-*")
	if err != nil {
		return nil, fmt.Errorf("vtt client run: boot temp dir: %w", err)
	}
	// dir IS the campaign directory (2026-09-01-create-scene-leaves Task 4)
	// — it is already a fresh, dedicated temp directory for this one run,
	// so campaign.Open needs no nested subdirectory (and no `.db`-suffixed
	// name, which would now be misleading: campaign.Open creates a
	// DIRECTORY here, not a file).
	campaignPath := dir

	rulesetDir := ""
	if sc.Ruleset != "" {
		rulesetDir, err = resolveRulesetDir(sc.Ruleset)
		if err != nil {
			_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
			return nil, fmt.Errorf("vtt client run: resolve scenario ruleset %q: %w", sc.Ruleset, err)
		}
	}

	adventuresDir := ""
	if sc.Adventures != "" {
		adventuresDir, err = resolveAdventuresDir(sc.Adventures)
		if err != nil {
			_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
			return nil, fmt.Errorf("vtt client run: resolve scenario adventures dir %q: %w", sc.Adventures, err)
		}
	}

	// BEFORE composeServer, so the maps this scenario names are preloaded and
	// validated at boot exactly as an operator's own installed maps are —
	// rather than relying on mapByID's on-demand probe, which would leave a
	// broken corpus map undetected until the step that loads it.
	if sc.Maps != "" {
		mapsDir, err := resolveMapsDir(sc.Maps)
		if err != nil {
			_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
			return nil, fmt.Errorf("vtt client run: resolve scenario maps dir %q: %w", sc.Maps, err)
		}
		if err := installMaps(mapsDir, campaignPath); err != nil {
			_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
			return nil, fmt.Errorf("vtt client run: install scenario maps: %w", err)
		}
	}

	srv, closeCompose, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir, adventuresDir)
	if err != nil {
		_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
		return nil, fmt.Errorf("vtt client run: boot server: %w", err)
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		_ = closeCompose()
		_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
		return nil, fmt.Errorf("vtt client run: listen: %w", err)
	}
	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- srv.Serve(ln) }()

	tokens, ids, err := mintInvites(campaignPath, sc)
	if err != nil {
		_ = srv.Close() // best-effort; the boot error below is what matters
		_ = closeCompose()
		_ = os.RemoveAll(dir) // best-effort temp cleanup; the returned error is what matters
		return nil, err
	}

	wsURL := "ws://" + ln.Addr().String() + "/ws"
	closeFn := func() error {
		shutdownErr := srv.Close()
		<-serveErrCh
		composeErr := closeCompose()
		removeErr := os.RemoveAll(dir)
		return firstNonNil(shutdownErr, composeErr, removeErr)
	}
	return &bootResult{WSURL: wsURL, Tokens: tokens, IDs: ids, close: closeFn}, nil
}

// mintInvites opens its own identity.DB handle on campaign.LogPath(campaignPath)
// (a second, short-lived handle alongside the one composeServer's gateway
// holds open — the same pattern serve_e2e_test.go and internal/gateway's
// exit fixture both use to mint invites against a server they didn't mint
// them through) and mints one invite per participant, closing the handle
// before returning either way. campaignPath is the campaign DIRECTORY
// (2026-09-01-create-scene-leaves Task 4); by the time mintInvites runs,
// composeServer has already created it, so the log identity shares with the
// store already exists. Returns BOTH the token (what a Dialer needs to
// connect) and the real, server-assigned participant id (P6 Task 4 fix
// round — previously discarded via `token, _, err`; now every caller that
// needs participant-id resolution, in-process or test-side, can reuse this
// one function instead of hand-rolling its own minting loop).
func mintInvites(campaignPath string, sc *harness.Scenario) (tokens, ids map[string]string, err error) {
	idb, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		return nil, nil, fmt.Errorf("vtt client run: open identity for minting: %w", err)
	}
	defer idb.Close()

	tokens = make(map[string]string, len(sc.Participants))
	ids = make(map[string]string, len(sc.Participants))
	for _, p := range sc.Participants {
		token, id, err := idb.CreateInvite(p.Name, identity.Role(p.Role))
		if err != nil {
			return nil, nil, fmt.Errorf("vtt client run: mint invite for %q: %w", p.Name, err)
		}
		tokens[p.Name] = token
		ids[p.Name] = id
	}
	return tokens, ids, nil
}

// firstNonNil returns the first non-nil error in errs, or nil if every one
// is nil — used by closeFn to report a real failure from any of its three
// independent cleanup steps without masking the other two silently.
func firstNonNil(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
