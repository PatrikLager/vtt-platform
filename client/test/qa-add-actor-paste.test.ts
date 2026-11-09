import { expect, test } from "bun:test";
import { parseActorJSON } from "../src/commands";

function paste(actor: unknown) {
  return parseActorJSON(JSON.stringify(actor));
}

function refusal(actor: unknown): string {
  const got = paste(actor);
  if (!(got instanceof Error)) {
    throw new Error(`pasted ${JSON.stringify(actor)} built a command; want it refused`);
  }
  return got.message;
}

function sentActor(actor: unknown) {
  const got = paste(actor);
  if (got instanceof Error) {
    throw new Error(`pasted ${JSON.stringify(actor)} was refused: ${got.message}`);
  }
  if (got.command.case !== "addActor") {
    throw new Error(`pasted actor built a ${String(got.command.case)} command`);
  }
  return got.command.value.actor!;
}

const kind = "ACTOR_KIND_NON_PARTY";

// VTT-672
test("qaPaste: a resource current written null is refused, naming the actor and the key", () => {
  const msg = refusal({ actorId: "qa-paste-null", kind, resources: { qa_pool: { current: null, max: 5 } } });
  expect(msg).toContain("qa-paste-null");
  expect(msg).toContain("qa_pool");
});

// VTT-672
test("qaPaste: a resource current left out is refused, naming the actor and the key", () => {
  const msg = refusal({ actorId: "qa-paste-out", kind, resources: { qa_pool: { max: 5 } } });
  expect(msg).toContain("qa-paste-out");
  expect(msg).toContain("qa_pool");
});

// VTT-672
test("qaPaste: an empty resource is refused, naming the actor and the key", () => {
  const msg = refusal({ actorId: "qa-paste-empty", kind: "ACTOR_KIND_PARTY_MEMBER", resources: { qa_pool: {} } });
  expect(msg).toContain("qa-paste-empty");
  expect(msg).toContain("qa_pool");
});

// VTT-672
test("qaPaste: the valueless resource is named among valued ones", () => {
  const msg = refusal({
    actorId: "qa-paste-mixed",
    kind,
    attributes: { qa_num: 2 },
    resources: { qa_full: { current: 3, max: 4 }, qa_gap: { max: 4 }, qa_zero: { current: 0 } },
  });
  expect(msg).toContain("qa-paste-mixed");
  expect(msg).toContain("qa_gap");
  expect(msg).not.toContain("qa_full");
  expect(msg).not.toContain("qa_zero");
});

// VTT-672
test("qaPaste: a resource current of 0 reaches the command, with or without a maximum", () => {
  const actor = sentActor({
    actorId: "qa-paste-zero",
    kind,
    resources: { qa_pool: { current: 0, max: 0 }, qa_bare: { current: 0 }, qa_some: { current: 3 } },
  });
  expect(actor.actorId).toBe("qa-paste-zero");
  expect(Object.keys(actor.resources).sort()).toEqual(["qa_bare", "qa_pool", "qa_some"]);
  expect(actor.resources.qa_bare!.current).toBe(0);
  expect(actor.resources.qa_some!.current).toBe(3);
});

// VTT-672
test("qaPaste: a controller is told before a valueless current", () => {
  for (const ctl of [{ controllerId: "someone" }, { controllerIds: ["someone"] }, { controllerIds: [""] }]) {
    const alone = refusal({ actorId: "qa-paste-ctl", kind, ...ctl });
    const both = refusal({ actorId: "qa-paste-ctl", kind, ...ctl, resources: { qa_pool: { current: null } } });
    expect(both).toBe(alone);
    expect(both).not.toContain("qa_pool");
  }
});

// VTT-672
test("qaPaste: a missing kind is told before a valueless current", () => {
  const alone = refusal({ actorId: "qa-paste-kind" });
  const both = refusal({ actorId: "qa-paste-kind", resources: { qa_pool: { current: null } } });
  expect(both).toBe(alone);
  expect(both).not.toContain("qa_pool");
});

// VTT-672 VTT-679
test("qaPaste: a maximum written null reaches the command as no maximum", () => {
  const actor = sentActor({ actorId: "qa-paste-nomax", kind, resources: { qa_pool: { current: 2, max: null } } });
  expect(actor.resources.qa_pool!.current).toBe(2);
  expect(actor.resources.qa_pool!.max).toBe(0);
});
