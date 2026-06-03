// Command server is the MCP server entrypoint. It wires configuration, the
// ES256 JWT provider, the App Store Connect client facade, the persistence
// layer, and registers all feature tools on the stdio MCP transport.
//
// Logging goes to stderr; stdout is reserved for the MCP stdio transport.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/tergeoo/asc-mcp/internal/features/apps"
	"github.com/tergeoo/asc-mcp/internal/features/iap"
	"github.com/tergeoo/asc-mcp/internal/features/localization"
	"github.com/tergeoo/asc-mcp/internal/features/screenshots"
	"github.com/tergeoo/asc-mcp/internal/features/submission"
	"github.com/tergeoo/asc-mcp/internal/features/versions"
	platformhttp "github.com/tergeoo/asc-mcp/internal/platform/http"
	"github.com/tergeoo/asc-mcp/internal/shared/asc"
	"github.com/tergeoo/asc-mcp/internal/shared/auth"
	"github.com/tergeoo/asc-mcp/internal/shared/config"
	"github.com/tergeoo/asc-mcp/internal/shared/db"
	"github.com/tergeoo/asc-mcp/internal/shared/store"
	"github.com/tergeoo/asc-mcp/internal/shared/toolkit"
)

const serverName = "asc-mcp"

// serverVersion is overridden at release time via -ldflags "-X main.serverVersion=...".
var serverVersion = "dev"

func main() {
	var (
		showVersion  = flag.Bool("version", false, "print version and exit")
		showSetup    = flag.Bool("setup", false, "print setup instructions (where to get credentials) and exit")
		listProfiles = flag.Bool("list-profiles", false, "list configured credential profiles and exit")
		profile      = flag.String("profile", "", "credential profile to use (loads <profile>.env); also via ASC_PROFILE")
		addProfile   = flag.String("add-profile", "", "create/update a named credential profile in the central config dir and exit")
		issuerFlag   = flag.String("issuer", "", "ASC Issuer ID (with -add-profile)")
		keyIDFlag    = flag.String("key-id", "", "ASC Key ID (with -add-profile)")
		p8Flag       = flag.String("p8", "", "path to the .p8 key file (with -add-profile)")
	)
	flag.Parse()

	// A -profile flag overrides the ASC_PROFILE environment variable.
	if *profile != "" {
		_ = os.Setenv("ASC_PROFILE", *profile)
	}

	switch {
	case *showVersion:
		fmt.Printf("%s %s\n", serverName, serverVersion)
		return
	case *showSetup:
		fmt.Print(config.SetupGuide())
		return
	case *addProfile != "":
		if err := runAddProfile(*addProfile, *issuerFlag, *keyIDFlag, *p8Flag); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case *listProfiles:
		names := config.Profiles()
		if len(names) == 0 {
			fmt.Printf("No profiles found in %s\n(create <name>.env there, then run with -profile <name>)\n", config.ConfigDir())
			return
		}
		fmt.Println("Available profiles (use -profile <name> or ASC_PROFILE=<name>):")
		for _, n := range names {
			fmt.Printf("  - %s\n", n)
		}
		return
	}

	logger := log.New(os.Stderr, "[asc-mcp] ", log.LstdFlags|log.Lmsgprefix)

	if err := run(logger); err != nil {
		// Missing credentials is the common first-run case: print actionable
		// guidance to stderr rather than a bare error.
		var missing *config.MissingConfigError
		if errors.As(err, &missing) {
			logger.Printf("configuration error: %v", err)
			fmt.Fprintln(os.Stderr)
			fmt.Fprint(os.Stderr, config.SetupGuide())
			os.Exit(1)
		}
		logger.Fatalf("fatal: %v", err)
	}
}

// runAddProfile creates a named credential profile, prompting for any value not
// supplied via flags. All files land in the central config dir.
func runAddProfile(name, issuer, keyID, p8 string) error {
	r := bufio.NewReader(os.Stdin)
	issuer = promptIfEmpty(r, "Issuer ID", issuer)
	keyID = promptIfEmpty(r, "Key ID", keyID)
	p8 = expandHome(promptIfEmpty(r, "Path to .p8 file", p8))

	envPath, err := config.SaveProfile(name, issuer, keyID, p8)
	if err != nil {
		return err
	}
	fmt.Printf("Saved profile %q:\n  %s\n  %s\n\nUse it with:\n  asc-mcp -profile %s        (or ASC_PROFILE=%s)\n",
		name, envPath, filepath.Join(config.ConfigDir(), name+".p8"), name, name)
	return nil
}

func promptIfEmpty(r *bufio.Reader, label, val string) string {
	if val != "" {
		return val
	}
	fmt.Fprintf(os.Stderr, "%s: ", label)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Profile != "" {
		logger.Printf("using profile %q", cfg.Profile)
	}
	if cfg.EnvFile != "" {
		logger.Printf("loaded configuration from %s", cfg.EnvFile)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Auth provider (ES256).
	provider, err := auth.NewProvider(cfg.IssuerID, cfg.KeyID, cfg.PrivateKey)
	if err != nil {
		return err
	}

	// ASC client facade.
	facade, err := asc.NewFacade(cfg.BaseURL, provider.Token)
	if err != nil {
		return err
	}

	// Persistence (optional).
	st := buildStore(ctx, cfg, logger)

	deps := &toolkit.Deps{ASC: facade, Store: st, DryRun: cfg.DryRun}

	// Optional observability server.
	if cfg.HTTPEnabled {
		httpSrv := platformhttp.New(cfg.HTTPAddr, st)
		go func() {
			logger.Printf("observability server listening on %s", cfg.HTTPAddr)
			if err := httpSrv.Start(ctx); err != nil {
				logger.Printf("observability server error: %v", err)
			}
		}()
	}

	// Build the MCP server and register every feature's tools.
	mcpServer := server.NewMCPServer(serverName, serverVersion,
		server.WithToolCapabilities(true),
	)
	apps.Register(mcpServer, deps)
	versions.Register(mcpServer, deps)
	localization.Register(mcpServer, deps)
	iap.Register(mcpServer, deps)
	screenshots.Register(mcpServer, deps)
	submission.Register(mcpServer, deps)

	if cfg.DryRun {
		logger.Printf("DRY-RUN mode: write tools validate and diff without calling App Store Connect")
	}
	logger.Printf("%s %s ready (db=%v) — serving MCP over stdio", serverName, serverVersion, st.Enabled())

	return server.ServeStdio(mcpServer)
}

// buildStore returns a Postgres-backed store when DB is enabled and reachable,
// otherwise a no-op store so the MCP core keeps working.
func buildStore(ctx context.Context, cfg *config.Config, logger *log.Logger) store.Store {
	if !cfg.DBEnabled {
		return store.Noop{}
	}
	database, err := db.Open(ctx, cfg.DBDSN)
	if err != nil {
		logger.Printf("WARNING: DB enabled but unavailable (%v); continuing without persistence", err)
		return store.Noop{}
	}
	if err := db.Migrate(database); err != nil {
		logger.Printf("WARNING: migrations failed (%v); continuing without persistence", err)
		return store.Noop{}
	}
	logger.Printf("persistence enabled (Postgres)")
	return db.NewStore(database)
}
