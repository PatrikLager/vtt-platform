# Session zero — findings

Dry run for the first real session.
Spec: `docs/superpowers/specs/2026-08-12-first-real-session-design.md`
Plan: `docs/superpowers/plans/2026-08-12-first-real-session.md`

Working notes. Every entry is something that needed a human to notice, explain
or work around — **including things that worked but were awkward**, because
those are the ones memory discards before session one.

Format per entry:

- **WHAT** happened
- **WHERE** — event sequence, screen, or command
- **COST** — stopped me / cost time / awkward

Tunnel hostnames are redacted throughout: a quick tunnel is a live door to the
machine while `cloudflared` runs, the full name adds nothing as evidence, and
the URL is dead within hours anyway.

---

## Entries

### 1. WebSocket upgrade through the tunnel — PASS

**WHAT.** The gateway calls `websocket.Accept(w, r, nil)`. With nil options
coder/websocket applies its default origin policy (`accept.go:239`): the
browser's `Origin` host must equal the `Host` header the server sees, with no
fallback patterns. Whether cloudflared preserves the original `Host` or rewrites
it to the origin URL is config-dependent and could not be settled by reading
code — so this was the plan's Task 1 and gated everything else.

**It passes.** cloudflared preserves the Host; the policy is satisfied.

| probe | result | role |
|---|---|---|
| localhost, `Origin: http://localhost:8080` | `101 Switching Protocols` | positive control |
| localhost, `Origin: https://evil.com` | `403 Forbidden` | negative control |
| tunnel, `Origin: https://katrina-….trycloudflare.com` | `101 Switching Protocols` | the answer |

**The negative control is what makes this a result.** Without it, `101` through
the tunnel could equally have meant "the origin check never ran". With a
confirmed `403` on a bad origin, we know the check is live *and* satisfied.

**WHERE.** 2026-08-12, cloudflared 2026.7.3, quick tunnel, `vtt serve` on :8080.

**COST.** None. No allowed-origin configuration is needed, so spec §3's
fix-properly path is not entered.

### 2. Two false results while establishing entry 1 — METHOD, not platform

Recorded because anyone re-running this will hit both, and because each looked
like an answer.

**`426 Upgrade Required` is not a rejection.** The first probe used plain
`curl`, which negotiates HTTP/2 with Cloudflare — and a classic
`Upgrade: websocket` header is HTTP/1.1 only. It reads like a refusal and is
actually the wrong protocol. **`--http1.1` is mandatory for this probe.**

**A malformed probe returned a plausible `101`.** The next attempt built the
header with `${2:+-H "Origin: $2"}`, which word-splits in the shell: the header
arrived as `"Origin:` + `localhost"`. Its controls returned `400`, which is what
exposed it — otherwise the tunnel's `101` would have been reported as success
while actually taking the *no-Origin* path (`accept.go:230` allows an absent
Origin), proving nothing about a browser.

**COST.** Awkward, and instructive. This is the fourth harness in a week to
produce a confident wrong answer, and the tell was the same every time: a
control that made no sense. Positive **and** negative controls, always.

### 3. "Test it on cellular" was justified wrongly — CORRECTION

**WHAT.** The plan told Patrik to verify from the iPad on cellular with wifi
off, on the grounds that same-network access "can succeed for reasons that will
not hold for a guest". He asked why, and the reason does not survive the
question.

The tunnel hostname is a public Cloudflare address. DNS resolves to their edge,
so traffic goes iPad → internet → Cloudflare → the Mac regardless of the iPad's
network. There is no local shortcut available to produce a false pass. The
argument would be sound for a LAN address such as `http://192.168.x.x:8080`,
and it was carried across to a case where it does not hold.

What IS true, and weaker: cellular exercises a different network path — mobile
latency, carrier NAT, possibly IPv6-only — which is closer to a remote guest's
conditions. Realism, not correctness.

**COST.** None to the result; entry 1's controls stand on their own and did not
depend on this. Recorded because a wrong reason in a plan is worse than no
reason: it gets followed, and later relied on.

### 4. Housekeeping from the probes

Four `OriginProbe*` agent participants were minted into `/tmp/vtt-session/session.db`
while testing. Harmless — session one starts on a fresh campaign per the plan —
but the campaign used for evidence must not be this one.

---

## Still to walk (plan Task 2)

- [ ] Seat the agent in the MCP host against the tunnel; confirm tools appear.
- [ ] DM loads an adventure and opens the door; record nudges needed.
- [ ] Join from the iPad as a guest would. **Time it**: link opened → seated.
- [ ] Handheld check: readable board? thumb-hittable buttons? participant list
      fits? anything overflowing horizontally?
- [ ] Promote + grant an actor; record how many nudges.
- [ ] Move a token from the iPad.
- [ ] Break the connection for ten seconds; reconnect. Did it say anything
      useful, or just look broken?
