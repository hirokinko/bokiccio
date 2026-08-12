//go:build tackler_integration

package tacklerfmt

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTacklerCompatibility(t *testing.T) {
	versionOutput, err := exec.Command("tackler", "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("tackler --version failed: %v: %s", err, versionOutput)
	}
	gotVersion, ok := strings.CutPrefix(strings.TrimSpace(string(versionOutput)), "tackler ")
	if !ok || !tacklerVersionAtLeast(gotVersion, minTacklerVersion) {
		t.Fatalf("tackler --version = %q, want tackler %s or later", strings.TrimSpace(string(versionOutput)), minTacklerVersion)
	}

	root := compatibilityRoot()
	manifest := readCompatibilityManifest(t, root)
	config, err := filepath.Abs(filepath.Join(root, "tackler.toml"))
	if err != nil {
		t.Fatalf("Abs(config) error = %v", err)
	}
	for _, testCase := range manifest.Cases {
		t.Run(testCase.String(), func(t *testing.T) {
			fixture, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(testCase.Path)))
			if err != nil {
				t.Fatalf("Abs(fixture) error = %v", err)
			}
			command := exec.Command("tackler", "--config", config, "--input.file", fixture)
			output, err := command.CombinedOutput()
			if testCase.Expect == "accept" && err != nil {
				t.Fatalf("tackler rejected fixture: %v\n%s", err, output)
			}
			if testCase.Expect == "reject" && err == nil {
				t.Fatalf("tackler accepted rejected fixture\n%s", output)
			}
			if testCase.Expect == "reject" && len(bytes.TrimSpace(output)) == 0 {
				t.Fatal("tackler rejected fixture without a diagnostic")
			}
		})
	}

	t.Run("export-golden", func(t *testing.T) {
		golden, err := filepath.Abs(filepath.Join("testdata", "explicit_entries.golden"))
		if err != nil {
			t.Fatalf("Abs(golden) error = %v", err)
		}
		output, err := exec.Command("tackler", "--config", config, "--input.file", golden).CombinedOutput()
		if err != nil {
			t.Fatalf("tackler rejected exporter golden: %v\n%s", err, output)
		}
	})
}
