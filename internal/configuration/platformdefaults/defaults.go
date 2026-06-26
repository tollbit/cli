package platformdefaults

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultSentinel = "__default__"

func CredentialsStorageDir(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && value != DefaultSentinel {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tollbit"), nil
}
