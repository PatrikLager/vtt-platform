package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Refuse UNSPECIFIED and nothing else, and never wrap this in ErrUnauthorized:
// a missing field is not a missing permission (SPEC-013).
func validateUpsertNote(cmd *vttv1.UpsertNote) error {
	if cmd.GetVisibility() == vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED {
		return fmt.Errorf("gateway: upsert_note: visibility — a note must say who may read it "+
			"(%s for every player and spectator, %s for the DM and the agent alone); "+
			"an unstated visibility cannot be told from a deliberate one, so it is refused rather than guessed",
			vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC, vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET)
	}
	return nil
}
