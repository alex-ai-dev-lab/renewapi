package service

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

type HTTPTransportPolicy struct {
	Protocol string
	Shards   int
}

var httpTransportPolicyWarnings sync.Map

func NormalizeHTTPTransportPolicy(settings dto.ChannelSettings) HTTPTransportPolicy {
	return normalizeHTTPTransportPolicy(settings.HTTPProtocol, settings.HTTP2ConnectionShards)
}

func normalizeHTTPTransportPolicy(protocol string, shards int) HTTPTransportPolicy {
	policy := HTTPTransportPolicy{Protocol: dto.HTTPProtocolAuto, Shards: 1}
	switch normalized := strings.ToLower(strings.TrimSpace(protocol)); normalized {
	case "", dto.HTTPProtocolAuto:
	case dto.HTTPProtocolHTTP1:
		policy.Protocol = dto.HTTPProtocolHTTP1
	default:
		warnHTTPTransportPolicyOnce("http_protocol", protocol)
	}
	switch {
	case shards == 0:
	case shards < 1:
		warnHTTPTransportPolicyOnce("http2_connection_shards", fmt.Sprintf("%d", shards))
	case shards > dto.MaxHTTP2ConnectionShards:
		warnHTTPTransportPolicyOnce("http2_connection_shards", fmt.Sprintf("%d", shards))
		policy.Shards = dto.MaxHTTP2ConnectionShards
	default:
		policy.Shards = shards
	}
	if policy.Protocol == dto.HTTPProtocolHTTP1 {
		policy.Shards = 1
	}
	return policy
}

func warnHTTPTransportPolicyOnce(field, value string) {
	key := field + "=" + value
	if _, loaded := httpTransportPolicyWarnings.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	common.SysLog(fmt.Sprintf("invalid channel http transport setting clamped: %s=%q", field, value))
}

func (p HTTPTransportPolicy) String() string {
	return fmt.Sprintf("%s|%d", p.Protocol, p.Shards)
}

func HTTPTransportFingerprint(settings dto.ChannelSettings) string {
	policy := NormalizeHTTPTransportPolicy(settings)
	proxyHash := sha256.Sum256([]byte(strings.TrimSpace(settings.Proxy)))
	return fmt.Sprintf("proxy_hash=%x|tls=%t|policy=%s", proxyHash[:8], settings.TLSInsecureSkipVerify, policy.String())
}
