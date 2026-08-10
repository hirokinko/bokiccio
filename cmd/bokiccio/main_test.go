package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/ingest"
)

func TestRunImportEndToEndAndRerun(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inputPath := copyFixture(t, root, "mixed.json", filepath.Join("..", "..", "internal", "ingest", "testdata", "mixed-outcomes-v1.json"))
	outputRoot := filepath.Join(root, "output")

	var firstStderr bytes.Buffer
	if got := run([]string{"import", "--input", inputPath, "--output", outputRoot}, &firstStderr); got != exitRecordErrors {
		t.Fatalf("run() exit = %d, want %d; stderr:\n%s", got, exitRecordErrors, &firstStderr)
	}
	for _, text := range []string{"success=1", "warning=1", "error=1", "duplicate=1", "journal=true", "bundle=runs/"} {
		if !strings.Contains(firstStderr.String(), text) {
			t.Errorf("stderr does not contain %q:\n%s", text, &firstStderr)
		}
	}
	if strings.Contains(firstStderr.String(), root) {
		t.Errorf("stderr exposes an absolute workspace path:\n%s", &firstStderr)
	}
	firstBundle := bundleFromSummary(t, outputRoot, firstStderr.String())
	firstReport := readReport(t, filepath.Join(firstBundle, ingest.ReportFile))
	if len(firstReport.Outcomes) != 4 {
		t.Fatalf("len(first report outcomes) = %d, want 4", len(firstReport.Outcomes))
	}
	journal, err := os.ReadFile(filepath.Join(firstBundle, ingest.JournalFile))
	if err != nil {
		t.Fatalf("ReadFile(journal) error = %v", err)
	}
	for _, text := range []string{"source: receipts/sample-success.jpg", "source: receipts/sample-warning.jpg", "WARN [receipt.total_mismatch]"} {
		if !bytes.Contains(journal, []byte(text)) {
			t.Errorf("journal does not contain %q:\n%s", text, journal)
		}
	}
	if bytes.Contains(journal, []byte("sample-error")) {
		t.Errorf("journal contains rejected record:\n%s", journal)
	}
	stateBefore, err := os.ReadFile(filepath.Join(outputRoot, ingest.StateFile))
	if err != nil {
		t.Fatalf("ReadFile(state) error = %v", err)
	}

	var secondStderr bytes.Buffer
	if got := run([]string{"import", "--input", inputPath, "--output", outputRoot}, &secondStderr); got != exitRecordErrors {
		t.Fatalf("run() retry exit = %d, want %d; stderr:\n%s", got, exitRecordErrors, &secondStderr)
	}
	for _, text := range []string{"success=0", "warning=0", "error=1", "duplicate=3", "journal=false"} {
		if !strings.Contains(secondStderr.String(), text) {
			t.Errorf("retry stderr does not contain %q:\n%s", text, &secondStderr)
		}
	}
	secondBundle := bundleFromSummary(t, outputRoot, secondStderr.String())
	if secondBundle == firstBundle {
		t.Fatal("committed rerun reused the pre-commit-generation bundle")
	}
	if _, err := os.Stat(filepath.Join(secondBundle, ingest.JournalFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry journal error = %v, want not exist", err)
	}
	stateAfter, err := os.ReadFile(filepath.Join(outputRoot, ingest.StateFile))
	if err != nil {
		t.Fatalf("ReadFile(state after retry) error = %v", err)
	}
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("retry changed state despite adding no successful identities")
	}
}

func TestRunImportExitSuccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inputPath := copyFixture(t, root, "valid.json", filepath.Join("..", "..", "internal", "ingest", "testdata", "valid-v1.json"))
	outputRoot := filepath.Join(root, "output")
	var stderr bytes.Buffer
	if got := run([]string{"import", "--input", inputPath, "--output", outputRoot}, &stderr); got != exitSuccess {
		t.Fatalf("run() exit = %d, want %d; stderr:\n%s", got, exitSuccess, &stderr)
	}
	if !strings.Contains(stderr.String(), "success=2 warning=0 error=0 duplicate=0 journal=true") {
		t.Fatalf("stderr summary = %q", stderr.String())
	}

	stderr.Reset()
	if got := run([]string{"import", "--input", inputPath, "--output", outputRoot}, &stderr); got != exitSuccess {
		t.Fatalf("run() retry exit = %d, want %d; stderr:\n%s", got, exitSuccess, &stderr)
	}
	if !strings.Contains(stderr.String(), "success=0 warning=0 error=0 duplicate=2 journal=false") {
		t.Fatalf("retry stderr summary = %q", stderr.String())
	}
}

func TestRunImportRunLevelFailures(t *testing.T) {
	t.Parallel()
	t.Run("usage", func(t *testing.T) {
		var stderr bytes.Buffer
		if got := run(nil, &stderr); got != exitRunLevelFailure || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("run() = %d, stderr %q", got, stderr.String())
		}
	})
	t.Run("help", func(t *testing.T) {
		var stderr bytes.Buffer
		if got := run([]string{"import", "--help"}, &stderr); got != exitSuccess || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("run() = %d, stderr %q", got, stderr.String())
		}
	})
	t.Run("global help", func(t *testing.T) {
		var stderr bytes.Buffer
		if got := run([]string{"--help"}, &stderr); got != exitSuccess || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("run() = %d, stderr %q", got, stderr.String())
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		root := t.TempDir()
		inputPath := filepath.Join(root, "invalid.json")
		if err := os.WriteFile(inputPath, []byte(`{"schema_version":2,"records":[]}`), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		outputRoot := filepath.Join(root, "output")
		var stderr bytes.Buffer
		if got := run([]string{"import", "--input", inputPath, "--output", outputRoot}, &stderr); got != exitRunLevelFailure {
			t.Fatalf("run() = %d, want %d", got, exitRunLevelFailure)
		}
		if !strings.Contains(stderr.String(), "unsupported schema version") {
			t.Fatalf("stderr = %q", stderr.String())
		}
		if _, err := os.Stat(outputRoot); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("output root error = %v, want not exist", err)
		}
	})
	t.Run("output I/O", func(t *testing.T) {
		root := t.TempDir()
		inputPath := copyFixture(t, root, "valid.json", filepath.Join("..", "..", "internal", "ingest", "testdata", "valid-v1.json"))
		outputRoot := filepath.Join(root, "not-a-directory")
		if err := os.WriteFile(outputRoot, []byte("occupied"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		var stderr bytes.Buffer
		if got := run([]string{"import", "--input", inputPath, "--output", outputRoot}, &stderr); got != exitRunLevelFailure {
			t.Fatalf("run() = %d, want %d", got, exitRunLevelFailure)
		}
		if !strings.Contains(stderr.String(), "run publication failed") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
}

func copyFixture(t *testing.T, root, name, source string) string {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", source, err)
	}
	destination := filepath.Join(root, name)
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", destination, err)
	}
	return destination
}

func bundleFromSummary(t *testing.T, outputRoot, summary string) string {
	t.Helper()
	marker := "bundle="
	index := strings.Index(summary, marker)
	if index < 0 {
		t.Fatalf("summary has no bundle: %q", summary)
	}
	relative := strings.TrimSpace(summary[index+len(marker):])
	return filepath.Join(outputRoot, filepath.FromSlash(relative))
}

func readReport(t *testing.T, path string) ingest.Report {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(report) error = %v", err)
	}
	defer file.Close()
	report, err := ingest.DecodeReport(file)
	if err != nil {
		t.Fatalf("DecodeReport() error = %v", err)
	}
	return report
}
