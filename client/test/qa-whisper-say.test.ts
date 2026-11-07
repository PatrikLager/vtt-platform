import "./support/dom";
import { afterEach, describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { toJson } from "@bufbuild/protobuf";
import {
  ClientCommandSchema, type ClientCommand, type CommandResult,
} from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import { readLine } from "../src/player";
import { addWhisper } from "../src/commands";
import { renderDMConsole, whisperTargets } from "../src/view/dm";
import { renderPlayerPanel } from "../src/view/player";
import { newState, type State } from "../src/state";
import { held, ok, refused, settle } from "./support/answers";
import { forgetTheSay } from "./support/say";

type Person = { id: string; name: string; role: "dm" | "agent" | "player" | "spectator" };

function qaActor(st: State, id: string, name: string, kind: ActorKind, controllers: string[]): void {
  st.Actors[id] = {
    actorId: id, name, moduleId: "", attributes: {}, resources: {},
    controllerId: controllers[0] ?? "", controllerIds: controllers, kind,
  };
}

const PM = ActorKind.PARTY_MEMBER;
const NPC = ActorKind.NON_PARTY;

const people: Person[] = [
  { id: "p-dm", name: "Dee", role: "dm" },
  { id: "p-lera", name: "Lera", role: "player" },
  { id: "p-sven", name: "Sven", role: "player" },
  { id: "p-ivo", name: "Ivo", role: "player" },
  { id: "p-tobbe", name: "Tobbe", role: "player" },
  { id: "p-twin", name: "Twins", role: "player" },
];

function table(): State {
  const st = newState();
  qaActor(st, "a1", "Tobbe", PM, ["p-sven"]);
  qaActor(st, "a3", "Two Words", PM, ["p-sven"]);
  qaActor(st, "pm-z", "Mira", PM, ["p-lera"]);
  qaActor(st, "pm-a", "Kel", PM, ["p-lera"]);
  qaActor(st, "c-wolf", "Wolf", NPC, ["p-lera"]);
  qaActor(st, "c-gob", "Goblin", NPC, []);
  qaActor(st, "c-ivo", "Pet", NPC, ["p-ivo"]);
  qaActor(st, "a6", "Other", PM, ["p-tobbe"]);
  qaActor(st, "t1", "Twin", PM, ["p-twin"]);
  qaActor(st, "t2", "twin", PM, ["p-twin"]);
  return st;
}

function peopleMap(): Map<string, { name: string }> {
  return new Map(people.map((p) => [p.id, { name: p.name }]));
}

type Sent = ClientCommand[];

function dmConsole(
  st: State, send: (c: ClientCommand) => Promise<CommandResult>, notes: string[], repaints: { n: number },
): HTMLElement {
  return renderDMConsole({
    st,
    participants: people.map((p) => ({ participantId: p.id, displayName: p.name })),
    adventures: [], maps: [], guideFor: async () => null, joinLink: null,
    roster: people.map((p) => ({ participantId: p.id, name: p.name, role: p.role })),
    origin: "", refreshSharing: () => {}, send,
    rerender: () => { repaints.n++; },
    notify: (m: string) => { notes.push(m); },
    confirm: () => false, doorsArmed: false, toggleDoors: () => {},
  });
}

function box(node: HTMLElement, field: string): HTMLInputElement {
  const el = node.querySelector<HTMLInputElement>(`[data-field="${field}"]`);
  if (!el) throw new Error(`no ${field}`);
  return el;
}

function type(node: HTMLElement, field: string, value: string): void {
  const el = box(node, field);
  el.value = value;
  el.dispatchEvent(new Event("input"));
}

async function press(node: HTMLElement, how: "send" | "enter"): Promise<void> {
  if (how === "send") {
    node.querySelector<HTMLButtonElement>('[data-action="add-narration"]')!.click();
  } else {
    box(node, "say-text").dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  }
  await settle();
}

function recorder(answer: () => CommandResult = ok): { sent: Sent; send: (c: ClientCommand) => Promise<CommandResult> } {
  const sent: Sent = [];
  return { sent, send: async (c) => { sent.push(c); return answer(); } };
}

function whisperOf(c: ClientCommand | undefined): { text: string; to: string[]; from: string } {
  if (c?.command.case !== "addWhisper") throw new Error(`sent ${c?.command.case}, want addWhisper`);
  return { text: c.command.value.text, to: [...c.command.value.toActorIds], from: c.command.value.fromActorId };
}

afterEach(() => forgetTheSay());

describe("readLine reads /w and /dm", () => {
  test("a /w line in any case whispers to the next word with the rest trimmed", () => {
    // VTT-668
    expect(readLine("/w Tobbe An arrow came")).toEqual({ kind: "whisper", name: "Tobbe", text: "An arrow came" });
    expect(readLine("/W Tobbe hi")).toEqual({ kind: "whisper", name: "Tobbe", text: "hi" });
    expect(readLine("/w   Tobbe    two  spaces   ")).toEqual({ kind: "whisper", name: "Tobbe", text: "two  spaces" });
  });

  test("a /w line whispers to the words between a pair of double quotes", () => {
    // VTT-668
    expect(readLine('/w "Two Words" hi there')).toEqual({ kind: "whisper", name: "Two Words", text: "hi there" });
    expect(readLine('/W "two words"   x  ')).toEqual({ kind: "whisper", name: "two words", text: "x" });
  });

  test("a /w line with no name or no text is still a whisper, never a say", () => {
    // VTT-668
    expect(readLine("/w Tobbe")).toEqual({ kind: "whisper", name: "Tobbe", text: "" });
    expect(readLine('/w "Two Words"')).toEqual({ kind: "whisper", name: "Two Words", text: "" });
    expect(readLine("/w").kind).toBe("whisper");
    expect(readLine("/w   ").kind).toBe("whisper");
  });

  test("a /dm line in any case whispers to the DM with the rest trimmed", () => {
    // VTT-668
    expect(readLine("/dm I pocket the ring")).toEqual({ kind: "toDM", text: "I pocket the ring" });
    expect(readLine("/DM   psst  ")).toEqual({ kind: "toDM", text: "psst" });
    expect(readLine("/Dm x")).toEqual({ kind: "toDM", text: "x" });
    expect(readLine("/dm")).toEqual({ kind: "toDM", text: "" });
    expect(readLine("/dm   ")).toEqual({ kind: "toDM", text: "" });
  });

  test("a first word that only begins with /w or /dm is said aloud, and /ooc matches exactly", () => {
    // VTT-668, VTT-599, VTT-501
    for (const raw of ["/wx Tobbe hi", "/whisperer Tobbe hi", "/dmx hi", "/wTobbe hi", "/dmhi"]) {
      expect(readLine(raw).kind).toBe("say");
    }
    expect(readLine("/ooc brb")).toEqual({ kind: "ooc", text: "brb" });
    expect(readLine("/OOC brb").kind).toBe("say");
  });
});

describe("addWhisper", () => {
  test("the DM's whisper builds the contract's add_whisper command", () => {
    const fixture = JSON.parse(readFileSync(join(import.meta.dir, "../../contract/testdata/add_whisper_command.json"), "utf8"));
    const got = toJson(ClientCommandSchema, addWhisper(fixture.addWhisper.text, ["act-tobbe"])) as Record<string, unknown>;
    delete got["requestId"];
    delete fixture.requestId;
    expect(got).toEqual(fixture);
  });

  test("a player's whisper names fromActorId and no toActorIds", () => {
    expect(whisperOf(addWhisper("hi", [], "a1"))).toEqual({ text: "hi", to: [], from: "a1" });
  });
});

describe("the add_whisper tool", () => {
  test("its description says whom a whisper reaches and how to answer one", () => {
    // VTT-591
    const tools = JSON.parse(readFileSync(join(import.meta.dir, "../../contract/gen/tools/tools.json"), "utf8")) as
      { name: string; description: string; inputSchema: { required: string[] } }[];
    const tool = tools.find((t) => t.name === "add_whisper");
    const d = tool?.description ?? "";
    expect(d).toContain("written into the world log and into the log of each party member it names and no other");
    expect(d).toContain("name every party member they control");
    expect(d).toMatch(/whisper to the DM with add_whisper, never add_narration/);
    expect(tool?.inputSchema.required).toEqual(expect.arrayContaining(["text", "toActorIds"]));
  });
});

describe("whisperTargets", () => {
  test("a party member's name, in any case, names that party member", () => {
    // VTT-593
    const st = table();
    expect(whisperTargets("Tobbe", st, peopleMap())).toEqual(["a1"]);
    expect(whisperTargets("tOBBE", st, peopleMap())).toEqual(["a1"]);
    expect(whisperTargets("two words", st, peopleMap())).toEqual(["a3"]);
  });

  test("a participant's name names their party members in actor-id order and no creature", () => {
    // VTT-593
    expect(whisperTargets("LERA", table(), peopleMap())).toEqual(["pm-a", "pm-z"]);
  });

  test("an unknown name, a shared name and a player with no party member give a reason", () => {
    // VTT-594
    for (const name of ["Nobody", "Goblin", "twin", "TWIN", "Ivo", "Dee", ""]) {
      const got = whisperTargets(name, table(), peopleMap());
      expect(typeof got).toBe("string");
      expect((got as string).length).toBeGreaterThan(0);
    }
  });
});

describe("the DM console's /w", () => {
  test("Send whispers /w Tobbe to that party member, as the DM whatever say-as holds", async () => {
    // VTT-593, VTT-600
    const { sent, send } = recorder();
    const node = dmConsole(table(), send, [], { n: 0 });
    type(node, "say-as", "Grimble");
    type(node, "say-text", "/w Tobbe An arrow from the east.");
    await press(node, "send");
    expect(sent.length).toBe(1);
    expect(whisperOf(sent[0])).toEqual({ text: "An arrow from the east.", to: ["a1"], from: "" });
  });

  test("Enter whispers a quoted name, any case, and a participant's name to their party members", async () => {
    // VTT-593
    const { sent, send } = recorder();
    const node = dmConsole(table(), send, [], { n: 0 });
    type(node, "say-text", '/W "two WORDS"   hi there  ');
    await press(node, "enter");
    type(node, "say-text", "/w lera psst");
    await press(node, "enter");
    type(node, "say-text", "/w Tobbe hey");
    await press(node, "enter");
    expect(sent.map(whisperOf)).toEqual([
      { text: "hi there", to: ["a3"], from: "" },
      { text: "psst", to: ["pm-a", "pm-z"], from: "" },
      { text: "hey", to: ["a1"], from: "" },
    ]);
  });

  test("a refused /w or /dm line sends nothing, tells the DM and keeps say-text", async () => {
    // VTT-594
    const lines = ["/w Nobody hi", "/w Goblin hi", "/w twin hi", "/w Ivo hi", "/w", "/w Tobbe", "/w Tobbe   ",
      '/w "Two Words"', "/dm hello", "/DM hello"];
    for (const how of ["send", "enter"] as const) {
      for (const line of lines) {
        const { sent, send } = recorder();
        const notes: string[] = [];
        const node = dmConsole(table(), send, notes, { n: 0 });
        type(node, "say-text", line);
        await press(node, how);
        expect({ line, how, sent: sent.length, told: notes.length }).toEqual({ line, how, sent: 0, told: 1 });
        expect(notes[0]!.length).toBeGreaterThan(0);
        expect(box(node, "say-text").value).toBe(line);
        expect(box(dmConsole(table(), send, [], { n: 0 }), "say-text").value).toBe(line);
      }
    }
  });

  test("a line the console refuses takes no lock, and the next line is sent", async () => {
    // VTT-594, VTT-535
    const { sent, send } = recorder();
    const node = dmConsole(table(), send, [], { n: 0 });
    type(node, "say-text", "/w Nobody hi");
    await press(node, "send");
    type(node, "say-text", "/w Tobbe hi");
    await press(node, "send");
    expect(sent.map(whisperOf)).toEqual([{ text: "hi", to: ["a1"], from: "" }]);
  });

  test("an accepted whisper empties say-text and keeps say-as; a refused one keeps both", async () => {
    // VTT-520, VTT-533, VTT-517
    const repaints = { n: 0 };
    const node = dmConsole(table(), recorder().send, [], repaints);
    type(node, "say-as", "Grimble");
    type(node, "say-text", "/w Tobbe hi");
    await press(node, "send");
    const after = dmConsole(table(), recorder().send, [], { n: 0 });
    expect([repaints.n > 0, box(after, "say-text").value, box(after, "say-as").value]).toEqual([true, "", "Grimble"]);

    const no = dmConsole(table(), recorder(() => refused("nope")).send, [], { n: 0 });
    type(no, "say-text", "/w Tobbe again");
    await press(no, "enter");
    const kept = dmConsole(table(), recorder().send, [], { n: 0 });
    expect([box(kept, "say-text").value, box(kept, "say-as").value]).toEqual(["/w Tobbe again", "Grimble"]);
  });

  test("while a whisper is unanswered, Send and Enter send nothing more", async () => {
    // VTT-535
    const h = held();
    const sent: Sent = [];
    const node = dmConsole(table(), (c) => { sent.push(c); return h.send(c); }, [], { n: 0 });
    type(node, "say-text", "/w Tobbe one");
    await press(node, "send");
    await press(node, "send");
    await press(node, "enter");
    expect(sent.length).toBe(1);
    h.release();
    await settle();
  });

  test("a first word that is no whisper command and not exactly /ooc is said in character as say-as", async () => {
    // VTT-600
    const { sent, send } = recorder();
    const node = dmConsole(table(), send, [], { n: 0 });
    type(node, "say-as", "  Grimble ");
    const lines = ["/wx Tobbe hi", "/whisperer Tobbe hi", "/gmx hi", "/dmx hi", "/OOC hi", "hello"];
    for (const line of lines) {
      type(node, "say-text", line);
      await press(node, "send");
    }
    expect(sent.map((c) => [c.command.case, c.command.case === "addNarration" ? c.command.value.as : "?",
      c.command.case === "addNarration" ? c.command.value.outOfCharacter : "?"]))
      .toEqual(lines.map(() => ["addNarration", "Grimble", false]));
    type(node, "say-as", "   ");
    type(node, "say-text", "/W2 x");
    await press(node, "enter");
    const last = sent[sent.length - 1];
    expect(last?.command.case === "addNarration" ? [last.command.value.as, last.command.value.text] : null)
      .toEqual(["", "/W2 x"]);
  });
});

function panel(
  st: State, selected: string, send: (c: ClientCommand) => Promise<CommandResult>, notes: string[],
  ui = { selectedActorId: selected, selectedAbilityId: "", doorsArmed: false },
): { node: HTMLElement; ui: typeof ui } {
  const node = renderPlayerPanel(st, { participantId: "p-lera", name: "Lera", role: "player" }, [], ui, send,
    () => {}, undefined, (m: string) => { notes.push(m); });
  return { node, ui };
}

function leraTable(): State {
  const st = newState();
  qaActor(st, "a1", "Hero", PM, ["p-lera"]);
  qaActor(st, "a2", "Mira", PM, ["p-lera"]);
  qaActor(st, "a9", "Tobbe", PM, ["p-sven"]);
  return st;
}

describe("the player panel's /dm", () => {
  test("Send whispers /dm text to the DM from the party member the panel acts as", async () => {
    // VTT-595
    const { sent, send } = recorder();
    const { node } = panel(leraTable(), "a2", send, []);
    type(node, "say-text", "/dm I pocket the ring");
    await press(node, "send");
    expect(sent.map(whisperOf)).toEqual([{ text: "I pocket the ring", to: [], from: "a2" }]);
  });

  test("Enter whispers /DM text, trimmed, from the only party member when none is selected", async () => {
    // VTT-595
    const st = newState();
    qaActor(st, "a1", "Hero", PM, ["p-lera"]);
    const { sent, send } = recorder();
    const { node } = panel(st, "", send, []);
    type(node, "say-text", "/DM   psst  ");
    await press(node, "enter");
    expect(sent.map(whisperOf)).toEqual([{ text: "psst", to: [], from: "a1" }]);
  });

  test("a /w line or a /dm line with nothing after it sends nothing, tells the player and keeps say-text", async () => {
    // VTT-596
    for (const how of ["send", "enter"] as const) {
      for (const line of ["/w Tobbe hi", "/W Tobbe hi", '/w "Two Words" hi', "/w", "/dm", "/dm   ", "/DM"]) {
        const { sent, send } = recorder();
        const notes: string[] = [];
        const { node, ui } = panel(leraTable(), "a1", send, notes);
        type(node, "say-text", line);
        await press(node, how);
        expect({ line, how, sent: sent.length, told: notes.length }).toEqual({ line, how, sent: 0, told: 1 });
        expect(notes[0]!.length).toBeGreaterThan(0);
        expect(box(node, "say-text").value).toBe(line);
        expect(box(panel(leraTable(), "a1", send, [], ui).node, "say-text").value).toBe(line);
      }
    }
  });

  test("an accepted /dm empties say-text and a refused one keeps it", async () => {
    // VTT-520, VTT-518
    const yes = panel(leraTable(), "a1", recorder().send, []);
    type(yes.node, "say-text", "/dm one");
    await press(yes.node, "send");
    expect(box(panel(leraTable(), "a1", recorder().send, [], yes.ui).node, "say-text").value).toBe("");

    const no = panel(leraTable(), "a1", recorder(() => refused("nope")).send, []);
    type(no.node, "say-text", "/dm two");
    await press(no.node, "enter");
    expect(box(no.node, "say-text").value).toBe("/dm two");
    expect(box(panel(leraTable(), "a1", recorder().send, [], no.ui).node, "say-text").value).toBe("/dm two");
  });

  test("while a /dm is unanswered, Send and Enter send nothing more", async () => {
    // VTT-535
    const h = held();
    const sent: Sent = [];
    const { node } = panel(leraTable(), "a1", (c) => { sent.push(c); return h.send(c); }, []);
    type(node, "say-text", "/dm one");
    await press(node, "send");
    await press(node, "enter");
    await press(node, "send");
    expect(sent.length).toBe(1);
    h.release();
    await settle();
  });

  test("/ooc goes out of character and /OOC stays in character as the party member", async () => {
    // VTT-501
    const { sent, send } = recorder();
    const { node } = panel(leraTable(), "a2", send, []);
    type(node, "say-text", "/ooc brb");
    await press(node, "send");
    type(node, "say-text", "/OOC hi");
    await press(node, "send");
    expect(sent.map((c) => c.command.case === "addNarration"
      ? [c.command.value.outOfCharacter, c.command.value.actorId, c.command.value.text] : c.command.case))
      .toEqual([[true, "", "brb"], [false, "a2", "/OOC hi"]]);
  });
});
