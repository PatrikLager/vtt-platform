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
  fetchMe, fetchRuleset, fetchAdventures, fetchAdventureGuide, fetchMaps,
  fetchJoinLink, fetchParticipants,
  type Ability, type AdventureMeta, type Me, type JoinLink, type Roster, type MapMeta,
} from "./metadata";
import { renderDMConsole } from "./view/dm";
import { setViewpoint } from "./commands";
import { joinSecretFrom, requestJoin } from "./join";
import { renderJoinView, type JoinViewState } from "./view/join";
import { loadPackImages, loadStandardPackImages } from "./view/pack-assets";
import type { ImageMap } from "./view/canvas";
import type { ClientCommand } from "../../contract/gen/ts/vtt/v1/commands_pb";

function gatewayURL(): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/ws`;
}

/**
 * Whether this role may read the join link and the roster.
 *
 * One definition rather than the three copies this grew: the answer is the
 * same question every time — the routes are gated dm/agent server-side — and
 * three copies is three places for it to drift out of step with the server.
 */
function mayShare(who: Me): boolean {
  // Takes a Me, not a Me | null. Every caller has already established that it
  // has one — a null check here would be a branch no call site can reach, and
  // an unreachable branch is a claim no test can check.
  return who.role === "dm" || who.role === "agent";
}

export function boot(root: HTMLElement): Session | null {
  const auth = new Auth(localStorage);
  const params = new URLSearchParams(location.search);
  // A token in the URL WINS over a stored one. The other order looks safer
  // and is not: it made re-invitation impossible, because a player who had
  // ever connected kept their old identity no matter which link they opened,
  // with no recovery short of clearing site data.
  //
  // The cost is that on a shared machine whoever pastes a link last is who
  // you are. That is the authority the link already carries — it is a bearer
  // credential, and anyone holding it can open a private window and be that
  // identity anyway — and unlike the previous behaviour it is recoverable.
  const token = params.get("token") ?? auth.get();
  if (token) {
    // A token arriving in the URL is stored and then removed from the address
    // bar: leaving it there puts a bearer credential into browser history and
    // into the Referer of every outbound link.
    auth.set(token);
    history.replaceState(null, "", location.pathname);
    return startSession(root, token);
  }

  // No credential — but perhaps a way to get one. THE STORED TOKEN ABOVE WINS
  // OVER THIS, which is the opposite precedence from ?token= and deliberately
  // so. A ?token= link is an act of re-invitation aimed at one person, so it
  // overrides. A ?join= link is a durable URL the whole table keeps and
  // reopens; if it won, every visit would mint a NEW participant, and a
  // returning player would arrive as a stranger with none of their characters
  // while the roster filled with duplicates nobody can tell apart.
  const secret = joinSecretFrom(params);
  if (secret !== null) {
    runJoin(root, auth, secret);
    return null;
  }

  root.textContent = "No invite token. Open the link your DM sent you.";
  return null;
}

/** Ask who this person is, exchange that for their own token, then boot. */
function runJoin(root: HTMLElement, auth: Auth, secret: string): void {
  const state: JoinViewState = { name: "", busy: false, error: "" };
  const paint = () =>
    renderJoinView(root, state, {
      // NO repaint on a keystroke: rebuilding the input mid-typing throws away
      // the caret. The value is re-seeded from state on the paints that do
      // happen, which is what keeps a refusal from costing them their typing.
      onName: (v) => {
        state.name = v;
      },
      onSubmit: () => void submit(),
    });

  const submit = async () => {
    // THE DISABLED BUTTON IS NOT THE WHOLE GUARD. Enter goes to the INPUT,
    // which is not disabled, so pressing Join and then tapping Enter would
    // post twice — and every post mints a PARTICIPANT. The same person would
    // be at the table twice holding two credentials, one of which nothing
    // will ever revoke because nobody knows it exists.
    //
    // Disabling the input instead would be the tidier fix and is not
    // available: a disabled control receives no keydown in a real browser,
    // but happy-dom delivers one anyway (measured), so the assignment would
    // be code this gate cannot observe. The guard goes where a test can reach
    // it.
    if (state.busy) return;
    state.busy = true;
    state.error = "";
    paint();

    const out = await requestJoin(location.origin, secret, state.name);
    if (!out.ok) {
      state.busy = false;
      state.error = out.message;
      paint();
      return;
    }
    // Stored and stripped in the same breath as the invite path above, and for
    // the same reason — except this URL is worse to leave lying about, because
    // the secret in it admits ANYONE, not just the person holding it.
    auth.set(out.token);
    history.replaceState(null, "", location.pathname);
    startSession(root, out.token);
  };

  paint();
}

function startSession(root: HTMLElement, token: string): Session {
  const session = new Session(gatewayURL(), token);
  let status = "connecting";
  let failure = "";
  let me: Me | null = null;
  let abilities: Ability[] = [];
  let adventures: AdventureMeta[] = [];
  let joinLink: JoinLink | null = null;
  let roster: Roster[] | null = null;
  let toast = "";

  // Real pack art (Task 10), merged in as each configured map's pack
  // resolves — see pack-assets.ts's own header comment for why this loads
  // EVERY configured map's pack rather than trying to correlate a live
  // scene back to one specific map. loadedPacks guards against re-fetching
  // the same pack every time paint() happens to run again before the first
  // load has resolved.
  let images: ImageMap = {};

  // The standard-vocabulary BASELINE (review finding C2, 2026-08-16): every
  // one of the eleven std:<kind>/<material> pictures, so a square with no
  // art override draws SOMETHING instead of nothing (both shipped
  // adventures carry zero overrides — see pack-assets.ts's own header
  // comment). Fired unconditionally, right here, rather than waiting on
  // fetchMe/fetchMaps below: unlike a configured map's own pack, this one
  // needs no token and no server-side maps/adventures configuration at all
  // (it comes straight from the client's own bundle, "/std-pack/..."), so it
  // must not be gated behind — or lost if — that metadata chain fails.
  // Merged UNDER whatever a map's own pack later supplies: "tile:" and
  // "std:" keys never collide (disjoint prefixes — scene-plan.ts's
  // tileImage picks one or the other, never both, for a given square), so
  // this is "baseline" in the sense the finding means (present for anything
  // an override does not name), not in merge-order precedence.
  void loadStandardPackImages(location.origin)
    .then((imgs) => {
      images = { ...images, ...imgs };
      paint();
    })
    .catch(() => {
      // loadStandardPackImages already tolerates a 404'd manifest or a
      // failed image (its own doc comment): this only catches the fetch
      // call itself throwing (e.g. no network) rather than answering with a
      // Response — the same "metadata unavailable degrades the client
      // gracefully" posture as the fetchMe/.../fetchMaps chain's own
      // trailing .catch below.
    });

  const loadedPacks = new Set<string>();
  const loadMapPacks = (maps: MapMeta[]) => {
    for (const m of maps) {
      if (!m.pack || loadedPacks.has(m.pack.id)) continue;
      loadedPacks.add(m.pack.id);
      void loadPackImages(location.origin, token, m.pack.id).then((imgs) => {
        images = { ...images, ...imgs };
        paint();
      });
    }
  };

  // Re-read the door, the link and the roster.
  //
  // Needed because these commands produce NO EVENT, deliberately: a role and a
  // door are identity state, not campaign history (spec §3.1, §4). Nothing
  // broadcasts, so nothing re-renders — without this the DM opens the door and
  // the console goes on saying "closed" until something unrelated happens, and
  // promotes a spectator who stays listed as one.
  //
  // Fired unconditionally rather than only for dm/agent: the routes are
  // role-gated server-side and answer null for anyone else, so a player's
  // console simply keeps no sharing panel.
  // A MONOTONIC TICKET PER REFRESH, so a slow answer cannot overwrite a fast
  // one. Two refreshes can be in flight at once — a promotion and an arrival a
  // moment apart — and both assignments below are last-writer-wins on
  // COMPLETION, not on issue order. A stale roster landing second repaints a
  // promoted player as the spectator they were.
  // An IDENTITY, not a counter. "Is this still the newest request?" is a
  // question about sameness, and a number answers it only by convention — any
  // strictly monotonic sequence behaves identically, so counting up and
  // counting down are indistinguishable and the direction is a detail no test
  // could ever justify. A fresh object per call has no direction to get wrong.
  let newest: object = {};
  const refreshSharing = () => {
    const ticket = {};
    newest = ticket;
    const current = () => ticket === newest;
    // CAUGHT, both of them, and the two failures are NOT the same failure.
    //
    // getJSON throws on 401/403 as well as on a network fault. A refusal means
    // "you may not read this" and dropping the panel is the right answer — a
    // revocation mid-session lands here. A network blip means "this read
    // failed", and dropping the panel for that takes the door and the roster
    // controls away from a DM who still has every right to them, silently,
    // until the next presence frame. So a transient KEEPS what is on screen.
    const refused = (e: unknown) => /forbidden|unauthorized/.test(String(e));
    void fetchJoinLink(location.origin, token)
      .then((l) => {
        if (!current()) return;
        joinLink = l;
        paint();
      })
      .catch((e: unknown) => {
        if (!current() || !refused(e)) return; // keep what we have; try again next time
        joinLink = null;
        paint();
      });
    void fetchParticipants(location.origin, token)
      .then((r) => {
        if (!current()) return;
        roster = r;
        paint();
      })
      .catch((e: unknown) => {
        if (!current() || !refused(e)) return;
        roster = null;
        paint();
      });
  };
  const ui: PlayerUIState = { selectedActorId: "", selectedAbilityId: "" };

  // The shoulder this spectator is riding (visibility spec §3.1.1). "" is a
  // real value and the one every connection opens in: perched on nobody, which
  // is no eyes and no board at all.
  //
  // WHAT THE SERVER CONFIRMED, never what was last clicked — see perchOn.
  let viewpoint = "";
  // Whether a perch has SHAPED THE LOG this client is holding, which is the
  // one question the redial below has to answer. See it for why neither "am I
  // perched right now" nor "am I a spectator" is that question.
  let perchShaped = false;

  // Returns the promise rather than swallowing it, so a caller that must read
  // server state back AFTER a command lands can wait for it. The DM console
  // does: the door and role commands produce no event, so an HTTP re-read
  // issued beside the command races it on a different transport and can repaint
  // the panel with the state the command was about to change.
  const act = (cmd: ClientCommand): Promise<void> =>
    session.send(cmd).then((res) => {
      // The result is shown verbatim on failure. A player who is told "not
      // authorized" can act on that; a silent no-op looks like a broken UI.
      toast = res.ok ? "" : `refused: ${res.error}`;
      paint();
    });

  /**
   * Hop onto a shoulder, or off one.
   *
   * THE INDICATOR MOVES ON THE SERVER'S ANSWER, not on the click, and the
   * extra line that costs is the point. A refused perch that had already moved
   * the label would tell the watcher they are riding a shoulder the server
   * never gave them, while the board — which comes from the server — showed
   * something else. That is the "the board changes under you with no way to
   * know why" failure the indicator exists to prevent, arriving from the other
   * direction. The refusal is shown verbatim, like every other command's.
   *
   * The empty id goes through here unchanged: it is a real command with a real
   * effect (setViewpoint's own comment), never a no-op to be skipped.
   */
  const perchOn = (actorId: string): Promise<void> =>
    session.send(setViewpoint(actorId)).then((res) => {
      if (res.ok) viewpoint = actorId;
      // STICKY, and set by a shoulder rather than by the current one. From the
      // moment a perch is accepted this client's log holds frames no replay
      // will ever produce again — see redial.
      if (res.ok && actorId !== "") perchShaped = true;
      toast = res.ok ? "" : `refused: ${res.error}`;
      paint();
    });

  /**
   * Redial after the connection dropped.
   *
   * A CLIENT WHOSE LOG A PERCH HAS SHAPED STARTS OVER; everyone else resumes.
   * A perch is connection state, so the server's projector is reborn perched
   * on nobody and its memory no longer matches the board this client is
   * holding — Wire.restart has the measurements, including why dropping the
   * sequence-0 frames instead makes it worse rather than better.
   *
   * THE CONDITION IS ABOUT THE LOG, and the two nearer-looking questions are
   * both wrong. "Am I perched RIGHT NOW" misses the watcher who hopped OFF
   * before redialling: nothing on the wire un-introduces a scene or an actor,
   * so their log still holds everything the perch put there. "Am I a
   * spectator" misses the other direction — a role is not fixed, and a
   * spectator PROMOTED to player (which the presence handler below re-reads
   * and applies mid-session) would take the resume path still holding perch
   * frames, and their new seat's projection re-introduces what it never sent
   * them. Whether a perch has been accepted on this log is neither, and it is
   * the thing that actually decides.
   *
   * Then the shoulder is re-sent, because the perch is the client's to keep
   * (spec §3.1.1: "a perch does not survive a reconnect, and the client
   * re-sends it on connect"). Sent as soon as the socket opens rather than
   * after the replay: the gateway's pump delivers this seat's whole backlog
   * before it takes a perch out of its box (internal/gateway/server.go), so
   * the ordering is structural and not a race — measured on the real server,
   * where an immediate perch's frames still arrived behind the entire replay.
   *
   * NOT re-sent when there is no shoulder: a fresh connection is already
   * perched on nobody, so setViewpoint("") would be a command with nothing to
   * do — and the emptied log is then no longer perch-shaped, which is why the
   * flag is cleared rather than left standing.
   */
  const redial = async (): Promise<void> => {
    if (!perchShaped) {
      await session.reconnect();
      return;
    }
    await session.restart();
    perchShaped = false;
    if (viewpoint !== "") await perchOn(viewpoint);
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
    // The one role that perches. MayPerch refuses the other three outright, so
    // offering them the control would be an affordance whose every use is a
    // refusal — and an unassigned PLAYER's answer to an empty board is to be
    // given a character, which is the onboarding flow working as intended.
    const isSpectator = me !== null && me.role === "spectator";
    renderSpectator(root, session.state, [...session.events], status, {
      panel: canAct ? renderPlayerPanel(session.state, me!, abilities, ui, act, paint) : undefined,
      console: isDM
        ? renderDMConsole({
            st: session.state,
            log: [...session.events],
            participants: session.participants,
            adventures,
            guideFor: (id) => fetchAdventureGuide(location.origin, token, id),
            joinLink,
            roster,
            origin: location.origin,
            refreshSharing,
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
      participants: session.participants,
      perch: isSpectator ? { current: viewpoint, onPerch: (id) => void perchOn(id) } : undefined,
      // Manual by spec §3.4. A player resumes from the last sequence already
      // folded; a spectator starts over, and redial says why.
      onReconnect: () => {
        void redial();
      },
      images,
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
  // TWO THINGS FOLLOW FROM A PRESENCE FRAME, and both are invisible without
  // one because neither produces an event.
  //
  // Hooked to presence rather than to onChange, which fires per FRAME — a
  // table in play is a frame a second, and a roster read behind every token
  // move is not a feature. An earlier version watched onChange and kept a key
  // of the participant set to tell a real change from an event; the key, its
  // sort and its separator were all machinery for a question presence already
  // answers.
  //
  // MY OWN ROLE CAN MOVE WHILE I AM SITTING HERE.
  //
  // /api/me is read once at connect, and a promotion changes what this person
  // may do without changing anything they can see: the server starts accepting
  // their commands and their screen still offers none. The server re-announces
  // a promoted participant for exactly this reason, so a presence batch naming
  // ME means "read your role again".
  //
  // Watching the participant LIST cannot serve here: a promotion re-announces
  // somebody already present, so the list does not change.
  session.onPresence((batch) => {
    if (me === null) return;

    // WHO ELSE IS HERE HAS CHANGED, so the roster the console renders is out
    // of date — a joiner with no promote button beside their name, or a
    // revoked participant still offered one. Fires on any presence frame,
    // including the re-announcement a promotion sends.
    if (mayShare(me)) refreshSharing();

    // AND MY OWN ROLE MAY HAVE MOVED. /api/me is read once at connect, and a
    // promotion changes what this person may do without changing anything
    // they can see: the server starts accepting their commands and their
    // screen still offers none. The server re-announces a promoted
    // participant for exactly this reason, so a batch naming ME means "read
    // your role again".
    const mine = me.participantId;
    if (!batch.some((p) => p.participantId === mine)) return;
    void fetchMe(location.origin, token).then((m) => {
      me = m;
      paint();
      // NO refresh here. A promotion is bounded to player and spectator
      // (spec §3.1a) and the server refuses a target who is currently dm or
      // agent, so re-reading a role can never WIDEN what this person may
      // read: whenever the new role could see the sharing panel, the old one
      // already could, and the refresh above has already run. A second call
      // guarded on the new role would be a branch no supported transition can
      // reach.
    });
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
      // As soon as the ROLE is known, and deliberately not later in this
      // chain. It sat after the ruleset and adventure fetches at first, which
      // meant a table with no ruleset — a perfectly ordinary state this client
      // already degrades gracefully for — threw before reaching it, and the DM
      // silently never got a sharing panel or a promote control. Found by the
      // test below, not by reading it.
      //
      // Gated on the role so a player never asks for a link the server would
      // refuse them.
      if (mayShare(me)) refreshSharing();
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
      return fetchMaps(location.origin, token);
    })
    .then((maps) => {
      // Kicks off pack loading; does NOT block on it (loadMapPacks fires
      // fetches and returns immediately) — a slow or large pack must not
      // hold up anything else in this chain, and each pack's own images
      // arrive on their own schedule via the .then inside loadMapPacks.
      loadMapPacks(maps);
    })
    .catch(() => {
      // Metadata being unavailable degrades the client to spectator-shaped:
      // the board and story still work, which is better than a blank page.
    });

  void session.start();
  return session;
}
