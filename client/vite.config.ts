import { defineConfig } from "vite";

// The client build. Its output is COMMITTED at cmd/vtt/webdist and guarded by
// check:drift, which means this config has one hard requirement beyond
// working: it must be REPRODUCIBLE. A gate on a nondeterministic artifact
// fails at random and gets disabled, which is worse than no gate.
//
// Everything below exists for that reason:
//
//   * No content hashes in filenames. Vite's default
//     `assets/index-a1b2c3d4.js` changes whenever the content does, which is
//     correct for cache-busting on a CDN and useless here — the file is
//     served from an embedded FS by a binary that ships with it, so the
//     binary IS the cache key. Hashed names would also churn the committed
//     diff on every unrelated change.
//   * Sourcemaps off. They embed absolute paths from the build machine, so
//     the artifact would differ between Patrik's laptop and CI and the gate
//     would fail on the first CI run for no real reason.
//   * emptyOutDir, so a renamed entry point cannot leave an orphan behind
//     that the drift check would then happily accept forever.
export default defineConfig({
  root: __dirname,
  build: {
    outDir: "../cmd/vtt/webdist",
    emptyOutDir: true,
    sourcemap: false,
    // Vite warns above 500 kB; this bundle is far smaller and a warning that
    // never fires is noise, but leaving the default documents the intent.
    rollupOptions: {
      output: {
        entryFileNames: "assets/[name].js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: "assets/[name][extname]",
      },
    },
  },
});
