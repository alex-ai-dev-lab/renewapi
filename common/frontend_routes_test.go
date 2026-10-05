package common

import "testing"

func TestRetiredFrontendRoutesPreserveQueries(t *testing.T) {
	cases := map[string]string{
		"/console/token?group=default":     "/keys?group=default",
		"/console/log?start_timestamp=123": "/usage-logs/common?start_timestamp=123",
		"/console/topup/":                  "/wallet", "/console": "/dashboard/overview",
		"/login?redirect=%2Fconsole": "/sign-in?redirect=%2Fconsole",
		"/api/user/self":             "/api/user/self", "/console/token-extra": "/console/token-extra",
	}
	for input, want := range cases {
		if got := ThemeAwarePath(input); got != want {
			t.Errorf("%s -> %s; want %s", input, got, want)
		}
	}
}
