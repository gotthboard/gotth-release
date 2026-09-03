package release

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildVerifiedArchiveWithRealGitCheckout(t *testing.T) {
	repository, commit := initializeGitRepository(t)
	source := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(source, []byte("verified artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputParent := t.TempDir()
	output := filepath.Join(outputParent, "release")
	config := validConfig(output)
	config.Commit = commit
	config.Target = "universal"
	config.Toolchains = nil
	config.Entries = []Entry{{Name: "artifact", Mode: 0o644, SourcePath: source}}

	result, err := BuildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config})
	if err != nil {
		t.Fatalf("BuildVerifiedArchive() returned error: %v", err)
	}
	if filepath.Dir(result.ArchivePath) != output || filepath.Dir(result.ChecksumPath) != output || len(result.SHA256) != 64 {
		t.Fatalf("result = %+v", result)
	}
	assertNoVerifiedStaging(t, outputParent)
}

func TestBuildVerifiedArchiveRejectsDirtyCheckoutWithoutOutput(t *testing.T) {
	repository, commit := initializeGitRepository(t)
	if err := os.WriteFile(filepath.Join(repository, "untracked"), []byte("dirty"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputParent := t.TempDir()
	config := validConfig(filepath.Join(outputParent, "release"))
	config.Commit = commit

	if result, err := BuildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config}); err == nil || result != (Result{}) {
		t.Fatalf("dirty release = (%+v, %v)", result, err)
	}
	assertOutputAbsent(t, config.OutputDirectory)
	assertNoVerifiedStaging(t, outputParent)
}

func TestBuildVerifiedArchiveRejectsWrongCommitWithoutOutput(t *testing.T) {
	repository, _ := initializeGitRepository(t)
	outputParent := t.TempDir()
	config := validConfig(filepath.Join(outputParent, "release"))
	if result, err := BuildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config}); err == nil || result != (Result{}) {
		t.Fatalf("wrong-commit release = (%+v, %v)", result, err)
	}
	assertOutputAbsent(t, config.OutputDirectory)
	assertNoVerifiedStaging(t, outputParent)
}

func TestBuildVerifiedArchiveUsesOnlyFixedGitChecks(t *testing.T) {
	repository := t.TempDir()
	outputParent := t.TempDir()
	config := validConfig(filepath.Join(outputParent, "release"))
	var calls [][]string
	runner := func(_ context.Context, directory string, environment []string, name string, arguments ...string) ([]byte, error) {
		if directory != repository || environment != nil || name != "git" {
			t.Fatalf("runner boundary = (%q, %v, %q)", directory, environment, name)
		}
		calls = append(calls, append([]string{name}, arguments...))
		if arguments[0] == "rev-parse" {
			return []byte(testCommit + "\n"), nil
		}
		return nil, nil
	}
	if _, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config}, runner); err != nil {
		t.Fatalf("buildVerifiedArchive() returned error: %v", err)
	}
	want := [][]string{
		{"git", "rev-parse", "--verify", "HEAD"}, {"git", "status", "--porcelain=v1", "--untracked-files=normal"},
		{"git", "rev-parse", "--verify", "HEAD"}, {"git", "status", "--porcelain=v1", "--untracked-files=normal"},
		{"git", "rev-parse", "--verify", "HEAD"}, {"git", "status", "--porcelain=v1", "--untracked-files=normal"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("Git calls = %q, want %q", calls, want)
	}
}

func TestBuildVerifiedArchiveRejectsCheckoutMutationAtEveryLaterGate(t *testing.T) {
	for _, dirtyCall := range []int{4, 6} {
		t.Run(string(rune('0'+dirtyCall)), func(t *testing.T) {
			repository := t.TempDir()
			outputParent := t.TempDir()
			config := validConfig(filepath.Join(outputParent, "release"))
			calls := 0
			runner := func(_ context.Context, _ string, _ []string, _ string, arguments ...string) ([]byte, error) {
				calls++
				if arguments[0] == "rev-parse" {
					return []byte(testCommit + "\n"), nil
				}
				if calls == dirtyCall {
					return []byte(" M tracked-file\n"), nil
				}
				return nil, nil
			}
			if result, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config}, runner); err == nil || result != (Result{}) {
				t.Fatalf("mutated checkout = (%+v, %v)", result, err)
			}
			assertOutputAbsent(t, config.OutputDirectory)
			assertNoVerifiedStaging(t, outputParent)
		})
	}
}

func TestBuildVerifiedArchiveRejectsDigestOnlySourceMutation(t *testing.T) {
	repository := t.TempDir()
	source := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(source, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	outputParent := t.TempDir()
	config := validConfig(filepath.Join(outputParent, "release"))
	config.Entries = []Entry{{Name: "artifact", Mode: 0o644, SourcePath: source}}
	calls := 0
	runner := func(_ context.Context, _ string, _ []string, _ string, arguments ...string) ([]byte, error) {
		calls++
		if calls == 5 {
			if err := os.WriteFile(source, []byte("after!"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(source, original.ModTime(), original.ModTime()); err != nil {
				t.Fatal(err)
			}
		}
		if arguments[0] == "rev-parse" {
			return []byte(testCommit + "\n"), nil
		}
		return nil, nil
	}
	result, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config}, runner)
	if err == nil || result != (Result{}) || !strings.Contains(err.Error(), "changed after snapshot") {
		t.Fatalf("mutated source = (%+v, %v)", result, err)
	}
	assertOutputAbsent(t, config.OutputDirectory)
	assertNoVerifiedStaging(t, outputParent)
}

func TestBuildVerifiedArchiveRejectsSymlinkSourceWithoutOutput(t *testing.T) {
	repository := t.TempDir()
	sourceDirectory := t.TempDir()
	source := filepath.Join(sourceDirectory, "artifact")
	link := filepath.Join(sourceDirectory, "link")
	if err := os.WriteFile(source, []byte("artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	outputParent := t.TempDir()
	config := validConfig(filepath.Join(outputParent, "release"))
	config.Entries = []Entry{{Name: "artifact", Mode: 0o644, SourcePath: link}}
	result, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: repository, Archive: config}, cleanTestRunner(nil))
	if err == nil || result != (Result{}) || !strings.Contains(err.Error(), "non-symlink regular") {
		t.Fatalf("symlink source = (%+v, %v)", result, err)
	}
	assertOutputAbsent(t, config.OutputDirectory)
	assertNoVerifiedStaging(t, outputParent)
}

func TestBuildVerifiedArchiveCancellationCleansStaging(t *testing.T) {
	repository := t.TempDir()
	outputParent := t.TempDir()
	config := validConfig(filepath.Join(outputParent, "release"))
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	runner := func(_ context.Context, _ string, _ []string, _ string, arguments ...string) ([]byte, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		if arguments[0] == "rev-parse" {
			return []byte(testCommit + "\n"), nil
		}
		return nil, nil
	}
	result, err := buildVerifiedArchive(ctx, VerifiedConfig{RepositoryDirectory: repository, Archive: config}, runner)
	if !errors.Is(err, context.Canceled) || result != (Result{}) {
		t.Fatalf("cancelled release = (%+v, %v)", result, err)
	}
	assertOutputAbsent(t, config.OutputDirectory)
	assertNoVerifiedStaging(t, outputParent)
}

func TestBuildVerifiedArchiveRejectsInvalidRepositoryAndExistingOutput(t *testing.T) {
	config := validConfig(filepath.Join(t.TempDir(), "release"))
	clean := cleanTestRunner(nil)
	if _, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: "relative", Archive: config}, clean); err == nil {
		t.Fatal("relative repository passed")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: file, Archive: config}, clean); err == nil {
		t.Fatal("regular-file repository passed")
	}
	config.OutputDirectory = t.TempDir()
	if _, err := buildVerifiedArchive(context.Background(), VerifiedConfig{RepositoryDirectory: t.TempDir(), Archive: config}, clean); err == nil {
		t.Fatal("existing output passed")
	}
	if _, err := buildVerifiedArchive(nil, VerifiedConfig{}, clean); err == nil {
		t.Fatal("nil context passed")
	}
	if _, err := buildVerifiedArchive(context.Background(), VerifiedConfig{}, nil); err == nil {
		t.Fatal("nil runner passed")
	}
}

func TestBoundedGitRunnerBoundary(t *testing.T) {
	buffer := &boundedBuffer{maximum: 3}
	if count, err := buffer.Write([]byte("abcd")); !errors.Is(err, errOutputLimit) || count != 4 || string(buffer.buffer) != "abc" {
		t.Fatalf("bounded write = (%d, %v, %q)", count, err, buffer.buffer)
	}
	if _, err := runGit(context.Background(), t.TempDir(), nil, "not-git"); err == nil {
		t.Fatal("non-Git command passed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runGit(ctx, t.TempDir(), nil, "git", "status"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Git error = %v", err)
	}
	if _, err := runGit(context.Background(), filepath.Join(t.TempDir(), "missing"), nil, "git", "status"); err == nil {
		t.Fatal("Git failure passed")
	}
}

func cleanTestRunner(callback func(int)) Runner {
	calls := 0
	return func(_ context.Context, _ string, _ []string, _ string, arguments ...string) ([]byte, error) {
		calls++
		if callback != nil {
			callback(calls)
		}
		if arguments[0] == "rev-parse" {
			return []byte(testCommit + "\n"), nil
		}
		return nil, nil
	}
}

func initializeGitRepository(t *testing.T) (string, string) {
	t.Helper()
	repository := t.TempDir()
	for _, arguments := range [][]string{
		{"init", "-q", repository},
		{"-C", repository, "config", "user.name", "Release Test"},
		{"-C", repository, "config", "user.email", "release@example.invalid"},
	} {
		command := exec.Command("git", arguments...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(repository, "tracked"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"-C", repository, "add", "tracked"}, {"-C", repository, "commit", "-q", "-m", "initial"}} {
		command := exec.Command("git", arguments...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	command := exec.Command("git", "-C", repository, "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return repository, strings.TrimSpace(string(output))
}

func assertOutputAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("rejected output exists at %q: %v", path, err)
	}
}

func assertNoVerifiedStaging(t *testing.T, parent string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(parent, ".gotth-release-verified-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("verified staging remains: %q", matches)
	}
}
