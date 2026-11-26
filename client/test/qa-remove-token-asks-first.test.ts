import "./support/dom";
import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";
import { CommandResultSchema, type CommandResult } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { renderDMConsole, type DMDeps } from "../src/view/dm";
import { newState, type Actor, type Token } from "../src/state";

const QUESTION = (who: string) => `Remove ${who} from the board? Its place, names and notes go with it.`;

function actor(id: string, name: string): Actor {
  return { actorId: id, name, moduleId: "", attributes: {}, resources: {}, controllerId: "", controllerIds: [], kind: 0, abilityIds: [], references: {} } as unknown as Actor;
}
function token(id: string, actorId: string, extra: Partial<Token> = {}): Token {
  return {
    ID: id, SceneID: "s1", ActorID: actorId, X: 0, Y: 0,
    Name: "", Notes: "", GMName: "", GMNotes: "", ...extra,
  };
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
  st.Tokens["t-gm"] = token("t-gm", "a-gob", { GMName: "Boss", Name: "Goblin A" });
  st.Tokens["t-act"] = token("t-act", "a-gob", { Name: "Visible" });
  st.Tokens["t-tok"] = token("t-tok", "a-nameless", { Name: "Scout" });
  st.Tokens["t-none"] = token("t-none", "a-nameless");
  st.Tokens["t-orphan"] = token("t-orphan", "ghost");
  st.Tokens["t-orphan-named"] = token("t-orphan-named", "ghost", { Name: "Lost" });
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

function field(root: HTMLElement): HTMLInputElement {
  return root.querySelector('input[data-field="token-id"]') as HTMLInputElement;
}
function btn(root: HTMLElement): HTMLButtonElement {
  const b = root.querySelector('[data-action="remove-token"]');
  if (!b) throw new Error("no remove-token button");
  return b as HTMLButtonElement;
}
async function press(rig: Rig, typed: string): Promise<void> {
  field(rig.root).value = typed;
  field(rig.root).dispatchEvent(new Event("input", { bubbles: true }));
  btn(rig.root).click();
  await Promise.resolve();
  await Promise.resolve();
}
function removed(c: any): string | undefined {
  return c?.command?.case === "removeToken" ? c.command.value.tokenId : undefined;
}
function clear(rig: Rig) {
  field(rig.root).value = "";
  field(rig.root).dispatchEvent(new Event("input", { bubbles: true }));
}

describe("remove token asks first", () => {
  const cases: Array<[string, string, string]> = [
    ["GM name wins over actor and token names", "  t-gm  ", "Boss"],
    ["no GM name: the actor's name wins over the token's", "t-act", "Goblin"],
    ["actor nameless: the token's name", "t-tok", "Scout"],
    ["no names at all: the id", "t-none", "t-none"],
    ["actor not held, no token name: the id", "t-orphan", "t-orphan"],
    ["actor not held, token named: the token's name", "t-orphan-named", "Lost"],
    ["unknown token id: the id", " nobody-9 ", "nobody-9"],
  ];
  for (const [label, typed, who] of cases) {
    test(`naming: ${label}`, async () => {
      const rig = mount(false);
      try {
        await press(rig, typed);
        expect(rig.confirms).toEqual([QUESTION(who)]); // VTT-952
        expect(rig.sent).toEqual([]); // VTT-952
      } finally {
        clear(rig);
      }
    });
  }

  test("a no leaves the field as typed, also after a fresh render", async () => {
    const rig = mount(false);
    try {
      await press(rig, "  t-gm ");
      expect(field(rig.root).value).toBe("  t-gm "); // VTT-954
      const third = renderDMConsole(rig.d);
      expect(field(third).value).toBe("  t-gm "); // VTT-954
      expect(rig.sent).toEqual([]); // VTT-952
    } finally {
      clear(rig);
    }
  });

  test("a yes sends exactly one removeToken with the trimmed id", async () => {
    const rig = mount(true);
    try {
      await press(rig, "  t-gm  ");
      expect(rig.confirms.length).toBe(1); // VTT-952
      expect(rig.sent.length).toBe(1); // VTT-953
      expect(removed(rig.sent[0])).toBe("t-gm"); // VTT-953
    } finally {
      clear(rig);
    }
  });

  test("the answer decides: same input, flipped answer flips the outcome", async () => {
    const no = mount(false);
    const yes = mount(true);
    try {
      await press(no, "t-gm");
      await press(yes, "t-gm");
      expect(no.sent.length).toBe(0); // VTT-952
      expect(yes.sent.length).toBe(1); // VTT-953
    } finally {
      clear(no);
      clear(yes);
    }
  });

  for (const [label, typed] of [["empty", ""], ["whitespace-only", "   \t "]] as const) {
    test(`${label} field shows the notice, asks and sends nothing`, async () => {
      const rig = mount(true);
      try {
        await press(rig, typed);
        expect(rig.notes.join("|")).toContain("name the token to remove"); // VTT-955
        expect(rig.confirms).toEqual([]); // VTT-955
        expect(rig.sent).toEqual([]); // VTT-955
      } finally {
        clear(rig);
      }
    });
  }

  test("a second press while unanswered sends and asks nothing; after the answer it asks again", async () => {
    const rig = mount(true, true);
    try {
      await press(rig, "t-gm");
      expect(rig.sent.length).toBe(1); // VTT-953
      const asked = rig.confirms.length;
      await press(rig, "t-gm");
      expect(rig.sent.length).toBe(1); // VTT-523
      expect(rig.confirms.length).toBe(asked);
      rig.settle();
      await Promise.resolve();
      await Promise.resolve();
      await press(rig, "t-gm");
      expect(rig.confirms.length).toBe(asked + 1); // VTT-952
    } finally {
      rig.settle();
      clear(rig);
    }
  });

  test("an empty press while unanswered shows no notice and sends nothing", async () => {
    const rig = mount(true, true);
    try {
      await press(rig, "t-gm");
      await press(rig, "");
      expect(rig.sent.length).toBe(1); // VTT-523
      expect(rig.notes.join("|")).not.toContain("name the token to remove");
    } finally {
      rig.settle();
      clear(rig);
    }
  });
});
