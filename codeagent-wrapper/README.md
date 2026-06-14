# codeagent-wrapper

## Exec Session Services

Long-lived services started inside an exec session should be backgrounded and torn down by explicit PID, for example by recording `$!` and killing that PID. Do not run them in the foreground expecting stdin/Ctrl-C teardown, because codex exec sessions may close stdin during shutdown.
