# M6d / R41–R42 候选验收证据

日期：2026-10-07（Asia/Shanghai）。本目录是最新记录；2026-10-05 及旧拒绝报告
属于历史快照。双审 **pending**，尚不登记阶段完成。

## 冻结候选与环境

- 测试源码：`b4b93359976c0529c8f52f50b02a968e3f99750e`。
- web：`207dcfa751545e11a63a3a013da7e60552fecd75`。
- 基线：source `v0.0.9` / `9f46b6e549d36a6eac8c197acfbe64d2a91585a4`；
  web `0aa7771acf0ef6322c9ac1e4c9839616646bb675`。
- Go 1.26.8、Node 24.21.0、Chrome Playwright、Linux amd64。
- 专属容器 `iolink-todo9-pg`：PostgreSQL 16.15 / TimescaleDB 2.30.0，
  `127.0.0.1:55439`。隔离 DSN 通过 `IOLINK_TEST_PG_DSN` 提供，不记录凭据。
  测试通过 `internal/testdb` 创建、删除自己的数据库。
- 真实管理测试使用 `npm run build --prefix web && make embed-front` 的生产前端，
  真实 admin HTTP 和 Timescale；不使用 demo adapter。
- 用户 `.github/workflows/ci.yml` 与 `.tmp/` 改动排除在提交/审阅范围之外。

## 执行命令

frozen 日志只对应上述候选；其他 logs 为调查历史，不代替最终验收。

| 验证面 | 命令与证据 | 结果 |
|---|---|---|
| 全库构建/vet/竞态/乱序/真实 DB 与 UI | `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated> IOLINK_M6D_BROWSER=1 make verify`；[日志](logs/frozen-go-verify.log) | passed |
| 契约 | `make verify-contracts`；67 operations / 333 synthetic fixtures | passed |
| 架构映射 | `python3 scripts/check_architecture_manifests.py --all` | passed |
| web | `npm test --prefix web -- --run`、`npm run typecheck --prefix web`、`npm run build --prefix web`；44 unit tests | passed |
| 既有管理浏览器回归 | `npm run e2e --prefix web -- --reporter=line`；17 scenarios，与真实管理单列 | passed |
| 集成镜像 | `docker build -t iolink:m6d-b4b9335 .` | passed |

契约、架构、web、容器连续输出见 [日志](logs/frozen-contracts-web-docker.log)。
`make docs-tools` 同日已执行，见 [日志](logs/candidate-contracts-web-docker.log)。
合成契约不替代 handler 测试。保留既有 Rollup 注释/bundle 大小提示；
不声称 Lighthouse 或远端 CI 已通过。

## 真实业务正向及拒绝路径

| 需求 | 行为 | 测试证据 |
|---|---|---|
| R41 Key 生命周期 | Secret 一次性展示、AES 加密库存；轮换旧 Key 和撤销即时失效；审计无 Secret | `TestM6dIssueAndAuthenticateOpenKey`、真实管理浏览器 |
| R41 权限和 License | viewer/member、停租户、缺 feature 拒绝；撤权上下文不能签发且无部分写入 | `TestM6dAdministration_rejectsStaleMembershipWithoutPartialWrite`、`TestM6dSignedHTTP_rejectsScopeTenantFeatureAndReplays`、原 M6d integration |
| R41 资源限制 | 农场/池塘/设备单独限制的列表过滤；无权限及跨租户详情 404 | `TestM6dResourceHTTP_filtersListsAndDetails`、`TestM6dDeviceResourceHTTP_filtersDevicesAndAlarms` |
| R41 UI/API 边界 | 新租户列表 `[]`；未知 camelCase 请求字段 400；坏 ID 不签发；scopes 选择/展示和 snake_case 资源实际持久化；坏成功响应拒绝 | `m6d-live.spec.ts`、`api-key-scope.test.ts`、`api-key-api.test.ts` |
| R42 签名 | 发布固定向量调用生产验证器；RFC3986/重复与空查询/路径/时间窗；body-only 篡改拒绝且不占 nonce | `TestM6dPublishedVector_authenticatesWithProductionVerifier`、`TestCanonicalOpen*`、`TestM6dBodyDigest_rejectsOnlyChangedBodyAndPreservesNonce` |
| R42 body 上限 | 4 MiB 可认证，多 1 byte 在认证前 413，不接受未签名尾部 | `TestRoutesRejectOversizedBody` |
| R42 replay/rate | 并发 nonce 仅一个成功；burst10 和 60/min refill；真实 429+正数 Retry-After | `TestM6dConcurrentNonceAndRateLimit`、`TestConsumeOpenRate*`、真实 signed HTTP |
| R42 持久状态 | 新 Service 拒绝已用 nonce，读取共享 DB rate state | `TestM6dRestart_preservesConsumedNonceAndRateState`；不是进程崩溃恢复演练 |

真实浏览器覆盖签发、刷新后 Secret 消失、轮换、撤销、日志导航；375/768/1440
宽度无页面横向溢出。Secret 所在整个 `.el-alert` 遮罩且关闭动画，截图见
[browser/](browser/)。页面/审计 HTTP 均检查不含 Secret；token 在测试进程内
生成和传递，不写日志。

## 旧审阅修复及验证边界

旧 `.omo/evidence/m6d-code-review.md` 拒绝 `2b46c21`，不能作为当前批准。
已补实时事务授权、设备限定池塘、body 溢出、注入日志器、完整契约、scope 选择/展示、
严格资源 ID、空列表、生产向量/body/restart/浏览器和 typed response 解析。
撤权签发的真实 [red](logs/http-red.log) 与修复后 [green](logs/http-green.log) 保留。
新解析器首轮误将 Go `omitempty` 的 `revoked_at` 设为必填；真实浏览器 201 后
不显示 Secret，已修 optional 并加省略字段回归。直接测试前须重新构建/嵌入前端，
不能使用旧 dist 认证新源码；最终 frozen 日志重新执行。

R41/R42 为软件范围；跨阶段微信/硬件/客户安装仍 `external_blocked`。
Redis 仅调研，不代表已支持。两位未参与编辑的独立审阅者须针对同一冻结候选
明确 APPROVE，之后再更新 IMPLEMENTED、acceptance/provider 与 release gate。

镜像配置 ID：`sha256:0a6aa019ff7030e5229b90d2a328d7fa64d84cedf93a931f8ab1d9156c04ad82`。
最终全库 core 竞态测试 91.454s，真实浏览器在该次执行中开启。
执行者逐张打开最终 375/768/1440 签发截图及审计截图，确认 Secret 遮罩、页面
排版及审计记录；窄屏表格使用内部横向滚动，不代表全部列同时显示。
冻结源码之后仅归档日志和更新 pending 文档；审阅候选中的产品代码必须与上述
测试源码一致，最终完成登记也只允许修改证据和状态文档。
