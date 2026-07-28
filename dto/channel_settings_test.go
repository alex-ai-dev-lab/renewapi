package dto

import "testing"

func TestChannelSettingsValidateHTTPTransport(t *testing.T) {
	tests := []struct {
		name    string
		setting ChannelSettings
		wantErr bool
	}{
		{name: "default"},
		{name: "auto shards", setting: ChannelSettings{HTTPProtocol: HTTPProtocolAuto, HTTP2ConnectionShards: 8}},
		{name: "http1", setting: ChannelSettings{HTTPProtocol: HTTPProtocolHTTP1, HTTP2ConnectionShards: 1}},
		{name: "invalid protocol", setting: ChannelSettings{HTTPProtocol: "h3"}, wantErr: true},
		{name: "too many shards", setting: ChannelSettings{HTTP2ConnectionShards: 9}, wantErr: true},
		{name: "http1 shards", setting: ChannelSettings{HTTPProtocol: HTTPProtocolHTTP1, HTTP2ConnectionShards: 2}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.setting.ValidateHTTPTransport()
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateHTTPTransport() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
