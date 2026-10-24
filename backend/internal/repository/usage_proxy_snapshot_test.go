package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

type usageExitProber struct {
	calls atomic.Int32
	fail  bool
}

func (p *usageExitProber) ProbeProxy(_ context.Context, _ string) (*service.ProxyExitInfo, int64, error) {
	p.calls.Add(1)
	if p.fail {
		return nil, 0, errors.New("probe unavailable")
	}
	return &service.ProxyExitInfo{IP: "203.0.113.7"}, 1, nil
}

func TestUsageProxySnapshotPersistenceAndProbeIsolation(t *testing.T) {
	prober := &usageExitProber{}
	r := &usageLogRepository{proxyExitProber: prober, proxyExitCache: gocache.New(time.Minute, time.Minute)}
	makeLog := func(name, url string) *service.UsageLog {
		return &service.UsageLog{ProxySnapshot: &service.UsageProxySnapshot{ID: 42, Name: name, URL: url}}
	}
	first := makeLog("original", "http://user:secret@proxy.example:8080")
	r.enrichProxySnapshot(context.Background(), first)
	require.Equal(t, "203.0.113.7", first.ProxySnapshot.ExitIP)
	require.NotNil(t, first.ProxySnapshot.ExitIPCheckedAt)
	second := makeLog("renamed", first.ProxySnapshot.URL)
	r.enrichProxySnapshot(context.Background(), second)
	require.EqualValues(t, 1, prober.calls.Load())
	require.Equal(t, "original", first.ProxySnapshot.Name)
	require.Equal(t, "renamed", second.ProxySnapshot.Name)
	require.Equal(t, first.ProxySnapshot.ExitIPCheckedAt, second.ProxySnapshot.ExitIPCheckedAt)
	third := makeLog("other session", "http://other:secret@proxy.example:8080")
	r.enrichProxySnapshot(context.Background(), third)
	require.EqualValues(t, 2, prober.calls.Load())
	encoded := usageProxySnapshotJSON(first.ProxySnapshot).(string)
	require.NotContains(t, encoded, "secret")
	require.NotContains(t, encoded, "proxy.example")
	loaded := decodeUsageProxySnapshot(sql.NullString{Valid: true, String: encoded})
	require.Equal(t, "original", loaded.Name)
	require.Equal(t, "203.0.113.7", loaded.ExitIP)
	require.Nil(t, decodeUsageProxySnapshot(sql.NullString{}))
	require.Nil(t, usageProxySnapshotJSON(nil))
	require.Equal(t, "{}", usageProxySnapshotJSON(&service.UsageProxySnapshot{}))
}

func TestUsageProxyProbeFailureRetainsIdentityAndIsCached(t *testing.T) {
	prober := &usageExitProber{fail: true}
	r := &usageLogRepository{proxyExitProber: prober, proxyExitCache: gocache.New(time.Minute, time.Minute)}
	for range 2 {
		log := &service.UsageLog{ProxySnapshot: &service.UsageProxySnapshot{ID: 1, Name: "proxy", URL: "http://proxy.example:8080"}}
		r.enrichProxySnapshot(context.Background(), log)
		require.Equal(t, "proxy", log.ProxySnapshot.Name)
		require.Empty(t, log.ProxySnapshot.ExitIP)
		require.Nil(t, log.ProxySnapshot.ExitIPCheckedAt)
	}
	require.EqualValues(t, 1, prober.calls.Load())
}
