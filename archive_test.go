package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func validConfig(output string) Config {
	return Config{
		Project: "example-app", OutputDirectory: output, Version: "1.2.3-alpha.4", Commit: testCommit,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: "go1.26.6", RequiredGoVersion: "1.26.6",
		Dependencies: []byte("example-app\ngolang.org/x/text v0.36.0\n"),
		Entries:      []Entry{{Name: "bin/example-app", Mode: 0o755, Data: []byte("binary")}, {Name: "README.md", Mode: 0o644, Data: []byte("readme\n")}},
	}
}

func TestBuildArchiveIsDeterministicAtomicAndChecksummed(t *testing.T) {
	first, err := BuildArchive(validConfig(filepath.Join(t.TempDir(), "release")))
	if err != nil {
		t.Fatalf("BuildArchive(first) returned error: %v", err)
	}
	second, err := BuildArchive(validConfig(filepath.Join(t.TempDir(), "release")))
	if err != nil {
		t.Fatalf("BuildArchive(second) returned error: %v", err)
	}
	firstBytes, _ := os.ReadFile(first.ArchivePath)
	secondBytes, _ := os.ReadFile(second.ArchivePath)
	if !bytes.Equal(firstBytes, secondBytes) || first.SHA256 != second.SHA256 {
		t.Fatal("deterministic archives differ")
	}
	digest := sha256.Sum256(firstBytes)
	if hex.EncodeToString(digest[:]) != first.SHA256 {
		t.Fatal("reported digest does not match archive")
	}
	checksum, _ := os.ReadFile(first.ChecksumPath)
	if string(checksum) != first.SHA256+"  "+filepath.Base(first.ArchivePath)+"\n" {
		t.Fatalf("checksum = %q", checksum)
	}
	reader, err := gzip.NewReader(bytes.NewReader(firstBytes))
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(reader)
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
	}
	root := "example-app-1.2.3-alpha.4-" + runtime.GOOS + "-" + runtime.GOARCH
	want := []string{root + "/", root + "/DEPENDENCIES.txt", root + "/README.md", root + "/RELEASE.txt", root + "/bin/example-app"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("entries = %q, want %q", names, want)
	}
}

func TestBuildArchiveReadsRegularSourceFile(t *testing.T) {
	source := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(source, []byte("source binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := validConfig(filepath.Join(t.TempDir(), "release"))
	config.Entries = []Entry{{Name: "bin/app", Mode: 0o755, SourcePath: source}}
	if _, err := BuildArchive(config); err != nil {
		t.Fatalf("BuildArchive() returned error: %v", err)
	}
	config.OutputDirectory = filepath.Join(t.TempDir(), "release")
	config.Entries[0].SourcePath = t.TempDir()
	if _, err := BuildArchive(config); err == nil || !strings.Contains(err.Error(), "regular") {
		t.Fatalf("directory source error = %v", err)
	}
}

func TestBuildArchiveRejectsInvalidBoundariesWithoutOutput(t *testing.T) {
	base := validConfig(filepath.Join(t.TempDir(), "release"))
	tests := []func(*Config){
		func(c *Config) { c.Project = "Bad Project" }, func(c *Config) { c.Version = "bad" },
		func(c *Config) { c.OutputDirectory = "relative" }, func(c *Config) { c.GOOS = "" },
		func(c *Config) { c.GoVersion = "" }, func(c *Config) { c.Dependencies = []byte("missing newline") },
		func(c *Config) { c.Entries[0].Name = "../escape" }, func(c *Config) { c.Entries[0].Name = "RELEASE.txt" },
		func(c *Config) { c.Entries[0].Mode = 0o777 }, func(c *Config) { c.Entries[0].SourcePath = "/tmp/source" },
		func(c *Config) { c.Entries[0].Data = nil }, func(c *Config) { c.Entries = append(c.Entries, c.Entries[0]) },
	}
	for index, mutate := range tests {
		config := base
		config.OutputDirectory = filepath.Join(t.TempDir(), "release")
		config.Entries = append([]Entry(nil), base.Entries...)
		mutate(&config)
		if got, err := BuildArchive(config); err == nil || got != (Result{}) {
			t.Errorf("invalid case %d = (%+v, %v)", index, got, err)
		}
	}
	existing := t.TempDir()
	config := validConfig(existing)
	if _, err := BuildArchive(config); err == nil {
		t.Fatal("existing output passed")
	}
}
