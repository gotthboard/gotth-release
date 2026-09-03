package release

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestVerifyCleanCheckout(t *testing.T) {
	runner := func(_ context.Context, _ string, _ []string, name string, arguments ...string) ([]byte, error) {
		if name == "git" && arguments[0] == "rev-parse" {
			return []byte(testCommit + "\n"), nil
		}
		return nil, nil
	}
	if err := VerifyCleanCheckout(context.Background(), "/repo", testCommit, runner); err != nil {
		t.Fatalf("VerifyCleanCheckout() returned error: %v", err)
	}
	for _, runner := range []Runner{
		func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return nil, errors.New("failed")
		},
		func(_ context.Context, _ string, _ []string, _ string, _ ...string) ([]byte, error) {
			return []byte("wrong\n"), nil
		},
		func(_ context.Context, _ string, _ []string, name string, arguments ...string) ([]byte, error) {
			if arguments[0] == "rev-parse" {
				return []byte(testCommit + "\n"), nil
			}
			return []byte(" M file\n"), nil
		},
	} {
		if err := VerifyCleanCheckout(context.Background(), "/repo", testCommit, runner); err == nil {
			t.Fatal("invalid checkout passed")
		}
	}
	if err := VerifyCleanCheckout(nil, "/repo", testCommit, runner); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("nil context error = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyCleanCheckout(cancelled, "/repo", testCommit, runner); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context error = %v", err)
	}
}
