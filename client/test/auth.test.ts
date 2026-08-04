import { test, expect } from "bun:test";
import { Auth, memoryStore } from "../src/auth";

test("a stored token round-trips", () => {
  const a = new Auth(memoryStore());
  a.set("tok-1");
  expect(a.get()).toBe("tok-1");
});

test("no token reads as null, not as an empty string", () => {
  expect(new Auth(memoryStore()).get()).toBeNull();
});

test("an empty stored value reads as null", () => {
  // Otherwise the client sends `Bearer ` and the resulting 401 looks like a
  // server fault rather than "you are not logged in".
  const store = memoryStore();
  store.setItem("vtt.token", "");
  expect(new Auth(store).get()).toBeNull();
});

test("storing an empty token is refused, and says why", () => {
  // The MESSAGE is asserted, not just that it threw. This is the only place
  // the refusal is explained, and a caller that hits it is holding an empty
  // string it believed was a token — "Error" alone sends them looking at the
  // store rather than at what they passed.
  expect(() => new Auth(memoryStore()).set("")).toThrow("auth: refusing to store an empty token");
});

test("a refused empty token leaves any existing one intact", () => {
  // set() must reject BEFORE writing. Otherwise a failed set is indistinguish-
  // able from a successful one at the store, and a user who fumbles an empty
  // paste is silently logged out.
  const store = memoryStore();
  const a = new Auth(store);
  a.set("tok-1");
  expect(() => a.set("")).toThrow();
  expect(a.get()).toBe("tok-1");
});

test("clear actually removes it — closing the tab does not", () => {
  const a = new Auth(memoryStore());
  a.set("tok-1");
  a.clear();
  expect(a.get()).toBeNull();
});
