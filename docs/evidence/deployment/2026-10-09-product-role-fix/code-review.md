# Product role fix bounded code review

## Verdict

- reviewed snapshot: `cf7727f40d66242af676daa728bcdb38e5cbcb77`
- reviewed Web submodule: `03b4286ae9b30ca0358b706ee3a62feace69abf6`
- codeQualityStatus: **WATCH**
- recommendation: **APPROVE**
- blockers: **[]**

This approval is limited to the products role fix in the reviewed source snapshot. It does not certify that a deployed image has been replaced with `cf7727f`.

## Scope and goal

Reviewed only the products handler change, the focused Go regression additions, and the Web products view/test changes in `cf7727f`:

- `internal/adminapi/products.go`
- `internal/adminapi/server_test.go`
- Web commit `03b4286` (`src/views/ProductsView.vue`, `tests/console-session.test.ts`)

The goal is for `member`, `viewer`, and `support` tenant roles to read products and model versions, while product/model creation, publication, and device assignment remain restricted to `owner`/`admin`. The UI should expose write controls only to `owner`/`admin`.

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

**M1 — Web role-controlled product controls have no dedicated regression test.**

- `web/src/views/ProductsView.vue:22-23,67-76` computes `canManage` from the tenant role and conditionally renders creation, model-edit, publish, and assignment controls.
- The Web diff changes the view but adds no ProductsView test. The existing `tests/console-session.test.ts` change only supplies a `getPlatformStats` mock for dashboard SSR and does not exercise `canManage` or product control visibility.
- A later UI regression could expose write controls to a read-only role or hide them from an owner/admin while the current 58-test suite remains green. This is a coverage gap, not a present authorization bypass because the backend policy remains authoritative.

**M2 — The new Go regression covers one write route, not the complete write matrix.**

- `internal/adminapi/server_test.go:364-389` proves member GETs for `/products` and `/products/1/models` return 200 and proves member `POST /products` returns 403.
- It does not directly exercise member model creation, model publication, or device-product assignment, nor an owner/admin positive write. `tenantRequired` and the policy map all these paths to write actions, so source inspection supports the intended result, but the test does not pin every changed outcome.
- This is a non-blocking test completeness risk; add a table-driven matrix when this area next changes.

### LOW

**L1 — Deployment evidence remains bounded to source and local checks.**

`docs/evidence/deployment/2026-10-09-product-role-fix/README.md` records live `/readyz` and member-read observations but does not include captured request/response logs. The root snapshot `cf7727f` is also not the published root `github/main` in this checkout. Treat production replacement and live member verification as pending release work.

## Correctness review

The two redundant owner/admin checks were removed from the GET handlers in `internal/adminapi/products.go:39-50,96-120`. GET authorization now comes from `tenantRequired` and `tenantPermission`, which maps GETs to `read`; `internal/authorization/policy.go` grants `products:read` to `member`, `viewer`, and `support`, while owner/admin retain `products:*`. Write routes remain behind the same middleware and therefore require `write`, preserving the 403 boundary for read-only roles.

The focused member test uses an HTTP server and a catalog seam, asserts observable status codes, and is not deletion-only or tautological. The Web view's `canManage` condition matches the backend role split. No changed production code introduces an untyped escape hatch, broad exception handling, needless abstraction, dead code, or parsing/normalization unrelated to the goal.

## Skill-perspective check

The `remove-ai-slops` and `programming` skills were loaded and applied to the changed Go production/test code and the Web TypeScript/Vue production/test diff. No deletion-only tests, implementation-constant mirrors, prompt tests, or unnecessary production extraction were found. The two MEDIUM items above are missing behavioral coverage, not overfit tests. No new 250-pure-LOC module or boundary violation was introduced by this snapshot.

## Verification performed

Independent commands on the reviewed snapshot:

```text
go test -race -shuffle=on ./internal/adminapi -count=1        PASS
npm test -- --run                                          PASS (8 files, 58 tests)
npm run typecheck                                          PASS
npm run build                                              PASS
make verify-contracts                                      PASS (96 operations, 568 fixtures)
git diff --check cf7727f^ cf7727f                          PASS
git -C web diff --check 953335e..03b4286                  PASS
```

No implementation files were edited during this review.
