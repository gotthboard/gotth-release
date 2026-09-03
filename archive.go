package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maximumProjectBytes          = 80
	maximumTargetBytes           = 80
	maximumToolchainNameBytes    = 80
	maximumToolchainVersionBytes = 256
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

// Entry is one regular file admitted to a release archive. Exactly one of Data
// and SourcePath must be supplied.
type Entry struct {
	// Name is a clean slash-separated path relative to the archive root.
	Name string
	// Mode is either 0644 or 0755.
	Mode int64
	// Data contains inline bytes. A non-nil empty slice represents an empty file.
	Data []byte
	// SourcePath names an absolute non-symlink regular file to stream.
	SourcePath string
}

// Toolchain records one language-neutral tool or runtime identity.
type Toolchain struct {
	// Name is the canonical lowercase metadata key.
	Name string
	// Version is a bounded, valid UTF-8 single line interpreted by the caller.
	Version string
}

// Config identifies one archive. Entries appear beneath a normalized root
// named project-version-target.
type Config struct {
	// Project is a bounded lowercase slug.
	Project string
	// OutputDirectory is an absolute path that must not already exist.
	OutputDirectory string
	// Version is canonical SemVer and Commit is a full lowercase Git object ID.
	Version string
	Commit  string
	// Target is a required opaque lowercase slug such as linux-amd64 or source.
	Target string
	// Toolchains is optional and is rendered in canonical name order.
	Toolchains []Toolchain
	// Dependencies is empty or canonical newline-terminated text.
	Dependencies []byte
	// Entries are caller-selected files; generated metadata names are reserved.
	Entries []Entry
}

// Result identifies the immutable archive and checksum file.
type Result struct {
	// ArchivePath and ChecksumPath are admitted beneath OutputDirectory.
	ArchivePath  string
	ChecksumPath string
	// SHA256 is the lowercase hexadecimal archive digest.
	SHA256 string
}

type preparedArchive struct {
	entries []Entry
	root    string
}

// BuildArchive atomically admits one deterministic tar.gz and SHA256SUMS file.
// BuildVerifiedArchive is the recommended entry point for release admission.
func BuildArchive(config Config) (Result, error) {
	return buildArchive(context.Background(), config)
}

func buildArchive(ctx context.Context, config Config) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("release context is required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	prepared, err := prepareArchive(config)
	if err != nil {
		return Result{}, err
	}
	parent := filepath.Dir(config.OutputDirectory)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output parent: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, ".gotth-release-")
	if err != nil {
		return Result{}, fmt.Errorf("create release staging directory: %w", err)
	}
	defer os.RemoveAll(temporary)

	stage := filepath.Join(temporary, "stage")
	if err := os.Mkdir(stage, 0o755); err != nil {
		return Result{}, fmt.Errorf("create release stage: %w", err)
	}
	archiveName := prepared.root + ".tar.gz"
	archivePath := filepath.Join(stage, archiveName)
	digest, err := writeArchive(ctx, archivePath, prepared.root, prepared.entries)
	if err != nil {
		return Result{}, fmt.Errorf("write release archive: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	checksumPath := filepath.Join(stage, "SHA256SUMS")
	if err := os.WriteFile(checksumPath, []byte(digest+"  "+archiveName+"\n"), 0o644); err != nil {
		return Result{}, fmt.Errorf("write release checksum: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := os.Rename(stage, config.OutputDirectory); err != nil {
		return Result{}, fmt.Errorf("admit release output: %w", err)
	}
	return Result{
		ArchivePath:  filepath.Join(config.OutputDirectory, archiveName),
		ChecksumPath: filepath.Join(config.OutputDirectory, "SHA256SUMS"),
		SHA256:       digest,
	}, nil
}

func prepareArchive(config Config) (preparedArchive, error) {
	if !canonicalSlug(config.Project, maximumProjectBytes) {
		return preparedArchive{}, fmt.Errorf("release project is invalid")
	}
	identity, err := ValidateIdentity(config.Version, config.Commit)
	if err != nil {
		return preparedArchive{}, err
	}
	if !filepath.IsAbs(config.OutputDirectory) {
		return preparedArchive{}, fmt.Errorf("output directory must be absolute")
	}
	if _, err := os.Lstat(config.OutputDirectory); err == nil {
		return preparedArchive{}, fmt.Errorf("output directory already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return preparedArchive{}, fmt.Errorf("inspect output directory: %w", err)
	}
	if !canonicalSlug(config.Target, maximumTargetBytes) {
		return preparedArchive{}, fmt.Errorf("release target is invalid")
	}

	toolchains := append([]Toolchain(nil), config.Toolchains...)
	seenToolchains := make(map[string]struct{}, len(toolchains))
	for _, toolchain := range toolchains {
		if !canonicalSlug(toolchain.Name, maximumToolchainNameBytes) || !canonicalSingleLine(toolchain.Version, maximumToolchainVersionBytes) {
			return preparedArchive{}, fmt.Errorf("release toolchain identity is invalid")
		}
		if _, exists := seenToolchains[toolchain.Name]; exists {
			return preparedArchive{}, fmt.Errorf("release toolchain name is duplicated")
		}
		seenToolchains[toolchain.Name] = struct{}{}
	}
	sort.Slice(toolchains, func(left, right int) bool { return toolchains[left].Name < toolchains[right].Name })

	dependencies, err := canonicalText(config.Dependencies)
	if err != nil {
		return preparedArchive{}, fmt.Errorf("dependency manifest is invalid")
	}
	entries := append([]Entry(nil), config.Entries...)
	seenEntries := make(map[string]struct{}, len(entries))
	for index := range entries {
		entry := &entries[index]
		clean := path.Clean(entry.Name)
		if entry.Name == "" || clean != entry.Name || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.ContainsAny(clean, "\\\x00") {
			return preparedArchive{}, fmt.Errorf("release entry name is invalid")
		}
		if entry.Name == "DEPENDENCIES.txt" || entry.Name == "RELEASE.txt" {
			return preparedArchive{}, fmt.Errorf("release entry uses a reserved name")
		}
		if entry.Mode != 0o644 && entry.Mode != 0o755 {
			return preparedArchive{}, fmt.Errorf("release entry mode is invalid")
		}
		if (entry.SourcePath == "") == (entry.Data == nil) {
			return preparedArchive{}, fmt.Errorf("release entry must have exactly one source")
		}
		if entry.SourcePath != "" && !filepath.IsAbs(entry.SourcePath) {
			return preparedArchive{}, fmt.Errorf("release entry source path must be absolute")
		}
		if _, exists := seenEntries[entry.Name]; exists {
			return preparedArchive{}, fmt.Errorf("release entry name is duplicated")
		}
		seenEntries[entry.Name] = struct{}{}
		if entry.Data != nil {
			entry.Data = bytes.Clone(entry.Data)
		}
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })

	var metadata strings.Builder
	fmt.Fprintf(&metadata, "project=%s\nversion=%s\ncommit=%s\ntarget=%s\n", config.Project, identity.Version, identity.Commit, config.Target)
	for _, toolchain := range toolchains {
		fmt.Fprintf(&metadata, "toolchain.%s=%s\n", toolchain.Name, toolchain.Version)
	}
	root := fmt.Sprintf("%s-%s-%s", config.Project, identity.Version, config.Target)
	if len(root)+1 > 100 {
		return preparedArchive{}, fmt.Errorf("release archive root is too long")
	}
	entries = append(entries,
		Entry{Name: "DEPENDENCIES.txt", Mode: 0o644, Data: dependencies},
		Entry{Name: "RELEASE.txt", Mode: 0o644, Data: []byte(metadata.String())},
	)
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })
	return preparedArchive{
		entries: entries,
		root:    root,
	}, nil
}

func canonicalSlug(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum && slugPattern.MatchString(value)
}

func canonicalSingleLine(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || character == '\u0085' || character == '\u2028' || character == '\u2029' {
			return false
		}
	}
	return true
}

func canonicalText(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return []byte{}, nil
	}
	if value[len(value)-1] != '\n' || bytes.IndexByte(value, 0) >= 0 || bytes.IndexByte(value, '\r') >= 0 {
		return nil, fmt.Errorf("text is not canonical")
	}
	return bytes.Clone(value), nil
}

func writeArchive(ctx context.Context, path, root string, entries []Entry) (string, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	gzipWriter, err := gzip.NewWriterLevel(io.MultiWriter(file, digest), gzip.BestCompression)
	if err != nil {
		_ = file.Close()
		return "", err
	}
	gzipWriter.Header.ModTime = time.Time{}
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	archiveErr := tarWriter.WriteHeader(&tar.Header{Name: root + "/", Mode: 0o755, Typeflag: tar.TypeDir, ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR})
	for _, entry := range entries {
		if archiveErr != nil {
			break
		}
		archiveErr = writeEntry(ctx, tarWriter, root, entry)
	}
	archiveErr = errors.Join(archiveErr, tarWriter.Close(), gzipWriter.Close(), file.Close())
	if archiveErr != nil {
		return "", archiveErr
	}
	return encodeDigest(digest), nil
}

func writeEntry(ctx context.Context, writer *tar.Writer, root string, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var reader io.Reader = bytes.NewReader(entry.Data)
	size := int64(len(entry.Data))
	var source *os.File
	var before os.FileInfo
	if entry.SourcePath != "" {
		var err error
		source, before, err = openRegularFile(entry.SourcePath)
		if err != nil {
			return err
		}
		defer source.Close()
		reader, size = source, before.Size()
	}
	header := &tar.Header{Name: root + "/" + entry.Name, Size: size, Mode: entry.Mode, Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	written, err := io.Copy(writer, contextReader{ctx: ctx, reader: reader})
	if err != nil {
		return err
	}
	if written != size {
		return fmt.Errorf("archive source size changed while reading")
	}
	if source != nil {
		after, err := source.Stat()
		if err != nil {
			return err
		}
		if !sameFileState(before, after) {
			return fmt.Errorf("archive source changed while reading")
		}
	}
	return nil
}

func openRegularFile(path string) (*os.File, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("archive source is not a non-symlink regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("archive source changed while opening")
	}
	return file, opened, nil
}

func sameFileState(before, after os.FileInfo) bool {
	return before.Mode().Type() == after.Mode().Type() && before.Mode().IsRegular() && after.Mode().IsRegular() &&
		os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.reader.Read(buffer)
	if contextErr := reader.ctx.Err(); contextErr != nil {
		return count, contextErr
	}
	return count, err
}

func encodeDigest(digest hash.Hash) string {
	return hex.EncodeToString(digest.Sum(nil))
}
