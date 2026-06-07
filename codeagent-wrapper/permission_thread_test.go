package main

import (
	"context"
	"testing"
)

// TestSkipPermissionsThreadedToExecArgs is a regression test for the bug where
// runCodexTaskWithContext rebuilt its Config from the TaskSpec but dropped
// SkipPermissions. The banner (built from the original Config) advertised
// --dangerously-skip-permissions while the actual exec omitted it, so the
// claude backend could read but never write. This verifies the flag now flows
// from TaskSpec.SkipPermissions into the real backend argument list.
func TestSkipPermissionsThreadedToExecArgs(t *testing.T) {
	origRunner := newCommandRunner
	defer func() { newCommandRunner = origRunner }()

	run := func(skip bool) []string {
		var captured []string
		newCommandRunner = func(ctx context.Context, name string, args ...string) commandRunner {
			captured = append([]string(nil), args...)
			return &execFakeRunner{
				stdout:  newReasonReadCloser(`{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}`),
				process: &execFakeProcess{pid: 4321},
			}
		}
		res := runCodexTaskWithContext(
			context.Background(),
			TaskSpec{ID: "t", Task: "payload", WorkDir: ".", SkipPermissions: skip},
			ClaudeBackend{}, nil, false, true, 5,
		)
		if res.ExitCode != 0 {
			t.Fatalf("unexpected exit code %d (err=%q) for skip=%v", res.ExitCode, res.Error, skip)
		}
		return captured
	}

	hasFlag := func(args []string) bool {
		for _, a := range args {
			if a == "--dangerously-skip-permissions" {
				return true
			}
		}
		return false
	}

	if withSkip := run(true); !hasFlag(withSkip) {
		t.Fatalf("SkipPermissions=true must thread --dangerously-skip-permissions into exec args, got %v", withSkip)
	}

	if withoutSkip := run(false); hasFlag(withoutSkip) {
		t.Fatalf("SkipPermissions=false must NOT include --dangerously-skip-permissions, got %v", withoutSkip)
	}
}
