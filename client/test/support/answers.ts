// The server's answer to a command, for a test's fake `send`.
//
// So that a test which cares whether a command was accepted says so in a
// word: ok(), refused(why), or a send held open until the test releases it.

import { create } from "@bufbuild/protobuf";
import {
  CommandResultSchema, type ClientCommand, type CommandResult,
} from "../../../contract/gen/ts/vtt/v1/commands_pb";

/** The server accepted the command. */
export function ok(): CommandResult {
  return create(CommandResultSchema, { ok: true });
}

/** The server refused the command, and says why. */
export function refused(error: string): CommandResult {
  return create(CommandResultSchema, { ok: false, error });
}

/**
 * Wait one macrotask, so every answer already given and every continuation
 * hung on it has run. An answer is awaited, never assumed.
 */
export function settle(): Promise<void> {
  return new Promise((r) => setTimeout(r, 0));
}

/**
 * A `send` whose answers wait for the test. Every command it is handed
 * waits until `release` answers it; release them all inside the test that
 * sent them, because the console's locks outlive the test.
 */
export function held(): {
  send: (c: ClientCommand) => Promise<CommandResult>;
  release: (answer?: CommandResult) => void;
} {
  const queue: ((r: CommandResult) => void)[] = [];
  return {
    send: () => new Promise<CommandResult>((r) => queue.push(r)),
    release: (answer = ok()) => {
      for (const answerOne of queue.splice(0)) answerOne(answer);
    },
  };
}
