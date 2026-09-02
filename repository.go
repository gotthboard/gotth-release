package release

import (
	"context"
	"fmt"
	"strings"
)

// Runner executes a bounded command and returns stdout.
type Runner func(context.Context, string, []string, string, ...string) ([]byte, error)

// VerifyCleanCheckout proves that HEAD is the exact release commit and that no
// tracked or untracked work would be omitted from the archive.
func VerifyCleanCheckout(ctx context.Context, directory, commit string, run Runner) error {
	if ctx == nil || run == nil {
		return fmt.Errorf("checkout verifier dependencies are required")
	}
	if _, err := ValidateIdentity("0.0.0", commit); err != nil {
		return err
	}
	head, err := commandLine(ctx, run, directory, "git", "rev-parse", "--verify", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve repository commit: %w", err)
	}
	if head != commit {
		return fmt.Errorf("release commit does not match repository HEAD")
	}
	status, err := run(ctx, directory, nil, "git", "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return fmt.Errorf("inspect repository state: %w", err)
	}
	if len(status) != 0 {
		return fmt.Errorf("release repository is dirty")
	}
	return nil
}

func commandLine(ctx context.Context, run Runner, directory, name string, arguments ...string) (string, error) {
	output, err := run(ctx, directory, nil, name, arguments...)
	if err != nil {
		return "", err
	}
	if len(output) == 0 || output[len(output)-1] != '\n' || strings.Count(string(output), "\n") != 1 {
		return "", fmt.Errorf("command returned a noncanonical single-line result")
	}
	return string(output[:len(output)-1]), nil
}
