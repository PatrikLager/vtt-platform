import "./support/dom"; // see that module: registers once, keeps real fetch/WebSocket

import { test, expect, beforeEach, spyOn } from "bun:test";
import { newState, type State } from "../src/state";
import { ActorKind, NoteVisibility } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { Roster, MapMeta } from "../src/metadata";
import { renderDMConsole } from "../src/view/dm";
import joinURL from "../../contract/testdata/join_url_format.json";
import {
  JoinDoor, type ClientCommand, type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { held, ok, refused, settle } from "./support/answers";

function harness(
  st: State = newState(),
  opts: {
    adventures?: { id: string; name: string }[];
    maps?: MapMeta[];
    guide?: string | null;
    participants?: { participantId: string; displayName: string }[];
    roster?: Roster[];
    doorsArmed?: boolean;
    // What the server answers each command: a result, "held" until the test
    // calls release(), or an Error the send rejects with. ok() when unset.
    answer?: CommandResult | "held" | Error;
  } = {},
) {
  const sent: ClientCommand[] = [];
  const notices: string[] = [];
  const answer = opts.answer ?? ok();
  const waiting = held();
  // Counted, as toggles are: the console asks for a repaint, and app.ts's
  // paint() is what would draw it.
  let repaints = 0;
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
    // DERIVED from participants unless a test says otherwise, because that is
    // what the tests written before the two were told apart already meant:
    // "these people are at the table". Only the roster/presence split tests
    // below pass them separately, which is the whole point of the split.
    roster: opts.roster ?? (opts.participants ?? []).map((p) => ({
      participantId: p.participantId, name: p.displayName, role: "player" as const,
    })),
    origin: "https://table.example",
    refreshSharing: () => {},
    send: async (c) => {
      sent.push(c);
      if (answer === "held") return waiting.send(c);
      if (answer instanceof Error) throw answer;
      return answer;
    },
    rerender: () => {
      repaints++;
    },
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
    repaints: () => repaints,
    release: waiting.release,
    settle,
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
  first.field("token-scene").value = "cave";
  first.field("token-scene").dispatchEvent(new Event("input"));

  // A second render is what an arriving event causes.
  const second = harness();
  expect(second.field("token-scene").value).toBe("cave");
});

test("a submitted session name is not restored on the next render", async () => {
  const h = harness();
  h.field("session-name").value = "Night One";
  h.field("session-name").dispatchEvent(new Event("input"));
  h.action("start-session").click();
  expect(h.sent).toHaveLength(1);
  await h.settle();

  expect(harness().field("session-name").value).toBe("");
});

// VTT-517
test("a refused Start session keeps the name as typed, whatever the refusal", async () => {
  try {
    for (const why of ["a session is already open", "wire: not connected"]) {
      const h = harness(newState(), { answer: refused(why) });
      h.field("session-name").value = "Night One";
      h.field("session-name").dispatchEvent(new Event("input"));
      h.action("start-session").click();
      await h.settle();

      expect(h.sent).toHaveLength(1);
      expect(h.repaints()).toBe(0);
      expect(harness().field("session-name").value).toBe("Night One");
    }
  } finally {
    type(harness(), { "session-name": "" });
  }
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
    "session-name",
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
    ["an actor needs an id", () => { const h = harness(); h.action("add-actor").click(); return h; }],
    ["a token needs an id, a scene and an actor", () => { const h = harness(); h.action("place-token").click(); return h; }],
    ["name the token to remove", () => { const h = harness(); h.action("remove-token").click(); return h; }],
    ["name the actor to remove", () => { const h = harness(); h.action("remove-actor").click(); return h; }],
    ["a note needs a key and some text", () => { const h = harness(); h.button("Save").click(); return h; }],
    ["Say who may read it: every player and spectator, or the DM alone.", () => {
      const h = harness(); fill(h, { "note-key": "k", "note-text": "t" }); h.button("Save").click(); return h;
    }],
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
// browser paints, and they are what stops choiceSelect being "simplified" back to
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
  // MEMBER with no refusal anywhere on the path, because commands.ts's
  // addActor could not express a kind at all.
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
test("removing a token sends its trimmed id and clears the field", async () => {
  const h = harness();
  fill(h, { "token-id": " t1 " });
  // THROUGH THE EVENT, the way "placing a token clears exactly the token
  // fields" does it further down. `fill` assigns the DOM value only, and
  // dm.ts's draft buffer is written by the `input` handler — so without this
  // line the draft was never populated and the "clears the field" half of
  // this test passed whether or not anything was cleared. That vacuum is how
  // this handler's clearDraft call survived mutation (measured 2026-09-01).
  h.field("token-id").dispatchEvent(new Event("input"));
  h.action("remove-token").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("removeToken");
  expect(p!.value["tokenId"]).toBe("t1");
  await h.settle();

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
test("removing an actor sends its trimmed id and clears the field", async () => {
  const h = harness();
  fill(h, { "actor-id": " a1 " });
  // Through the event, for the reason the token twin above states.
  h.field("actor-id").dispatchEvent(new Event("input"));
  h.action("remove-actor").click();
  const [p] = payloads(h);
  expect(p!.case).toBe("removeActor");
  expect(p!.value["actorId"]).toBe("a1");
  await h.settle();

  const next = harness();
  expect(next.field("actor-id").value).toBe("");
});

// VTT-229
test("a note is not sent until who may read it is chosen", () => {
  const h = harness();
  fill(h, { "note-key": "k", "note-text": "body" });
  const vis = h.node.querySelector(".note-visibility") as HTMLSelectElement;
  expect(vis.options[vis.selectedIndex]!.value).toBe("");
  h.button("Save").click();
  expect(h.sent).toHaveLength(0);
  expect(h.notices).toEqual(["Say who may read it: every player and spectator, or the DM alone."]);
});

// VTT-229
test("a saved note carries the visibility chosen", async () => {
  for (const [wire, want] of [
    ["NOTE_VISIBILITY_PUBLIC", NoteVisibility.PUBLIC],
    ["NOTE_VISIBILITY_SECRET", NoteVisibility.SECRET],
  ] as const) {
    const h = harness();
    fill(h, { "note-key": "k", "note-text": "body" });
    (h.node.querySelector(".note-visibility") as HTMLSelectElement).value = wire;
    h.button("Save").click();
    expect(payloads(h)).toEqual([
      { case: "upsertNote", value: expect.objectContaining({ key: "k", visibility: want }) },
    ]);
    await h.settle();
  }
});

// VTT-229
test("a choice is not carried to the next note once the server accepts one", async () => {
  const h = harness();
  fill(h, { "note-key": "k", "note-text": "body" });
  const vis = h.node.querySelector(".note-visibility") as HTMLSelectElement;
  vis.value = "NOTE_VISIBILITY_SECRET";
  vis.dispatchEvent(new Event("change"));
  h.button("Save").click();
  await h.settle();
  expect(h.sent).toHaveLength(1);

  const next = harness();
  fill(next, { "note-key": "k", "note-text": "body" });
  expect((next.node.querySelector(".note-visibility") as HTMLSelectElement).value).toBe("");
  next.button("Save").click();
  expect(next.sent).toHaveLength(0);
});

test("a note's key, title and text are trimmed on the way out", () => {
  const h = harness();
  fill(h, { "note-key": " k ", "note-title": " T ", "note-text": " body " });
  (h.node.querySelector(".note-visibility") as HTMLSelectElement).value = "NOTE_VISIBILITY_PUBLIC";
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
      { id: "cellar", name: "The Cellar", gridWidth: 10, gridHeight: 9, cellPx: 64 },
      { id: "wood", name: "", gridWidth: 40, gridHeight: 40, cellPx: 64 },
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
    maps: [{ id: "wood", name: "", gridWidth: 40, gridHeight: 40, cellPx: 64 }],
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

  // The two Remove controls, which share their id boxes with Add and Place.
  // Without the trim, "   " is truthy and the removal goes out naming a blank
  // id — a command the server answers with an error the DM never asked for,
  // for a control that had already decided to refuse. Both trims survived
  // mutation until this walked them (measured 2026-09-01). The wording is
  // asserted for this test's own stated reason: "nothing was sent" cannot say
  // which guard refused.
  const removeActor = harness();
  fill(removeActor, { "actor-id": "   " });
  removeActor.action("remove-actor").click();
  expect(removeActor.sent).toHaveLength(0);
  expect(removeActor.notices).toEqual(["name the actor to remove"]);

  const removeToken = harness();
  fill(removeToken, { "token-id": "   " });
  removeToken.action("remove-token").click();
  expect(removeToken.sent).toHaveLength(0);
  expect(removeToken.notices).toEqual(["name the token to remove"]);
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

test("adding an actor clears exactly the actor fields", async () => {
  const h = harness();
  fill(h, { "actor-id": "a1", "actor-name": "Lera", "token-scene": "keep-me" });
  for (const f of ["actor-id", "actor-name", "token-scene"]) {
    h.field(f).dispatchEvent(new Event("input"));
  }
  const kind = h.node.querySelector(".actor-kind") as HTMLSelectElement;
  kind.value = "ACTOR_KIND_NON_PARTY";
  kind.dispatchEvent(new Event("change"));
  h.action("add-actor").click();
  await h.settle();

  const next = harness();
  expect(next.field("actor-id").value).toBe("");
  expect(next.field("actor-name").value).toBe("");
  // The kind goes back to "unanswered" with the rest. A remembered kind is the
  // one stale draft that would be actively dangerous: the next actor typed in
  // would silently inherit the last one's standing.
  expect((next.node.querySelector(".actor-kind") as HTMLSelectElement).value).toBe("");
  // A field belonging to another form must survive.
  expect(next.field("token-scene").value).toBe("keep-me");
});

test("placing a token clears exactly the token fields", async () => {
  const h = harness();
  fill(h, { "token-id": "t1", "token-scene": "s1", "token-actor": "a1", "token-x": "2", "token-y": "3", "note-key": "keep-me" });
  for (const f of ["token-id", "token-scene", "token-actor", "token-x", "token-y", "note-key"]) {
    h.field(f).dispatchEvent(new Event("input"));
  }
  h.action("place-token").click();
  await h.settle();

  const next = harness();
  for (const f of ["token-id", "token-scene", "token-actor", "token-x", "token-y"]) {
    expect(next.field(f).value).toBe("");
  }
  expect(next.field("note-key").value).toBe("keep-me");
});

test("adding an actor from JSON clears the paste box", async () => {
  const h = harness();
  const box = h.node.querySelector('[data-field="actor-json"]') as HTMLTextAreaElement;
  box.value = '{"actorId":"a9","name":"Nine","kind":"ACTOR_KIND_NON_PARTY"}';
  box.dispatchEvent(new Event("input"));
  h.button("Add from JSON").click();
  await h.settle();

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
  //
  // THE NAMED LIST BELOW IS NOT THE GUARD, and treating it as one is what let
  // both Remove buttons be blanked with this test green (measured 2026-09-01):
  // a list exempts every control added after it was written, and updating it
  // is the edit nobody makes. The list pins the labels a DM is told to look
  // for; the loop after it is the rule.
  const h = harness();
  for (const label of ["Start session", "Add", "Add from JSON", "Place", "Save", "Delete", "Remove"]) {
    expect(Array.from(h.node.querySelectorAll("button")).some((b) => b.textContent === label)).toBe(true);
  }
  const buttons = Array.from(h.node.querySelectorAll("button"));
  expect(buttons.length).toBeGreaterThan(7);
  for (const b of buttons) expect((b.textContent ?? "").trim()).not.toBe("");
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
    harness(open, { maps: [{ id: "m", name: "M", gridWidth: 4, gridHeight: 4, cellPx: 64 }] }).node.querySelectorAll("h3"),
  ).map((n) => n.textContent);
  for (const t of ["Session", "Add actor", "…or paste actor JSON",
                   "Place token", "Adventures", "Maps", "Notes", "Remove condition"]) {
    expect(titles).toContain(t);
  }
  expect(titles.every((t) => t !== "")).toBe(true);
});

test("a group's row holds only its elements, with no stray text", () => {
  // `const row: HTMLElement[] = []` seeded with a sentinel leaks that sentinel
  // as text into a row that should hold only controls.
  //
  // BOTH `row` DECLARATIONS, not just Adventures': dm.ts has two identical
  // `const row: HTMLElement[] = [];` statements (Adventures and Maps), and
  // an adventures-only fixture exercises only the first -- the Maps one
  // survived a mutant exactly this comment already described, because
  // nothing here ever gave it a row to check. `maps: [...]` makes the loop
  // below walk that row too.
  const h = harness(newState(), {
    adventures: [{ id: "a", name: "A" }],
    maps: [{ id: "m", name: "M", gridWidth: 4, gridHeight: 4, cellPx: 64 }],
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
  expect(h.field("actor-id").className).toBe("");
  // `if (cls)` -> true assigns "", which leaves an empty class attribute.
  expect(h.field("actor-id").hasAttribute("class")).toBe(false);
  expect(h.field("session-name").className).toBe("wide");
});

test("a button created without an action carries no data-action", () => {
  // `if (action) b.dataset["action"] = action` -> true stamps
  // data-action="undefined" on every plain button, which these tests select by.
  //
  // "guide" (not one of the submit buttons): every submit button carries a
  // data-action, because the control-level invariant in
  // command-surface.test.ts needs one for each command. "guide" fetches an
  // adventure's text; it is not a ClientCommand at all, so it never needed
  // one. (This sentence used to name "Create", the Create-scene button, which
  // left the console on 2026-09-02 with create_scene itself.) It is
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
  // A group's own children are its heading and its row, both elements: a text
  // node here is text written into the group itself.
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
  for (const f of ["token-x", "token-y"]) {
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
    "session-name",
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
    send: async (c) => { sent.push(c); return ok(); },
    rerender: () => {},
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
    send: async (c) => { sent.push(c); return ok(); }, rerender: () => {}, notify: () => {},
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
    refreshSharing: () => {}, send: async (c) => { sent.push(c); return ok(); }, notify: () => {}, confirm: () => true,
    rerender: () => {},
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
    send: async (c) => { sent.push(c); return ok(); }, notify: () => {}, confirm: () => true,
    rerender: () => {},
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
    send: async () => ok(), rerender: () => {}, notify: () => {},
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

// --- the roster/presence split (Patrik's ruling, 2026-09-06) ----------------
//
// Control is DURABLE: it lives in the log as controllerIds and outlives any
// connection. Presence is connection-scoped and lives only in memory. The
// console used to resolve BOTH the grant list and a holder's name through
// presence, which made durable control look broken — a held character read as
// held by a uuid, and a character could not be handed to anyone who was not
// dialled in at that instant.
//
// MapTool settles the design (CLAUDE.md rule 9): Token.ownerList is a Set of
// player names ON THE TOKEN, saved with the campaign, and removeOwner is called
// from exactly one place in that codebase — a human picking it from a
// right-click menu. Its disconnect path touches the player list and never the
// owner list. Twenty years, never wired together.

test("a character can be handed to someone who is not connected", () => {
  // THE SESSION-PREP CASE: a DM assigns characters before anyone dials in.
  // The server has never disagreed — controls() in internal/gateway/authz.go
  // tests id membership and asks nothing about presence, and
  // validateGrantActorControl checks only that the grant states a kind.
  const h = harness(tableWithActor(), {
    participants: [],
    roster: [{ participantId: "p-robin", name: "Robin", role: "player" }],
  });

  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]')!;
  const target = row.querySelector(".grant-target") as HTMLSelectElement;
  // Array.from, not spread: HTMLOptionsCollection is not iterable under
  // this tsconfig, and bun test accepts the spread while client:typecheck
  // rejects it — a green suite and a red gate.
  expect(Array.from(target.options).map((o) => o.value)).toContain("p-robin");

  // DRIVEN, not merely present: a select whose option exists but whose value
  // will not take is still a console that cannot grant.
  target.value = "p-robin";
  (row.querySelector(".grant-kind") as HTMLSelectElement).value = "ACTOR_KIND_PARTY_MEMBER";
  (row.querySelector("button.grant") as HTMLButtonElement).click();
  expect(h.sent).toHaveLength(1);
  expect(h.sent[0]!.command.value).toMatchObject({
    actorId: "act-warden", participantId: "p-robin",
  });
});

test("a holder who is away is named, not printed as a participant id", () => {
  // tableWithActor gives act-warden to p-ana. With p-ana off the wire the
  // console must still name them: a character held by "p-ana" reads as held by
  // nobody legible, which is what made durable control look like a bug.
  const h = harness(tableWithActor(), {
    participants: [],
    roster: [{ participantId: "p-ana", name: "Ana", role: "player" }],
  });

  const held = h.node.querySelector('.control-actor[data-actor="act-warden"] .held-who')!;
  expect(held.textContent).toBe("Ana");
  expect(held.textContent).not.toContain("p-ana");
});

test("presence marks a holder away, and says nothing when they are here", () => {
  // BOTH DIRECTIONS. A marker that is always on and a marker that is never on
  // both pass a one-sided check — the same shape that once produced two
  // byte-identical screenshots named for opposite states.
  const roster: Roster[] = [{ participantId: "p-ana", name: "Ana", role: "player" }];
  const away = harness(tableWithActor(), { participants: [], roster });
  const here = harness(tableWithActor(), {
    participants: [{ participantId: "p-ana", displayName: "Ana" }], roster,
  });

  expect(away.node.querySelector(".control-actor .held .away")!.textContent).toBe("away");
  expect(here.node.querySelector(".control-actor .held .away")).toBeNull();
});

// The four rules the union actually has. Every fixture above derives the roster
// FROM presence, so the two sets are identical and none of these rules is
// exercised by any of them — the degenerate-fixture shape, where a mutant
// survives at full line coverage because no input tells the arms apart.

test("someone in presence but not yet on the roster can still be granted", () => {
  // THE SHARE-LINK GUEST: they are on the wire before a refetched roster knows
  // them. This is the whole reason the union has a second half, and without it
  // the console would refuse to hand a character to somebody it can see.
  const h = harness(tableWithActor(), {
    roster: [],
    participants: [{ participantId: "p-guest", displayName: "Robin" }],
  });

  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]')!;
  const target = row.querySelector(".grant-target") as HTMLSelectElement;
  expect(Array.from(target.options).map((o) => o.textContent))
    .toEqual(["choose a participant", "Robin"]);
});

test("when the roster and presence disagree about a name, the roster wins", () => {
  // The roster is identity; presence is whatever a client announced. Naming
  // from presence would let the displayed name drift from the one the DM
  // administers in "Who may do what" two panels down.
  const h = harness(tableWithActor(), {
    roster: [{ participantId: "p-ana", name: "Ana Marsh", role: "player" }],
    participants: [{ participantId: "p-ana", displayName: "ana" }],
  });

  const row = h.node.querySelector('.control-actor[data-actor="act-warden"]')!;
  expect(row.querySelector(".held-who")!.textContent).toBe("Ana Marsh");
  expect(Array.from((row.querySelector(".grant-target") as HTMLSelectElement).options)
    .map((o) => o.textContent)).toEqual(["choose a participant", "Ana Marsh"]);
});

test("an EMPTY roster name falls through to the one presence announced", () => {
  // identity.CreateInvite does not validate the name, so `--name ""` is
  // reachable. Nullish-coalescing would render a blank where a name belongs,
  // which reads as nobody rather than as somebody unnamed.
  const h = harness(tableWithActor(), {
    roster: [{ participantId: "p-ana", name: "", role: "player" }],
    participants: [{ participantId: "p-ana", displayName: "Ana" }],
  });

  expect(h.node.querySelector(".control-actor .held-who")!.textContent).toBe("Ana");
});

test("the grant list is sorted by name across BOTH sources", () => {
  // Zoe comes from the roster and is inserted FIRST; Ana comes from presence
  // and is inserted second. Insertion order would show Zoe first, so this is
  // what makes the sort observable at all.
  // The ids run OPPOSITE to the names on purpose: Zoe is p-a and Ana is p-b.
  // With ids ordered the same way as names, a comparator that ignored names
  // entirely and sorted by id would produce this same list, and the name arm
  // would be unobservable — which is exactly what the first version of this
  // fixture did.
  const h = harness(tableWithActor(), {
    roster: [{ participantId: "p-a", name: "Zoe", role: "player" }],
    participants: [{ participantId: "p-b", displayName: "Ana" }],
  });

  const target = h.node.querySelector(".grant-target") as HTMLSelectElement;
  expect(Array.from(target.options).map((o) => o.textContent))
    .toEqual(["choose a participant", "Ana", "Zoe"]);
  expect(Array.from(target.options).map((o) => o.value)).toEqual(["", "p-b", "p-a"]);
});

test("two people sharing a display name are ordered by participant id", () => {
  // Asserted on VALUE, not text: the tie-break is invisible in the labels, so a
  // text assertion passes whichever way the comparator points.
  const h = harness(tableWithActor(), {
    // SCRAMBLED insertion order, and neither ascending nor descending would do.
    // A comparator that loses its tie-break collapses to a CONSTANT, and the
    // two constants fail in opposite directions: jammed at -1 it reverses the
    // input, jammed at +1 it is a stable no-op that returns the input
    // untouched. So an ascending fixture is killed by the first and passed by
    // the second, a descending one exactly the other way round, and each looks
    // like a fixed test while leaving the other half unpinned. The insertion
    // order must be neither the expected order nor its reverse.
    roster: [
      { participantId: "p-b", name: "Sam", role: "player" },
      { participantId: "p-c", name: "Sam", role: "player" },
      { participantId: "p-a", name: "Sam", role: "player" },
    ],
    participants: [],
  });

  const target = h.node.querySelector(".grant-target") as HTMLSelectElement;
  expect(Array.from(target.options).map((o) => o.value)).toEqual(["", "p-a", "p-b", "p-c"]);
});

test("a holder in neither the roster nor presence is marked away", () => {
  // The one holder the console cannot name. They are also the person most
  // certainly not at the table, so the marker matters most here — and a guard
  // that skipped them left the uuid reading as somebody present.
  const h = harness(tableWithActor(), { roster: [], participants: [] });

  const held = h.node.querySelector(".control-actor .held")!;
  expect(held.querySelector(".held-who")!.textContent).toBe("p-ana");
  // The TEXT, not just the element: an empty marker occupies the DOM and says
  // nothing, which is indistinguishable from no marker to the person reading it.
  expect(held.querySelector(".away")!.textContent).toBe("away");
});

test("a holder whose only name is empty falls back to the id, not to blank", () => {
  // DISTINCT from the fall-through test above, and the distinction is the whole
  // point: there the participant is ALSO in presence, so the union resolves the
  // name before the display ever asks. Here the roster is the only source and
  // its name is empty, which is the one state that reaches held-who's own
  // fallback. A blank there reads as a character held by nobody — the exact
  // misreading the uuid fallback exists to prevent.
  const h = harness(tableWithActor(), {
    roster: [{ participantId: "p-ana", name: "", role: "player" }],
    participants: [],
  });

  expect(h.node.querySelector(".control-actor .held-who")!.textContent).toBe("p-ana");
});

// --- describing tokens --------------------------------------------------------

function tableWithTokens(...ids: string[]): State {
  const st = newState();
  for (const id of ids) st.Tokens[id] = { ID: id, SceneID: "s1", ActorID: "a1", X: 0, Y: 0 };
  return st;
}

// VTT-464
test("the console draws a describe row per token of the world, by id", () => {
  const h = harness(tableWithTokens("t2", "t1"));
  const rows = Array.from(h.node.querySelectorAll(".describe-row")) as HTMLElement[];
  expect(rows.map((r) => r.dataset["token"])).toEqual(["t1", "t2"]);
  const g = rows[0]!.parentElement!;
  expect(g.className).toBe("dmgroup");
  expect(g.querySelector("h3")!.textContent).toBe("Describe tokens");
  for (const r of rows) {
    const id = r.dataset["token"]!;
    expect(r.className).toBe("row describe-row");
    expect(r.querySelector(".describe-id")!.textContent).toBe(id);
    for (const [f, cls] of [["name", ""], ["notes", "wide"], ["gm-name", ""], ["gm-notes", "wide"]] as const) {
      const box = r.querySelector(`[data-field="describe-${f}:${id}"]`) as HTMLInputElement;
      expect(box.placeholder.length).toBeGreaterThan(0);
      expect(box.className).toBe(cls);
      expect(box.tagName).toBe(cls === "wide" ? "TEXTAREA" : "INPUT");
    }
    expect(r.querySelector('[data-action="describe-token"]')!.textContent).toBe("Describe");
  }
});

test("a table with no tokens draws no Describe tokens group", () => {
  const h = harness(newState());
  const titles = Array.from(h.node.querySelectorAll("h3")).map((t) => t.textContent);
  expect(titles).not.toContain("Describe tokens");
  expect(h.node.querySelector(".describe-row")).toBeNull();
});

// VTT-464
test("Describe sends describe_token for its row's token with the four fields as typed, empty ones included", () => {
  const h = harness(tableWithTokens("t1", "t2"));
  fill(h, { "describe-name:t1": "a hunched goblin ", "describe-gm-name:t1": "Goblin Cutter", "describe-gm-notes:t1": "wakes easily" });
  const row = h.node.querySelector('.describe-row[data-token="t1"]')!;
  (row.querySelector('[data-action="describe-token"]') as HTMLButtonElement).click();

  expect(h.sent).toHaveLength(1);
  const c = h.sent[0]!.command;
  expect(c.case).toBe("describeToken");
  if (c.case !== "describeToken") return;
  expect({ ...c.value }).toMatchObject({
    tokenId: "t1", name: "a hunched goblin ", notes: "", gmName: "Goblin Cutter", gmNotes: "wakes easily",
  });
});

function type(h: ReturnType<typeof harness>, values: Record<string, string>) {
  fill(h, values);
  for (const f of Object.keys(values)) h.field(f).dispatchEvent(new Event("input"));
}

test("a typed describe field survives a repaint, and an accepted Describe clears only its own row", async () => {
  const h = harness(tableWithTokens("t1", "t2"));
  type(h, { "describe-notes:t1": "asleep", "describe-gm-notes:t2": "the leader" });

  const again = harness(tableWithTokens("t1", "t2"));
  expect(again.field("describe-notes:t1").value).toBe("asleep");
  expect(again.field("describe-gm-notes:t2").value).toBe("the leader");

  (again.node.querySelector('.describe-row[data-token="t1"] [data-action="describe-token"]') as HTMLButtonElement).click();
  await again.settle();
  const after = harness(tableWithTokens("t1", "t2"));
  expect(after.field("describe-notes:t1").value).toBe("");
  expect(after.field("describe-gm-notes:t2").value).toBe("the leader");
  type(after, { "describe-gm-notes:t2": "" });
});

test("a describe field with no draft shows the token's present text", async () => {
  const st = newState();
  st.Tokens["t9"] = {
    ID: "t9", SceneID: "s1", ActorID: "a1", X: 0, Y: 0,
    Name: "a hunched goblin", Notes: "asleep", GMName: "Goblin Cutter", GMNotes: "wakes easily",
  };
  const h = harness(st);
  expect(h.field("describe-name:t9").value).toBe("a hunched goblin");
  expect(h.field("describe-notes:t9").value).toBe("asleep");
  expect(h.field("describe-gm-name:t9").value).toBe("Goblin Cutter");
  expect(h.field("describe-gm-notes:t9").value).toBe("wakes easily");

  type(h, { "describe-name:t9": "an angry goblin" });
  expect(harness(st).field("describe-name:t9").value).toBe("an angry goblin");
  type(harness(st), { "describe-name:t9": "" });
  expect(harness(st).field("describe-name:t9").value).toBe("");
  (harness(st).node.querySelector('.describe-row[data-token="t9"] [data-action="describe-token"]') as HTMLButtonElement).click();
  await settle();
  expect(harness(st).field("describe-name:t9").value).toBe("a hunched goblin");
});

// --- what the server's answer does to the fields that fed it ----------------
//
// Each form the console clears is listed once: what is typed into it, and
// what each field it clears reads when nothing has been typed in it. A field
// is named by a selector, because the selects carry no data-field.

/** Two tokens with text of their own and two actors, so every form draws. */
function tableForEveryForm(): State {
  const st = newState();
  for (const id of ["tok-r1", "tok-r2"]) {
    st.Tokens[id] = {
      ID: id, SceneID: "s1", ActorID: "a1", X: 0, Y: 0,
      Name: `${id} name`, Notes: `${id} notes`, GMName: `${id} gm`, GMNotes: `${id} gm notes`,
    };
  }
  for (const id of ["act-r1", "act-r2"]) {
    st.Actors[id] = {
      actorId: id, name: id, moduleId: "", attributes: {}, resources: {},
      controllerId: "", controllerIds: [], kind: ActorKind.UNSPECIFIED,
    };
  }
  return st;
}

function everyForm(answer: CommandResult | "held" | Error = ok()) {
  return harness(tableForEveryForm(), {
    participants: [{ participantId: "p-bo", displayName: "Bo" }],
    answer,
  });
}

type Harness = ReturnType<typeof harness>;

interface Form {
  name: string;
  /** What is typed into each field the form reads, by selector. */
  typed: Record<string, string>;
  /** Each field an accepted answer clears, as drawn with nothing typed in it. */
  cleared: Record<string, string>;
  press: (h: Harness) => void;
}

const box = (field: string) => `[data-field="${field}"]`;
const grantRow = (actor: string) => `.control-actor[data-actor="${actor}"]`;
const click = (selector: string) => (h: Harness) =>
  (h.node.querySelector(selector) as HTMLButtonElement).click();

function describeForm(token: string): Form {
  const typed: Record<string, string> = {};
  const cleared: Record<string, string> = {};
  for (const [part, present] of [
    ["name", "name"], ["notes", "notes"], ["gm-name", "gm"], ["gm-notes", "gm notes"],
  ] as const) {
    typed[box(`describe-${part}:${token}`)] = `typed ${part}`;
    cleared[box(`describe-${part}:${token}`)] = `${token} ${present}`;
  }
  return {
    name: `Describe ${token}`, typed, cleared,
    press: click(`.describe-row[data-token="${token}"] [data-action="describe-token"]`),
  };
}

function grantForm(actor: string): Form {
  const target = `${grantRow(actor)} .grant-target`;
  const kind = `${grantRow(actor)} .grant-kind`;
  return {
    name: `Grant ${actor}`,
    typed: { [target]: "p-bo", [kind]: "ACTOR_KIND_NON_PARTY" },
    cleared: { [target]: "", [kind]: "" },
    press: click(`${grantRow(actor)} .grant`),
  };
}

const PLACE = ["token-id", "token-scene", "token-actor", "token-x", "token-y"].map(box);

const FORMS: Form[] = [
  {
    name: "Start session",
    typed: { [box("session-name")]: "Night One" },
    cleared: { [box("session-name")]: "" },
    press: click('[data-action="start-session"]'),
  },
  {
    name: "Add actor",
    typed: {
      [box("actor-id")]: "act-new", [box("actor-name")]: "Lera",
      ".actor-kind": "ACTOR_KIND_PARTY_MEMBER",
    },
    cleared: { [box("actor-id")]: "", [box("actor-name")]: "", ".actor-kind": "" },
    press: click('[data-action="add-actor"]'),
  },
  {
    name: "Remove actor",
    typed: { [box("actor-id")]: "act-gone" },
    cleared: { [box("actor-id")]: "" },
    press: click('[data-action="remove-actor"]'),
  },
  {
    name: "Add from JSON",
    typed: { [box("actor-json")]: '{"actorId":"a9","name":"Nine","kind":"ACTOR_KIND_NON_PARTY"}' },
    cleared: { [box("actor-json")]: "" },
    press: (h) => h.button("Add from JSON").click(),
  },
  {
    name: "Place token",
    typed: Object.fromEntries(PLACE.map((sel, i) => [sel, ["tok-new", "s1", "a1", "2", "3"][i]!])),
    cleared: Object.fromEntries(PLACE.map((sel) => [sel, ""])),
    press: click('[data-action="place-token"]'),
  },
  {
    name: "Remove token",
    typed: { [box("token-id")]: "tok-gone" },
    cleared: { [box("token-id")]: "" },
    press: click('[data-action="remove-token"]'),
  },
  describeForm("tok-r1"),
  {
    name: "Save note",
    typed: {
      [box("note-key")]: "ravine", [box("note-title")]: "Ravine",
      [box("note-text")]: "The trail is watched.", ".note-visibility": "NOTE_VISIBILITY_PUBLIC",
    },
    cleared: { ".note-visibility": "" },
    press: click('[data-action="upsert-note"]'),
  },
  grantForm("act-r1"),
];

/** Type into a box through `input`, or choose in a select through `change`. */
function enter(h: Harness, values: Record<string, string>): void {
  for (const [selector, v] of Object.entries(values)) {
    const control = h.node.querySelector(selector) as HTMLInputElement | HTMLSelectElement;
    control.value = v;
    control.dispatchEvent(new Event(control.tagName === "SELECT" ? "change" : "input"));
  }
}

function read(h: Harness, selectors: string[]): Record<string, string> {
  return Object.fromEntries(
    selectors.map((sel) => [sel, (h.node.querySelector(sel) as HTMLInputElement).value]),
  );
}

/** Leave no typed text for a later test: every form's fields typed empty. */
function forgetEveryForm(): void {
  const all = [...FORMS, describeForm("tok-r2"), grantForm("act-r2")];
  enter(everyForm(), Object.fromEntries(all.flatMap((f) => Object.keys(f.typed)).map((s) => [s, ""])));
}

// VTT-517
test("a refused command leaves every field of its form as typed", async () => {
  try {
    for (const form of FORMS) {
      const h = everyForm(refused("not authorized"));
      enter(h, form.typed);
      form.press(h);
      await h.settle();

      expect([form.name, h.sent.length, h.repaints()]).toEqual([form.name, 1, 0]);
      expect([form.name, read(everyForm(), Object.keys(form.typed))])
        .toEqual([form.name, form.typed]);
    }
  } finally {
    forgetEveryForm();
  }
});

// VTT-517 VTT-519
test("a note's key, title and text stay as typed, refused or accepted", async () => {
  const note = FORMS.find((f) => f.name === "Save note")!;
  const kept = ["note-key", "note-title", "note-text"].map(box);
  try {
    for (const answer of [refused("not authorized"), ok()]) {
      const h = everyForm(answer);
      enter(h, note.typed);
      note.press(h);
      await h.settle();

      expect(h.sent).toHaveLength(1);
      expect(read(everyForm(), kept)).toEqual({
        [box("note-key")]: "ravine",
        [box("note-title")]: "Ravine",
        [box("note-text")]: "The trail is watched.",
      });
    }
  } finally {
    forgetEveryForm();
  }
});

// VTT-519
test("an accepted Start session empties the name, and asks for one repaint after the answer", async () => {
  const h = harness();
  try {
    h.field("session-name").value = "Night One";
    h.field("session-name").dispatchEvent(new Event("input"));
    h.action("start-session").click();
    expect(h.repaints()).toBe(0);
    await h.settle();

    expect(h.sent).toHaveLength(1);
    expect(h.repaints()).toBe(1);
    expect(harness().field("session-name").value).toBe("");
  } finally {
    type(harness(), { "session-name": "" });
  }
});

// VTT-519
test("an accepted command clears the fields of its own form and of no other", async () => {
  try {
    for (const form of FORMS) {
      const typed: Record<string, string> = {};
      for (const other of FORMS) if (other !== form) Object.assign(typed, other.typed);
      Object.assign(typed, form.typed);
      const h = everyForm(ok());
      enter(h, typed);
      form.press(h);
      await h.settle();

      expect([form.name, h.sent.length]).toEqual([form.name, 1]);
      expect([form.name, read(everyForm(), Object.keys(typed))])
        .toEqual([form.name, { ...typed, ...form.cleared }]);
    }
  } finally {
    forgetEveryForm();
  }
});

// VTT-523
test("a control whose command is unanswered sends nothing when pressed again, each row its own", async () => {
  const controls = [...FORMS, describeForm("tok-r2"), grantForm("act-r2")];
  const h = everyForm("held");
  let repainted: Harness | undefined;
  try {
    for (const form of controls) enter(h, form.typed);
    for (const [i, form] of controls.entries()) {
      form.press(h);
      expect([form.name, h.sent.length]).toEqual([form.name, i + 1]);
    }
    repainted = everyForm("held");
    for (const form of controls) {
      form.press(h);
      form.press(repainted);
    }
    expect([h.sent.length, repainted.sent.length]).toEqual([controls.length, 0]);

    h.release(refused("not authorized"));
    await h.settle();
    for (const form of controls) form.press(h);
    expect(h.sent).toHaveLength(2 * controls.length);
  } finally {
    h.release(refused("not authorized"));
    repainted?.release(refused("not authorized"));
    await h.settle();
    forgetEveryForm();
  }
});

// VTT-524
test("a press that sends changes nothing the next render draws until its answer", async () => {
  const h = everyForm("held");
  try {
    for (const form of FORMS) {
      enter(h, form.typed);
      const drawn = () => {
        const c = everyForm();
        return [form.name, c.node.outerHTML, read(c, Object.keys(form.typed))];
      };
      const before = drawn();
      form.press(h);
      expect(drawn()).toEqual(before);
    }
    expect([h.sent.length, h.repaints()]).toEqual([FORMS.length, 0]);
  } finally {
    h.release(refused("not authorized"));
    await h.settle();
    forgetEveryForm();
  }
});

// VTT-523
test("a press the console's own guard refuses holds nothing back", async () => {
  try {
    for (const form of FORMS.filter((f) => !f.name.startsWith("Describe"))) {
      forgetEveryForm();
      const h = everyForm(refused("not authorized"));
      form.press(h);
      expect([form.name, h.sent.length, h.notices.length]).toEqual([form.name, 0, 1]);
      enter(h, form.typed);
      form.press(h);
      expect([form.name, h.sent.length]).toEqual([form.name, 1]);
      await h.settle();
    }
  } finally {
    forgetEveryForm();
  }
});

// VTT-523 VTT-525
test("a send that rejects lets its control go, clears nothing, and reports the error", async () => {
  const thrown = new Error("paint threw");
  const h = harness(newState(), { answer: thrown });
  const reported = spyOn(console, "error").mockImplementation(() => {});
  try {
    type(h, { "session-name": "Night One" });
    h.action("start-session").click();
    await h.settle();
    h.action("start-session").click();
    await h.settle();

    expect(h.sent).toHaveLength(2);
    expect(h.repaints()).toBe(0);
    expect(harness().field("session-name").value).toBe("Night One");
    expect(reported.mock.calls).toEqual([[thrown], [thrown]]);
  } finally {
    reported.mockRestore();
    type(harness(), { "session-name": "" });
  }
});
