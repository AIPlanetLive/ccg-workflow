package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeTestExecutable(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func TestResolveBackendCommandPrefersPATH(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	pathCommand := filepath.Join(pathDir, "codex")
	localCommand := filepath.Join(home, ".local", "bin", "codex")
	writeTestExecutable(t, pathCommand, 0o755)
	writeTestExecutable(t, localCommand, 0o755)
	t.Setenv("HOME", home)
	t.Setenv("PATH", pathDir)

	if got := resolveBackendCommand("codex"); got != "codex" {
		t.Fatalf("resolveBackendCommand(codex) = %q, want original PATH command; PATH executable is %q", got, pathCommand)
	}
}

func TestResolveBackendCommandPreservesErrDot(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	workDir := t.TempDir()
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", workDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore Chdir(%q) error = %v", originalDir, err)
		}
	})

	writeTestExecutable(t, filepath.Join(workDir, "codex"), 0o755)
	home := t.TempDir()
	localCommand := filepath.Join(home, ".local", "bin", "codex")
	writeTestExecutable(t, localCommand, 0o755)
	t.Setenv("HOME", home)
	t.Setenv("PATH", ".")

	if _, err := exec.LookPath("codex"); !errors.Is(err, exec.ErrDot) {
		t.Fatalf("LookPath(codex) error = %v, want exec.ErrDot", err)
	}
	if got := resolveBackendCommand("codex"); got != "codex" {
		t.Fatalf("resolveBackendCommand(codex) = %q, want original command to preserve exec.ErrDot", got)
	}
}

func TestResolveBackendCommandFallsBackToUserLocalBin(t *testing.T) {
	for _, backend := range []string{"codex", "claude", "gemini"} {
		t.Run(backend, func(t *testing.T) {
			home := t.TempDir()
			localCommand := filepath.Join(home, ".local", "bin", backend)
			writeTestExecutable(t, localCommand, 0o755)
			t.Setenv("HOME", home)
			t.Setenv("PATH", t.TempDir())

			if got := resolveBackendCommand(backend); got != localCommand {
				t.Fatalf("resolveBackendCommand(%s) = %q, want fallback %q", backend, got, localCommand)
			}
		})
	}
}

func TestResolveBackendCommandRejectsNonExecutableFallback(t *testing.T) {
	home := t.TempDir()
	writeTestExecutable(t, filepath.Join(home, ".local", "bin", "codex"), 0o644)
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	if got := resolveBackendCommand("codex"); got != "codex" {
		t.Fatalf("resolveBackendCommand(codex) = %q, want original command", got)
	}
}

func TestResolveBackendCommandKeepsOriginalOnMiss(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	if got := resolveBackendCommand("codex"); got != "codex" {
		t.Fatalf("resolveBackendCommand(codex) = %q, want original command", got)
	}
}

func TestResolvedCodexCommandKeepsLogicalBackendCwdBehavior(t *testing.T) {
	defer resetTestHooks()

	home := t.TempDir()
	localCommand := filepath.Join(home, ".local", "bin", "codex")
	writeTestExecutable(t, localCommand, 0o755)
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	var runner *execFakeRunner
	var command string
	newCommandRunner = func(ctx context.Context, name string, args ...string) commandRunner {
		command = name
		runner = &execFakeRunner{
			stdout:  newReasonReadCloser(`{"type":"item.completed","item":{"type":"agent_message","text":"resolved"}}`),
			process: &execFakeProcess{pid: 17},
		}
		return runner
	}

	workDir := t.TempDir()
	res := runCodexTaskWithContext(context.Background(), TaskSpec{Task: "payload", WorkDir: workDir}, CodexBackend{}, nil, false, false, 1)
	if res.ExitCode != 0 || res.Message != "resolved" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if command != localCommand {
		t.Fatalf("command = %q, want resolved command %q", command, localCommand)
	}
	if runner == nil || runner.dir != "" {
		t.Fatalf("resolved codex must leave cmd.Dir unset, runner=%v dir=%q", runner, runner.dir)
	}
}
