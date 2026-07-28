package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gorilla/websocket"

	"golang.org/x/net/proxy"
)

type HTTPClientOptions struct {
	Proxy                 string
	TLSInsecureSkipVerify bool
	HTTPProtocol          string
	HTTP2ConnectionShards int
}

const maxCachedHTTPClients = 256

var (
	httpClient              *http.Client
	ssrfProtectedHTTPClient *http.Client
	proxyClientLock         sync.Mutex
	proxyClients            = make(map[HTTPClientOptions]*http.Client)
	protectedFetchClients   = make(map[HTTPClientOptions]*http.Client)
)

func checkRedirect(req *http.Request, via []*http.Request) error {
	urlStr := req.URL.String()
	if err := validateURLWithCurrentFetchSetting(urlStr, true); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func checkProtectedFetchRedirect(req *http.Request, via []*http.Request) error {
	urlStr := req.URL.String()
	if err := ValidateSSRFProtectedFetchURL(urlStr); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func validateURLWithCurrentFetchSetting(urlStr string, applyDomainIPFilter bool) error {
	fetchSetting := system_setting.GetFetchSetting()
	if fetchSetting == nil {
		return nil
	}
	return common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, applyDomainIPFilter && fetchSetting.ApplyIPFilterForDomain)
}

func ValidateSSRFProtectedFetchURL(urlStr string) error {
	return validateURLWithCurrentFetchSetting(urlStr, true)
}

func InitHttpClient() {
	client, err := NewHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		common.SysError("failed to initialize http client: " + err.Error())
		return
	}
	if httpClient != nil {
		closeHTTPClientIdleConnections(httpClient)
	}
	httpClient = client
	if ssrfProtectedHTTPClient != nil {
		closeHTTPClientIdleConnections(ssrfProtectedHTTPClient)
	}
	ssrfProtectedHTTPClient = newProtectedFetchHTTPClient()
}

// GetHttpClient is the general provider client. User-controlled URLs must use
// GetSSRFProtectedHTTPClient or GetSSRFProtectedHTTPClientWithOptions.
func GetHttpClient() *http.Client {
	return httpClient
}

func GetSSRFProtectedHTTPClient() *http.Client {
	client, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{})
	if err != nil {
		return ssrfProtectedHTTPClient
	}
	return client
}

func GetSSRFProtectedHTTPClientWithOptions(options HTTPClientOptions) (*http.Client, error) {
	if fetchSetting := system_setting.GetFetchSetting(); fetchSetting == nil || !fetchSetting.EnableSSRFProtection {
		return GetHttpClientWithOptions(options)
	}
	options = normalizeHTTPClientOptions(options)
	if isDefaultHTTPClientOptions(options) && ssrfProtectedHTTPClient != nil {
		return ssrfProtectedHTTPClient, nil
	}

	proxyClientLock.Lock()
	if client, ok := protectedFetchClients[options]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	client, err := newProtectedFetchHTTPClientWithOptions(options)
	if err != nil {
		return nil, err
	}
	proxyClientLock.Lock()
	if existing, ok := protectedFetchClients[options]; ok {
		proxyClientLock.Unlock()
		closeHTTPClientIdleConnections(client)
		return existing, nil
	}
	evictHTTPClientIfFull(protectedFetchClients)
	protectedFetchClients[options] = client
	proxyClientLock.Unlock()
	return client, nil
}

func NewHttpClient() *http.Client {
	client, err := NewHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		return http.DefaultClient
	}
	return client
}

// GetHttpClientWithProxy returns the default client or a proxy-enabled one when proxyURL is provided.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	return GetHttpClientWithOptions(HTTPClientOptions{Proxy: proxyURL})
}

func HTTPClientOptionsFromChannelSettings(settings dto.ChannelSettings) HTTPClientOptions {
	return HTTPClientOptions{
		Proxy: settings.Proxy, TLSInsecureSkipVerify: settings.TLSInsecureSkipVerify,
		HTTPProtocol: settings.HTTPProtocol, HTTP2ConnectionShards: settings.HTTP2ConnectionShards,
	}
}

func GetHttpClientWithChannelSettings(settings dto.ChannelSettings) (*http.Client, error) {
	return GetHttpClientWithOptions(HTTPClientOptionsFromChannelSettings(settings))
}

func GetHttpClientWithOptions(options HTTPClientOptions) (*http.Client, error) {
	options = normalizeHTTPClientOptions(options)
	if isDefaultHTTPClientOptions(options) {
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return NewHttpClientWithOptions(options)
	}

	proxyClientLock.Lock()
	if client, ok := proxyClients[options]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	client, err := NewHttpClientWithOptions(options)
	if err != nil {
		return nil, err
	}
	proxyClientLock.Lock()
	if existing, ok := proxyClients[options]; ok {
		proxyClientLock.Unlock()
		closeHTTPClientIdleConnections(client)
		return existing, nil
	}
	evictHTTPClientIfFull(proxyClients)
	proxyClients[options] = client
	proxyClientLock.Unlock()
	return client, nil
}

func InvalidateHTTPClientOptions(options HTTPClientOptions) {
	options = normalizeHTTPClientOptions(options)
	proxyClientLock.Lock()
	clients := []*http.Client{proxyClients[options], protectedFetchClients[options]}
	delete(proxyClients, options)
	delete(protectedFetchClients, options)
	proxyClientLock.Unlock()
	for _, client := range clients {
		closeHTTPClientIdleConnections(client)
	}
}

func InvalidateHTTPClientSettings(settings dto.ChannelSettings) {
	InvalidateHTTPClientOptions(HTTPClientOptionsFromChannelSettings(settings))
}

func NewWebSocketDialerWithChannelSettings(settings dto.ChannelSettings) (*websocket.Dialer, error) {
	return NewWebSocketDialerWithOptions(HTTPClientOptionsFromChannelSettings(settings))
}

func evictHTTPClientIfFull(cache map[HTTPClientOptions]*http.Client) {
	if len(cache) < maxCachedHTTPClients {
		return
	}
	for key, client := range cache {
		delete(cache, key)
		closeHTTPClientIdleConnections(client)
		return
	}
}

// ResetProxyClientCache 清空代理客户端缓存并重建默认客户端，确保代理或全局 TLS 设置变更后立即生效。
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	if httpClient != nil {
		closeHTTPClientIdleConnections(httpClient)
		httpClient = nil
	}
	if ssrfProtectedHTTPClient != nil {
		closeHTTPClientIdleConnections(ssrfProtectedHTTPClient)
		ssrfProtectedHTTPClient = nil
	}
	for _, client := range proxyClients {
		closeHTTPClientIdleConnections(client)
	}
	for _, client := range protectedFetchClients {
		closeHTTPClientIdleConnections(client)
	}
	proxyClients = make(map[HTTPClientOptions]*http.Client)
	protectedFetchClients = make(map[HTTPClientOptions]*http.Client)
	InitHttpClient()
}

func closeHTTPClientIdleConnections(client *http.Client) {
	if client == nil || client.Transport == nil {
		return
	}
	if closer, ok := client.Transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// NewProxyHttpClient 创建支持代理的 HTTP 客户端
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	return GetHttpClientWithOptions(HTTPClientOptions{Proxy: proxyURL})
}

func NewHttpClientWithOptions(options HTTPClientOptions) (*http.Client, error) {
	options = normalizeHTTPClientOptions(options)
	transport, err := newHTTPTransportWithOptions(options)
	if err != nil {
		return nil, err
	}
	var roundTripper http.RoundTripper = transport
	policy := normalizeHTTPTransportPolicy(options.HTTPProtocol, options.HTTP2ConnectionShards)
	if policy.Shards > 1 {
		roundTripper = newShardedRoundTripper(policy.Shards, func() *http.Transport {
			return transport.Clone()
		})
	}
	client := &http.Client{
		Transport:     roundTripper,
		CheckRedirect: checkRedirect,
	}
	if common.RelayTimeout > 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client, nil
}

func NewWebSocketDialerWithOptions(options HTTPClientOptions) (*websocket.Dialer, error) {
	options = normalizeHTTPClientOptions(options)
	dialer := *websocket.DefaultDialer
	if options.Proxy != "" {
		parsedURL, err := parseProxyURL(options.Proxy)
		if err != nil {
			return nil, err
		}
		switch parsedURL.Scheme {
		case "http", "https":
			dialer.Proxy = http.ProxyURL(parsedURL)
		case "socks5", "socks5h":
			socksDialer, err := newSocks5Dialer(parsedURL)
			if err != nil {
				return nil, err
			}
			dialer.Proxy = nil
			dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialSOCKS5Context(ctx, socksDialer, network, addr)
			}
		default:
			return nil, unsupportedProxySchemeError(parsedURL.Scheme)
		}
	}
	if shouldSkipTLSVerify(options) {
		dialer.TLSClientConfig = newInsecureTLSConfig()
	}
	return &dialer, nil
}

func normalizeHTTPClientOptions(options HTTPClientOptions) HTTPClientOptions {
	options.Proxy = strings.TrimSpace(options.Proxy)
	policy := normalizeHTTPTransportPolicy(options.HTTPProtocol, options.HTTP2ConnectionShards)
	options.HTTPProtocol = policy.Protocol
	options.HTTP2ConnectionShards = policy.Shards
	return options
}

func isDefaultHTTPClientOptions(options HTTPClientOptions) bool {
	return options.Proxy == "" && !options.TLSInsecureSkipVerify &&
		options.HTTPProtocol == dto.HTTPProtocolAuto && options.HTTP2ConnectionShards == 1
}

func newHTTPTransportWithOptions(options HTTPClientOptions) (*http.Transport, error) {
	options = normalizeHTTPClientOptions(options)
	dialTimeout := boundedHTTPTimeout(common.RelayDialTimeout, 30)
	tlsTimeout := boundedHTTPTimeout(common.RelayTLSHandshakeTimeout, 10)
	idleTimeout := boundedHTTPTimeout(common.RelayIdleConnTimeout, 90)
	netDialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		MaxIdleConns:          common.RelayMaxIdleConns,
		MaxIdleConnsPerHost:   common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   tlsTimeout,
		ExpectContinueTimeout: time.Second,
		DialContext:           netDialer.DialContext,
		ForceAttemptHTTP2:     true,
	}
	if options.Proxy == "" {
		transport.Proxy = http.ProxyFromEnvironment // Support HTTP_PROXY, HTTPS_PROXY, NO_PROXY env vars
	} else {
		parsedURL, err := parseProxyURL(options.Proxy)
		if err != nil {
			return nil, err
		}
		switch parsedURL.Scheme {
		case "http", "https":
			transport.Proxy = http.ProxyURL(parsedURL)
		case "socks5", "socks5h":
			socksDialer, err := newSocks5Dialer(parsedURL)
			if err != nil {
				return nil, err
			}
			// proxy.SOCKS5 使用 tcp 参数，所有 TCP 连接包括 DNS 查询都将通过代理进行。行为与 socks5h 相同
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialSOCKS5Context(ctx, socksDialer, network, addr)
			}
		default:
			return nil, unsupportedProxySchemeError(parsedURL.Scheme)
		}
	}
	if shouldSkipTLSVerify(options) {
		transport.TLSClientConfig = newInsecureTLSConfig()
	}
	applyHTTPTransportPolicy(transport, normalizeHTTPTransportPolicy(options.HTTPProtocol, options.HTTP2ConnectionShards))
	return transport, nil
}

func boundedHTTPTimeout(seconds int, fallback int) time.Duration {
	if seconds <= 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

func shouldSkipTLSVerify(options HTTPClientOptions) bool {
	return common.TLSInsecureSkipVerify || options.TLSInsecureSkipVerify
}

func newInsecureTLSConfig() *tls.Config {
	// #nosec G402 -- disabled only when an administrator explicitly enables
	// global or per-channel upstream compatibility for broken/self-signed TLS.
	return &tls.Config{InsecureSkipVerify: true}
}

func parseProxyURL(proxyURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("proxy URL must include scheme and host")
	}
	return parsed, nil
}

func newSocks5Dialer(parsedURL *url.URL) (proxy.ContextDialer, error) {
	var auth *proxy.Auth
	if parsedURL.User != nil {
		auth = &proxy.Auth{
			User:     parsedURL.User.Username(),
			Password: "",
		}
		if password, ok := parsedURL.User.Password(); ok {
			auth.Password = password
		}
	}
	dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, &net.Dialer{
		Timeout: boundedHTTPTimeout(common.RelayDialTimeout, 30), KeepAlive: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("SOCKS5 dialer does not support context cancellation")
	}
	return contextDialer, nil
}

func dialSOCKS5Context(ctx context.Context, dialer proxy.ContextDialer, network, addr string) (net.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, boundedHTTPTimeout(common.RelayDialTimeout, 30))
	defer cancel()
	return dialer.DialContext(dialCtx, network, addr)
}

func unsupportedProxySchemeError(scheme string) error {
	return fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", scheme)
}
