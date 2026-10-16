package campaign_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

func subscribeLog(t *testing.T, c *campaign.Campaign, actorID string, after int64) (<-chan *vttv1.Envelope, int64) {
	t.Helper()
	ch, unsubscribe, head, err := c.SubscribeCharacter(actorID, after, 0, 0)
	if err != nil {
		t.Fatalf("SubscribeCharacter(%q, %d): %v", actorID, after, err)
	}
	t.Cleanup(unsubscribe)
	return ch, head
}

func received(t *testing.T, ch <-chan *vttv1.Envelope, n int) []*vttv1.Envelope {
	t.Helper()
	out := make([]*vttv1.Envelope, 0, n)
	for len(out) < n {
		select {
		case env, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed after %d of %d entries", len(out), n)
			}
			out = append(out, env)
		case <-time.After(2 * time.Second):
			t.Fatalf("received %d of %d entries", len(out), n)
		}
	}
	return out
}

func assertSilent(t *testing.T, name string, ch <-chan *vttv1.Envelope) {
	t.Helper()
	select {
	case env, ok := <-ch:
		if !ok {
			t.Fatalf("%s: channel closed, want it open and silent", name)
		}
		t.Fatalf("%s: received %s, want nothing", name, describe(env))
	case <-time.After(200 * time.Millisecond):
	}
}

func assertEnded(t *testing.T, name string, ch <-chan *vttv1.Envelope) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatalf("%s: channel still open", name)
		}
	}
}

func assertDelivered(t *testing.T, got []*vttv1.Envelope, want []campaign.CharacterEntry) {
	t.Helper()
	for i := range want {
		if !proto.Equal(got[i], want[i].Env) {
			t.Fatalf("delivered entry %d = %s at %d, the log holds %s at %d", i,
				describe(got[i]), got[i].GetSequence(), describe(want[i].Env), want[i].Env.GetSequence())
		}
	}
}

// VTT-329
func TestACharacterSubscriptionCarriesWhatTheFanOutWrites(t *testing.T) {
	c := openAt(t, filepath.Join(t.TempDir(), "campaign"))
	hallWithHero(t, c, 5)
	before := logOf(t, c, "hero")
	if len(before) < 3 {
		t.Fatalf("the hero's log holds %d entries; the fixture needs a catch-up after cursor 2", len(before))
	}

	ch, head := subscribeLog(t, c, "hero", 2)
	if head != int64(len(before)) {
		t.Fatalf("catch-up head = %d, want %d", head, len(before))
	}
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "the door creaks"}))
	if _, err := c.AppendBatch([]*vttv1.Envelope{
		cenv(nextID(), &vttv1.NarrationAdded{Text: "aside"}),
		cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(5, 1), To: at(2, 1)}),
	}); err != nil {
		t.Fatal(err)
	}

	want := logOf(t, c, "hero")[2:]
	if !hasKind(want, "ActorAdded goblin") || !hasKind(want, "TokenPlaced t-gob") {
		t.Fatalf("the move did not bring the goblin into sight; the fixture proves nothing: %v", kindsOf(want))
	}
	assertDelivered(t, received(t, ch, len(want)), want)
	assertSilent(t, "hero", ch)
}

// VTT-332
func TestAFailedAppendNotifiesNoCharacter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	hallWithHero(t, c, 5)
	head := int64(len(logOf(t, c, "hero")))
	ch, _ := subscribeLog(t, c, "hero", head)

	raw := rawLog(t, dir)
	execSQL(t, raw, `CREATE TRIGGER refuse_entries BEFORE INSERT ON character_events
		BEGIN SELECT RAISE(ABORT, 'refused by the test'); END`)
	move := func() *vttv1.Envelope {
		return cenv(nextID(), &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", From: at(5, 1), To: at(2, 1)})
	}
	if _, err := c.Append(move()); err == nil || !strings.Contains(err.Error(), "refused by the test") {
		t.Fatalf("the move's append = %v, want the trigger's refusal", err)
	}
	assertSilent(t, "the hero after a failed append", ch)

	execSQL(t, raw, `DROP TRIGGER refuse_entries`)
	must(t, c, move())
	want := logOf(t, c, "hero")[head:]
	if len(want) == 0 || want[0].Env.GetSequence() != head+1 {
		t.Fatalf("the move appended again wrote %v; want entries from %d", kindsOf(want), head+1)
	}
	assertDelivered(t, received(t, ch, len(want)), want)
	assertSilent(t, "the hero", ch)
}

// VTT-335
func TestClosingTheCampaignEndsCharacterSubscriptions(t *testing.T) {
	c := openAt(t, filepath.Join(t.TempDir(), "campaign"))
	hallWithHero(t, c, 5)
	hero, _ := subscribeLog(t, c, "hero", 0)
	nobody, _ := subscribeLog(t, c, "nobody", 0)

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	assertEnded(t, "hero", hero)
	assertEnded(t, "nobody", nobody)
}
