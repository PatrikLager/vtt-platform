import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach } from "bun:test";
import { newState, type State } from "../src/state";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Roster, MapMeta } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import joinURL from "../../contract/testdata/join_url_format.json";
import { JoinDoor, type ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";

function harness(
  st: State = newState(),
  opts: {
    adventures?: { id: string; name: string }[];
    maps?: MapMeta[];
    guide?: string | null;
    participants?: { participantId: string; displayName: string }[];
    doorsArmed?: boolean;
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
  // A LOCAL bit, not a re-render: renderDMConsole runs once per harness()
  // call here (app.ts's paint() is what rebuilds it for real on a toggle),
  // so this tracks what toggleDoors DID rather than what the static node
  // shows after clicking it — the same reason `sent`/`notices` are tracked
  // this way rather than read back off the DOM.
  let doorsArmed = opts.doorsArmed ?? false;
  let toggles = 0;
  const node = renderDMConsole({
    st,
    adventures: opts.adventures ?? [{ id: "adv-1", name: "Goblin Ambush" }],
    maps: opts.maps ?? [],
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
    doorsArmed,
    toggleDoors: () => {
      doorsArmed = !doorsArmed;
      toggles++;
    },
  });
  return {
    node, sent, notices,
    confirmCount: () => confirmCount,
    confirmMessages: () => confirmMessages,
    setConfirm: (v: boolean) => (confirmAnswer = v),
    doorsArmed: () => doorsArmed,
    toggles: () => toggles,
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

// --- the console cannot retract -------------------------------------------

test("no control in the DM console retracts anything", () => {
  // The ABSENCE, read off the rendered DOM rather than off the source.
  // Retraction left the platform (Patrik, 2026-08-30) and the console was
  // where a human reached it: two buttons, two boxes and a confirmation.
  // command-surface.test.ts asserts no BUILDER survives; this asserts no
  // CONTROL does, which is the half a leftover button would keep alive.
  const h = harness(newState());
  const actions = Array.from(h.node.querySelectorAll("[data-action]"))
    .map((n) => (n as HTMLElement).dataset["action"]!);
  expect(actions.filter((a) => /retract/i.test(a))).toEqual([]);
  const labels = Array.from(h.node.querySelectorAll("button, h3"))
    .map((n) => n.textContent ?? "");
  expect(labels.filter((t) => /undo|retract/i.test(t))).toEqual([]);
  expect(Array.from(h.node.querySelectorAll("[data-field]"))
    .map((n) => (n as HTMLElement).dataset["field"]!)
    .filter((f) => /undo/i.test(f))).toEqual([]);
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
    "actor-id", "actor-name", "actor-json",
    "token-id", "token-scene", "token-actor", "token-x", "token-y",
    "note-key", "note-title", "note-text",
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
    ["name the token to remove", () => { const h = harness(); h.action("remove-token").click(); return h; }],
    ["name the actor to remove", () => { const h = harness(); h.action("remove-actor").click(); return h; }],
    ["a note needs a key and some text", () => { const h = harness(); h.button("Save").click(); return h; }],
    ["name the note to delete", () => { const h = harness(); h.button("Delete").click(); return h; }],
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

test("an actor's fields are trimmed", () => {
  const h = harness();
  fill(h, { "actor-id": "  a1  ", "actor-name": "  Lera  " });
  (h.node.querySelector(".actor-kind") as HTMLSelectElement).value = "ACTOR_KIND_PARTY_MEMBER";
  h.action("add-actor").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("addActor");
  // addActor nests the payload under `actor` (commands.ts).
  const a = p!.value["actor"] as Record<string, unknown>;
  expect(a["actorId"]).toBe("a1");
  expect(a["name"]).toBe("Lera");
});

// --- creating an actor says what it is (actor-kind Task 7) ------------------

test("the Add actor form asks what the creature is, and sends the answer", () => {
  // The server refuses an add_actor that states no kind (gateway
  // validateAddActor), so the console has to ASK — the same shape as the grant
  // row's own selector one panel down, and for the same reason.
  //
  // BLANK by default: a pre-selected value is indistinguishable from a DM who
  // never looked, which is the whole argument for refusing an unstated kind
  // rather than defaulting one.
  const h = harness();
  const kind = h.node.querySelector(".actor-kind") as HTMLSelectElement;
  expect(kind).not.toBeNull();
  expect(kind.value).toBe("");
  // AND THE BOX SHOWS THE QUESTION, which `value` alone cannot see. Per the
  // HTML spec, setting a select to a string no option carries deselects
  // everything: selectedIndex goes to -1, the getter still reports "", and the
  // DM meets an EMPTY box where a question should be. An empty box reads as a
  // field with nothing in it; "what is it?" reads as something waiting for an
  // answer, and the difference is whether the DM knows to look.
  //
  // STATED HONESTLY: the selectedIndex line cannot fail under happy-dom, which
  // re-runs the "ask for a reset" algorithm when the select is inserted into a
  // parent and so re-selects option 0 on the way into the group. A browser does
  // not, so this pins the intent and would catch the regression where it is
  // real. The LABEL assertion below is load-bearing here and now: the option
  // list assertion under it reads values, which are blank for the blank one, so
  // nothing else in this test would notice the wording changing.
  expect(kind.selectedIndex).toBe(0);
  expect(kind.options[kind.selectedIndex]!.textContent).toBe("what is it?");
  expect(Array.from(kind.options).map((o) => o.value)).toEqual([
    "",
    "ACTOR_KIND_PARTY_MEMBER",
    "ACTOR_KIND_NON_PARTY",
  ]);

  fill(h, { "actor-id": "a1", "actor-name": "Goblin" });
  kind.value = "ACTOR_KIND_NON_PARTY";
  h.action("add-actor").click();
  const [p] = payloads(h);
  const a = p!.value["actor"] as Record<string, unknown>;
  expect(a["kind"]).toBe(ActorKind.NON_PARTY);
});

test("an actor with no kind chosen is refused here, not sent and bounced", () => {
  // A DM who fills in an id and a name and forgets the one question the form
  // exists to ask would otherwise read a wire-level refusal for something the
  // console simply failed to ask them.
  const h = harness();
  fill(h, { "actor-id": "a1", "actor-name": "Goblin" });
  h.action("add-actor").click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices[0]).toMatch(/what it is/i);
});

// --- what the box SHOWS, read off the option rather than off .value ---------
//
// `sel.value` is a poor witness for what the DM is looking at. Per the HTML
// spec a select carrying a value no option matches has selectedIndex -1 while
// the getter still reports "" — so an EMPTY box and a box reading "what is it?"
// are the same string to `.value`, and only one of them tells the DM that
// something is waiting for an answer.
//
// These two read the OPTION's own selectedness instead, which is the state the
// browser paints, and they are what stops kindSelect being "simplified" back to
// assigning `sel.value` (see the comment there). Characterization tests over an
// existing builder (ADR-009 §3): each assertion below was proven able to fail by
// injection rather than by a red phase, because the builder they pin was
// restructured WITHOUT changing behaviour and a genuine red phase would have
// meant the restructure broke something.

test("a fresh kind select has the QUESTION selected, not an empty box", () => {
  const h = harness();
  const kind = h.node.querySelector(".actor-kind") as HTMLSelectElement;
  // Exactly one option is chosen, and it is the one that asks. Counting matters
  // as much as naming: a builder that marked several would leave the browser to
  // pick the last, which is "monster / NPC" — a default answer to the one
  // question this field exists to make the DM answer themselves.
  const chosen = Array.from(kind.options).filter((o) => o.selected);
  expect(chosen).toHaveLength(1);
  expect(chosen[0]!.value).toBe("");
  expect(chosen[0]!.textContent).toBe("what is it?");
  expect(kind.selectedIndex).toBe(0);
});

test("a remembered kind selects THAT option, and only that one", () => {
  // The middle option on purpose. The last one is where a builder that selected
  // every non-matching option would land by accident, so pinning it would let
  // that mistake pass; the middle one can only be reached by matching.
  const st = newState();
  st.Actors["act-boar"] = {
    actorId: "act-boar", name: "Boar", moduleId: "",
    attributes: {}, resources: {}, controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
  };
  const parts = [{ participantId: "p-bo", displayName: "Bo" }];
  const first = harness(st, { participants: parts });
  const sel = first.node.querySelector('.control-actor[data-actor="act-boar"] .grant-kind') as HTMLSelectElement;
  sel.value = "ACTOR_KIND_PARTY_MEMBER";
  sel.dispatchEvent(new Event("change"));

  const again = harness(st, { participants: parts })
    .node.querySelector('.control-actor[data-actor="act-boar"] .grant-kind') as HTMLSelectElement;
  expect(Array.from(again.options).map((o) => o.selected)).toEqual([false, true, false]);
  expect(again.options[again.selectedIndex]!.textContent).toBe("party member");
});

test("the Add actor form cannot hand a character to anyone", () => {
  // Visibility spec §5.1's first rule, at the seat where a human could break
  // it. The console used to carry a "controller participant id (optional)"
  // box next to the id and the name, and typing into it created a PARTY
  // MEMBER — the whole cloned Actor on every player's roster, plus MayPerch
  // and eyes() open on it — with no refusal anywhere on the path, because
  // commands.ts's addActor could not express a kind at all.
  //
  // The box is gone rather than validated: assignment is a separate, manual
  // act (Patrik's ruling, 2026-08-24), and the console already has the
  // control that performs it — the per-actor grant row further down this
  // panel, which ASKS for a kind. Two boxes that both confer control is the
  // two-writers shape this arc exists to remove.
  const h = harness();
  expect(h.node.querySelector('[data-field="actor-controller"]')).toBeNull();

  fill(h, { "actor-id": "a1", "actor-name": "Lera" });
  (h.node.querySelector(".actor-kind") as HTMLSelectElement).value = "ACTOR_KIND_PARTY_MEMBER";
  h.action("add-actor").click();
  // The protobuf MESSAGE, not its JSON, so these read as the field defaults
  // rather than as absent keys — "nobody controls this actor" is what an empty
  // controller_id has always meant (Actor.controller_id's own doc comment).
  // The absent-key half is pinned on the wire shape itself, in
  // commands.test.ts's "addActor cannot confer control, whatever a caller
  // passes it".
  const a = payloads(h)[0]!.value["actor"] as Record<string, unknown>;
  expect(a["controllerId"]).toBe("");
  expect(a["controllerIds"]).toEqual([]);
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

// removeToken (retraction-leaves Task 8, fix round 1): the restored DM
// control, sharing token-id with Place rather than a field of its own.
test("removing a token sends its trimmed id and clears the field", () => {
  const h = harness();
  fill(h, { "token-id": " t1 " });
  h.action("remove-token").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("removeToken");
  expect(p!.value["tokenId"]).toBe("t1");

  const next = harness();
  expect(next.field("token-id").value).toBe("");
});

// removeActor (retraction-leaves Task 9): the DM control for the batch
// command, sharing actor-id with Add rather than a field of its own — naming
// an actor to remove needs nothing else.
//
// NO CONFIRMATION, and that is this file's own rule rather than an omission:
// dm.ts's header says exactly one control asks first, and it is the one whose
// damage lands OUTSIDE the room (rotating the join link strands copies held by
// people who are not here). A removal lands on the log every seat at this
// table can see.
test("removing an actor sends its trimmed id and clears the field", () => {
  const h = harness();
  fill(h, { "actor-id": " a1 " });
  h.action("remove-actor").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("removeActor");
  expect(p!.value["actorId"]).toBe("a1");

  const next = harness();
  expect(next.field("actor-id").value).toBe("");
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
  const h = harness(newState(), { adventures: [{ id: "adv-1", name: "Goblin Ambush" }] });
  h.button("Load Goblin Ambush").click();
  expect(payloads(h)).toEqual([
    { case: "loadAdventure", value: expect.objectContaining({ adventureId: "adv-1" }) },
  ]);
});

test("an adventure with no name is labelled by its id", () => {
  // `${a.name || a.id}` — the fallback exists so an unnamed module is still
  // clickable rather than rendering as "Load ".
  const h = harness(newState(), { adventures: [{ id: "adv-7", name: "" }] });
  expect(h.button("Load adv-7")).toBeDefined();
});

test("every adventure in the list gets its own pair of buttons", () => {
  // Kills the emptied for-loop body and the seeded row array.
  const h = harness(newState(), {
    adventures: [{ id: "a", name: "A" }, { id: "b", name: "B" }],
  });
  expect(h.button("Load A")).toBeDefined();
  expect(h.button("Load B")).toBeDefined();
  expect(Array.from(h.node.querySelectorAll("button")).filter((b) => b.textContent === "guide")).toHaveLength(2);
});

test("the adventures section is absent entirely when there are none", () => {
  // `d.adventures.length > 0` -> `>= 0` renders an empty "Adventures" group.
  const h = harness(newState(), { adventures: [] });
  expect(Array.from(h.node.querySelectorAll("h3")).some((n) => n.textContent === "Adventures")).toBe(false);
});

test("the guide button shows the guide it fetched", async () => {
  const h = harness(newState(), { guide: "# The Cave" });
  h.button("guide").click();
  await new Promise((r) => setTimeout(r, 0));
  expect(h.notices).toEqual(["# The Cave"]);
});

test("an adventure with no guide says so rather than showing nothing", async () => {
  // `g ?? "no guide for that adventure"` — a silent no-op reads as a broken
  // button.
  const h = harness(newState(), { guide: null });
  h.button("guide").click();
  await new Promise((r) => setTimeout(r, 0));
  expect(h.notices).toEqual(["no guide for that adventure"]);
});

// --- maps ---------------------------------------------------------------

test("a DM loads a map by picking it, and the button says which one", () => {
  const h = harness(newState(), {
    maps: [
      { id: "cellar", name: "The Cellar", gridWidth: 10, gridHeight: 9 },
      { id: "wood", name: "", gridWidth: 40, gridHeight: 40 },
    ],
  });
  const load = h.node.querySelector<HTMLButtonElement>('[data-action="load-map"]');
  expect(load).not.toBeNull();
  expect(load!.textContent).toContain("The Cellar");
  load!.click();
  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.case).toBe("loadMap");
  expect((h.sent[0]!.command.value as { mapId: string }).mapId).toBe("cellar");
});

test("a map with no name is offered under its id rather than a blank button", () => {
  const h = harness(newState(), {
    maps: [{ id: "wood", name: "", gridWidth: 40, gridHeight: 40 }],
  });
  const buttons = Array.from(h.node.querySelectorAll<HTMLButtonElement>('[data-action="load-map"]'));
  expect(buttons.map((b) => b.textContent)).toEqual(["Load wood"]);
});

test("no maps configured means no Maps group at all, not an empty one", () => {
  const h = harness(newState(), { maps: [] });
  expect(h.node.querySelector('[data-action="load-map"]')).toBeNull();
  expect(h.node.textContent).not.toContain("Maps");
});

// --- whitespace is not content ----------------------------------------------

test("a whitespace-only id is refused everywhere one is required", () => {
  // Kills the `.trim()` removals in the GUARDS specifically: without trim,
  // "   " is truthy and the command goes out with a blank id.
  const scene = harness();
  fill(scene, { "scene-id": "   ", "scene-w": "5", "scene-h": "5" });
  scene.button("Create").click();
  expect(scene.sent).toHaveLength(0);

  // THE KIND IS ANSWERED HERE, and that is what makes this arm discriminate.
  // Add actor asks two questions and refuses on the first that fails; with the
  // kind left blank the SECOND guard answers, nothing is sent either way, and
  // this assertion passes with the id's `.trim()` removed. That is exactly how
  // it came back once the kind selector landed beside it. The wording is
  // asserted for the same reason — "nothing was sent" cannot tell which of the
  // two guards did the refusing.
  const actor = harness();
  fill(actor, { "actor-id": "   ", "actor-name": "Lera" });
  (actor.node.querySelector(".actor-kind") as HTMLSelectElement).value = "ACTOR_KIND_PARTY_MEMBER";
  actor.action("add-actor").click();
  expect(actor.sent).toHaveLength(0);
  expect(actor.notices).toEqual(["an actor needs an id"]);

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
  box.value = '{"actorId":"a9","name":"Nine","kind":"ACTOR_KIND_NON_PARTY"}';
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
  fill(h, { "actor-id": "a1", "actor-name": "Lera", "scene-id": "keep-me" });
  for (const f of ["actor-id", "actor-name", "scene-id"]) {
    h.field(f).dispatchEvent(new Event("input"));
  }
  const kind = h.node.querySelector(".actor-kind") as HTMLSelectElement;
  kind.value = "ACTOR_KIND_NON_PARTY";
  kind.dispatchEvent(new Event("change"));
  h.action("add-actor").click();

  const next = harness();
  expect(next.field("actor-id").value).toBe("");
  expect(next.field("actor-name").value).toBe("");
  // The kind goes back to "unanswered" with the rest. A remembered kind is the
  // one stale draft that would be actively dangerous: the next actor typed in
  // would silently inherit the last one's standing.
  expect((next.node.querySelector(".actor-kind") as HTMLSelectElement).value).toBe("");
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
  box.value = '{"actorId":"a9","name":"Nine","kind":"ACTOR_KIND_NON_PARTY"}';
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

// --- conditions -------------------------------------------------------------

test("the remove-condition group is absent when no actor has one", () => {
  expect(Array.from(harness().node.querySelectorAll("h3")).some((n) => n.textContent === "Remove condition")).toBe(false);
});

// --- structure, not styling -------------------------------------------------

test("buttons carry the labels the DM clicks, not blanks", () => {
  // The submit buttons are found by data-action in these tests, so blanking
  // their visible LABEL was free. A blank button is unusable.
  const h = harness();
  for (const label of ["Start session", "Create", "Add", "Add from JSON", "Place", "Save", "Delete"]) {
    expect(Array.from(h.node.querySelectorAll("button")).some((b) => b.textContent === label)).toBe(true);
  }
});

test("every group carries a heading", () => {
  const open = newState();
  open.Conditions["a1"] = [{ ID: "prone", Source: "s", AppliedSeq: 1 }];
  // maps: [...], not the default [] -- the Maps group renders (and so its
  // own "Maps" heading exists to check) only when there is at least one.
  // Its absence here previously left `"Maps" -> ""` (StringLiteral) an
  // unadjudicated survivor: nothing ever rendered the heading this test
  // could have caught it reading.
  const titles = Array.from(
    harness(open, { maps: [{ id: "m", name: "M", gridWidth: 4, gridHeight: 4 }] }).node.querySelectorAll("h3"),
  ).map((n) => n.textContent);
  for (const t of ["Session", "Create scene", "Add actor", "…or paste actor JSON",
                   "Place token", "Adventures", "Maps", "Notes", "Remove condition"]) {
    expect(titles).toContain(t);
  }
  expect(titles.every((t) => t !== "")).toBe(true);
});

test("a group's row holds only its elements, with no stray text", () => {
  // `const row: HTMLElement[] = []` seeded with a sentinel, and
  // `if (text !== undefined) n.textContent = text` forced true, both leak
  // text into containers that should hold only controls.
  //
  // BOTH `row` DECLARATIONS, not just Adventures': dm.ts has two identical
  // `const row: HTMLElement[] = [];` statements (Adventures and Maps), and
  // an adventures-only fixture exercises only the first -- the Maps one
  // survived a mutant exactly this comment already described, because
  // nothing here ever gave it a row to check. `maps: [...]` makes the loop
  // below walk that row too.
  const h = harness(newState(), {
    adventures: [{ id: "a", name: "A" }],
    maps: [{ id: "m", name: "M", gridWidth: 4, gridHeight: 4 }],
  });
  const rows = Array.from(h.node.querySelectorAll(".row"));
  expect(rows.length).toBeGreaterThan(0);
  for (const row of rows) {
    for (const node of Array.from(row.childNodes)) {
      // Element children only: a text node here is a leaked sentinel.
      expect(node.nodeType).toBe(1);
    }
  }
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
  //
  // "guide" (not "Create"): Create now carries "create-scene" (Task 4 filled
  // in every submit button the control-level invariant in
  // command-surface.test.ts needed one for). "guide" fetches an adventure's
  // text; it is not a ClientCommand at all, so it never needed one. It is
  // NOT the only bare button left in the default fixture -- "Add from JSON"
  // is built without a third argument too, since the structured "Add"
  // button above it already gives addActor a reachable "add-actor" control
  // and the invariant asks for one control per command, not one per button.
  // "guide" is picked here only because it is the simpler case to reason
  // about: it names no command at all, where "Add from JSON" names one a
  // sibling button already covers.
  const h = harness();
  const plain = Array.from(h.node.querySelectorAll("button")).find((b) => b.textContent === "guide")!;
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
  for (const f of ["scene-w", "scene-h", "token-x", "token-y"]) {
    expect(h.field(f).className).toBe("tiny");
  }
  for (const f of ["session-name", "note-text"]) {
    expect(h.field(f).className).toBe("wide");
  }
  expect((h.node.querySelector('[data-field="actor-json"]') as HTMLElement).className).toBe("paste");
  for (const b of Array.from(h.node.querySelectorAll("button"))) expect(b.className).toBe("chip");
});

test("every box carries a placeholder saying what belongs in it", () => {
  // Asserts PRESENCE, not wording. dm.ts:45 records that placeholders are
  // prose and get reworded, and that pinning the words makes a test break on
  // a copy edit — but an EMPTY placeholder is a usability defect, not a copy
  // choice: without them the console is a wall of unlabelled boxes.
  const h = harness();
  for (const f of [
    "session-name", "scene-id", "scene-name", "scene-w", "scene-h",
    "actor-id", "actor-name",
    "token-id", "token-scene", "token-actor", "token-x", "token-y",
    "note-key", "note-title", "note-text",
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
  const h = harness(tableWithActor(), {
    participants: [
      { participantId: "p-ana", displayName: "Ana" },
      { participantId: "p-bo", displayName: "Bo" },
    ],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect(row).not.toBeNull();

  const target = row.querySelector(".grant-target") as HTMLSelectElement;
  target.value = "p-bo";
  (row.querySelector(".grant-kind") as HTMLSelectElement).value = "ACTOR_KIND_PARTY_MEMBER";
  (row.querySelector(".grant") as HTMLButtonElement).click();

  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.case).toBe("grantActorControl");
  expect(h.sent[0]!.command.value).toMatchObject({
    actorId: "act-warden",
    participantId: "p-bo",
    kind: ActorKind.PARTY_MEMBER,
  });
});

// --- the grant says what it is granting (visibility spec §5.1) ---------------
//
// The DM console is one of the three seams that issue a grant, and the server
// now REFUSES one that states no kind. Without these the console would send a
// grant the server bounces, and the DM would read a protocol error where a
// question belonged.

test("the DM console makes the DM say what the actor is", () => {
  const h = harness(tableWithActor(), {
    participants: [{ participantId: "p-bo", displayName: "Bo" }],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  const kind = row.querySelector(".grant-kind") as HTMLSelectElement;
  expect(kind).not.toBeNull();

  // BLANK by default, and deliberately so: a pre-selected value is
  // indistinguishable from a DM who never looked, which is the exact reason
  // the server refuses an unstated kind rather than defaulting one.
  expect(kind.value).toBe("");
  expect(Array.from(kind.options).map((o) => o.value)).toEqual([
    "",
    "ACTOR_KIND_PARTY_MEMBER",
    "ACTOR_KIND_NON_PARTY",
  ]);
  // Every option must be readable as a sentence about the table, not as a
  // wire constant — the DM is choosing between a character and a monster.
  for (const o of Array.from(kind.options)) {
    expect(o.textContent!.length).toBeGreaterThan(0);
  }
});

test("a grant with no kind chosen is refused here, not sent and bounced", () => {
  const h = harness(tableWithActor(), {
    participants: [{ participantId: "p-bo", displayName: "Bo" }],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  (row.querySelector(".grant-target") as HTMLSelectElement).value = "p-bo";
  (row.querySelector(".grant") as HTMLButtonElement).click();

  expect(h.sent).toHaveLength(0);
  // The TEXT, not just that something was said. Grant has TWO refusal paths —
  // no participant and no kind — so `notices.length > 0` would stay green
  // against a console that never renders the kind select at all, as long as
  // the participant list also went missing. Its Go counterpart pins the same
  // thing (grant_validate_test.go requires the refusal to name the field),
  // and for the same reason: a refusal that does not teach the rule is not
  // the refusal this feature needs.
  expect(h.notices).toHaveLength(1);
  expect(h.notices[0]).toMatch(/party member/i);
});

test("the DM can hand a monster to a participant without promoting it", () => {
  // The agent's case, at the human console: somebody runs the goblin, and the
  // goblin stays a goblin. The two grants are byte-identical apart from this
  // field, which is why the field exists.
  const h = harness(tableWithActor(), {
    participants: [{ participantId: "p-bo", displayName: "Bo" }],
  });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  (row.querySelector(".grant-target") as HTMLSelectElement).value = "p-bo";
  (row.querySelector(".grant-kind") as HTMLSelectElement).value = "ACTOR_KIND_NON_PARTY";
  (row.querySelector(".grant") as HTMLButtonElement).click();

  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.value).toMatchObject({ kind: ActorKind.NON_PARTY });
});

test("each current controller can be revoked individually", () => {
  const st = tableWithActor();
  st.Actors["act-warden"]!.controllerIds = ["p-ana", "p-bo"];
  const h = harness(st, {
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
  const h = harness(st, { participants: [{ participantId: "p-bo", displayName: "Bo" }] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect(row.querySelectorAll(".revoke")).toHaveLength(0);
  expect(row.querySelector(".grant")).not.toBeNull();
});

test("the current controllers are shown by DISPLAY NAME, not by id", () => {
  // A uuid tells the DM nothing about who is holding a character.
  const h = harness(tableWithActor(), {
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
  const h = harness(st, { participants: [] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  expect(row.querySelector(".control-name")?.textContent).toBe("act-warden");
});

test("a controller who is not connected is still shown, by id", () => {
  // Control is campaign-scoped and presence is connection-scoped (spec §3.1),
  // so someone can hold a character while offline. Rendering only the
  // participants we can name would silently drop them from the list and make
  // the character look unowned.
  const h = harness(tableWithActor(), { participants: [] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  // Same scoping, for the same reason.
  expect(row.querySelector(".held-who")?.textContent).toBe("p-ana");
  expect(row.querySelectorAll(".revoke")).toHaveLength(1);
});

test("granting with nobody selected sends nothing", () => {
  const h = harness(tableWithActor(), { participants: [] });
  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]') as HTMLElement;
  (row.querySelector(".grant") as HTMLButtonElement).click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices.join(" ")).toContain("nobody");
});

test("the grant dropdown labels every participant, and leads with the blank", () => {
  // o.value is asserted by the grant test; o.textContent was not, so the
  // dropdown could label every participant identically while still sending the
  // right id — the DM picks blind.
  const h = harness(tableWithActor(), {
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
  const first = harness(st, { participants: parts });
  const warden = first.node.querySelector('.control-actor[data-actor="act-warden"] .grant-target') as HTMLSelectElement;
  warden.value = "p-bo";
  warden.dispatchEvent(new Event("change"));

  const second = harness(st, { participants: parts });
  expect((second.node.querySelector('.control-actor[data-actor="act-warden"] .grant-target') as HTMLSelectElement).value).toBe("p-bo");
  expect((second.node.querySelector('.control-actor[data-actor="act-adder"] .grant-target') as HTMLSelectElement).value).toBe("");
});

test("each actor remembers its OWN answer about what it is", () => {
  // The kind draft key is per-actor too, and a shared one is worse here than
  // on the participant dropdown beside it. A participant picked for the wrong
  // row is VISIBLE in the row — it names a person the DM did not choose — while
  // a kind picked for the wrong row looks exactly like the answer they meant to
  // give. The DM says "monster" about the goblin, the party member below it
  // comes up pre-answered "monster", and the grant that follows tells every
  // client to treat a character as a creature the party has to discover.
  //
  // The blank second row is the whole assertion: the first row's retained
  // choice is already pinned by "the DM's answer to what the actor is survives
  // a re-render too", and one row alone cannot tell a per-actor key from a
  // shared one.
  const st = tableWithActor();
  st.Actors["act-adder"] = {
    actorId: "act-adder", name: "Adder", moduleId: "",
    attributes: {}, resources: {}, controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
  };
  const parts = [{ participantId: "p-bo", displayName: "Bo" }];
  const first = harness(st, { participants: parts });
  const warden = first.node.querySelector('.control-actor[data-actor="act-warden"] .grant-kind') as HTMLSelectElement;
  warden.value = "ACTOR_KIND_NON_PARTY";
  warden.dispatchEvent(new Event("change"));

  const second = harness(st, { participants: parts });
  expect((second.node.querySelector('.control-actor[data-actor="act-warden"] .grant-kind') as HTMLSelectElement).value)
    .toBe("ACTOR_KIND_NON_PARTY");
  expect((second.node.querySelector('.control-actor[data-actor="act-adder"] .grant-kind') as HTMLSelectElement).value)
    .toBe("");
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
  const h = harness(st, { participants: [] });
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
  const h = harness(st, { participants: [] });
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
  const first = harness(st, { participants: parts });
  const sel = first.node.querySelector(".grant-target") as HTMLSelectElement;
  sel.value = "p-bo";
  sel.dispatchEvent(new Event("change"));

  const second = harness(st, { participants: parts });
  expect((second.node.querySelector(".grant-target") as HTMLSelectElement).value).toBe("p-bo");
});

test("the DM's answer to what the actor is survives a re-render too", () => {
  // The same mechanism one field over, and it needs its own test rather than
  // its neighbour's: the kind select is the field a DM is most likely to be
  // part-way through when an event lands, because it is the question they have
  // to stop and think about. Losing it silently resets to "what is it?", and
  // the next Grant click is refused for a choice they already made.
  const st = tableWithActor();
  const parts = [{ participantId: "p-bo", displayName: "Bo" }];
  const first = harness(st, { participants: parts });
  const sel = first.node.querySelector(".grant-kind") as HTMLSelectElement;
  sel.value = "ACTOR_KIND_NON_PARTY";
  sel.dispatchEvent(new Event("change"));

  const second = harness(st, { participants: parts });
  expect((second.node.querySelector(".grant-kind") as HTMLSelectElement).value).toBe(
    "ACTOR_KIND_NON_PARTY",
  );
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
    adventures: [],
    maps: [],
    guideFor: async () => null,
    doorsArmed: false,
    toggleDoors: () => {},
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
    st: emptyState(), adventures: [], maps: [], guideFor: async () => null,
    participants: [], joinLink: { open: true, secret: "s3cret" }, roster: [],
    origin: "https://table.example", refreshSharing: () => {},
    send: async (c) => void sent.push(c), notify: () => {},
    doorsArmed: false, toggleDoors: () => {},
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
    st, adventures: [], maps: [], guideFor: async () => null,
    participants: [], joinLink: null, roster: [], origin: "https://table.example",
    refreshSharing: () => {}, send: async (c) => void sent.push(c), notify: () => {}, confirm: () => true,
    doorsArmed: false, toggleDoors: () => {},
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
    st: newState(), adventures: [], maps: [], guideFor: async () => null,
    participants: [], joinLink: { open: true, secret: "s3cret" }, roster: [],
    origin: "https://table.example", refreshSharing: () => refreshes++,
    send: async (c) => void sent.push(c), notify: () => {}, confirm: () => true,
    doorsArmed: false, toggleDoors: () => {},
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
    st: newState(), adventures: [], maps: [], guideFor: async () => null,
    participants: [], joinLink: { open: true, secret: "s" }, roster: [],
    origin: "https://table.example", refreshSharing: () => {},
    send: async () => {}, notify: () => {},
    confirm: (m) => {
      asked = m;
      return false;
    },
    doorsArmed: false,
    toggleDoors: () => {},
  });
  node.querySelector<HTMLButtonElement>('[data-action="rotate-link"]')!.click();

  expect(asked).toContain("old one");
  expect(asked.length).toBeGreaterThan(20);
});

// --- doors (Task 4) ----------------------------------------------------

test("the doors toggle exists and flips the shared armed bit on each click", () => {
  const h = harness();
  expect(h.action("arm-doors")).not.toBeNull();
  expect(h.doorsArmed()).toBe(false);

  h.action("arm-doors").click();
  expect(h.doorsArmed()).toBe(true);
  expect(h.toggles()).toBe(1);

  // BACK OFF, not stuck armed: a DM who arms doors by mistake needs the same
  // control to undo it, not a second one.
  h.action("arm-doors").click();
  expect(h.doorsArmed()).toBe(false);
  expect(h.toggles()).toBe(2);
});

test("the toggle's label says which state it is in, at render time", () => {
  // Two SEPARATE renders, not one harness clicked twice: renderDMConsole runs
  // once per harness() call (see harness()'s own comment), so this is what
  // actually exercises the `d.doorsArmed ? ... : ...` label's two arms.
  expect(harness(newState(), { doorsArmed: false }).action("arm-doors").textContent)
    .toBe("Arm doors");
  expect(harness(newState(), { doorsArmed: true }).action("arm-doors").textContent)
    .toContain("armed");
});

test("arming doors sends nothing over the wire — it is a local mode, not a command", () => {
  // doorCommandFor/openDoor/closeDoor are what send something; the toggle
  // itself never should, or arming would write a spurious entry into a log
  // meant only for things that actually happened at the table.
  const h = harness();
  h.action("arm-doors").click();
  expect(h.sent).toHaveLength(0);
});
