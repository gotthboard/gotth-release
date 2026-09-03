# Implementation specification

- Project names are bounded lowercase slugs.
- Versions are canonical SemVer, including numeric prerelease rules.
- Commits are exact 40-character lowercase hexadecimal object names.
- Targets are required bounded lowercase slugs such as `linux-amd64`, `source`,
  or `universal`; they are not interpreted by the library.
- Toolchains are an optional unique list of bounded lowercase names and
  bounded single-line UTF-8 versions. They are sorted before rendering.
- Output paths are absolute and must not already exist.
- Entry paths are relative, clean, unique, and cannot escape the archive root.
- Only regular files with modes 0644 or 0755 are admitted.
- `DEPENDENCIES.txt` and `RELEASE.txt` are reserved generated entries.
- The dependency manifest may be empty; a nonempty manifest must end in one
  newline and contain no carriage return or NUL.
- USTAR timestamps are Unix epoch; gzip timestamp is zero and OS is 255.
- Entries are sorted lexically and checksum text is canonical.
- `BuildVerifiedArchive` invokes no consumer command. It runs only fixed Git
  `rev-parse` and porcelain-status arguments, with bounded output and no shell.
- Source paths are absolute, non-symlink regular files. The verified operation
  snapshots and hashes them, then compares their type, identity, size, time,
  and digest again before final admission.
- Checkout verification occurs before snapshotting, after snapshotting, and
  after archive construction. A mismatch, dirty checkout, source change,
  cancellation, timeout, or I/O failure leaves no final output.
