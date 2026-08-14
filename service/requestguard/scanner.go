package requestguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	parentservice "github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var ErrEndpointUnavailable = errors.New("RequestGuard endpoint unavailable")

const maxGuardResponseBytes = 1 << 20

type scanner interface {
	Evaluate(ctx context.Context, snapshot Snapshot, endpoint operation_setting.RequestGuardEndpoint, secret, requestHost string) (Result, error)
}

type scanError struct {
	Kind       error
	Class      string
	HTTPStatus int
}

func (e *scanError) Error() string {
	if e == nil {
		return "RequestGuard scan failed"
	}
	if e.HTTPStatus > 0 {
		return fmt.Sprintf("%s: status %d", e.Class, e.HTTPStatus)
	}
	return e.Class
}

func (e *scanError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

func newScanError(kind error, class string, status int) error {
	return &scanError{Kind: kind, Class: class, HTTPStatus: status}
}

func scanErrorDetails(err error) (string, int) {
	var typed *scanError
	if errors.As(err, &typed) {
		return typed.Class, typed.HTTPStatus
	}
	return "endpoint_unavailable", 0
}

type openAICompatibleScanner struct {
	clients sync.Map
}

type guardClientKey struct {
	AllowPrivateIP bool
	ProxyPolicy    string
	ProxyURL       string
}

func (s *openAICompatibleScanner) Evaluate(ctx context.Context, snapshot Snapshot, endpoint operation_setting.RequestGuardEndpoint, secret, requestHost string) (Result, error) {
	requestURL, err := guardChatCompletionsURL(endpoint.BaseURL)
	if err != nil {
		return Result{}, newScanError(ErrEndpointUnavailable, "invalid_endpoint_url", 0)
	}
	if sameHost(requestURL, requestHost) {
		return Result{}, newScanError(ErrEndpointUnavailable, "recursive_endpoint", 0)
	}
	selectedCodec, err := codecFor(endpoint.Codec)
	if err != nil {
		return Result{}, newScanError(ErrInvalidResponse, "codec_unavailable", 0)
	}
	payload := struct {
		Model       string         `json:"model"`
		Messages    []guardMessage `json:"messages"`
		Temperature float64        `json:"temperature"`
		TopP        float64        `json:"top_p"`
		MaxTokens   int            `json:"max_tokens"`
		Stream      bool           `json:"stream"`
	}{
		Model: endpoint.Model, Messages: selectedCodec.Messages(snapshot),
		Temperature: 0, TopP: 1, MaxTokens: 256, Stream: false,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return Result{}, newScanError(ErrEndpointUnavailable, "request_encode", 0)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return Result{}, newScanError(ErrEndpointUnavailable, "request_build", 0)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set(internalRequestHeader, "1")
	if strings.TrimSpace(secret) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(secret))
	}

	client, err := s.clientFor(endpoint)
	if err != nil {
		return Result{}, newScanError(ErrEndpointUnavailable, "client_policy", 0)
	}
	resp, err := client.Do(req)
	if err != nil {
		class := "network_error"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			class = "timeout"
		}
		return Result{}, newScanError(ErrEndpointUnavailable, class, 0)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return Result{}, newScanError(ErrEndpointUnavailable, "http_status", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxGuardResponseBytes+1))
	if err != nil {
		return Result{}, newScanError(ErrEndpointUnavailable, "response_read", resp.StatusCode)
	}
	if len(responseBody) > maxGuardResponseBytes {
		return Result{}, newScanError(ErrInvalidResponse, "response_too_large", resp.StatusCode)
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(responseBody, &response); err != nil || len(response.Choices) == 0 {
		return Result{}, newScanError(ErrInvalidResponse, "malformed_openai_response", resp.StatusCode)
	}
	result, err := selectedCodec.Decode(response.Choices[0].Message.Content)
	if err != nil {
		return Result{}, newScanError(ErrInvalidResponse, "codec_invalid", resp.StatusCode)
	}
	result.HTTPStatus = resp.StatusCode
	return result, nil
}

func (s *openAICompatibleScanner) clientFor(endpoint operation_setting.RequestGuardEndpoint) (*http.Client, error) {
	key := guardClientKey{AllowPrivateIP: endpoint.AllowPrivateIP, ProxyPolicy: endpoint.ProxyPolicy, ProxyURL: endpoint.ProxyURL}
	if cached, ok := s.clients.Load(key); ok {
		return cached.(*http.Client), nil
	}
	options := parentservice.StrictSSRFHTTPClientOptions{AllowPrivateIP: endpoint.AllowPrivateIP}
	switch endpoint.ProxyPolicy {
	case operation_setting.RequestGuardProxyDisabled:
	case operation_setting.RequestGuardProxyEnvironment:
		options.UseEnvironmentProxy = true
	case operation_setting.RequestGuardProxyExplicit:
		options.Proxy = endpoint.ProxyURL
	default:
		return nil, fmt.Errorf("unsupported proxy policy")
	}
	client, err := parentservice.NewStrictSSRFHTTPClient(options)
	if err != nil {
		return nil, err
	}
	actual, loaded := s.clients.LoadOrStore(key, client)
	if loaded {
		if closer, ok := client.Transport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
		return actual.(*http.Client), nil
	}
	return client, nil
}

func guardChatCompletionsURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid URL")
	}
	pathValue := strings.TrimRight(parsed.Path, "/")
	switch {
	case strings.HasSuffix(pathValue, "/chat/completions"):
	case strings.HasSuffix(pathValue, "/v1"):
		pathValue += "/chat/completions"
	default:
		pathValue += "/v1/chat/completions"
	}
	parsed.Path = pathValue
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func sameHost(targetURL, requestHost string) bool {
	requestHost = strings.TrimSpace(requestHost)
	if requestHost == "" {
		return false
	}
	target, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	request, err := url.Parse("//" + requestHost)
	if err != nil {
		return false
	}
	targetHostname := strings.TrimSuffix(target.Hostname(), ".")
	requestHostname := strings.TrimSuffix(request.Hostname(), ".")
	return targetHostname != "" && strings.EqualFold(targetHostname, requestHostname)
}
