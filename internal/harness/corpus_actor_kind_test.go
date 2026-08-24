package harness_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence holds the whole
// scenario corpus — definitions and goldens alike — to visibility spec §5.1.
//
// WHAT IT DEFENDS, which is not the same as what it checks. §5.1's model is
// that CONTROL IS CONFERRED ONCE, BY A GRANT THAT DECLARES KIND, and that an
// actor says what it is at birth. Three rules follow and this gate holds the
// corpus to all three:
//
//   - an actor is never created holding a controller. Creation makes a
//     character; a grant hands it over. (internal/gateway's validateAddActor
//     refuses the other shape, and engine.Apply refuses it again in the fold.)
//   - every created actor states a kind. An unstated kind cannot be told from a
//     deliberate one, so it is refused rather than guessed.
//   - every grant states a kind, for the same reason — and standing is exactly
//     what separates the DM assigning a character from an agent taking a
//     monster to run, which is the ambiguity the original leak lived in.
//
// WHY THE CORPUS SPECIFICALLY. The fixtures are the only place the old model
// can still survive. No campaign and no ruleset is in use by anyone, which is
// why this arc could delete its migration rule outright — so the corpus IS the
// entire "existing history" there is, and a fixture is the last place anyone
// looks for a design regression. Authored adventure content is the one other
// candidate and it is already covered at its own door: internal/adventure's
// loader refuses an actor whose kind is empty or unrecognised.
//
// WHY A GATE RATHER THAN JUST THE CONVERSION THAT LANDED WITH IT. The two fail
// differently. Converting the fixtures wrongly is a red suite; this gate not
// EXISTING is silent, and stays silent until someone writes a fixture in the
// old shape next month and nothing objects.
//
// IF YOU ARRIVED HERE BECAUSE A NEW FIXTURE FAILS: the fixture is wrong, not
// this gate. Add the actor with no controller and a stated kind, then grant it,
// saying what it is. If you believe §5.1 itself has changed, change §5.1 first
// — deleting this test to make a fixture pass is how four tasks of reasoning
// leak back in through test data.
func TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence(t *testing.T) {
	audit, err := auditActorKind("../../scenarios")
	if err != nil {
		t.Fatal(err)
	}
	for _, why := range corpusFailures(audit) {
		t.Error(why)
	}
	t.Logf("held %s across %d corpus files to spec §5.1, plus %d occurrence(s) carrying the old "+
		"shape that a step pins as REFUSED rather than as history",
		audit.tally(), audit.Files, audit.Refusals)
}

// TestAnActorCreatedHoldingAControllerIsCaught is fault injection for the first
// of the three rules: it is the exact shape the corpus carried before this arc
// converted it.
//
// MEASURED, at 18b7212 (the commit before Task 4's conversion): 12 such events
// across 7 golden streams. That is not the figure the plan carries — it says 8
// across 5 — and the difference is the whole of session-zero's two PROJECTION
// streams, 4 events the plan's count walked straight past. Which is the case
// for matching by key at any depth rather than by file: the number that missed
// the projections was produced by a human doing this by hand once.
func TestAnActorCreatedHoldingAControllerIsCaught(t *testing.T) {
	corpus := cleanCorpus()
	requireClean(t, corpus)

	corpus[goldenStreamFile] = `[
	  {"eventId":"evt-1","sequence":"1","actorAdded":{"actor":
	    {"actorId":"act-a","name":"A","kind":"ACTOR_KIND_PARTY_MEMBER","controllerId":"p-1"}}},
	  {"eventId":"evt-2","sequence":"2","actorControlGranted":
	    {"actorId":"act-a","participantId":"p-1","kind":"ACTOR_KIND_PARTY_MEMBER"}}
	]`
	requireOneFailureAbout(t, corpus, goldenStreamFile, "created holding a controller")
}

// TestAnActorCreatedStatingNoKindIsCaught is fault injection for the second
// rule — the one Task 7 added, and the one an older fixture satisfies by
// accident because kind simply did not exist when it was written.
func TestAnActorCreatedStatingNoKindIsCaught(t *testing.T) {
	corpus := cleanCorpus()
	requireClean(t, corpus)

	corpus[goldenStreamFile] = `[
	  {"eventId":"evt-1","sequence":"1","actorAdded":{"actor":{"actorId":"act-a","name":"A"}}},
	  {"eventId":"evt-2","sequence":"2","actorControlGranted":
	    {"actorId":"act-a","participantId":"p-1","kind":"ACTOR_KIND_PARTY_MEMBER"}}
	]`
	requireOneFailureAbout(t, corpus, goldenStreamFile, "states no kind")
}

// TestAGrantThatStatesNoKindIsCaught is fault injection for the third rule. A
// grant is where standing is conferred, so silence here is the original leak:
// it is what made the DM assigning a character and an agent taking a monster
// byte-identical.
func TestAGrantThatStatesNoKindIsCaught(t *testing.T) {
	corpus := cleanCorpus()
	requireClean(t, corpus)

	corpus[goldenStreamFile] = `[
	  {"eventId":"evt-1","sequence":"1","actorAdded":{"actor":
	    {"actorId":"act-a","name":"A","kind":"ACTOR_KIND_PARTY_MEMBER"}}},
	  {"eventId":"evt-2","sequence":"2","actorControlGranted":{"actorId":"act-a","participantId":"p-1"}}
	]`
	requireOneFailureAbout(t, corpus, goldenStreamFile, "grant states no kind")
}

// TestTheOldShapeIsAllowedOnlyWhereTheStepPinsThatVeryRefusal is the derivation
// itself, stated as the DIFFERENCE between three corpora rather than as the
// absence of a complaint about one.
//
// scenarios/denials.json deliberately sends an add_actor carrying a controller
// and a grant stating no kind, and asserts the server refuses both. That
// fixture defends §5.1; it does not break it, because a refused command never
// becomes history and history is what this gate is about. So the obligation is
// derived from the step's own expectation rather than from a list of files to
// skip.
//
// THE THIRD CORPUS IS THE ONE THAT MATTERS, and an earlier draft of this gate
// got it wrong. Asking only "is this step denied?" lets a step denied for some
// OTHER reason launder the old shape — and that is reachable rather than
// theoretical, because gateway.handleCommand runs Authorize (server.go:958)
// well before it reaches validateAddActor (:1152). A player's add_actor is
// refused as unauthorized whatever else is wrong with it, so denials.json's
// two authz steps are ready-made carriers: the corpus would carry a committed
// add_actor seeding a controller, green, as the template the next author
// copies.
func TestTheOldShapeIsAllowedOnlyWhereTheStepPinsThatVeryRefusal(t *testing.T) {
	// A SECOND scenario file rather than a replacement, because that is the
	// corpus's own shape: denials.json sits BESIDE the scenarios that work. A
	// corpus whose only add_actor is a pinned refusal has judged nothing and
	// fails the vacuity check instead — correctly, and measured, because the
	// first draft of this test replaced the file and hit exactly that.
	pinned := cleanCorpus()
	pinned[denialsFile] = scenarioSendingTheOldShape(
		`{"deniedContaining": "control is conferred by grant_actor_control"}`,
		`{"deniedContaining": "must say what the actor IS"}`)
	if failures := failuresOver(t, pinned); len(failures) != 0 {
		t.Errorf("a step that asserts THIS shape's refusal is defending the rule, not breaking it, "+
			"and must not fail this gate; got %v", failures)
	}

	claimed := cleanCorpus()
	claimed[denialsFile] = scenarioSendingTheOldShape(`{"ok": true}`, `{"ok": true}`)
	failures := failuresOver(t, claimed)
	if !anyMentions(failures, "created holding a controller") || !anyMentions(failures, "grant states no kind") {
		t.Errorf("the same commands expected to SUCCEED are the fixture this gate exists to catch; "+
			"got %v", failures)
	}

	elsewhere := cleanCorpus()
	elsewhere[denialsFile] = scenarioSendingTheOldShape(
		`{"deniedContaining": "not authorized"}`, `{"deniedContaining": "not authorized"}`)
	failures = failuresOver(t, elsewhere)
	if !anyMentions(failures, "created holding a controller") || !anyMentions(failures, "grant states no kind") {
		t.Errorf("a step refused for an unrelated reason still COMMITS the old shape to the corpus "+
			"and must not be exempted by it; got %v", failures)
	}
}

// TestAVagueDenialClaimPinsNothing closes the evasion that is this gate's own
// failure mode, one level up: being got round by being VAGUE.
//
// Every refusal below contains the word "actor", and so does the authz denial a
// player earns for add_actor (`role "player" may not issue "add_actor"`,
// internal/gateway/authz.go). So a step claiming `"deniedContaining": "actor"`
// passes the harness — the denial really does contain it — while matching every
// refusal this gate knows, which under a match-any rule pins all of them and
// reports nothing.
//
// NOBODY HAS TO LIE TO DO THAT; they only have to be lazy, and an
// under-specific claim is far likelier than a forged one. A guard should assume
// the lazy case. So a claim pins a refusal only when it names exactly ONE — an
// ambiguous claim identifies nothing and pins nothing.
func TestAVagueDenialClaimPinsNothing(t *testing.T) {
	corpus := cleanCorpus()
	corpus[denialsFile] = scenarioSendingTheOldShape(
		`{"deniedContaining": "actor"}`, `{"deniedContaining": "actor"}`)

	failures := failuresOver(t, corpus)
	if !anyMentions(failures, "created holding a controller") {
		t.Errorf("a claim vague enough to name every refusal names none of them, and must not "+
			"pin a seeded controller; got %v", failures)
	}
	if !anyMentions(failures, "grant states no kind") {
		t.Errorf("a claim vague enough to name every refusal names none of them, and must not "+
			"pin a kindless grant; got %v", failures)
	}
}

// TestAnEmptyCorpusIsAFailureRatherThanAPass is the first of the two vacuity
// checks. A walker that visits no files reports success having checked nothing.
func TestAnEmptyCorpusIsAFailureRatherThanAPass(t *testing.T) {
	failures := failuresOver(t, map[string]string{})
	if !anyMentions(failures, "no corpus files") {
		t.Errorf("an empty corpus must fail rather than pass over nothing; got %v", failures)
	}
}

// TestACorpusMissingAnyOneOfTheFourShapesIsAFailureRatherThanAPass is the
// second and sharper vacuity check: files that parse, and one whole shape
// unexercised among them.
//
// This is the defect one level up from an empty corpus, and this arc has
// shipped it twice — TestARapidHopIsCoalescedToTheShoulderItEndedOn passed
// against the first-wins box it was written to reject, and
// TestTheVisibleSetIsSentInAStableOrder passed against its own stub because
// sort.StringsAreSorted(nil) is true. Both were nil-or-empty cases.
//
// PER SHAPE, NOT POOLED, which is the same distinction Task 8's projected
// gate had to make ("that guard catches the corpus EMPTYING; it does not catch
// a new golden diluting it"). A pooled counter is propped up by whichever
// shape is most numerous: the corpus holds 19 addActor COMMANDS, so pooling
// them with events would keep the count far from zero while every recorded
// stream — the history half, which is the half §5.1 is about — went unchecked.
func TestACorpusMissingAnyOneOfTheFourShapesIsAFailureRatherThanAPass(t *testing.T) {
	for _, shape := range corpusShapes {
		t.Run(shape.Name, func(t *testing.T) {
			corpus := cleanCorpus()
			for file, body := range corpus {
				corpus[file] = renameShapeAway(body, shape)
			}
			failures := failuresOver(t, corpus)
			if !anyMentions(failures, shape.Absent) {
				t.Errorf("a corpus containing no %s must fail rather than report success over "+
					"nothing; got %v", shape.Name, failures)
			}
		})
	}
}

// TestTheOldShapeIsCaughtInEitherWireSpelling closes the evasion this arc has
// already paid for once: protojson accepts both the proto field name and its
// lowerCamel JSON name, and a grep for the snake_case form alone found
// cmd/vtt/tools.json while missing the entire client.
func TestTheOldShapeIsCaughtInEitherWireSpelling(t *testing.T) {
	corpus := cleanCorpus()
	requireClean(t, corpus)

	corpus[goldenStreamFile] = `[
	  {"eventId":"evt-1","sequence":"1","actor_added":{"actor":
	    {"actorId":"act-a","name":"A","kind":"ACTOR_KIND_PARTY_MEMBER","controller_ids":["p-1"]}}},
	  {"eventId":"evt-2","sequence":"2","actor_control_granted":{"actorId":"act-a","participantId":"p-1"}}
	]`
	failures := failuresOver(t, corpus)
	if !anyMentions(failures, "created holding a controller") {
		t.Errorf("a snake_case actor_added carrying controller_ids must be caught; got %v", failures)
	}
	if !anyMentions(failures, "grant states no kind") {
		t.Errorf("a snake_case actor_control_granted stating no kind must be caught; got %v", failures)
	}
}

// TestAKindIsStatedOnlyByAValueTheContractDefines pins what "states a kind"
// means. The zero value is the whole failure mode — an omitted enum arrives as
// UNSPECIFIED and is indistinguishable from a caller who deliberately said
// nothing — and a value the contract has never heard of is not a statement
// either, however deliberate it looks.
func TestAKindIsStatedOnlyByAValueTheContractDefines(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		stated bool
	}{
		{`"ACTOR_KIND_PARTY_MEMBER"`, true},
		{`"ACTOR_KIND_NON_PARTY"`, true},
		{`2`, true}, // protojson emits the name and accepts the number.
		{`"ACTOR_KIND_UNSPECIFIED"`, false},
		{`0`, false},
		{`"PARTY_MEMBER"`, false}, // not a name the contract defines.
		{`"party_member"`, false},
		{`null`, false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			corpus := cleanCorpus()
			corpus[goldenStreamFile] = fmt.Sprintf(`[
			  {"eventId":"evt-1","sequence":"1","actorAdded":{"actor":
			    {"actorId":"act-a","name":"A","kind":%s}}},
			  {"eventId":"evt-2","sequence":"2","actorControlGranted":
			    {"actorId":"act-a","participantId":"p-1","kind":%s}}
			]`, tc.kind, tc.kind)

			// Two ifs rather than a switch, and the creation side names its own
			// message rather than the substring both share. "states no kind"
			// appears in the GRANT's message too, so asserting it proved nothing
			// about creation: MEASURED by deleting creationProblems' kind branch,
			// after which every !stated case still passed. That would have been
			// this arc's THIRD assertion satisfied by something other than what
			// it meant to test.
			failures := failuresOver(t, corpus)
			if tc.stated && len(failures) != 0 {
				t.Errorf("kind %s is one the contract defines and must count as stated; got %v",
					tc.kind, failures)
			}
			if !tc.stated && !anyMentions(failures, "created actor states no kind") {
				t.Errorf("kind %s does not name a kind the contract defines, so the created actor "+
					"states nothing and must be caught; got %v", tc.kind, failures)
			}
			if !tc.stated && !anyMentions(failures, "grant states no kind") {
				t.Errorf("kind %s does not name a kind the contract defines, so the grant states "+
					"nothing and must be caught; got %v", tc.kind, failures)
			}
		})
	}
}

// The two files every synthesized corpus below carries, one of each kind the
// real corpus holds: a scenario definition and a recorded golden stream.
// denialsFile is the third, added only by the tests that need a fixture sending
// a shape the platform refuses — as scenarios/denials.json does for real.
const (
	scenarioFile     = "one.json"
	goldenStreamFile = "goldens/one/stream.json"
	denialsFile      = "denials.json"
)

// cleanCorpus is the smallest corpus that satisfies all three rules and
// exercises all four shapes. Every injection test starts from it and changes
// ONE thing, so what the gate reports is attributable to that change.
func cleanCorpus() map[string]string {
	return map[string]string{
		scenarioFile: `{"name":"one","participants":[{"name":"dm","role":"dm"}],"steps":[
		  {"by":"dm","command":{"addActor":{"actor":
		    {"actorId":"act-a","name":"A","kind":"ACTOR_KIND_NON_PARTY"}}},"expect":{"ok":true}},
		  {"by":"dm","command":{"grantActorControl":
		    {"actorId":"act-a","participantId":"{{id:dm}}","kind":"ACTOR_KIND_NON_PARTY"}},
		   "expect":{"ok":true}}
		]}`,
		goldenStreamFile: `[
		  {"eventId":"evt-1","sequence":"1","actorAdded":{"actor":
		    {"actorId":"act-a","name":"A","kind":"ACTOR_KIND_PARTY_MEMBER"}}},
		  {"eventId":"evt-2","sequence":"2","actorControlGranted":
		    {"actorId":"act-a","participantId":"p-1","kind":"ACTOR_KIND_PARTY_MEMBER"}}
		]`,
	}
}

// renameShapeAway rewrites one shape's every spelling to a name this gate does
// not match, which is what a contract rename would do to the corpus.
func renameShapeAway(body string, shape *corpusShape) string {
	for _, spelling := range shape.Spellings {
		body = strings.ReplaceAll(body, `"`+spelling+`"`, `"renamedAway"`)
	}
	return body
}

// scenarioSendingTheOldShape is one scenario whose add_actor seeds a controller
// and whose grant says nothing, under whatever expectations the caller passes.
// The expectations are the ONLY variable, which is what makes the three corpora
// in TestTheOldShapeIsAllowedOnlyWhereTheStepPinsThatVeryRefusal a measurement.
func scenarioSendingTheOldShape(onCreate, onGrant string) string {
	return fmt.Sprintf(`{"name":"old","participants":[{"name":"dm","role":"dm"}],"steps":[
	  {"by":"dm","command":{"addActor":{"actor":
	    {"actorId":"act-a","name":"A","kind":"ACTOR_KIND_NON_PARTY","controllerId":"p-1"}}},
	   "expect":%s},
	  {"by":"dm","command":{"grantActorControl":{"actorId":"act-a","participantId":"p-1"}},
	   "expect":%s}
	]}`, onCreate, onGrant)
}

// requireClean fails the test outright if the baseline corpus does not pass,
// because an injection measured against an already-failing baseline measures
// nothing.
func requireClean(t *testing.T, corpus map[string]string) {
	t.Helper()
	if failures := failuresOver(t, corpus); len(failures) != 0 {
		t.Fatalf("the baseline corpus must pass before anything is injected into it: %v", failures)
	}
}

// requireOneFailureAbout asserts the corpus fails exactly once, naming the file
// and the rule. Exactly once, not at least once: a finding that fires twice for
// one injected fault would still read as green to an at-least assertion while
// meaning the walk visits the same bytes twice.
func requireOneFailureAbout(t *testing.T, corpus map[string]string, file, about string) {
	t.Helper()
	failures := failuresOver(t, corpus)
	if len(failures) != 1 {
		t.Fatalf("want exactly one failure about %q, got %d: %v", about, len(failures), failures)
	}
	if !strings.Contains(failures[0], about) {
		t.Errorf("the failure must say which rule was broken (%q); got %q", about, failures[0])
	}
	if !strings.Contains(failures[0], file) {
		t.Errorf("the failure must name the file that broke it (%q); got %q", file, failures[0])
	}
}

// failuresOver writes a corpus to a temporary directory and audits it.
func failuresOver(t *testing.T, files map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	audit, err := auditActorKind(root)
	if err != nil {
		t.Fatal(err)
	}
	return corpusFailures(audit)
}

// anyMentions reports whether any failure contains want.
func anyMentions(failures []string, want string) bool {
	for _, f := range failures {
		if strings.Contains(f, want) {
			return true
		}
	}
	return false
}

// actorKindProblem is one broken rule, paired with the refusal the platform
// gives for that exact shape.
type actorKindProblem struct {
	// Why names the rule and its reason, because a gate that says only what it
	// checks invites deletion by whoever wants the next fixture to pass.
	Why string
	// Refusal is internal/gateway's own wording when it turns this shape away.
	// A scenario step may carry the shape ONLY by claiming a refusal that is
	// part of this string — see hold.
	Refusal string
}

// actorKindFinding is one place in the corpus that breaks spec §5.1.
type actorKindFinding struct {
	File string
	Path string
	Why  string
}

// String names the file, the path within it, and the rule — in that order,
// because the first question a reader has is which fixture to open.
func (f actorKindFinding) String() string {
	return fmt.Sprintf("%s at %s: %s", f.File, f.Path, f.Why)
}

// actorKindAudit is one pass over a corpus root: what it found wrong, and how
// much it actually looked at.
type actorKindAudit struct {
	Findings []actorKindFinding
	// Files is every .json under the root, whatever its kind.
	Files int
	// Held counts, PER SHAPE NAME, the occurrences this gate actually judged.
	// An occurrence whose every problem a step pins as refused was not judged
	// and is counted under Refusals instead.
	Held     map[string]int
	Refusals int
}

// tally renders the per-shape counts in a stable order, for the gate's log.
func (a actorKindAudit) tally() string {
	parts := make([]string, 0, len(corpusShapes))
	for _, shape := range corpusShapes {
		parts = append(parts, fmt.Sprintf("%d %s", a.Held[shape.Name], shape.Name))
	}
	return strings.Join(parts, ", ")
}

// corpusFailures is every reason a corpus fails spec §5.1 — including the
// reasons that are about the audit rather than about any one fixture.
//
// THE VACUITY CHECKS ARE NOT PADDING. A pass that walked no files, or walked
// files and matched no occurrence of some shape, is a guard reporting success
// having checked nothing. That is the same defect the fixtures themselves would
// have, one level up.
//
// They are also what keeps the literal wire spellings in corpusShapes safe
// rather than brittle. Rename ActorAdded in the contract and the corpus
// regenerates under the new name; that shape's counter falls to zero and the
// gate says so, instead of matching nothing forever while staying green.
func corpusFailures(a actorKindAudit) []string {
	var out []string
	for _, f := range a.Findings {
		out = append(out, f.String())
	}
	if a.Files == 0 {
		out = append(out, "no corpus files were walked at all, so this gate checked nothing "+
			"(spec §5.1)")
	}
	for _, shape := range corpusShapes {
		if a.Held[shape.Name] == 0 {
			out = append(out, shape.Absent)
		}
	}
	return out
}

// corpusShape is one wire message this gate matches: every spelling protojson
// accepts for it, the rules that govern it, and what it means for the corpus to
// contain none of it.
//
// FOUR SHAPES AND NOT TWO, because the event and the command that carry the
// same payload are renamed independently and blind the gate to different
// halves. The events are the recorded history §5.1 is actually about.
//
// IT IS LOUD ON RENAMES AND SILENT ON ADDITIONS, and only the first half is
// defended. A fifth writer of standing arriving in the contract would simply
// never be matched here while all four counters stayed comfortably above zero.
// Nothing detects that but a person adding it to this table.
type corpusShape struct {
	Name string
	// Spellings is the lowerCamel JSON name and the proto field name. protojson
	// accepts either, so a sweep for one alone would miss the first fixture
	// hand-written in the other — and this arc has already had a grep miss an
	// entire client for exactly that reason.
	Spellings []string
	Problems  func(payload any) []actorKindProblem
	Absent    string
}

// corpusShapes are the two messages that create an actor and the two that
// confer control, each in both spellings.
var corpusShapes = []*corpusShape{
	{
		Name:      "ActorAdded event",
		Spellings: []string{"actorAdded", "actor_added"},
		Problems:  creationProblems,
		Absent: "no ActorAdded event anywhere in the corpus was held to spec §5.1 — either the " +
			"golden streams stopped creating actors, or this event's wire name has changed and " +
			"the gate is now matching nothing in the recorded history it exists to guard",
	},
	{
		Name:      "add_actor command",
		Spellings: []string{"addActor", "add_actor"},
		Problems:  creationProblems,
		Absent: "no add_actor command anywhere in the corpus was held to spec §5.1 — either the " +
			"scenarios stopped creating actors, or this command's wire name has changed and the " +
			"gate is now matching nothing",
	},
	{
		Name:      "ActorControlGranted event",
		Spellings: []string{"actorControlGranted", "actor_control_granted"},
		Problems:  grantProblems,
		Absent: "no ActorControlGranted event anywhere in the corpus was held to spec §5.1 — " +
			"either the golden streams stopped granting control, or this event's wire name has " +
			"changed and the gate is now matching nothing in the recorded history it exists to " +
			"guard",
	},
	{
		Name:      "grant_actor_control command",
		Spellings: []string{"grantActorControl", "grant_actor_control"},
		Problems:  grantProblems,
		Absent: "no grant_actor_control command anywhere in the corpus was held to spec §5.1 — " +
			"either the scenarios stopped granting control, or this command's wire name has " +
			"changed and the gate is now matching nothing",
	},
}

// shapesByKey indexes corpusShapes by every spelling of every shape.
var shapesByKey = func() map[string]*corpusShape {
	out := map[string]*corpusShape{}
	for _, shape := range corpusShapes {
		for _, spelling := range shape.Spellings {
			out[spelling] = shape
		}
	}
	return out
}()

// auditActorKind walks every .json file under root and holds every actor
// creation and every grant it finds to spec §5.1.
//
// IT WALKS BY KEY, NOT BY FILE KIND, and that is the design rather than a
// shortcut. The corpus holds four kinds of JSON — scenario definitions,
// recorded event streams, hand-derived states, seat declarations — and a fifth
// arriving next month is exactly what an audit that switched on filename would
// skip in silence. Matching the key wherever it appears covers projections/, a
// nesting level nobody has invented yet, and any new .json file, and it needs
// no list of files the rule applies to. It also catches what a hand count
// misses: the plan's own pre-conversion measurement walked past session-zero's
// projection streams and undercounted by a third.
//
// THERE IS NO EXEMPTION LIST, deliberately. Task 8 of the visibility arc is the
// precedent: its projected-fixture gate asks "does this golden hide a
// creature?" and requires projections only where the answer is yes, so it has
// nothing to maintain and nothing to rot. A list of fixtures exempt from this
// rule is the artifact that goes stale silently, which is the very failure this
// gate exists to prevent.
func auditActorKind(root string) (actorKindAudit, error) {
	audit := actorKindAudit{Held: map[string]int{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// EqualFold, because a .JSON fixture is still a fixture and skipping it
		// would be this gate's own silent exemption.
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		// #nosec G304 -- path is not external input: it is a path this very
		// WalkDir produced, under a root the caller named. Every caller is a
		// test in this file, passing either the committed corpus or its own
		// t.TempDir().
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		audit.Files++
		audit.walk(filepath.ToSlash(rel), "", doc, "")
		return nil
	})
	return audit, err
}

// walk descends one decoded document, holding every creation and grant it
// passes to the rules.
//
// denied travels DOWN rather than being looked up: a step's expectation sits
// beside its command, not inside it, so the only place both are visible is the
// step object itself on the way past.
func (a *actorKindAudit) walk(file, path string, node any, denied string) {
	switch n := node.(type) {
	case map[string]any:
		if claim := deniedClaimOf(n); claim != "" {
			denied = claim
		}
		for _, key := range jsonKeys(n) {
			child, at := n[key], path+"/"+key
			if shape, ok := shapesByKey[key]; ok {
				a.hold(file, at, shape, shape.Problems(child), denied)
			}
			a.walk(file, at, child, denied)
		}
	case []any:
		for i, child := range n {
			a.walk(file, fmt.Sprintf("%s[%d]", path, i), child, denied)
		}
	}
}

// hold records one occurrence: as findings, or as a refusal the corpus pins.
//
// A STEP THAT PINS THIS SHAPE'S REFUSAL IS NOT AN EXEMPTION, and that
// difference is this whole function. scenarios/denials.json deliberately sends
// an add_actor carrying a controller and a grant stating no kind, and asserts
// the server refuses both. That fixture is DEFENDING §5.1: a refused command
// never becomes history, and history is what this gate is about. So the
// obligation is derived from the step's own expectation rather than from a list
// of files to skip.
//
// THE STEP MUST PIN THIS SHAPE'S REFUSAL, not merely SOME refusal, and the
// difference is not academic. gateway.handleCommand runs Authorize before it
// reaches validateAddActor, so a player's add_actor is turned away as
// unauthorized whatever else is wrong with it — and "is this step denied?"
// would let those steps carry a seeded controller into the committed corpus,
// green, as the template the next author copies.
//
// So the step's claim must be part of the refusal this shape actually earns.
// Those strings are literals here because internal/harness may not import
// internal/gateway, but they are self-correcting rather than stale-able: if the
// gateway's wording drifts, denials.json's own assertions must change with it,
// and a claim that is no longer part of the refusal stops pinning anything and
// this gate goes loud. A fixture can still lie its way out — but only in the
// platform's own words, which is a forgery rather than a slip.
func (a *actorKindAudit) hold(file, path string, shape *corpusShape, problems []actorKindProblem, denied string) {
	pinned := 0
	for _, p := range problems {
		if pins(denied, p.Refusal) {
			pinned++
			continue
		}
		a.Findings = append(a.Findings, actorKindFinding{File: file, Path: path, Why: p.Why})
	}
	if pinned > 0 && pinned == len(problems) {
		a.Refusals++
		return
	}
	a.Held[shape.Name]++
}

// pins reports whether a step's deniedContaining claim names this refusal, and
// only this one.
//
// EXACTLY ONE, because the alternative is evasion by VAGUENESS — this gate's
// own failure mode, one level up from the fixtures it guards. Every refusal
// below contains the word "actor", and so does the authz denial a player earns
// for add_actor (`role "player" may not issue "add_actor"`, authz.go). A step
// claiming `"deniedContaining": "actor"` therefore passes the harness while
// naming every refusal at once, and a match-ANY rule would let it pin all of
// them and report nothing. Nobody has to lie to write that; they only have to
// be lazy, which is the likelier failure and the one a guard should assume.
//
// A claim that could be any of several refusals has identified none of them.
func pins(claim, refusal string) bool {
	if claim == "" {
		return false
	}
	named := 0
	for _, candidate := range allRefusals {
		if strings.Contains(candidate, claim) {
			named++
		}
	}
	return named == 1 && strings.Contains(refusal, claim)
}

// allRefusals is every refusal a claim could be naming, and the set pins
// measures a claim's ambiguity against.
var allRefusals = []string{
	refusesASeededController,
	refusesACreationWithNoKind,
	refusesAGrantWithNoKind,
}

// The refusals internal/gateway gives for each shape this gate rejects,
// verbatim from validateAddActor and validateGrantActorControl. A scenario step
// pins a refusal by claiming a substring of one of these; see hold.
const (
	refusesASeededController = "creating an actor does not hand it to anyone; control is " +
		"conferred by grant_actor_control, which also says whether the actor is a party member " +
		"or not. Add the actor with no controller, then grant it"
	refusesACreationWithNoKind = "creating an actor must say what it IS"
	refusesAGrantWithNoKind    = "a grant must say what the actor IS"
)

// creationProblems holds one ActorAdded or AddActor payload to the two rules
// that govern creation.
//
// IT IS DELIBERATELY STRICTER THAN internal/gateway's validateAddActor in one
// place. That function returns early for an actor with no id, so as not to
// answer "your KIND is wrong" to a caller who sent nothing to ask about — a
// refusal that misdescribes the rule teaches the wrong rule. A fixture has no
// caller to teach: an actor with no id in the committed corpus is broken
// whatever else is true of it, and over-strictness here costs a sentence while
// under-strictness costs the rule.
//
// Which shapes count IS mirrored from that function. A declared-but-empty
// controller_ids ([""]) is refused there too, for a reason worth repeating: it
// confers control on nobody, but a caller who wrote that field meant to seed a
// controller, and the answer to "you cannot seed one here" must not depend on
// whether the id they picked happened to be usable.
func creationProblems(payload any) []actorKindProblem {
	actor := lookup(payload, "actor")
	var out []actorKindProblem
	// The mirror field and the authoritative set are ONE fault between them, not
	// two: a fixture setting both has made a single mistake and is told so once.
	seedsControl := false
	if id, _ := lookup(actor, "controllerId", "controller_id").(string); id != "" {
		seedsControl = true
	}
	if ids, _ := lookup(actor, "controllerIds", "controller_ids").([]any); len(ids) > 0 {
		seedsControl = true
	}
	if seedsControl {
		out = append(out, actorKindProblem{
			Why: "an actor is created holding a controller. Creation makes a character; control " +
				"is conferred once, by a grant that declares kind (spec §5.1). Add the actor with " +
				"no controller, then grant it",
			Refusal: refusesASeededController,
		})
	}
	if !statesAKind(lookup(actor, "kind")) {
		out = append(out, actorKindProblem{
			Why: "a created actor states no kind. Every actor says what it IS at birth (spec " +
				"§5.1), because an unstated kind cannot be told from a deliberate one and is " +
				"refused rather than guessed. Set kind to ACTOR_KIND_PARTY_MEMBER or " +
				"ACTOR_KIND_NON_PARTY",
			Refusal: refusesACreationWithNoKind,
		})
	}
	return out
}

// grantProblems holds one ActorControlGranted or GrantActorControl payload to
// the rule that governs a grant.
//
// THE REASON DIFFERS BETWEEN THE COMMAND AND THE EVENT, and saying only the
// first would be a claim a reader can falsify in thirty seconds. A kindless
// grant COMMAND is refused on the wire (validateGrantActorControl). A kindless
// ActorControlGranted EVENT is not: engine.Apply keeps accepting them on
// purpose, because "a fold that refused them would poison every campaign in
// existence". What makes such an event wrong HERE is that nothing can produce
// one — no modern command emits it, and there is no older history to have
// emitted it, since §5.1's migration rule was deleted 2026-08-24 on the grounds
// that no campaign is in use by anyone. So a kindless grant in a committed
// stream is a hand-edit or a regression, and either way it is the old model
// coming back the only way it still can.
func grantProblems(payload any) []actorKindProblem {
	if statesAKind(lookup(payload, "kind")) {
		return nil
	}
	return []actorKindProblem{{
		Why: "a grant states no kind. Control is conferred once, by a grant that declares kind " +
			"(spec §5.1): the command is REFUSED on the wire, and no other writer can produce " +
			"such an event, so a fixture carrying one is reintroducing the ambiguity the leak " +
			"lived in — the DM assigning a character and an agent taking a monster, byte-identical",
		Refusal: refusesAGrantWithNoKind,
	}}
}

// deniedClaimOf returns the refusal a scenario step claims its command earns
// (harness.Expect's deniedContaining), or "" if this object is not such a step.
func deniedClaimOf(obj map[string]any) string {
	denied, _ := lookup(lookup(obj, "expect"), "deniedContaining").(string)
	return denied
}

// statesAKind reports whether v names a kind the contract defines and is not
// the UNSPECIFIED zero.
//
// The accepted set is read from the generated enum maps rather than typed here,
// so a contract that gains a third kind needs no edit and one that renames a
// kind fails loudly. Both wire forms are accepted because protojson emits the
// name and accepts the number.
func statesAKind(v any) bool {
	switch k := v.(type) {
	case string:
		n, ok := vttv1.ActorKind_value[k]
		return ok && n != int32(vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)
	case float64:
		if k != float64(int32(k)) {
			return false
		}
		_, ok := vttv1.ActorKind_name[int32(k)]
		return ok && int32(k) != int32(vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)
	default:
		return false
	}
}

// lookup returns the first of names present in node, or nil.
//
// Exact matches are tried before case-insensitive ones, so the result cannot
// depend on Go's randomized map order. It falls back to a fold at all because
// encoding/json does: the harness would honour "DeniedContaining", so an audit
// that honoured only the exact spelling could be evaded with a capital letter.
func lookup(node any, names ...string) any {
	obj, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	for _, want := range names {
		if v, present := obj[want]; present {
			return v
		}
	}
	for _, key := range jsonKeys(obj) {
		for _, want := range names {
			if strings.EqualFold(key, want) {
				return obj[key]
			}
		}
	}
	return nil
}

// jsonKeys are one object's keys in a stable order, so findings come out in the
// same order on every run.
func jsonKeys(obj map[string]any) []string {
	out := make([]string, 0, len(obj))
	for k := range obj {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
