package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A run killed by an external signal (exit 130) must leave a durable,
// well-formed terminal record carrying the resume handle (session_id).
func TestWriteTerminalRecord_KilledRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEAGENT_STATE_DIR", dir)

	result := TaskResult{
		ExitCode:  130,
		SessionID: "sess-abc123",
		Error:     "execution cancelled",
		Message:   "partial work before kill",
	}
	path, err := writeTerminalRecord(&Config{Backend: "codex"}, result)
	if err != nil {
		t.Fatalf("writeTerminalRecord: %v", err)
	}
	if got, want := filepath.Dir(path), filepath.Join(dir, "results"); got != want {
		t.Fatalf("record dir = %s, want %s", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var rec terminalRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("record is not valid json: %v (%s)", err, data)
	}
	if rec.SessionID != "sess-abc123" {
		t.Errorf("session_id = %q, want sess-abc123 (resume handle lost)", rec.SessionID)
	}
	if rec.ExitCode != 130 || rec.Backend != "codex" || rec.Reason != "execution cancelled" {
		t.Errorf("record fields wrong: %+v", rec)
	}
	if rec.Timestamp == "" {
		t.Error("record missing timestamp")
	}
}

// Overwriting is atomic and the message is bounded so a runaway backend cannot
// bloat the record.
func TestWriteTerminalRecord_MessageBounded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEAGENT_STATE_DIR", dir)

	big := strings.Repeat("x", maxRecordMessageBytes+1024)
	path, err := writeTerminalRecord(&Config{Backend: "codex"}, TaskResult{Message: big})
	if err != nil {
		t.Fatalf("writeTerminalRecord: %v", err)
	}
	data, _ := os.ReadFile(path)
	var rec terminalRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(rec.Message) != maxRecordMessageBytes {
		t.Fatalf("message len = %d, want bounded to %d", len(rec.Message), maxRecordMessageBytes)
	}
	// No leftover temp file in the results dir.
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestResolveStateDir_EnvOverride(t *testing.T) {
	t.Setenv("CODEAGENT_STATE_DIR", "/custom/state/path")
	if got := resolveStateDir(); got != "/custom/state/path" {
		t.Fatalf("resolveStateDir() = %q, want /custom/state/path", got)
	}
}

// Old failure records are pruned; recent ones (incl. a killed run awaiting
// resume) are kept.
func TestCleanupOldResults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEAGENT_STATE_DIR", dir)
	resultsDir := filepath.Join(dir, "results")
	if err := os.MkdirAll(resultsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(resultsDir, "codeagent-wrapper-111.result.json")
	fresh := filepath.Join(resultsDir, "codeagent-wrapper-222.result.json")
	if err := os.WriteFile(old, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-resultRetention - time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	cleanupOldResults()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old record should be pruned, stat err = %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh record should be kept: %v", err)
	}
}
