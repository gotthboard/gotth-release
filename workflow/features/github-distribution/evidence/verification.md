# GitHub Distribution Verification

Status: complete

## Identity and scope

- Pinned base: `36c6ae4e310fc4710becdc838785294d31bbc566`
- Publicly verified candidate: `5e3a0dc6d0169671616a6208ff3c69be3dd505d3`
- Declared module: `github.com/gotthboard/gotth-release`
- Runtime/API behavior: unchanged; this is a module-identity and distribution
  contract migration.

Exact stale-prefix searches found no old module or import identity in Go source,
`go.mod`, examples, or fixtures. Canonical Forgejo URLs remain only where the
development, issue, contribution, and security-reporting endpoints are stated.

## Verification

- Local `go mod tidy` produced no dependency drift.
- Local `go vet -mod=readonly ./...` passed.
- Local `go test -mod=readonly ./...` passed.
- On `development`, `make verify` passed with race coverage 83.8%.
- On `development`, `go test -mod=readonly -race -count=50 ./...` passed.
- No repository-specific verification exception was required.
- A fresh public GitHub clone of `feature/github-distribution` resolved exact
  commit `5e3a0dc6d0169671616a6208ff3c69be3dd505d3` and passed `go test -mod=readonly ./...`.
- A fresh external consumer compiled the public package through both direct VCS
  resolution and `https://proxy.golang.org,direct` at
  `v0.0.0-20260903060720-5e3a0dc6d016`.
- Complete Forgejo and GitHub advertised head/tag ref sets matched after the
  candidate push.
- A fresh public GitHub `main` clone resolved
  `66c7f6528f766ea7ddedf5247c3f07ecdf83cf7a`, produced no `go mod tidy` drift, and passed
  `go test -mod=readonly ./...`.
- Fresh external consumers resolved `@main` through direct VCS and
  `https://proxy.golang.org,direct`, then compiled at
  `v0.0.0-20260903062630-66c7f6528f76`.

The slash-containing feature ref is accepted by direct VCS resolution but is
not a legal version query for the module proxy. The pre-promotion proxy lane
therefore used the exact candidate pseudo-version above; both final `@main`
lanes passed after promotion.

## Impact graph

Graphify recorded 80 nodes / 247 edges at implementation commit. Graph SHA-256:
`acb30104c750eda84bd48ad7cb9a874e1234b36f33b132fedc99ddb973920570`.
Subsequent commits before this record changed documentation only. The merged
suite graph had 4,116 nodes and 11,415 edges, with no
cross-repository module dependency edge.

## Admission and residual gates

Two cold Judge passes reviewed the completed candidate before promotion. This
completion update changes evidence and workflow state only and receives two
fresh cold passes before commit. No performance benchmark applies because
executable paths and data flow are unchanged.

No license was selected. Release tags remain blocked until Danny closes that
decision gate. GitHub metadata mutation lacks authentication. Forgejo is still
private, so unauthenticated public contribution and private vulnerability
reporting remain unresolved. Account conversion and ownership changes were not
performed.
