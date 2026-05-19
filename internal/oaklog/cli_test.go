package oaklog

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubUploader struct {
	result UploadResult
	err    error
	req    *UploadRequest
}

func (s stubUploader) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	if s.req != nil {
		*s.req = req
	}
	return s.result, s.err
}

func TestRunTopLevelHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		var out, errOut bytes.Buffer
		if err := run(context.Background(), args, bytes.NewReader(nil), &out, &errOut); err != nil {
			t.Fatalf("run(%v) returned error: %v", args, err)
		}
		got := out.String()
		if !strings.Contains(got, "oaklog [flags] <log-file|->") || !strings.Contains(got, "oaklog <provider> [flags] <log-file|->") {
			t.Fatalf("top-level help missing usage, got: %q", got)
		}
		if !strings.Contains(got, "Providers:") || !strings.Contains(got, "Default provider:") {
			t.Fatalf("top-level help missing providers, got: %q", got)
		}
		if strings.Contains(got, "--pastebin") || strings.Contains(got, "--mclogs") {
			t.Fatalf("top-level help should not show flat provider flags, got: %q", got)
		}
	}
}

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	old := Version
	Version = "test"
	defer func() { Version = old }()

	for _, args := range [][]string{{"-v"}, {"--version"}} {
		out.Reset()
		errOut.Reset()
		if err := run(context.Background(), args, bytes.NewReader(nil), &out, &errOut); err != nil {
			t.Fatalf("run(%v) returned error: %v", args, err)
		}
		if got := strings.TrimSpace(out.String()); got != "oaklog test" {
			t.Fatalf("unexpected version output: %q", got)
		}
	}
}

func TestRunDefaultProvider(t *testing.T) {
	path := writeTempFile(t, "latest.log", "hello\n")
	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()

	var gotReq UploadRequest
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.Provider != ProviderMCLogs {
			t.Fatalf("expected mclo.gs provider, got %s", cfg.Provider)
		}
		return stubUploader{
			result: UploadResult{Provider: string(ProviderMCLogs), URL: "https://mclo.gs/abc1234"},
			req:    &gotReq,
		}
	}

	if err := run(context.Background(), []string{path}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "https://mclo.gs/abc1234" {
		t.Fatalf("unexpected output: %q", got)
	}
	if gotReq.Source != "oaklog" {
		t.Fatalf("expected default source oaklog, got %q", gotReq.Source)
	}
}

func TestRunDefaultShortFlags(t *testing.T) {
	path := writeTempFile(t, "latest.log", "hello\n")
	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()

	var gotTimeout time.Duration
	var gotReq UploadRequest
	newUploader = func(cfg providerConfig) Uploader {
		gotTimeout = cfg.Timeout
		return stubUploader{
			result: UploadResult{Provider: string(ProviderMCLogs), URL: "https://mclo.gs/abc1234"},
			req:    &gotReq,
		}
	}

	if err := run(context.Background(), []string{"-j", "-s", "varda", "-t", "10s", path}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if gotTimeout != 10*time.Second {
		t.Fatalf("expected timeout 10s, got %s", gotTimeout)
	}
	if gotReq.Source != "varda" {
		t.Fatalf("expected source varda, got %q", gotReq.Source)
	}
	if !strings.Contains(out.String(), `"provider": "mclogs"`) {
		t.Fatalf("expected json output, got %q", out.String())
	}
}

func TestRunMCLLogsSubcommand(t *testing.T) {
	path := writeTempFile(t, "latest.log", "hello\n")
	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()

	var gotReq UploadRequest
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.Provider != ProviderMCLogs {
			t.Fatalf("expected mclo.gs provider, got %s", cfg.Provider)
		}
		return stubUploader{
			result: UploadResult{Provider: string(ProviderMCLogs), URL: "https://mclo.gs/abc1234"},
			req:    &gotReq,
		}
	}

	if err := run(context.Background(), []string{"mclogs", "--json", "--source", "varda", path}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if gotReq.Source != "varda" {
		t.Fatalf("expected source varda, got %q", gotReq.Source)
	}
	if !strings.Contains(out.String(), `"provider": "mclogs"`) {
		t.Fatalf("expected json output, got %q", out.String())
	}
}

func TestRunPastebinSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	t.Setenv("PASTEBIN_API", "test-key")

	var gotReq UploadRequest
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.Provider != ProviderPastebin {
			t.Fatalf("expected pastebin provider, got %s", cfg.Provider)
		}
		if cfg.PastebinAPI != "test-key" {
			t.Fatalf("expected API key from env, got %q", cfg.PastebinAPI)
		}
		if cfg.PastebinPrivate != pastebinVisibilityUnlisted {
			t.Fatalf("expected default unlisted visibility, got %q", cfg.PastebinPrivate)
		}
		return stubUploader{
			result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"},
			req:    &gotReq,
		}
	}

	if err := run(context.Background(), []string{"pastebin", "--source", "varda", "-"}, strings.NewReader("hello\n"), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if gotReq.Source != "varda" {
		t.Fatalf("expected source varda, got %q", gotReq.Source)
	}
	if gotReq.Content == nil || string(gotReq.Content) != "hello\n" {
		t.Fatalf("unexpected stdin content: %q", string(gotReq.Content))
	}
	if got := strings.TrimSpace(out.String()); got != "https://pastebin.com/UIFdu235s" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRunPastebinVisibilityFlags(t *testing.T) {
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	t.Setenv("PASTEBIN_API", "test-key")

	t.Run("default unlisted", func(t *testing.T) {
		var out, errOut bytes.Buffer
		old := newUploader
		defer func() { newUploader = old }()
		newUploader = func(cfg providerConfig) Uploader {
			if cfg.PastebinPrivate != pastebinVisibilityUnlisted {
				t.Fatalf("expected unlisted visibility, got %q", cfg.PastebinPrivate)
			}
			return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
		}
		if err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	})

	t.Run("public", func(t *testing.T) {
		var out, errOut bytes.Buffer
		old := newUploader
		defer func() { newUploader = old }()
		newUploader = func(cfg providerConfig) Uploader {
			if cfg.PastebinPrivate != pastebinVisibilityPublic {
				t.Fatalf("expected public visibility, got %q", cfg.PastebinPrivate)
			}
			return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
		}
		if err := run(context.Background(), []string{"pastebin", "--public", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	})

	t.Run("unlisted", func(t *testing.T) {
		var out, errOut bytes.Buffer
		old := newUploader
		defer func() { newUploader = old }()
		newUploader = func(cfg providerConfig) Uploader {
			if cfg.PastebinPrivate != pastebinVisibilityUnlisted {
				t.Fatalf("expected unlisted visibility, got %q", cfg.PastebinPrivate)
			}
			return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
		}
		if err := run(context.Background(), []string{"pastebin", "--unlisted", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		var out, errOut bytes.Buffer
		err := run(context.Background(), []string{"pastebin", "--public", "--unlisted", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), "--public and --unlisted cannot be used together") {
			t.Fatalf("expected visibility conflict error, got %v", err)
		}
	})
}

func TestRunPastebinAPIFlagWins(t *testing.T) {
	badUserConfig := writeTempFile(t, "bad-user.env", "PASTEBIN_API=\nthis is invalid dotenv syntax\n")
	badSystemConfig := writeTempFile(t, "bad-system.env", "PASTEBIN_API=\nthis is invalid dotenv syntax\n")
	setPastebinConfigPaths(t, badUserConfig, badSystemConfig)
	t.Setenv("PASTEBIN_API", "from-env")

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

func TestRunPastebinAPIFileWinsOverEnv(t *testing.T) {
	badUserConfig := writeTempFile(t, "bad-user.env", "not dotenv syntax [\n")
	badSystemConfig := writeTempFile(t, "bad-system.env", "not dotenv syntax [\n")
	setPastebinConfigPaths(t, badUserConfig, badSystemConfig)
	t.Setenv("PASTEBIN_API", "from-env")
	tokenFile := writeTempFile(t, "token.txt", "  from-file  \n")

	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.PastebinAPI != "from-file" {
			t.Fatalf("expected file token, got %q", cfg.PastebinAPI)
		}
		return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
	}

	if err := run(context.Background(), []string{"pastebin", "--pastebin-api-file", tokenFile, writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRunPastebinEnvWinsOverBrokenConfig(t *testing.T) {
	badUserConfig := writeTempFile(t, "bad-user.env", "not dotenv syntax [\n")
	systemConfig := writeTempFile(t, "system.env", "PASTEBIN_API=from-system\n")
	setPastebinConfigPaths(t, badUserConfig, systemConfig)
	t.Setenv("PASTEBIN_API", "from-env")

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

func TestRunPastebinUserConfigWorks(t *testing.T) {
	userConfig := writeTempFile(t, "user.env", "PASTEBIN_API=from-user\n")
	systemConfig := writeTempFile(t, "system.env", "PASTEBIN_API=from-system\n")
	setPastebinConfigPaths(t, userConfig, systemConfig)

	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.PastebinAPI != "from-user" {
			t.Fatalf("expected user config token, got %q", cfg.PastebinAPI)
		}
		return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
	}

	if err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRunPastebinMalformedUserConfigFailsWhenNeeded(t *testing.T) {
	badUserConfig := writeTempFile(t, "bad-user.env", "not dotenv syntax [\n")
	setPastebinConfigPaths(t, badUserConfig, filepath.Join(t.TempDir(), "missing-system.env"))

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Fatalf("expected user config parse error, got %v", err)
	}
}

func TestRunPastebinSystemConfigWorks(t *testing.T) {
	userConfig := filepath.Join(t.TempDir(), "missing-user.env")
	systemConfig := writeTempFile(t, "system.env", "PASTEBIN_API=from-system\n")
	setPastebinConfigPaths(t, userConfig, systemConfig)

	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.PastebinAPI != "from-system" {
			t.Fatalf("expected system config token, got %q", cfg.PastebinAPI)
		}
		return stubUploader{result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"}}
	}

	if err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRunPastebinMalformedSystemConfigFailsWhenNeeded(t *testing.T) {
	userConfig := filepath.Join(t.TempDir(), "missing-user.env")
	badSystemConfig := writeTempFile(t, "bad-system.env", "not dotenv syntax [\n")
	setPastebinConfigPaths(t, userConfig, badSystemConfig)

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Fatalf("expected system config parse error, got %v", err)
	}
}

func TestRunPastebinSkipsSystemConfigWhenDisabled(t *testing.T) {
	userConfig := filepath.Join(t.TempDir(), "missing-user.env")
	oldUser := userConfigPath
	oldSystem := systemConfigPath
	userConfigPath = func() (string, error) {
		return userConfig, nil
	}
	systemConfigPath = func() (string, bool) {
		return "", false
	}
	t.Cleanup(func() {
		userConfigPath = oldUser
		systemConfigPath = oldSystem
	})

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), pastebinAPIKeyError) {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestRunPastebinCLIWinsOverMalformedConfig(t *testing.T) {
	badUserConfig := writeTempFile(t, "bad-user.env", "not dotenv syntax [\n")
	systemConfig := writeTempFile(t, "system.env", "PASTEBIN_API=from-system\n")
	setPastebinConfigPaths(t, badUserConfig, systemConfig)

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

func TestRunPastebinAPIFileEmpty(t *testing.T) {
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	tokenFile := writeTempFile(t, "empty-token.txt", "   \n")

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", "--pastebin-api-file", tokenFile, writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--pastebin-api-file is empty") {
		t.Fatalf("expected empty token file error, got %v", err)
	}
}

func TestRunPastebinMissingTokenFailsCleanly(t *testing.T) {
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	unset := unsetEnv(t, "PASTEBIN_API")
	defer unset()

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), pastebinAPIKeyError) {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestRunPastebinHelpShowsNewFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"pastebin", "--help"}, bytes.NewReader(nil), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"--pastebin-api", "--pastebin-api-file", "--unlisted           create an unlisted paste (default)", "--public             create a public paste"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q, got: %q", want, got)
		}
	}
	if strings.Contains(got, "--config") {
		t.Fatalf("help should not mention --config, got: %q", got)
	}
}

func TestRunPastebinRejectsConfigFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"pastebin", "--config", "custom.env", writeTempFile(t, "latest.log", "hello\n")}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestRunRejectsNonPositiveTimeout(t *testing.T) {
	path := writeTempFile(t, "latest.log", "hello\n")
	tests := []struct {
		name string
		args []string
	}{
		{name: "default zero", args: []string{"--timeout", "0s", path}},
		{name: "default negative", args: []string{"--timeout", "-1s", path}},
		{name: "provider zero", args: []string{"mclogs", "--timeout", "0s", path}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			err := run(context.Background(), tt.args, bytes.NewReader(nil), &out, &errOut)
			if err == nil || !strings.Contains(err.Error(), "--timeout must be greater than 0") {
				t.Fatalf("expected timeout validation error, got %v", err)
			}
		})
	}
}

func TestRunTopLevelVisibilityFlagsError(t *testing.T) {
	tests := [][]string{
		{"--public", "latest.log"},
		{"--unlisted", "latest.log"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			err := run(context.Background(), args, bytes.NewReader(nil), &out, &errOut)
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("expected unknown flag error, got %v", err)
			}
		})
	}
}

func TestRunMCLLogsRejectsVisibilityFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"mclogs", "--public", "latest.log"}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -public") {
		t.Fatalf("expected unknown visibility flag error, got %v", err)
	}
}

func TestRunMCLLogsTruncatesBeforeUpload(t *testing.T) {
	content := buildLogLines(t, maxMCLogsLines+1)
	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()

	var gotReq UploadRequest
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.Provider != ProviderMCLogs {
			t.Fatalf("expected mclo.gs provider, got %s", cfg.Provider)
		}
		return stubUploader{
			result: UploadResult{Provider: string(ProviderMCLogs), URL: "https://mclo.gs/truncated"},
			req:    &gotReq,
		}
	}

	if err := run(context.Background(), []string{"mclogs", "-"}, bytes.NewReader(content), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.HasPrefix(string(gotReq.Content), mcLogsTruncationMarker) {
		t.Fatalf("expected truncation marker, got %q", string(gotReq.Content[:len(mcLogsTruncationMarker)]))
	}
	if !strings.HasPrefix(string(gotReq.Content), mcLogsTruncationMarker+"line 3\n") {
		t.Fatalf("expected marker on its own line and oldest lines removed, got prefix %q", string(gotReq.Content[:len(mcLogsTruncationMarker)+len("line 3\n")]))
	}
	if countLogLines(gotReq.Content) != maxMCLogsLines {
		t.Fatalf("expected %d lines, got %d", maxMCLogsLines, countLogLines(gotReq.Content))
	}
}

func TestRunPastebinDoesNotTruncateToMCLogsTail(t *testing.T) {
	setPastebinConfigPaths(t, filepath.Join(t.TempDir(), "missing-user"), filepath.Join(t.TempDir(), "missing-system"))
	t.Setenv("PASTEBIN_API", "test-key")
	content := buildLogLines(t, maxMCLogsLines+1)
	var out, errOut bytes.Buffer
	old := newUploader
	defer func() { newUploader = old }()

	var gotReq UploadRequest
	newUploader = func(cfg providerConfig) Uploader {
		if cfg.Provider != ProviderPastebin {
			t.Fatalf("expected pastebin provider, got %s", cfg.Provider)
		}
		return stubUploader{
			result: UploadResult{Provider: string(ProviderPastebin), URL: "https://pastebin.com/UIFdu235s", Raw: "https://pastebin.com/raw/UIFdu235s"},
			req:    &gotReq,
		}
	}

	if err := run(context.Background(), []string{"pastebin", "-"}, bytes.NewReader(content), &out, &errOut); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if strings.Contains(string(gotReq.Content), mcLogsTruncationMarker) {
		t.Fatalf("pastebin content should not include mclo.gs truncation marker")
	}
	if countLogLines(gotReq.Content) != maxMCLogsLines+1 {
		t.Fatalf("expected pastebin to keep all lines, got %d", countLogLines(gotReq.Content))
	}
}

func TestRunRejectsOversizeContentFile(t *testing.T) {
	path := writeTempFile(t, "big.log", strings.Repeat("a", maxLogSize+1))
	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{path}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "log file is larger than 10 MiB") {
		t.Fatalf("expected oversize content error, got %v", err)
	}
}

func TestRunRejectsInvalidSource(t *testing.T) {
	path := writeTempFile(t, "latest.log", "hello\n")

	var out, errOut bytes.Buffer
	err := run(context.Background(), []string{"--source", "   ", path}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--source cannot be empty") {
		t.Fatalf("expected empty source error, got %v", err)
	}

	err = run(context.Background(), []string{"mclogs", "--source", strings.Repeat("a", maxSourceLength+1), path}, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--source") || !strings.Contains(err.Error(), "64 characters or fewer") {
		t.Fatalf("expected long source error, got %v", err)
	}
}

func TestRunNoArgsReturnsError(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run(context.Background(), nil, bytes.NewReader(nil), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "exactly one log file path is required") {
		t.Fatalf("expected missing arg error, got %v", err)
	}
}

func TestReadLogFileValidation(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		_, err := readLogFile(filepath.Join(t.TempDir(), "missing.log"))
		if err == nil || !strings.Contains(err.Error(), "log file not found") {
			t.Fatalf("expected missing file error, got %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		_, err := readLogFile(dir)
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("expected directory error, got %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		path := writeTempFile(t, "empty.log", "")
		_, err := readLogFile(path)
		if err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("expected empty file error, got %v", err)
		}
	})

	t.Run("large", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "large.log")
		if err := os.WriteFile(path, bytes.Repeat([]byte("a"), maxLogSize+1), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := readLogFile(path)
		if err == nil || !strings.Contains(err.Error(), "larger than 10 MiB") {
			t.Fatalf("expected size error, got %v", err)
		}
	})

	t.Run("valid", func(t *testing.T) {
		path := writeTempFile(t, "ok.log", "line 1\n")
		content, err := readLogFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(content) != "line 1\n" {
			t.Fatalf("unexpected content")
		}
	})
}

func TestPrepareLogContentWithinLimit(t *testing.T) {
	content := []byte("line 1\nline 2\n")
	got := prepareLogContent(content)
	if !bytes.Equal(got, content) {
		t.Fatalf("expected content unchanged, got %q", string(got))
	}
}

func TestPrepareLogContentTruncatesTail(t *testing.T) {
	content := buildLogLines(t, maxMCLogsLines+1)
	got := prepareLogContent(content)

	if !strings.HasPrefix(string(got), mcLogsTruncationMarker) {
		t.Fatalf("expected truncation marker, got prefix %q", string(got[:len(mcLogsTruncationMarker)]))
	}
	if !strings.HasPrefix(string(got), mcLogsTruncationMarker+"line 3\n") {
		t.Fatalf("expected marker on its own line and oldest lines removed, got prefix %q", string(got[:len(mcLogsTruncationMarker)+len("line 3\n")]))
	}
	if strings.Contains(string(got), "line 1\n") || strings.Contains(string(got), "line 2\n") {
		t.Fatalf("expected oldest lines removed")
	}
	if !strings.Contains(string(got), fmt.Sprintf("line %d\n", maxMCLogsLines+1)) {
		t.Fatalf("expected newest line preserved")
	}
	if countLogLines(got) != maxMCLogsLines {
		t.Fatalf("expected %d lines after truncation, got %d", maxMCLogsLines, countLogLines(got))
	}
}

func TestPrepareLogContentTruncatesTailWithoutTrailingNewline(t *testing.T) {
	content := bytes.TrimSuffix(buildLogLines(t, maxMCLogsLines+1), []byte("\n"))
	got := prepareLogContent(content)

	if !strings.HasPrefix(string(got), mcLogsTruncationMarker+"line 3\n") {
		t.Fatalf("expected marker on its own line and oldest lines removed, got prefix %q", string(got[:len(mcLogsTruncationMarker)+len("line 3\n")]))
	}
	if !strings.HasSuffix(string(got), fmt.Sprintf("line %d", maxMCLogsLines+1)) {
		t.Fatalf("expected newest line preserved without added newline")
	}
	if countLogLines(got) != maxMCLogsLines {
		t.Fatalf("expected %d lines after truncation, got %d", maxMCLogsLines, countLogLines(got))
	}
}

func setPastebinConfigPaths(t *testing.T, userPath, systemPath string) {
	t.Helper()
	oldUser := userConfigPath
	oldSystem := systemConfigPath
	userConfigPath = func() (string, error) {
		return userPath, nil
	}
	systemConfigPath = func() (string, bool) {
		return systemPath, true
	}
	t.Cleanup(func() {
		userConfigPath = oldUser
		systemConfigPath = oldSystem
	})
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildLogLines(t *testing.T, lines int) []byte {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return []byte(b.String())
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

func chdirTempDir(t *testing.T) func() {
	t.Helper()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatal(err)
		}
	}
}
