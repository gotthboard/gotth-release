# Coverage

The canonical behavior map is `workflow/artifacts/global-coverage-map.md`.
`make verify` runs the outside-package contract and all white-box tests under
`pkg/release`. Current statement coverage is 83.8%. Remaining branches are
defensive injected-runner, filesystem, and standard-library failure paths; they
are not hidden by the structural admission.
