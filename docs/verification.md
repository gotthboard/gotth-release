# Verification

`make verify` checks formatting, vet, race behavior, and coverage. Tests build
the same archive twice in different directories and compare every byte, verify
the digest and archive entries, exercise streamed files, reject invalid
identity/target/toolchain/path/mode/source/duplicate/reserved inputs, and reject
dirty or mismatched checkouts.

The generic verified-admission suite additionally uses disposable real Git
repositories and controlled failure seams to prove the fixed command contract,
three clean-checkout gates, source snapshots and mutation detection,
cancellation, symlink refusal, bounded command output, staging cleanup, and the
absence of a final output on every rejected path.
