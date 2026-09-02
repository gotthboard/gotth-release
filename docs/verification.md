# Verification

`make verify` checks formatting, vet, race behavior, and coverage. Tests build
the same archive twice in different directories and compare every byte, verify
the digest and archive entries, exercise streamed files, reject invalid
identity/platform/path/mode/source/duplicate/reserved inputs, and reject dirty
or mismatched checkouts.
