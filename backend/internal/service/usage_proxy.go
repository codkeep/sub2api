package service

import "time"

// UsageProxySnapshot never contains proxy credentials. ExitIP is an independent
// proxy probe observation, not proof of the address used by a rotating proxy
// for the upstream model request.
type UsageProxySnapshot struct {
	ID              int64      `json:"id,omitempty"`
	Name            string     `json:"name,omitempty"`
	ExitIP          string     `json:"exit_ip,omitempty"`
	ExitIPCheckedAt *time.Time `json:"exit_ip_checked_at,omitempty"`
	// Internal routing snapshot; omitted from the admin DTO.
	URL string `json:"-"`
}

func usageProxySnapshot(account *Account) *UsageProxySnapshot {
	if account == nil {
		return nil
	}
	if account.Proxy == nil || account.ProxyID == nil {
		return &UsageProxySnapshot{}
	}
	return &UsageProxySnapshot{ID: account.Proxy.ID, Name: account.Proxy.Name, URL: account.Proxy.URL()}
}
