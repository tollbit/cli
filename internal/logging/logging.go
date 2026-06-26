package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

const (
	LevelEnvVar  = "TOLLBIT_LOG_LEVEL"
	OutputEnvVar = "TOLLBIT_LOG_OUTPUT"
)

func NewContext(ctx context.Context, stderr io.Writer) (context.Context, func() error, error) {
	levelValue := strings.TrimSpace(os.Getenv(LevelEnvVar))
	if levelValue == "" || strings.EqualFold(levelValue, "disabled") {
		return zerolog.Nop().WithContext(ctx), noopCleanup, nil
	}

	level, err := zerolog.ParseLevel(levelValue)
	if err != nil {
		return ctx, nil, fmt.Errorf("invalid %s %q: %w", LevelEnvVar, levelValue, err)
	}

	writer, cleanup, err := logWriter(strings.TrimSpace(os.Getenv(OutputEnvVar)), stderr)
	if err != nil {
		return ctx, nil, err
	}
	logger := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	return logger.WithContext(ctx), cleanup, nil
}

func logWriter(output string, stderr io.Writer) (io.Writer, func() error, error) {
	if output == "" || strings.EqualFold(output, "stderr") {
		return stderr, noopCleanup, nil
	}
	f, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log output: %w", err)
	}
	return f, f.Close, nil
}

func noopCleanup() error {
	return nil
}
