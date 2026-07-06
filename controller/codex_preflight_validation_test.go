package controller

import (
	"os"
	"testing"
)

func TestValidateCodexPreflightBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "chatgpt", raw: "https://chatgpt.com", wantErr: false},
		{name: "chatgpt subdomain", raw: "https://foo.chatgpt.com/", wantErr: false},
		{name: "http rejected", raw: "http://chatgpt.com", wantErr: true},
		{name: "localhost rejected", raw: "https://localhost:8080", wantErr: true},
		{name: "ip rejected", raw: "https://127.0.0.1", wantErr: true},
		{name: "lookalike rejected", raw: "https://chatgpt.com.evil.example", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateCodexPreflightBaseURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCodexPreflightBaseURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCodexPreflightBaseURLAllowsConfiguredHost(t *testing.T) {
	old := os.Getenv("CODEX_PREFLIGHT_ALLOWED_HOSTS")
	t.Cleanup(func() {
		_ = os.Setenv("CODEX_PREFLIGHT_ALLOWED_HOSTS", old)
	})
	_ = os.Setenv("CODEX_PREFLIGHT_ALLOWED_HOSTS", "codex.example.com")

	if _, err := validateCodexPreflightBaseURL("https://codex.example.com"); err != nil {
		t.Fatalf("configured host should be allowed: %v", err)
	}
}

func TestValidateCodexPreflightProxy(t *testing.T) {
	if err := validateCodexPreflightProxy("socks5://127.0.0.1:1080"); err != nil {
		t.Fatalf("socks5 proxy should be allowed: %v", err)
	}
	if err := validateCodexPreflightProxy("file:///tmp/proxy"); err == nil {
		t.Fatal("file proxy should be rejected")
	}
}
