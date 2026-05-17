package oaklog

import (
	"errors"
	"fmt"
	"os"
	"strings"

	goenv "github.com/rannday/go-env"
)

const pastebinAPIKeyError = "PASTEBIN_API is required when using oaklog pastebin"

type pastebinEnvConfig struct {
	APIKey string `env:"PASTEBIN_API"`
}

var userConfigPath = defaultUserConfigPath
var systemConfigPath = defaultSystemConfigPath

func resolvePastebinAPIKey(opts cliOptions) (string, error) {
	if key := strings.TrimSpace(opts.PastebinAPI); key != "" {
		return key, nil
	}

	if key, err := readPastebinAPIKeyFile(opts.PastebinAPIFile); err != nil {
		return "", err
	} else if key != "" {
		return key, nil
	}

	if key, ok := lookupPastebinAPIKeyEnv(); ok {
		return key, nil
	}

	path, err := userConfigPath()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config path: %w", err)
	}
	if key, err := readPastebinAPIKeyFromDotenv(path); err != nil {
		return "", err
	} else if key != "" {
		return key, nil
	}

	if path, ok := systemConfigPath(); ok {
		if key, err := readPastebinAPIKeyFromDotenv(path); err != nil {
			return "", err
		} else if key != "" {
			return key, nil
		}
	}

	for _, path := range devConfigPaths() {
		if key, err := readPastebinAPIKeyFromDotenv(path); err != nil {
			return "", err
		} else if key != "" {
			return key, nil
		}
	}

	return "", errors.New(pastebinAPIKeyError)
}

func lookupPastebinAPIKeyEnv() (string, bool) {
	value, ok := os.LookupEnv("PASTEBIN_API")
	if !ok {
		return "", false
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	return value, true
}

func readPastebinAPIKeyFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read --pastebin-api-file %s: %w", path, err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", errors.New("--pastebin-api-file is empty")
	}

	return key, nil
}

func readPastebinAPIKeyFromDotenv(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("config file is not a regular file: %s", path)
	}

	var cfg pastebinEnvConfig
	if err := goenv.Load(&cfg, goenv.Options{DotEnvPath: path}); err != nil {
		return "", fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		return "", fmt.Errorf("config file %s does not contain PASTEBIN_API", path)
	}

	return key, nil
}
