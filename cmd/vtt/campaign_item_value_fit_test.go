package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func itemValueRulesetDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"id":"item-values","name":"Item Values","format_version":"3","version":"1","properties":[
	{"key":"quebec","type":"number","name":"Quebec","description":"A number only yankee lists."},
	{"key":"romeo","type":"number","name":"Romeo","description":"A number both forms list."},
	{"key":"sierra","type":"number","name":"Sierra","description":"A number no form lists."}],
	"forms":[
	{"key":"zulu","name":"Zulu","description":"The form declared first.","properties":["romeo"]},
	{"key":"yankee","name":"Yankee","description":"The form declared second.","properties":["quebec","romeo"]}]}`
	for name, body := range map[string]string{"ruleset.json": manifest, "guide.md": "# Item Values\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// VTT-1064
func TestACampaignWhoseActorHoldsAValueACatalogueFormListsIsNotServed(t *testing.T) {
	dir := writeFitCampaign(t,
		&vttv1.Actor{ActorId: "zed", Attributes: map[string]int32{"romeo": 1}},
		&vttv1.Actor{ActorId: "fit", Attributes: map[string]int32{"sierra": 2}},
		&vttv1.Actor{ActorId: "abe", Attributes: map[string]int32{"sierra": 1, "quebec": 3}},
	)
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", itemValueRulesetDir(t), "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose actors hold a key a form lists was served")
	}
	msg := err.Error()
	abe := strings.Index(msg, `actor "abe": attributes: holds property "quebec", which form "yankee" lists for its entries`)
	zed := strings.Index(msg, `actor "zed": attributes: holds property "romeo", which form "zulu" lists for its entries`)
	if abe < 0 || zed < 0 || abe > zed {
		t.Fatalf("error = %q, want abe's key a form lists, then zed's", msg)
	}
	if strings.Contains(msg, `"fit"`) {
		t.Errorf("error = %q, names the fitting actor", msg)
	}
}

// VTT-1064
func TestACampaignWhoseActorsHoldOnlyNumbersNoFormListsIsServed(t *testing.T) {
	dir := writeFitCampaign(t, &vttv1.Actor{ActorId: "fit", Attributes: map[string]int32{"sierra": 2}})
	_, closeFn, err := composeServer(dir, "127.0.0.1:0", itemValueRulesetDir(t), "")
	if err != nil {
		t.Fatalf("a campaign whose sheets fit was refused: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
