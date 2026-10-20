package cliupdate

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/tollbit/cli/internal/client/versionpolicy"
)

type fakePolicyClient struct {
	policy versionpolicy.Policy
	err    error
	calls  int
}

func (f *fakePolicyClient) Get(context.Context) (versionpolicy.Policy, error) {
	f.calls++
	return f.policy, f.err
}

func newTestChecker(t *testing.T, client *fakePolicyClient) (Checker, policyCache) {
	t.Helper()
	dir := t.TempDir()
	h, _ := newTestHandler(t)
	return NewChecker(dir, client, time.Hour, h), newPolicyCache(dir)
}

func TestCheckerMissingCacheFetchesAndSaves(t *testing.T) {
	client := &fakePolicyClient{policy: versionpolicy.Policy{Minimum: versionBelowCurrent, Latest: "9.9.9"}}
	c, cache := newTestChecker(t, client)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if client.calls != 1 {
		t.Fatalf("expected one policy request, got %d", client.calls)
	}
	saved, ok, err := cache.load(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected a saved policy, got exists=%t err=%v", ok, err)
	}
	if saved.Minimum != versionBelowCurrent || saved.Latest != "9.9.9" || saved.CheckedAt.IsZero() {
		t.Fatalf("unexpected saved policy %#v", saved)
	}
}

func TestCheckerFreshCacheSkipsRequest(t *testing.T) {
	client := &fakePolicyClient{}
	c, cache := newTestChecker(t, client)
	saveCached(t, cache, cachedPolicy{CheckedAt: time.Now().Add(-time.Minute), Minimum: versionAbove(t)})

	err := c.Run(context.Background())

	if client.calls != 0 {
		t.Fatalf("expected no policy request, got %d", client.calls)
	}
	var required RequiredError
	if !errors.As(err, &required) {
		t.Fatalf("expected the cached minimum to apply, got %v", err)
	}
}

func TestCheckerStaleCacheRefreshes(t *testing.T) {
	client := &fakePolicyClient{policy: versionpolicy.Policy{Minimum: versionBelowCurrent}}
	c, cache := newTestChecker(t, client)
	stale := time.Now().Add(-2 * time.Hour).UTC()
	saveCached(t, cache, cachedPolicy{CheckedAt: stale, Minimum: versionAbove(t)})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("expected the refreshed policy to apply, got %v", err)
	}

	if client.calls != 1 {
		t.Fatalf("expected one policy request, got %d", client.calls)
	}
	saved, _, _ := cache.load(context.Background())
	if saved.Minimum != versionBelowCurrent || !saved.CheckedAt.After(stale) {
		t.Fatalf("expected a rewritten cache, got %#v", saved)
	}
}

func TestCheckerRequestFailureDoesNotBlock(t *testing.T) {
	client := &fakePolicyClient{err: errors.New("unreachable")}
	c, cache := newTestChecker(t, client)

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if _, ok, _ := cache.load(context.Background()); ok {
		t.Fatal("expected no cache after a failed request")
	}
}

func TestCheckerUnreadableCacheRefetches(t *testing.T) {
	client := &fakePolicyClient{}
	c, cache := newTestChecker(t, client)
	if err := os.WriteFile(cache.path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if client.calls != 1 {
		t.Fatalf("expected one policy request, got %d", client.calls)
	}
}

func saveCached(t *testing.T, cache policyCache, cached cachedPolicy) {
	t.Helper()
	if err := cache.save(context.Background(), cached); err != nil {
		t.Fatal(err)
	}
}
