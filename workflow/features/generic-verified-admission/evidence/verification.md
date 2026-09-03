# Generic verified-admission verification

Verified implementation commit
`fb4e805eaf3306735c453c7aabc2922e2268125b` on 2026-09-03 UTC with Go
`go1.26.6-X:nodwarf5`.

## Contract evidence

- `Config` now requires an opaque target and accepts optional canonical
  toolchain records; it contains no Go-specific platform or toolchain fields.
- `RELEASE.txt` records project, canonical SemVer, exact 40-character lowercase
  Git commit, opaque target, and sorted toolchain identities.
- `BuildVerifiedArchive` performs three exact-HEAD/clean-worktree gates around
  source snapshotting and archive construction.
- Source files are non-symlink regular files copied into private same-filesystem
  staging and checked by file identity, type, size, modification time, and
  SHA-256 digest before final admission.
- The high-level runner executes only fixed Git argument vectors through
  `exec.CommandContext`; no shell, consumer build, tag, publish, deployment, or
  rollback command is admitted.
- Cancellation or rejection removes private staging and leaves the requested
  final output absent.

## Commands and results

- `make verify`: pass; format, pinned toolchain, vet, race, and coverage.
- `go test -mod=readonly -race -count=50 ./...`: pass.
- Clean local clone of the committed feature branch followed by `make verify`:
  pass with no generated worktree changes.
- Statement coverage: 83.8%. The changed public success paths and relevant
  invalid-input, dirty/mismatched checkout, later checkout mutation,
  digest-only source mutation, symlink, traversal, cancellation, deterministic
  ordering, checksum, and cleanup behavior are exercised. Remaining uncovered
  branches are chiefly operating-system create/write/stat/close failures that
  cannot be induced portably without contaminating the production API with
  test-only filesystem seams.

## Determinism and graph evidence

- Archives built in separate output directories compare byte-for-byte even
  when caller entry and toolchain order differs.
- Graphify 0.9.32 code-only graph at the implementation commit: 79 nodes, 187
  directed edges, 7 communities, no self-loops, exact duplicate edges, or
  same-endpoint relation groups.
- Graph SHA-256:
  `2f46692d6909f13ca59793a76f141e13a774eea704b35ec275ac589f5beec01c`.
- Graph cache is outside the repository at
  `/home/linus/.cache/openclaw-graphify/gotth-release-generic/graphify-out/graph.json`;
  extraction left the repository unchanged.

## Explicit boundary

The operation detects mutations observed at its verification gates. It does
not claim to lock arbitrary producer processes; callers must serialize writers
to checkout, source, and output paths during admission. No release tag or
consumer pin is created by this feature.
