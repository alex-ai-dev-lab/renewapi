// Package ua provides the UA management module entry point.
//
// Current state (Step 3): the implementation lives in:
//   - controller/user_agent.go (HTTP handlers)
//   - model/user_agent.go (DB model)
//   - model/user_agent_cache.go (in-memory cache)
//   - relay/channel/api_request.go (UA injection in adaptor)
//
// This file is a placeholder/marker for future deep migration. The current
// thin-wrapper approach keeps the F1 module structure intact while enabling
// other Step 3 modules to reference it through pkg/compat/ua/.
package ua

import (
	"context"
	"github.com/QuantumNous/new-api/model"
)

// InitCache is the unified entry point for UA cache initialization.
// Called from main.go startup sequence.
func InitCache() {
	model.InitUserAgentCache()
}

func Run(ctx context.Context) {
	model.RunUserAgentCacheSync(ctx)
}
