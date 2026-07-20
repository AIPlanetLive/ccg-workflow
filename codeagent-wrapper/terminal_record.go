package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// terminalRecord is a durable, structured summary of a single-task wrapper run's
// outcome. It exists so a run terminated by an external signal (SIGTERM →
// exit 130, e.g. a monitoring patrol that misjudged a quiet-but-working task)
// leaves an auditable result and a resume handle (session_id) on disk, instead
// of vanishing with its temp log. See docs/issues H8.
type terminalRecord struct {
	Backend   string `json:"backend"`
	PID       int    `json:"pid"`
	ExitCode  int    `json:"exit_code"`
	SessionID string `json:"session_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
	LogPath   string `json:"log_path,omitempty"`
	Timestamp string `json:"timestamp"`
}

const maxRecordMessageBytes = 8192

// resolveStateDir returns a durable base directory for wrapper state that,
// unlike os.TempDir(), is not swept by the OS between runs. Override with
// CODEAGENT_STATE_DIR; otherwise $XDG_STATE_HOME/codeagent-wrapper, else
// ~/.local/state/codeagent-wrapper. Returns "" if none can be resolved.
func resolveStateDir() string {
	if d := strings.TrimSpace(os.Getenv("CODEAGENT_STATE_DIR")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); d != "" {
		return filepath.Join(d, "codeagent-wrapper")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "state", "codeagent-wrapper")
	}
	return ""
}

// writeTerminalRecord persists a terminal record for the run and returns its
// path. Best-effort: any failure returns an error the caller may log but must
// not treat as fatal (the run's own exit code stays authoritative). The write
// is atomic (temp file + rename) so a concurrent reader never sees a partial
// record.
func writeTerminalRecord(cfg *Config, result TaskResult) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("nil config")
	}
	base := resolveStateDir()
	if base == "" {
		return "", fmt.Errorf("no durable state dir available")
	}
	dir := filepath.Join(base, "results")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	msg := result.Message
	if len(msg) > maxRecordMessageBytes {
		msg = msg[:maxRecordMessageBytes]
		// Trim a possibly-split trailing UTF-8 rune so the record stays clean.
		for len(msg) > 0 && !utf8.ValidString(msg) {
			msg = msg[:len(msg)-1]
		}
	}
	rec := terminalRecord{
		Backend:   cfg.Backend,
		PID:       os.Getpid(),
		ExitCode:  result.ExitCode,
		SessionID: result.SessionID,
		Reason:    result.Error,
		Message:   msg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if l := activeLogger(); l != nil {
		rec.LogPath = l.Path()
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, fmt.Sprintf("%s-%d.result.json", primaryLogPrefix(), os.Getpid()))
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, nil
}

const resultRetention = 14 * 24 * time.Hour

// cleanupOldResults removes terminal-record files older than resultRetention so
// failure records do not accumulate unbounded. Best-effort; called at startup.
func cleanupOldResults() {
	base := resolveStateDir()
	if base == "" {
		return
	}
	dir := filepath.Join(base, "results")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-resultRetention)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".result.json") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}
