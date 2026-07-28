package model

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// userAgentCache provides a low-overhead, lock-protected snapshot of the
// user_agents table so the relay hot path never touches the DB per request.
var (
	userAgentByID        atomic.Pointer[map[int]*UserAgent]
	userAgentGlobalByCat atomic.Pointer[map[string]*UserAgent]
)

// InitUserAgentCache loads the initial user-agent snapshot.
func InitUserAgentCache() {
	refreshUserAgentCache()
}

func RunUserAgentCacheSync(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshUserAgentCache()
		}
	}
}

func refreshUserAgentCache() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError("recovered panic in refreshUserAgentCache")
		}
	}()
	uas, err := GetAllUserAgents()
	if err != nil {
		common.SysError("failed to refresh user-agent cache: " + err.Error())
		return
	}
	byID := make(map[int]*UserAgent, len(uas))
	globalByCat := make(map[string]*UserAgent)
	for _, ua := range uas {
		if ua == nil || !ua.Enabled {
			continue
		}
		byID[ua.Id] = ua
		if ua.IsGlobal {
			// First global per category wins; deterministic by GetAllUserAgents order.
			if _, ok := globalByCat[ua.ModelCategory]; !ok {
				globalByCat[ua.ModelCategory] = ua
			}
		}
	}
	userAgentByID.Store(&byID)
	userAgentGlobalByCat.Store(&globalByCat)
}

// GetCachedUserAgentByID returns the UA value for an explicit channel selection.
func GetCachedUserAgentByID(id int) (string, bool) {
	m := userAgentByID.Load()
	if m == nil {
		return "", false
	}
	if ua, ok := (*m)[id]; ok && ua != nil {
		return ua.Value, true
	}
	return "", false
}

// GetCachedGlobalUserAgent resolves a global UA by model category, falling back
// to the "other" category when the specific category has no global entry.
func GetCachedGlobalUserAgent(category string) (string, bool) {
	m := userAgentGlobalByCat.Load()
	if m == nil {
		return "", false
	}
	if ua, ok := (*m)[category]; ok && ua != nil {
		return ua.Value, true
	}
	if ua, ok := (*m)[ModelCategoryOther]; ok && ua != nil {
		return ua.Value, true
	}
	return "", false
}
