package packager

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

// RunCommand runs an external tool (pkgbuild, makensis, wix, ...), streaming
// its combined stdout+stderr to onLine one line at a time. If ctx is
// canceled — the GUI's Cancel button, or a quit while a build is running —
// the process is killed promptly rather than left orphaned; WaitDelay bounds
// how long Wait() waits for it to actually exit after that kill signal.
func RunCommand(ctx context.Context, dir string, onLine func(string), name string, args ...string) error {
	return runCommand(ctx, dir, nil, onLine, name, args...)
}

// RunCommandEnv is RunCommand plus extra environment variables appended
// on top of the current process's own environment (not replacing it — a
// hook script still needs $PATH/$HOME/etc.). Used by the post-build hook
// to pass along each built artifact's path; every other RunCommand caller
// needs nothing beyond the inherited environment.
func RunCommandEnv(ctx context.Context, dir string, env []string, onLine func(string), name string, args ...string) error {
	return runCommand(ctx, dir, env, onLine, name, args...)
}

func runCommand(ctx context.Context, dir string, extraEnv []string, onLine func(string), name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if extraEnv != nil {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	cmd.Cancel = func() error { return cmd.Process.Kill() }
	cmd.WaitDelay = 5 * time.Second

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			if onLine != nil {
				onLine(scanner.Text())
			}
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		pw.Close()
		<-done
		return startErr
	}

	runErr := cmd.Wait()
	pw.Close()
	<-done
	return runErr
}
