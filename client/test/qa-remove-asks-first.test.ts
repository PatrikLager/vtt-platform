import "./support/dom";
import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { CommandResultSchema, type CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { newState, type Actor } from "../src/state";

const QUESTION = (who: string) => `Remove ${who}? Nothing brings it back, and its tokens leave the board.`;

function actor(id: string, name: string): Actor {
  return {
    actorId: id,
    name,
    moduleId: "",
    attributes: {},
    resources: {},
    controllerId: "",
    controllerIds: [],
    kind: 0,
    abilityIds: [],
    references: {},
  } as unknown as Actor;
}

interface Rig {
  root: HTMLElement;
  sent: any[];
  confirms: string[];
  notes: string[];
  answer: { yes: boolean };
  settle: () => void;
  d: DMDeps;
}

function mount(yes: boolean, hold = false): Rig {
  const st = newState();
  st.Actors["a-gob"] = actor("a-gob", "Goblin");
  st.Actors["a-nameless"] = actor("a-nameless", "");
  const rig = { sent: [] as any[], confirms: [] as string[], notes: [] as string[], answer: { yes } } as Rig;
  let resolvers: Array<(r: CommandResult) => void> = [];
  const d: DMDeps = {
    st,
    participants: [],
    adventures: [],
    maps: [],
    abilities: [],
    guideFor: async () => null,
    joinLink: null,
    roster: null,
    origin: "http://localhost",
    refreshSharing: () => {},
    send: (c) => {
      rig.sent.push(c);
      if (!hold) return Promise.resolve(create(CommandResultSchema, { ok: true }));
      return new Promise<CommandResult>((r) => resolvers.push(r));
    },
    rerender: () => {},
    notify: (m) => {
      rig.notes.push(m);
    },
    confirm: (m) => {
      rig.confirms.push(m);
      return rig.answer.yes;
    },
    doorsArmed: false,
    toggleDoors: () => {},
  };
  rig.d = d;
  rig.settle = () => {
    for (const r of resolvers) r(create(CommandResultSchema, { ok: true }));
    resolvers = [];
  };
  rig.root = renderDMConsole(d);
  document.body.appendChild(rig.root);
  return rig;
}

function actorField(root: HTMLElement): HTMLInputElement {
  return root.querySelector('input[data-field="actor-id"]') as HTMLInputElement;
}
function removeBtn(root: HTMLElement): HTMLButtonElement {
  const b = root.querySelector('[data-action="remove-actor"]');
  if (!b) throw new Error("no remove-actor button");
  return b as HTMLButtonElement;
}
async function press(rig: Rig, typed: string): Promise<void> {
  actorField(rig.root).value = typed;
  actorField(rig.root).dispatchEvent(new Event("input", { bubbles: true }));
  removeBtn(rig.root).click();
  await Promise.resolve();
  await Promise.resolve();
}
function shown(rig: Rig): string {
  return rig.notes.join("|");
}
function removed(c: any): string | undefined {
  return c?.command?.case === "removeActor" ? c.command.value.actorId : undefined;
}
function clear(rig: Rig) {
  actorField(rig.root).value = "";
  actorField(rig.root).dispatchEvent(new Event("input", { bubbles: true }));
}

describe("remove asks first", () => {
  test("named actor is asked by its name, padded field", async () => {
    const rig = mount(false);
    try {
      await press(rig, "  a-gob  "); // VTT-950
      expect(rig.confirms).toEqual([QUESTION("Goblin")]); // VTT-950
      expect(rig.sent).toEqual([]); // VTT-950
    } finally {
      clear(rig);
    }
  });

  test("nameless actor is asked by its id", async () => {
    const rig = mount(false);
    try {
      await press(rig, "a-nameless");
      expect(rig.confirms).toEqual([QUESTION("a-nameless")]); // VTT-950
      expect(rig.sent).toEqual([]); // VTT-950
    } finally {
      clear(rig);
    }
  });

  test("unknown id is asked by the id", async () => {
    const rig = mount(false);
    try {
      await press(rig, " nobody-9 ");
      expect(rig.confirms).toEqual([QUESTION("nobody-9")]); // VTT-950
      expect(rig.sent).toEqual([]); // VTT-950
    } finally {
      clear(rig);
    }
  });

  test("a no leaves the field as typed, also after a fresh render", async () => {
    const rig = mount(false);
    try {
      await press(rig, "  a-gob ");
      expect(actorField(rig.root).value).toBe("  a-gob "); // VTT-948
      const again = mount(false);
      expect(actorField(again.root).value).toBe("  a-gob "); // VTT-948
      const third = renderDMConsole(rig.d);
      expect(actorField(third).value).toBe("  a-gob "); // VTT-948
    } finally {
      clear(rig);
    }
  });

  test("a yes sends exactly one removeActor with the trimmed id", async () => {
    const rig = mount(true);
    try {
      await press(rig, "  a-gob  ");
      expect(rig.confirms.length).toBe(1); // VTT-950
      expect(rig.sent.length).toBe(1); // VTT-947
      expect(removed(rig.sent[0])).toBe("a-gob"); // VTT-947
    } finally {
      clear(rig);
    }
  });

  test("the answer decides: same input, flipped answer flips the outcome", async () => {
    const no = mount(false);
    const yes = mount(true);
    try {
      await press(no, "a-gob");
      await press(yes, "a-gob");
      expect(no.sent.length).toBe(0); // VTT-950
      expect(yes.sent.length).toBe(1); // VTT-947
    } finally {
      clear(no);
    }
  });

  test("empty field shows the notice, asks and sends nothing", async () => {
    const rig = mount(true);
    try {
      await press(rig, "");
      expect(shown(rig)).toContain("name the actor to remove"); // VTT-951
      expect(rig.confirms).toEqual([]); // VTT-951
      expect(rig.sent).toEqual([]); // VTT-951
    } finally {
      clear(rig);
    }
  });

  test("whitespace-only field shows the notice, asks and sends nothing", async () => {
    const rig = mount(true);
    try {
      await press(rig, "   \t ");
      expect(shown(rig)).toContain("name the actor to remove"); // VTT-951
      expect(rig.confirms).toEqual([]); // VTT-951
      expect(rig.sent).toEqual([]); // VTT-951
    } finally {
      clear(rig);
    }
  });

  test("a second press while the first remove_actor is unanswered does not ask", async () => {
    const rig = mount(true, true);
    try {
      await press(rig, "a-gob");
      expect(rig.sent.length).toBe(1); // VTT-947
      const asked = rig.confirms.length;
      await press(rig, "a-gob");
      expect(rig.confirms.length).toBe(asked);
      expect(rig.sent.length).toBe(1); // VTT-523
      rig.settle();
      await Promise.resolve();
      await Promise.resolve();
      await press(rig, "a-gob");
      expect(rig.confirms.length).toBe(asked + 1); // VTT-950
    } finally {
      rig.settle();
      clear(rig);
    }
  });
});
