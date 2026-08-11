import { expect, test } from "bun:test";
import fixture from "../../contract/testdata/join_url_format.json";

// The ?join= format spans Go and TypeScript, so neither mutation gate can see
// it whole: gremlins does not mutate string literals, and Stryker cannot see
// the Go side at all. A shared constant is not available across two languages
// either — the fixture is the nearest thing to one, and #46 is what happens
// without it.
//
// The DM console's own writer is asserted against this fixture in
// dm-view.test.ts, where the console harness lives.

test("the fixture agrees with itself", () => {
  // A fixture four sites derive from must be internally consistent, or half of
  // them are sent somewhere the other half does not look — and every
  // fixture-derived assertion still passes, because they all derive from the
  // same wrong thing.
  expect(fixture.shareSuffix).toBe(`/?${fixture.queryParameter}=`);
  expect(fixture.exampleShareURL).toContain(fixture.shareSuffix);
});

test("the reader looks for the parameter the writers write", () => {
  // app.ts reads with URLSearchParams.get(...), so what needs pinning is
  // narrow and sharp: the NAME the reader asks for is the name a writer
  // produced. Driving the whole boot here would test the join flow instead.
  const url = new URL(`https://table.example${fixture.shareSuffix}THE-SECRET`);
  expect(url.searchParams.get(fixture.queryParameter)).toBe("THE-SECRET");
});

test("app.ts asks for exactly this parameter", async () => {
  // The READER side, read from the source. app.ts is a module with a
  // DOM-dependent boot, so importing it to observe the lookup would drag the
  // whole client in; the lookup itself is one call and this pins its argument.
  const src = await Bun.file(new URL("../src/app.ts", import.meta.url)).text();
  expect(src).toContain(`params.get("${fixture.queryParameter}")`);
});
