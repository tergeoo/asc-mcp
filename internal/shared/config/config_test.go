package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLoadInlineKey(t *testing.T) {
	setEnv(t, map[string]string{
		"ASC_ISSUER_ID":  "issuer",
		"ASC_KEY_ID":     "kid",
		"ASC_PRIVATE_KEY": `-----BEGIN EC PRIVATE KEY-----\nABC\n-----END EC PRIVATE KEY-----`,
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.BaseURL != defaultBaseURL {
		t.Fatalf("baseURL = %s", cfg.BaseURL)
	}
	// The literal \n must be converted to real newlines.
	if want := "-----BEGIN EC PRIVATE KEY-----\nABC\n-----END EC PRIVATE KEY-----"; string(cfg.PrivateKey) != want {
		t.Fatalf("private key newlines not expanded: %q", string(cfg.PrivateKey))
	}
}

func TestLoadKeyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.p8")
	if err := os.WriteFile(path, []byte("KEYDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	setEnv(t, map[string]string{
		"ASC_ISSUER_ID": "issuer",
		"ASC_KEY_ID":    "kid",
		"ASC_KEY_PATH":  path,
	})
	t.Setenv("ASC_PRIVATE_KEY", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if string(cfg.PrivateKey) != "KEYDATA" {
		t.Fatalf("key from file = %q", string(cfg.PrivateKey))
	}
}

func TestLoadMissingCredentials(t *testing.T) {
	setEnv(t, map[string]string{
		"ASC_ISSUER_ID":   "",
		"ASC_KEY_ID":      "",
		"ASC_PRIVATE_KEY": "",
		"ASC_KEY_PATH":    "",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error when credentials are missing")
	}
}

func TestLoadFromEnvFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.p8")
	if err := os.WriteFile(keyPath, []byte("KEYDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, "config.env")
	content := "ASC_ISSUER_ID=file-issuer\nASC_KEY_ID=FILEKEY\nASC_KEY_PATH=" + keyPath + "\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// Ensure nothing is set in the environment so the file is the source.
	for _, k := range []string{"ASC_ISSUER_ID", "ASC_KEY_ID", "ASC_PRIVATE_KEY", "ASC_KEY_PATH"} {
		t.Setenv(k, "")
	}
	t.Setenv("ASC_ENV_FILE", envFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.IssuerID != "file-issuer" || cfg.KeyID != "FILEKEY" {
		t.Fatalf("values not loaded from file: %+v", cfg)
	}
	if cfg.EnvFile != envFile {
		t.Fatalf("EnvFile = %q, want %q", cfg.EnvFile, envFile)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.p8")
	_ = os.WriteFile(keyPath, []byte("K"), 0o600)
	envFile := filepath.Join(dir, "config.env")
	_ = os.WriteFile(envFile, []byte("ASC_ISSUER_ID=from-file\nASC_KEY_ID=K\nASC_KEY_PATH="+keyPath+"\n"), 0o600)

	t.Setenv("ASC_ENV_FILE", envFile)
	t.Setenv("ASC_ISSUER_ID", "from-env") // explicit env must win

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.IssuerID != "from-env" {
		t.Fatalf("env should override file: got %q", cfg.IssuerID)
	}
}

func TestProfileSelectsFile(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "asc-mcp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "acme.p8")
	_ = os.WriteFile(keyPath, []byte("K"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "acme.env"),
		[]byte("ASC_ISSUER_ID=acme\nASC_KEY_ID=ACME\nASC_KEY_PATH="+keyPath+"\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "config.env"),
		[]byte("ASC_ISSUER_ID=default\nASC_KEY_ID=DEF\n"), 0o600)

	for _, k := range []string{"ASC_ISSUER_ID", "ASC_KEY_ID", "ASC_PRIVATE_KEY", "ASC_KEY_PATH", "ASC_ENV_FILE", "XDG_CONFIG_HOME"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("HOME", home)
	t.Setenv("ASC_PROFILE", "acme")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.IssuerID != "acme" || cfg.Profile != "acme" {
		t.Fatalf("profile not used: issuer=%q profile=%q", cfg.IssuerID, cfg.Profile)
	}
}

func TestProfilesLists(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "asc-mcp")
	_ = os.MkdirAll(dir, 0o755)
	for _, n := range []string{"acme.env", "personal.env", "config.env"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("x=1"), 0o600)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", home)

	got := map[string]bool{}
	for _, p := range Profiles() {
		got[p] = true
	}
	if !got["acme"] || !got["personal"] {
		t.Fatalf("expected acme+personal profiles, got %v", got)
	}
	if got["config"] {
		t.Fatal("config.env must not be listed as a profile")
	}
}

func TestSaveProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", home)

	src := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(src, []byte("-----BEGIN PRIVATE KEY-----\nx\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	envPath, err := SaveProfile("acme", "iss-1", "KID1", src)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// .env references the centrally-copied key, both with 0600 perms.
	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "ASC_ISSUER_ID=iss-1") || !strings.Contains(string(data), "acme.p8") {
		t.Fatalf("unexpected env contents: %s", data)
	}
	keyDst := filepath.Join(ConfigDir(), "acme.p8")
	info, err := os.Stat(keyDst)
	if err != nil {
		t.Fatalf("key not copied: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key perms = %v, want 0600", info.Mode().Perm())
	}

	// And it loads back as a profile.
	for _, k := range []string{"ASC_ISSUER_ID", "ASC_KEY_ID", "ASC_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_ENV_FILE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("ASC_PROFILE", "acme")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load saved profile: %v", err)
	}
	if cfg.IssuerID != "iss-1" || cfg.KeyID != "KID1" {
		t.Fatalf("loaded wrong values: %+v", cfg)
	}
}

func TestSaveProfileRejectsBadInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := SaveProfile("bad/name", "i", "k", "/tmp/x"); err == nil {
		t.Fatal("expected error for invalid profile name")
	}
	src := filepath.Join(t.TempDir(), "notakey.txt")
	_ = os.WriteFile(src, []byte("hello"), 0o600)
	if _, err := SaveProfile("ok", "i", "k", src); err == nil {
		t.Fatal("expected error for non-.p8 content")
	}
}

func TestMissingConfigErrorType(t *testing.T) {
	for _, k := range []string{"ASC_ISSUER_ID", "ASC_KEY_ID", "ASC_PRIVATE_KEY", "ASC_KEY_PATH", "ASC_ENV_FILE"} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", t.TempDir()) // avoid picking up a real ~/.config file
	_, err := Load()
	var missing *MissingConfigError
	if !errors.As(err, &missing) {
		t.Fatalf("expected *MissingConfigError, got %T: %v", err, err)
	}
	if len(missing.Missing) == 0 {
		t.Fatal("expected missing fields to be listed")
	}
}

func TestSetupGuideMentionsKeySources(t *testing.T) {
	g := SetupGuide()
	for _, want := range []string{"ASC_ISSUER_ID", "ASC_KEY_ID", ".p8", "config.env", "appstoreconnect.apple.com"} {
		if !strings.Contains(g, want) {
			t.Fatalf("setup guide missing %q", want)
		}
	}
}

func TestLoadDBRequiresDSN(t *testing.T) {
	setEnv(t, map[string]string{
		"ASC_ISSUER_ID":   "issuer",
		"ASC_KEY_ID":      "kid",
		"ASC_PRIVATE_KEY": "key",
		"DB_ENABLED":      "true",
		"DB_DSN":          "",
	})
	if _, err := Load(); err == nil {
		t.Fatal("expected error: DB_ENABLED without DB_DSN")
	}
}
