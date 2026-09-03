package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const maximumGitOutputBytes = 1024 * 1024

// VerifiedConfig identifies the Git checkout and archive inputs for one
// verified release admission.
type VerifiedConfig struct {
	// RepositoryDirectory is the absolute clean Git checkout to verify.
	RepositoryDirectory string
	// Archive names the final output and caller-selected release inputs.
	Archive Config
}

type sourceState struct {
	path   string
	info   os.FileInfo
	digest [sha256.Size]byte
}

// BuildVerifiedArchive proves exact clean Git identity and unchanged source
// inputs around archive construction before atomically admitting the result.
func BuildVerifiedArchive(ctx context.Context, config VerifiedConfig) (Result, error) {
	return buildVerifiedArchive(ctx, config, runGit)
}

func buildVerifiedArchive(ctx context.Context, config VerifiedConfig, run Runner) (Result, error) {
	if ctx == nil || run == nil {
		return Result{}, fmt.Errorf("verified release dependencies are required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !filepath.IsAbs(config.RepositoryDirectory) {
		return Result{}, fmt.Errorf("repository directory must be absolute")
	}
	repositoryInfo, err := os.Stat(config.RepositoryDirectory)
	if err != nil {
		return Result{}, fmt.Errorf("inspect repository directory: %w", err)
	}
	if !repositoryInfo.IsDir() {
		return Result{}, fmt.Errorf("repository path is not a directory")
	}

	archiveConfig := cloneConfig(config.Archive)
	if _, err := prepareArchive(archiveConfig); err != nil {
		return Result{}, err
	}
	if err := VerifyCleanCheckout(ctx, config.RepositoryDirectory, archiveConfig.Commit, run); err != nil {
		return Result{}, fmt.Errorf("verify checkout before snapshot: %w", err)
	}

	parent := filepath.Dir(archiveConfig.OutputDirectory)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, ".gotth-release-verified-")
	if err != nil {
		return Result{}, fmt.Errorf("create verified release staging: %w", err)
	}
	defer os.RemoveAll(staging)

	snapshotDirectory := filepath.Join(staging, "snapshot")
	if err := os.Mkdir(snapshotDirectory, 0o700); err != nil {
		return Result{}, fmt.Errorf("create source snapshot: %w", err)
	}
	states, err := snapshotSources(ctx, snapshotDirectory, archiveConfig.Entries)
	if err != nil {
		return Result{}, fmt.Errorf("snapshot release sources: %w", err)
	}
	if err := VerifyCleanCheckout(ctx, config.RepositoryDirectory, archiveConfig.Commit, run); err != nil {
		return Result{}, fmt.Errorf("verify checkout after snapshot: %w", err)
	}

	buildOutput := filepath.Join(staging, "output")
	archiveConfig.OutputDirectory = buildOutput
	result, err := buildArchive(ctx, archiveConfig)
	if err != nil {
		return Result{}, fmt.Errorf("build verified release archive: %w", err)
	}
	if err := VerifyCleanCheckout(ctx, config.RepositoryDirectory, archiveConfig.Commit, run); err != nil {
		return Result{}, fmt.Errorf("verify checkout after archive: %w", err)
	}
	if err := verifySources(ctx, states); err != nil {
		return Result{}, fmt.Errorf("verify release sources: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if _, err := os.Lstat(config.Archive.OutputDirectory); err == nil {
		return Result{}, fmt.Errorf("output directory already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("inspect output directory: %w", err)
	}
	if err := os.Rename(buildOutput, config.Archive.OutputDirectory); err != nil {
		return Result{}, fmt.Errorf("admit verified release output: %w", err)
	}
	return Result{
		ArchivePath:  filepath.Join(config.Archive.OutputDirectory, filepath.Base(result.ArchivePath)),
		ChecksumPath: filepath.Join(config.Archive.OutputDirectory, filepath.Base(result.ChecksumPath)),
		SHA256:       result.SHA256,
	}, nil
}

func cloneConfig(config Config) Config {
	config.Dependencies = bytes.Clone(config.Dependencies)
	config.Toolchains = append([]Toolchain(nil), config.Toolchains...)
	config.Entries = append([]Entry(nil), config.Entries...)
	for index := range config.Entries {
		config.Entries[index].Data = bytes.Clone(config.Entries[index].Data)
	}
	return config
}

func snapshotSources(ctx context.Context, directory string, entries []Entry) ([]sourceState, error) {
	states := make([]sourceState, 0, len(entries))
	for index := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entries[index].SourcePath == "" {
			continue
		}
		snapshotPath := filepath.Join(directory, fmt.Sprintf("%08d", index))
		state, err := snapshotSource(ctx, entries[index].SourcePath, snapshotPath)
		if err != nil {
			return nil, err
		}
		entries[index].SourcePath = snapshotPath
		states = append(states, state)
	}
	return states, nil
}

func snapshotSource(ctx context.Context, sourcePath, snapshotPath string) (sourceState, error) {
	source, before, err := openRegularFile(sourcePath)
	if err != nil {
		return sourceState{}, err
	}
	defer source.Close()
	snapshot, err := os.OpenFile(snapshotPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return sourceState{}, err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(snapshot, digest), contextReader{ctx: ctx, reader: source})
	closeErr := snapshot.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return sourceState{}, err
	}
	if written != before.Size() {
		return sourceState{}, fmt.Errorf("source changed while snapshotting")
	}
	after, err := source.Stat()
	if err != nil {
		return sourceState{}, err
	}
	current, err := os.Lstat(sourcePath)
	if err != nil {
		return sourceState{}, err
	}
	if !sameFileState(before, after) || !sameFileState(before, current) {
		return sourceState{}, fmt.Errorf("source changed while snapshotting")
	}
	var encoded [sha256.Size]byte
	copy(encoded[:], digest.Sum(nil))
	return sourceState{path: sourcePath, info: before, digest: encoded}, nil
}

func verifySources(ctx context.Context, states []sourceState) error {
	for _, state := range states {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, before, err := openRegularFile(state.path)
		if err != nil {
			return err
		}
		if !sameFileState(state.info, before) {
			_ = file.Close()
			return fmt.Errorf("source %q changed before verification", state.path)
		}
		digest := sha256.New()
		written, copyErr := io.Copy(digest, contextReader{ctx: ctx, reader: file})
		after, statErr := file.Stat()
		closeErr := file.Close()
		if err := errors.Join(copyErr, statErr, closeErr); err != nil {
			return err
		}
		current, err := os.Lstat(state.path)
		if err != nil {
			return err
		}
		if written != state.info.Size() || !sameFileState(state.info, after) || !sameFileState(state.info, current) || !bytes.Equal(digest.Sum(nil), state.digest[:]) {
			return fmt.Errorf("source %q changed after snapshot", state.path)
		}
	}
	return nil
}

func runGit(ctx context.Context, directory string, environment []string, name string, arguments ...string) ([]byte, error) {
	if name != "git" {
		return nil, fmt.Errorf("verified release runner only permits git")
	}
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	if environment != nil {
		command.Env = append([]string(nil), environment...)
	}
	stdout := &boundedBuffer{maximum: maximumGitOutputBytes}
	stderr := &boundedBuffer{maximum: maximumGitOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		if errors.Is(stdout.err, errOutputLimit) || errors.Is(stderr.err, errOutputLimit) {
			return nil, fmt.Errorf("git output exceeds %d bytes", maximumGitOutputBytes)
		}
		if len(stderr.buffer) != 0 {
			return nil, fmt.Errorf("git command failed: %w: %s", err, bytes.TrimSpace(stderr.buffer))
		}
		return nil, fmt.Errorf("git command failed: %w", err)
	}
	if stdout.err != nil || stderr.err != nil {
		return nil, fmt.Errorf("git output exceeds %d bytes", maximumGitOutputBytes)
	}
	return bytes.Clone(stdout.buffer), nil
}

var errOutputLimit = errors.New("command output limit exceeded")

type boundedBuffer struct {
	buffer  []byte
	maximum int
	err     error
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	if len(buffer.buffer)+len(value) > buffer.maximum {
		remaining := buffer.maximum - len(buffer.buffer)
		if remaining > 0 {
			buffer.buffer = append(buffer.buffer, value[:remaining]...)
		}
		buffer.err = errOutputLimit
		return len(value), errOutputLimit
	}
	buffer.buffer = append(buffer.buffer, value...)
	return len(value), nil
}
