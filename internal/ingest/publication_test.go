package ingest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

func TestRunPublishesBundleThenDeduplicatesNextRun(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := readFixture(t, "testdata/mixed-outcomes-v1.json")
	options := tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted}

	first, err := Run(input, root, options)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertFileEquals(t, filepath.Join(first.BundlePath, ReportFile), first.Artifact.ReportBytes)
	assertFileEquals(t, filepath.Join(first.BundlePath, JournalFile), first.Artifact.Journal)
	assertFileEquals(t, filepath.Join(root, StateFile), first.Artifact.StateBytes)

	second, err := Run(input, root, options)
	if err != nil {
		t.Fatalf("Run() second error = %v", err)
	}
	if second.BundlePath == first.BundlePath {
		t.Fatal("second committed run reused the first generation's bundle")
	}
	if len(second.Artifact.Journal) != 0 {
		t.Fatalf("second journal = %q, want no journal", second.Artifact.Journal)
	}
	if _, err := os.Stat(filepath.Join(second.BundlePath, JournalFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second journal file error = %v, want not exist", err)
	}
	before, err := os.ReadFile(filepath.Join(root, StateFile))
	if err != nil {
		t.Fatalf("ReadFile(state) error = %v", err)
	}
	third, err := Run(input, root, options)
	if err != nil {
		t.Fatalf("Run() third error = %v", err)
	}
	if third.BundlePath != second.BundlePath {
		t.Fatalf("third bundle = %q, want idempotent retry %q", third.BundlePath, second.BundlePath)
	}
	after, err := os.ReadFile(filepath.Join(root, StateFile))
	if err != nil {
		t.Fatalf("ReadFile(state) after retry error = %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("duplicate-only retry changed committed state")
	}
}

func TestRunRetriesPublishedBundleAfterStateCommitFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	committedState, err := EncodeState(EmptyState())
	if err != nil {
		t.Fatalf("EncodeState() error = %v", err)
	}
	statePath := filepath.Join(root, StateFile)
	if err := os.WriteFile(statePath, committedState, 0o644); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}
	input := readFixture(t, "testdata/valid-v1.json")
	options := tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted}
	want, err := Build(input, EmptyState(), options)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	injected := errors.New("stop before state commit")
	_, err = runWithHooks(input, root, options, publicationHooks{fail: func(stage publicationStage) error {
		if stage == stageBeforeStateCommit {
			return injected
		}
		return nil
	}})
	if !errors.Is(err, injected) || !errors.Is(err, ErrPublication) {
		t.Fatalf("runWithHooks() error = %v", err)
	}
	bundlePath := filepath.Join(root, RunsDirectory, want.RunIdentity.HexDigest())
	assertFileEquals(t, filepath.Join(bundlePath, ReportFile), want.ReportBytes)
	assertFileEquals(t, statePath, committedState)

	retried, err := Run(input, root, options)
	if err != nil {
		t.Fatalf("Run() retry error = %v", err)
	}
	if retried.BundlePath != bundlePath {
		t.Fatalf("retry bundle = %q, want %q", retried.BundlePath, bundlePath)
	}
	assertFileEquals(t, statePath, want.StateBytes)
}

func TestRunAllErrorPublishesReportWithoutJournalOrState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := []byte(`{
  "schema_version": 1,
  "records": [{
    "source": {"namespace": "manual", "display": "entries/error.json"},
    "occurred_at": "2026-08-10",
    "description": "不正な仕訳",
    "postings": [
      {"account": "invalid account", "amount": "100", "commodity": "JPY"},
      {"account": "資産:現金", "amount": "-100", "commodity": "JPY"}
    ]
  }]
}`)

	result, err := Run(input, root, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Artifact.HasErrors || len(result.Artifact.Report.Outcomes) != 1 || result.Artifact.Report.Outcomes[0].Status != OutcomeError {
		t.Fatalf("Run() artifact = %+v", result.Artifact)
	}
	assertFileEquals(t, filepath.Join(result.BundlePath, ReportFile), result.Artifact.ReportBytes)
	if _, err := os.Stat(filepath.Join(result.BundlePath, JournalFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(root, StateFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state error = %v, want not exist", err)
	}
}

func TestRunFailureBeforeBundleCommitPreservesExistingState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stateBytes, err := EncodeState(EmptyState())
	if err != nil {
		t.Fatalf("EncodeState() error = %v", err)
	}
	statePath := filepath.Join(root, StateFile)
	if err := os.WriteFile(statePath, stateBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}
	input := readFixture(t, "testdata/valid-v1.json")
	options := tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted}
	want, err := Build(input, EmptyState(), options)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	injected := errors.New("stop before bundle commit")
	_, err = runWithHooks(input, root, options, publicationHooks{fail: func(stage publicationStage) error {
		if stage == stageBeforeBundleCommit {
			return injected
		}
		return nil
	}})
	if !errors.Is(err, injected) {
		t.Fatalf("runWithHooks() error = %v", err)
	}
	assertFileEquals(t, statePath, stateBytes)
	if _, err := os.Stat(filepath.Join(root, RunsDirectory, want.RunIdentity.HexDigest())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final bundle after failure error = %v, want not exist", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, RunsDirectory))
	if err != nil {
		t.Fatalf("ReadDir(runs) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary bundle was not cleaned: %+v", entries)
	}
}

func TestRunRejectsConflictingImmutableBundle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	input := readFixture(t, "testdata/valid-v1.json")
	options := tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted}
	want, err := Build(input, EmptyState(), options)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	bundlePath := filepath.Join(root, RunsDirectory, want.RunIdentity.HexDigest())
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundlePath, ReportFile), []byte("different\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Run(input, root, options); !errors.Is(err, ErrBundleConflict) {
		t.Fatalf("Run() error = %v, want ErrBundleConflict", err)
	}
	if _, err := os.Stat(filepath.Join(root, StateFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state after conflict error = %v, want not exist", err)
	}
}

func assertFileEquals(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file %q differs\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}
