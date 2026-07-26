// adventures.go is the shared boot-time adventures-directory loader
// (adventure-format Task 4): both `vtt serve --adventures-dir`
// (serve_compose.go's composeServer) and `vtt mcp --adventures-dir`
// (mcp.go) walk an adventures directory and call adventure.Load per
// subdirectory against the already-loaded served ruleset — factored here
// once rather than duplicated in both call sites.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// loadAdventuresDir loads and validates EVERY immediate subdirectory of dir
// as an adventure against rs (adventure-format spec §7: "All available
// adventures load+validate at BOOT... fail loud at startup, not at the
// table"): a subdirectory that fails to load for any reason — including one
// declaring a different ruleset id than rs (adventure.Load's own
// ruleset-id-match check) — fails the WHOLE call, naming the file+field
// adventure.Load's own error already names. os.ReadDir returns entries
// sorted by filename, so this walk is deterministic.
//
// Each entry's directory-ness is resolved via os.Stat, which FOLLOWS
// symlinks — deliberately, not os.ReadDir's own DirEntry.IsDir() (which
// reports a symlink entry's OWN type, os.ModeSymlink, never ModeDir, even
// when the link points at a real directory). This lets an adventures dir
// be composed of symlinks into a shared, single-source-of-truth adventure
// tree without content duplication — e.g. a per-table adventures directory
// scoped to one ruleset, built from symlinks into a repo-wide adventures/
// tree that itself holds adventures for MULTIPLE rulesets (this repo's own
// scenarios/testdata/dnd45e-minimal-adventures/ fixture, used by
// scenarios/adventure-night.json, is exactly this shape). A non-directory
// entry (stray file, or a symlink resolving to one) is silently skipped,
// matching adventure.Load's own jsonFilesIn precedent of ignoring
// non-matching entries rather than erroring on them; a broken symlink
// (os.Stat itself failing) is a boot error, same as any other unreadable
// entry.
//
// The returned map is keyed by each adventure's own manifest id
// (Adventure.ID), NOT its directory name — the two are conventionally the
// same (adventures/<id>/) but nothing enforces it structurally. Two
// subdirectories that happen to declare the SAME manifest id would
// otherwise silently collide in the map (the second overwriting the
// first) — checked explicitly here and reported as a boot error naming
// both directories, rather than left as a silent footgun.
func loadAdventuresDir(dir string, rs *rules.Ruleset) (map[string]*adventure.Adventure, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read adventures dir: %w", err)
	}
	out := make(map[string]*adventure.Adventure, len(entries))
	dirOf := make(map[string]string, len(entries))
	for _, e := range entries {
		sub := filepath.Join(dir, e.Name())
		info, err := os.Stat(sub) // follows symlinks, unlike e.IsDir()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", sub, err)
		}
		if !info.IsDir() {
			continue
		}
		adv, err := adventure.Load(sub, rs)
		if err != nil {
			return nil, err
		}
		if prior, dup := dirOf[adv.ID]; dup {
			return nil, fmt.Errorf("adventures dir %s: adventure id %q declared by both %s and %s", dir, adv.ID, prior, e.Name())
		}
		out[adv.ID] = adv
		dirOf[adv.ID] = e.Name()
	}
	return out, nil
}

// loadAdventureGuides reads dir/guide.md for every adventure in advs
// (adventure.Adventure.GuidePath — never read by internal/adventure itself,
// see its own doc comment: "the guide's secrets must never enter the
// compiled event log") and returns adventure id -> guide.md content,
// verbatim. A missing or EMPTY guide.md fails loud (mirrors
// internal/adventure/conformance's own checkGuide binding: "Non-empty
// guide.md required" — reimplemented here rather than imported, since
// internal/adventure/conformance is a test-only proof package, not a
// production dependency cmd/vtt should take on).
func loadAdventureGuides(advs map[string]*adventure.Adventure) (map[string]string, error) {
	out := make(map[string]string, len(advs))
	for id, adv := range advs {
		raw, err := os.ReadFile(adv.GuidePath)
		if err != nil {
			return nil, fmt.Errorf("read guide for adventure %q: %w", id, err)
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("adventure %q: guide.md (%s) must not be empty", id, adv.GuidePath)
		}
		out[id] = string(raw)
	}
	return out, nil
}
