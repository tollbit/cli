package envfile

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

const EnvFilePathVar = "TOLLBIT_ENV_FILE"

// keyPrefix limits dotenv files to the CLI's own namespace so a loaded file
// cannot inject unrelated process variables (PATH, AWS_*, ...).
const keyPrefix = "TOLLBIT_"

// LoadDefault loads the dotenv file named by TOLLBIT_ENV_FILE, if set. It does
// not auto-discover a .env in the current working directory: loading is
// explicit so a stray .env in an unrelated directory can never take effect.
// Variables already set in the process environment are not overwritten.
func LoadDefault() error {
	path := strings.TrimSpace(os.Getenv(EnvFilePathVar))
	if path == "" {
		return nil
	}
	return Load(path)
}

// Load parses a dotenv file and sets TOLLBIT_-prefixed variables that are not
// already set. Non-TOLLBIT_ keys are ignored. A missing file is a no-op.
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
	if !strings.HasPrefix(key, keyPrefix) {
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
