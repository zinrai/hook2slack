// hook2slack is a webhook-to-Slack relay. See README.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		usage()
		return fmt.Errorf("no subcommand given")
	}
	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "serve":
		return cmdServe(args)
	case "check":
		return cmdCheck(args)
	case "render":
		return cmdRender(args)
	case "version":
		printVersion()
		return nil
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown subcommand %q", sub)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: hook2slack <subcommand> [flags]

Subcommands:
  serve    run the HTTP server
  check    validate configuration, schema, and template without starting the server
  render   render a payload through a schema+template and print the Slack JSON it would POST
  version  print build version

Run "hook2slack <subcommand> -h" for subcommand-specific flags.
`)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func cmdServe(args []string) error {
	fs := newFlagSet("serve")
	var configPath string
	fs.StringVar(&configPath, "config", "hook2slack.yaml", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	schema, err := LoadSchema(cfg.SchemaFile)
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	tmpl, err := LoadTemplate(cfg.TemplateFile)
	if err != nil {
		return fmt.Errorf("load template: %w", err)
	}

	slackClient := NewSlackClient()

	mux := newServeMux(cfg, schema, tmpl, slackClient, logger)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("hook2slack serving", "listen", cfg.Listen)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// cmdCheck loads config, schema, and template without starting the
// server. Intended for CI use before deployment.
func cmdCheck(args []string) error {
	fs := newFlagSet("check")
	var configPath string
	fs.StringVar(&configPath, "config", "hook2slack.yaml", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if _, err := LoadSchema(cfg.SchemaFile); err != nil {
		return fmt.Errorf("load schema: %w", err)
	}
	if _, err := LoadTemplate(cfg.TemplateFile); err != nil {
		return fmt.Errorf("load template: %w", err)
	}
	fmt.Fprintln(os.Stdout, "ok")
	return nil
}

// cmdRender runs the schema-validation + template-rendering pipeline
// against a sample payload and prints the JSON body that would be
// POSTed to Slack. Intended for iterating on a new schema+template
// pair before deploying it. No Slack request is made.
func cmdRender(args []string) error {
	fs := newFlagSet("render")
	var schemaPath, templatePath, payloadPath string
	fs.StringVar(&schemaPath, "schema", "", "path to the JSON Schema file")
	fs.StringVar(&templatePath, "template", "", "path to the Go text/template file")
	fs.StringVar(&payloadPath, "payload", "", "path to a sample JSON payload file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if schemaPath == "" || templatePath == "" || payloadPath == "" {
		fs.Usage()
		return fmt.Errorf("--schema, --template, and --payload are all required")
	}

	schema, err := LoadSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}
	tmpl, err := LoadTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("load template: %w", err)
	}

	payloadBytes, err := os.ReadFile(payloadPath)
	if err != nil {
		return fmt.Errorf("read payload %s: %w", payloadPath, err)
	}
	var payload any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("parse payload %s: %w", payloadPath, err)
	}

	if err := schema.Validate(payload); err != nil {
		return err
	}

	rendered, err := tmpl.Render(payload)
	if err != nil {
		return err
	}

	// Same validity check that SlackClient.Post applies, so a
	// malformed template surfaces here with the same error rather
	// than as a Slack rejection in production.
	var attachment json.RawMessage
	if err := json.Unmarshal(rendered, &attachment); err != nil {
		return fmt.Errorf("rendered template is not valid JSON: %w", err)
	}

	body := slackPayload{
		Attachments: []json.RawMessage{attachment},
	}
	out, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preview: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(out))
	return nil
}
