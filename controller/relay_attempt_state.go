package controller

import (
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/gin-gonic/gin"
)

// These contracts belong to the selected upstream attempt. Clearing them in
// InitChannelMeta would also clear contracts during nested protocol conversion.
func resetRelayAttemptGuards(c *gin.Context, info *relaycommon.RelayInfo) {
	info.AntiPoisonGuardPrefix = ""
	info.AntiPoisonAnswerEnvelopeNonce = ""
	info.AntiPoisonResponseProofNonce = ""
	info.AntiPoisonCanaryNonce = ""
	info.AntiPoisonToolsDeclared = false
	info.AntiPoisonAllowedTools = nil
	info.AntiPoisonConfigCache = nil
	openaicompat.SetResponsesBridgeToolMapping(c, openaicompat.ResponsesBridgeToolMapping{})
	for _, key := range []constant.ContextKey{
		constant.ContextKeyAntiPoisonEvidenceResponse, constant.ContextKeyAntiPoisonRiskLevel,
		constant.ContextKeyAntiPoisonRiskSignal, constant.ContextKeyAntiPoisonActionTaken,
		constant.ContextKeyAntiPoisonStreamMode, constant.ContextKeyAntiPoisonOpaqueScore,
		constant.ContextKeyAntiPoisonOpaqueHits, constant.ContextKeyAntiPoisonToolGuardResult,
		constant.ContextKeyAntiPoisonShapeResult, constant.ContextKeyAntiPoisonEnvelopeResult,
		constant.ContextKeyAntiPoisonProofResult, constant.ContextKeyAntiPoisonCanaryResult,
	} {
		c.Set(string(key), nil)
	}
}
