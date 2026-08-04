// The browser entry point, and nothing else.
//
// This exists so that app.ts contains no module-level side effects. The
// bootstrap below runs on IMPORT, which put it permanently out of reach of the
// test suite: by the time any test executes, the module has been imported and
// the branch has already been taken, so no in-process test can reach its other
// side. That is a property of the harness, not of the code — the mutants there
// are perfectly observable in a browser — so suppressing them in the gate or
// adjudicating them "equivalent" would both have claimed something untrue.
//
// Splitting the file says the honest thing instead: app.ts is a library, and
// is mutation-gated as one; this file is the wiring that runs it, is excluded
// from `mutate` in stryker.conf.json, and is deliberately kept small enough
// that reading it IS the review.
//
// Referenced from client/index.html. Vite still names the bundle after the
// HTML entry, so the built asset path does not change.

import { boot } from "./app";

// The `typeof document` guard is NOT redundant with this file being the
// browser entry: client/test/all-modules.test.ts imports every source module to
// prove each one loads, and bun shares one process across test files, so
// whether a DOM exists here depends on which file registered happy-dom first.
// Without the guard that test fails on its own and passes in the suite — an
// ordering accident in the one test whose job is to have no such thing. It
// came across from app.ts with the code it guards; losing it in the move was a
// regression the mutation gate could not see, because this file is not gated.
if (typeof document !== "undefined") {
  const root = document.getElementById("app");
  if (root) boot(root);
}
