import { stopFixtures } from "./setup";

export default function globalTeardown(): void {
  stopFixtures();
}
