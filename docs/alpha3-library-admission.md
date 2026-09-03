# Alpha.3 library admission

## Scope and authority

This pass admits `pkg/release` as the sole public package for GOTTH Board
alpha.3. The module root is governance only. It does not authorize tags,
publishing, deployment, consumer builds, or rollback automation.

## Requirement traceability

| Requirement | Design/specification | Code | Verification |
|---|---|---|---|
| `REL-A3-01` | `docs/architecture.md` | `pkg/release/` | canonical external-package test |
| `REL-A3-02` | `docs/implementation-spec.md` | `pkg/release/` | package inventory and outside-consumer compile |
| `REL-A3-03` | repository layout in `README.md` | `pkg/`, `docs/`, `workflow/` | tracked-file inventory |
| `REL-A3-04` | `docs/verification.md` | tests and workflow evidence | clean clone and two Judge passes |
| `REL-A3-05` | `docs/implementation-spec.md` | archive/repository implementation | negative, mutation, and boundary tests |

## Runtime boundary

- Supported implementation target: Go 1.26.6 on Linux amd64 for admission.
- External mechanisms: Git command execution through the injected bounded
  runner, regular-file reads, same-filesystem staging/rename, USTAR, gzip, and
  SHA-256.
- Hard limits and refusal behavior are defined in
  `docs/implementation-spec.md`; successful output is complete only when both
  archive and checksum are atomically admitted and verified by tests.
- Git, path, file-type, mutation, cancellation, duplicate/reserved-name, and
  output-existence boundaries are exercised by the existing suite.

## Performance admission

No speedup is claimed. The archive, hashing, copying, and Git mechanisms are
unchanged; the original implementation moved intact to `pkg/release`.
End-to-end benchmark/Amdahl evidence is therefore N/A for this layout-only
admission. Any future performance claim must benchmark archive workloads and
retain matched raw evidence.

## Failure and rollback

No compatibility facade is retained because no consumer tag established the
old root import path. Before a consumer tag, rollback is a revert of this
admission commit. No artifact, repository tag, release, or deployment is
created by the pass.
