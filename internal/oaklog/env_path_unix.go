//go:build !windows

package oaklog

import (
	"os"
	"path/filepath"
)

func defaultUserConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "oaklog", "env"), nil
}

func defaultSystemConfigPath() (string, bool) {
	return filepath.Join(string(os.PathSeparator), "etc", "oaklog", "env"), true
}
