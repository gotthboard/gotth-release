# Architecture

`ValidateIdentity` admits canonical SemVer and a full lowercase Git object
name. `VerifyCleanCheckout` checks HEAD and porcelain status through an injected
bounded runner. `BuildArchive` remains the deterministic low-level primitive:
it validates caller-selected entries, adds release and dependency metadata,
sorts everything, writes USTAR through deterministic gzip headers, hashes the
byte stream, and renames one complete staging directory into place.

`BuildVerifiedArchive` is the recommended boundary. It validates the final
destination, verifies the exact clean checkout, copies input files into private
same-filesystem staging while hashing them, rechecks the checkout, constructs
the archive from the snapshot, rechecks the checkout and original source
digests, then atomically renames the complete output into place. Any failure or
cancellation removes staging and leaves the final destination absent.

Source files are streamed. Inline metadata is copied once. No executable is
retained in memory and no external command is hidden inside archive creation.
The high-level operation invokes only fixed Git argument vectors without a
shell. Consumer build, tag, publish, deployment, and rollback commands remain
outside the library.
