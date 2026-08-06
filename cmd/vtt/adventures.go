// adventures.go is the shared boot-time adventures-directory loader
// (adventure-format Task 4): both `vtt serve --adventures-dir`
// (serve_compose.go's composeServer) and `vtt mcp --adventures-dir`
// (mcp.go) walk an adventures directory and call adventure.Load per
// subdirectory against the already-loaded served ruleset — factored here
// once rather than duplicated in both call sites.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// loadAdventuresDir loads and validates every immediate subdirectory of dir
// as an adventure against rs (adventure-format spec §7: "All adventures FOR
// THE SERVED RULESET load+validate at BOOT... fail loud at startup, not at
// the table").
//
// A subdirectory that fails to load fails the WHOLE call, naming the
// file+field adventure.Load's own error already names — with ONE exception:
// an adventure declaring a different ruleset id than rs is SKIPPED. The dir
// is a library, and a library may hold books for other tables.
//
// AMENDED 2026-08-06 by Patrik. The original binding (plan
// 2026-07-26-adventure-format.md:60) made any mismatch a boot error — "the
// dir is for THIS table" — which left this repo's own ./adventures
// unbootable and forced a symlinked single-ruleset fixture that gremlins'
// workdir copy silently drops. Do not "restore consistency" by deleting the
// skip below: that reinstates both problems. Per-adventure Load still
// rejects a mismatch outright, because asking for THAT adventure is a
// mistake; only a directory scan tolerates one.
//
// os.ReadDir returns entries sorted by filename, so this walk — and the
// skipped list in the empty-library error — is deterministic.
//
// Each entry's directory-ness is resolved via os.Stat, which FOLLOWS
// symlinks — deliberately, not os.ReadDir's own DirEntry.IsDir() (which
// reports a symlink entry's OWN type, os.ModeSymlink, never ModeDir, even
// when the link points at a real directory). This lets an adventures dir
// be composed of symlinks into a shared, single-source-of-truth adventure
// tree without content duplication — e.g. a per-table adventures directory
// scoped to one ruleset, built from symlinks into a repo-wide adventures/
// tree that itself holds adventures for MULTIPLE rulesets. This repo had
// exactly such a fixture until 2026-08-06; it is gone, because selecting by
// ruleset (below) removed the need for it — and because gremlins' workdir
// copy DROPS symlinks silently, so cmd/vtt's mutants were all false kills for
// as long as its tests depended on one (tools/mutation-scope.md). The symlink
// support itself stays: an operator composing a per-table dir this way is a
// legitimate shape.
//
// A non-directory entry (stray file, or a symlink resolving to one) is
// silently skipped,
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
	var skipped []string
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
			if errors.Is(err, adventure.ErrRulesetMismatch) {
				// Not for this table. An adventures dir is a LIBRARY, and a
				// library may hold books for several tables — serve the ones
				// written for the served ruleset and leave the rest alone.
				//
				// Only this error is skippable, and that is the whole reason
				// internal/adventure exports a sentinel for it: every other
				// failure means the adventure is MALFORMED, and swallowing
				// those would drop a broken adventure out of the library
				// silently instead of failing the boot.
				skipped = append(skipped, e.Name())
				continue
			}
			return nil, err
		}
		if prior, dup := dirOf[adv.ID]; dup {
			return nil, fmt.Errorf("adventures dir %s: adventure id %q declared by both %s and %s", dir, adv.ID, prior, e.Name())
		}
		out[adv.ID] = adv
		dirOf[adv.ID] = e.Name()
	}
	// Zero adventures loaded from an EXISTING dir (either literally empty,
	// or every entry was a non-directory stray, silently skipped above) is
	// a boot error, not a quiet success (fix-wave F4, spec §7: "fail loud
	// at startup, not at the table"). Without this check, a typo'd or
	// never-synced --adventures-dir booted cleanly with zero adventures
	// configured — inconsistent with the NONEXISTENT-dir case just above,
	// which already fails loud via os.ReadDir's own error.
	if len(out) == 0 {
		// Still a boot error (fix-wave F4, spec §7 "fail loud at startup, not
		// at the table"). Selecting by ruleset narrows the silent case to
		// "some matched"; it never licenses booting a table with nothing on
		// it. When entries were skipped, say which ruleset matched none of
		// them — otherwise the operator reads "contains no adventures" about
		// a directory they can see is full.
		if len(skipped) > 0 {
			return nil, fmt.Errorf("adventures dir %s has no adventures for ruleset %q (skipped: %s)",
				dir, rs.ID, strings.Join(skipped, ", "))
		}
		return nil, fmt.Errorf("adventures dir %s contains no adventures", dir)
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
