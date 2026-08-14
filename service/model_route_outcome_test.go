package service

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelRouteOutcomeDoesNotDowngradeSuccess(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	InitModelRouteOutcome(ctx)
	SetModelRouteOutcome(ctx, ModelRouteOutcomeSuccess)
	SetModelRouteOutcome(ctx, ModelRouteOutcomeServerError)
	require.Equal(t, ModelRouteOutcomeSuccess, GetModelRouteOutcome(ctx))
	require.False(t, ModelRouteOutcomeRefundsTotal(GetModelRouteOutcome(ctx)))
}

func TestFinalModelRouteOutcomeTreatsUncompletedUnsetAsServerError(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	InitModelRouteOutcome(ctx)
	require.Equal(t, ModelRouteOutcomeServerError, FinalModelRouteOutcome(ctx, false))
	require.True(t, ModelRouteOutcomeRefundsTotal(GetModelRouteOutcome(ctx)))
}
