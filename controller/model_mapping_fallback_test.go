package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type recordingBillingSettler struct {
	reserved int
}

func (s *recordingBillingSettler) Settle(int) error         { return nil }
func (s *recordingBillingSettler) Refund(*gin.Context)      {}
func (s *recordingBillingSettler) NeedsRefund() bool        { return s.reserved > 0 }
func (s *recordingBillingSettler) GetPreConsumedQuota() int { return s.reserved }
func (s *recordingBillingSettler) Reserve(targetQuota int) error {
	s.reserved = targetQuota
	return nil
}

func TestRetryNextMappedModelCandidatePinsSameChannel(t *testing.T) {
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	info := &relaycommon.RelayInfo{
		StartTime:         time.Now(),
		FirstResponseTime: time.Now().Add(-time.Second),
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 91},
		ModelMappingRoute: relaycommon.ModelMappingRouteCursor{
			ChannelId:  91,
			Source:     "glm-5.2",
			Candidates: []string{"first", "second"},
		},
	}
	retry := &service.RetryParam{ExcludedChannelIds: map[int]bool{91: true}}
	channel := &model.Channel{Id: 91}
	relayErr := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)

	require.True(t, retryNextMappedModelCandidate(c, info, retry, channel, relayErr))
	require.Equal(t, 1, info.ModelMappingRoute.Index)
	require.Equal(t, 91, retry.ModelMappingFallbackChannelId)
	require.False(t, retry.ExcludedChannelIds[91])
}

func TestSwitchRelayFallbackModelRespectsTokenModelLimit(t *testing.T) {
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-5.5": true})
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-5.5"}
	retry := &service.RetryParam{ModelName: "gpt-5.5", Retry: common.GetPointer(0)}

	err := switchRelayFallbackModel(c, info, retry, "gpt-5.6", 100, &types.TokenCountMeta{})
	require.NotNil(t, err)
	require.Equal(t, types.ErrorCodeModelNotFound, err.GetErrorCode())
	require.Equal(t, "gpt-5.5", info.OriginModelName)
	require.Equal(t, "gpt-5.5", retry.ModelName)
}

func TestSwitchRelayFallbackModelRepricesActualModel(t *testing.T) {
	originalPrices, err := common.Marshal(ratio_setting.GetModelPriceCopy())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(originalPrices)))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"fallback-expensive":2}`))

	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	billing := &recordingBillingSettler{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "primary-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		Billing:         billing,
	}
	retry := &service.RetryParam{
		ModelName:          "primary-model",
		Retry:              common.GetPointer(0),
		ExcludedChannelIds: make(map[int]bool),
	}

	newAPIError := switchRelayFallbackModel(c, info, retry, "fallback-expensive", 100, &types.TokenCountMeta{})
	require.Nil(t, newAPIError)
	require.Equal(t, "fallback-expensive", info.OriginModelName)
	require.Equal(t, "fallback-expensive", retry.ModelName)
	require.True(t, info.PriceData.UsePrice)
	require.Equal(t, 2.0, info.PriceData.ModelPrice)
	require.Positive(t, info.PriceData.QuotaToPreConsume)
	require.Equal(t, info.PriceData.QuotaToPreConsume, billing.reserved)
}

func TestSwitchRelayFallbackModelRepricesUsingFinalRouteGroup(t *testing.T) {
	originalPrices, err := common.Marshal(ratio_setting.GetModelPriceCopy())
	require.NoError(t, err)
	originalGroups := ratio_setting.GroupRatio2JSONString()
	originalSpecialGroups := ratio_setting.GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(originalPrices)))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalSpecialGroups))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"fallback-final-group":2}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"fallback-route":3}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))

	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "fallback-route")
	billing := &recordingBillingSettler{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "primary-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		Billing:         billing,
	}
	retry := &service.RetryParam{
		ModelName:          "primary-model",
		Retry:              common.GetPointer(0),
		ExcludedChannelIds: make(map[int]bool),
	}

	newAPIError := switchRelayFallbackModel(c, info, retry, "fallback-final-group", 100, &types.TokenCountMeta{})
	require.Nil(t, newAPIError)
	require.Equal(t, "fallback-route", info.UsingGroup)
	require.Equal(t, 3.0, info.PriceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, int(2*common.QuotaPerUnit*3), info.PriceData.QuotaToPreConsume)
	require.Equal(t, info.PriceData.QuotaToPreConsume, billing.reserved)
}
