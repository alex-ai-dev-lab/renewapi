package compat

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/requestguard"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type requestGuardHook struct {
	NoOpRelayHook
}

func NewRequestGuardHook() RelayHook {
	return &requestGuardHook{}
}

func (h *requestGuardHook) Name() string { return "requestguard" }

func (h *requestGuardHook) OnRequestPreflight(c *gin.Context, info *relaycommon.RelayInfo, request dto.Request) *types.NewAPIError {
	return requestguard.Preflight(c, info, request)
}
