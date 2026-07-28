package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

type contextOnlyDialer struct {
	mu           sync.Mutex
	deadlineSeen bool
}

func (d *contextOnlyDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	d.mu.Lock()
	_, d.deadlineSeen = ctx.Deadline()
	d.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func resetHTTPClientTestState(t *testing.T) {
	t.Helper()
	oldTLSInsecureSkipVerify := common.TLSInsecureSkipVerify
	oldRelayTimeout := common.RelayTimeout
	oldDialTimeout := common.RelayDialTimeout
	oldTLSHandshakeTimeout := common.RelayTLSHandshakeTimeout
	oldIdleConnTimeout := common.RelayIdleConnTimeout
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("NO_PROXY", "")

	common.TLSInsecureSkipVerify = false
	common.RelayTimeout = 0
	common.RelayDialTimeout = 30
	common.RelayTLSHandshakeTimeout = 10
	common.RelayIdleConnTimeout = 90
	ResetProxyClientCache()
	InitHttpClient()

	t.Cleanup(func() {
		common.TLSInsecureSkipVerify = oldTLSInsecureSkipVerify
		common.RelayTimeout = oldRelayTimeout
		common.RelayDialTimeout = oldDialTimeout
		common.RelayTLSHandshakeTimeout = oldTLSHandshakeTimeout
		common.RelayIdleConnTimeout = oldIdleConnTimeout
		ResetProxyClientCache()
		InitHttpClient()
	})
}

func TestHTTPTransportHasBoundedConnectionPhases(t *testing.T) {
	resetHTTPClientTestState(t)
	transport, err := newHTTPTransportWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if transport.DialContext == nil || transport.TLSHandshakeTimeout != 10*time.Second ||
		transport.IdleConnTimeout != 90*time.Second || transport.ExpectContinueTimeout != time.Second {
		t.Fatalf("transport timeouts are not bounded: %#v", transport)
	}
}

func TestSOCKS5DialPropagatesContextAndDeadline(t *testing.T) {
	resetHTTPClientTestState(t)
	dialer := &contextOnlyDialer{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := dialSOCKS5Context(ctx, dialer, "tcp", "example.com:443")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dial error = %v, want context.Canceled", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("canceled SOCKS dial did not return promptly")
	}
	dialer.mu.Lock()
	deadlineSeen := dialer.deadlineSeen
	dialer.mu.Unlock()
	if !deadlineSeen {
		t.Fatal("SOCKS dial context must carry a deadline")
	}
}

func TestConcurrentHTTPClientCacheMissReturnsSingleClient(t *testing.T) {
	resetHTTPClientTestState(t)
	options := HTTPClientOptions{Proxy: "http://127.0.0.1:9", TLSInsecureSkipVerify: true, HTTP2ConnectionShards: 2}
	const workers = 32
	start := make(chan struct{})
	clients := make(chan *http.Client, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			client, err := GetHttpClientWithOptions(options)
			clients <- client
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(clients)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var expected *http.Client
	for client := range clients {
		if expected == nil {
			expected = client
		} else if client != expected {
			t.Fatal("concurrent cache miss returned multiple clients")
		}
	}
}

func TestHTTPClientCacheIsBounded(t *testing.T) {
	resetHTTPClientTestState(t)
	for i := 0; i < maxCachedHTTPClients+50; i++ {
		_, err := GetHttpClientWithOptions(HTTPClientOptions{Proxy: fmt.Sprintf("http://127.0.0.1:%d", 10000+i)})
		if err != nil {
			t.Fatal(err)
		}
	}
	proxyClientLock.Lock()
	size := len(proxyClients)
	proxyClientLock.Unlock()
	if size > maxCachedHTTPClients {
		t.Fatalf("client cache size = %d, max = %d", size, maxCachedHTTPClients)
	}
}

func TestHTTPTransportPolicyAndShards(t *testing.T) {
	resetHTTPClientTestState(t)
	policy := NormalizeHTTPTransportPolicy(dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolHTTP1, HTTP2ConnectionShards: 1})
	transport, err := newHTTPTransportWithOptions(HTTPClientOptions{HTTPProtocol: policy.Protocol})
	if err != nil {
		t.Fatal(err)
	}
	if transport.ForceAttemptHTTP2 || transport.TLSNextProto == nil {
		t.Fatal("http1 policy did not disable HTTP/2 negotiation")
	}

	shardedClient, err := GetHttpClientWithChannelSettings(dto.ChannelSettings{HTTP2ConnectionShards: 3})
	if err != nil {
		t.Fatal(err)
	}
	sharded, ok := shardedClient.Transport.(*shardedRoundTripper)
	if !ok || sharded.n != 3 {
		t.Fatalf("transport = %T, shards = %v", shardedClient.Transport, sharded)
	}
	if sharded.pickShard("https://example.com") == sharded.pickShard("https://example.com") {
		t.Fatal("same origin did not rotate across shards")
	}
}

func TestHTTP1PolicyDisablesTLSHTTP2Negotiation(t *testing.T) {
	resetHTTPClientTestState(t)
	protocol := make(chan int, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protocol <- r.ProtoMajor
		w.WriteHeader(http.StatusNoContent)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	client, err := GetHttpClientWithOptions(HTTPClientOptions{
		TLSInsecureSkipVerify: true,
		HTTPProtocol:          dto.HTTPProtocolHTTP1,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := <-protocol; got != 1 {
		t.Fatalf("negotiated HTTP/%d, want HTTP/1", got)
	}
}

func TestInvalidateHTTPClientOptionsEvictsOnlyExactPolicy(t *testing.T) {
	resetHTTPClientTestState(t)
	firstOptions := HTTPClientOptions{Proxy: "http://127.0.0.1:19001", HTTP2ConnectionShards: 2}
	secondOptions := HTTPClientOptions{Proxy: "http://127.0.0.1:19001", HTTP2ConnectionShards: 3}
	first, err := GetHttpClientWithOptions(firstOptions)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GetHttpClientWithOptions(secondOptions)
	if err != nil {
		t.Fatal(err)
	}

	InvalidateHTTPClientOptions(firstOptions)
	reloadedFirst, err := GetHttpClientWithOptions(firstOptions)
	if err != nil {
		t.Fatal(err)
	}
	reusedSecond, err := GetHttpClientWithOptions(secondOptions)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedFirst == first {
		t.Fatal("exactly invalidated client was reused")
	}
	if reusedSecond != second {
		t.Fatal("unrelated transport policy was evicted")
	}
}

func TestHTTPTransportFingerprintDoesNotExposeProxyCredentials(t *testing.T) {
	fingerprint := HTTPTransportFingerprint(dto.ChannelSettings{Proxy: "socks5://user:secret@example.com:1080"})
	if strings.Contains(fingerprint, "user") || strings.Contains(fingerprint, "secret") || strings.Contains(fingerprint, "example.com") {
		t.Fatalf("fingerprint exposed proxy credentials: %s", fingerprint)
	}
}

func TestHTTPClientOptionsTLSInsecureSkipVerify(t *testing.T) {
	resetHTTPClientTestState(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	defaultClient, err := GetHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatalf("default client: %v", err)
	}
	if resp, err := defaultClient.Get(server.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected default client to reject httptest self-signed certificate")
	}

	insecureClient, err := GetHttpClientWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("insecure client: %v", err)
	}
	resp, err := insecureClient.Get(server.URL)
	if err != nil {
		t.Fatalf("expected insecure client to accept httptest self-signed certificate: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestGlobalTLSInsecureSkipVerifyDefaultsToVerified(t *testing.T) {
	resetHTTPClientTestState(t)

	if common.TLSInsecureSkipVerify {
		t.Fatal("TLS_INSECURE_SKIP_VERIFY must default to certificate verification")
	}
}

func TestHTTPClientCacheKeyIncludesProxyAndTLS(t *testing.T) {
	resetHTTPClientTestState(t)

	defaultClientA, err := GetHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatalf("default client A: %v", err)
	}
	defaultClientB, err := GetHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatalf("default client B: %v", err)
	}
	if defaultClientA != defaultClientB {
		t.Fatal("default clients should reuse the initialized client")
	}

	insecureClientA, err := GetHttpClientWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("insecure client A: %v", err)
	}
	insecureClientB, err := GetHttpClientWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("insecure client B: %v", err)
	}
	if insecureClientA != insecureClientB {
		t.Fatal("same TLS options should reuse cached client")
	}
	if insecureClientA == defaultClientA {
		t.Fatal("TLS skip client must not share the default verified client")
	}

	proxyURL := "http://127.0.0.1:9"
	proxyClientA, err := GetHttpClientWithOptions(HTTPClientOptions{Proxy: proxyURL})
	if err != nil {
		t.Fatalf("proxy client A: %v", err)
	}
	proxyClientB, err := GetHttpClientWithOptions(HTTPClientOptions{Proxy: proxyURL})
	if err != nil {
		t.Fatalf("proxy client B: %v", err)
	}
	if proxyClientA != proxyClientB {
		t.Fatal("same proxy options should reuse cached client")
	}
	if proxyClientA == defaultClientA {
		t.Fatal("proxy client must not share the default client")
	}

	proxyInsecureClient, err := GetHttpClientWithOptions(HTTPClientOptions{
		Proxy:                 proxyURL,
		TLSInsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("proxy insecure client: %v", err)
	}
	if proxyInsecureClient == proxyClientA {
		t.Fatal("proxy client cache key must include TLSInsecureSkipVerify")
	}
	if proxyInsecureClient == insecureClientA {
		t.Fatal("client cache key must include Proxy")
	}
}

func TestGlobalTLSInsecureSkipVerifyStillApplies(t *testing.T) {
	resetHTTPClientTestState(t)
	common.TLSInsecureSkipVerify = true
	ResetProxyClientCache()
	InitHttpClient()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := GetHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatalf("global insecure client: %v", err)
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("expected global TLS_INSECURE_SKIP_VERIFY behavior to accept self-signed certificate: %v", err)
	}
	_ = resp.Body.Close()
}

func TestResetProxyClientCacheRebuildsDefaultClientForGlobalTLS(t *testing.T) {
	resetHTTPClientTestState(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	verifiedClient, err := GetHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatalf("verified client: %v", err)
	}
	if resp, err := verifiedClient.Get(server.URL); err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected initial default client to reject self-signed certificate")
	}

	common.TLSInsecureSkipVerify = true
	ResetProxyClientCache()

	reloadedClient, err := GetHttpClientWithOptions(HTTPClientOptions{})
	if err != nil {
		t.Fatalf("reloaded client: %v", err)
	}
	if reloadedClient == verifiedClient {
		t.Fatal("default client should be rebuilt after cache reset")
	}
	resp, err := reloadedClient.Get(server.URL)
	if err != nil {
		t.Fatalf("expected rebuilt default client to accept self-signed certificate: %v", err)
	}
	_ = resp.Body.Close()
}

func TestTLSVerificationErrorDoesNotDisableChannel(t *testing.T) {
	oldAutomaticDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = oldAutomaticDisable
	})

	err := types.NewError(
		errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority"),
		types.ErrorCodeDoRequestFailed,
	)
	if ShouldDisableChannel(err) {
		t.Fatal("TLS verification errors must not auto-disable channels")
	}
}
