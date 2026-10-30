package cliruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/statefile"
)

const (
	EndUserProximityLocal  EndUserProximity = "local"
	EndUserProximityRemote EndUserProximity = "remote"

	EndUserProximitySourceConfigured        EndUserProximitySource = "configured"
	EndUserProximitySourceSavedRuntimeState EndUserProximitySource = "saved_runtime_state"
	EndUserProximitySourceAutoDetect        EndUserProximitySource = "auto_detect"

	stateFilename = "runtime.json"
)

type (
	EndUserProximity       string
	EndUserProximitySource string

	Config struct {
		ConfiguredEndUserProximity string
		StateDir                   string
	}

	Runtime struct {
		configuredEndUserProximity string
		stateDir                   string
		statePath                  string
		goos                       string
		getenv                     func(string) string
	}

	Status struct {
		ConfiguredEndUserProximity string                 `json:"configured_end_user_proximity"`
		SavedEndUserProximity      EndUserProximity       `json:"saved_end_user_proximity,omitempty"`
		EndUserProximity           EndUserProximity       `json:"end_user_proximity"`
		EndUserProximitySource     EndUserProximitySource `json:"end_user_proximity_source"`
		StateDir                   string                 `json:"state_dir"`
		StatePath                  string                 `json:"state_path"`
		StateExists                bool                   `json:"state_exists"`
	}

	state struct {
		EndUserProximity EndUserProximity `json:"end_user_proximity"`
		UpdatedAt        string           `json:"updated_at"`
	}
)

func New(cfg Config) (*Runtime, error) {
	configuredEndUserProximity := strings.TrimSpace(cfg.ConfiguredEndUserProximity)
	if configuredEndUserProximity == "" {
		configuredEndUserProximity = configuration.RuntimeEndUserProximityAutoDetect
	}
	stateDir := strings.TrimSpace(cfg.StateDir)
	if stateDir == "" {
		return nil, errors.New("runtime state dir is required")
	}
	return &Runtime{
		configuredEndUserProximity: configuredEndUserProximity,
		stateDir:                   stateDir,
		statePath:                  filepath.Join(stateDir, stateFilename),
		goos:                       runtime.GOOS,
		getenv:                     os.Getenv,
	}, nil
}

func (r *Runtime) EndUserProximity(ctx context.Context) (EndUserProximity, error) {
	switch r.configuredEndUserProximity {
	case configuration.RuntimeEndUserProximityLocal:
		return EndUserProximityLocal, nil
	case configuration.RuntimeEndUserProximityRemote:
		return EndUserProximityRemote, nil
	case configuration.RuntimeEndUserProximityAutoDetect:
		st, exists, err := r.readState(ctx)
		if err != nil {
			return "", err
		}
		if exists {
			return st.EndUserProximity, nil
		}
		return r.detectEndUserProximity(ctx), nil
	default:
		return "", fmt.Errorf("runtime.end_user_proximity must be %q, %q, or %q", configuration.RuntimeEndUserProximityAutoDetect, configuration.RuntimeEndUserProximityLocal, configuration.RuntimeEndUserProximityRemote)
	}
}

func (r *Runtime) SetEndUserProximity(ctx context.Context, proximity EndUserProximity) error {
	if err := validateEndUserProximity(proximity); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(r.stateDir, 0o700); err != nil {
		return fmt.Errorf("create runtime state directory: %w", err)
	}
	b, err := json.MarshalIndent(state{
		EndUserProximity: proximity,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return statefile.WriteAtomic(ctx, r.statePath, b)
}

func (r *Runtime) Status(ctx context.Context) (Status, error) {
	st, exists, err := r.readState(ctx)
	if err != nil {
		return Status{}, err
	}
	var proximity EndUserProximity
	var source EndUserProximitySource
	switch r.configuredEndUserProximity {
	case configuration.RuntimeEndUserProximityLocal:
		proximity = EndUserProximityLocal
		source = EndUserProximitySourceConfigured
	case configuration.RuntimeEndUserProximityRemote:
		proximity = EndUserProximityRemote
		source = EndUserProximitySourceConfigured
	case configuration.RuntimeEndUserProximityAutoDetect:
		if exists {
			proximity = st.EndUserProximity
			source = EndUserProximitySourceSavedRuntimeState
		} else {
			proximity = r.detectEndUserProximity(ctx)
			source = EndUserProximitySourceAutoDetect
		}
	default:
		return Status{}, fmt.Errorf("runtime.end_user_proximity must be %q, %q, or %q", configuration.RuntimeEndUserProximityAutoDetect, configuration.RuntimeEndUserProximityLocal, configuration.RuntimeEndUserProximityRemote)
	}
	savedProximity := EndUserProximity("")
	if exists {
		savedProximity = st.EndUserProximity
	}
	return Status{
		ConfiguredEndUserProximity: r.configuredEndUserProximity,
		SavedEndUserProximity:      savedProximity,
		EndUserProximity:           proximity,
		EndUserProximitySource:     source,
		StateDir:                   r.stateDir,
		StatePath:                  r.statePath,
		StateExists:                exists,
	}, nil
}

func (r *Runtime) StatePath() string {
	return r.statePath
}

func (r *Runtime) readState(ctx context.Context) (state, bool, error) {
	if err := ctx.Err(); err != nil {
		return state{}, false, err
	}
	raw, err := os.ReadFile(r.statePath)
	if os.IsNotExist(err) {
		return state{}, false, nil
	}
	if err != nil {
		return state{}, false, fmt.Errorf("read runtime state: %w", err)
	}
	var st state
	if err := json.Unmarshal(raw, &st); err != nil {
		return state{}, false, fmt.Errorf("parse runtime state: %w", err)
	}
	if err := validateEndUserProximity(st.EndUserProximity); err != nil {
		return state{}, false, fmt.Errorf("runtime state is invalid: %w", err)
	}
	return st, true, nil
}

func validateEndUserProximity(proximity EndUserProximity) error {
	switch proximity {
	case EndUserProximityLocal, EndUserProximityRemote:
		return nil
	default:
		return fmt.Errorf("runtime end-user proximity must be %q or %q", EndUserProximityLocal, EndUserProximityRemote)
	}
}
