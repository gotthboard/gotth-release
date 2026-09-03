# Product requirements

Provide reusable release primitives that prove exact Git/SemVer source identity
and emit byte-identical archives from the same inputs regardless of the
consumer's programming language. Outputs must contain canonical project,
target, toolchain, and dependency metadata; normalized ordering, modes, and
times; and a SHA-256 checksum file. A recommended high-level operation must
verify repository cleanliness before and after input capture and archive
construction, detect source-file changes, honor cancellation, and admit the
finished output atomically or not at all.

Non-goals: running consumer build commands, choosing application files,
building containers, tagging or pushing repositories, publishing or deploying
artifacts, defining database grants, supporting non-Git identity, or pretending
every consumer has the same rollback contract.

## Alpha.3 admission requirements

- `REL-A3-01`: New consumers import the documented `pkg/release` package.
- `REL-A3-02`: Exactly one public Go package exists; the module root is
  governance only.
- `REL-A3-03`: Implementation, governance, and evidence occupy separate trees.
- `REL-A3-04`: Verification includes canonical external-consumer compilation,
  clean-clone testing, workflow evidence, and two clean Judge passes.
- `REL-A3-05`: Git, filesystem, archive, and command limits remain explicit and
  fail closed.
