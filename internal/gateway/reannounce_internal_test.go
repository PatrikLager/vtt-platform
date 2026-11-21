package gateway

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// VTT-032 VTT-906
func TestAReannouncementIsNeverQueuedForARevokedWatcher(t *testing.T) {
	// Connections with no pump: nothing can close the revoked watcher's before
	// the fan-out, so its empty out channel is the deny set's own answer.
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = camp.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	s := New(camp, ids)
	mint := func(name string) string {
		_, id, err := ids.CreateInvite(name, identity.RoleSpectator)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	tam, wren, rook := mint("Tam"), mint("Wren"), mint("Rook")
	own, watcher, lost := conn(tam, "Tam", 4), conn(wren, "Wren", 4), conn(rook, "Rook", 4)
	for _, c := range []*presenceConn{own, watcher, lost} {
		join(s.presence, c)
	}
	if err := ids.Revoke(rook); err != nil {
		t.Fatal(err)
	}

	s.reannounce(tam)

	want := &vttv1.PresenceChanged{ParticipantId: tam, DisplayName: "Tam", State: vttv1.PresenceState_PRESENCE_STATE_CONNECTED}
	for _, c := range []*presenceConn{own, watcher} {
		if len(c.out) != 1 {
			t.Fatalf("VTT-906: %s's connection holds %d frames, want the one re-announcement", c.displayName, len(c.out))
		}
		var frame vttv1.ServerFrame
		if err := protojson.Unmarshal(<-c.out, &frame); err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(frame.GetPresenceChanged(), want) {
			t.Errorf("VTT-906: %s was queued %v, want %v", c.displayName, frame.GetPresenceChanged(), want)
		}
	}
	if len(lost.out) != 0 {
		t.Errorf("VTT-032: the revoked watcher was queued %d frames, want none", len(lost.out))
	}
}
