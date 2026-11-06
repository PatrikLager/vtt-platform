# A player sees a companion's sheet only once their character has seen the companion

## The problem

**Every character log is sent every companion's whole sheet, seen or not.**
- `look` in `internal/perceive/projection.go` puts every party member into the look's `actors`, under the comment "Know every party member, seen or not (SPEC-016)".
- `transitions` introduces each actor of the look that the log does not hold. `introduce` sends it as `shownAs` gives it, and `shownAs` gives a party member whole: its name, attributes, resources and module data (VTT-485).
- So party member B's character log is written party member A's sheet as soon as A is a party member, whether or not B's character has ever seen A.
- Every seat that watches B's log is sent it, so a player reads the sheet of a companion their character has never laid eyes on.

**The rest of the owner's rule already holds:**
- a companion's changes reach a log only while its character sees the companion: `classify`'s `ResourceChanged` arm forwards on `saw` (VTT-486);
- a companion out of sight that the log's character has seen is held as it was last seen, and is brought to its present when it comes into sight again (VTT-469). VTT-471 says the same of every party member, seen or not, and this ticket narrows it;
- a creature reaches a log without any value (VTT-465);
- the DM and the agent read the world (VTT-176).

**Why it is wrong.**
- The owner ruled on 2026-11-06: "En spelare som "ser" sin medspelare, kan se dess character sheet, allt … En spelare som inte ser sin medspelare, kan inte se character sheet, för du kan inte se "karaktären" - DM ser allt. NPC och monster, kan endast ses av DM".
- Blueprint principle 16 says a player is sent exactly what the character they follow perceived.
- The same ruling sets aside ADR-012 Decision §2's visibility per property: "ett spel bland vänner". No property is marked; sight decides.

## Done looks like

1. **An unseen companion arrives without its sheet.** In a world where B's character has never seen party member A, B's log holds A with its id, name, kind, module id and controllers, and with no attributes, resources, module data or conditions. A `perceive` test that does not exist yet shows it, and fails today.
2. **The sheet arrives with the first sight.** At the event at which B's character first sees A, B's log is brought to A's whole sheet as it is then. A `perceive` test that does not exist yet shows it, and fails today because the sheet arrived earlier.
3. **The window tells the truth about it.** At that first sight, a window watching B's log does not say that A left, or that A joined the party again. Its Story shows A coming into view as it does today for a companion already held. The Events panel is drawn only for the DM and the agent, whose windows hold the world, so it does not change. A client test that does not exist yet shows it.
4. **What already holds still holds.**
   - A companion the log's character has seen stays at its last-seen sheet out of sight and is brought to its present on sight.
   - A creature arrives without values.
   - The DM's and the agent's streams are unchanged.

   The existing tests of VTT-469, VTT-486, VTT-465 and VTT-176 pass. Where a test's fixture corrects or holds whole a companion its viewer has never seen, that fixture changes so the viewer sees it first, and its assertions stay.
5. **End to end.** A seat watching B through the gateway is sent no frame that carries A's attributes or resources before B's character sees A. A cross-layer test that does not exist yet shows it, and fails today.
6. **The gate passes.** `task check` exits 0.

## Rules this puts on the system

- A character log is introduced to a party member its character has not seen with that party member's id, name, kind, module id and controllers alone.
- A character log is brought to a party member's whole sheet at the event at which its character first sees that party member.
- A window watching a character log does not describe, as a departure and a fresh arrival, the withdrawal and re-introduction that bring it a companion's sheet.
- The ticket narrows existing rows, each withdrawn and succeeded per the requirements skill:
  - VTT-485, which introduces every party member whole;
  - VTT-471, which holds every party member out of sight at its resources;
  - VTT-449, under which the Story shows every entry of the log its window holds;
  - VTT-317, under which a log opens with the party as the world stood, found at Task 0's review.

## What it touches

1. `internal/perceive/projection.go`: `look`, `shownAs` and `shownWith`, and their callers `introduce`, `correct`, `Resume` and `recall`.
2. The `internal/perceive` tests that introduce a party member nobody has seen.
3. `internal/campaign` and `internal/gateway` tests that read a companion's values from a log before its sight.
4. The client's Story, in `client/src/view/feed.ts` (`buildFeed`), with `task build:client` and the committed `cmd/vtt/webdist` bundle.

The ruleset, `/api/ruleset` and the contract are not touched.

## Specifications this moves

docs/specifications/016-the-projection.md
docs/specifications/021-each-party-member-keeps-its-own-log.md
docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **The name.** The owner's ruling says a player cannot see an unseen companion's sheet. This ticket reads the companion's name as not part of the sheet, so a log still knows every party member by name, and a window's Story still says "… is in the party" (`describe` in `client/src/view/spectator.ts`). Whom a seat may follow is named by the gateway (`watchable`), not by the log. The owner signed off the plan with this reading on 2026-11-06.
- **Logs already written.** A log written before this change holds every companion's sheet from its opening, and keeps it (blueprint principle 18). Nobody plays the product yet, which is the owner's ruling of 2026-09-04 on removing the old solution first.
