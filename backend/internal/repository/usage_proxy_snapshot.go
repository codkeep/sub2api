package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"database/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	gocache "github.com/patrickmn/go-cache"
)

func usageProxySnapshotJSON(snapshot *service.UsageProxySnapshot) any {
	if snapshot == nil {
		return nil
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}
	return string(encoded)
}

func decodeUsageProxySnapshot(value sql.NullString) *service.UsageProxySnapshot {
	if !value.Valid {
		return nil
	}
	var snapshot *service.UsageProxySnapshot
	if json.Unmarshal([]byte(value.String), &snapshot) != nil {
		return nil
	}
	return snapshot
}

// NewUsageLogRepositoryWithProxySnapshot enriches new usage records, never
// historical reads. Probes run in the usage-record worker, off the response path.
func NewUsageLogRepositoryWithProxySnapshot(client *dbent.Client, db *sql.DB, prober service.ProxyExitInfoProber) service.UsageLogRepository {
	r := newUsageLogRepositoryWithSQL(client, db)
	r.proxyExitProber = prober
	r.proxyExitCache = gocache.New(30*time.Second, time.Minute)
	return r
}

func (r *usageLogRepository) enrichProxySnapshot(ctx context.Context, log *service.UsageLog) {
	if log == nil || log.ProxySnapshot == nil || log.ProxySnapshot.ID == 0 || log.ProxySnapshot.ExitIP != "" || log.ProxySnapshot.URL == "" || r.proxyExitProber == nil {
		return
	}
	snapshot := *log.ProxySnapshot
	// Hash includes credentials to isolate rotating/sticky proxy sessions, without
	// storing credential-bearing URLs in cache keys or persisted JSON.
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(snapshot.URL)))
	var observed *service.UsageProxySnapshot
	if cached, ok := r.proxyExitCache.Get(key); ok {
		observed = cached.(*service.UsageProxySnapshot)
	} else {
		result := r.proxyExitGroup.DoChan(key, func() (any, error) {
			if cached, ok := r.proxyExitCache.Get(key); ok {
				return cached, nil
			}
			probeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			info, _, err := r.proxyExitProber.ProbeProxy(probeCtx, snapshot.URL)
			observation := &service.UsageProxySnapshot{}
			ttl := 5 * time.Second
			if err == nil && info != nil && net.ParseIP(info.IP) != nil {
				now := time.Now().UTC()
				observation.ExitIP = info.IP
				observation.ExitIPCheckedAt = &now
				ttl = 30 * time.Second
			}
			r.proxyExitCache.Set(key, observation, ttl)
			return observation, nil
		})
		select {
		case res := <-result:
			if res.Err == nil {
				observed, _ = res.Val.(*service.UsageProxySnapshot)
			}
		case <-ctx.Done():
			return
		}
	}
	if observed != nil {
		snapshot.ExitIP, snapshot.ExitIPCheckedAt = observed.ExitIP, observed.ExitIPCheckedAt
		log.ProxySnapshot = &snapshot
	}
}
