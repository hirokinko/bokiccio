package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
	"github.com/hirokinko/bokiccio/internal/webprod"
	"github.com/hirokinko/bokiccio/internal/webstore"
	"github.com/hirokinko/bokiccio/internal/webui"
)

func runMigrate(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("bokiccio migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printMigrateUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitRunLevelFailure
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unexpected positional arguments")
		printMigrateUsage(stderr)
		return exitRunLevelFailure
	}
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
	if err := webstore.Migrate(ctx, database); err != nil {
		fmt.Fprintln(stderr, "error: database migration failed")
		return exitRunLevelFailure
	}
	fmt.Fprintf(stderr, "database schema is current (version %d)\n", webstore.SchemaVersion)
	return exitSuccess
}

func runServe(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("bokiccio serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printServeUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		return exitRunLevelFailure
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unexpected positional arguments")
		printServeUsage(stderr)
		return exitRunLevelFailure
	}
	config, err := webprod.LoadServerConfig(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(stderr, "error: configuration: %v\n", err)
		return exitRunLevelFailure
	}
	startupContext, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	database, err := webprod.OpenRemote(startupContext, config.Database)
	if err != nil {
		cancelStartup()
		fmt.Fprintln(stderr, "error: database connection failed")
		return exitRunLevelFailure
	}
	defer database.Close()
	if err := webstore.CheckSchema(startupContext, database); err != nil {
		cancelStartup()
		fmt.Fprintln(stderr, "error: database schema is not current; run bokiccio migrate")
		return exitRunLevelFailure
	}
	validator, err := webprod.NewGoogleIAPValidator(startupContext)
	cancelStartup()
	if err != nil {
		fmt.Fprintln(stderr, "error: IAP validator initialization failed")
		return exitRunLevelFailure
	}
	store := webstore.New(database)
	application, err := webprod.NewApplicationHandler(webapp.NewHandler(store),
		webui.NewHandler(store, webui.HandlerOptions{Development: config.Development}))
	if err != nil {
		fmt.Fprintln(stderr, "error: HTTP application initialization failed")
		return exitRunLevelFailure
	}
	handler, err := webprod.NewProductionHandler(application, validator, config.Security)
	if err != nil {
		fmt.Fprintln(stderr, "error: HTTP security initialization failed")
		return exitRunLevelFailure
	}
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(config.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          log.New(stderr, "http: ", 0),
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	fmt.Fprintf(stderr, "serving IAP-protected API on :%d\n", config.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stderr, "error: HTTP server failed")
		return exitRunLevelFailure
	}
	return exitSuccess
}

func printMigrateUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio migrate")
	fmt.Fprintln(destination)
	fmt.Fprintf(destination, "Requires %s and %s.\n", webprod.DatabaseURLEnv, webprod.DatabaseTokenEnv)
}

func printServeUsage(destination io.Writer) {
	fmt.Fprintln(destination, "usage: bokiccio serve")
	fmt.Fprintln(destination)
	fmt.Fprintf(destination, "Requires %s, %s, %s, %s, and %s.\n",
		webprod.DatabaseURLEnv, webprod.DatabaseTokenEnv, webprod.IAPAudienceEnv,
		webprod.ExternalOriginEnv, webprod.PortEnv)
}
