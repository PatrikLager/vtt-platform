# ADR-008: `vtt` CLI shell — adopt the ckeletin pattern, not the framework

**Status:** Accepted (2026-07-24)
**Context:** The `vtt` binary needs a command shell for `serve`, `invite`,
`revoke`, and `version` (spec §4.5, §6). Peiman's ckeletin-go project is a
full CLI scaffold: cobra + viper, plus an updateable `.ckeletin/` framework
layer that stays in sync with an upstream template and owns its own
discipline (quality gates, structure conventions). This project already has
its own quality gateway (Taskfile `task check`: vet, tests, drift, breaking,
vocabulary, arch-lint) and its own ADR trail. A second, externally-updated
layer asserting the same kind of discipline over the same codebase would
give the project two sources of truth for one concern — they will diverge,
and whichever one loses gets silently ignored.
**Decision:**

1. **Pattern adopted, framework rejected.** Use cobra for command parsing
   with ultra-thin commands: every `RunE` is ≤30 lines and does nothing but
   parse its own flags and delegate to `internal/gateway`, `internal/identity`,
   or `internal/campaign` — all logic lives there, none in `cmd/vtt`. The
   updateable `.ckeletin/` scaffold itself is NOT adopted: this project's own
   Taskfile, gates, and ADRs already own that discipline layer, and a second,
   template-synced copy of it would duel with them rather than reinforce them.
2. **Viper deferred.** No config file exists yet — `--campaign` and `--addr`
   flags fully cover `serve`/`invite`/`revoke` today. Adding a
   config-precedence library ahead of an actual config file is premature;
   revisit when a `vtt.yaml` (or equivalent) is introduced.
3. **Origin.** The pattern — thin commands over an auditable core, cobra
   without its config/scaffold baggage — is credited to Peiman's ckeletin-go,
   the same meta-pillar ("enforcement by automation") already adopted
   elsewhere in this project's discipline layer (spec §8).

**Consequences:** `cmd/vtt` holds one file per command
(`serve.go`/`invite.go`/`revoke.go`/`version.go`), each exporting a
`newXCmd() *cobra.Command` constructor assembled by `main.go`'s `newRootCmd()`
— the same function tests drive in-process via `SetArgs`/`SetOut`/`Execute`,
with no `exec` of the built binary. `RunE` length (≤30 lines) is enforced by
eyeball and a doc comment this round, not tooling. Arch-lint's `cmd`
component may depend on `gateway`, `identity`, `campaign`, `contract`, and
itself; no other component may depend on `cmd`. `go.mod` gains
`github.com/spf13/cobra`, pinned exact, promoted from an existing transitive
dependency to a direct one. `viper` is not a dependency at all, and stays
that way until a config file exists.
