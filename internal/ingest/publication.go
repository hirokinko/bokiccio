package ingest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

const (
	RunsDirectory = "runs"
	ReportFile    = "report.json"
	JournalFile   = "journal.txn"
	StateFile     = "state-v1.json"
)

var (
	ErrPublication    = errors.New("run publication failed")
	ErrBundleConflict = errors.New("immutable run bundle conflicts with existing content")
)

type RunResult struct {
	Artifact   Artifact
	BundlePath string
}

// Run loads the committed state, builds one deterministic artifact, publishes
// its immutable bundle, and atomically replaces the state manifest last.
func Run(input []byte, outputRoot string, options tacklerfmt.Options) (RunResult, error) {
	return runWithHooks(input, outputRoot, options, publicationHooks{})
}

type publicationStage string

const (
	stageAfterReport        publicationStage = "after_report"
	stageAfterJournal       publicationStage = "after_journal"
	stageBeforeBundleCommit publicationStage = "before_bundle_commit"
	stageBeforeStateCommit  publicationStage = "before_state_commit"
)

type publicationHooks struct {
	fail func(publicationStage) error
}

func runWithHooks(input []byte, outputRoot string, options tacklerfmt.Options, hooks publicationHooks) (RunResult, error) {
	if outputRoot == "" {
		return RunResult{}, fmt.Errorf("%w: output root is empty", ErrPublication)
	}
	state, err := loadState(filepath.Join(outputRoot, StateFile))
	if err != nil {
		return RunResult{}, err
	}
	artifact, err := Build(input, state, options)
	if err != nil {
		return RunResult{}, err
	}
	bundlePath, err := publish(outputRoot, state, artifact, hooks)
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Artifact: artifact, BundlePath: bundlePath}, nil
}

func loadState(path string) (State, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return EmptyState(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("%w: open state: %w", ErrPublication, err)
	}
	defer file.Close()
	state, err := DecodeState(file)
	if err != nil {
		return State{}, err
	}
	return state, nil
}

func publish(outputRoot string, previous State, artifact Artifact, hooks publicationHooks) (string, error) {
	runsPath := filepath.Join(outputRoot, RunsDirectory)
	if err := os.MkdirAll(runsPath, 0o755); err != nil {
		return "", fmt.Errorf("%w: create runs directory: %w", ErrPublication, err)
	}
	finalPath := filepath.Join(runsPath, artifact.RunIdentity.HexDigest())
	if _, err := os.Stat(finalPath); err == nil {
		if err := verifyExistingBundle(finalPath, artifact); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: inspect run bundle: %w", ErrPublication, err)
	} else if err := publishNewBundle(runsPath, finalPath, artifact, hooks); err != nil {
		return "", err
	}

	if artifact.NextState.Generation != previous.Generation {
		if err := injectFailure(hooks, stageBeforeStateCommit); err != nil {
			return "", err
		}
		if err := atomicWrite(filepath.Join(outputRoot, StateFile), artifact.StateBytes); err != nil {
			return "", fmt.Errorf("%w: commit state: %w", ErrPublication, err)
		}
	}
	return finalPath, nil
}

func publishNewBundle(runsPath, finalPath string, artifact Artifact, hooks publicationHooks) (resultErr error) {
	temporaryPath, err := os.MkdirTemp(runsPath, ".run-tmp-")
	if err != nil {
		return fmt.Errorf("%w: create temporary bundle: %w", ErrPublication, err)
	}
	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(temporaryPath)
		}
	}()

	if err := atomicWrite(filepath.Join(temporaryPath, ReportFile), artifact.ReportBytes); err != nil {
		return fmt.Errorf("%w: write report: %w", ErrPublication, err)
	}
	if err := injectFailure(hooks, stageAfterReport); err != nil {
		return err
	}
	if len(artifact.Journal) > 0 {
		if err := atomicWrite(filepath.Join(temporaryPath, JournalFile), artifact.Journal); err != nil {
			return fmt.Errorf("%w: write journal: %w", ErrPublication, err)
		}
		if err := injectFailure(hooks, stageAfterJournal); err != nil {
			return err
		}
	}
	if err := injectFailure(hooks, stageBeforeBundleCommit); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		if _, statErr := os.Stat(finalPath); statErr == nil {
			return verifyExistingBundle(finalPath, artifact)
		}
		return fmt.Errorf("%w: commit run bundle: %w", ErrPublication, err)
	}
	return nil
}

func verifyExistingBundle(path string, artifact Artifact) error {
	if err := verifyFile(filepath.Join(path, ReportFile), artifact.ReportBytes); err != nil {
		return fmt.Errorf("%w: report: %v", ErrBundleConflict, err)
	}
	journalPath := filepath.Join(path, JournalFile)
	if len(artifact.Journal) == 0 {
		if _, err := os.Stat(journalPath); err == nil {
			return fmt.Errorf("%w: unexpected journal", ErrBundleConflict)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: inspect journal: %v", ErrBundleConflict, err)
		}
		return nil
	}
	if err := verifyFile(journalPath, artifact.Journal); err != nil {
		return fmt.Errorf("%w: journal: %v", ErrBundleConflict, err)
	}
	return nil
}

func verifyFile(path string, expected []byte) error {
	actual, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("content differs")
	}
	return nil
}

func atomicWrite(path string, data []byte) (resultErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".write-tmp-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if resultErr != nil {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func injectFailure(hooks publicationHooks, stage publicationStage) error {
	if hooks.fail == nil {
		return nil
	}
	if err := hooks.fail(stage); err != nil {
		return fmt.Errorf("%w: injected at %s: %w", ErrPublication, stage, err)
	}
	return nil
}
