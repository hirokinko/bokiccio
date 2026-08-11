package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hirokinko/bokiccio/internal/webprod"
	"github.com/hirokinko/bokiccio/internal/webstore"
)

const backupOperationTimeout = 5 * time.Minute

func runBackup(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("bokiccio backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printBackupUsage(stderr) }
	outputPath := flags.String("output", "", "new local path for the logical backup")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitRunLevelFailure
	}
	if flags.NArg() != 0 || *outputPath == "" {
		fmt.Fprintln(stderr, "error: --output is required and positional arguments are not accepted")
		printBackupUsage(stderr)
		return exitRunLevelFailure
	}
	config, err := webprod.LoadDatabaseConfig(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(stderr, "error: configuration: %v\n", err)
		return exitRunLevelFailure
	}
	ctx, cancel := context.WithTimeout(context.Background(), backupOperationTimeout)
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
	data, err := webstore.New(database).Backup(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "error: backup failed")
		return exitRunLevelFailure
	}
	if err := writeNewPrivateFile(*outputPath, data); err != nil {
		fmt.Fprintln(stderr, "error: write backup failed")
		return exitRunLevelFailure
	}
	fmt.Fprintf(stderr, "backup written (format %d, schema %d)\n", webstore.BackupFormatVersion, webstore.SchemaVersion)
	return exitSuccess
}

func runRestore(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("bokiccio restore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printRestoreUsage(stderr) }
	inputPath := flags.String("input", "", "path to a Bokiccio logical backup")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitRunLevelFailure
	}
	if flags.NArg() != 0 || *inputPath == "" {
		fmt.Fprintln(stderr, "error: --input is required and positional arguments are not accepted")
		printRestoreUsage(stderr)
		return exitRunLevelFailure
	}
	config, err := webprod.LoadDatabaseConfig(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(stderr, "error: configuration: %v\n", err)
		return exitRunLevelFailure
	}
	data, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Fprintln(stderr, "error: read backup failed")
		return exitRunLevelFailure
	}
	ctx, cancel := context.WithTimeout(context.Background(), backupOperationTimeout)
	defer cancel()
	database, err := webprod.OpenRemote(ctx, config)
	if err != nil {
		fmt.Fprintln(stderr, "error: database connection failed")
		return exitRunLevelFailure
	}
	defer database.Close()
	if err := webstore.New(database).Restore(ctx, data); err != nil {
		fmt.Fprintln(stderr, "error: restore failed")
		return exitRunLevelFailure
	}
	fmt.Fprintf(stderr, "backup restored (format %d, schema %d)\n", webstore.BackupFormatVersion, webstore.SchemaVersion)
	return exitSuccess
}

func writeNewPrivateFile(path string, data []byte) (resultErr error) {
	directory := filepath.Dir(path)
	published := false
	temporary, err := os.CreateTemp(directory, ".bokiccio-backup-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		if resultErr != nil && published {
			_ = os.Remove(path)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
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
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	published = true
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return err
	}
	return nil
}

func printBackupUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio backup --output <new-path>")
	fmt.Fprintln(destination)
	fmt.Fprintf(destination, "Requires %s and %s. The output file is created with permission 0600 and is never overwritten.\n",
		webprod.DatabaseURLEnv, webprod.DatabaseTokenEnv)
}

func printRestoreUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio restore --input <path>")
	fmt.Fprintln(destination)
	fmt.Fprintf(destination, "Requires %s and %s. The target must have current schema and no application data.\n",
		webprod.DatabaseURLEnv, webprod.DatabaseTokenEnv)
}
