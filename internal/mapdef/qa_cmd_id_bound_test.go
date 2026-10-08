package mapdef_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

var qaCmdMapObjectField = regexp.MustCompile(`(?i)objects?(\[\d+\])?[ _.]?id`)

type qaCmdMapCase struct {
	label string
	ids   []string
	value string
}

func qaCmdMapLong() []qaCmdMapCase {
	return []qaCmdMapCase{
		{"129 ascii", []string{"qa-1", strings.Repeat("a", 129), "qa-3"}, strings.Repeat("a", 129)},
		{"65 two-byte runes", []string{"qa-1", "qa-2", strings.Repeat("é", 65)}, strings.Repeat("é", 65)},
		{"43 three-byte runes", []string{strings.Repeat("€", 43), "qa-2"}, strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", []string{"qa-1", strings.Repeat("😀", 32) + "a"},
			strings.Repeat("😀", 32) + "a"},
	}
}

func qaCmdMapBadShape() []qaCmdMapCase {
	edge := strings.Repeat("é", 64)
	return []qaCmdMapCase{
		{"empty", []string{"qa-1", "", "qa-3"}, ""},
		{"repeated apart", []string{"qa-1", "qa-2", "qa-3", "qa-1"}, "qa-1"},
		{"repeated adjacent", []string{"qa-1", "qa-2", "qa-2"}, "qa-2"},
		{"repeated at the bound", []string{"qa-1", edge, "qa-3", edge}, edge},
	}
}

func qaCmdMapObjects(ids []string) []map[string]any {
	objs := make([]map[string]any, 0, len(ids))
	for i, id := range ids {
		objs = append(objs, map[string]any{
			"id": id, "kind": "crate", "at": []int{i % 4, i / 4}, "size": []int{1, 1}, "rot": 0,
			"blocks_sight": false, "blocks_move": true, "art": "qa-crate",
		})
	}
	return objs
}

func qaCmdMapWrite(t *testing.T, dir, mapID string, objectIDs []string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format_version": 1, "id": mapID, "name": "qa", "grid_width": 4, "grid_height": 4,
		"objects": qaCmdMapObjects(objectIDs),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, mapID+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func qaCmdMapNamesFileAndField(t *testing.T, err error, file, value string) {
	t.Helper()
	msg := err.Error()
	if !strings.Contains(msg, file) {
		t.Errorf("refusal %q does not name the file %s", msg, file)
	}
	rest := strings.ReplaceAll(msg, file, "<file>")
	if value != "" {
		rest = strings.ReplaceAll(rest, value, "<v>")
	}
	t.Logf("refusal: %s", rest)
	if !qaCmdMapObjectField.MatchString(rest) {
		t.Errorf("refusal %q does not name the object id field", rest)
	}
}

func qaCmdMapRefusedBoth(t *testing.T, c qaCmdMapCase) {
	t.Helper()
	dir := t.TempDir()
	path := qaCmdMapWrite(t, dir, "qa-map", c.ids)
	m, err := mapdef.Load(path)
	if err == nil || m != nil {
		t.Fatalf("Load: map=%v err=%v, want a refusal", m != nil, err)
	}
	qaCmdMapNamesFileAndField(t, err, path, c.value)
	m, err = mapdef.LoadInstalled(dir, "qa-map", "")
	if err == nil || m != nil {
		t.Fatalf("LoadInstalled: map=%v err=%v, want a refusal", m != nil, err)
	}
	qaCmdMapNamesFileAndField(t, err, "maps/qa-map.json", c.value)
	if strings.Contains(err.Error(), dir) {
		t.Errorf("LoadInstalled refusal %q names the path opened", err)
	}
}

// VTT-294
func TestQACmdAMapWhoseObjectIDIsOverTheBoundIsRefusedNamingFileAndField(t *testing.T) {
	for _, c := range qaCmdMapLong() {
		t.Run(c.label, func(t *testing.T) { qaCmdMapRefusedBoth(t, c) })
	}
}

// VTT-295
func TestQACmdAMapWhoseObjectIDIsEmptyOrRepeatedIsRefusedNamingFileAndField(t *testing.T) {
	for _, c := range qaCmdMapBadShape() {
		t.Run(c.label, func(t *testing.T) { qaCmdMapRefusedBoth(t, c) })
	}
}

func qaCmdMapAtBound() [][]string {
	stem := strings.Repeat("é", 63) + "x"
	return [][]string{
		{"S" + strings.Repeat("a", 126) + "E", "qa-2", strings.Repeat("😀", 32)},
		{stem + "a", stem + "b", strings.Repeat("€", 42) + "ab"},
		{"e", "é", "E"},
	}
}

// VTT-294 VTT-295
func TestQACmdAMapWithDistinctObjectIDsUpToTheBoundLoadsWhole(t *testing.T) {
	for _, ids := range qaCmdMapAtBound() {
		dir := t.TempDir()
		m, err := mapdef.Load(qaCmdMapWrite(t, dir, "qa-map", ids))
		if err != nil {
			t.Fatalf("Load refused object ids of %d to %d bytes: %v", len(ids[1]), len(ids[0]), err)
		}
		installed, err := mapdef.LoadInstalled(dir, "qa-map", "")
		if err != nil {
			t.Fatalf("LoadInstalled refused object ids up to the bound: %v", err)
		}
		for _, got := range []*mapdef.Map{m, installed} {
			var held []string
			for _, o := range got.Objects {
				held = append(held, o.ID)
			}
			if !slices.Equal(held, ids) {
				t.Fatalf("loaded map holds %d object ids, not the %d sent whole", len(held), len(ids))
			}
		}
		envs, _, err := mapdef.Compile(installed, "")
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		var compiled []string
		for _, o := range envs[0].GetSceneCreated().GetObjects() {
			compiled = append(compiled, o.GetObjectId())
		}
		if !slices.Equal(compiled, ids) {
			t.Fatalf("compiled SceneCreated carries %d object ids, not the %d loaded", len(compiled), len(ids))
		}
	}
}

var errQACmdField = errors.New("qa field error")

// VTT-294 VTT-295
func TestQACmdCheckObjectIDsRefusesThroughTheFieldErrorItIsGiven(t *testing.T) {
	objs := func(ids []string) []mapdef.Object {
		out := make([]mapdef.Object, 0, len(ids))
		for _, id := range ids {
			out = append(out, mapdef.Object{ID: id, Kind: "crate", W: 1, H: 1, Art: "qa-crate"})
		}
		return out
	}
	for _, c := range append(qaCmdMapLong(), qaCmdMapBadShape()...) {
		t.Run(c.label, func(t *testing.T) {
			var field string
			err := mapdef.CheckObjectIDs(objs(c.ids), func(f, _ string) error {
				field = f
				return errQACmdField
			})
			if !errors.Is(err, errQACmdField) {
				t.Fatalf("CheckObjectIDs: %v, want the field error it was given", err)
			}
			if !qaCmdMapObjectField.MatchString(field) {
				t.Errorf("field %q does not name an object id", field)
			}
		})
	}
	for _, ids := range qaCmdMapAtBound() {
		if err := mapdef.CheckObjectIDs(objs(ids), func(string, string) error { return errQACmdField }); err != nil {
			t.Fatalf("CheckObjectIDs refused distinct ids up to the bound: %v", err)
		}
	}
}
