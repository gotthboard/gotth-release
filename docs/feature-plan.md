# Feature plan

1. Extract semantic identity and deterministic archive mechanics.
2. Replace GOTTH Board's fixed binaries and deploy paths with caller entries.
3. Preserve clean-checkout and atomic-admission behavior.
4. Verify byte identity, archive shape, traversal rejection, and failure paths.
5. Let each consumer own its build commands and release/rollback policy.

## Generic verified release admission

1. Replace Go-specific archive metadata with required opaque target identity
   and sorted language-neutral toolchain records.
2. Add one safe `BuildVerifiedArchive` operation that owns checkout checks,
   input snapshots, mutation detection, staging, cancellation, and final
   admission.
3. Preserve `BuildArchive` and `VerifyCleanCheckout` as explicit low-level
   primitives without presenting their independent use as safe orchestration.
4. Prove deterministic output, clean and exact Git identity, input mutation
   refusal, symlink and traversal refusal, cancellation, bounded Git output,
   and absence of partial final output.
5. Admit no build runner, repository host, package manager, deployment, tag,
   publication, or rollback policy.
