import { stopFixture } from "./setup";

export default function globalTeardown(): void {
  stopFixture();
}
