# Implementation specification

- Project names are bounded lowercase slugs.
- Versions are canonical SemVer, including numeric prerelease rules.
- Commits are exact 40-character lowercase hexadecimal object names.
- Output paths are absolute and must not already exist.
- Entry paths are relative, clean, unique, and cannot escape the archive root.
- Only regular files with modes 0644 or 0755 are admitted.
- `DEPENDENCIES.txt` and `RELEASE.txt` are reserved generated entries.
- USTAR timestamps are Unix epoch; gzip timestamp is zero and OS is 255.
- Entries are sorted lexically and checksum text is canonical.
