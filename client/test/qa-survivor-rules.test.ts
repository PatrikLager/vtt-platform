import "./support/dom";
import { describe, expect, test } from "bun:test";
import { readLine } from "../src/player";
import { renderPlayerPanel, type PlayerUIState } from "../src/view/player";
import { renderDMConsole } from "../src/view/dm";
import { newState } from "../src/state";
import type { Roster } from "../src/metadata";
import { ActorKind } from "../../contract/gen/ts/vtt/v1/events_pb";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";
import { ok, settle } from "./support/answers";

describe("a Say line whose first word is /w", () => {
  // VTT-668
  test("whispers to the next word with the rest as its text", () => {
    expect(readLine("/w Ada see you at noon")).toEqual({
      kind: "whisper", name: "Ada", text: "see you at noon",
    });
  });

  // VTT-668
  test("reads /w in any case", () => {
    expect(readLine("/W Ada see you")).toEqual({
      kind: "whisper", name: "Ada", text: "see you",
    });
  });

  // VTT-668
  test("trims the text at both ends and keeps its inner spacing", () => {
    expect(readLine("/w Ada   see  you   ")).toEqual({
      kind: "whisper", name: "Ada", text: "see  you",
    });
  });

  // VTT-668
  test("finds the next word past any run of spaces", () => {
    expect(readLine("/w    Ada   see you")).toEqual({
      kind: "whisper", name: "Ada", text: "see you",
    });
  });

  // VTT-668
  test("keeps the name as typed, case and all", () => {
    expect(readLine("/w aDA see you")).toEqual({
      kind: "whisper", name: "aDA", text: "see you",
    });
  });

  // VTT-668
  test("has no text when nothing follows the name", () => {
    expect(readLine("/w Ada")).toEqual({
      kind: "whisper", name: "Ada", text: "",
    });
    expect(readLine("/w Ada    ")).toEqual({
      kind: "whisper", name: "Ada", text: "",
    });
  });

  // VTT-668
  test("has no name and no text when nothing follows /w", () => {
    expect(readLine("/w")).toEqual({ kind: "whisper", name: "", text: "" });
    expect(readLine("/w   ")).toEqual({ kind: "whisper", name: "", text: "" });
  });

  // VTT-668
  test("keeps quotes in the text after an unquoted name", () => {
    expect(readLine('/w Ada "see you" soon')).toEqual({
      kind: "whisper", name: "Ada", text: '"see you" soon',
    });
  });

  // VTT-668
  test("treats a quote inside the next word as part of the name", () => {
    expect(readLine('/w Ada"Byron see you"')).toEqual({
      kind: "whisper", name: 'Ada"Byron', text: 'see you"',
    });
  });

  // VTT-668
  test("is not a whisper when /w is not the first word", () => {
    expect(readLine("see /w Ada hello").kind).toBe("say");
  });

  // VTT-668
  test("is not a whisper when the first word only starts with /w", () => {
    expect(readLine("/wx Ada hello").kind).toBe("say");
    expect(readLine("/wAda hello").kind).toBe("say");
  });

  // VTT-668
  test("reads past spaces typed before /w", () => {
    expect(readLine("   /w Ada see you")).toEqual({
      kind: "whisper", name: "Ada", text: "see you",
    });
  });
});

describe("a /w line whose next word opens with a double quote", () => {
  // VTT-668
  test("whispers to the words up to the next quote", () => {
    expect(readLine('/w "Ada Byron" see you at noon')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "see you at noon",
    });
  });

  // VTT-668
  test("reads a quoted name after /W in any case", () => {
    expect(readLine('/W "Ada Byron" see you')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "see you",
    });
  });

  // VTT-668
  test("trims the quoted name and the text after it", () => {
    expect(readLine('/w "  Ada Byron  "   see you   ')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "see you",
    });
  });

  // VTT-668
  test("closes the name at the next quote, not the last", () => {
    expect(readLine('/w "Ada" said "see you"')).toEqual({
      kind: "whisper", name: "Ada", text: 'said "see you"',
    });
  });

  // VTT-668
  test("takes the text straight after the closing quote", () => {
    expect(readLine('/w "Ada Byron"see you')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "see you",
    });
  });

  // VTT-668
  test("has no text when nothing follows the closing quote", () => {
    expect(readLine('/w "Ada Byron"')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "",
    });
    expect(readLine('/w "Ada Byron"   ')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "",
    });
  });

  // VTT-668
  test("finds the opening quote past a run of spaces", () => {
    expect(readLine('/w    "Ada Byron" see you')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "see you",
    });
  });

  // VTT-668
  test("whispers to the rest of the line when no quote closes it", () => {
    expect(readLine('/w "Ada Byron see you')).toEqual({
      kind: "whisper", name: "Ada Byron see you", text: "",
    });
  });

  // VTT-668
  test("trims the rest of the line when no quote closes it", () => {
    expect(readLine('/w "   Ada Byron   ')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "",
    });
  });

  // VTT-668
  test("has no name when a lone quote ends the line", () => {
    expect(readLine('/w "')).toEqual({ kind: "whisper", name: "", text: "" });
  });
});

describe("a Say line whose first word is /dm", () => {
  // VTT-668
  test("whispers to the DM with the rest as its text", () => {
    expect(readLine("/dm see you at noon")).toEqual({
      kind: "toDM", text: "see you at noon",
    });
  });

  // VTT-668
  test("reads /dm in any case", () => {
    for (const word of ["/DM", "/Dm", "/dM"]) {
      expect(readLine(`${word} See You`)).toEqual({
        kind: "toDM", text: "See You",
      });
    }
  });

  // VTT-668
  test("trims the /dm text at both ends and keeps its inner spacing", () => {
    expect(readLine("/dm    see  you   ")).toEqual({
      kind: "toDM", text: "see  you",
    });
  });

  // VTT-668
  test("has no text when nothing follows /dm", () => {
    expect(readLine("/dm")).toEqual({ kind: "toDM", text: "" });
    expect(readLine("/dm    ")).toEqual({ kind: "toDM", text: "" });
  });

  // VTT-668
  test("names nobody: a quoted first word stays in the text", () => {
    expect(readLine('/dm "Ada Byron" see you')).toEqual({
      kind: "toDM", text: '"Ada Byron" see you',
    });
  });

  // VTT-668
  test("is not a whisper to the DM when /dm is not the first word", () => {
    expect(readLine("see /dm hello").kind).toBe("say");
  });

  // VTT-668
  test("is not a whisper to the DM when the first word only starts /dm", () => {
    expect(readLine("/dms hello").kind).toBe("say");
    expect(readLine("/dmhello").kind).toBe("say");
  });
});

describe("the aliases the specification names", () => {
  // VTT-599
  test("/whisper in any case reads as /w does", () => {
    expect(readLine("/whisper Ada see you")).toEqual({
      kind: "whisper", name: "Ada", text: "see you",
    });
    expect(readLine('/WHISPER "Ada Byron" see you')).toEqual({
      kind: "whisper", name: "Ada Byron", text: "see you",
    });
  });

  // VTT-599
  test("/gm and /togm in any case read as /dm does", () => {
    for (const word of ["/gm", "/GM", "/togm", "/ToGM"]) {
      expect(readLine(`${word} see you`)).toEqual({
        kind: "toDM", text: "see you",
      });
    }
  });
});

type Panel = {
  node: HTMLElement;
  sent: ClientCommand[];
  told: string[];
};

function playerPanel(): Panel {
  const st = newState();
  st.Actors["actor-1"] = {
    actorId: "actor-1", name: "Ada Byron", moduleId: "module-1",
    attributes: {}, resources: {}, controllerId: "p-1",
    controllerIds: ["p-1"], kind: ActorKind.PARTY_MEMBER, abilityIds: [],
  };
  const me = { participantId: "p-1", name: "Pat", role: "player" as const };
  const ui: PlayerUIState = {
    selectedActorId: "actor-1", selectedAbilityId: "", doorsArmed: false,
  };
  const sent: ClientCommand[] = [];
  const told: string[] = [];
  const node = renderPlayerPanel(
    st, me, [], ui,
    async (c) => { sent.push(c); return ok(); },
    () => {}, undefined, (m) => { told.push(m); },
  );
  return { node, sent, told };
}

function sayBox(node: HTMLElement): HTMLInputElement {
  const box = node.querySelector<HTMLInputElement>('[data-field="say-text"]');
  if (!box) throw new Error("the panel draws no say-text box");
  return box;
}

function type(node: HTMLElement, line: string): void {
  const box = sayBox(node);
  box.value = line;
  box.dispatchEvent(new Event("input", { bubbles: true }));
}

function clickSend(node: HTMLElement): void {
  const send = Array.from(node.querySelectorAll("button"))
    .find((b) => b.textContent?.trim() === "Send");
  if (!send) throw new Error("the panel draws no Send");
  send.click();
}

function pressEnter(node: HTMLElement): void {
  sayBox(node).dispatchEvent(
    new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
  );
}

type Gesture = (node: HTMLElement) => void;

const gestures: [string, Gesture][] = [
  ["Send", clickSend],
  ["Enter", pressEnter],
];

async function sayInto(p: Panel, line: string, g: Gesture): Promise<void> {
  type(p.node, line);
  g(p.node);
  await settle();
}

async function expectGestureLive(p: Panel, g: Gesture): Promise<void> {
  await sayInto(p, "/w Ada see you", g);
  expect(p.told.length).toBe(1);
  await sayInto(p, "good evening", g);
  expect(p.sent.length).toBe(1);
}

describe("the player panel, given a line with nothing to say", () => {
  for (const [name, gesture] of gestures) {
    // VTT-666
    test(`${name} on an empty line sends nothing and tells nothing`,
      async () => {
        const p = playerPanel();
        await sayInto(p, "", gesture);
        expect(p.sent).toEqual([]);
        expect(p.told).toEqual([]);
        await expectGestureLive(p, gesture);
      });

    // VTT-666
    test(`${name} on a line of spaces sends nothing and tells nothing`,
      async () => {
        const p = playerPanel();
        await sayInto(p, "     ", gesture);
        expect(p.sent).toEqual([]);
        expect(p.told).toEqual([]);
        await expectGestureLive(p, gesture);
      });

    // VTT-666
    test(`${name} on a bare /ooc sends nothing and tells nothing`,
      async () => {
        const p = playerPanel();
        await sayInto(p, "/ooc", gesture);
        expect(p.sent).toEqual([]);
        expect(p.told).toEqual([]);
        await expectGestureLive(p, gesture);
      });

    // VTT-666
    test(`${name} on /ooc and spaces sends nothing and tells nothing`,
      async () => {
        const p = playerPanel();
        await sayInto(p, "/ooc     ", gesture);
        expect(p.sent).toEqual([]);
        expect(p.told).toEqual([]);
        await expectGestureLive(p, gesture);
      });

    // VTT-666
    test(`${name} on a spaced-out bare /ooc sends and tells nothing`,
      async () => {
        const p = playerPanel();
        await sayInto(p, "   /ooc   ", gesture);
        expect(p.sent).toEqual([]);
        expect(p.told).toEqual([]);
        await expectGestureLive(p, gesture);
      });
  }
});

type Booted = {
  asked: string[];
  sent: ClientCommand[];
};

const dmRow: Roster = { participantId: "p-dm", name: "Dee Marsh", role: "dm" };

async function boot(
  row: Roster,
  answer: boolean,
  joinLink: { open: boolean; secret: string } | null,
): Promise<Booted> {
  const asked: string[] = [];
  const sent: ClientCommand[] = [];
  const node = renderDMConsole({
    st: newState(),
    participants: [{ participantId: row.participantId, displayName: row.name }],
    adventures: [], maps: [], guideFor: async () => null,
    joinLink, roster: [dmRow, row], origin: "https://table.example",
    refreshSharing: () => {},
    send: async (c) => { sent.push(c); return ok(); },
    rerender: () => {}, notify: () => {},
    confirm: (m) => { asked.push(m); return answer; },
    doorsArmed: false, toggleDoors: () => {},
  });
  const buttons = Array.from(node.querySelectorAll("button"))
    .filter((b) => b.textContent?.trim() === "Boot");
  if (buttons.length !== 1) {
    throw new Error(`expected one Boot, found ${buttons.length}`);
  }
  buttons[0]!.click();
  await settle();
  return { asked, sent };
}

const player: Roster = {
  participantId: "p-7", name: "Mira Vale", role: "player",
};

const links: [string, { open: boolean; secret: string } | null][] = [
  ["an open link", { open: true, secret: "s-1" }],
  ["a closed link", { open: false, secret: "s-1" }],
  ["no link panel", null],
];

function sentences(m: string): string[] {
  return m.split(/(?<=[.!?])\s+/);
}

function saysSignInStopsForGood(m: string): boolean {
  return sentences(m).some((s) =>
    /sign[- ]?in/i.test(s) &&
    /stop|no longer work|not work|won't work|cease/i.test(s) &&
    /for good|permanent|forever|never again/i.test(s));
}

function saysControlsNoCharacter(m: string): boolean {
  return sentences(m).some((s) =>
    /character/i.test(s) &&
    /(no longer|not|never|n't|lose|loses|stop)\b[^.]*\bcontrol/i.test(s));
}

function saysBackOnlyByOpenLink(m: string): boolean {
  return sentences(m).some((s) =>
    /share link|link/i.test(s) && /\bonly\b/i.test(s) &&
    /\bopen\b/i.test(s) && /come back|return|rejoin/i.test(s));
}

describe("Boot's question to the DM", () => {
  for (const [label, link] of links) {
    // VTT-667
    test(`says the sign-in stops working for good, with ${label}`,
      async () => {
        const b = await boot(player, false, link);
        expect(b.asked.length).toBe(1);
        expect(saysSignInStopsForGood(b.asked[0]!)).toBe(true);
      });

    // VTT-667
    test(`says they no longer control any character, with ${label}`,
      async () => {
        const b = await boot(player, false, link);
        expect(b.asked.length).toBe(1);
        expect(saysControlsNoCharacter(b.asked[0]!)).toBe(true);
      });

    // VTT-667
    test(`says they come back only by the link while open, with ${label}`,
      async () => {
        const b = await boot(player, false, link);
        expect(b.asked.length).toBe(1);
        expect(saysBackOnlyByOpenLink(b.asked[0]!)).toBe(true);
      });
  }

  // VTT-667
  test("asks the same of a spectator's row", async () => {
    const watcher: Roster = {
      participantId: "p-8", name: "Sol Reyes", role: "spectator",
    };
    const b = await boot(watcher, false, { open: true, secret: "s-1" });
    expect(b.asked.length).toBe(1);
    const m = b.asked[0]!;
    expect(saysSignInStopsForGood(m)).toBe(true);
    expect(saysControlsNoCharacter(m)).toBe(true);
    expect(saysBackOnlyByOpenLink(m)).toBe(true);
  });

  // VTT-569
  test("holds the participant's display name", async () => {
    const b = await boot(player, false, { open: true, secret: "s-1" });
    expect(b.asked.length).toBe(1);
    expect(b.asked[0]).toContain("Mira Vale");
  });

  // VTT-569
  test("sends nothing on a no", async () => {
    const b = await boot(player, false, { open: true, secret: "s-1" });
    expect(b.asked.length).toBe(1);
    expect(b.sent).toEqual([]);
  });

  // VTT-570
  test("sends the removal of that participant on a yes", async () => {
    const b = await boot(player, true, { open: true, secret: "s-1" });
    expect(b.asked.length).toBe(1);
    expect(b.sent.length).toBe(1);
    const c = b.sent[0]!.command;
    expect(c.case).toBe("removeParticipant");
    if (c.case === "removeParticipant") {
      expect(c.value.participantId).toBe("p-7");
    }
  });
});
