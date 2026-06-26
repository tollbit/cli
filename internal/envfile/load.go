package envfile

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

const EnvFilePathVar = "TOLLBIT_ENV_FILE"

// LoadDefault reads TOLLBIT_ENV_FILE or .env in the current working directory.
// Variables already set in the process environment are not overwritten.
func LoadDefault() error {
	if path := strings.TrimSpace(os.Getenv(EnvFilePathVar)); path != "" {
		return Load(path)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	return Load(filepath.Join(wd, ".env"))
}

// Load parses a dotenv file and sets variables that are not already set.
func Load(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if err := applyLine(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func applyLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	if after, ok := strings.CutPrefix(line, "export "); ok {
		line = strings.TrimSpace(after)
	}
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if os.Getenv(key) != "" {
		return nil
	}
	value = strings.TrimSpace(value)
	return os.Setenv(key, unquote(value))
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		return strings.ReplaceAll(value[1:len(value)-1], `\"`, `"`)
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}
