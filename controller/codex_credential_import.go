package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type codexCredentialNormalizeRequest struct {
	Input string `json:"input"`
}

type codexCredentialImportRequest struct {
	Input          string `json:"input"`
	CandidateIndex *int   `json:"candidate_index"`
}

type codexCredentialPreflightRequest struct {
	Input                 string `json:"input"`
	CandidateIndex        *int   `json:"candidate_index"`
	ChannelID             int    `json:"channel_id"`
	BaseURL               string `json:"base_url"`
	Proxy                 string `json:"proxy"`
	TLSInsecureSkipVerify bool   `json:"tls_insecure_skip_verify"`
}

func NormalizeCodexCredential(c *gin.Context) {
	req := codexCredentialNormalizeRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.NormalizeCodexCredential(req.Input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func ImportCodexCredentialForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	req := codexCredentialImportRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeCodex {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
		return
	}

	candidate, err := selectCodexCredentialCandidate(req.Input, req.CandidateIndex, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("key", candidate.Key).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    candidate,
	})
}

func PreflightCodexCredential(c *gin.Context) {
	req := codexCredentialPreflightRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	input := req.Input
	baseURL := strings.TrimSpace(req.BaseURL)
	proxyURL := strings.TrimSpace(req.Proxy)
	tlsInsecureSkipVerify := req.TLSInsecureSkipVerify
	var ch *model.Channel
	if req.ChannelID > 0 {
		channel, err := model.GetChannelById(req.ChannelID, true)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if channel == nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
			return
		}
		if channel.Type != constant.ChannelTypeCodex {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
			return
		}
		ch = channel
		if strings.TrimSpace(input) == "" {
			input = ch.Key
		}
		if baseURL == "" {
			baseURL = ch.GetBaseURL()
		}
		setting := ch.GetSetting()
		if proxyURL == "" {
			proxyURL = setting.Proxy
		}
		tlsInsecureSkipVerify = tlsInsecureSkipVerify || setting.TLSInsecureSkipVerify
	}
	if baseURL == "" {
		baseURL = "https://chatgpt.com"
	}

	candidate, err := selectCodexCredentialCandidate(input, req.CandidateIndex, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	oauthKey, err := codex.ParseOAuthKey(candidate.Key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	client, err := service.GetHttpClientWithOptions(service.HTTPClientOptions{
		Proxy:                 proxyURL,
		TLSInsecureSkipVerify: tlsInsecureSkipVerify,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	statusCode, body, fetchErr := service.FetchCodexWhamUsage(ctx, client, baseURL, oauthKey.AccessToken, oauthKey.AccountID)
	category := "ok"
	message := ""
	if fetchErr != nil {
		category = categorizeCodexPreflightError(fetchErr)
		message = fetchErr.Error()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "preflight failed: " + category,
			"data": gin.H{
				"category":  category,
				"error":     message,
				"candidate": candidate,
				"base_url":  baseURL,
				"proxy":     proxyURL,
			},
		})
		return
	}
	ok := statusCode >= 200 && statusCode < 300
	if !ok {
		category = categorizeCodexPreflightStatus(statusCode)
		message = fmt.Sprintf("upstream status: %d", statusCode)
	}
	var payload any
	if common.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}
	if ch != nil && ok {
		candidate.Label = firstNonEmptyString(candidate.Label, ch.Name)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": ok,
		"message": message,
		"data": gin.H{
			"category":        category,
			"upstream_status": statusCode,
			"candidate":       candidate,
			"base_url":        baseURL,
			"proxy":           proxyURL,
			"usage":           payload,
		},
	})
}

func selectCodexCredentialCandidate(input string, candidateIndex *int, requireSingleWhenUnspecified bool) (*service.CodexCredentialCandidate, error) {
	result, err := service.NormalizeCodexCredential(input)
	if err != nil {
		return nil, err
	}
	index := 0
	if candidateIndex != nil {
		index = *candidateIndex
	} else if requireSingleWhenUnspecified && len(result.Candidates) > 1 {
		return nil, fmt.Errorf("detected %d Codex credentials; choose a candidate before importing", len(result.Candidates))
	}
	if index < 0 || index >= len(result.Candidates) {
		return nil, fmt.Errorf("candidate index %d out of range", index)
	}
	return &result.Candidates[index], nil
}

func categorizeCodexPreflightStatus(statusCode int) string {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "unauthorized"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if statusCode >= 500 {
			return "upstream_error"
		}
		return "bad_response"
	}
}

func categorizeCodexPreflightError(err error) string {
	if err == nil {
		return "ok"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded"):
		return "network_timeout"
	case strings.Contains(msg, "proxyconnect") || strings.Contains(msg, "proxy"):
		return "proxy_error"
	case strings.Contains(msg, "connection refused"):
		return "connection_refused"
	default:
		return "network_error"
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
