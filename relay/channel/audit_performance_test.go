package channel

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Runs the real transport pipeline against a local RoundTripper. No provider IO.
func BenchmarkAuditOutboundBody(b *testing.B) {
	service.InitHttpClient()
	oldDisk := common.GetDiskCacheConfig()
	common.OptionMapRWMutex.Lock()
	oldOptions := common.OptionMap
	common.OptionMap = map[string]string{"header_rule_setting": `{"enabled":false,"rules":[]}`}
	common.OptionMapRWMutex.Unlock()
	b.Cleanup(func() {
		common.SetDiskCacheConfig(oldDisk)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptions
		common.OptionMapRWMutex.Unlock()
	})
	for _, size := range []int{1, 10, 50} {
		for _, identity := range []bool{false, true} {
			for _, disk := range []bool{false, true} {
				b.Run(fmt.Sprintf("%dMiB/identity=%t/disk=%t", size, identity, disk), func(b *testing.B) {
					common.SetDiskCacheConfig(common.DiskCacheConfig{Enabled: disk, ThresholdMB: 0, MaxSizeMB: 1024, Path: b.TempDir()})
					common.OptionMapRWMutex.Lock()
					common.OptionMap["client_identity_setting"] = fmt.Sprintf(`{"enabled":%t,"codex":{"enabled":true,"mode":"force_global","installation_id":"audit"}}`, identity)
					common.OptionMapRWMutex.Unlock()
					payload := append([]byte(`{"model":"test","input":"`), bytes.Repeat([]byte("x"), size<<20)...)
					payload = append(payload, []byte(`"}`)...)
					runtime.GC()
					settings := dto.ChannelSettings{RequiresCodexIdentity: common.GetPointer(identity)}
					client, err := service.GetHttpClientWithChannelSettings(settings)
					if err != nil {
						b.Fatal(err)
					}
					var activeHeap, steadyHeap uint64
					calls := 0
					originalTransport := client.Transport
					defer func() { client.Transport = originalTransport }()
					client.Transport = normalizationRoundTripper(func(request *http.Request) (*http.Response, error) {
						calls++
						first := sha256.New()
						if _, err := io.Copy(first, request.Body); err != nil {
							return nil, err
						}
						replay, err := request.GetBody()
						if err != nil {
							return nil, err
						}
						second := sha256.New()
						_, err = io.Copy(second, replay)
						_ = replay.Close()
						if err != nil {
							return nil, err
						}
						if !bytes.Equal(first.Sum(nil), second.Sum(nil)) {
							return nil, fmt.Errorf("replay differs")
						}
						runtime.GC()
						var active runtime.MemStats
						runtime.ReadMemStats(&active)
						activeHeap = active.HeapAlloc
						runtime.GC()
						runtime.ReadMemStats(&active)
						steadyHeap = active.HeapAlloc
						stats := common.GetDiskCacheStats()
						if disk && stats.ActiveDiskFiles == 0 {
							return nil, fmt.Errorf("disk mode did not use disk storage")
						}
						return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader([]byte(`{}`))), Request: request}, nil
					})
					b.ReportAllocs()
					b.SetBytes(int64(len(payload)))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						c, _ := gin.CreateTestContext(httptest.NewRecorder())
						c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
						info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI, ChannelSetting: settings}}
						response, err := DoApiRequest(normalizationTestAdaptor{}, c, info, bytes.NewReader(payload))
						if err != nil {
							b.Fatal(err)
						}
						_ = response.Body.Close()
					}
					b.StopTimer()
					if calls != b.N {
						b.Fatal("local transport was not used")
					}
					b.ReportMetric(float64(activeHeap)/(1<<20), "active_heap_MiB")
					b.ReportMetric(float64(steadyHeap)/(1<<20), "steady_heap_MiB")
				})
			}
		}
	}
}
