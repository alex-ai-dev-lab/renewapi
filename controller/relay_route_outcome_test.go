package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetRelayErrorModelRouteOutcomeClassifiesFinalUpstreamFailure(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	service.InitModelRouteOutcome(ctx)
	setRelayErrorModelRouteOutcome(ctx, types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests))
	require.Equal(t, service.ModelRouteOutcomeUpstreamRetryable, service.GetModelRouteOutcome(ctx))
}

func TestSetRelayErrorModelRouteOutcomeKeepsExistingOutcome(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	service.InitModelRouteOutcome(ctx)
	service.SetModelRouteOutcome(ctx, service.ModelRouteOutcomeClientRejected)
	setRelayErrorModelRouteOutcome(ctx, types.NewOpenAIError(errors.New("bad gateway"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway))
	require.Equal(t, service.ModelRouteOutcomeClientRejected, service.GetModelRouteOutcome(ctx))
}

func TestSetRelayErrorModelRouteOutcomeClassifiesGatewayInternalError(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	service.InitModelRouteOutcome(ctx)
	setRelayErrorModelRouteOutcome(ctx, types.NewError(errors.New("internal failure"), types.ErrorCodeGenRelayInfoFailed))
	require.Equal(t, service.ModelRouteOutcomeServerError, service.GetModelRouteOutcome(ctx))
}
