package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

type staticSSRFResolver map[string][]net.IPAddr

func (r staticSSRFResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if ips, ok := r[host]; ok {
		return ips, nil
	}
	return nil, fmt.Errorf("unexpected lookup for %s", host)
}

func staticProtection(protection *common.SSRFProtection) func() (*common.SSRFProtection, bool, error) {
	return func() (*common.SSRFProtection, bool, error) { return protection, true, nil }
}

func testPipeConn(t *testing.T) net.Conn {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})
	return clientConn
}

func configureSSRFTestFetchSetting(t *testing.T, allowPrivate bool) {
	t.Helper()
	fetchSetting := system_setting.GetFetchSetting()
	original := *fetchSetting
	t.Cleanup(func() { *fetchSetting = original })
	fetchSetting.EnableSSRFProtection = true
	fetchSetting.AllowPrivateIp = allowPrivate
	fetchSetting.DomainFilterMode = false
	fetchSetting.IpFilterMode = false
	fetchSetting.DomainList = nil
	fetchSetting.IpList = nil
	fetchSetting.AllowedPorts = nil
	fetchSetting.ApplyIPFilterForDomain = true
}

func TestProtectedFetchDialerRejectsPrivateReboundAddress(t *testing.T) {
	dialer := &protectedFetchDialer{
		resolver: staticSSRFResolver{"safe.example": {{IP: net.ParseIP("127.0.0.1")}}},
		dialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			t.Fatalf("blocked address must not be dialed: %s", address)
			return nil, nil
		},
		getProtection: staticProtection(&common.SSRFProtection{ApplyIPFilterForDomain: true}),
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "safe.example:80")
	require.Error(t, err)
	require.Nil(t, conn)
	require.Contains(t, err.Error(), "private IP address not allowed")
}

func TestProtectedFetchDialerRejectsMixedResolvedIPsBeforeDial(t *testing.T) {
	var dialed []string
	dialer := &protectedFetchDialer{
		resolver: staticSSRFResolver{"safe.example": {
			{IP: net.ParseIP("8.8.8.8")},
			{IP: net.ParseIP("10.0.0.1")},
		}},
		dialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			return testPipeConn(t), nil
		},
		getProtection: staticProtection(&common.SSRFProtection{ApplyIPFilterForDomain: true}),
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "safe.example:443")
	require.Error(t, err)
	require.Nil(t, conn)
	require.Empty(t, dialed)
}

func TestProtectedFetchDialerPinsAllowedResolvedIP(t *testing.T) {
	var dialed []string
	dialer := &protectedFetchDialer{
		resolver: staticSSRFResolver{"safe.example": {{IP: net.ParseIP("8.8.8.8")}}},
		dialContext: func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			return testPipeConn(t), nil
		},
		getProtection: staticProtection(&common.SSRFProtection{ApplyIPFilterForDomain: true}),
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "safe.example:443")
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, []string{"8.8.8.8:443"}, dialed)
}

func TestProtectedFetchRoundTripperRejectsPrivateTargetBeforeProxyDial(t *testing.T) {
	configureSSRFTestFetchSetting(t, false)
	proxyURL, err := url.Parse("http://127.0.0.1:3128")
	require.NoError(t, err)
	var dialed []string
	client := newProtectedFetchHTTPClientWithProxy(
		staticSSRFResolver{},
		func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			return nil, errors.New("proxy should not be dialed")
		},
		staticProtection(&common.SSRFProtection{ApplyIPFilterForDomain: true}),
		func(*http.Request) (*url.URL, error) { return proxyURL, nil },
	)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/resource", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Empty(t, dialed)
}

func TestProtectedFetchClientPreservesTLSAndProxyOptions(t *testing.T) {
	configureSSRFTestFetchSetting(t, true)
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	verified, err := newProtectedFetchHTTPClientWithOptions(HTTPClientOptions{})
	require.NoError(t, err)
	resp, err := verified.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)

	insecure, err := newProtectedFetchHTTPClientWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	require.NoError(t, err)
	resp, err = insecure.Get(server.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()

	httpProxy, err := newProtectedFetchHTTPClientWithOptions(HTTPClientOptions{Proxy: "http://127.0.0.1:9"})
	require.NoError(t, err)
	require.NotNil(t, httpProxy)
	socksProxy, err := newProtectedFetchHTTPClientWithOptions(HTTPClientOptions{Proxy: "socks5://127.0.0.1:9"})
	require.NoError(t, err)
	roundTripper := socksProxy.Transport.(*ssrfProtectedRoundTripper)
	require.False(t, roundTripper.protectDirectDial)
}

func TestGetSSRFProtectedHTTPClientFallsBackWhenProtectionDisabled(t *testing.T) {
	fetchSetting := system_setting.GetFetchSetting()
	original := *fetchSetting
	originalHTTPClient := httpClient
	originalProtectedClient := ssrfProtectedHTTPClient
	t.Cleanup(func() {
		*fetchSetting = original
		httpClient = originalHTTPClient
		ssrfProtectedHTTPClient = originalProtectedClient
	})
	fetchSetting.EnableSSRFProtection = false
	expected := &http.Client{}
	httpClient = expected
	ssrfProtectedHTTPClient = &http.Client{}
	require.Same(t, expected, GetSSRFProtectedHTTPClient())
}

func TestProtectedFetchClientCacheIncludesProxyAndTLSOptions(t *testing.T) {
	configureSSRFTestFetchSetting(t, false)
	resetHTTPClientTestState(t)

	defaultA, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{})
	require.NoError(t, err)
	defaultB, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{})
	require.NoError(t, err)
	require.Same(t, defaultA, defaultB)

	insecureA, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	require.NoError(t, err)
	insecureB, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	require.NoError(t, err)
	require.Same(t, insecureA, insecureB)
	require.NotSame(t, defaultA, insecureA)

	proxyA, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{Proxy: "http://127.0.0.1:9"})
	require.NoError(t, err)
	proxyB, err := GetSSRFProtectedHTTPClientWithOptions(HTTPClientOptions{Proxy: "http://127.0.0.1:9"})
	require.NoError(t, err)
	require.Same(t, proxyA, proxyB)
	require.NotSame(t, defaultA, proxyA)
}

func TestProtectedFetchRoundTripperReusesTransportPerProxy(t *testing.T) {
	client := newProtectedFetchHTTPClientWithDialer(nil, nil, nil)
	roundTripper := client.Transport.(*ssrfProtectedRoundTripper)
	proxyURL, err := url.Parse("http://127.0.0.1:3128")
	require.NoError(t, err)

	direct := roundTripper.transportFor(nil)
	require.Same(t, direct, roundTripper.transportFor(nil))
	require.NotSame(t, direct, roundTripper.transportFor(proxyURL))
	require.True(t, direct.ForceAttemptHTTP2)
	require.Equal(t, httpTLSHandshakeTimeout, direct.TLSHandshakeTimeout)
	require.Equal(t, httpIdleConnTimeout, direct.IdleConnTimeout)
	require.Equal(t, httpExpectContinueTimeout, direct.ExpectContinueTimeout)
}
