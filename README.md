# gotth-release

`gotth-release` is a Go library for language-neutral release artifacts: exact
Git and SemVer identity, verified clean-checkout orchestration, deterministic
normalized tar.gz archives, and SHA-256 admission records.

Callers select the files. The library supplies no hidden deployment layout and
does not infer a product's binaries, database grants, container files, tags, or
rollback policy. That boundary is what makes the mechanism reusable without
turning one application's release procedure into everybody else's accident.

The recommended `BuildVerifiedArchive` entry point verifies the checkout,
snapshots caller-selected files, builds in private staging, rechecks both the
repository and source files, and only then admits the output atomically.

```go
result, err := release.BuildVerifiedArchive(ctx, release.VerifiedConfig{
	RepositoryDirectory: checkout,
	Archive: release.Config{
		Project: "example", Version: version, Commit: commit,
		Target: "linux-amd64", OutputDirectory: output,
		Toolchains: []release.Toolchain{{Name: "zig", Version: zigVersion}},
		Entries: []release.Entry{{
			Name: "bin/example", Mode: 0o755, SourcePath: artifact,
		}},
	},
})
```

`Target` is opaque. A compiled program can use `linux-amd64`, a static site can
use `site`, and a source distribution can use `source`. Toolchain records are
optional. The caller must serialize mutations to its checkout, selected source
files, and output parent while admission runs; the library detects mutations at
its verification gates but does not pretend to own every producer process.

Extracted from the deterministic artifact builder admitted in GOTTH Board
1.0.0-alpha.2, then generalized without retaining Go-specific artifact
metadata.
