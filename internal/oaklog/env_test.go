package oaklog

import (
	"os"
	"strings"
	"testing"
)

func TestLoadPastebinAPIKeyMissingDoesNotFail(t *testing.T) {
	wd := chdirTemp(t)
	defer wd()
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()

	apiKey, err := loadPastebinAPIKey()
	if err == nil || apiKey != "" {
		t.Fatalf("expected missing key error, got apiKey=%q err=%v", apiKey, err)
	}
	if !strings.Contains(err.Error(), pastebinAPIKeyError) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPastebinAPIKeyUsesDotEnv(t *testing.T) {
	wd := chdirTemp(t)
	defer wd()
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()

	if err := os.WriteFile(".env", []byte(strings.TrimSpace(`
# comment

PASTEBIN_API = "abc123"
`)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	apiKey, err := loadPastebinAPIKey()
	if err != nil {
		t.Fatalf("loadPastebinAPIKey returned error: %v", err)
	}
	if apiKey != "abc123" {
		t.Fatalf("expected quoted value to load, got %q", apiKey)
	}
}

func TestLoadPastebinAPIKeyKeepsEnvValue(t *testing.T) {
	wd := chdirTemp(t)
	defer wd()
	t.Setenv("PASTEBIN_API", "from-env")

	if err := os.WriteFile(".env", []byte("PASTEBIN_API=from-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	apiKey, err := loadPastebinAPIKey()
	if err != nil {
		t.Fatalf("loadPastebinAPIKey returned error: %v", err)
	}
	if apiKey != "from-env" {
		t.Fatalf("expected env to win, got %q", apiKey)
	}
}

func TestLoadPastebinAPIKeyRejectsBlank(t *testing.T) {
	t.Setenv("PASTEBIN_API", "   ")
	apiKey, err := loadPastebinAPIKey()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if apiKey != "" {
		t.Fatalf("expected empty api key, got %q", apiKey)
	}
	if !strings.Contains(err.Error(), pastebinAPIKeyError) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPastebinAPIKeyDoesNotLeakDotEnvValue(t *testing.T) {
	wd := chdirTemp(t)
	defer wd()
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()

	if err := os.WriteFile(".env", []byte("PASTEBIN_API=supersecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PASTEBIN_API", "   ")
	_, err := loadPastebinAPIKey()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("validation error leaked secret: %v", err)
	}
}

func unsetEnv(t *testing.T, key string) func() {
	t.Helper()
	old, ok := os.LookupEnv(key)
	if ok {
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	return func() {
		if ok {
			if err := os.Setenv(key, old); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}
