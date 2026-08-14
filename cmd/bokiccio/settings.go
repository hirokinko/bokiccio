package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hirokinko/bokiccio/internal/webprod"
	"github.com/hirokinko/bokiccio/internal/webstore"
)

func runSettings(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		printSettingsUsage(stderr)
		return exitRunLevelFailure
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printSettingsUsage(stderr)
		return exitSuccess
	}
	if args[0] != "set" {
		fmt.Fprintf(stderr, "error: unknown settings command %q\n", args[0])
		printSettingsUsage(stderr)
		return exitRunLevelFailure
	}
	return runSettingsSet(args[1:], stderr)
}

func runSettingsSet(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("bokiccio settings set", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printSettingsSetUsage(stderr) }
	uploadEnabled := flags.String("file-upload-enabled", "", "whether file upload is enabled (true or false)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitRunLevelFailure
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unexpected positional arguments")
		printSettingsSetUsage(stderr)
		return exitRunLevelFailure
	}
	if *uploadEnabled != "true" && *uploadEnabled != "false" {
		fmt.Fprintln(stderr, "error: --file-upload-enabled must be true or false")
		printSettingsSetUsage(stderr)
		return exitRunLevelFailure
	}
	enabled := *uploadEnabled == "true"
	config, err := webprod.LoadDatabaseConfig(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(stderr, "error: configuration: %v\n", err)
		return exitRunLevelFailure
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := webprod.OpenRemote(ctx, config)
	if err != nil {
		fmt.Fprintln(stderr, "error: database connection failed")
		return exitRunLevelFailure
	}
	defer database.Close()
	if err := webstore.CheckSchema(ctx, database); err != nil {
		fmt.Fprintln(stderr, "error: database schema is not current; run bokiccio migrate")
		return exitRunLevelFailure
	}
	if err := webstore.New(database).SetFileUploadEnabled(ctx, enabled); err != nil {
		fmt.Fprintln(stderr, "error: application setting update failed")
		return exitRunLevelFailure
	}
	writeFileUploadEnabled(stderr, enabled)
	return exitSuccess
}

func writeFileUploadEnabled(destination io.Writer, enabled bool) {
	fmt.Fprintln(destination, enabled)
}

func printSettingsUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio settings <command> [options]")
	fmt.Fprintln(destination)
	fmt.Fprintln(destination, "commands:")
	fmt.Fprintln(destination, "  set  change operator-managed application settings")
}

func printSettingsSetUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio settings set --file-upload-enabled=<true|false>")
	fmt.Fprintln(destination)
	fmt.Fprintf(destination, "Requires %s and %s.\n", webprod.DatabaseURLEnv, webprod.DatabaseTokenEnv)
}
