package main

import (
	"slices"
	"testing"
)

// TestBuildCodexArgs_ReadOnlySandbox verifies the opt-in least-privilege tier:
// CODEX_SANDBOX=read-only swaps the full-capability bypass for a read-only
// sandbox, while the default path stays byte-identical (non-breaking).
func TestBuildCodexArgs_ReadOnlySandbox(t *testing.T) {
	base := []string{"--skip-git-repo-check", "-C", "/tmp", "--json", "task"}

	t.Run("default keeps bypass (unchanged)", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "")
		got := buildCodexArgs(&Config{Mode: "new", WorkDir: "/tmp"}, "task")
		want := append([]string{"e", "--dangerously-bypass-approvals-and-sandbox"}, base...)
		if !slices.Equal(got, want) {
			t.Fatalf("default: got %v want %v", got, want)
		}
	})

	t.Run("read-only opt-in replaces bypass with read-only sandbox", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "read-only")
		got := buildCodexArgs(&Config{Mode: "new", WorkDir: "/tmp"}, "task")
		want := append([]string{"e", "--sandbox", "read-only"}, base...)
		if !slices.Equal(got, want) {
			t.Fatalf("read-only: got %v want %v", got, want)
		}
		if slices.Contains(got, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatalf("read-only must not carry the bypass flag: %v", got)
		}
	})

	t.Run("read-only is trimmed and case-insensitive", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "  Read-Only  ")
		got := buildCodexArgs(&Config{Mode: "new", WorkDir: "/tmp"}, "task")
		if !slices.Contains(got, "--sandbox") || slices.Contains(got, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatalf("case-insensitive read-only failed: %v", got)
		}
	})
}

// TestBuildCodexArgs_SandboxSelectionIsThreeWay pins what `--help` now promises
// about CODEX_SANDBOX. The switch has two cases, so an unrecognised sandbox value
// does not simply "fall back to the bypass" — with CODEX_REQUIRE_APPROVAL=true
// neither case fires and codex is launched with no sandbox flag at all, deferring
// to its own configuration. Documenting only the first two outcomes would tell a
// caller they are bypassed when they are not, or sandboxed when they are not.
func TestBuildCodexArgs_SandboxSelectionIsThreeWay(t *testing.T) {
	const bypass = "--dangerously-bypass-approvals-and-sandbox"

	cases := []struct {
		name           string
		sandbox        string
		requireApprove string
		wantSandbox    bool
		wantBypass     bool
	}{
		{"read-only wins over the bypass", "read-only", "", true, false},
		{"read-only wins even when approval is required", "read-only", "true", true, false},
		{"unset falls through to the bypass", "", "", false, true},
		{"unrecognised value falls through to the bypass", "workspace-write", "", false, true},
		{"unrecognised value plus required approval yields neither flag", "workspace-write", "true", false, false},
		{"unset plus required approval yields neither flag", "", "true", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CODEX_SANDBOX", tc.sandbox)
			t.Setenv("CODEX_REQUIRE_APPROVAL", tc.requireApprove)
			got := buildCodexArgs(&Config{Mode: "new", WorkDir: "/tmp"}, "task")

			if slices.Contains(got, "--sandbox") != tc.wantSandbox {
				t.Fatalf("--sandbox presence = %v, want %v: %v", !tc.wantSandbox, tc.wantSandbox, got)
			}
			if slices.Contains(got, bypass) != tc.wantBypass {
				t.Fatalf("bypass presence = %v, want %v: %v", !tc.wantBypass, tc.wantBypass, got)
			}
		})
	}
}
