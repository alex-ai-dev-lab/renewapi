package service

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

func TestHTTPClientConcurrentResetAndLookup(t *testing.T) {
	resetHTTPClientTestState(t)
	settings := system_setting.GetFetchSetting()
	previous := settings.EnableSSRFProtection
	settings.EnableSSRFProtection = true
	t.Cleanup(func() { settings.EnableSSRFProtection = previous })
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 40 {
				if worker == 0 {
					ResetProxyClientCache()
					continue
				}
				if client := GetHttpClient(); client == nil {
					t.Error("default client disappeared during reset")
				}
				for _, options := range []HTTPClientOptions{{}, {LongStream: true}, {LongStream: true, HTTP2ConnectionShards: 2}, {HTTPProtocol: "http1"}} {
					if client, err := GetHttpClientWithOptions(options); err != nil || client == nil {
						t.Errorf("relay lookup: %v", err)
					}
					if client, err := GetSSRFProtectedHTTPClientWithOptions(options); err != nil || client == nil {
						t.Errorf("protected lookup: %v", err)
					}
				}
			}
		}()
	}
	wg.Wait()
}

func TestHTTPClientResetPreservesActiveResponse(t *testing.T) {
	resetHTTPClientTestState(t)
	configureSSRFTestFetchSetting(t, true)
	for _, profile := range []struct {
		name      string
		protected bool
		options   HTTPClientOptions
		protocol  int
	}{
		{"ordinary-http1", false, HTTPClientOptions{HTTPProtocol: "http1"}, 1},
		{"stream-h2-shards", false, HTTPClientOptions{LongStream: true, HTTP2ConnectionShards: 2}, 2},
		{"protected-h2", true, HTTPClientOptions{}, 2},
	} {
		t.Run(profile.name, func(t *testing.T) {
			finish := make(chan struct{})
			upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "ready\n")
				w.(http.Flusher).Flush()
				select {
				case <-finish:
					_, _ = io.WriteString(w, "done\n")
				case <-r.Context().Done():
				}
			}))
			upstream.EnableHTTP2 = true
			upstream.StartTLS()
			defer upstream.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			options := profile.options
			options.TLSInsecureSkipVerify = true
			lookup := GetHttpClientWithOptions
			if profile.protected {
				lookup = GetSSRFProtectedHTTPClientWithOptions
			}
			client, err := lookup(options)
			if err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, upstream.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			reader := bufio.NewReader(resp.Body)
			if line, err := reader.ReadString('\n'); err != nil || line != "ready\n" {
				t.Fatalf("response did not start: %q, %v", line, err)
			}
			if resp.ProtoMajor != profile.protocol {
				t.Fatalf("protocol = %s", resp.Proto)
			}
			ResetProxyClientCache()
			replacement, err := lookup(options)
			if err != nil || replacement == client {
				t.Fatalf("reset did not replace cached client: %v", err)
			}
			close(finish)
			if rest, err := io.ReadAll(reader); err != nil || string(rest) != "done\n" {
				t.Fatalf("reset interrupted active response: %q, %v", rest, err)
			}
		})
	}
}

func BenchmarkHTTPClientLookup(b *testing.B) {
	InitHttpClient()
	b.Cleanup(ResetProxyClientCache)
	for _, profile := range []struct {
		name    string
		options HTTPClientOptions
	}{
		{"default", HTTPClientOptions{}},
		{"stream", HTTPClientOptions{LongStream: true}},
	} {
		b.Run(profile.name, func(b *testing.B) {
			if _, err := GetHttpClientWithOptions(profile.options); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if client, err := GetHttpClientWithOptions(profile.options); err != nil || client == nil {
						b.Errorf("lookup: %v", err)
					}
				}
			})
		})
	}
}
