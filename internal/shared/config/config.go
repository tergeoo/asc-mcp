// Package config loads and validates runtime configuration from the
// environment. Secrets (the .p8 private key) are held only in memory and are
// never logged or echoed back through tools.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var profileNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// SaveProfile stores a named credential profile centrally: it copies the .p8
// into the config dir as <name>.p8 (0600) and writes <name>.env (0600) with the
// issuer id, key id and key path. Returns the path of the written .env file.
func SaveProfile(name, issuerID, keyID, p8Source string) (string, error) {
	if !profileNameRe.MatchString(name) {
		return "", fmt.Errorf("invalid profile name %q (use letters, digits, '-' or '_')", name)
	}
	if issuerID == "" || keyID == "" || p8Source == "" {
		return "", errors.New("issuer id, key id and .p8 path are all required")
	}
	src, err := os.ReadFile(p8Source)
	if err != nil {
		return "", fmt.Errorf("read .p8 file: %w", err)
	}
	if !bytes.Contains(src, []byte("PRIVATE KEY")) {
		return "", fmt.Errorf("%s does not look like a .p8 PEM key", p8Source)
	}
	dir := ConfigDir()
	if dir == "" {
		return "", errors.New("cannot determine config directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	keyDst := filepath.Join(dir, name+".p8")
	if err := os.WriteFile(keyDst, src, 0o600); err != nil {
		return "", fmt.Errorf("write key: %w", err)
	}
	envPath := filepath.Join(dir, name+".env")
	content := fmt.Sprintf("ASC_ISSUER_ID=%s\nASC_KEY_ID=%s\nASC_KEY_PATH=%s\n", issuerID, keyID, keyDst)
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write profile: %w", err)
	}
	return envPath, nil
}

// Config is the fully-resolved server configuration.
type Config struct {
	// App Store Connect credentials.
	IssuerID   string
	KeyID      string
	PrivateKey []byte // PEM contents of the .p8 key

	// ASC API base URL (overridable for tests / sandbox proxies).
	BaseURL string

	// Database. When DBEnabled is false the persistence layer is a no-op and
	// the MCP core runs without idempotency/audit/cache.
	DBEnabled bool
	DBDSN     string

	// Behavioural flags.
	DryRun bool

	// Optional echo HTTP server for observability (healthz/metrics/debug REST).
	HTTPEnabled bool
	HTTPAddr    string

	// EnvFile is the config file that was auto-loaded, if any (for logging).
	EnvFile string
	// Profile is the active ASC_PROFILE, if any (for logging).
	Profile string
}

const defaultBaseURL = "https://api.appstoreconnect.apple.com"

// MissingConfigError signals that required credentials are absent. The server
// prints the setup guide when it sees this.
type MissingConfigError struct{ Missing []string }

func (e *MissingConfigError) Error() string {
	return "missing required configuration: " + strings.Join(e.Missing, ", ")
}

// loadEnvFile loads the first existing config file from a conventional search
// path into the process environment (without overriding values already set).
// Returns the path that was loaded, or "".
func loadEnvFile() string {
	for _, p := range candidateEnvFiles() {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			vals, err := godotenv.Read(p)
			if err != nil {
				continue
			}
			// A file value fills any variable that is empty or unset; a non-empty
			// environment variable (e.g. from the MCP client) always wins.
			for k, v := range vals {
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
			return p
		}
	}
	return ""
}

// candidateEnvFiles is the ordered search path for the config file. When
// ASC_PROFILE is set, per-profile files (<profile>.env) are used instead of the
// default config.env, so multiple App Store Connect accounts/teams can each have
// their own credentials and be switched with one variable.
func candidateEnvFiles() []string {
	// An explicit path always wins.
	paths := []string{os.Getenv("ASC_ENV_FILE")}

	profile := os.Getenv("ASC_PROFILE")
	base := "config.env"
	if profile != "" {
		base = profile + ".env"
	} else {
		paths = append(paths, ".env") // project-local default only
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "asc-mcp", base))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "asc-mcp", base))
		if profile == "" {
			paths = append(paths, filepath.Join(home, ".asc-mcp", ".env"))
		}
	}
	return paths
}

// ConfigDir returns the conventional per-user config directory for asc-mcp.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "asc-mcp")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "asc-mcp")
}

// Profiles lists the available profile names (from <name>.env files in the
// config directory, excluding the default config.env).
func Profiles() []string {
	dir := ConfigDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".env") || name == "config.env" {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".env"))
	}
	return out
}

// Load reads configuration from the environment (and an optional config file)
// and validates it.
func Load() (*Config, error) {
	envFile := loadEnvFile()

	cfg := &Config{
		EnvFile: envFile,
		Profile: os.Getenv("ASC_PROFILE"),
		IssuerID:    os.Getenv("ASC_ISSUER_ID"),
		KeyID:       os.Getenv("ASC_KEY_ID"),
		BaseURL:     getenvDefault("ASC_BASE_URL", defaultBaseURL),
		DBDSN:       os.Getenv("DB_DSN"),
		DBEnabled:   getenvBool("DB_ENABLED", false),
		DryRun:      getenvBool("ASC_DRY_RUN", false),
		HTTPEnabled: getenvBool("HTTP_ENABLED", false),
		HTTPAddr:    getenvDefault("HTTP_ADDR", ":8080"),
	}

	key, err := loadPrivateKey()
	if err != nil {
		return nil, err
	}
	cfg.PrivateKey = key

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadPrivateKey resolves the .p8 key from ASC_PRIVATE_KEY (inline PEM, with
// literal "\n" allowed) or from a file at ASC_KEY_PATH.
func loadPrivateKey() ([]byte, error) {
	if inline := os.Getenv("ASC_PRIVATE_KEY"); inline != "" {
		return []byte(strings.ReplaceAll(inline, `\n`, "\n")), nil
	}
	if path := os.Getenv("ASC_KEY_PATH"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read ASC_KEY_PATH (%s): %w", path, err)
		}
		return data, nil
	}
	// Neither configured — reported as a missing-config item by validate().
	return nil, nil
}

func (c *Config) validate() error {
	var missing []string
	if c.IssuerID == "" {
		missing = append(missing, "ASC_ISSUER_ID")
	}
	if c.KeyID == "" {
		missing = append(missing, "ASC_KEY_ID")
	}
	if len(c.PrivateKey) == 0 {
		missing = append(missing, "ASC_KEY_PATH (or ASC_PRIVATE_KEY)")
	}
	if len(missing) > 0 {
		return &MissingConfigError{Missing: missing}
	}
	if c.DBEnabled && c.DBDSN == "" {
		return errors.New("DB_ENABLED=true requires DB_DSN")
	}
	return nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
