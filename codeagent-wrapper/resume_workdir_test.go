package main

import (
	"slices"
	"testing"
)

// TestBuildCodexArgs_ResumeCarriesWorkdir pins the fix for a silent-failure bug:
// resume used to omit -C, so the resumed agent inherited the caller's cwd instead
// of the workdir it was given. Nothing errored — the agent simply worked on the
// wrong tree, which under git-worktree isolation meant an independent reviewer
// reviewing a different checkout than the one under review.
//
// -C is an option of `codex exec`, parsed before the `resume` subcommand, so it
// applies to both modes identically. Verified against codex-cli 0.146.0:
// `codex exec -C /tmp resume --help` exits 0, while an unknown flag in the same
// position exits 2 with "unexpected argument".
func TestBuildCodexArgs_ResumeCarriesWorkdir(t *testing.T) {
	t.Run("resume passes -C with the configured workdir", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "")
		got := buildCodexArgs(&Config{Mode: "resume", SessionID: "sess-1", WorkDir: "/tmp/wt"}, "task")

		i := slices.Index(got, "-C")
		if i < 0 {
			t.Fatalf("resume must pass -C so the agent does not inherit the caller cwd: %v", got)
		}
		if i+1 >= len(got) || got[i+1] != "/tmp/wt" {
			t.Fatalf("-C must carry the configured workdir: %v", got)
		}
	})

	t.Run("resume and new mode agree on the workdir they hand codex", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "")
		resumed := buildCodexArgs(&Config{Mode: "resume", SessionID: "sess-1", WorkDir: "/tmp/wt"}, "task")
		fresh := buildCodexArgs(&Config{Mode: "new", WorkDir: "/tmp/wt"}, "task")

		if workdirAfterC(t, resumed) != workdirAfterC(t, fresh) {
			t.Fatalf("resume and new disagree on workdir: resume=%v new=%v", resumed, fresh)
		}
	})

	t.Run("resume still carries the session id and the resume subcommand", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "")
		got := buildCodexArgs(&Config{Mode: "resume", SessionID: "sess-1", WorkDir: "/tmp/wt"}, "task")

		r := slices.Index(got, "resume")
		if r < 0 || r+1 >= len(got) || got[r+1] != "sess-1" {
			t.Fatalf("resume subcommand and session id must survive the -C addition: %v", got)
		}
		// -C belongs to `codex exec`, so it must precede the subcommand.
		if c := slices.Index(got, "-C"); c > r {
			t.Fatalf("-C must precede the resume subcommand: %v", got)
		}
	})

	t.Run("read-only sandbox composes with resume", func(t *testing.T) {
		t.Setenv("CODEX_SANDBOX", "read-only")
		got := buildCodexArgs(&Config{Mode: "resume", SessionID: "sess-1", WorkDir: "/tmp/wt"}, "task")

		if !slices.Contains(got, "--sandbox") || !slices.Contains(got, "-C") {
			t.Fatalf("read-only resume must carry both --sandbox and -C: %v", got)
		}
		if slices.Contains(got, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatalf("read-only resume must not carry the bypass flag: %v", got)
		}
	})
}

func workdirAfterC(t *testing.T, args []string) string {
	t.Helper()
	i := slices.Index(args, "-C")
	if i < 0 || i+1 >= len(args) {
		t.Fatalf("no -C in %v", args)
	}
	return args[i+1]
}
