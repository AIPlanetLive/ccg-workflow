package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBackendParseJSONStream_UnknownEventsAreSilent(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"turn.started"}`,
		`{"type":"assistant","text":"hi"}`,
		`{"type":"user","text":"yo"}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}`,
	}, "\n")

	var infos []string
	infoFn := func(msg string) { infos = append(infos, msg) }

	message, threadID := parseJSONStreamInternal(strings.NewReader(input), nil, infoFn, nil, nil)
	if message != "ok" {
		t.Fatalf("message=%q, want %q (infos=%v)", message, "ok", infos)
	}
	if threadID != "" {
		t.Fatalf("threadID=%q, want empty (infos=%v)", threadID, infos)
	}

	for _, msg := range infos {
		if strings.Contains(msg, "Agent event:") {
			t.Fatalf("unexpected log for unknown event: %q", msg)
		}
	}
}

func TestParseJSONStreamInternalWithContent_EmitsProgressLines(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"thread.started","thread_id":"tid-123"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"type":"reasoning","text":"Checking files and APIs"}}`,
		`{"type":"item.completed","item":{"type":"mcp_tool_call"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"echo hi","aggregated_output":"hi\n","exit_code":0}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"Done with changes"}}`,
		`{"type":"turn.completed","thread_id":"tid-123"}`,
	}, "\n")

	var progress []string
	message, threadID := parseJSONStreamInternalWithContent(
		strings.NewReader(input),
		nil,
		func(string) {},
		nil,
		nil,
		nil,
		func(line string) { progress = append(progress, line) },
		nil,
	)

	if message != "Done with changes" {
		t.Fatalf("message=%q, want %q", message, "Done with changes")
	}
	if threadID != "tid-123" {
		t.Fatalf("threadID=%q, want %q", threadID, "tid-123")
	}

	joined := strings.Join(progress, "\n")
	for _, want := range []string{
		"[PROGRESS] session_started id=tid-123",
		"[PROGRESS] turn_started",
		"[PROGRESS] reasoning text=\"Checking files and APIs\"",
		"[PROGRESS] mcp_call",
		"[PROGRESS] cmd_done cmd=\"echo hi\" exit=0",
		"[PROGRESS] message text=\"Done with changes\"",
		"[PROGRESS] turn_completed total_events=7",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing progress %q in %q", want, joined)
		}
	}
}

func TestParseJSONStreamInternalWithContent_ClaudeStreamingAssistantBlocks(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"claude-1"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Working on it"},{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"go test ./..."}},{"type":"thinking","thinking":"Need to verify parser routing"}]},"session_id":"claude-1"}`,
		`{"type":"result","subtype":"success","result":"Final answer","session_id":"claude-1"}`,
	}, "\n")

	type contentEvent struct {
		content     string
		contentType string
	}
	var content []contentEvent
	var progress []string
	message, threadID := parseJSONStreamInternalWithContent(
		strings.NewReader(input),
		nil,
		func(string) {},
		nil,
		nil,
		func(c, ct string) { content = append(content, contentEvent{content: c, contentType: ct}) },
		func(line string) { progress = append(progress, line) },
		nil,
	)

	if message != "Final answer" {
		t.Fatalf("message=%q, want %q", message, "Final answer")
	}
	if threadID != "claude-1" {
		t.Fatalf("threadID=%q, want %q", threadID, "claude-1")
	}

	wantContent := []contentEvent{
		{content: "Working on it", contentType: "message"},
		{content: `$ Bash {"command":"go test ./..."}`, contentType: "command"},
		{content: "Need to verify parser routing", contentType: "reasoning"},
		{content: "Final answer", contentType: "message"},
	}
	if !reflect.DeepEqual(content, wantContent) {
		t.Fatalf("content=%#v, want %#v", content, wantContent)
	}

	joined := strings.Join(progress, "\n")
	for _, want := range []string{
		`[PROGRESS] message text="Working on it"`,
		`[PROGRESS] tool_use cmd="Bash {\"command\":\"go test ./...\"}"`,
		`[PROGRESS] reasoning text="Need to verify parser routing"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing progress %q in %q", want, joined)
		}
	}
}

func TestParseJSONStreamInternalWithContent_ClaudeStreamingToolResult(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":[{"type":"text","text":"command output"}]}]},"session_id":"claude-2"}`,
		`{"type":"result","subtype":"success","result":"Done","session_id":"claude-2"}`,
	}, "\n")

	var content []string
	var contentTypes []string
	var progress []string
	message, threadID := parseJSONStreamInternalWithContent(
		strings.NewReader(input),
		nil,
		func(string) {},
		nil,
		nil,
		func(c, ct string) {
			content = append(content, c)
			contentTypes = append(contentTypes, ct)
		},
		func(line string) { progress = append(progress, line) },
		nil,
	)

	if message != "Done" {
		t.Fatalf("message=%q, want %q", message, "Done")
	}
	if threadID != "claude-2" {
		t.Fatalf("threadID=%q, want %q", threadID, "claude-2")
	}
	if len(content) < 1 || len(contentTypes) < 1 {
		t.Fatalf("expected tool_result content callback, got content=%v types=%v", content, contentTypes)
	}
	if content[0] != "command output" || contentTypes[0] != "message" {
		t.Fatalf("first content=(%q,%q), want (%q,%q); all content=%v types=%v", content[0], contentTypes[0], "command output", "message", content, contentTypes)
	}
	joined := strings.Join(progress, "\n")
	if !strings.Contains(joined, "[PROGRESS] tool_result") {
		t.Fatalf("missing tool_result progress in %q", joined)
	}
}

func TestParseJSONStreamInternalWithContent_ClaudeTerminalResultUnchanged(t *testing.T) {
	input := `{"type":"result","subtype":"success","result":"terminal only","session_id":"claude-3"}`

	message, threadID := parseJSONStreamInternalWithContent(
		strings.NewReader(input),
		nil,
		func(string) {},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if message != "terminal only" {
		t.Fatalf("message=%q, want %q", message, "terminal only")
	}
	if threadID != "claude-3" {
		t.Fatalf("threadID=%q, want %q", threadID, "claude-3")
	}
}

func TestParseJSONStreamInternalWithContent_ClaudeStreamingDoesNotMisrouteCodexOrGemini(t *testing.T) {
	t.Run("codex", func(t *testing.T) {
		input := `{"type":"item.completed","item":{"type":"agent_message","text":"codex ok"}}`

		var content []string
		message, threadID := parseJSONStreamInternalWithContent(
			strings.NewReader(input),
			nil,
			func(string) {},
			nil,
			nil,
			func(c, ct string) { content = append(content, ct+":"+c) },
			nil,
			nil,
		)

		if message != "codex ok" {
			t.Fatalf("message=%q, want %q", message, "codex ok")
		}
		if threadID != "" {
			t.Fatalf("threadID=%q, want empty", threadID)
		}
		if !reflect.DeepEqual(content, []string{"agent_message:codex ok"}) {
			t.Fatalf("content=%v, want codex agent_message content", content)
		}
	})

	t.Run("gemini", func(t *testing.T) {
		input := strings.Join([]string{
			`{"type":"message","role":"assistant","content":"gemini","delta":true,"session_id":"gemini-1"}`,
			`{"type":"result","status":"success","session_id":"gemini-1"}`,
		}, "\n")

		var content []string
		message, threadID := parseJSONStreamInternalWithContent(
			strings.NewReader(input),
			nil,
			func(string) {},
			nil,
			nil,
			func(c, ct string) { content = append(content, ct+":"+c) },
			nil,
			nil,
		)

		if message != "gemini" {
			t.Fatalf("message=%q, want %q", message, "gemini")
		}
		if threadID != "gemini-1" {
			t.Fatalf("threadID=%q, want %q", threadID, "gemini-1")
		}
		if !reflect.DeepEqual(content, []string{"message:gemini"}) {
			t.Fatalf("content=%v, want gemini message content", content)
		}
	})
}

func TestSafeProgressSnippet_UsesRuneSafeTruncation(t *testing.T) {
	got := safeProgressSnippet("中文测试进度输出", 5)
	if got != "中文..." {
		t.Fatalf("got %q, want %q", got, "中文...")
	}

	got = safeProgressSnippet("中文", 2)
	if got != "中文" {
		t.Fatalf("got %q, want %q", got, "中文")
	}
}

func TestFormatProgressLine_HandlesNilFields(t *testing.T) {
	if got := formatProgressLine("turn_started", nil); got != "turn_started" {
		t.Fatalf("got %q, want %q", got, "turn_started")
	}
}

func TestParseArgs_ParsesProgressFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"codeagent-wrapper", "--progress", "task body", "/tmp/work"}
	cfg, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs error: %v", err)
	}
	if !cfg.Progress {
		t.Fatalf("expected Progress=true")
	}
	if cfg.Task != "task body" || cfg.WorkDir != "/tmp/work" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
