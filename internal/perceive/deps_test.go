package perceive

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/PatrikLager/vtt-platform/"

// Keep these equal to perceive's mayDependOn in .go-arch-lint.yml: contract
// by prefix, as its component is contract/gen/go/**, the rest exactly.
const contractPrefix = module + "contract/gen/go/"

var allowedExactly = []string{
	module + "internal/engine",
	module + "internal/sight",
	module + "internal/perceive",
}

func allowed(path string) bool {
	if !strings.HasPrefix(path, module) || strings.HasPrefix(path, contractPrefix) {
		return true
	}
	return slices.Contains(allowedExactly, path)
}

// VTT-311
func TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight(t *testing.T) {
	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	sawSelf := false
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		read++
		if name == "deps_test.go" {
			sawSelf = true
		}
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: import %s: %v", name, imp.Path.Value, err)
			}
			if !allowed(path) {
				t.Errorf("%s imports %s: the perception decision may import only its own package, contract, engine and sight of this module", name, path)
			}
		}
	}
	// Fail on reading nothing: a walk that finds no file passes every import.
	if read < 2 || !sawSelf {
		t.Fatalf("read %d files (deps_test.go among them: %v); the check read too little to say anything", read, sawSelf)
	}
	t.Logf("read %d files: %s", read, strings.Join(files, ", "))
}

// Fail when the module is renamed: every import of it would then pass as foreign.
func TestTheImportRuleNamesThisModule(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	want := "module " + strings.TrimSuffix(module, "/") + "\n"
	if !strings.HasPrefix(string(mod), want) {
		t.Fatalf("go.mod does not open with %q", want)
	}
}

func TestTheImportRuleTellsAnAllowedImportFromAForbiddenOne(t *testing.T) {
	for path, want := range map[string]bool{
		"fmt":                             true,
		module + "contract/gen/go/vtt/v1": true,
		module + "internal/engine":        true,
		module + "internal/sight":         true,
		module + "internal/perceive":      true,
		module + "internal/gateway":       false,
		module + "internal/identity":      false,
		module + "internal/campaign":      false,
		module + "internal/store":         false,
		module + "internal/engineer":      false,
		module + "internal/engine/x":      false,
		module + "internal/sight/x":       false,
		module + "internal/perceive/sub":  false,
		module + "contract/gen/gox":       false,
		module + "contract-spike/proto":   false,
	} {
		if got := allowed(path); got != want {
			t.Errorf("allowed(%q) = %v, want %v", path, got, want)
		}
	}
}
