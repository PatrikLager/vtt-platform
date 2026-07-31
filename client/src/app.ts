// The app shell: read the token, open a session, and report status.
//
// Deliberately thin. This task (T4) owns the WIRE — connect, replay, the
// reconnect cursor, request correlation — and that logic lives in wire.ts and
// session.ts where it is unit-tested against a fake gateway. Rendering the
// board, the story feed and the action panels is T6-T8's job, and the build
// that bundles this into cmd/vtt/webdist is T5's.
//
// What is here is only the composition: which pieces talk to which, and what
// the user sees before any of them have anything to show.

import { Auth } from "./auth";
import { Session } from "./session";
import type { WireStatus } from "./wire";
import { renderSpectator } from "./view/spectator";
import { renderPlayerPanel, moveCommandFor, type PlayerUIState } from "./view/player";
import {
  fetchMe, fetchRuleset, fetchAdventures, fetchAdventureGuide,
  type Ability, type AdventureMeta, type Me,
} from "./metadata";
import { renderDMConsole } from "./view/dm";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";

function gatewayURL(): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/ws`;
}

export function boot(root: HTMLElement): Session | null {
  const auth = new Auth(localStorage);
  const token = auth.get() ?? new URLSearchParams(location.search).get("token");
  if (!token) {
    root.textContent = "No invite token. Open the link your DM sent you.";
    return null;
  }
  // A token arriving in the URL is stored and then removed from the address
  // bar: leaving it there puts a bearer credential into browser history and
  // into the Referer of every outbound link.
  auth.set(token);
  history.replaceState(null, "", location.pathname);

  const session = new Session(gatewayURL(), token);
  let status = "connecting";
  let failure = "";
  let me: Me | null = null;
  let abilities: Ability[] = [];
  let adventures: AdventureMeta[] = [];
  let toast = "";
  const ui: PlayerUIState = { selectedActorId: "", selectedAbilityId: "" };

  const act = (cmd: ClientCommand) => {
    void session.send(cmd).then((res) => {
      // The result is shown verbatim on failure. A player who is told "not
      // authorized" can act on that; a silent no-op looks like a broken UI.
      toast = res.ok ? "" : `refused: ${res.error}`;
      paint();
    });
  };

  const paint = () => {
    if (failure !== "") {
      root.replaceChildren();
      const p = document.createElement("p");
      p.className = "fatal";
      // A fold error means the client cannot derive the true board. Saying so
      // is the honest option: rendering a plausible-looking wrong board would
      // invite a player to act on a position that never existed.
      p.textContent = `Cannot derive state from the log: ${failure}`;
      root.appendChild(p);
      return;
    }
    const canAct = me !== null && (me.role === "player" || me.role === "dm" || me.role === "agent");
    const isDM = me !== null && (me.role === "dm" || me.role === "agent");
    renderSpectator(root, session.state, [...session.events], status, {
      panel: canAct ? renderPlayerPanel(session.state, me!, abilities, ui, act, paint) : undefined,
      console: isDM
        ? renderDMConsole({
            st: session.state,
            log: [...session.events],
            adventures,
            guideFor: (id) => fetchAdventureGuide(location.origin, token, id),
            send: act,
            notify: (m) => {
              toast = m;
              paint();
            },
            // window.confirm is deliberate for a destructive action: it is
            // modal and unmissable, which a custom banner is not.
            confirm: (m) => window.confirm(m),
          })
        : undefined,
      onCell: canAct
        ? (cell) => {
            const cmd = moveCommandFor(session.state, me!, ui, cell);
            if (cmd) act(cmd);
          }
        : undefined,
      toast: toast || undefined,
    });
  };

  session.onStatus((s: WireStatus) => {
    status = s === "open" ? "connected" : s;
    paint();
  });
  session.onError((e) => {
    failure = e.message;
    paint();
  });
  session.onChange(paint);

  paint();

  // Identity and the ability list come from the HTTP side; both are needed
  // before the player panel can render anything useful, and neither blocks
  // the board from appearing.
  void fetchMe(location.origin, token)
    .then((m) => {
      me = m;
      paint();
      return fetchRuleset(location.origin, token);
    })
    .then((rs) => {
      abilities = rs.abilities;
      paint();
      return fetchAdventures(location.origin, token);
    })
    .then((advs) => {
      adventures = advs;
      paint();
    })
    .catch(() => {
      // Metadata being unavailable degrades the client to spectator-shaped:
      // the board and story still work, which is better than a blank page.
    });

  void session.start();
  return session;
}

// Guarded so importing this module in a test does not require a DOM.
if (typeof document !== "undefined") {
  const root = document.getElementById("app");
  if (root) boot(root);
}
