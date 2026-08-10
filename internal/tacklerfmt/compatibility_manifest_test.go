package tacklerfmt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	wantTacklerVersion = "26.1.2"
	wantGrammarCommit  = "0641bb09b7cc52bd037c6f6ce4cc377fb72facec"
)

type compatibilityManifest struct {
	Reference compatibilityReference `json:"reference"`
	Cases     []compatibilityCase    `json:"cases"`
}

type compatibilityReference struct {
	TacklerVersion string `json:"tackler_version"`
	GrammarCommit  string `json:"grammar_commit"`
}

type compatibilityCase struct {
	ID      string `json:"id"`
	Feature string `json:"feature"`
	Expect  string `json:"expect"`
	Path    string `json:"path"`
}

func TestCompatibilityManifest(t *testing.T) {
	t.Parallel()
	root := compatibilityRoot()
	manifest := readCompatibilityManifest(t, root)

	if manifest.Reference.TacklerVersion != wantTacklerVersion {
		t.Errorf("tackler_version = %q, want %q", manifest.Reference.TacklerVersion, wantTacklerVersion)
	}
	if manifest.Reference.GrammarCommit != wantGrammarCommit {
		t.Errorf("grammar_commit = %q, want %q", manifest.Reference.GrammarCommit, wantGrammarCommit)
	}
	if len(manifest.Cases) == 0 {
		t.Fatal("manifest has no compatibility cases")
	}

	seenIDs := make(map[string]bool)
	seenPaths := make(map[string]bool)
	for _, testCase := range manifest.Cases {
		if testCase.ID == "" || testCase.Feature == "" {
			t.Errorf("case has empty id or feature: %+v", testCase)
		}
		if seenIDs[testCase.ID] {
			t.Errorf("duplicate case id %q", testCase.ID)
		}
		seenIDs[testCase.ID] = true
		if testCase.Expect != "accept" && testCase.Expect != "reject" {
			t.Errorf("case %q expect = %q, want accept or reject", testCase.ID, testCase.Expect)
		}
		if !strings.HasPrefix(testCase.Path, testCase.Expect+"/") {
			t.Errorf("case %q path %q is outside its expectation directory", testCase.ID, testCase.Path)
		}
		if filepath.IsAbs(testCase.Path) || filepath.Clean(testCase.Path) != testCase.Path || strings.HasPrefix(testCase.Path, "..") {
			t.Errorf("case %q has unsafe path %q", testCase.ID, testCase.Path)
			continue
		}
		if seenPaths[testCase.Path] {
			t.Errorf("duplicate fixture path %q", testCase.Path)
		}
		seenPaths[testCase.Path] = true
		validateCompatibilityFixture(t, root, testCase)
	}

	fixturePaths, err := filepath.Glob(filepath.Join(root, "*", "*.txn"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(fixturePaths) != len(seenPaths) {
		t.Errorf("fixture count = %d, manifest paths = %d", len(fixturePaths), len(seenPaths))
	}
	for _, path := range fixturePaths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("Rel(%q) error = %v", path, err)
		}
		if !seenPaths[filepath.ToSlash(relative)] {
			t.Errorf("fixture %q is not declared in manifest", relative)
		}
	}
}

func compatibilityRoot() string {
	return filepath.Join("testdata", "compatibility")
}

func readCompatibilityManifest(t *testing.T, root string) compatibilityManifest {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatalf("Open(manifest) error = %v", err)
	}
	defer file.Close()

	var manifest compatibilityManifest
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatalf("Decode(manifest) error = %v", err)
	}
	return manifest
}

func validateCompatibilityFixture(t *testing.T, root string, testCase compatibilityCase) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(testCase.Path)))
	if err != nil {
		t.Errorf("case %q: ReadFile() error = %v", testCase.ID, err)
		return
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("case %q: fixture must be non-empty and end with newline", testCase.ID)
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), '\r') {
		t.Errorf("case %q: fixture must be UTF-8 with LF line endings", testCase.ID)
	}
	header := regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}(?: |$)`)
	if count := len(header.FindAll(data, -1)); count != 1 {
		t.Errorf("case %q: transaction header count = %d, want 1", testCase.ID, count)
	}
	for _, forbidden := range []string{"/home/", ".tmp/", "gs://"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("case %q: fixture contains forbidden real-data marker %q", testCase.ID, forbidden)
		}
	}
}

func (c compatibilityCase) String() string {
	return fmt.Sprintf("%s (%s, %s)", c.ID, c.Feature, c.Expect)
}
