package configuration

import (
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	configurableDev     = "dev"
	configurableRelease = "release"
)

var (
	configurablePathsOnce sync.Once
	configurablePaths     map[string]string
)

// IsConfigurable reports whether a config path can be overridden by external
// sources in the current build mode. Paths use mapstructure names joined with
// dots, for example "auth.base_url" or "credentials.storage_dir".
//
// A field tagged configurable:"release" is configurable in all builds. A field
// tagged configurable:"dev" is configurable only in dev builds. Fields without
// a configurable tag are loaded from config files but cannot be changed through
// environment variables or global flags.
func IsConfigurable(path string) bool {
	mode, ok := allConfigurablePaths()[path]
	if !ok {
		return false
	}
	return mode == configurableRelease || (mode == configurableDev && IsDev)
}

// allConfigurablePaths returns every Config leaf field tagged with
// configurable. The result is cached because env loading and CLI flag
// registration both query this metadata during startup.
func allConfigurablePaths() map[string]string {
	configurablePathsOnce.Do(func() {
		paths := make(map[string]string)
		_ = walkConfigType(reflect.TypeOf(Config{}), "", func(path string, field reflect.StructField) error {
			mode := field.Tag.Get("configurable")
			if mode != "" {
				paths[path] = mode
			}
			return nil
		})
		configurablePaths = paths
	})
	return configurablePaths
}

// walkConfigFields walks a concrete Config value and visits each leaf field
// with its dot-delimited config path. It recurses through nested config structs
// and treats time.Duration as a scalar leaf so callers can parse and assign
// duration values directly.
func walkConfigFields(value reflect.Value, prefix string, visit func(path string, value reflect.Value) error) error {
	typeOfValue := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := typeOfValue.Field(i)
		path := configPath(prefix, field)
		if path == "" {
			continue
		}
		fieldValue := value.Field(i)
		if fieldValue.Kind() == reflect.Struct && fieldValue.Type() != reflect.TypeOf(time.Duration(0)) {
			if err := walkConfigFields(fieldValue, path, visit); err != nil {
				return err
			}
			continue
		}
		if err := visit(path, fieldValue); err != nil {
			return err
		}
	}
	return nil
}

// walkConfigType walks the Config type and visits each leaf struct field with
// its dot-delimited config path. It is used to read mapstructure/configurable
// tags without needing a Config value.
func walkConfigType(typeOfValue reflect.Type, prefix string, visit func(path string, field reflect.StructField) error) error {
	for i := 0; i < typeOfValue.NumField(); i++ {
		field := typeOfValue.Field(i)
		path := configPath(prefix, field)
		if path == "" {
			continue
		}
		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0)) {
			if err := walkConfigType(field.Type, path, visit); err != nil {
				return err
			}
			continue
		}
		if err := visit(path, field); err != nil {
			return err
		}
	}
	return nil
}

// configPath derives a dot-delimited config path segment from the field's
// mapstructure tag. Falling back to the lowercase Go field name keeps the
// helper usable for simple fields that do not need explicit mapstructure tags.
func configPath(prefix string, field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
	if name == "" || name == "-" {
		name = strings.ToLower(field.Name)
	}
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
