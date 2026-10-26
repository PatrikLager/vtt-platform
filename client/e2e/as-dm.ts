import { expect, type Page } from "@playwright/test";

/** asDM sends the DM's commands on a raw socket, one at a time; any refusal fails the test. */
export async function asDM(page: Page, dmToken: string, commands: Record<string, unknown>[]) {
  const refused = await page.evaluate(
    async ([t, cs]) => {
      const ws = new WebSocket(`${location.origin.replace("http", "ws")}/ws?token=${t}&after=0`);
      await new Promise((r) => ws.addEventListener("open", r));
      const out: string[] = [];
      for (const [i, c] of cs.entries()) {
        const requestId = `e2e-dm-${i}`;
        const answer = new Promise<{ ok?: boolean; error?: string }>((r) =>
          ws.addEventListener("message", (m) => {
            const f = JSON.parse(String(m.data));
            if (f.result?.requestId === requestId) r(f.result);
          }),
        );
        ws.send(JSON.stringify({ requestId, ...c }));
        const res = await answer;
        if (!res.ok) out.push(`${Object.keys(c)[0]}: ${res.error}`);
      }
      ws.close();
      return out;
    },
    [dmToken, commands] as const,
  );
  expect(refused).toEqual([]);
}
