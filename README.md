# gotth-release

`gotth-release` is a language-neutral release library for exact Git and
SemVer identity, verified clean-checkout orchestration, deterministic
normalized tar.gz archives, and SHA-256 admission records.

Callers select the files. The library supplies no hidden deployment layout and
does not infer a product's binaries, database grants, container files, tags, or
rollback policy. That boundary is what makes the mechanism reusable without
turning one application's release procedure into everybody else's accident.

The recommended `BuildVerifiedArchive` entry point verifies the checkout,
snapshots caller-selected files, builds in private staging, rechecks both the
repository and source files, and only then admits the output atomically.

Extracted from the deterministic artifact builder admitted in GOTTH Board
1.0.0-alpha.2, then generalized without retaining Go-specific artifact
metadata.
