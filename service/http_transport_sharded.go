package service

import (
	"crypto/tls"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

type shardedRoundTripper struct {
	shards   []http.RoundTripper
	n        uint32
	counters sync.Map
}

func newShardedRoundTripper(shards int, factory func() *http.Transport) *shardedRoundTripper {
	if shards < 1 {
		shards = 1
	}
	transports := make([]http.RoundTripper, shards)
	for i := range transports {
		transport := factory()
		transport.MaxIdleConns = max(1, transport.MaxIdleConns/shards)
		transport.MaxIdleConnsPerHost = max(1, transport.MaxIdleConnsPerHost/shards)
		transports[i] = transport
	}
	return &shardedRoundTripper{shards: transports, n: uint32(shards)}
}

func (s *shardedRoundTripper) pickShard(origin string) uint32 {
	if s.n <= 1 {
		return 0
	}
	counter, _ := s.counters.LoadOrStore(origin, &atomic.Uint32{})
	return (counter.(*atomic.Uint32).Add(1) - 1) % s.n
}

func (s *shardedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	origin := ""
	if req != nil && req.URL != nil {
		origin = strings.ToLower(req.URL.Scheme) + "://" + strings.ToLower(req.URL.Host)
	}
	return s.shards[s.pickShard(origin)].RoundTrip(req)
}

func (s *shardedRoundTripper) CloseIdleConnections() {
	for _, shard := range s.shards {
		if closer, ok := shard.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}

func applyHTTPTransportPolicy(transport *http.Transport, policy HTTPTransportPolicy) {
	if policy.Protocol != "http1" {
		transport.ForceAttemptHTTP2 = true
		return
	}
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	if transport.TLSClientConfig != nil {
		config := transport.TLSClientConfig.Clone()
		config.NextProtos = nil
		transport.TLSClientConfig = config
	}
}
