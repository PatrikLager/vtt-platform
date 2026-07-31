import { startFixture } from "./setup";

export default async function globalSetup(): Promise<void> {
  const f = await startFixture();
  // eslint-disable-next-line no-console
  console.log(`e2e: serving at ${f.base}`);
}
