package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var projectPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Entry is one regular file admitted to a release archive. Exactly one of Data
// and SourcePath must be supplied.
type Entry struct {
	Name       string
	Mode       int64
	Data       []byte
	SourcePath string
}

// Config identifies one archive. Entries appear beneath a normalized root
// named project-version-goos-goarch.
type Config struct {
	Project           string
	OutputDirectory   string
	Version           string
	Commit            string
	GOOS              string
	GOARCH            string
	GoVersion         string
	RequiredGoVersion string
	Dependencies      []byte
	Entries           []Entry
}

// Result identifies the immutable archive and checksum file.
type Result struct {
	ArchivePath  string
	ChecksumPath string
	SHA256       string
}

// BuildArchive atomically admits one deterministic tar.gz and SHA256SUMS file.
func BuildArchive(config Config) (Result, error) {
	if !projectPattern.MatchString(config.Project) || len(config.Project) > 80 {
		return Result{}, fmt.Errorf("release project is invalid")
	}
	identity, err := ValidateIdentity(config.Version, config.Commit)
	if err != nil {
		return Result{}, err
	}
	if !filepath.IsAbs(config.OutputDirectory) {
		return Result{}, fmt.Errorf("output directory must be absolute")
	}
	if _, err := os.Lstat(config.OutputDirectory); err == nil {
		return Result{}, fmt.Errorf("output directory already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("inspect output directory: %w", err)
	}
	if config.GOOS == "" || config.GOARCH == "" || strings.ContainsAny(config.GOOS+config.GOARCH, "/\\\x00\r\n") {
		return Result{}, fmt.Errorf("release platform is invalid")
	}
	if config.GoVersion == "" || config.RequiredGoVersion == "" || strings.ContainsAny(config.GoVersion+config.RequiredGoVersion, "\x00\r\n") {
		return Result{}, fmt.Errorf("Go toolchain identity is invalid")
	}
	dependencies, err := canonicalText(config.Dependencies)
	if err != nil {
		return Result{}, fmt.Errorf("dependency manifest is invalid")
	}
	entries := append([]Entry(nil), config.Entries...)
	seen := make(map[string]struct{}, len(entries))
	for index := range entries {
		entry := &entries[index]
		clean := filepath.ToSlash(filepath.Clean(entry.Name))
		if entry.Name == "" || clean != entry.Name || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.ContainsRune(clean, '\x00') {
			return Result{}, fmt.Errorf("release entry name is invalid")
		}
		if entry.Name == "DEPENDENCIES.txt" || entry.Name == "RELEASE.txt" {
			return Result{}, fmt.Errorf("release entry uses a reserved name")
		}
		if entry.Mode != 0o644 && entry.Mode != 0o755 {
			return Result{}, fmt.Errorf("release entry mode is invalid")
		}
		if (entry.SourcePath == "") == (entry.Data == nil) {
			return Result{}, fmt.Errorf("release entry must have exactly one source")
		}
		if entry.SourcePath != "" && !filepath.IsAbs(entry.SourcePath) {
			return Result{}, fmt.Errorf("release entry source path must be absolute")
		}
		if _, exists := seen[entry.Name]; exists {
			return Result{}, fmt.Errorf("release entry name is duplicated")
		}
		seen[entry.Name] = struct{}{}
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })
	root := fmt.Sprintf("%s-%s-%s-%s", config.Project, identity.Version, config.GOOS, config.GOARCH)
	metadata := fmt.Sprintf("version=%s\ncommit=%s\ngoos=%s\ngoarch=%s\ngo_version=%s\ngo_required=%s\n", identity.Version, identity.Commit, config.GOOS, config.GOARCH, config.GoVersion, config.RequiredGoVersion)
	entries = append(entries,
		Entry{Name: "DEPENDENCIES.txt", Mode: 0o644, Data: dependencies},
		Entry{Name: "RELEASE.txt", Mode: 0o644, Data: []byte(metadata)},
	)
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })

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
	archiveName := root + ".tar.gz"
	archivePath := filepath.Join(stage, archiveName)
	digest, err := writeArchive(archivePath, root, entries)
	if err != nil {
		return Result{}, fmt.Errorf("write release archive: %w", err)
	}
	checksumPath := filepath.Join(stage, "SHA256SUMS")
	if err := os.WriteFile(checksumPath, []byte(digest+"  "+archiveName+"\n"), 0o644); err != nil {
		return Result{}, fmt.Errorf("write release checksum: %w", err)
	}
	if err := os.Rename(stage, config.OutputDirectory); err != nil {
		return Result{}, fmt.Errorf("admit release output: %w", err)
	}
	return Result{ArchivePath: filepath.Join(config.OutputDirectory, archiveName), ChecksumPath: filepath.Join(config.OutputDirectory, "SHA256SUMS"), SHA256: digest}, nil
}

func canonicalText(value []byte) ([]byte, error) {
	if len(value) == 0 || value[len(value)-1] != '\n' || bytes.IndexByte(value, 0) >= 0 || bytes.IndexByte(value, '\r') >= 0 {
		return nil, fmt.Errorf("text is not canonical")
	}
	return append([]byte(nil), value...), nil
}

func writeArchive(path, root string, entries []Entry) (string, error) {
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
		archiveErr = writeEntry(tarWriter, root, entry)
	}
	archiveErr = errors.Join(archiveErr, tarWriter.Close(), gzipWriter.Close(), file.Close())
	if archiveErr != nil {
		return "", archiveErr
	}
	return encodeDigest(digest), nil
}

func writeEntry(writer *tar.Writer, root string, entry Entry) error {
	var reader io.Reader = bytes.NewReader(entry.Data)
	size := int64(len(entry.Data))
	var source *os.File
	if entry.SourcePath != "" {
		var err error
		source, err = os.Open(entry.SourcePath)
		if err != nil {
			return err
		}
		defer source.Close()
		info, err := source.Stat()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive source is not a regular file")
		}
		reader, size = source, info.Size()
	}
	header := &tar.Header{Name: root + "/" + entry.Name, Size: size, Mode: entry.Mode, Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := io.Copy(writer, reader)
	return err
}

func encodeDigest(digest hash.Hash) string {
	return hex.EncodeToString(digest.Sum(nil))
}
