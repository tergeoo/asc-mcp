package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetupGuide returns human-readable, copy-pasteable instructions for obtaining
// App Store Connect credentials and storing them securely. It is printed on
// startup when configuration is missing, and on demand via `asc-mcp -setup`.
func SetupGuide() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "~"
	}
	cfgDir := filepath.Join(home, ".config", "asc-mcp")
	cfgFile := filepath.Join(cfgDir, "config.env")
	keyFile := filepath.Join(cfgDir, "AuthKey_XXXXXXXXXX.p8")

	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	w("App Store Connect MCP server — setup")
	w("====================================")
	w("")
	w("You need three things, all from App Store Connect (one-time, ~2 min):")
	w("")
	w("  1. Open https://appstoreconnect.apple.com and sign in.")
	w("  2. Users and Access  ->  Integrations  ->  App Store Connect API (Team Keys).")
	w("  3. Click  +  to generate a key. Give it the \"App Manager\" role")
	w("     (or \"Admin\" if you also need app-availability/pricing).")
	w("")
	w("  From that page take:")
	w("     ASC_ISSUER_ID   the \"Issuer ID\" shown at the top of the keys list")
	w("     ASC_KEY_ID      the \"Key ID\" of the row you just created")
	w("     the .p8 file    \"Download API Key\" — downloadable ONLY ONCE, keep it safe")
	w("")
	w("Recommended secure + convenient storage")
	w("---------------------------------------")
	w("  mkdir -p %s", cfgDir)
	w("  mv ~/Downloads/AuthKey_*.p8 %s/", cfgDir)
	w("  chmod 600 %s/AuthKey_*.p8", cfgDir)
	w("")
	w("  cat > %s <<'EOF'", cfgFile)
	w("  ASC_ISSUER_ID=00000000-0000-0000-0000-000000000000")
	w("  ASC_KEY_ID=XXXXXXXXXX")
	w("  ASC_KEY_PATH=%s", keyFile)
	w("  EOF")
	w("  chmod 600 %s", cfgFile)
	w("")
	w("The server auto-loads the first config file it finds, in this order:")
	w("  1. $ASC_ENV_FILE            (explicit path, if set)")
	w("  2. ./.env                   (project-local)")
	w("  3. $XDG_CONFIG_HOME/asc-mcp/config.env")
	w("  4. %s", cfgFile)
	w("  5. %s", filepath.Join(home, ".asc-mcp", ".env"))
	w("Values already in the environment always win over the file.")
	w("")
	w("Alternatively, set them inline in your MCP client config (.mcp.json):")
	w("  {\"mcpServers\":{\"asc\":{\"command\":\"/path/to/asc-mcp\",\"env\":{")
	w("     \"ASC_ISSUER_ID\":\"…\",\"ASC_KEY_ID\":\"…\",\"ASC_KEY_PATH\":\"%s\"}}}}", keyFile)
	w("")
	w("Multiple App Store Connect accounts / teams")
	w("-------------------------------------------")
	w("An API key belongs to one team, so each account = its own credentials.")
	w("Two convenient options:")
	w("")
	w("  A) Profiles (one binary, switch with a flag/var). Create one per account")
	w("     (copies the .p8 and writes the config into the central dir for you):")
	w("       asc-mcp -add-profile acme -issuer <ID> -key-id <KID> -p8 ~/Downloads/AuthKey_*.p8")
	w("       asc-mcp -add-profile personal -issuer <ID> -key-id <KID> -p8 <path>")
	w("     (omit flags to be prompted). Then:")
	w("       asc-mcp -profile acme       (or ASC_PROFILE=acme)")
	w("       asc-mcp -list-profiles")
	w("     Stored centrally in: %s", cfgDir)
	w("")
	w("  B) Separate MCP servers (recommended for MCP clients). Register the")
	w("     server twice under different names, each with its own env:")
	w("       {\"mcpServers\":{")
	w("          \"asc-acme\":     {\"command\":\"asc-mcp\",\"env\":{\"ASC_PROFILE\":\"acme\"}},")
	w("          \"asc-personal\": {\"command\":\"asc-mcp\",\"env\":{\"ASC_PROFILE\":\"personal\"}}}}")
	w("     The agent then picks tools by server name (asc-acme / asc-personal).")
	w("")
	w("Optional settings: ASC_DRY_RUN=true (validate without writing),")
	w("  DB_ENABLED=true + DB_DSN=postgres://… (idempotency/audit),")
	w("  HTTP_ENABLED=true + HTTP_ADDR=:8080 (healthz/metrics).")
	w("")
	w("Security: the .p8 key and generated tokens are never logged or returned by")
	w("tools. Never commit the .p8 or config.env. Run `asc-mcp -setup` to see this.")
	return b.String()
}
