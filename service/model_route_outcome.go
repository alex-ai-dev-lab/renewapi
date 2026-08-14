package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

type ModelRouteOutcome string

const (
	ModelRouteOutcomeUnset             ModelRouteOutcome = "unset"
	ModelRouteOutcomeSuccess           ModelRouteOutcome = "success"
	ModelRouteOutcomeRouteUnavailable  ModelRouteOutcome = "route_unavailable"
	ModelRouteOutcomeUpstreamRetryable ModelRouteOutcome = "upstream_retryable"
	ModelRouteOutcomeServerError       ModelRouteOutcome = "server_error"
	ModelRouteOutcomeClientRejected    ModelRouteOutcome = "client_rejected"
)

const ginKeyModelRouteOutcome = "model_route_outcome"

func InitModelRouteOutcome(c *gin.Context) {
	if c != nil {
		c.Set(ginKeyModelRouteOutcome, string(ModelRouteOutcomeUnset))
	}
}

func SetModelRouteOutcome(c *gin.Context, outcome ModelRouteOutcome) {
	if c == nil || outcome == "" {
		return
	}
	current := GetModelRouteOutcome(c)
	if current == outcome {
		return
	}
	if current == ModelRouteOutcomeSuccess && outcome != ModelRouteOutcomeSuccess {
		return
	}
	c.Set(ginKeyModelRouteOutcome, string(outcome))
	if outcome != ModelRouteOutcomeUnset && outcome != ModelRouteOutcomeSuccess {
		common.SysLog(fmt.Sprintf(
			"route_outcome outcome=%s model=%s group=%s request_id=%s",
			outcome,
			common.GetContextKeyString(c, constant.ContextKeyOriginalModel),
			common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			c.GetString(common.RequestIdKey),
		))
	}
}

func GetModelRouteOutcome(c *gin.Context) ModelRouteOutcome {
	if c == nil {
		return ModelRouteOutcomeUnset
	}
	switch outcome := ModelRouteOutcome(c.GetString(ginKeyModelRouteOutcome)); outcome {
	case ModelRouteOutcomeSuccess,
		ModelRouteOutcomeRouteUnavailable,
		ModelRouteOutcomeUpstreamRetryable,
		ModelRouteOutcomeServerError,
		ModelRouteOutcomeClientRejected:
		return outcome
	default:
		return ModelRouteOutcomeUnset
	}
}

func FinalModelRouteOutcome(c *gin.Context, completed bool) ModelRouteOutcome {
	if RelaySemanticSuccess(c) {
		SetModelRouteOutcome(c, ModelRouteOutcomeSuccess)
		return ModelRouteOutcomeSuccess
	}
	outcome := GetModelRouteOutcome(c)
	if !completed && outcome == ModelRouteOutcomeUnset {
		SetModelRouteOutcome(c, ModelRouteOutcomeServerError)
		return ModelRouteOutcomeServerError
	}
	return outcome
}

func ModelRouteOutcomeRefundsTotal(outcome ModelRouteOutcome) bool {
	switch outcome {
	case ModelRouteOutcomeRouteUnavailable,
		ModelRouteOutcomeUpstreamRetryable,
		ModelRouteOutcomeServerError:
		return true
	default:
		return false
	}
}
