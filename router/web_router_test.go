package router

import "testing"

func TestIsWebStaticAssetPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "default javascript", path: "/static/js/async/3479.js", want: true},
		{name: "default stylesheet", path: "/static/css/index.css", want: true},
		{name: "classic asset", path: "/assets/index-abc123.js", want: true},
		{name: "root public image", path: "/logo.png", want: true},
		{name: "root favicon", path: "/favicon.ico", want: true},
		{name: "root robots", path: "/robots.txt", want: true},
		{name: "public route", path: "/sign-in", want: false},
		{name: "authenticated route", path: "/profile", want: false},
		{name: "nested api-like file", path: "/api/export.csv", want: false},
		{name: "well-known challenge", path: "/.well-known/acme-challenge/token", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isWebStaticAssetPath(tt.path); got != tt.want {
				t.Fatalf("isWebStaticAssetPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
