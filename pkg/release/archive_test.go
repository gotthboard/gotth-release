package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func validConfig(output string) Config {
	return Config{
		Project: "example-app", OutputDirectory: output, Version: "1.2.3-alpha.4", Commit: testCommit,
		Target:       "linux-amd64",
		Toolchains:   []Toolchain{{Name: "rust", Version: "1.89.0"}, {Name: "go", Version: "1.26.6"}},
		Dependencies: []byte("example-app\nexample.org/library v1.0.0\n"),
		Entries: []Entry{
			{Name: "bin/example-app", Mode: 0o755, Data: []byte("binary")},
			{Name: "README.md", Mode: 0o644, Data: []byte("readme\n")},
		},
	}
}

func TestBuildArchiveIsLanguageNeutralDeterministicAtomicAndChecksummed(t *testing.T) {
	first, err := BuildArchive(validConfig(filepath.Join(t.TempDir(), "release")))
	if err != nil {
		t.Fatalf("BuildArchive(first) returned error: %v", err)
	}
	secondConfig := validConfig(filepath.Join(t.TempDir(), "release"))
	secondConfig.Toolchains[0], secondConfig.Toolchains[1] = secondConfig.Toolchains[1], secondConfig.Toolchains[0]
	secondConfig.Entries[0], secondConfig.Entries[1] = secondConfig.Entries[1], secondConfig.Entries[0]
	second, err := BuildArchive(secondConfig)
	if err != nil {
		t.Fatalf("BuildArchive(second) returned error: %v", err)
	}
	firstBytes, err := os.ReadFile(first.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) || first.SHA256 != second.SHA256 {
		t.Fatal("deterministic archives differ")
	}
	digest := sha256.Sum256(firstBytes)
	if hex.EncodeToString(digest[:]) != first.SHA256 {
		t.Fatal("reported digest does not match archive")
	}
	checksum, err := os.ReadFile(first.ChecksumPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(checksum) != first.SHA256+"  "+filepath.Base(first.ArchivePath)+"\n" {
		t.Fatalf("checksum = %q", checksum)
	}

	files, names := readArchive(t, firstBytes)
	root := "example-app-1.2.3-alpha.4-linux-amd64"
	wantNames := []string{root + "/", root + "/DEPENDENCIES.txt", root + "/README.md", root + "/RELEASE.txt", root + "/bin/example-app"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("entries = %q, want %q", names, wantNames)
	}
	wantMetadata := "project=example-app\nversion=1.2.3-alpha.4\ncommit=" + testCommit + "\ntarget=linux-amd64\ntoolchain.go=1.26.6\ntoolchain.rust=1.89.0\n"
	if got := string(files[root+"/RELEASE.txt"]); got != wantMetadata {
		t.Fatalf("release metadata = %q, want %q", got, wantMetadata)
	}
}

func TestBuildArchiveAcceptsEmptyDependencyManifest(t *testing.T) {
	config := validConfig(filepath.Join(t.TempDir(), "release"))
	config.Dependencies = nil
	result, err := BuildArchive(config)
	if err != nil {
		t.Fatalf("BuildArchive() returned error: %v", err)
	}
	archive, err := os.ReadFile(result.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := readArchive(t, archive)
	if dependencies := files["example-app-1.2.3-alpha.4-linux-amd64/DEPENDENCIES.txt"]; len(dependencies) != 0 {
		t.Fatalf("empty dependencies = %q", dependencies)
	}
}

func TestBuildArchiveReadsOnlyNonSymlinkRegularSourceFiles(t *testing.T) {
	sourceDirectory := t.TempDir()
	source := filepath.Join(sourceDirectory, "binary")
	if err := os.WriteFile(source, []byte("source binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := validConfig(filepath.Join(t.TempDir(), "release"))
	config.Entries = []Entry{{Name: "bin/app", Mode: 0o755, SourcePath: source}}
	if _, err := BuildArchive(config); err != nil {
		t.Fatalf("BuildArchive() returned error: %v", err)
	}

	for name, invalidSource := range map[string]string{"directory": sourceDirectory, "symlink": filepath.Join(sourceDirectory, "link")} {
		if name == "symlink" {
			if err := os.Symlink(source, invalidSource); err != nil {
				t.Fatal(err)
			}
		}
		config.OutputDirectory = filepath.Join(t.TempDir(), "release")
		config.Entries[0].SourcePath = invalidSource
		if _, err := BuildArchive(config); err == nil || !strings.Contains(err.Error(), "regular") {
			t.Errorf("%s source error = %v", name, err)
		}
	}
}

func TestBuildArchiveRejectsInvalidBoundariesWithoutOutput(t *testing.T) {
	base := validConfig(filepath.Join(t.TempDir(), "release"))
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"project", func(c *Config) { c.Project = "Bad Project" }},
		{"version", func(c *Config) { c.Version = "bad" }},
		{"relative output", func(c *Config) { c.OutputDirectory = "relative" }},
		{"target", func(c *Config) { c.Target = "Linux AMD64" }},
		{"toolchain name", func(c *Config) { c.Toolchains[0].Name = "Rust Compiler" }},
		{"toolchain version", func(c *Config) { c.Toolchains[0].Version = "1.89\nmalformed" }},
		{"toolchain unicode line", func(c *Config) { c.Toolchains[0].Version = "1.89\u2028malformed" }},
		{"duplicate toolchain", func(c *Config) { c.Toolchains = append(c.Toolchains, c.Toolchains[0]) }},
		{"long root", func(c *Config) { c.Project = strings.Repeat("a", 60); c.Target = strings.Repeat("b", 60) }},
		{"dependency newline", func(c *Config) { c.Dependencies = []byte("missing newline") }},
		{"dependency carriage return", func(c *Config) { c.Dependencies = []byte("bad\r\n") }},
		{"traversal", func(c *Config) { c.Entries[0].Name = "../escape" }},
		{"backslash", func(c *Config) { c.Entries[0].Name = `bin\\escape` }},
		{"reserved", func(c *Config) { c.Entries[0].Name = "RELEASE.txt" }},
		{"mode", func(c *Config) { c.Entries[0].Mode = 0o777 }},
		{"two sources", func(c *Config) { c.Entries[0].SourcePath = "/tmp/source" }},
		{"no source", func(c *Config) { c.Entries[0].Data = nil }},
		{"duplicate entry", func(c *Config) { c.Entries = append(c.Entries, c.Entries[0]) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(base)
			config.OutputDirectory = filepath.Join(t.TempDir(), "release")
			test.mutate(&config)
			if got, err := BuildArchive(config); err == nil || got != (Result{}) {
				t.Fatalf("invalid config = (%+v, %v)", got, err)
			}
			if _, err := os.Lstat(config.OutputDirectory); !os.IsNotExist(err) {
				t.Fatalf("rejected output exists: %v", err)
			}
		})
	}
	existing := t.TempDir()
	config := validConfig(existing)
	if _, err := BuildArchive(config); err == nil {
		t.Fatal("existing output passed")
	}
}

func TestBuildArchiveHonorsCancellationWithoutOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	config := validConfig(filepath.Join(t.TempDir(), "release"))
	if got, err := buildArchive(ctx, config); err == nil || got != (Result{}) {
		t.Fatalf("cancelled build = (%+v, %v)", got, err)
	}
	if _, err := os.Lstat(config.OutputDirectory); !os.IsNotExist(err) {
		t.Fatalf("cancelled output exists: %v", err)
	}
}

func readArchive(t *testing.T, compressed []byte) (map[string][]byte, []string) {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	files := make(map[string][]byte)
	var names []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !header.ModTime.Equal(time.Unix(0, 0).UTC()) {
			t.Fatalf("entry %q has nonzero time", header.Name)
		}
		names = append(names, header.Name)
		if header.Typeflag == tar.TypeReg {
			files[header.Name], err = io.ReadAll(tarReader)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return files, names
}
