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

test("storing an empty token is refused", () => {
  expect(() => new Auth(memoryStore()).set("")).toThrow();
});

test("clear actually removes it — closing the tab does not", () => {
  const a = new Auth(memoryStore());
  a.set("tok-1");
  a.clear();
  expect(a.get()).toBeNull();
});
