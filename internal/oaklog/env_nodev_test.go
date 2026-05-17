//go:build !oaklog_dev

package oaklog

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPastebinIgnoresRepoRootDotenvWithoutDevTag(t *testing.T) {
	wd := chdirTempDir(t)
	defer wd()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()
	if err := os.WriteFile(".env", []byte("PASTEBIN_API=from-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), pastebinAPIKeyError) {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestRunPastebinIgnoresMalformedRepoRootDotenvWithoutDevTag(t *testing.T) {
	wd := chdirTempDir(t)
	defer wd()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()
	if err := os.WriteFile(".env", []byte("not dotenv syntax [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), pastebinAPIKeyError) {
		t.Fatalf("expected missing token error, got %v", err)
	}
}
