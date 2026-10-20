package cliupdate

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/client/versionpolicy"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
)

// Checker refreshes the cached version policy at most once per interval and
// hands it to the Handler, which installs the update below the minimum when
// it may. Cache and network problems are logged at debug level and never fail
// the check; only the Handler's verdict can.
type Checker struct {
	cache    policyCache
	policy   versionpolicy.Client
	interval time.Duration
	handler  *Handler
}

// NewChecker returns a Checker that caches the policy in cacheDir.
func NewChecker(cacheDir string, policy versionpolicy.Client, interval time.Duration, handler *Handler) Checker {
	return Checker{
		cache:    newPolicyCache(cacheDir),
		policy:   policy,
		interval: interval,
		handler:  handler,
	}
}

func (c Checker) Run(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	cached, ok, err := c.cache.load(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("update check: ignoring unreadable cache")
		ok = false
	}
	if !ok || stale(cached.CheckedAt, c.interval) {
		fetched, err := c.fetch(ctx)
		if err != nil {
			log.Debug().Err(err).Msg("update check: version policy request failed")
		} else {
			cached, ok = fetched, true
		}
	}
	if !ok {
		return nil
	}
	err = c.handler.HandleVersionPolicy(versionpolicy.Policy{Minimum: cached.Minimum, Latest: cached.Latest})
	var required RequiredError
	if errors.As(err, &required) {
		return c.handler.installRequired(ctx, required)
	}
	return err
}

// RecordUpdateRequired saves the versions a gateway's 426
// cli_update_required response reports as the cached policy, so the next
// command's check acts on them without waiting for the interval. Other errors
// are ignored.
func (c Checker) RecordUpdateRequired(ctx context.Context, err error) {
	var problem problemjson.Problem
	if !errors.As(err, &problem) || !problem.IsCLIUpdateRequired() {
		return
	}
	minimum, _ := problem.StringProperty("minimumVersion")
	if minimum == "" {
		return
	}
	latest, _ := problem.StringProperty("latestVersion")
	cached := cachedPolicy{CheckedAt: time.Now().UTC(), Minimum: minimum, Latest: latest}
	if err := c.cache.save(ctx, cached); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("update check: could not save cache")
	}
}

func (c Checker) fetch(ctx context.Context) (cachedPolicy, error) {
	policy, err := c.policy.Get(ctx)
	if err != nil {
		return cachedPolicy{}, err
	}
	cached := cachedPolicy{
		CheckedAt: time.Now().UTC(),
		Minimum:   policy.Minimum,
		Latest:    policy.Latest,
	}
	if err := c.cache.save(ctx, cached); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("update check: could not save cache")
	}
	return cached, nil
}

// stale treats a checked_at in the future as stale so a clock change cannot
// pin an old policy.
func stale(checkedAt time.Time, interval time.Duration) bool {
	age := time.Since(checkedAt)
	return age < 0 || age >= interval
}
