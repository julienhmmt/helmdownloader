package tui

import "testing"

func TestShellArg(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"/out/redis.tar.gz": "/out/redis.tar.gz",
		"/my out/a.tar.gz":  "'/my out/a.tar.gz'",
		"/x/it's.tar.gz":    `'/x/it'\''s.tar.gz'`,
		"/x/$HOME.tar.gz":   "'/x/$HOME.tar.gz'",
	}
	for in, want := range cases {
		if got := shellArg(in); got != want {
			t.Errorf("shellArg(%q) = %q, want %q", in, got, want)
		}
	}
}
