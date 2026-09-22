package operation_setting

import (
	"encoding/json"
	"sync"
)

type ChannelTestSettingT struct {
	Prompt                   string `json:"prompt"`
	MaxTokens                int    `json:"max_tokens"`
	ReasoningEffort          string `json:"reasoning_effort"`
	EndpointType             string `json:"endpoint_type"`
	StreamMode               string `json:"stream_mode"`
	TimeoutSeconds           int    `json:"timeout_seconds"`
	AutoTestOnlyAutoDisabled bool   `json:"auto_test_only_auto_disabled"`
}

func defaultChannelTestSetting() ChannelTestSettingT {
	return ChannelTestSettingT{
		Prompt: "hi",
		// 0 means "use the model-specific default" (e.g. 50 for thinking models,
		// 3000 for gemini). A non-zero global value is treated as an explicit cap.
		MaxTokens:  0,
		StreamMode: "auto",
	}
}

var channelTestSetting = defaultChannelTestSetting()
var channelTestSettingMutex sync.RWMutex

func GetChannelTestSetting() ChannelTestSettingT {
	channelTestSettingMutex.RLock()
	defer channelTestSettingMutex.RUnlock()
	return channelTestSetting
}

func ChannelTestSetting2JsonString() string {
	channelTestSettingMutex.RLock()
	defer channelTestSettingMutex.RUnlock()
	b, _ := json.Marshal(channelTestSetting)
	return string(b)
}

func UpdateChannelTestSettingByJsonString(s string) error {
	ns := defaultChannelTestSetting()
	if err := json.Unmarshal([]byte(s), &ns); err != nil {
		return err
	}
	channelTestSettingMutex.Lock()
	channelTestSetting = ns
	channelTestSettingMutex.Unlock()
	return nil
}
