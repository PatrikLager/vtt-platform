import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Roster } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import joinURL from "../../contract/testdata/join_url_format.json";
import { JoinDoor, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { create } from "@bufbuild/protobuf";
import { EnvelopeSchema, TokenMovedSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";

function harness(
  st: State = newState(),
  log: Envelope[] = [],
  opts: {
    adventures?: { id: string; name: string }[];
    guide?: string | null;
    participants?: { participantId: string; displayName: string }[];
  } = {},
) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  let confirmAnswer = true;
  // Counted, not just answered: some guards must run BEFORE the dialog opens,
  // and "nothing was sent" cannot tell a validation refusal from a declined
  // confirmation.
  let confirmCount = 0;
  const confirmMessages: string[] = [];
  const node = renderDMConsole({
    st,
    log,
    adventures: opts.adventures ?? [{ id: "adv-1", name: "Goblin Ambush" }],
    guideFor: async () => (opts.guide === undefined ? "# guide" : opts.guide),
    participants: opts.participants ?? [],
    // The sharing panel is exercised by its own tests below; these existing
    // cases assert the rest of the console, so they render without one.
    joinLink: null,
    roster: [],
    origin: "https://table.example",
    refreshSharing: () => {},
    send: async (c) => void sent.push(c),
    notify: (m) => notices.push(m),
    confirm: (m: string) => {
      confirmCount++;
      confirmMessages.push(m);
      return confirmAnswer;
    },
  });
  return {
    node, sent, notices,
    confirmCount: () => confirmCount,
    confirmMessages: () => confirmMessages,
    setConfirm: (v: boolean) => (confirmAnswer = v),
    field: (f: string) => node.querySelector(`[data-field="${f}"]`) as HTMLInputElement,
    action: (a: string) => node.querySelector(`[data-action="${a}"]`) as HTMLButtonElement,
    button: (label: string) =>
      Array.from(node.querySelectorAll("button")).find((b) => b.textContent === label) as HTMLButtonElement,
  };
}

beforeEach(() => {
  document.body.innerHTML = "";
});

// --- validation guards ------------------------------------------------------

test("starting a session without a name is refused before anything is sent", () => {
  const h = harness();
  h.action("start-session").click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toMatch(/name/i);
});

test("a scene with no id or a zero dimension is refused", () => {
  const h = harness();
  h.field("scene-id").value = "";
  h.button("Create")!.click();
  expect(h.sent).toHaveLength(0);

  const h2 = harness();
  h2.field("scene-id").value = "s1";
  h2.field("scene-w").value = "0";
  h2.field("scene-h").value = "5";
  h2.button("Create")!.click();
  expect(h2.sent).toHaveLength(0);
  expect(h2.notices[0]).toMatch(/width and height/i);
});

test("an actor with no id is refused", () => {
  const h = harness();
  h.action("add-actor").click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toMatch(/id/i);
});

test("a token missing any one of id, scene or actor is refused", () => {
  // The title used to promise all three and exercise only the empty-scene
  // case, so a guard that checked scene alone would have passed it.
  const fields = ["token-id", "token-scene", "token-actor"] as const;
  for (const missing of fields) {
    const h = harness();
    for (const f of fields) h.field(f).value = f === missing ? "" : "x";
    h.action("place-token").click();
    expect(h.sent).toHaveLength(0);
    // And it says which, rather than failing mutely.
    expect(h.notices).not.toHaveLength(0);
  }
});

test("a note needs both a key and text", () => {
  const h = harness();
  h.field("note-key").value = "k";
  h.field("note-text").value = "";
  h.button("Save")!.click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toMatch(/key and some text/i);
});

test("malformed pasted JSON is reported, not sent", () => {
  const h = harness();
  (h.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement).value = "{oops";
  h.button("Add from JSON")!.click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toMatch(/JSON/i);
});

// --- undo, the careful one --------------------------------------------------

const moved = (seq: number) =>
  create(EnvelopeSchema, {
    sequence: BigInt(seq),
    payload: { case: "tokenMoved", value: create(TokenMovedSchema, { tokenId: "t1", to: { x: 1, y: 1 } }) },
  });

test("undo asks for confirmation and does nothing when declined", () => {
  // A retraction is visible to everyone at the table the moment it lands.
  const h = harness(newState(), [moved(1)]);
  h.setConfirm(false);
  h.button("Undo #1")!.click();
  expect(h.sent).toHaveLength(0);
});

test("undo sends the retraction when confirmed", () => {
  const h = harness(newState(), [moved(1), moved(2)]);
  h.button("Undo #2")!.click();
  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.case).toBe("retractEvents");
});

test("an empty log offers nothing to undo and sends nothing", () => {
  const h = harness(newState(), []);
  h.button("Nothing to undo")!.click();
  expect(h.sent).toHaveLength(0);
});

test("an invalid range is refused BEFORE the confirmation dialog", () => {
  // Confirming an undo that would do nothing still writes a marker implying
  // something changed.
  let confirmCalls = 0;
  const st = newState();
  const node = renderDMConsole({
    participants: [],
    joinLink: null,
    roster: [],
    origin: "https://table.example",
    refreshSharing: () => {},
    st, log: [moved(1)], adventures: [],
    guideFor: async () => null,
    send: async () => {},
    notify: () => {},
    confirm: () => {
      confirmCalls++;
      return true;
    },
  });
  (node.querySelector('[data-field="undo-from"]') as HTMLInputElement).value = "5";
  (node.querySelector('[data-field="undo-to"]') as HTMLInputElement).value = "9";
  Array.from(node.querySelectorAll("button")).find((b) => b.textContent === "Undo range")!.click();
  expect(confirmCalls).toBe(0);
});

// --- conditions -------------------------------------------------------------

test("a condition on an actor gets a removal button that sends removeCondition", () => {
  const st = newState();
  st.Actors["a1"] = {
    actorId: "a1", name: "A", moduleId: "", attributes: {}, resources: {}, controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
  };
  st.Conditions["a1"] = [{ ID: "dazed", Source: "dm", AppliedSeq: 3 }];
  const h = harness(st);
  h.button("a1: dazed ✕")!.click();
  expect(h.sent[0]!.command.case).toBe("removeCondition");
});

test("with no conditions the removal group is absent entirely", () => {
  const h = harness();
  expect(h.node.textContent).not.toContain("Remove condition");
});

// --- session state drives which control is offered --------------------------

test("an open session offers End, a closed one offers Start", () => {
  const open = newState();
  open.Sessions = [{ ID: "s1", Name: "Night", StartSeq: 1, EndSeq: 0 }];
  expect(harness(open).button("End session")).toBeTruthy();
  expect(harness(newState()).action("start-session")).toBeTruthy();
});

// --- the draft buffer -------------------------------------------------------
//
// The draft exists because the console re-renders on every event, which used
// to wipe whatever the DM was mid-way through typing (found by the e2e, not
// by a unit test). These pin the other half: that a field which HAS been
// submitted does not come back.

test("text survives a re-render, so an incoming event cannot eat what the DM is typing", () => {
  const first = harness();
  first.field("scene-id").value = "cave";
  first.field("scene-id").dispatchEvent(new Event("input"));

  // A second render is what an arriving event causes.
  const second = harness();
  expect(second.field("scene-id").value).toBe("cave");
});

test("a submitted session name is not restored on the next render", () => {
  const h = harness();
  h.field("session-name").value = "Night One";
  h.field("session-name").dispatchEvent(new Event("input"));
  h.action("start-session").click();
  expect(h.sent).toHaveLength(1);

  expect(harness().field("session-name").value).toBe("");
});

test("a REFUSED start still clears the name — the text is gone with the command", () => {
  // Pins current behaviour, which does not match the comment on clearDraft
  // ("after the command it fed succeeded"): send() is fire-and-forget here,
  // so the draft is cleared whether or not the gateway accepts it. A DM
  // whose command is refused has to retype. Deliberately recorded as-is
  // rather than quietly changed — see the note raised with this test.
  const h = harness();
  h.field("session-name").value = "Night One";
  h.field("session-name").dispatchEvent(new Event("input"));
  h.action("start-session").click();

  expect(harness().field("session-name").value).toBe("");
});

test("a name rejected by the client's own guard is kept, because nothing was sent", () => {
  const h = harness();
  h.field("session-name").value = "   ";
  h.field("session-name").dispatchEvent(new Event("input"));
  h.action("start-session").click();

  expect(h.sent).toHaveLength(0);
  expect(h.notices).toContain("a session needs a name");
  expect(harness().field("session-name").value).toBe("   ");
});

// ============================================================================
// Mutation-driven suite.
//
// view/dm.ts scored 40.71% mutation against 89% line coverage: 166 survivors,
// 80 of them emptied string LITERALS and 20 removed a `.trim()`. The console
// is nine near-identical forms, and what was untested was not the rendering —
// it was every message, every field name, and every piece of input
// normalisation on the way to a command.
//
// The existing assertions above use loose regexes (`/name/i`), which is why
// blanking a message survived. These assert exact text and exact payloads.

/** The command kinds and values that reached the wire. */
function payloads(h: ReturnType<typeof harness>) {
  return h.sent.map((c) => ({ case: c.command.case, value: c.command.value as Record<string, unknown> }));
}

function fill(h: ReturnType<typeof harness>, values: Record<string, string>) {
  for (const [field, v] of Object.entries(values)) h.field(field).value = v;
}

// --- the field contract -----------------------------------------------------

test("every input the console owns is reachable by its stable data-field", () => {
  // The data-field names are the console's test contract AND the draft
  // buffer's keys: renaming one silently drops that box's remembered text on
  // the next re-render. Nothing pinned the names.
  const h = harness();
  for (const f of [
    "session-name", "scene-id", "scene-name", "scene-w", "scene-h",
    "actor-id", "actor-name", "actor-controller", "actor-json",
    "token-id", "token-scene", "token-actor", "token-x", "token-y",
    "note-key", "note-title", "note-text", "undo-from", "undo-to",
  ]) {
    expect(h.field(f)).not.toBeNull();
  }
});

// --- exact refusal messages -------------------------------------------------

test("each guard refuses with its own exact wording", () => {
  // One message per guard, and they must differ: a DM told "invalid" cannot
  // tell which box to fix.
  const cases: [string, () => ReturnType<typeof harness>][] = [
    ["a session needs a name", () => { const h = harness(); h.action("start-session").click(); return h; }],
    ["scene needs an id and a positive width and height", () => { const h = harness(); h.button("Create").click(); return h; }],
    ["an actor needs an id", () => { const h = harness(); h.action("add-actor").click(); return h; }],
    ["a token needs an id, a scene and an actor", () => { const h = harness(); h.action("place-token").click(); return h; }],
    ["a note needs a key and some text", () => { const h = harness(); h.button("Save").click(); return h; }],
    ["name the note to delete", () => { const h = harness(); h.button("Delete").click(); return h; }],
    ["nothing to undo", () => { const h = harness(); h.button("Nothing to undo").click(); return h; }],
  ];
  const seen: string[] = [];
  for (const [want, run] of cases) {
    const h = run();
    expect(h.notices).toEqual([want]);
    expect(h.sent).toHaveLength(0);
    seen.push(want);
  }
  expect(new Set(seen).size).toBe(seen.length);
});

test("a scene is refused for a zero or negative dimension, on either axis", () => {
  for (const [w, hgt] of [["0", "5"], ["5", "0"], ["-1", "5"], ["5", "-1"], ["abc", "5"]]) {
    const h = harness();
    fill(h, { "scene-id": "s1", "scene-w": w!, "scene-h": hgt! });
    h.button("Create").click();
    expect(h.sent).toHaveLength(0);
    expect(h.notices).toEqual(["scene needs an id and a positive width and height"]);
  }
});

test("a note is refused when either the key or the text is missing", () => {
  for (const [key, text] of [["", "t"], ["k", ""], ["  ", "t"], ["k", "  "]]) {
    const h = harness();
    fill(h, { "note-key": key!, "note-text": text! });
    h.button("Save").click();
    expect(h.sent).toHaveLength(0);
    expect(h.notices).toEqual(["a note needs a key and some text"]);
  }
});

// --- input normalisation reaches the command --------------------------------

test("a scene's id and name are TRIMMED, and its dimensions become numbers", () => {
  // Removing a .trim() is invisible unless the sent payload is inspected: the
  // form still works and the server stores " s1 " as a distinct scene id.
  const h = harness();
  fill(h, { "scene-id": "  s1  ", "scene-name": "  The Hall  ", "scene-w": " 6 ", "scene-h": " 4 " });
  h.button("Create").click();
  expect(payloads(h)).toEqual([
    { case: "createScene", value: expect.objectContaining({ sceneId: "s1", name: "The Hall", gridWidth: 6, gridHeight: 4 }) },
  ]);
});

test("an actor's fields are trimmed, and a blank controller is sent as unset", () => {
  const h = harness();
  fill(h, { "actor-id": "  a1  ", "actor-name": "  Lera  ", "actor-controller": "   " });
  h.action("add-actor").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("addActor");
  // addActor nests the payload under `actor` (commands.ts:205).
  const a = p!.value["actor"] as Record<string, unknown>;
  expect(a["actorId"]).toBe("a1");
  expect(a["name"]).toBe("Lera");
  // `controller.value.trim() || undefined` — whitespace must not become a
  // participant id of "", and commands.ts omits the key entirely when unset.
  expect(a["controllerId"] ?? "").toBe("");
});

test("a named controller is carried through", () => {
  const h = harness();
  fill(h, { "actor-id": "a1", "actor-name": "Lera", "actor-controller": "  p-7  " });
  h.action("add-actor").click();
  expect((payloads(h)[0]!.value["actor"] as Record<string, unknown>)["controllerId"]).toBe("p-7");
});

test("a token's ids are trimmed and its coordinates parsed, defaulting to 0", () => {
  const h = harness();
  fill(h, { "token-id": " t1 ", "token-scene": " s1 ", "token-actor": " a1 ", "token-x": " 3 ", "token-y": "" });
  h.action("place-token").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("placeToken");
  expect(p!.value["tokenId"]).toBe("t1");
  expect(p!.value["sceneId"]).toBe("s1");
  expect(p!.value["actorId"]).toBe("a1");
  // `Number(ty.value) || 0` — an empty box is the origin, not NaN.
  expect(p!.value["position"]).toEqual(expect.objectContaining({ x: 3, y: 0 }));
});

test("a note's key, title and text are trimmed on the way out", () => {
  const h = harness();
  fill(h, { "note-key": " k ", "note-title": " T ", "note-text": " body " });
  h.button("Save").click();
  expect(payloads(h)).toEqual([
    { case: "upsertNote", value: expect.objectContaining({ key: "k", title: "T", text: "body" }) },
  ]);
});

test("deleting a note trims its key", () => {
  const h = harness();
  fill(h, { "note-key": "  k  " });
  h.button("Delete").click();
  expect(payloads(h)).toEqual([{ case: "deleteNote", value: expect.objectContaining({ key: "k" }) }]);
});

// --- undo: validate, then confirm, then send --------------------------------

test("a retraction is NOT sent when the DM declines the confirmation", () => {
  const h = harness(newState(), [moved(1)]);
  h.setConfirm(false);
  h.button("Undo #1").click();
  expect(h.sent).toHaveLength(0);
});

test("the single-event undo retracts exactly that one sequence", () => {
  const h = harness(newState(), [moved(1), moved(2)]);
  h.button("Undo #2").click();
  expect(payloads(h)).toEqual([
    { case: "retractEvents", value: expect.objectContaining({ fromSequence: 2n, toSequence: 2n, reason: "undo" }) },
  ]);
});

test("an invalid range is refused BEFORE the confirmation dialog opens", () => {
  // Ordering is the point: validating after confirming would ask the DM to
  // approve something that then does nothing, and a pointless marker is
  // visible to the whole table.
  // confirmCount is the assertion that matters. "Nothing was sent" cannot
  // distinguish a validation refusal from a declined dialog, so a version
  // that confirmed first and validated second would pass without it.
  const h = harness(newState(), [moved(1)]);
  h.setConfirm(true);
  fill(h, { "undo-from": "5", "undo-to": "9" });
  h.button("Undo range").click();
  expect(h.confirmCount()).toBe(0);
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toContain("past the last event");
});

test("a valid range is sent with both ends and the undo reason", () => {
  const h = harness(newState(), [moved(1), moved(2)]);
  fill(h, { "undo-from": "1", "undo-to": "2" });
  h.button("Undo range").click();
  expect(payloads(h)).toEqual([
    { case: "retractEvents", value: expect.objectContaining({ fromSequence: 1n, toSequence: 2n, reason: "undo" }) },
  ]);
});

// --- adventures -------------------------------------------------------------

test("the adventures section is absent when there are none", () => {
  const h = harness();
  expect(Array.from(h.node.querySelectorAll("h3")).some((n) => n.textContent === "Adventures")).toBe(true);
});

// --- adventures -------------------------------------------------------------
//
// The whole section was unexercised: the harness supplied adventures and no
// test ever clicked Load or guide, so the loop body, the name-or-id fallback
// and the missing-guide message were all free to be deleted.

test("each adventure offers a Load button that sends its id", () => {
  const h = harness(newState(), [], { adventures: [{ id: "adv-1", name: "Goblin Ambush" }] });
  h.button("Load Goblin Ambush").click();
  expect(payloads(h)).toEqual([
    { case: "loadAdventure", value: expect.objectContaining({ adventureId: "adv-1" }) },
  ]);
});

test("an adventure with no name is labelled by its id", () => {
  // `${a.name || a.id}` — the fallback exists so an unnamed module is still
  // clickable rather than rendering as "Load ".
  const h = harness(newState(), [], { adventures: [{ id: "adv-7", name: "" }] });
  expect(h.button("Load adv-7")).toBeDefined();
});

test("every adventure in the list gets its own pair of buttons", () => {
  // Kills the emptied for-loop body and the seeded row array.
  const h = harness(newState(), [], {
    adventures: [{ id: "a", name: "A" }, { id: "b", name: "B" }],
  });
  expect(h.button("Load A")).toBeDefined();
  expect(h.button("Load B")).toBeDefined();
  expect(Array.from(h.node.querySelectorAll("button")).filter((b) => b.textContent === "guide")).toHaveLength(2);
});

test("the adventures section is absent entirely when there are none", () => {
  // `d.adventures.length > 0` -> `>= 0` renders an empty "Adventures" group.
  const h = harness(newState(), [], { adventures: [] });
  expect(Array.from(h.node.querySelectorAll("h3")).some((n) => n.textContent === "Adventures")).toBe(false);
});

test("the guide button shows the guide it fetched", async () => {
  const h = harness(newState(), [], { guide: "# The Cave" });
  h.button("guide").click();
  await new Promise((r) => setTimeout(r, 0));
  expect(h.notices).toEqual(["# The Cave"]);
});

test("an adventure with no guide says so rather than showing nothing", async () => {
  // `g ?? "no guide for that adventure"` — a silent no-op reads as a broken
  // button.
  const h = harness(newState(), [], { guide: null });
  h.button("guide").click();
  await new Promise((r) => setTimeout(r, 0));
  expect(h.notices).toEqual(["no guide for that adventure"]);
});

// --- whitespace is not content ----------------------------------------------

test("a whitespace-only id is refused everywhere one is required", () => {
  // Kills the `.trim()` removals in the GUARDS specifically: without trim,
  // "   " is truthy and the command goes out with a blank id.
  const scene = harness();
  fill(scene, { "scene-id": "   ", "scene-w": "5", "scene-h": "5" });
  scene.button("Create").click();
  expect(scene.sent).toHaveLength(0);

  const actor = harness();
  fill(actor, { "actor-id": "   " });
  actor.action("add-actor").click();
  expect(actor.sent).toHaveLength(0);

  for (const field of ["token-id", "token-scene", "token-actor"]) {
    const h = harness();
    fill(h, { "token-id": "t", "token-scene": "s", "token-actor": "a", [field]: "   " });
    h.action("place-token").click();
    expect(h.sent).toHaveLength(0);
  }

  const note = harness();
  fill(note, { "note-key": "   " });
  note.button("Delete").click();
  expect(note.sent).toHaveLength(0);
});

test("a token's y coordinate is carried through, not just its x", () => {
  // `Number(ty.value) || 0` -> `&& 0` sends 0 for any non-empty y. The
  // earlier test left y empty, where both forms produce 0.
  const h = harness();
  fill(h, { "token-id": "t", "token-scene": "s", "token-actor": "a", "token-x": "2", "token-y": "7" });
  h.action("place-token").click();
  expect(payloads(h)[0]!.value["position"]).toEqual(expect.objectContaining({ x: 2, y: 7 }));
});

// --- the paste box ----------------------------------------------------------

test("the actor JSON textarea remembers what was typed across a re-render", () => {
  // Its own draft entry, separate from the inputs' — and its listener was
  // free to be emptied without any test noticing.
  const first = harness();
  const box = first.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement;
  box.value = '{"actorId":"a9"}';
  box.dispatchEvent(new Event("input"));
  const second = harness();
  expect((second.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement).value)
    .toBe('{"actorId":"a9"}');
});

test("pasted JSON that does not parse is reported, not thrown", () => {
  const h = harness();
  const box = h.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement;
  box.value = "{not json";
  box.dispatchEvent(new Event("input"));
  h.button("Add from JSON").click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices).toHaveLength(1);
  expect(h.notices[0]).not.toBe("");
});

test("pasted JSON that parses is sent as an addActor command", () => {
  const h = harness();
  const box = h.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement;
  box.value = '{"actorId":"a9","name":"Nine"}';
  box.dispatchEvent(new Event("input"));
  h.button("Add from JSON").click();
  expect(payloads(h)[0]!.case).toBe("addActor");
});

// --- the draft clears the RIGHT fields ---------------------------------------
//
// clearDraft takes a list of field names, and every name in those lists was
// free to be blanked: clearing the wrong key leaves stale text in the box the
// DM just submitted, and wipes one they are still typing in.

test("adding an actor clears exactly the actor fields", () => {
  const h = harness();
  fill(h, { "actor-id": "a1", "actor-name": "Lera", "actor-controller": "p-7", "scene-id": "keep-me" });
  for (const f of ["actor-id", "actor-name", "actor-controller", "scene-id"]) {
    h.field(f).dispatchEvent(new Event("input"));
  }
  h.action("add-actor").click();

  const next = harness();
  expect(next.field("actor-id").value).toBe("");
  expect(next.field("actor-name").value).toBe("");
  expect(next.field("actor-controller").value).toBe("");
  // A field belonging to another form must survive.
  expect(next.field("scene-id").value).toBe("keep-me");
});

test("placing a token clears exactly the token fields", () => {
  const h = harness();
  fill(h, { "token-id": "t1", "token-scene": "s1", "token-actor": "a1", "token-x": "2", "token-y": "3", "note-key": "keep-me" });
  for (const f of ["token-id", "token-scene", "token-actor", "token-x", "token-y", "note-key"]) {
    h.field(f).dispatchEvent(new Event("input"));
  }
  h.action("place-token").click();

  const next = harness();
  for (const f of ["token-id", "token-scene", "token-actor", "token-x", "token-y"]) {
    expect(next.field(f).value).toBe("");
  }
  expect(next.field("note-key").value).toBe("keep-me");
});

test("adding an actor from JSON clears the paste box", () => {
  const h = harness();
  const box = h.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement;
  box.value = '{"actorId":"a9","name":"Nine"}';
  box.dispatchEvent(new Event("input"));
  h.button("Add from JSON").click();

  const next = harness();
  expect((next.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement).value).toBe("");
});

test("the paste box starts empty on a fresh console", () => {
  // `draft["actor-json"] ?? ""` — without the fallback the box renders
  // "undefined" as its text.
  expect((harness().node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement).value).toBe("");
});

// --- session heading and controls track the open session --------------------

test("the session group names the open session and offers End", () => {
  const open = newState();
  open.Sessions = [{ ID: "s", Name: "Night One", StartSeq: 1, EndSeq: 0 }];
  const h = harness(open);
  expect(Array.from(h.node.querySelectorAll("h3")).some((n) => n.textContent === "Session: Night One")).toBe(true);
  expect(h.button("End session")).toBeDefined();
  expect(h.action("start-session")).toBeNull();
});

test("a CLOSED session leaves the console offering Start, not End", () => {
  // `find((s) => s.EndSeq === 0)` -> `find(() => true)` picks the closed
  // session and renders "Session: ..." with an End button that cannot work.
  const closed = newState();
  closed.Sessions = [{ ID: "s", Name: "Night One", StartSeq: 1, EndSeq: 9 }];
  const h = harness(closed);
  expect(Array.from(h.node.querySelectorAll("h3")).some((n) => n.textContent === "Session")).toBe(true);
  expect(h.action("start-session")).not.toBeNull();
  expect(h.button("End session")).toBeUndefined();
});

// --- destructive actions name what they will destroy -------------------------

test("the single-undo confirmation names the sequence being retracted", () => {
  // The DM's last defence before a retraction everyone at the table sees. An
  // emptied prompt is a blank modal asking for consent to nothing.
  const h = harness(newState(), [moved(1), moved(2)]);
  h.button("Undo #2").click();
  expect(h.confirmMessages()[0]).toBe("Retract event #2? Everyone at the table sees this.");
});

test("the range-undo confirmation names both ends", () => {
  const h = harness(newState(), [moved(1), moved(2)]);
  fill(h, { "undo-from": "1", "undo-to": "2" });
  h.button("Undo range").click();
  expect(h.confirmMessages()[0]).toBe("Retract events #1–#2? Everyone at the table sees this.");
});

test("an undo-from box left empty is read as 0 and refused", () => {
  // `Number(from.value) || 0` feeding retractableRange, whose own guard
  // rejects a zero sequence.
  const h = harness(newState(), [moved(1)]);
  fill(h, { "undo-to": "1" });
  h.button("Undo range").click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toBe("sequences start at 1");
});

// --- conditions -------------------------------------------------------------

test("the remove-condition group is absent when no actor has one", () => {
  expect(Array.from(harness().node.querySelectorAll("h3")).some((n) => n.textContent === "Remove condition")).toBe(false);
});

// --- structure, not styling -------------------------------------------------

test("buttons carry the labels the DM clicks, not blanks", () => {
  // The submit buttons are found by data-action in these tests, so blanking
  // their visible LABEL was free. A blank button is unusable.
  const h = harness();
  for (const label of ["Start session", "Create", "Add", "Add from JSON", "Place", "Save", "Delete", "Undo range"]) {
    expect(Array.from(h.node.querySelectorAll("button")).some((b) => b.textContent === label)).toBe(true);
  }
});

test("every group carries a heading", () => {
  const open = newState();
  open.Conditions["a1"] = [{ ID: "prone", Source: "s", AppliedSeq: 1 }];
  const titles = Array.from(harness(open).node.querySelectorAll("h3")).map((n) => n.textContent);
  for (const t of ["Session", "Create scene", "Add actor", "…or paste actor JSON",
                   "Place token", "Adventures", "Notes", "Remove condition", "Undo"]) {
    expect(titles).toContain(t);
  }
  expect(titles.every((t) => t !== "")).toBe(true);
});

test("a group's row holds only its elements, with no stray text", () => {
  // `const row: HTMLElement[] = []` seeded with a sentinel, and
  // `if (text !== undefined) n.textContent = text` forced true, both leak
  // text into containers that should hold only controls.
  const h = harness(newState(), [], { adventures: [{ id: "a", name: "A" }] });
  const rows = Array.from(h.node.querySelectorAll(".row"));
  expect(rows.length).toBeGreaterThan(0);
  for (const row of rows) {
    for (const node of Array.from(row.childNodes)) {
      // Element children only: a text node here is a leaked sentinel.
      expect(node.nodeType).toBe(1);
    }
  }
});

test("the undo group holds exactly its button, two boxes and the range button", () => {
  const h = harness(newState(), [moved(1)]);
  const undo = Array.from(h.node.querySelectorAll(".dmgroup"))
    .find((g) => g.querySelector("h3")?.textContent === "Undo")!;
  const row = undo.querySelector(".row")!;
  expect(row.children.length).toBe(4);
});

test("an input created without a class carries no class at all", () => {
  // `cls = ""` -> a truthy default puts a bogus class on every unstyled box.
  const h = harness();
  expect(h.field("scene-id").className).toBe("");
  expect(h.field("session-name").className).toBe("wide");
});

test("a button created without an action carries no data-action", () => {
  // `if (action) b.dataset["action"] = action` -> true stamps
  // data-action="undefined" on every plain button, which these tests select by.
  const h = harness();
  const plain = Array.from(h.node.querySelectorAll("button")).find((b) => b.textContent === "Create")!;
  expect(plain.hasAttribute("data-action")).toBe(false);
  expect(h.action("start-session").getAttribute("data-action")).toBe("start-session");
});

test("the paste box shows an example of the JSON it wants", () => {
  // The placeholder IS the documentation for this box — there is no other
  // hint about the shape it accepts.
  const box = harness().node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement;
  expect(box.placeholder).toContain("actorId");
});

test("elements created without a class carry no class, not the string 'undefined'", () => {
  // `if (cls) n.className = cls` -> always-assign writes undefined into
  // className, which stringifies. Headings are the elements built without one.
  const h = harness();
  expect(h.node.querySelector("h2")!.className).toBe("");
  expect(h.node.querySelector("h3")!.className).toBe("");
});

test("a group carries no stray text of its own", () => {
  // `if (text !== undefined) n.textContent = text` -> always-assign sets
  // textContent to undefined on the container elements, which renders the
  // literal word "undefined" above every group.
  for (const g of Array.from(harness().node.querySelectorAll(".dmgroup"))) {
    for (const node of Array.from(g.childNodes)) expect(node.nodeType).toBe(1);
  }
});

test("the console's styling hooks are the ones the stylesheet targets", () => {
  // Class names are not decoration here: `tiny` and `wide` size the boxes,
  // and blanking one produces a form whose numeric fields are full width.
  // Cheap to pin and stable — unlike placeholder prose, these change only on
  // purpose.
  const h = harness();
  expect(h.node.className).toBe("dm");
  for (const f of ["scene-w", "scene-h", "token-x", "token-y", "undo-from", "undo-to"]) {
    expect(h.field(f).className).toBe("tiny");
  }
  for (const f of ["session-name", "actor-controller", "note-text"]) {
    expect(h.field(f).className).toBe("wide");
  }
  expect((h.node.querySelector('[data-field="actor-json"]') as HTMLElement).className).toBe("paste");
  for (const b of Array.from(h.node.querySelectorAll("button"))) expect(b.className).toBe("chip");
});

test("every box carries a placeholder saying what belongs in it", () => {
  // Asserts PRESENCE, not wording. dm.ts:45 records that placeholders are
  // prose and get reworded, and that pinning the words makes a test break on
  // a copy edit — but an EMPTY placeholder is a usability defect, not a copy
  // choice: the console is nineteen unlabelled boxes without them.
  const h = harness();
  for (const f of [
    "session-name", "scene-id", "scene-name", "scene-w", "scene-h",
    "actor-id", "actor-name", "actor-controller",
    "token-id", "token-scene", "token-actor", "token-x", "token-y",
    "note-key", "note-title", "note-text", "undo-from", "undo-to",
  ]) {
    expect(h.field(f).placeholder.length).toBeGreaterThan(0);
  }
});

test("the console is titled", () => {
  expect(harness().node.querySelector("h2")!.textContent).toBe("DM console");
});

// --- handing a character over (T7) -----------------------------------------
//
// The DM console is where a character is assigned. Without this the whole
// actor-control feature is reachable only by an agent over MCP: the contract,
// both folds and authz all carry it, and no human at the table can use it.

function tableWithActor(): State {
  const st = newState();
  st.Actors["act-warden"] = {
    actorId: "act-warden", name: "Warden", moduleId: "",
    attributes: {}, resources: {},
    controllerId: "p-ana", controllerIds: ["p-ana"], kind: ActorKind.UNSPECIFIED,
  };
  return st;
}

test("the DM can hand a character to another participant", () => {
  const h = harness(tableWithActor(), [], {
    participants: [
      { participantId: "p-ana", displayName: "Ana" },
      { participantId: "p-bo", displayName: "Bo" },
    ],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect(row).not.toBeNull();

  const target = row.querySelector(".grant-target") as HTMLSelectElement;
  target.value = "p-bo";
  (row.querySelector(".grant") as HTMLButtonElement).click();

  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.case).toBe("grantActorControl");
  expect(h.sent[0]!.command.value).toMatchObject({ actorId: "act-warden", participantId: "p-bo" });
});

test("each current controller can be revoked individually", () => {
  const st = tableWithActor();
  st.Actors["act-warden"]!.controllerIds = ["p-ana", "p-bo"];
  const h = harness(st, [], {
    participants: [
      { participantId: "p-ana", displayName: "Ana" },
      { participantId: "p-bo", displayName: "Bo" },
    ],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  const revokes = row.querySelectorAll(".revoke");
  expect(revokes).toHaveLength(2);
  // The label and the wrapper class: a blank button is clickable and tells the
  // DM nothing, and .held is the hook the layout hangs on.
  expect((revokes[0] as HTMLElement).textContent).toBe("Revoke");
  expect(row.querySelectorAll(".held")).toHaveLength(2);
  // BOTH labels, in order. querySelector(".held-who") reads only the first
  // match, so a lookup that ignored the controller id and always named
  // participants[0] passed every other assertion here — and then the LABEL and
  // the Revoke button disagree, which is worse than either error alone.
  expect(Array.from(row.querySelectorAll(".held-who")).map((n) => n.textContent))
    .toEqual(["Ana", "Bo"]);

  (revokes[1] as HTMLButtonElement).click();
  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.case).toBe("revokeActorControl");
  // The SECOND controller, not the first: a per-controller button that always
  // revokes controller_ids[0] would look right and take the wrong character
  // away from the wrong person.
  expect(h.sent[0]!.command.value).toMatchObject({ actorId: "act-warden", participantId: "p-bo" });
});

test("an unowned actor shows no revoke controls but can still be granted", () => {
  const st = tableWithActor();
  st.Actors["act-warden"]!.controllerId = "";
  st.Actors["act-warden"]!.controllerIds = [];
  const h = harness(st, [], { participants: [{ participantId: "p-bo", displayName: "Bo" }] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect(row.querySelectorAll(".revoke")).toHaveLength(0);
  expect(row.querySelector(".grant")).not.toBeNull();
});

test("the current controllers are shown by DISPLAY NAME, not by id", () => {
  // A uuid tells the DM nothing about who is holding a character.
  const h = harness(tableWithActor(), [], {
    participants: [{ participantId: "p-ana", displayName: "Ana" }],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  // Scoped to .held-who, NOT the row's whole textContent: the grant dropdown
  // lists every participant by display name, so a row-wide assertion is
  // satisfied by an <option> and passes even when the CONTROLLER is rendered
  // as a raw id. Measured — that is exactly what it did.
  expect(row.querySelector(".held-who")?.textContent).toBe("Ana");
  // The ACTOR's name too: a nameless row still carries Grant and Revoke, so
  // the DM cannot tell which character the buttons act on.
  expect(row.querySelector(".control-name")?.textContent).toBe("Warden");
});

test("an actor with no name falls back to its id rather than rendering blank", () => {
  const st = tableWithActor();
  st.Actors["act-warden"]!.name = "";
  const h = harness(st, [], { participants: [] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect(row.querySelector(".control-name")?.textContent).toBe("act-warden");
});

test("a controller who is not connected is still shown, by id", () => {
  // Control is campaign-scoped and presence is connection-scoped (spec §3.1),
  // so someone can hold a character while offline. Rendering only the
  // participants we can name would silently drop them from the list and make
  // the character look unowned.
  const h = harness(tableWithActor(), [], { participants: [] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  // Same scoping, for the same reason.
  expect(row.querySelector(".held-who")?.textContent).toBe("p-ana");
  expect(row.querySelectorAll(".revoke")).toHaveLength(1);
});

test("granting with nobody selected sends nothing", () => {
  const h = harness(tableWithActor(), [], { participants: [] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  (row.querySelector(".grant") as HTMLButtonElement).click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices.join(" ")).toContain("nobody");
});

test("the grant dropdown labels every participant, and leads with the blank", () => {
  // o.value is asserted by the grant test; o.textContent was not, so the
  // dropdown could label every participant identically while still sending the
  // right id — the DM picks blind.
  const h = harness(tableWithActor(), [], {
    participants: [
      { participantId: "p-ana", displayName: "Ana" },
      { participantId: "p-bo", displayName: "Bo" },
    ],
  });
  const target = h.node.querySelector(".grant-target") as HTMLSelectElement;
  expect(Array.from(target.querySelectorAll("option")).map((o) => o.textContent))
    .toEqual(["choose a participant", "Ana", "Bo"]);
  expect(target.value).toBe("");
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect((row.querySelector(".grant") as HTMLElement).textContent).toBe("Grant");
});

test("each actor remembers its OWN grant choice", () => {
  // The draft key is per-actor. A shared key would let a choice made for one
  // character reappear pre-selected on another — and the DM would grant the
  // wrong one without ever touching the dropdown.
  const st = tableWithActor();
  st.Actors["act-adder"] = {
    actorId: "act-adder", name: "Adder", moduleId: "",
    attributes: {}, resources: {}, controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
  };
  const parts = [{ participantId: "p-bo", displayName: "Bo" }];
  const first = harness(st, [], { participants: parts });
  const warden = first.node.querySelector('.control-actor[data-actor="act-warden"] .grant-target') as HTMLSelectElement;
  warden.value = "p-bo";
  warden.dispatchEvent(new Event("change"));

  const second = harness(st, [], { participants: parts });
  expect((second.node.querySelector('.control-actor[data-actor="act-warden"] .grant-target') as HTMLSelectElement).value).toBe("p-bo");
  expect((second.node.querySelector('.control-actor[data-actor="act-adder"] .grant-target') as HTMLSelectElement).value).toBe("");
});

test("the control panel's order does not depend on insertion order", () => {
  // The MIRROR of the test below, inserting the other way round. A comparator
  // that always returns -1 happens to produce the right answer for one
  // insertion order and the wrong one for the other, so one test cannot see
  // it — measured, it survived the container gate. Same trap as the
  // participant comparator in session.ts.
  const st = newState();
  const mk = (id: string) => ({
    actorId: id, name: id, moduleId: "",
    attributes: {}, resources: {}, controllerId: "", controllerIds: [] as string[], kind: ActorKind.UNSPECIFIED,
  });
  st.Actors["act-adder"] = mk("act-adder");
  st.Actors["act-warden"] = mk("act-warden");
  const h = harness(st, [], { participants: [] });
  expect(Array.from(h.node.querySelectorAll(".control-actor")).map((r) => (r as HTMLElement).dataset["actor"]))
    .toEqual(["act-adder", "act-warden"]);
});

test("the control panel lists actors in a stable order", () => {
  // Insertion order is whatever the log happened to do. A panel that reshuffles
  // as events arrive moves the Revoke button out from under the DM's cursor.
  const st = tableWithActor();
  st.Actors["act-adder"] = {
    actorId: "act-adder", name: "Adder", moduleId: "",
    attributes: {}, resources: {}, controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
  };
  const h = harness(st, [], { participants: [] });
  expect(Array.from(h.node.querySelectorAll(".control-actor")).map((r) => (r as HTMLElement).dataset["actor"]))
    .toEqual(["act-adder", "act-warden"]);
});

const hasControlPanel = (n: HTMLElement) =>
  Array.from(n.querySelectorAll("h3")).some((x) => x.textContent === "Who controls what");

test("a campaign with no actors shows no control panel at all", () => {
  expect(hasControlPanel(harness().node)).toBe(false);
});

test("a campaign with an actor shows the control panel", () => {
  expect(hasControlPanel(harness(tableWithActor()).node)).toBe(true);
});

test("the DM's choice of participant survives a re-render", () => {
  // The console is rebuilt on every event. Without a draft buffer the DM picks
  // a name, a token moves, and Grant then reports "choose a participant first"
  // — which reads as the DM's mistake. Found for the text inputs by the e2e;
  // this is the same mechanism for the select.
  const st = tableWithActor();
  const parts = [{ participantId: "p-bo", displayName: "Bo" }];
  const first = harness(st, [], { participants: parts });
  const sel = first.node.querySelector(".grant-target") as HTMLSelectElement;
  sel.value = "p-bo";
  sel.dispatchEvent(new Event("change"));

  const second = harness(st, [], { participants: parts });
  expect((second.node.querySelector(".grant-target") as HTMLSelectElement).value).toBe("p-bo");
});

// --- sharing the table, and who may do what (plan J6) --------------------

function emptyState(): State {
  return newState();
}

function shareConsole(opts: {
  joinLink?: { open: boolean; secret: string } | null;
  roster?: Roster[] | null;
} = {}) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  let refreshes = 0;
  const node = renderDMConsole({
    st: emptyState(),
    log: [],
    adventures: [],
    guideFor: async () => null,
    participants: [],
    joinLink: opts.joinLink === undefined ? { open: false, secret: "s3cret" } : opts.joinLink,
    // `=== undefined`, not `??`: null is a MEANINGFUL value here (the read
    // failed) and `?? []` would quietly turn it into "an empty table".
    roster: opts.roster === undefined ? [] : opts.roster,
    origin: "https://table.example",
    refreshSharing: () => refreshes++,
    send: async (c) => void sent.push(c),
    notify: (m) => notices.push(m),
    confirm: () => true,
  });
  return { node, sent, notices, refreshes: () => refreshes };
}

test("the DM can open the door, and the console says which way it is", () => {
  // THE SEAM THIS WHOLE TASK EXISTS FOR. Before it, identity.SetJoinOpen had
  // no caller anywhere outside its own tests: five completed tasks, every gate
  // green, and the shared join link admitted nobody because nothing in the
  // product could open it.
  const c = shareConsole({ joinLink: { open: false, secret: "s3cret" } });

  expect(c.node.querySelector(".door-state")!.textContent).toContain("closed");
  c.node.querySelector<HTMLButtonElement>('[data-action="open-door"]')!.click();

  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.case).toBe("setJoinDoor");
  // The VALUE, not just the command: a console that sent UNSPECIFIED would be
  // refused every time and look broken rather than unauthorized.
  expect(c.sent[0]!.command.value).toMatchObject({ door: JoinDoor.OPEN });
});

test("an open door offers to close it, not to open it again", () => {
  // One button whose meaning follows the state. Two always-present buttons
  // would leave a DM asking which one is currently in effect.
  const c = shareConsole({ joinLink: { open: true, secret: "s3cret" } });

  expect(c.node.querySelector(".door-state")!.textContent).toContain("open");
  expect(c.node.querySelector('[data-action="open-door"]')).toBeNull();
  c.node.querySelector<HTMLButtonElement>('[data-action="close-door"]')!.click();

  expect(c.sent[0]!.command.value).toMatchObject({ door: JoinDoor.CLOSED });
});

test("the console shows the whole link a DM can paste, not just the secret", () => {
  // A secret on its own is not shareable: the DM would have to know to wrap it
  // in a URL, and the one they invent is where the "?join=" spelling gets
  // wrong.
  //
  // NOT the only writer, and this comment used to claim it was: cmd/vtt's
  // `join-link show` and `rotate` print the same format, so there are THREE
  // writers against app.ts's one reader. They were tied together by NOTHING,
  // and renaming the parameter left the CLI printing a dead link with every
  // gate green — gremlins does not mutate string literals and Stryker cannot
  // see Go at all.
  //
  // FIXED (#46): the shape now lives in contract/testdata/join_url_format.json
  // and every site derives from it, so a rename fails all four at once. The
  // expectation below is built from the fixture rather than written out, which
  // is the point — a literal here would be a fifth copy.
  const c = shareConsole({ joinLink: { open: true, secret: "s3cret" } });
  const link = c.node.querySelector<HTMLInputElement>('[data-field="join-link"]')!;
  expect(link.value).toBe(`https://table.example${joinURL.shareSuffix}s3cret`);
  expect(link.readOnly).toBe(true);
});

test("rotating asks first, because it locks out a link already sent", () => {
  // Destructive in a way opening is not: everyone who was sent the old link
  // silently stops being able to use it, and there is no undo.
  const sent: ClientCommand[] = [];
  let asked = 0;
  const node = renderDMConsole({
    st: emptyState(), log: [], adventures: [], guideFor: async () => null,
    participants: [], joinLink: { open: true, secret: "s3cret" }, roster: [],
    origin: "https://table.example", refreshSharing: () => {},
    send: async (c) => void sent.push(c), notify: () => {},
    confirm: () => {
      asked++;
      return false;
    },
  });
  node.querySelector<HTMLButtonElement>('[data-action="rotate-link"]')!.click();

  expect(asked).toBe(1);
  expect(sent).toHaveLength(0); // answered no
});

test("a spectator can be promoted, and a player is not offered promotion again", () => {
  // The client half of promote_participant, which J3 shipped WITHOUT one: the
  // command reached the contract, authz and the MCP tool list, and no console
  // could issue it.
  const c = shareConsole({
    roster: [
      { participantId: "p-watch", name: "Zoe", role: "spectator" },
      { participantId: "p-play", name: "Lera", role: "player" },
      { participantId: "p-dm", name: "Ari", role: "dm" },
    ],
  });

  const rows = c.node.querySelectorAll(".roster-row");
  expect(rows.length).toBe(3); // everyone is SHOWN, including the DM

  c.node.querySelector<HTMLButtonElement>('[data-action="promote-p-watch"]')!.click();
  expect(c.sent).toHaveLength(1);
  expect(c.sent[0]!.command.value).toMatchObject({ participantId: "p-watch", role: "player" });

  // A player gets the reverse control, not a second promotion.
  expect(c.node.querySelector('[data-action="promote-p-play"]')).toBeNull();
  c.node.querySelector<HTMLButtonElement>('[data-action="demote-p-play"]')!.click();
  expect(c.sent[1]!.command.value).toMatchObject({ participantId: "p-play", role: "spectator" });

  // A DM is offered NEITHER. promote_participant may only ever name player or
  // spectator (spec §3.1a) — a button that reached dm would make the shared
  // link a route to full authority in two steps, and the server refuses it, so
  // offering it would only ever produce a confusing failure.
  expect(c.node.querySelector('[data-action="promote-p-dm"]')).toBeNull();
  expect(c.node.querySelector('[data-action="demote-p-dm"]')).toBeNull();
});

test("without a join link there is no sharing panel, rather than an empty one", () => {
  // A player's console never asks for it, and a failed fetch must not leave a
  // panel showing a blank link the DM might paste to somebody.
  const c = shareConsole({ joinLink: null });
  expect(c.node.querySelector(".door-state")).toBeNull();
  expect(c.node.querySelector('[data-field="join-link"]')).toBeNull();
});

test("ending a session is addressable, not just clickable by its label", () => {
  // A data-action, like every other console control. Without one the only
  // handle is the button's TEXT, and the e2e specs need to close a session
  // they opened — leaving one open is what broke `task e2e` for two weeks
  // (see client/e2e/setup.ts's note on the shared campaign).
  const st = newState();
  st.Sessions = [{ ID: "s1", Name: "Open Table", StartSeq: 1, EndSeq: 0 }];
  const sent: ClientCommand[] = [];
  const node = renderDMConsole({
    st, log: [], adventures: [], guideFor: async () => null,
    participants: [], joinLink: null, roster: [], origin: "https://table.example",
    refreshSharing: () => {}, send: async (c) => void sent.push(c), notify: () => {}, confirm: () => true,
  });

  node.querySelector<HTMLButtonElement>('[data-action="end-session"]')!.click();
  expect(sent[0]!.command.case).toBe("endSession");
});

test("rotating goes through once the DM says yes", async () => {
  // The other half of the confirm. Asserting only the refusal would leave the
  // path that actually replaces the link — the destructive one — unrun.
  const sent: ClientCommand[] = [];
  let refreshes = 0;
  const node = renderDMConsole({
    st: newState(), log: [], adventures: [], guideFor: async () => null,
    participants: [], joinLink: { open: true, secret: "s3cret" }, roster: [],
    origin: "https://table.example", refreshSharing: () => refreshes++,
    send: async (c) => void sent.push(c), notify: () => {}, confirm: () => true,
  });

  node.querySelector<HTMLButtonElement>('[data-action="rotate-link"]')!.click();

  expect(sent).toHaveLength(1);
  expect(sent[0]!.command.case).toBe("rotateJoinLink");
  // The re-read happens AFTER the server answers, not beside the command:
  // both travel on different transports, and an HTTP read fired alongside a
  // WS command can arrive first and repaint with the state the command was
  // about to change — with no event to correct it afterwards.
  expect(refreshes).toBe(0);
  await Promise.resolve();
  await Promise.resolve();
  expect(refreshes).toBe(1);
});

test("a roster that could not be read is not the same as an empty table", () => {
  // List() always contains at least the caller, so an empty roster cannot
  // happen — an empty array would be a failed fetch wearing the costume of an
  // ordinary answer, and the panel would vanish with nothing said.
  const absent = shareConsole({ roster: null });
  expect(absent.node.querySelector(".roster-row")).toBeNull();
  expect(absent.node.textContent).not.toContain("Who may do what");

  const empty = shareConsole({ roster: [] });
  expect(empty.node.textContent).toContain("Who may do what");
});

test("the sharing panel says what it means, in words a DM reads", () => {
  // The LABELS, not just the data-actions. A console wired correctly and
  // labelled wrongly is a console that does the opposite of what its buttons
  // say — and these particular words carry the difference between "anyone
  // with this link can walk in" and "nobody can".
  const shut = shareConsole({ joinLink: { open: false, secret: "s" } });
  expect(shut.node.querySelector(".door-state")!.textContent).toBe("door: closed");
  expect(
    shut.node.querySelector('[data-action="open-door"]')!.textContent,
  ).toBe("Open the door");

  const open = shareConsole({ joinLink: { open: true, secret: "s" } });
  expect(open.node.querySelector(".door-state")!.textContent).toBe("door: open");
  expect(
    open.node.querySelector('[data-action="close-door"]')!.textContent,
  ).toBe("Close the door");
  expect(open.node.querySelector('[data-action="rotate-link"]')!.textContent).toBe("New link");

  // The panel's WHOLE composition, so a stray node cannot slip in unnoticed:
  // this is the one panel that renders a shared secret, and anything else
  // sitting in it is something a DM might read as part of the link.
  const panel = open.node.querySelector(".dmgroup:has(.door-state)")!;
  expect(panel.querySelector("h3")!.textContent).toBe("Sharing this table");
  const row = panel.querySelector(".row")!;
  expect(row.children.length).toBe(4); // state, link, door button, rotate
  expect(row.textContent).toBe("door: openClose the doorNew link");
  // The link input is full-width, or the URL is cropped to something the DM
  // will copy half of.
  expect(open.node.querySelector<HTMLInputElement>('[data-field="join-link"]')!.className)
    .toBe("wide");
});

test("the roster names each person, their role, and what the button will do", () => {
  const c = shareConsole({
    roster: [
      { participantId: "p-w", name: "Zoe", role: "spectator" },
      { participantId: "p-p", name: "Lera", role: "player" },
    ],
  });

  const watcher = c.node.querySelector('.roster-row[data-participant="p-w"]')!;
  expect(watcher.querySelector(".who")!.textContent).toBe("Zoe");
  expect(watcher.querySelector(".role")!.textContent).toBe("spectator");
  expect(watcher.querySelector("button")!.textContent).toBe("Make player");

  const player = c.node.querySelector('.roster-row[data-participant="p-p"]')!;
  expect(player.querySelector(".role")!.textContent).toBe("player");
  expect(player.querySelector("button")!.textContent).toBe("Make spectator");
});

test("the rotate confirmation says what is lost, not just 'are you sure'", () => {
  // The DM is about to invalidate a link other people are already holding.
  // "Are you sure?" gives them nothing to be sure ABOUT.
  let asked = "";
  const node = renderDMConsole({
    st: newState(), log: [], adventures: [], guideFor: async () => null,
    participants: [], joinLink: { open: true, secret: "s" }, roster: [],
    origin: "https://table.example", refreshSharing: () => {},
    send: async () => {}, notify: () => {},
    confirm: (m) => {
      asked = m;
      return false;
    },
  });
  node.querySelector<HTMLButtonElement>('[data-action="rotate-link"]')!.click();

  expect(asked).toContain("old one");
  expect(asked.length).toBeGreaterThan(20);
});
