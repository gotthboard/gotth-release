# Judge pass 1 — rejected and repaired

The first cold review rejected a module-root compatibility facade. No released
tag or consumer pin established that import path, so the facade created a
second public API and permanent maintenance cost without preserving userspace.

Repair: remove the facade, retain exactly one public library package at
`pkg/release`, keep the module root for governance, and update requirements,
architecture, specification, tests, and workflow state accordingly.
