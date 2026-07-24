package campaign

import (
	"errors"
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestPoisonedCampaignRejectsAllOperations is a white-box test (package
// campaign, not campaign_test): the two paths that set poisoned are both
// post-persist and defensively unreachable in normal operation (Append
// validates on a snapshot before persisting; Undo dry-runs the full fold
// before persisting the marker), so there is no way to reach a poisoned
// Campaign through the public API alone. This test sets c.poisoned directly
// to exercise the guard on every method.
func TestPoisonedCampaignRejectsAllOperations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	c.poisoned = true

	env := &vttv1.Envelope{
		EventId:   "e1",
		SessionId: "sess-1",
		ActorRole: "dm",
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "n"},
		},
	}
	if _, err := c.Append(env); !errors.Is(err, errPoisoned) {
		t.Fatalf("Append on poisoned Campaign: got %v, want errPoisoned", err)
	}

	if err := c.Undo(1, 1, "reason", "e2", "dm", "test-participant"); !errors.Is(err, errPoisoned) {
		t.Fatalf("Undo on poisoned Campaign: got %v, want errPoisoned", err)
	}

	if st := c.State(); st != nil {
		t.Fatalf("State on poisoned Campaign: got %+v, want nil", st)
	}

	if _, _, err := c.Subscribe(0, 4); !errors.Is(err, errPoisoned) {
		t.Fatalf("Subscribe on poisoned Campaign: got %v, want errPoisoned", err)
	}
}
