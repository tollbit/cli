package cliupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tollbit/cli/internal/statefile"
)

const policyCacheFilename = "update-check.json"

type (
	// policyCache is the last version policy the CLI fetched, kept in
	// update-check.json so the check hits the network at most once per
	// interval.
	policyCache struct {
		path string
	}

	cachedPolicy struct {
		CheckedAt time.Time `json:"checked_at"`
		Minimum   string    `json:"minimum"`
		Latest    string    `json:"latest"`
	}
)

func newPolicyCache(dir string) policyCache {
	return policyCache{path: filepath.Join(dir, policyCacheFilename)}
}

// load returns the cached policy and whether one was saved.
func (c policyCache) load(ctx context.Context) (cachedPolicy, bool, error) {
	if err := ctx.Err(); err != nil {
		return cachedPolicy{}, false, err
	}
	raw, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return cachedPolicy{}, false, nil
	}
	if err != nil {
		return cachedPolicy{}, false, fmt.Errorf("read version policy cache: %w", err)
	}
	var cached cachedPolicy
	if err := json.Unmarshal(raw, &cached); err != nil {
		return cachedPolicy{}, false, fmt.Errorf("parse version policy cache: %w", err)
	}
	return cached, true, nil
}

func (c policyCache) save(ctx context.Context, cached cachedPolicy) error {
	b, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	return statefile.WriteAtomic(ctx, c.path, append(b, '\n'))
}
