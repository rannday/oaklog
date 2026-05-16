package oaklog

import (
	"errors"
	"os"
	"strings"

	goenv "github.com/rannday/go-env"
)

const pastebinAPIKeyError = "PASTEBIN_API is required when using --pastebin"

type pastebinEnvConfig struct {
	APIKey string `env:"PASTEBIN_API" required:"true"`
}

func loadPastebinAPIKey() (string, error) {
	var cfg pastebinEnvConfig
	opts := goenv.Options{}
	if _, err := os.Stat(".env"); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
	} else {
		opts.DotEnvPath = ".env"
	}

	if err := goenv.Load(&cfg, opts); err != nil {
		if strings.Contains(err.Error(), "PASTEBIN_API") {
			return "", errors.New(pastebinAPIKeyError)
		}
		return "", err
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return "", errors.New(pastebinAPIKeyError)
	}
	return apiKey, nil
}
