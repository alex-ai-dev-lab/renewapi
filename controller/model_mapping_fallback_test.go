package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRetryNextMappedModelCandidatePinsSameChannel(t *testing.T) {
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	info := &relaycommon.RelayInfo{
		StartTime:                      time.Now(),
		FirstResponseTime:              time.Now().Add(-time.Second),
		ChannelMeta:                    &relaycommon.ChannelMeta{ChannelId: 91},
		ModelMappingFallbackChannelId:  91,
		ModelMappingFallbackSource:     "glm-5.2",
		ModelMappingFallbackCandidates: []string{"first", "second"},
		ModelMappingFallbackIndex:      0,
	}
	retry := &service.RetryParam{ExcludedChannelIds: map[int]bool{91: true}}
	channel := &model.Channel{Id: 91}
	relayErr := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)

	require.True(t, retryNextMappedModelCandidate(c, info, retry, channel, relayErr))
	require.Equal(t, 1, info.ModelMappingFallbackIndex)
	require.Equal(t, 91, retry.ModelMappingFallbackChannelId)
	require.False(t, retry.ExcludedChannelIds[91])
}
