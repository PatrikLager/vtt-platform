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
	// {{id:<name>}} placeholder (e.g. an AddActor's controller_id that must
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
// campaign file, mints one invite token per sc.Participants (role/controls
// taken straight from the scenario), and returns a bootResult ready for
// dialerFor(boot.WSURL, boot.Tokens).
func bootSelfContained(sc *harness.Scenario) (*bootResult, error) {
	dir, err := os.MkdirTemp("", "vtt-harness-run-*")
	if err != nil {
		return nil, fmt.Errorf("vtt client run: boot temp dir: %w", err)
	}
	campaignPath := filepath.Join(dir, "campaign.db")

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

	srv, closeCompose, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir, adventuresDir, "")
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

// mintInvites opens its own identity.DB handle on campaignPath (a second,
// short-lived handle alongside the one composeServer's gateway holds open —
// the same pattern serve_e2e_test.go and internal/gateway's exit fixture
// both use to mint invites against a server they didn't mint them through)
// and mints one invite per participant, closing the handle before
// returning either way. Returns BOTH the token (what a Dialer needs to
// connect) and the real, server-assigned participant id (P6 Task 4 fix
// round — previously discarded via `token, _, err`; now every caller that
// needs participant-id resolution, in-process or test-side, can reuse this
// one function instead of hand-rolling its own minting loop).
func mintInvites(campaignPath string, sc *harness.Scenario) (tokens, ids map[string]string, err error) {
	idb, err := identity.Open(campaignPath)
	if err != nil {
		return nil, nil, fmt.Errorf("vtt client run: open identity for minting: %w", err)
	}
	defer idb.Close()

	tokens = make(map[string]string, len(sc.Participants))
	ids = make(map[string]string, len(sc.Participants))
	for _, p := range sc.Participants {
		token, id, err := idb.CreateInvite(p.Name, identity.Role(p.Role), p.Controls)
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
