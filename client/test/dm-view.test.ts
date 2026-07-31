import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type State } from "../src/state";
import { renderDMConsole } from "../src/view/dm";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { create } from "@bufbuild/protobuf";
import { EnvelopeSchema, TokenMovedSchema, type Envelope } from "../../contract/gen/ts/vtt/v1/events_pb";

function harness(st: State = newState(), log: Envelope[] = []) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  let confirmAnswer = true;
  const node = renderDMConsole({
    st,
    log,
    adventures: [{ id: "adv-1", name: "Goblin Ambush" }],
    guideFor: async () => "# guide",
    send: (c) => sent.push(c),
    notify: (m) => notices.push(m),
    confirm: () => confirmAnswer,
  });
  return {
    node, sent, notices,
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
    st, log: [moved(1)], adventures: [],
    guideFor: async () => null,
    send: () => {},
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
    actorId: "a1", name: "A", moduleId: "", attributes: {}, resources: {}, controllerId: "",
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
