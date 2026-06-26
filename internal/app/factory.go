package app

import (
	"github.com/tollbit/tollbit-cli/internal/configuration"
)

type Option func(*options)

type Factory struct {
	Config  configuration.Config
	Options []Option
}

func (f Factory) New(overrides configuration.OverrideOptions) (*App, error) {
	config, err := f.Config.WithOverrides(overrides)
	if err != nil {
		return nil, err
	}
	return New(config, f.Options...)
}

type options struct {
	dependencies Dependencies
}

func OverrideDependencies(deps Dependencies) Option {
	return func(opts *options) {
		opts.dependencies = deps
	}
}
