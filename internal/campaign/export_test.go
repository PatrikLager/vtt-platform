package campaign

import "github.com/PatrikLager/vtt-platform/internal/engine"

// DiscardedForTest is how many frame sets this Campaign dropped because they
// would not fold onto their log (VTT-320).
func (c *Campaign) DiscardedForTest() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.discarded
}

// CharacterFoldForTest is a copy of what this Campaign holds as actorID's
// log, nil for an actor with none (VTT-324).
func (c *Campaign) CharacterFoldForTest(actorID string) *engine.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l := c.logs[actorID]; l != nil {
		return l.fold.Snapshot()
	}
	return nil
}
