package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestHandleStreamReplaysAccumulatedContentForLateClient(t *testing.T) {
	ws := NewWebServer("codex")
	ws.StartSession("session-1", "codex", "task")
	ws.SendContentWithType("session-1", "codex", "thinking\n", "reasoning")
	ws.SendContentWithType("session-1", "codex", "answer\n", "message")
	ws.EndSession("session-1", "codex")

	req := httptest.NewRequest(http.MethodGet, "/api/stream/session-1", nil)
	rr := httptest.NewRecorder()

	ws.handleStream(rr, req)

	events := parseSSEContentEvents(t, rr.Body.String())
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2; body:\n%s", len(events), rr.Body.String())
	}

	if events[0].Done {
		t.Fatalf("first event Done=true, want accumulated content event first: %#v", events[0])
	}
	if events[0].SessionID != "session-1" {
		t.Fatalf("first event SessionID=%q, want session-1", events[0].SessionID)
	}
	if events[0].Backend != "codex" {
		t.Fatalf("first event Backend=%q, want codex", events[0].Backend)
	}
	if events[0].Content != "thinking\nanswer\n" {
		t.Fatalf("first event Content=%q, want accumulated content", events[0].Content)
	}
	if events[0].ContentType != "message" {
		t.Fatalf("first event ContentType=%q, want message", events[0].ContentType)
	}

	if !events[1].Done {
		t.Fatalf("second event Done=false, want done event: %#v", events[1])
	}
}

func parseSSEContentEvents(t *testing.T, body string) []ContentEvent {
	t.Helper()

	var events []ContentEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		var event ContentEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatalf("unmarshal SSE event %q: %v", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan SSE body: %v", err)
	}

	return events
}
