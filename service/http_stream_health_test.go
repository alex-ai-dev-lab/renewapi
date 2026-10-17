package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestLongStreamHTTP2AllowsHealthySilentGeneration(t *testing.T) {
	resetHTTPClientTestState(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-time.After(180 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	transport, err := newHTTPTransportWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	if err := configureLongStreamHTTP2(transport, 20*time.Millisecond, 80*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.ProtoMajor != 2 || string(body) != "data: [DONE]\n\n" {
		t.Fatalf("proto=%s body=%q err=%v", resp.Proto, body, err)
	}
}

func TestLongStreamHTTP2ClosesPeerThatIgnoresPing(t *testing.T) {
	resetHTTPClientTestState(t)
	// A real TLS/H2 peer sends headers, then deliberately never ACKs PING.
	certServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certs := certServer.TLS.Certificates
	certServer.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certs, NextProtos: []string{"h2"}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var pings atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		preface := make([]byte, len(http2.ClientPreface))
		if _, err := io.ReadFull(conn, preface); err != nil || string(preface) != http2.ClientPreface {
			return
		}
		framer := http2.NewFramer(conn, conn)
		if err := framer.WriteSettings(); err != nil {
			return
		}
		var headers bytes.Buffer
		encoder := hpack.NewEncoder(&headers)
		_ = encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"})
		for {
			frame, err := framer.ReadFrame()
			if err != nil {
				return
			}
			switch frame := frame.(type) {
			case *http2.SettingsFrame:
				if !frame.IsAck() {
					if err := framer.WriteSettingsAck(); err != nil {
						return
					}
				}
			case *http2.HeadersFrame:
				if err := framer.WriteHeaders(http2.HeadersFrameParam{StreamID: frame.StreamID, BlockFragment: headers.Bytes(), EndHeaders: true}); err != nil {
					return
				}
			case *http2.PingFrame:
				pings.Add(1)
			}
		}
	}()
	transport, err := newHTTPTransportWithOptions(HTTPClientOptions{TLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	if err := configureLongStreamHTTP2(transport, 30*time.Millisecond, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+listener.Addr().String(), nil)
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err == nil || ctx.Err() != nil || pings.Load() == 0 {
		t.Fatalf("err=%v context=%v pings=%d", err, ctx.Err(), pings.Load())
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("peer was not disconnected")
	}
}

func TestLongStreamProfileShardsAndHTTP1(t *testing.T) {
	resetHTTPClientTestState(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, r.Proto+" "+r.RemoteAddr) }))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	for _, protocol := range []string{dto.HTTPProtocolAuto, dto.HTTPProtocolHTTP1} {
		t.Run(protocol, func(t *testing.T) {
			shards := 1
			if protocol == dto.HTTPProtocolAuto {
				shards = 2
			}
			client, err := NewHttpClientWithOptions(HTTPClientOptions{LongStream: true, TLSInsecureSkipVerify: true, HTTPProtocol: protocol, HTTP2ConnectionShards: shards})
			if err != nil {
				t.Fatal(err)
			}
			defer closeHTTPClientIdleConnections(client)
			var last string
			for range 2 {
				resp, err := client.Get(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if protocol == dto.HTTPProtocolAuto && (resp.ProtoMajor != 2 || string(body) == last) {
					t.Fatalf("H2 shards share a connection: %s", body)
				}
				if protocol == dto.HTTPProtocolHTTP1 && resp.ProtoMajor != 1 {
					t.Fatalf("HTTP1 ignored: %s", resp.Proto)
				}
				last = string(body)
			}
		})
	}
}

func TestLongStreamClientCacheAndDisable(t *testing.T) {
	resetHTTPClientTestState(t)
	options := HTTPClientOptions{LongStream: true}
	stream, err := GetHttpClientWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := GetHttpClientWithOptions(options)
	regular, _ := GetHttpClientWithOptions(HTTPClientOptions{})
	if stream != again || stream == regular {
		t.Fatal("stream profile missing from cache key")
	}
	if regular.Transport.(*http.Transport).TLSNextProto != nil {
		t.Fatal("regular client eagerly configured for streaming")
	}
	InvalidateHTTPClientSettings(dto.ChannelSettings{})
	next, _ := GetHttpClientWithOptions(options)
	if next == stream {
		t.Fatal("stream profile survived channel invalidation")
	}
	t.Setenv("RELAY_H2_READ_IDLE_TIMEOUT", "0")
	disabled, err := NewHttpClientWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Transport.(*http.Transport).TLSNextProto != nil {
		t.Fatal("disabled H2 probe still configured")
	}
}

func TestLongStreamHTTP2ThroughConnectProxy(t *testing.T) {
	resetHTTPClientTestState(t)
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Proto)
	}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	var connects atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		connects.Add(1)
		upstream, err := net.DialTimeout("tcp", origin.Listener.Addr().String(), time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer upstream.Close()
		downstream, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer downstream.Close()
		_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		if err := buffered.Flush(); err != nil {
			return
		}
		copied := make(chan struct{})
		go func() { defer close(copied); _, _ = io.Copy(upstream, buffered) }()
		_, _ = io.Copy(downstream, upstream)
		_ = downstream.Close()
		<-copied
	}))
	defer proxy.Close()
	client, err := NewHttpClientWithOptions(HTTPClientOptions{Proxy: proxy.URL, LongStream: true, TLSInsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer closeHTTPClientIdleConnections(client)
	client.Timeout = 3 * time.Second
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.ProtoMajor != 2 || string(body) != "HTTP/2.0" || connects.Load() != 1 {
		t.Fatalf("proxy connects=%d proto=%s body=%s err=%v", connects.Load(), response.Proto, body, err)
	}
}
