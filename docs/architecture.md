# Architecture

`ValidateIdentity` admits canonical SemVer and a full lowercase Git object
name. `VerifyCleanCheckout` checks HEAD and porcelain status through an injected
bounded runner. `BuildArchive` validates caller-selected entries, adds release
and dependency metadata, sorts everything, writes USTAR through deterministic
gzip headers, hashes the byte stream, and renames one complete staging
directory into place.

Source files are streamed. Inline metadata is copied once. No executable is
retained in memory and no external command is hidden inside archive creation.
