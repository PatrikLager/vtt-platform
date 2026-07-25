package rules

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// CryptoRoller is the production Roller (Task 6 wiring): every individual
// die is drawn independently from crypto/rand, uniform over [1, sides].
// Dice are rolled exactly ONCE, at Resolve time (ruleset-interpreter spec
// §5 decision 3) — CryptoRoller has no seed and cannot be replayed; the
// individual per-die results Roll returns here are exactly what Resolve
// (resolve.go's evalRecording) records onto the AbilityUsed event's Rolls,
// and THAT recorded testimony — never a second call to Roll — is what any
// later replay of the log observes. Tests use the package's own
// deterministic test Rollers instead (never this type): CryptoRoller is
// wired in ONLY at the gateway's live call site (internal/gateway).
type CryptoRoller struct{}

// NewCryptoRoller returns a ready-to-use production Roller. The zero value
// (CryptoRoller{}) is equally usable — this constructor exists only for
// call-site symmetry with the rest of the package's exported API.
func NewCryptoRoller() *CryptoRoller { return &CryptoRoller{} }

// Roll draws n independent, uniform dice in [1, sides] via crypto/rand and
// returns them alongside their sum. By the time Eval (expr.go) ever calls
// this, n and sides are already bounded by the grammar's parse-time DICE
// limits (count 1..100, sides 1..1000 — expr.go's doc comment) — Roll does
// not re-validate them, but treats sides <= 0 as a degenerate always-1 die
// rather than dividing by zero, so a caller that somehow bypasses the
// parser still gets a harmless, deterministic answer instead of a crash.
//
// crypto/rand.Int failing at all is effectively unreachable on any real
// deployment target (it only happens if the OS's own entropy source fails
// to read) — Roll panics rather than silently return a wrong or zero
// result if it ever does, since the Roller interface (expr.go) has no
// error return through which a genuine failure could be propagated.
func (CryptoRoller) Roll(n, sides int) ([]int, int) {
	results := make([]int, n)
	total := 0
	for i := 0; i < n; i++ {
		v := 1
		if sides > 0 {
			roll, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
			if err != nil {
				panic(fmt.Sprintf("rules: CryptoRoller: crypto/rand failed: %v", err))
			}
			v = int(roll.Int64()) + 1
		}
		results[i] = v
		total += v
	}
	return results, total
}
