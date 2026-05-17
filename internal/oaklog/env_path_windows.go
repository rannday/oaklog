//go:build windows

package oaklog

import (
	"errors"
	"os"
	"path/filepath"
)

func defaultUserConfigPath() (string, error) {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		return "", errors.New("USERPROFILE is not set")
	}
	return filepath.Join(home, ".config", "oaklog", "env"), nil
}

func defaultSystemConfigPath() (string, bool) {
	return "", false
}
