//go:build oaklog_dev

package oaklog

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevDotenvFallbackWorks(t *testing.T) {
	wd := chdirTempDir(t)
	defer wd()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()
	if err := os.WriteFile(".env", []byte("PASTEBIN_API=from-dev-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.PastebinAPI != "from-dev-dotenv" {
			t.Fatalf("expected dev dotenv token, got %q", cfg.PastebinAPI)
		}
		return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
	}

	if err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestDevEnvWinsOverDotenvFallback(t *testing.T) {
	wd := chdirTempDir(t)
	defer wd()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	t.Setenv("PASTEBIN_API", "from-env")
	if err := os.WriteFile(".env", []byte("PASTEBIN_API=from-dev-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.PastebinAPI != "from-env" {
			t.Fatalf("expected env token, got %q", cfg.PastebinAPI)
		}
		return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
	}

	if err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestDevCLIWinsOverMalformedDotenvFallback(t *testing.T) {
	wd := chdirTempDir(t)
	defer wd()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	if err := os.WriteFile(".env", []byte("not dotenv syntax [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.PastebinAPI != "from-cli" {
			t.Fatalf("expected CLI token, got %q", cfg.PastebinAPI)
		}
		return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
	}

	if err := run(context.Background(), []string{"pastebin", "--pastebin-api", "from-cli", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestDevMalformedDotenvErrorsWhenReached(t *testing.T) {
	wd := chdirTempDir(t)
	defer wd()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	if err := os.WriteFile(".env", []byte("not dotenv syntax [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Fatalf("expected dev dotenv parse error, got %v", err)
	}
}
