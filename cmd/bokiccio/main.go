package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hirokinko/bokiccio/internal/ingest"
	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

const (
	exitSuccess         = 0
	exitRecordErrors    = 1
	exitRunLevelFailure = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitRunLevelFailure
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(stderr)
		return exitSuccess
	}
	if args[0] != "import" {
		fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
		printUsage(stderr)
		return exitRunLevelFailure
	}
	return runImport(args[1:], stderr)
}

func runImport(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("bokiccio import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printImportUsage(stderr) }
	inputPath := flags.String("input", "", "path to normalized import JSON")
	outputRoot := flags.String("output", "", "root directory for run bundles and state")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitRunLevelFailure
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unexpected positional arguments")
		printImportUsage(stderr)
		return exitRunLevelFailure
	}
	if *inputPath == "" || *outputRoot == "" {
		fmt.Fprintln(stderr, "error: --input and --output are required")
		printImportUsage(stderr)
		return exitRunLevelFailure
	}

	input, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: read input: %v\n", err)
		return exitRunLevelFailure
	}
	result, err := ingest.Run(input, *outputRoot, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		fmt.Fprintf(stderr, "error: import failed: %v\n", err)
		return exitRunLevelFailure
	}

	counts := countOutcomes(result.Artifact.Report.Outcomes)
	bundle := filepath.Join(ingest.RunsDirectory, result.Artifact.RunIdentity.HexDigest())
	fmt.Fprintf(stderr,
		"run %s: success=%d warning=%d error=%d duplicate=%d journal=%t bundle=%s\n",
		result.Artifact.RunIdentity.HexDigest(),
		counts[ingest.OutcomeSuccess],
		counts[ingest.OutcomeWarning],
		counts[ingest.OutcomeError],
		counts[ingest.OutcomeDuplicate],
		len(result.Artifact.Journal) > 0,
		filepath.ToSlash(bundle),
	)
	if result.Artifact.HasErrors {
		return exitRecordErrors
	}
	return exitSuccess
}

func countOutcomes(outcomes []ingest.Outcome) map[ingest.OutcomeStatus]int {
	counts := make(map[ingest.OutcomeStatus]int, 4)
	for _, outcome := range outcomes {
		counts[outcome.Status]++
	}
	return counts
}

func printUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio import --input <path> --output <directory>")
}

func printImportUsage(destination io.Writer) {
	printUsage(destination)
	fmt.Fprintln(destination)
	fmt.Fprintln(destination, "Imports normalized JSON into an immutable local run bundle.")
}
