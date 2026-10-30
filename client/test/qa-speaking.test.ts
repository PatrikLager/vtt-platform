// "D7" and "D9" below are decisions of
// docs/superpowers/plans/2026-10-26-a-player-speaks-as-their-character.md;
// "ticket done-N" is item N of its ticket's "Done looks like", and "ticket
// rules" its "Rules this puts on the system".
import "./support/dom";
import { afterEach, describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import {
  ActorKind, EnvelopeSchema, NarrationAddedSchema,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import type {
  Envelope,
} from "../../contract/gen/ts/vtt/v1/events_pb";
import type {
  ClientCommand,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { newState } from "../src/state";
import type { State } from "../src/state";
import type { Me } from "../src/metadata";
import { renderPlayerPanel } from "../src/view/player";
import type { PlayerUIState } from "../src/view/player";
import { renderSpectator } from "../src/view/spectator";
import { ok, settle } from "./support/answers";
import { forgetTheSay, sayConsole } from "./support/say";

function world(): State {
  const st = newState();
  const base = { moduleId: "m", attributes: {}, resources: {} };
  st.Actors["a1"] = {
    ...base, actorId: "a1", name: "Bran", controllerId: "p1",
    controllerIds: ["p1"], kind: ActorKind.PARTY_MEMBER,
  };
  st.Actors["a2"] = {
    ...base, actorId: "a2", name: "Cora", controllerId: "p1",
    controllerIds: ["p1"], kind: ActorKind.PARTY_MEMBER,
  };
  return st;
}
afterEach(() => {
  document.body.replaceChildren();
  forgetTheSay();
});

const player: Me = { participantId: "p1", name: "Ada", role: "player" };
const dm: Me = { participantId: "d1", name: "Dee", role: "dm" };
const agent: Me = { participantId: "g1", name: "Bot", role: "agent" };

// A player's panel, or a DM's or an agent's console.
function rig(me: Me, sel = "a2") {
  const st = world();
  const ui: PlayerUIState = {
    selectedActorId: sel, selectedAbilityId: "", doorsArmed: false,
  };
  const sent: ClientCommand[] = [];
  const root = document.createElement("div");
  document.body.appendChild(root);
  const answer = async (c: ClientCommand) => {
    sent.push(c);
    return ok();
  };
  const paint = () => {
    renderSpectator(root, st, [], "connected", me.role === "player"
      ? { panel: renderPlayerPanel(st, me, [], ui, answer, paint) }
      : { console: sayConsole(st, answer, paint) });
  };
  paint();
  const q = (f: string) =>
    root.querySelector(`[data-field="${f}"]`) as HTMLInputElement | null;
  const type = (f: string, v: string) => {
    const el = q(f)!;
    el.value = v;
    el.dispatchEvent(new Event("input"));
  };
  const send = () => {
    (root.querySelector('[data-action="add-narration"]') as HTMLElement)
      .click();
  };
  return { st, ui, sent, root, paint, q, type, send };
}
function narr(c: ClientCommand) {
  expect(c.command.case).toBe("addNarration");
  return c.command.value as {
    text: string; as: string; actorId: string; outOfCharacter: boolean;
  };
}

describe("say form", () => {
  test("qaSpk: a player's panel draws no speaker box", () => {
    const r = rig(player);
    // VTT-500 "draws no box for a speaker"
    expect(r.q("say-as") === null).toBe(true);
    expect(r.q("say-text") === null).toBe(false);
  });

  test("qaSpk: a player's Send speaks as the acted-as member", () => {
    const r = rig(player, "a2");
    r.type("say-text", "I draw my sword");
    r.send();
    expect(r.sent.length).toBe(1);
    const n = narr(r.sent[0]!);
    // VTT-500 "speaks as the party member the panel acts as"
    expect(n.actorId).toBe("a2");
    expect(n.text).toBe("I draw my sword");
    // D7 "in character" for anything not /ooc
    expect(n.outOfCharacter).toBe(false);
  });

  test("qaSpk: the acted-as member follows the selection", () => {
    const r = rig(player, "a1");
    r.type("say-text", "hello");
    r.send();
    // VTT-500 the member the panel acts as
    expect(narr(r.sent[0]!).actorId).toBe("a1");
  });

  test("qaSpk: Enter in the box sends the same line", () => {
    const r = rig(player, "a2");
    r.type("say-text", "by Enter");
    r.q("say-text")!.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
    );
    // D7 "A player's Send, or Enter"
    expect(r.sent.length).toBe(1);
    const n = narr(r.sent[0]!);
    expect(n.actorId).toBe("a2");
    expect(n.text).toBe("by Enter");
  });

  test("qaSpk: /ooc goes out of character, rest as text", () => {
    const r = rig(player, "a2");
    r.type("say-text", "/ooc where is the pizza");
    r.send();
    const n = narr(r.sent[0]!);
    // VTT-501 "out of character, with the rest of the line"
    expect(n.outOfCharacter).toBe(true);
    expect(n.text).toBe("where is the pizza");
    // ticket done-2 "speaks as no character"
    expect(n.actorId).toBe("");
  });

  test("qaSpk: /ooc text is trimmed", () => {
    const r = rig(player);
    r.type("say-text", "/ooc   brb  ");
    r.send();
    // D7 "rest of the line, trimmed"
    expect(narr(r.sent[0]!).text).toBe("brb");
  });

  test("qaSpk: /oocx is not /ooc", () => {
    const r = rig(player, "a2");
    r.type("say-text", "/oocx hi");
    r.send();
    const n = narr(r.sent[0]!);
    // D7 "First word exactly /ooc"
    expect(n.outOfCharacter).toBe(false);
    expect(n.text).toBe("/oocx hi");
    expect(n.actorId).toBe("a2");
  });

  test("qaSpk: /OOC upper case is not /ooc", () => {
    const r = rig(player, "a2");
    r.type("say-text", "/OOC hi");
    r.send();
    // D7 "exact lowercase word" (a preference within the ruling)
    expect(narr(r.sent[0]!).outOfCharacter).toBe(false);
  });

  test("qaSpk: /ooc is read only as the first word", () => {
    const r = rig(player, "a2");
    r.type("say-text", "say /ooc now");
    r.send();
    const n = narr(r.sent[0]!);
    // VTT-501 "whose first word is /ooc"
    expect(n.outOfCharacter).toBe(false);
    expect(n.text).toBe("say /ooc now");
  });

  test("qaSpk: any other leading slash stays text", () => {
    const r = rig(player, "a2");
    r.type("say-text", "/me waves");
    r.send();
    const n = narr(r.sent[0]!);
    // D7 "Any other leading / stays text"
    expect(n.text).toBe("/me waves");
    expect(n.outOfCharacter).toBe(false);
    expect(n.actorId).toBe("a2");
  });

  test("qaSpk: /ooc alone sends nothing, box stays", () => {
    const r = rig(player);
    r.type("say-text", "/ooc");
    r.send();
    // D7 "sends nothing and leaves the box as typed"
    expect(r.sent.length).toBe(0);
    expect(r.q("say-text")!.value).toBe("/ooc");
  });

  test("qaSpk: /ooc then spaces sends nothing", () => {
    const r = rig(player);
    r.type("say-text", "/ooc   ");
    r.send();
    // D7 "/ooc with nothing after it sends nothing"
    expect(r.sent.length).toBe(0);
  });

  test("qaSpk: an empty line sends nothing", () => {
    const r = rig(player);
    r.send();
    // D7 "as an empty line does today"
    expect(r.sent.length).toBe(0);
  });

  test("qaSpk: Send empties the box", async () => {
    const r = rig(player);
    r.type("say-text", "hello");
    r.send();
    await settle();
    // VTT-520 "A line the server accepts leaves ... say-text empty"
    expect(r.q("say-text")!.value).toBe("");
  });

  test("qaSpk: an /ooc Send empties the box", async () => {
    const r = rig(player);
    r.type("say-text", "/ooc hi");
    r.send();
    await settle();
    // VTT-520 same, for the out-of-character send
    expect(r.q("say-text")!.value).toBe("");
  });

  test("qaSpk: a player's typing survives a repaint", () => {
    const r = rig(player);
    r.type("say-text", "half a sent");
    r.paint();
    // VTT-522 "shows what was typed in it"
    expect(r.q("say-text")!.value).toBe("half a sent");
    r.paint();
    expect(r.q("say-text")!.value).toBe("half a sent");
  });

  test("qaSpk: the focused say-text keeps focus and caret", () => {
    const r = rig(player);
    r.type("say-text", "abcdef");
    const el = r.q("say-text")!;
    el.focus();
    el.setSelectionRange(2, 4, "forward");
    r.paint();
    const now = r.q("say-text")!;
    // VTT-491 focus goes to the new tree's box
    expect(now === el).toBe(false);
    expect(document.activeElement === now).toBe(true);
    // VTT-491 "same selection start, end and direction"
    expect(now.selectionStart).toBe(2);
    expect(now.selectionEnd).toBe(4);
    expect(now.selectionDirection).toBe("forward");
  });
});

describe("DM and agent", () => {
  for (const me of [dm, agent]) {
    test(`qaSpk: a ${me.role} console draws the as box`, () => {
      const r = rig(me);
      // VTT-527 "draws a speaker box say-as, a line box say-text"
      expect(r.q("say-as") === null).toBe(false);
      expect(r.q("say-text") === null).toBe(false);
    });

    test(`qaSpk: a ${me.role} Send carries the box as as`, () => {
      const r = rig(me);
      r.type("say-as", "Goblin chief");
      r.type("say-text", "Grr");
      r.send();
      const n = narr(r.sent[0]!);
      // VTT-529 "with what say-as holds, trimmed, as as"
      expect(n.as).toBe("Goblin chief");
      expect(n.text).toBe("Grr");
      expect(n.outOfCharacter).toBe(false);
    });

    test(`qaSpk: a ${me.role} with an empty box sends no as`, () => {
      const r = rig(me);
      r.type("say-text", "The door creaks");
      r.send();
      const n = narr(r.sent[0]!);
      // VTT-529 "and no as when it is blank"
      expect(n.as).toBe("");
      expect(n.text).toBe("The door creaks");
    });

    test(`qaSpk: a ${me.role} reads /ooc too`, () => {
      const r = rig(me);
      r.type("say-text", "/ooc break time");
      r.send();
      const n = narr(r.sent[0]!);
      // VTT-530 "goes out of character, with the rest of the line"
      expect(n.outOfCharacter).toBe(true);
      expect(n.text).toBe("break time");
    });

    test(`qaSpk: a ${me.role} as box survives a repaint`, () => {
      const r = rig(me);
      r.type("say-as", "Goblin");
      r.type("say-text", "Grr");
      r.paint();
      // VTT-522 "say-as shows what it showed before it"
      expect(r.q("say-as")!.value).toBe("Goblin");
      // VTT-534 "say-text shows what was typed in it"
      expect(r.q("say-text")!.value).toBe("Grr");
    });

    test(`qaSpk: a ${me.role} focused as box keeps focus`, () => {
      const r = rig(me);
      r.type("say-as", "Goblin");
      const el = r.q("say-as")!;
      el.focus();
      el.setSelectionRange(1, 3, "backward");
      r.paint();
      const now = r.q("say-as")!;
      // VTT-491 focus and selection with that data-field
      expect(document.activeElement === now).toBe(true);
      expect(now.selectionStart).toBe(1);
      expect(now.selectionEnd).toBe(3);
      expect(now.selectionDirection).toBe("backward");
    });

    test(`qaSpk: a ${me.role} Send keeps the as box and empties the text box`, async () => {
      const r = rig(me);
      r.type("say-as", "Goblin");
      r.type("say-text", "Grr");
      r.send();
      await settle();
      // VTT-521 "Send leaves ... say-as holding what it held"
      expect(r.q("say-as")!.value).toBe("Goblin");
      // VTT-533 "leaves each DM console field that fed it as it is drawn"
      expect(r.q("say-text")!.value).toBe("");
    });
  }
});

function line(
  n: number, text: string, as: string, ooc: boolean,
): Envelope {
  return create(EnvelopeSchema, {
    eventId: `e${n}`, sequence: BigInt(n),
    payload: {
      case: "narrationAdded",
      value: create(NarrationAddedSchema, {
        text, as, outOfCharacter: ooc,
      }),
    },
  });
}
function draw(log: Envelope[], ticker: boolean) {
  const root = document.createElement("div");
  document.body.appendChild(root);
  renderSpectator(root, world(), log, "connected", { ticker });
  return root;
}
const feedLines = (root: HTMLElement) =>
  Array.from(root.querySelectorAll(".feed p")) as HTMLElement[];

describe("drawing a narration", () => {
  test("qaSpk: the Story draws an ooc line in double parens", () => {
    const root = draw([line(1, "where is the pizza", "Ada", true)], false);
    const ps = feedLines(root);
    expect(ps.length).toBe(1);
    // VTT-502 "as, ': ', and its text between '(( ' and ' ))'"
    expect(ps[0]!.textContent).toBe("Ada: (( where is the pizza ))");
    // D9 "its own class, ooc"
    expect(ps[0]!.classList.contains("ooc")).toBe(true);
  });

  test("qaSpk: the Events panel draws an ooc line the same", () => {
    const root = draw([line(1, "brb", "Ada", true)], true);
    const rows = Array.from(root.querySelectorAll(".ticker .tick"));
    expect(rows.length).toBe(1);
    // VTT-502 "and the Events panel where a window draws one"
    expect(rows[0]!.textContent).toContain("Ada: (( brb ))");
  });

  test("qaSpk: an in-character line carries no parens", () => {
    const root = draw([line(1, "I draw my sword", "Cora", false)], true);
    const p = feedLines(root)[0]!;
    // ticket done-8 "<name>: I draw my sword"
    expect(p.textContent).toBe("Cora: I draw my sword");
    expect(p.classList.contains("ooc")).toBe(false);
    expect(p.textContent).not.toContain("((");
    const tick = root.querySelector(".ticker .tick")!;
    expect(tick.textContent).not.toContain("((");
    expect(tick.textContent).toContain("Cora: I draw my sword");
  });

  test("qaSpk: a DM's nameless line stays plain narration", () => {
    const root = draw([line(1, "The door creaks", "", false)], false);
    const p = feedLines(root)[0]!;
    // ticket rules "keeps the speaker they give it, or none"
    expect(p.textContent).toBe("The door creaks");
    expect(p.classList.contains("ooc")).toBe(false);
  });

  test("qaSpk: the Story keeps the order of mixed lines", () => {
    const root = draw([
      line(1, "I draw my sword", "Cora", false),
      line(2, "brb", "Ada", true),
    ], false);
    const ts = feedLines(root).map((p) => p.textContent);
    // ticket done-8 both readings in one Story, in order
    expect(ts).toEqual(["Cora: I draw my sword", "Ada: (( brb ))"]);
  });
});
