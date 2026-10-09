# 产品与物模型角色权限修复：独立门审

- recommendation: **APPROVE（同一快照的软件范围）**
- reviewed root snapshot: `cf7727f40d66242af676daa728bcdb38e5cbcb77`
- reviewed web snapshot: `03b4286ae9b30ca0358b706ee3a62feace69abf6`
- production edits by reviewer: none
- review date: 2026-10-09

## originalIntent

修复普通租户成员访问产品目录和物模型列表时错误返回 403 的问题，同时保持权限模型：member/viewer/support 可读产品资源，owner/admin 可写产品和物模型；平台 `ADMIN` 与普通业务用户的 `/admin/v1`、`/user/v1` 路由分层保持不变；已有功能继续正常工作。

## desiredOutcome

在同一代码快照上，普通 member 通过 `/user/v1/products` 与 `/user/v1/products/{id}/models` 得到 200，写操作得到 403；owner/admin 的产品写策略仍为允许；Web 产品页对 owner/admin 显示创建、建模、发布、设备绑定控件，对其他租户角色只提供读取；平台控制面和 user/admin 路由边界不被这次修复改变。线上部署是否已替换该快照需单独核实。

## userOutcomeReview

**通过（bounded software gate）。** `internal/adminapi/products.go` 删除了两个 GET handler 中重复的 owner/admin 判断，GET 现在统一由 `tenantRequired` 的 resource/action 策略执行。`tenantPermission` 对 GET 映射为 `read`；`authorization.Policy` 为 member/viewer/support 的 `products` 授予 `read`，为 owner/admin 的 `products` 授予 `*`，因此写操作仍只允许 owner/admin。产品路由仍同时由既有 `Routes`/`UserRoutes` 挂载，未改平台端点或 user/admin 分层。

Web 子模块的 `ProductsView.vue` 通过 `tenantRole` 计算 `canManage`，仅 owner/admin 渲染创建、建模、发布和设备升级控件；目录和模型读取对其他租户角色仍可见。产品 API 函数仍走既有 `userHttp`，这次没有把平台请求移入 user 路由。

我复现了后端 focused race tests：

```text
go test -race -shuffle=on ./internal/adminapi -count=1        PASS
go test -race -shuffle=on ./internal/adminapi ./cmd/iolinkd -count=1  PASS
```

契约检查也通过：

```text
make verify-contracts
96 target operations, 568 synthetic request/response fixtures PASS
```

Web 验证中，一次与其他命令并行执行时 `console-session.test.ts` 出现 5 秒 SSR timeout；按证据中声明的原命令单独重跑后通过：

```text
npm test -- --run       8 files / 58 tests PASS
npm run typecheck       PASS
npm run build           PASS
```

该单次 timeout 未在单独重跑中复现，故不构成当前快照的软件门阻断；它应作为后续测试稳定性观察项保留。

## criterion audit

| Criterion | Result | Evidence pointer |
|---|---|---|
| C1 member 可读产品目录和物模型 | PASS | `internal/adminapi/products.go:39-50,96-120`; `internal/adminapi/auth.go:174-193`; `internal/adminapi/server_test.go:364-374` |
| C2 owner/admin 可写，member 写入被拒绝 | PASS（策略 + member HTTP 回归） | `internal/authorization/policy.go:33-45`; `internal/adminapi/server_test.go:375-388`; persistence write authorization `internal/persistence/products.go:140-205,226-272` |
| C3 ADMIN/user route 分层不变 | PASS | `internal/adminapi/server.go:220-241`; prior split gate `docs/evidence/deployment/2026-10-09-user-api-split/gate-review.md:C1-C4`; current diff only removes GET role guards |
| C4 Web 仅 owner/admin 显示写控件，其他角色可读 | PASS | `web/src/views/ProductsView.vue:22-23,67-76`; web commit `03b4286` |
| C5 已有测试/契约功能正常 | PASS（含重跑） | `docs/evidence/deployment/2026-10-09-product-role-fix/README.md:验证`; reproduced Go/web/contract commands above |
| C6 发布/线上边界记录准确 | PASS as a bounded gate; external deployment pending | `docs/evidence/deployment/2026-10-09-product-role-fix/README.md:发布`; `git ls-remote github refs/heads/main` = `e7be77d`, while reviewed root is `cf7727f` |

## remove-ai-slops and programming pass

我直接检查了 `cf7727f` 与 Web 子模块 `03b4286` 的生产和测试 diff，并加载了 `remove-ai-slops` 与 `programming` 标准。删除的 handler 守卫是本次根因修复，不是无效删除；新增 HTTP 回归断言真实可观察的状态码；没有 deletion-only、由输出推导期望值、测试仅复制实现、无关生产抽象、宽泛异常捕获、debug 输出、死代码或新增未类型化逃逸。`fakeProductCatalog` 仅用于既有 httptest seam，生产文件大小和边界未因本次修改恶化。相邻旧 code review 报告明确记录了两项 skill perspective；本报告对当前快照再次完成直接 pass。

## blockers

[]

## checked artifact paths

- `git show --find-renames cf7727f`
- `internal/adminapi/products.go`
- `internal/adminapi/auth.go`
- `internal/adminapi/server.go`
- `internal/adminapi/server_test.go`
- `internal/authorization/policy.go`
- `internal/persistence/products.go`
- `web/src/views/ProductsView.vue`
- `web/src/api/admin.ts`
- `docs/evidence/deployment/2026-10-09-product-role-fix/README.md`
- `docs/evidence/deployment/2026-10-09-user-api-split/code-review.md`
- `docs/evidence/deployment/2026-10-09-user-api-split/gate-review.md`
- `docs/IMPLEMENTED.md`
- `docs/EXTENSIONS.md`
- `docs/ACCEPTANCE.md`
- `docs/README.md`
- Web commit `03b4286`
- `make verify-contracts` output
- focused Go race test outputs
- Web test/typecheck/build outputs

## exact evidence gaps and deployment boundary

1. `docs/evidence/deployment/2026-10-09-product-role-fix/` contains a README only; it does not include captured `/readyz`, Compose health, or live member request logs. The README's online read-only statements are therefore treated as unverified prose, not as production acceptance.
2. The reviewed root commit `cf7727f` is not on the remote `github/main` (`github/main` remains `e7be77d`, tag `v0.0.14` remains immutable), and no new root release tag/image was found. Web `03b4286` is on `github/main`. The existing online image has not been replaced by this root snapshot.
3. Consequently this APPROVE covers the reviewed source snapshot and reproducible software checks only. It does **not** claim that a deployed service currently serves member product/model GETs; after publishing a root tag/image, rerun `/readyz`, health, authenticated member GET 200, and member write 403 against that deployment.
4. No product-specific code-review artifact exists in this evidence directory. The direct diff review above supplies current-snapshot coverage; the neighboring user-API report is cited only for the unchanged route-split review.
