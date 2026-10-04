# Independent code review: final M6b candidate

- Verdict: **APPROVE** for the R36.a/R37.a software candidate.
- `codeQualityStatus`: **WATCH**
- `recommendation`: **APPROVE**
- `blockers`: **[]**
- Confidence: **HIGH** for the reviewed authorization corrections and existing HTTP/DB scope; no certification of future resources or external acceptance.
- Exact candidate: **03354381b5954087d82d0d3f807cad9993b94b8d**
- Candidate checkout: `/tmp/iolink-m6b-release-final`, clean before and after review, including its web submodule.
- Executable source receipt: `ef6510ffa5e43fd9796352ae324e09dfc3820c68`; candidate differences from that source are documentation only.
- Web snapshot: `e84092326d9979e066c611e2be8eb6b9736e09f3`.
- Baseline: `main` = `5c34af407e884a1c6ef6bb80daed3e15980a21db`; special correction comparison: `c56cb22b00712d1cd8b2624af60e7f06636fbf24` to candidate.
- Reviewer: independent, read-only code-quality reviewer; no implementation edits or commits.
- Date: 2026-10-04, Asia/Shanghai.

## Scope and inputs

Consulted AGENTS.md, docs/README.md, docs/CONTRIBUTING.md, the M6b requirements in docs/PLAN.md and docs/EXTENSIONS.md, R36.a/R37.a and common negative gates in docs/ACCEPTANCE.md, current docs/IMPLEMENTED.md, relevant version contracts, branch production/test differences, and the final committed evidence bundle. Historical review reports were treated as history, not current approval.

Reviewed changed authorization context/policy, app resource scope SQL, telemetry authorization, admin mutations, product persistence, HTTP error mapping, changed regression fixtures, and the web expiry changes. The entire branch contains substantial historical raw evidence; file presence and receipt results were checked independently, and the affected real database/HTTP tests were rerun.

No notepad path was supplied or found in the candidate. The input goal and repository authority documents provide the criteria. `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; the required report fallback is this original-workspace `.omo/evidence/m6b-final-code-review.md`, outside the candidate.

## Skill-perspective check

**Ran.** Loaded `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.13/skills/remove-ai-slops/SKILL.md` and `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.13/skills/programming/SKILL.md`, plus Go README/testing and TypeScript README references, before judging tests or maintainability.

The overfit/slop pass found no deletion-only tests, tests solely pinning requested removals, prompt/prose pins, implementation-constant mirrors, or new unnecessary production extraction/normalization. The authorization rechecks are required at the write boundary; they are not redundant validation. The web expiry parser is required by the explicit timezone/precision correction. The fake executor now calls real production authorization against real DB membership state before its observable side effect, rather than duplicating the policy.

Both skill perspectives flag continued growth of pre-existing oversized modules (MEDIUM finding below). The programming perspective also flags two insufficiently specific HTTP rejection assertions. These are nonblocking quality debt; the observed final outcomes and the narrow new security regressions are correct.

## Findings by severity

### CRITICAL

None.

### HIGH

None remaining. The earlier version-revocation, partial alarm-context, and unsupported current verification claims are closed for this exact candidate.

### MEDIUM

1. **Changed production modules continue to exceed the skill size ceiling.** `internal/core/admin_store.go:88` grows from 930 to 1042 pure LOC; `internal/adminapi/handlers.go:353` from 1007 to 1075; `internal/persistence/products.go:156` from 378 to 402. `internal/core/repos.go:180`, `internal/adminapi/server.go:324`, and `internal/appapi/server.go:343` are also already over 250 pure LOC. These files aggregate multiple resource responsibilities, making future authorization audits harder. New shared helpers limit duplicated policy logic, and this review found no resulting authorization failure, so this is MEDIUM maintenance debt rather than a release blocker. Split future work by farm, pond, device, rule, alarm and catalog responsibilities without broadening this security correction.

2. **Two older reviewer regressions accept every error status.** `internal/core/tenant_write_revocation_http_test.go:74` and `:134` assert `status >= 400`. A SQL failure returning 500 with unchanged state would pass, though the common negative contract distinguishes authorization failures from internal errors. The current independent run actually returns 403, and the product, telemetry and alarm correction regressions pin exact status codes. Tighten these two assertions to the intended 403 response when updating their coverage. They remain meaningful mutation-prevention tests, but do not independently lock the HTTP error mapping.

### LOW

1. **Stale source labels in synthetic test logs.** `internal/core/tenant_write_revocation_http_test.go:73` and `:133` print `source=9120469` when the tests run on this newer snapshot. This can confuse a reader of an isolated log fragment. The final receipt and this review's raw log explicitly identify their source, so it does not invalidate the verification. Remove the literal historical source label or record source identity once in the runner.

## Correctness and regression assessment

- **Telemetry version revocation:** `internal/core/telemetry_authorization.go:24` loads live membership role/version with tenant/user membership locks; `:39` compares the authenticated permission version before any telemetry, water projection, shadow, alarm or outbox write. Missing policy/context, inactive/expired membership, platform authority drift and denied roles fail closed. Both HTTP stacks attach the authenticated version to context.
- **Alarm version revocation:** `internal/core/app_resource_scope.go:43` adds a version predicate to the shared confirmation scope. The singular UPDATE and its already-confirmed idempotency query in `internal/core/repos.go:631` and `:647`, plus both batch count/update queries in `internal/core/admin_store.go:1042` and `:1075`, use it. The real HTTP tests confirm both routes reject post-authentication version increments with 404 and unchanged persisted state.
- **Partial alarm contexts:** `internal/core/admin_store.go:952` and `:992` use `HasTenantScope`, so tenant-only, actor-only, role-only and version-only contexts cannot take the legacy unscoped path. `ConfirmAlarmByActor` requires tenant, matching actor and role; batch requires matching tenant/actor and an allowed policy decision. Five partial variants reject singular/batch operations for both open and already-confirmed alarms.
- **Resource scope:** owner/admin live tenant membership grants the documented same-tenant scope without requiring a farm assignment; lower roles still need farm ownership or an active assignment. Every affected public query applies the selected tenant condition. Cross-tenant and hidden-farm HTTP reads/mutations and platform-default denial are covered by the real fixture tests.
- **Write atomicity:** admin mutation authorization runs inside the mutation transaction with live membership/user/tenant locks. Product/model/device writes do the same. Telemetry and its outbox intent commit together. Batch alarm confirmation validates the whole set and rolls back if the write set changes. The before/after business-state snapshots are useful assertions of rejected-write atomicity, not tautologies.
- **Internal catalog tenant parameter observation resolved:** `ProductStore.CreateProduct/CreateProductModel/PublishProductModel` accept a separate tenant argument while their policy check reads the context. All current production callers are `internal/adminapi/products.go:66`, `:158`, and `:191`; each gets the target tenant from `productCatalog` at `:30`, which selects the authenticated `domain.TenantID`. The routes require tenant context. Request bodies cannot choose that argument, and product/model IDs are checked against it in SQL. There is no demonstrated cross-tenant HTTP path for this candidate. A future internal caller must preserve this target derivation; adding a generic duplicate check is not required to close the current external boundary.
- **SQL safety:** helper table aliases and placeholder indices come from static repository call sites. Interpolated roles are restricted to literal allowed roles, and interpolated actors/versions are typed integers. Caller-controlled resource IDs, telemetry fields and other values remain SQL arguments. No new raw string interpolation from HTTP input was found.
- **Fixture correction:** changing the already-confirmed alarm fixture to its valid existing support context preserves its idempotency assertion. The adjacent M2 fixture now uses the tenant member role intended for farm ownership and explicitly switches tenant; this preserves the lifecycle test under tenant-manager scope semantics.

## Independently executed verification

1. On the unchanged exact candidate, with `GOFLAGS=-buildvcs=false`, an isolated DSN derived without printing credentials from `iolink-todo9-pg`, localhost port 55439:

   `go test -race -shuffle=on -count=1 -v ./internal/core ./internal/migrate ./internal/appapi ./internal/adminapi -run 'Test(M6b|AppResource|AppTelemetry|AppAlarm|TelemetrySubmission|TelemetryHTTP|LegacyTenant|Platform|ProductionMux|Reviewer)'`

   **EXIT 0, 249 PASS markers, zero skips and zero failures.** Raw artifact: `/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.omo/evidence/m6b-final-reviewer-focused.log`.

2. To verify test sensitivity without editing the candidate, overlaid only `admin_store.go`, `telemetry_authorization.go` and `app_resource_scope.go` from rejected `c56cb22` using Go `-overlay`, then ran the final telemetry-version, partial-context, and alarm-revocation tests against the real isolated DB.

   **Expected EXIT 1, zero skips.** Telemetry returned 202 and changed state; alarm version cases returned 200/204 and changed state; all four singular/batch open/confirmed partial-context subtests failed. This independently demonstrates that the new tests detect the reported bugs. Overlay map, original files and raw red log: `/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.omo/evidence/m6b-final-baseline-overlay/` (`red.log`, `overlay.json`). These are sensitivity evidence, not candidate failures.

3. Independently checked correction source/contract whitespace (`git diff --check c56cb22...HEAD -- internal docs/api docs/architecture docs/IMPLEMENTED.md`), executable-source-to-candidate paths, exact HEAD, and superproject/submodule cleanliness. Passed. Historical raw browser context/log whitespace is preserved; no branch-wide clean-whitespace claim is made.

## Inspected final committed evidence

`docs/evidence/acceptance/R37/2026-10-04-version-revocation/verification.json` cites `ef6510f` and the exact unchanged web snapshot. All eleven artifact paths exist and their raw logs end with `EXIT 0`. The full `make verify` raw output includes build, vet and the Go race/shuffle run. Focused DB logs have 239 PASS markers and no skips; reviewer/fake-executor logs have 14 PASS markers and no skips. Contracts report 53 operations/245 synthetic fixtures; architecture reports 55 requirements. Frontend artifacts report 21 unit tests, successful typecheck/build and 16 browser scenarios. The README explicitly bounds browser evidence to the demo UI adapter and real backend evidence to HTTP/Timescale tests.

Inspected raw red/green and fixture-failure/fixture-green artifacts rather than accepting their titles. The current full verification receipt replaces the rejected candidate's unsupported claim. Historical approvals and probes are labelled historical, and docs/IMPLEMENTED.md and the M6b gate keep completion pending fresh dual approval.

## Boundaries of this approval

This is one independent approval of the exact candidate named above, with MEDIUM/LOW quality follow-ups and no blockers. It covers R36.a/R37.a software scope. It does not mark M6b complete by itself, approve later resources, or claim hardware, real WeChat, public TLS or capacity acceptance. Those remain separately gated or `external_blocked` as documented.
