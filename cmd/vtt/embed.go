package main

import (
	"embed"
	"io/fs"
)

// webdist is the built web client, committed to the repository and embedded
// into the binary so `vtt serve` is a single file with nothing to install
// alongside it.
//
// It lives in cmd/vtt because go:embed cannot reach outside its own package
// directory — the same constraint that put tools.json here. The gateway takes
// the FS via WithStatic rather than reading the disk itself, which keeps all
// filesystem ownership in cmd/vtt (ADR-008).
//
// The directory is COMMITTED, not generated at build time, and check:drift
// enforces that what is committed matches what `vite build` produces. That is
// only sound because the build is reproducible: client/vite.config.ts turns
// off content hashes and sourcemaps for exactly that reason, and it was
// verified by building twice into separate directories and diffing before the
// gate was wired.
//
//go:embed all:webdist
var webdist embed.FS

// clientFS returns the bundle rooted at webdist/, so "/index.html" resolves
// rather than "/webdist/index.html".
//
// Returns nil if the bundle is missing or empty, and the caller then serves
// API-only: a binary built before the client exists must still run, and a
// nil FS is a clean 404 instead of a panic.
func clientFS() fs.FS {
	sub, err := fs.Sub(webdist, "webdist")
	if err != nil {
		return nil
	}
	// An embed of an empty directory yields a usable-but-empty FS; treat that
	// as "no client" so the 404 path is reached rather than serving nothing
	// with a 200.
	if entries, err := fs.ReadDir(sub, "."); err != nil || len(entries) == 0 {
		return nil
	}
	return sub
}
