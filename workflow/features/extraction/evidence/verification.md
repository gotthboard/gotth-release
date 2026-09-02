# Extraction verification

Verified on 2026-09-02 with Go `go1.26.6-X:nodwarf5`:

- `go vet -mod=readonly ./...`
- `go test -mod=readonly -race -cover ./...`
- statement coverage: 88.9%
- archives built in separate directories compare byte-for-byte
- SHA-256 output and normalized archive entry order/timestamps verified
- streamed source files, traversal, mode, duplicate, reserved-name, identity,
  output-existence, mismatched-HEAD, and dirty-checkout paths exercised

The remaining uncovered branches are filesystem/compressor close failures that
the public API cannot safely induce without test-only replacement hooks.
