import { describe, expect, test } from "bun:test";
import { parseActorJSON } from "../src/commands";

function qaAbPastedAbilities(paste: Record<string, unknown>): string[] {
  const cmd = parseActorJSON(JSON.stringify(paste));
  if (cmd instanceof Error) {
    throw new Error(`paste refused: ${cmd.message}`);
  }
  if (cmd.command.case !== "addActor") {
    throw new Error(`paste built ${String(cmd.command.case)}, want addActor`);
  }
  return [...(cmd.command.value.actor?.abilityIds ?? [])];
}

describe("qaAb: the console's paste carries a sheet's abilities", () => {
  // VTT-683
  test("qaAb: a pasted actor's abilityIds reach the command in their order", () => {
    const want = ["sober-up", "Fists", "chair-swing", "fists", "zz-unheld"];
    expect(
      qaAbPastedAbilities({
        actorId: "qa-paste",
        kind: "ACTOR_KIND_PARTY_MEMBER",
        attributes: { brawn: 2 },
        abilityIds: want,
      }),
    ).toEqual(want);
  });

  // VTT-683
  test("qaAb: a repeated id in a pasted actor reaches the command as pasted", () => {
    const want = ["fists", "chair-swing", "fists"];
    expect(
      qaAbPastedAbilities({ actorId: "qa-paste-repeat", kind: "ACTOR_KIND_NON_PARTY", abilityIds: want }),
    ).toEqual(want);
  });

  test("qaAb: a pasted actor naming no ability builds a command naming none", () => {
    expect(qaAbPastedAbilities({ actorId: "qa-paste-none", kind: "ACTOR_KIND_NON_PARTY" })).toEqual([]);
  });
});
