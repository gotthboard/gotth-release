# Product requirements

Provide reusable release primitives that prove exact source identity and emit
byte-identical native archives from the same inputs. Outputs must contain
canonical metadata, a dependency manifest, normalized ordering/modes/times,
and a SHA-256 checksum file, and must appear atomically or not at all.

Non-goals: choosing application binaries, building containers, tagging or
pushing repositories, deploying artifacts, defining database grants, or
pretending every consumer has the same rollback contract.
