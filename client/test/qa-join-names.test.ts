import "./support/dom";
import { test, expect, afterEach } from "bun:test";
import { requestJoin } from "../src/join";
import { renderJoinView } from "../src/view/join";
import { boot } from "../src/app";

const realFetch = globalThis.fetch;
const realWS = globalThis.WebSocket;
const TAKEN = "gateway: that name is already taken at this table; choose another\n";
const BAD = "gateway: a display name is required, up to 64 ordinary characters\n";
const DOOR = "gateway: this link is not accepting anyone\n";

afterEach(() => {
  globalThis.fetch = realFetch;
  globalThis.WebSocket = realWS;
  try {
    localStorage.clear();
  } catch {
    /* none */
  }
  document.body.innerHTML = "";
});

function answer(status: number, body: string) {
  globalThis.fetch = (async () => new Response(body, { status })) as unknown as typeof fetch;
}

async function settle() {
  for (let i = 0; i < 8; i++) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

async function flow(status: number, body: string, typed: string) {
  localStorage.clear();
  (globalThis as unknown as { happyDOM: { setURL(u: string): void } }).happyDOM.setURL(
    "http://localhost/?join=s3cret",
  );
  let dialled = 0;
  globalThis.WebSocket = class {
    constructor() {
      dialled++;
    }
  } as unknown as typeof WebSocket;
  answer(status, body);
  const root = document.createElement("div");
  document.body.appendChild(root);
  boot(root);
  const input = root.querySelector('[data-field="join-name"]') as HTMLInputElement;
  input.value = typed;
  input.dispatchEvent(new Event("input", { bubbles: true }));
  const btn = root.querySelector('[data-action="join"]') as HTMLButtonElement;
  btn.click();
  await settle();
  return { root, dialled: () => dialled };
}

function errText(root: HTMLElement): string {
  return (root.querySelector(".error")?.textContent ?? "").trim();
}

test("a 400 taken-name body is read as a name refusal saying the name is taken, trimmed and unprefixed", async () => {
  // VTT-556
  answer(400, TAKEN);
  const r = await requestJoin("http://localhost", "s3cret", "Ana");
  expect(r.ok).toBe(false);
  if (r.ok) return;
  expect(r.reason).toBe("name");
  expect(r.message).toBe("that name is already taken at this table; choose another");
});

test("a 400 unusable-name body is shown unprefixed and trimmed", async () => {
  // VTT-556
  answer(400, BAD);
  const r = await requestJoin("http://localhost", "s3cret", "");
  expect(r.ok).toBe(false);
  if (r.ok) return;
  expect(r.reason).toBe("name");
  expect(r.message).toBe("a display name is required, up to 64 ordinary characters");
});

test("a 400 body without the gateway prefix is still shown, trimmed", async () => {
  // VTT-556
  answer(400, "  plain words \n");
  const r = await requestJoin("http://localhost", "s3cret", "Ana");
  if (r.ok) throw new Error("expected refusal");
  expect(r.message).toBe("plain words");
});

test("a 400 with an empty body gives a fixed non-empty refusal of the name", async () => {
  // VTT-556
  answer(400, "");
  const r = await requestJoin("http://localhost", "s3cret", "Ana");
  if (r.ok) throw new Error("expected refusal");
  expect(r.reason).toBe("name");
  expect(r.message.trim().length).toBeGreaterThan(0);
  expect(r.message).not.toContain("gateway");
  answer(403, DOOR);
  const door = await requestJoin("http://localhost", "s3cret", "Ana");
  if (door.ok) throw new Error("expected refusal");
  expect(r.message).not.toBe(door.message);
});

test("a 400 whose body is only whitespace gives the same fixed refusal as an empty body", async () => {
  // VTT-556
  answer(400, "");
  const empty = await requestJoin("http://localhost", "s3cret", "Ana");
  answer(400, "  \n ");
  const ws = await requestJoin("http://localhost", "s3cret", "Ana");
  if (empty.ok || ws.ok) throw new Error("expected refusals");
  expect(ws.message).toBe(empty.message);
  expect(empty.message.length).toBeGreaterThan(0);
});

test("the door refusal (403) is not read as a name refusal and does not say the name is taken", async () => {
  answer(403, DOOR);
  const r = await requestJoin("http://localhost", "s3cret", "Ana");
  if (r.ok) throw new Error("expected refusal");
  expect(r.reason).toBe("door");
  expect(r.message).not.toContain("already taken");
  answer(400, TAKEN);
  const t = await requestJoin("http://localhost", "s3cret", "Ana");
  if (t.ok) throw new Error("expected refusal");
  expect(t.message).toContain("already taken");
  expect(t.message).not.toBe(r.message);
});

test("a 200 admission still returns the token", async () => {
  answer(200, JSON.stringify({ token: "tok", participantId: "p1", name: "Ana", role: "spectator" }));
  const r = await requestJoin("http://localhost", "s3cret", "Ana");
  expect(r).toEqual({ ok: true, token: "tok" });
});

test("the join form draws the error it is given and keeps the typed name in the box", () => {
  // VTT-548
  const root = document.createElement("div");
  document.body.appendChild(root);
  renderJoinView(
    root,
    { name: "Ana", busy: false, error: "that name is already taken at this table; choose another" },
    { onName() {}, onSubmit() {} },
  );
  expect(errText(root)).toContain("already taken");
  expect((root.querySelector('[data-field="join-name"]') as HTMLInputElement).value).toBe("Ana");
});

test("a join refused as taken through boot shows the taken text, keeps the name, stores nothing, dials nothing, re-enables Join", async () => {
  // VTT-556 VTT-548
  const { root, dialled } = await flow(400, TAKEN, "Ana");
  expect(errText(root)).toContain("that name is already taken at this table; choose another");
  expect(errText(root)).not.toContain("gateway:");
  expect((root.querySelector('[data-field="join-name"]') as HTMLInputElement).value).toBe("Ana");
  expect(localStorage.getItem("vtt.token")).toBeNull();
  expect(dialled()).toBe(0);
  expect((root.querySelector('[data-action="join"]') as HTMLButtonElement).disabled).toBe(false);
});

test("a join refused as unusable through boot shows that text and keeps the name", async () => {
  // VTT-556 VTT-548
  const { root, dialled } = await flow(400, BAD, "  ");
  expect(errText(root)).toContain("a display name is required");
  expect((root.querySelector('[data-field="join-name"]') as HTMLInputElement).value).toBe("  ");
  expect(localStorage.getItem("vtt.token")).toBeNull();
  expect(dialled()).toBe(0);
});

test("a join refused with an empty 400 body through boot shows a non-empty fixed refusal and keeps the name", async () => {
  // VTT-556 VTT-548
  const { root } = await flow(400, "", "Ana");
  expect(errText(root).length).toBeGreaterThan(0);
  expect((root.querySelector('[data-field="join-name"]') as HTMLInputElement).value).toBe("Ana");
});

test("a join refused at the door through boot shows a refusal that is not the taken text, and keeps the name", async () => {
  // VTT-548
  const { root, dialled } = await flow(403, DOOR, "Ana");
  expect(errText(root).length).toBeGreaterThan(0);
  expect(errText(root)).not.toContain("already taken");
  expect((root.querySelector('[data-field="join-name"]') as HTMLInputElement).value).toBe("Ana");
  expect(localStorage.getItem("vtt.token")).toBeNull();
  expect(dialled()).toBe(0);
  expect((root.querySelector('[data-action="join"]') as HTMLButtonElement).disabled).toBe(false);
});
