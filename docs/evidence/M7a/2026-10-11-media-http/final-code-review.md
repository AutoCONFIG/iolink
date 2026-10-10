# M7a HTTP/HLS code-quality review

- `codeQualityStatus`: WATCH
- `recommendation`: APPROVE
- `snapshotSHA`: `5765e23aac2f3c54f15f27a7fc1ef0eccbe88a90`
- `reviewScope`: HTTP/HLS changes only, cumulative `e3f7e93..5765e23`; GB files and GB-only commit content were excluded
- `blockers`: `[]`
- `reportPath`: `.omo/evidence/m7a-media-http-code-review.md`
- `ulwStatus`: `ULW_LOOP_PLAN_MISSING`; fallback report path used (no currentAttemptDir/notepad)

## Goal and success criteria

Verify the final provider-neutral media HTTP/HLS seam at the exact SHA. The review covers generic media-store failures mapping to 503, one atomic segment-mapping batch with no client-visible residuals, bounded mapping/fetch contexts, manifest and segment output caps, unknown/unsafe HLS tag rejection, RawPath/Fragment/query/encoded-path rejection, malformed segment UUID 400, strict HTTPS grant-base validation, token-only media authorization/status mapping, security headers, relevant tests, and scope control. Video/ZLM/RTSP/GB/real-provider/browser/WeChat acceptance remains external-blocked.

## Reviewed files

- `internal/media/contracts.go`
- `internal/playback/service.go`
- `internal/playback/service_test.go`
- `internal/mediahttp/handler.go`
- `internal/mediahttp/handler_test.go`

The product diff is 27 additions in `contracts.go`, a new 122-line HTTP handler, a new 271-line handler test, and 184 added/2 removed lines in `playback/service.go`; the GB additions in the same history were intentionally ignored.

## Skill perspective check

This check ran before judging maintainability or test relevance. I loaded the Go `programming` skill and its HTTP backend, concurrency, error-handling, type-pattern, data-modeling, and testing references, and loaded `remove-ai-slops`.

The overfit/slop pass found no deletion-only tests, requested-removal-only tests, natural-language prompt pins, output-derived self-comparisons, broad catches, untyped production escape hatches, provider SDK leakage, dead helpers, or unnecessary provider parsing beyond the required media boundary. Tests assert observable status, headers, bodies, mapping calls, and deadlines. The atomicity fake has an evidence limitation recorded below. The Go module-size rule is exceeded by the already large playback service and worsened by this increment.

## Findings by severity

### CRITICAL

None.

### HIGH

None. The final code is fail-closed for the named HTTP criteria and all fresh targeted/full checks passed.

### MEDIUM

1. **Playback service is materially oversized.** `internal/playback/service.go` is 518 pure non-comment LOC (well above the 250-line programming/remove-ai-slops ceiling), combining grant creation, owner session reads/stops, token authorization, HLS parsing/rewriting, segment mapping, source normalization, job processing, and retry mapping. The new HLS responsibilities increase coupling and future change risk. This is a maintainability follow-up, not a current correctness blocker.

2. **Atomic/no-residual test is only a service-level fake.** `internal/mediahttp/handler_test.go:73-84,199-212` makes `CreateSegmentMappings` return an error before recording rows; `len(store.created)==0` therefore proves the fake's own error branch, not rollback or absence of rows in a real SQL transaction. The single batch port and service checks are correct, and no persistence adapter exists in this scope, so the real transaction/CAS proof is deferred and explicitly external-blocked. A future PG adapter test must inject a mid-batch failure and re-query the table.

### LOW

1. **Canonical UUID shape is not pinned.** `internal/mediahttp/handler.go:84-87` uses `google/uuid.Parse`, which accepts raw-hex, URN, and brace forms in addition to the canonical path UUID shape. A noncanonical segment identifier is therefore parsed and reaches authorization/mapping (normally ending in 401/404) instead of the handler's 400 malformed-path branch. This does not bypass token/session binding, but a later HTTP-contract test should decide and pin canonical UUID syntax.

2. **Header/body assertions are uneven on error paths.** The handler sets `Cache-Control: no-store` and `Referrer-Policy: no-referrer` before every branch, and the successful manifest test checks both, but error tests mostly assert only status (for example `handler_test.go:154-160`). The implementation is straightforward; adding one representative error-header/code assertion would make the all-media-response contract less dependent on inspection.

## Criterion review

- Generic `AuthorizeMedia` store errors go through `normalizeMediaStoreError` and become `ErrUnavailable`/HTTP 503; known not-found/forbidden/conflict categories remain 404/403/409.
- Manifest mapping is collected and persisted through one `CreateSegmentMappings(ctx, mappings)` call; returned rows are count/order/identity/path checked before returning the rewritten body.
- Manifest source and mapping work share a default 3-second deadline; segment mapping and source fetch share a default 10-second deadline. The dedicated deadline regression observes `ResolveSegmentMapping` receiving a deadline.
- Source and rewritten manifest sizes are capped at 256 KiB; segment bytes are capped at 8 MiB before the handler writes.
- Only the allowlisted HLS tags and the exact dated TS path are accepted; unknown tags, URI/key/map/byterange/master/fMP4-style inputs, traversal, absolute paths, encoding, query, and fragment data are rejected.
- `RawPath`, query, and fragment values are rejected before route dispatch; malformed segment UUIDs now return 400 rather than falling through to 404.
- Enabled playback requires an absolute HTTPS base with nonempty hostname and no userinfo, query, fragment, or opaque component. Invalid configuration fails before session persistence.
- The media handler does not call business actor middleware; it uses the dedicated token/audience/claim/hash/live-session checks and maps pending to 425 + `Retry-After: 1`, authorization failures to 401/403, unknown mappings to 404, capability/store errors to 503, and rejected media to 502.
- Media responses set `Cache-Control: no-store` and `Referrer-Policy: no-referrer`; success content types are the manifest MIME and `video/mp2t`.

## Fresh verification

- `git rev-parse HEAD`: exact `5765e23aac2f3c54f15f27a7fc1ef0eccbe88a90`.
- `go test -race -shuffle=on -count=1 ./internal/mediahttp ./internal/playback`: **PASS**.
- `go vet ./internal/mediahttp ./internal/playback`: **PASS**.
- `go vet ./...`: **PASS**.
- `go test -race -shuffle=on -count=1 ./...`: **PASS** (all packages, including mediahttp/playback and GB package).
- `gofumpt -d` on all changed Go files: **PASS** (no diff).
- `git diff --check e3f7e93..5765e23`: **PASS**.
- Checked-in scoped receipt: `docs/evidence/M7a/2026-10-11-media-http/README.md` (its no-provider/external-blocked statement was treated as untrusted and matched by fresh runs).

## External and deferred evidence

There is no PostgreSQL segment-mapping adapter, provider source implementation, composition-root media route, real ZLMediaKit/RTSP stream, camera, browser, WeChat, TLS/NAT, or external network in this snapshot. Those checks are `external_blocked`, not passes. The fakes prove application orchestration only; a later provider increment must supply real transaction rollback/no-residual and source-boundary integration evidence.

## Final recommendation

**APPROVE** the HTTP/HLS software seam at the exact SHA, with `blockers=[]`. Retain the two medium follow-ups and external-blocked provider/transaction evidence in the next M7a increment.
