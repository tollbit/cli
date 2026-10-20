package app

import (
	"io"

	"github.com/tollbit/cli/internal/configuration"
)

type Option func(*options)

type Factory struct {
	Config configuration.Config
	// Process is the run's shared state, handed to every App the factory
	// builds. Nil means no update check and no update notices.
	Process *Process
	Options []Option
}

// NewFactory returns the run's Factory, with a new Process whose update
// notices go to stderr.
func NewFactory(config configuration.Config, stderr io.Writer, opts ...Option) Factory {
	return Factory{
		Config:  config,
		Process: NewProcess(config, stderr),
		Options: opts,
	}
}

func (f Factory) New(overrides configuration.OverrideOptions) (*App, error) {
	config, err := f.Config.WithOverrides(overrides)
	if err != nil {
		return nil, err
	}
	return New(config, append([]Option{withProcess(f.Process)}, f.Options...)...)
}

type options struct {
	dependencies Dependencies
	process      *Process
}

func OverrideDependencies(deps Dependencies) Option {
	return func(opts *options) {
		opts.dependencies = deps
	}
}

func withProcess(process *Process) Option {
	return func(opts *options) {
		opts.process = process
	}
}
