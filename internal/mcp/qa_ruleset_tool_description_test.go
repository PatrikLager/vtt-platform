package mcp_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func qaDescToolText(t *testing.T) string {
	t.Helper()
	fs := newFakeServer(t, nil)
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Name == "get_ruleset" {
			return strings.ToLower(strings.Join(strings.Fields(tl.Description), " "))
		}
	}
	t.Fatal("get_ruleset is not listed")
	return ""
}

// VTT-853
func TestQADescToolSaysEachAbilityCarriesTheRulesetsDescriptionOfIt(t *testing.T) {
	text := qaDescToolText(t)
	i := strings.Index(text, "description")
	if i < 0 {
		t.Fatalf("description never says description:\n%s", text)
	}
	rest := text[i:]
	if j := strings.Index(rest, ")"); j >= 0 {
		rest = rest[:j]
	}
	if !strings.Contains(rest, "absent") && !strings.Contains(rest, "without") &&
		!strings.Contains(rest, "none") && !strings.Contains(rest, "omitted") {
		t.Errorf("no absent-when-none wording:\n%s", text)
	}
}

// VTT-860
func TestQADescToolSaysItAnswersTheRulesetsNameForAFreeAbility(t *testing.T) {
	text := qaDescToolText(t)
	i := strings.Index(text, "freeusagename")
	if i < 0 {
		t.Fatalf("description never names freeUsageName:\n%s", text)
	}
	rest := text[i:]
	if j := strings.Index(rest, ". "); j >= 0 {
		rest = rest[:j]
	}
	if !strings.Contains(rest, "resource") {
		t.Errorf("freeUsageName not tied to no-resource:\n%s", text)
	}
	if !strings.Contains(rest, "absent") && !strings.Contains(rest, "none") &&
		!strings.Contains(rest, "without") && !strings.Contains(rest, "omitted") {
		t.Errorf("does not say it is absent when none is given:\n%s", text)
	}
}
