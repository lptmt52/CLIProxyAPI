# Local v8 merge adaptation

## Scope

Merged the local v7 customization into upstream `fd48ea68`, preserving the v8
module, management contract, executor splits, Home lifecycle and scheduler.
The original 19 conflicted files and their base/ours/theirs versions were saved
under `.merge-v8-backup/`. No running service, production configuration or auth
files were replaced. The merge is resolved but not committed.

## Preserved features

- SQLite-backed token usage and the custom statistics page. New API routes are
  `/v8/management/observability/usage/token-usage` and
  `/v8/management/observability/usage/statistics`; both require management auth.
- Provider labels, priority editing, Gemini disabling and OpenAI-compatible
  recovery probes. The v8 log-level path is
  `observability.logs.log-level`.
- JSON/TXT and multi-account credential imports, including ChatGPT session and
  Sub2API conversion; provider/status filtering and batch status/delete actions.
- Batch status changes reuse v8's single-credential persistence, locking,
  config-key handling and plugin hooks instead of the old direct manager update.
- Request, provider and executor diagnostics, attached to the new split files.
- Custom management-page entries. Provider edits now read and update native v8
  groups, preserving sibling keys and inherited group settings. Quota actions
  call `/credentials`; pagination bundle edits are atomic and idempotent.

## Deliberate v8 adaptations

- Pagination follows the native v8 top-level `page`, `page_size`, `total` and
  `has_more` response instead of the custom nested pagination object.
- The former `readyCount <= 1` retry shortcut was not restored over v8's retry
  policy: it conflated ready credentials with the number of eligible credentials
  and bypassed per-key retry settings. Native v8 selection, weighted scheduling,
  request-scoped errors, cooldown handling and Home dispatch remain authoritative.
- Codex stream handling keeps v8 terminal-event, cancellation, bootstrap and
  empty/incomplete-response behavior. The old executor body is not reinstated
  merely to preserve diagnostic logging.
- Health-probe recovery saves configuration before publishing a new immutable
  snapshot, invalidates stale config commits and retains generation checks.
- SQLite initialization occurs before the listener binds, avoiding a shutdown
  race during initialization.
- Existing legacy management routes remain available; new extension routes use
  v8. This is not a rewrite of the deprecated v0 contract.

## Verification (2026-10-01, Asia/Shanghai)

- `gofmt -w .` and `git diff --check` passed.
- `go mod tidy` completed; the module and tracked Go imports use `/v8`.
- Full `go test ./...` rerun passed: 100 packages with tests. The first run had a
  timing failure in `internal/home/TestEnsureClientsWaitsForPreviousTargetClose`;
  five isolated repetitions and the subsequent complete run passed. Home source
  was not modified to mask that failure.
- `go build -o .merge-v8-backup/test-output.exe ./cmd/server` passed; the temporary
  executable is removed after verification, not installed over the running one.
- `node test/management_v8_adapter.cjs` passed grouped-key field preservation and
  stale-selection checks.
- Added authenticated v8 usage-route, legacy-to-v8 configuration preservation,
  deterministic recovery, quota patch and actual bundled JavaScript syntax checks.
  The JavaScript syntax check uses Node.js when available.

Interactive browser acceptance and restarting the production backend are separate
steps. To deploy, build the merged source, then replace/restart the old process
when ready; `git pull` or resolving the index alone does not upgrade that process.
