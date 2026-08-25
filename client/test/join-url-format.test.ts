import { expect, test } from "bun:test";
import fixture from "../../contract/testdata/join_url_format.json";
import { joinSecretFrom } from "../src/join";

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

test("the client's reader returns the secret a fixture-shaped link carries", () => {
  // The READER side, OBSERVED rather than read out of the source.
  //
  // This used to assert that app.ts's text contained `params.get("join")`.
  // That assertion could not survive mutation testing: Stryker rewrites string
  // literals in every file it mutates, so the text it looked for stopped
  // existing and the gate died in its dry run, before a single mutant ran.
  // See joinSecretFrom's own comment for the whole story.
  //
  // Behaviour is also the better assertion. Text proved a call was WRITTEN;
  // this proves it WORKS, on a URL built from the fixture rather than from a
  // literal typed here — which is the whole point of there being a fixture.
  const url = new URL(`https://table.example${fixture.shareSuffix}THE-SECRET`);
  expect(joinSecretFrom(url.searchParams)).toBe("THE-SECRET");
});

test("the client's reader reports NO secret when the link carries none", () => {
  // The other half, and not decoration: a reader that returned something for
  // an ordinary visit would send every arriving player down the join flow
  // instead of the table, and the assertion above cannot tell the difference.
  expect(joinSecretFrom(new URL("https://table.example/").searchParams)).toBeNull();
});
