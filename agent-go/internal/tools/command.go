package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultCommandTimeout = 3 * time.Second

type commandOutput struct {
	Stdout string
	Stderr string
}

func (o commandOutput) Combined() string {
	parts := []string{}
	if strings.TrimSpace(o.Stdout) != "" {
		parts = append(parts, strings.TrimSpace(o.Stdout))
	}
	if strings.TrimSpace(o.Stderr) != "" {
		parts = append(parts, strings.TrimSpace(o.Stderr))
	}
	return strings.Join(parts, "\n")
}

func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (commandOutput, error) {
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := commandOutput{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if runCtx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %s: %s %s", timeout, name, strings.Join(args, " "))
	}
	if err != nil {
		combined := output.Combined()
		if combined == "" {
			combined = err.Error()
		}
		return output, fmt.Errorf("command failed: %s %s: %w: %s", name, strings.Join(args, " "), err, combined)
	}
	return output, nil
}
