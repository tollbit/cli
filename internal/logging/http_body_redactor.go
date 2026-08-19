package logging

import (
	"encoding/json"
	"strings"
)

const RedactedValue = "[REDACTED]"

type (
	JSONFieldRedactor func(json.RawMessage) json.RawMessage

	HTTPBodyRedactorConfig struct {
		FullyRedactedPaths []string
		JSONFields         map[string]JSONFieldRedactor
		MaxLength          int
	}

	HTTPBodyRedactor struct {
		fullyRedactedPaths map[string]struct{}
		jsonFields         map[string]JSONFieldRedactor
		maxLength          int
	}
)

func NewHTTPBodyRedactor(cfg HTTPBodyRedactorConfig) *HTTPBodyRedactor {
	paths := make(map[string]struct{}, len(cfg.FullyRedactedPaths))
	for _, path := range cfg.FullyRedactedPaths {
		paths[path] = struct{}{}
	}
	fields := make(map[string]JSONFieldRedactor, len(cfg.JSONFields))
	for field, redactor := range cfg.JSONFields {
		fields[field] = redactor
	}
	maxLength := cfg.MaxLength
	if maxLength <= 0 {
		maxLength = 2048
	}
	return &HTTPBodyRedactor{
		fullyRedactedPaths: paths,
		jsonFields:         fields,
		maxLength:          maxLength,
	}
}

func (r *HTTPBodyRedactor) Redact(path string, body []byte) string {
	value := strings.TrimSpace(string(body))
	if value == "" {
		return ""
	}
	if _, ok := r.fullyRedactedPaths[path]; ok {
		return RedactedValue
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil {
		if len(r.jsonFields) > 0 {
			return RedactedValue
		}
		return r.truncate(value)
	}
	for field, redact := range r.jsonFields {
		if raw, ok := object[field]; ok {
			object[field] = redact(raw)
		}
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return RedactedValue
	}
	return r.truncate(string(encoded))
}

func RedactJSONField(json.RawMessage) json.RawMessage {
	return json.RawMessage(`"[REDACTED]"`)
}

func AbbreviateJSONSecret(prefixLength, suffixLength int) JSONFieldRedactor {
	return func(raw json.RawMessage) json.RawMessage {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return RedactJSONField(raw)
		}
		if prefixLength < 0 || suffixLength < 0 || len(value) <= prefixLength+suffixLength {
			return RedactJSONField(raw)
		}
		encoded, err := json.Marshal(value[:prefixLength] + "..." + value[len(value)-suffixLength:])
		if err != nil {
			return RedactJSONField(raw)
		}
		return encoded
	}
}

func (r *HTTPBodyRedactor) truncate(value string) string {
	if len(value) <= r.maxLength {
		return value
	}
	return value[:r.maxLength] + "..."
}
