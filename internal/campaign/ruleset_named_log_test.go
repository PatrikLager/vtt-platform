package campaign_test

import (
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

func named(id, version, name string) *vttv1.RulesetNamed {
	return &vttv1.RulesetNamed{RulesetId: id, Version: version, Name: name}
}

func plainRules(version string) *vttv1.RulesetNamed {
	return named("plain-rules", version, "Plain Rules")
}

// leadOf is the entries before the first one a world event caused: an
// opening's or a resume's.
func leadOf(entries []campaign.CharacterEntry) []campaign.CharacterEntry {
	end := 0
	for end < len(entries) && entries[end].WorldSeq == 0 {
		end++
	}
	return entries[:end]
}

func rulesetsNamedIn(entries []campaign.CharacterEntry) []campaign.CharacterEntry {
	var out []campaign.CharacterEntry
	for _, e := range entries {
		if e.Env.GetRulesetNamed() != nil {
			out = append(out, e)
		}
	}
	return out
}

// assertNothingAnEventGave requires an opening's or a resume's entry to carry
// no world sequence, event id, session, time or issuer (SPEC-021, VTT-272).
func assertNothingAnEventGave(t *testing.T, what string, e campaign.CharacterEntry) {
	t.Helper()
	env := e.Env
	if e.WorldSeq != 0 || env.GetEventId() != "" || env.GetSessionId() != "" || env.GetOccurredAt() != nil ||
		env.GetParticipantId() != "" || env.GetActorRole() != "" {
		t.Errorf("%s carries world sequence %d and %v, want nothing an event gave", what, e.WorldSeq, env)
	}
}

// VTT-981
func TestALogOpenedAfterTheWorldNamesARulesetOpensWithIt(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "Night one"}))
	must(t, c, cenv(nextID(), partyMember("bard")))
	must(t, c, cenv(nextID(), plainRules("1")))
	must(t, c, cenv(nextID(), plainRules("2")))
	must(t, c, cenv(nextID(), &vttv1.NoteUpserted{Key: "pub", Title: "Map", Text: "north",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}))
	joined := must(t, c, cenv(nextID(), partyMember("asme")))
	used := must(t, c, cenv(nextID(), &vttv1.AbilityUsed{ActorId: "asme", AbilityId: "qa-shove",
		TargetIds: []string{"asme"}}))

	got := logOf(t, c, "asme")
	want := "SessionStarted; RulesetNamed plain-rules 2; ActorAdded bard; NoteUpserted pub; " +
		"ActorAdded asme; AbilityUsed asme qa-shove"
	if j := strings.Join(kindsOf(got), "; "); j != want {
		t.Fatalf("asme's log = %s, want %s", j, want)
	}
	assertNothingAnEventGave(t, "the opening's RulesetNamed", got[1])
	if n := got[1].Env.GetRulesetNamed().GetName(); n != "Plain Rules" {
		t.Errorf("the opening's RulesetNamed names %q, want the world's name", n)
	}
	if got[4].WorldSeq != joined || got[5].WorldSeq != used {
		t.Errorf("the frames after the opening are of world events %d and %d, want %d and %d",
			got[4].WorldSeq, got[5].WorldSeq, joined, used)
	}
	held := c.CharacterFoldForTest("asme")
	if held.RulesetID != "plain-rules" || held.RulesetVersion != "2" || held.RulesetName != "Plain Rules" {
		t.Errorf("asme's log names %q %q %q, want the world's", held.RulesetID, held.RulesetVersion, held.RulesetName)
	}
	assertFoldsAtEveryPrefix(t, "asme's log", got)
	assertNoneDropped(t, c)
}

// VTT-980
func TestALogOpenBeforeTheWorldNamesARulesetIsSentTheWorlds(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "one"}))
	hallWithHero(t, c, 5)
	must(t, c, cenv(nextID(), partyMember("scout")))
	held := map[string]int{"hero": len(logOf(t, c, "hero")), "scout": len(logOf(t, c, "scout"))}
	entry := cenv(nextID(), plainRules("1"))
	entry.ParticipantId = "p-dm"
	seq := must(t, c, entry)
	used := must(t, c, cenv(nextID(), &vttv1.AbilityUsed{ActorId: "hero", AbilityId: "qa-shove",
		TargetIds: []string{"hero"}}))

	for _, id := range []string{"hero", "scout"} {
		got := logOf(t, c, id)
		if n := len(rulesetsNamedIn(got)); n != 1 {
			t.Errorf("%s's log holds %d RulesetNamed entries, want the world's one: %v", id, n, kindsOf(got))
		}
		after := got[held[id]:]
		if len(after) == 0 || describe(after[0].Env) != "RulesetNamed plain-rules 1" || after[0].WorldSeq != seq {
			t.Fatalf("%s's log does not hold the world's entry at world event %d: %v", id, seq, kindsOf(after))
		}
		env := after[0].Env
		if env.GetEventId() != entry.GetEventId() || env.GetRulesetNamed().GetName() != "Plain Rules" {
			t.Errorf("%s is sent %v, want the world's entry", id, env)
		}
		if env.GetParticipantId() != "" || env.GetActorRole() != "" {
			t.Errorf("%s is sent the entry with its issuer %q %q", id, env.GetParticipantId(), env.GetActorRole())
		}
		assertFoldsAtEveryPrefix(t, id+"'s log", got)
	}
	hero := logOf(t, c, "hero")[held["hero"]:]
	if len(hero) < 2 || describe(hero[1].Env) != "AbilityUsed hero qa-shove" || hero[1].WorldSeq != used {
		t.Errorf("the hero's outcome does not follow the entry: %v", kindsOf(hero))
	}
	assertNoneDropped(t, c)
}

// resumeAcross pauses the hero, whose log was sent each of before while it
// was active, names each of during while it is paused, and resumes it; it
// returns the hero's entries from the resume on.
func resumeAcross(t *testing.T, before, during []*vttv1.RulesetNamed) (*campaign.Campaign, []campaign.CharacterEntry) {
	t.Helper()
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "one"}))
	hallWithHero(t, c, 2)
	for _, rn := range before {
		must(t, c, cenv(nextID(), rn))
	}
	must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)))
	held := len(logOf(t, c, "hero"))
	must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "two"}))
	for _, rn := range during {
		must(t, c, cenv(nextID(), rn))
	}
	must(t, c, cenv(nextID(), grant("hero", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)))
	got := logOf(t, c, "hero")
	assertFoldsAtEveryPrefix(t, "the hero's log", got)
	assertNoneDropped(t, c)
	return c, got[held:]
}

// VTT-982
func TestAPausedLogResumesWithTheRulesetTheWorldNamedWhileItWasPaused(t *testing.T) {
	for _, tc := range []struct {
		name           string
		before, during []*vttv1.RulesetNamed
		want           *vttv1.RulesetNamed
	}{
		{"the log names none", nil, []*vttv1.RulesetNamed{plainRules("1")}, plainRules("1")},
		{"a new version", []*vttv1.RulesetNamed{plainRules("1")}, []*vttv1.RulesetNamed{plainRules("2")}, plainRules("2")},
		{"another id at the same version", []*vttv1.RulesetNamed{plainRules("1")},
			[]*vttv1.RulesetNamed{named("other-rules", "1", "Other Rules")}, named("other-rules", "1", "Other Rules")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, got := resumeAcross(t, tc.before, tc.during)
			lead := leadOf(got)
			want := "SessionEnded; SessionStarted; RulesetNamed " + tc.want.GetRulesetId() + " " + tc.want.GetVersion() +
				"; TokenHidden t-gob; TokenHidden t-hero"
			if j := strings.Join(kindsOf(lead), "; "); j != want {
				t.Fatalf("the resume wrote %s, want %s", j, want)
			}
			assertNothingAnEventGave(t, "the resume's RulesetNamed", lead[2])
			if n := lead[2].Env.GetRulesetNamed().GetName(); n != tc.want.GetName() {
				t.Errorf("the resume's RulesetNamed names %q, want %q", n, tc.want.GetName())
			}
			held := c.CharacterFoldForTest("hero")
			if held.RulesetID != tc.want.GetRulesetId() || held.RulesetVersion != tc.want.GetVersion() {
				t.Errorf("the resumed log names %q %q", held.RulesetID, held.RulesetVersion)
			}
		})
	}
}

// VTT-982
func TestAPausedLogWhoseRulesetIsTheWorldsResumesWithNone(t *testing.T) {
	for _, tc := range []struct {
		name           string
		before, during []*vttv1.RulesetNamed
	}{
		{"nothing named while paused", []*vttv1.RulesetNamed{plainRules("1")}, nil},
		{"another version and back", []*vttv1.RulesetNamed{plainRules("1")},
			[]*vttv1.RulesetNamed{plainRules("2"), plainRules("1")}},
		{"the same id and version under another name", []*vttv1.RulesetNamed{plainRules("1")},
			[]*vttv1.RulesetNamed{named("plain-rules", "1", "Plain Rules Revised")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := resumeAcross(t, tc.before, tc.during)
			lead := leadOf(got)
			want := "SessionEnded; SessionStarted; TokenHidden t-gob; TokenHidden t-hero"
			if j := strings.Join(kindsOf(lead), "; "); j != want {
				t.Fatalf("the resume wrote %s, want %s", j, want)
			}
			if extra := rulesetsNamedIn(got); len(extra) != 0 {
				t.Errorf("the resumed log was written %v", kindsOf(extra))
			}
		})
	}
}

// VTT-982 VTT-981
func TestAReopenedCampaignWritesNoSecondRulesetNamed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "one"}))
	hallWithHero(t, c, 2)
	must(t, c, cenv(nextID(), partyMember("bard")))
	must(t, c, cenv(nextID(), plainRules("1")))
	must(t, c, cenv(nextID(), grant("bard", vttv1.ActorKind_ACTOR_KIND_NON_PARTY)))
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	c = openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.NarrationAdded{Text: "after the reopen"}))
	must(t, c, cenv(nextID(), grant("bard", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)))
	must(t, c, cenv(nextID(), partyMember("asme")))
	for _, id := range []string{"hero", "bard", "asme"} {
		got := logOf(t, c, id)
		if n := len(rulesetsNamedIn(got)); n != 1 {
			t.Errorf("%s's log holds %d RulesetNamed entries, want one: %v", id, n, kindsOf(got))
		}
		assertFoldsAtEveryPrefix(t, id+"'s log", got)
	}
	if lead := leadOf(logOf(t, c, "asme")); len(lead) < 2 || describe(lead[1].Env) != "RulesetNamed plain-rules 1" {
		t.Errorf("asme's log, opened after the reopen, does not open with the world's ruleset: %v", kindsOf(lead))
	}
	assertNoneDropped(t, c)
}
