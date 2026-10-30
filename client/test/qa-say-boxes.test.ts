import "./support/dom";
import { afterEach, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  ActorSchema,
  EnvelopeSchema,
  NarrationAddedSchema,
  type Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { renderSpectator } from "../src/view/spectator";
import { newState } from "../src/state";
import type { Me } from "../src/metadata";
import { ok, settle } from "./support/answers";

// The DM's panel, which keeps both Say boxes; a player's draws one (VTT-500).
const ME: Me = { participantId: "p1", name: "Pat", role: "dm" };
const AS = '[data-field="say-as"]';
const TEXT = '[data-field="say-text"]';
const SEND = '[data-action="add-narration"]';

type Said = { text: string; as: string };

function party(...ids: string[]) {
  const st = newState();
  for (const id of ids) {
    st.Actors[id] = create(ActorSchema, {
      actorId: id, name: id.toUpperCase(), controllerId: "p1", controllerIds: ["p1"],
    });
  }
  return st;
}

// One table as `startSession`'s paint draws it: one ui across every paint,
// the panel rebuilt and handed to renderSpectator, rerender repainting.
function table(ids: string[] = ["a1"]) {
  const root = document.createElement("div");
  document.body.replaceChildren(root);
  const st = party(...ids);
  const log: Envelope[] = [];
  const ui: PlayerUIState = {
    selectedActorId: ids[0] ?? "", selectedAbilityId: "", doorsArmed: false,
  };
  const sent: ClientCommand[] = [];
  let withPanel = true;
  const switcher = {
    actors: ids.map((id) => ({ actorId: id, name: id.toUpperCase() })),
    onSwitch: () => {},
  };
  const paint = (): void => {
    const panel = withPanel
      ? renderPlayerPanel(st, ME, [], ui, async (c) => {
        sent.push(c);
        return ok();
      }, paint,
        ME.role === "player" ? switcher : undefined)
      : undefined;
    renderSpectator(root, st, log, "open", { panel });
  };
  paint();
  const box = (sel: string): HTMLInputElement => {
    const el = root.querySelector<HTMLInputElement>(sel);
    if (!el) throw new Error(`no ${sel} in the window`);
    return el;
  };
  return {
    root, ui, sent, paint, box,
    dmNarrates(text: string): void {
      log.push(create(EnvelopeSchema, {
        eventId: `e${log.length + 1}`,
        sequence: BigInt(log.length + 1),
        payload: { case: "narrationAdded", value: create(NarrationAddedSchema, { text }) },
      }));
      paint();
    },
    hidePanel(hide: boolean): void { withPanel = !hide; },
    said(): Said[] {
      return sent.flatMap((c) => c.command.case === "addNarration"
        ? [{ text: c.command.value.text, as: c.command.value.as }] : []);
    },
  };
}

function typeInto(el: HTMLInputElement, value: string): void {
  el.value = value;
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

afterEach(() => { document.body.replaceChildren(); });

// VTT-491
test("qaSay: both Say boxes are boxes, each with its own data-field", () => {
  const t = table();
  const as = t.box(AS);
  const text = t.box(TEXT);
  for (const el of [as, text]) {
    expect(["INPUT", "TEXTAREA"]).toContain(el.tagName);
    expect(el.getAttribute("data-field") ?? "").not.toBe("");
  }
  expect(as.getAttribute("data-field")).not.toBe(text.getAttribute("data-field"));
  expect(t.root.querySelectorAll(AS).length).toBe(1);
  expect(t.root.querySelectorAll(TEXT).length).toBe(1);
});

// VTT-522
test("qaSay: a first paint shows both boxes empty", () => {
  const t = table();
  expect(t.box(AS).value).toBe("");
  expect(t.box(TEXT).value).toBe("");
});

// VTT-522
test("qaSay: the DM's narration repaints and both boxes keep what was typed", () => {
  const t = table();
  const oldAs = t.box(AS);
  const oldText = t.box(TEXT);
  typeInto(oldAs, "  Brother Tuck ");
  typeInto(oldText, "  I draw my ");
  t.dmNarrates("The door creaks open.");
  expect(t.root.textContent ?? "").toContain("The door creaks open.");
  expect(t.box(AS) === oldAs).toBe(false);
  expect(t.box(TEXT) === oldText).toBe(false);
  expect(t.box(AS).value).toBe("  Brother Tuck ");
  expect(t.box(TEXT).value).toBe("  I draw my ");
});

// VTT-491
test("qaSay: the focused text box keeps the focus and the caret mid-text", () => {
  const t = table();
  typeInto(t.box(AS), "Tuck");
  const old = t.box(TEXT);
  typeInto(old, "I my sword");
  old.focus();
  old.setSelectionRange(2, 2);
  expect(document.activeElement === old).toBe(true);
  t.dmNarrates("A goblin shrieks.");
  const now = t.box(TEXT);
  expect(now === old).toBe(false);
  expect(document.activeElement === now).toBe(true);
  expect(now.selectionStart).toBe(2);
  expect(now.selectionEnd).toBe(2);
  expect(document.activeElement === t.box(AS)).toBe(false);
});

// VTT-491
test("qaSay: the focused 'as' box keeps the focus and a backward selection", () => {
  const t = table();
  const old = t.box(AS);
  typeInto(old, "Brother Tuck");
  typeInto(t.box(TEXT), "hello there");
  old.focus();
  old.setSelectionRange(3, 9, "backward");
  t.dmNarrates("Rain.");
  const now = t.box(AS);
  expect(now === old).toBe(false);
  expect(document.activeElement === now).toBe(true);
  expect(now.selectionStart).toBe(3);
  expect(now.selectionEnd).toBe(9);
  expect(now.selectionDirection).toBe("backward");
  expect(t.box(TEXT).value).toBe("hello there");
});

// VTT-522
test("qaSay: Send after a repaint sends the narration with both boxes", () => {
  const t = table();
  typeInto(t.box(AS), "Brother Tuck");
  typeInto(t.box(TEXT), "I draw my sword");
  t.dmNarrates("Wind.");
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  expect(t.said()).toEqual([{ text: "I draw my sword", as: "Brother Tuck" }]);
});

// VTT-522
test("qaSay: text typed into the new box after a repaint joins the kept text", () => {
  const t = table();
  const old = t.box(TEXT);
  typeInto(old, "I draw my");
  old.focus();
  t.dmNarrates("The DM speaks.");
  const now = t.box(TEXT);
  expect(document.activeElement === now).toBe(true);
  typeInto(now, `${now.value} sword`);
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  expect(t.said()).toEqual([{ text: "I draw my sword", as: "" }]);
});

// VTT-520 VTT-521 VTT-522
test("qaSay: after an accepted Send the text box is empty and the 'as' box kept at the paints that follow", async () => {
  const t = table();
  typeInto(t.box(AS), "Brother Tuck");
  typeInto(t.box(TEXT), "I draw my sword");
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  expect(t.said().length).toBe(1);
  await settle();
  t.dmNarrates("Silence.");
  expect(t.box(AS).value).toBe("Brother Tuck");
  expect(t.box(TEXT).value).toBe("");
  t.paint();
  expect(t.box(AS).value).toBe("Brother Tuck");
  expect(t.box(TEXT).value).toBe("");
});

// VTT-520 VTT-521 VTT-522
test("qaSay: a second Send sends the text typed since the first's answer, as the same speaker", async () => {
  const t = table();
  typeInto(t.box(AS), "Tuck");
  typeInto(t.box(TEXT), "first line");
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  await settle();
  expect(t.box(TEXT).value).toBe("");
  typeInto(t.box(TEXT), "second line");
  t.dmNarrates("Hm.");
  expect(t.box(AS).value).toBe("Tuck");
  expect(t.box(TEXT).value).toBe("second line");
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  expect(t.said()).toEqual([
    { text: "first line", as: "Tuck" },
    { text: "second line", as: "Tuck" },
  ]);
});

// VTT-522
test("qaSay: a blank Send sends nothing and leaves both boxes as typed", async () => {
  const t = table();
  typeInto(t.box(AS), "Brother Tuck");
  typeInto(t.box(TEXT), "   ");
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  expect(t.said()).toEqual([]);
  t.dmNarrates("Rain.");
  expect(t.box(AS).value).toBe("Brother Tuck");
  expect(t.box(TEXT).value).toBe("   ");
  typeInto(t.box(TEXT), "Amen");
  t.root.querySelector<HTMLButtonElement>(SEND)?.click();
  expect(t.said()).toEqual([{ text: "Amen", as: "Brother Tuck" }]);
  await settle();
});

// VTT-520 VTT-521 VTT-522
test("qaSay: an accepted Enter in the text box keeps the 'as' box and empties the text box", async () => {
  const t = table();
  typeInto(t.box(AS), "Tuck");
  const text = t.box(TEXT);
  typeInto(text, "I bow");
  text.focus();
  text.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  expect(t.said()).toEqual([{ text: "I bow", as: "Tuck" }]);
  await settle();
  t.dmNarrates("Applause.");
  expect(t.box(AS).value).toBe("Tuck");
  expect(t.box(TEXT).value).toBe("");
});

// VTT-491
test("qaSay: Enter's repaint leaves the focus in the new text box", async () => {
  const t = table();
  const text = t.box(TEXT);
  typeInto(text, "I bow");
  text.focus();
  text.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  await settle();
  expect(t.box(TEXT) === text).toBe(false);
  expect(document.activeElement === t.box(TEXT)).toBe(true);
  t.dmNarrates("Applause.");
  const now = t.box(TEXT);
  expect(now === text).toBe(false);
  expect(document.activeElement === now).toBe(true);
});

// VTT-522
test("qaSay: the 'as' box alone keeps its text and the text box stays empty", () => {
  const t = table();
  const as = t.box(AS);
  typeInto(as, "Brother Tuck");
  as.focus();
  t.dmNarrates("Rain.");
  expect(t.box(AS).value).toBe("Brother Tuck");
  expect(t.box(TEXT).value).toBe("");
  expect(document.activeElement === t.box(AS)).toBe(true);
});

// VTT-522
test("qaSay: a box its typist emptied stays empty after a repaint", () => {
  const t = table();
  typeInto(t.box(AS), "Tuck");
  typeInto(t.box(TEXT), "abc");
  typeInto(t.box(AS), "");
  typeInto(t.box(TEXT), "");
  t.dmNarrates("Rain.");
  expect(t.box(AS).value).toBe("");
  expect(t.box(TEXT).value).toBe("");
});

// VTT-492
test("qaSay: with neither box focused a repaint keeps the text, focuses nothing", () => {
  const t = table();
  typeInto(t.box(AS), "Tuck");
  typeInto(t.box(TEXT), "I wait");
  t.box(TEXT).focus();
  t.box(TEXT).blur();
  expect(document.activeElement === document.body).toBe(true);
  t.dmNarrates("Rain.");
  expect(document.activeElement === document.body).toBe(true);
  expect(t.box(AS).value).toBe("Tuck");
  expect(t.box(TEXT).value).toBe("I wait");
});

// VTT-492
test("qaSay: a focused Send button: a repaint focuses nothing in the window", () => {
  const t = table();
  typeInto(t.box(TEXT), "I wait");
  t.root.querySelector<HTMLButtonElement>(SEND)?.focus();
  t.dmNarrates("Rain.");
  expect(t.root.contains(document.activeElement)).toBe(false);
  expect(t.box(TEXT).value).toBe("I wait");
});

// VTT-522
test("qaSay: a paint without the panel in between loses nothing typed", () => {
  const t = table();
  typeInto(t.box(AS), "Tuck");
  typeInto(t.box(TEXT), "I wait");
  t.hidePanel(true);
  t.dmNarrates("Rain.");
  expect(t.root.querySelector(TEXT) === null).toBe(true);
  t.hidePanel(false);
  t.paint();
  expect(t.box(AS).value).toBe("Tuck");
  expect(t.box(TEXT).value).toBe("I wait");
});

// VTT-522
test("qaSay: a switch of the watched party member keeps the boxes' text", () => {
  const t = table(["a1", "a2"]);
  typeInto(t.box(AS), "Tuck");
  typeInto(t.box(TEXT), "I wait");
  t.ui.selectedActorId = "a2";
  t.paint();
  expect(t.box(AS).value).toBe("Tuck");
  expect(t.box(TEXT).value).toBe("I wait");
});

// VTT-492
test("qaSay: a repaint that takes the focused box away focuses nothing", () => {
  const t = table();
  const text = t.box(TEXT);
  typeInto(text, "I wait");
  text.focus();
  t.hidePanel(true);
  expect(() => t.dmNarrates("Rain.")).not.toThrow();
  expect(document.activeElement === document.body).toBe(true);
  t.hidePanel(false);
  t.paint();
  expect(t.root.contains(document.activeElement)).toBe(false);
  expect(t.box(TEXT).value).toBe("I wait");
});
