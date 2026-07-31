import { restored, unrestored, NETWORK_GLOBALS } from "./support/dom";

import { test, expect } from "bun:test";

// support/dom.ts hands the real networking back after registering happy-dom.
// Nothing pinned that it succeeded, and a review found it was restoring five
// names out of ~30 replaced. These fail HERE, with a name, instead of as a
// parse error in metadata.test.ts or wire.test.ts — which drive a real
// Bun.serve and would be the ones to break.

test("every networking global is the one Bun provided, not a simulation", () => {
  expect(unrestored()).toEqual([]);
});

test("happy-dom really does replace them, so the restore is not dead code", () => {
  // If this ever goes empty, the registrator stopped clobbering these and the
  // restore loop is being carried for nothing — or, far worse, the names
  // changed and the list is silently guarding globals that no longer exist.
  expect(restored().length).toBeGreaterThan(0);
});

test("a restored global is usable as its native self", () => {
  // Type identity is necessary but not sufficient: the point is that Bun's
  // fetch accepts these. AbortSignal is the one most likely to break first.
  const c = new AbortController();
  expect(c.signal).toBeInstanceOf(AbortSignal);
  expect(new URL("http://x/y?z=1").pathname).toBe("/y");
  expect(new Headers({ a: "b" }).get("a")).toBe("b");
});

test("the DOM this module exists to provide is actually present", () => {
  // The other direction: restoring too much would break the view tests.
  expect(typeof document).toBe("object");
  expect(new Event("input")).toBeTruthy();
  const el = document.createElement("input");
  let fired = false;
  el.addEventListener("input", () => (fired = true));
  el.dispatchEvent(new Event("input"));
  expect(fired).toBe(true);
});

test("the list covers the names the real-network tests actually use", () => {
  for (const n of ["fetch", "WebSocket", "Response", "Request", "Headers", "URL"]) {
    expect(NETWORK_GLOBALS).toContain(n as (typeof NETWORK_GLOBALS)[number]);
  }
});
