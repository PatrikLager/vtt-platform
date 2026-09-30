package gateway

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-231
func TestAnUpsertNoteWithNoVisibilityIsRefused(t *testing.T) {
	cmd := &vttv1.UpsertNote{Key: "kobold-den", Text: "Three kobolds guard the east tunnel."}
	if v := cmd.GetVisibility(); v != vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED {
		t.Fatalf("fixture check: this command must state NO visibility, got %v", v)
	}
	err := validateUpsertNote(cmd)
	if err == nil {
		t.Fatal("an upsert_note that does not say who may read it must be refused, not defaulted")
	}
	if !strings.Contains(err.Error(), "visibility") {
		t.Errorf("refusal %q never names the field that is missing", err)
	}
}

// VTT-231
func TestEveryNoteVisibilityTheContractOffersIsAccepted(t *testing.T) {
	values := vttv1.NoteVisibility(0).Descriptor().Values()
	if values.Len() < 3 {
		t.Fatalf("fixture check: NoteVisibility should offer UNSPECIFIED and at least two real "+
			"values, got %d — this loop would otherwise assert nothing", values.Len())
	}
	for i := range values.Len() {
		v := values.Get(i)
		vis := vttv1.NoteVisibility(v.Number())
		if vis == vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED {
			continue
		}
		t.Run(string(v.Name()), func(t *testing.T) {
			cmd := &vttv1.UpsertNote{Key: "kobold-den", Text: "Three kobolds.", Visibility: vis}
			if err := validateUpsertNote(cmd); err != nil {
				t.Fatalf("the contract offers %v and an upsert_note stating it was refused: %v", vis, err)
			}
		})
	}
}
