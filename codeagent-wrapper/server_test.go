package main

import (
	"os"
	"testing"
)

// TestMain defaults the Web UI browser auto-open OFF for the whole test suite,
// so plain `go test` does not spawn a browser tab per WebServer-starting test.
// An explicit CODEAGENT_OPEN_BROWSER in the environment is respected.
func TestMain(m *testing.M) {
	if _, ok := os.LookupEnv("CODEAGENT_OPEN_BROWSER"); !ok {
		os.Setenv("CODEAGENT_OPEN_BROWSER", "false")
	}
	os.Exit(m.Run())
}

func TestBrowserAutoOpenEnabled(t *testing.T) {
	cases := []struct {
		name string
		env  string
		set  bool
		want bool
	}{
		{name: "default unset opens", set: false, want: true},
		{name: "empty opens", env: "", set: true, want: true},
		{name: "false disables", env: "false", set: true, want: false},
		{name: "FALSE disables (case-insensitive)", env: "FALSE", set: true, want: false},
		{name: "zero disables", env: "0", set: true, want: false},
		{name: "off disables", env: "off", set: true, want: false},
		{name: "true opens", env: "true", set: true, want: true},
		{name: "one opens", env: "1", set: true, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("CODEAGENT_OPEN_BROWSER", tc.env)
			} else {
				// Ensure a clean default even if the ambient env sets it.
				t.Setenv("CODEAGENT_OPEN_BROWSER", "")
			}
			if got := browserAutoOpenEnabled(); got != tc.want {
				t.Fatalf("browserAutoOpenEnabled() with env=%q set=%v = %v, want %v", tc.env, tc.set, got, tc.want)
			}
		})
	}
}
