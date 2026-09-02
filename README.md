# gotth-release

`gotth-release` is a product-neutral Go library for exact release identity,
clean-checkout verification, deterministic normalized tar.gz archives, and
SHA-256 admission records.

Callers select the files. The library supplies no hidden deployment layout and
does not infer a product's binaries, database grants, container files, tags, or
rollback policy. That boundary is what makes the mechanism reusable without
turning one application's release procedure into everybody else's accident.

Extracted from the deterministic artifact builder admitted in GOTTH Board
1.0.0-alpha.2.
