package configuration

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func applyEnv(config *Config) error {
	return walkConfigFields(reflect.ValueOf(config).Elem(), "", func(path string, value reflect.Value) error {
		if !IsConfigurable(path) {
			return nil
		}
		name := envName(path)
		raw, ok := os.LookupEnv(name)
		if !ok {
			return nil
		}
		return setValueFromString(value, name, strings.TrimSpace(raw))
	})
}

func setValueFromString(value reflect.Value, name string, raw string) error {
	if value.Type() == reflect.TypeOf(time.Duration(0)) {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		value.SetInt(int64(parsed))
		return nil
	}
	switch value.Kind() {
	case reflect.String:
		value.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		value.SetBool(parsed)
	case reflect.Int32:
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		value.SetInt(parsed)
	default:
		return fmt.Errorf("read %s: unsupported config type %s", name, value.Type())
	}
	return nil
}

func envName(path string) string {
	name := strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
	return envPrefix + "_" + name
}
