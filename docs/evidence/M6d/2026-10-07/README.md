# M6d / R41–R42 候选验收证据

日期：2026-10-07，修订及复验2026-10-08（Asia/Shanghai）。本目录是最新记录；2026-10-05 及旧拒绝报告
属于历史快照。双审 **pending**，尚不登记阶段完成。

## 冻结候选与环境

- 测试源码：`657807e195bcfc80b940706c58ad166a2de7a56f`。
- web：`66c52624f6d97c9dd7fe29b05b64b3db39c4a72d`。
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

`final-3-*` 与 `865-followup-web.log` 对应上述候选。
`final-2-go-verify.log` 对应旧21c3704；`final-06d2486-*` 的全库运行因浏览器
重复提示定位失败。两者保留为旧快照历史，不认证新候选。
`frozen-*` 对应旧 b4b9335/web207dcfa，
该快照在 d82aa82 审阅中被拒绝，不能用于批准新候选。其他 logs 为调查历史。

| 验证面 | 命令与证据 | 结果 |
|---|---|---|
| 全库构建/vet/竞态/乱序/真实 DB | `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated> make verify`；[日志](logs/final-3-go-verify.log) | passed |
| 真实管理浏览器 | 独立生产浏览器运行；[日志](logs/865-followup-web.log)及 [截图](browser/) | passed |
| 契约 | `make verify-contracts`；67 operations / 334 synthetic fixtures | passed |
| 架构映射 | `python3 scripts/check_architecture_manifests.py --all` | passed |
| web | `npm test --prefix web -- --run`、`npm run build --prefix web`（含vue-tsc）；46 unit tests；[日志](logs/865-followup-web.log) | passed |
| 既有管理浏览器回归 | `npm run e2e --prefix web -- --reporter=line`；17 scenarios，与真实管理单列 | passed |
| 集成镜像 | `docker build -t iolink:m6d-final3 .` | passed |

契约、架构、容器输出见 [日志](logs/final-3-contracts-docker.log)。
17个浏览器回归输出见 [日志](logs/865-followup-web.log)。
`make docs-tools` 同日已执行，见 [日志](logs/candidate-contracts-web-docker.log)。
合成契约不替代 handler 测试。保留既有 Rollup 注释/bundle 大小提示；
不声称 Lighthouse 或远端 CI 已通过。

## 真实业务正向及拒绝路径

| 需求 | 行为 | 测试证据 |
|---|---|---|
| R41 Key 生命周期 | Secret 一次性展示、AES 加密库存；轮换旧 Key 和撤销即时失效；审计无 Secret | `TestM6dIssueAndAuthenticateOpenKey`、真实管理浏览器 |
| R41 权限和 License | viewer/member、停租户、缺 feature 拒绝；撤权上下文不能签发且无部分写入 | `TestM6dAdministration_rejectsStaleMembershipWithoutPartialWrite`、`TestM6dSignedHTTP_rejectsScopeTenantFeatureAndReplays`、原 M6d integration |
| R41 资源限制 | 农场/池塘/设备单独限制的列表过滤；无权限及跨租户详情 404 | `TestM6dResourceHTTP_filtersListsAndDetails`、`TestM6dDeviceResourceHTTP_filtersDevicesAndAlarms` |
| R41 UI/API 边界 | 新租户列表 `[]`；未知字段、null、非法资源项 400；外租户资源 404；坏 ID/设备逗号输入无请求且不签发；scopes 选择/展示和 snake_case 资源实际持久化；坏成功响应拒绝 | `m6d-live.spec.ts`、`api-key-scope.test.ts`、`api-key-api.test.ts` |
| R41 范围变更完整性 | 签发校验当前租户存在且启用的资源；轮换重新校验归属；拒绝无 Key/审计部分写入且保留旧 Key | `TestM6dResourceValidation*`、`TestM6dRotation_rejectsChangedResourceOwnershipWithoutWrites` |
| R42 签名 | 发布固定向量调用生产验证器；RFC3986/重复与空查询/路径/时间窗；body-only 篡改拒绝且不占 nonce | `TestM6dPublishedVector_authenticatesWithProductionVerifier`、`TestCanonicalOpen*`、`TestM6dBodyDigest_rejectsOnlyChangedBodyAndPreservesNonce` |
| R42 body 上限 | 4 MiB 可认证，多 1 byte 在认证前 413，不接受未签名尾部 | `TestRoutesRejectOversizedBody` |
| R42 replay/rate | 并发 nonce 仅一个成功；burst10 和 60/min refill；真实 429+正数 Retry-After | `TestM6dConcurrentNonceAndRateLimit`、`TestConsumeOpenRate*`、真实 signed HTTP |
| R42 时间窗/配置 | 正确重签的 ±300/±301、零/负数/int64 极值/溢出差值；部署 30/min burst2 的初始化、轮换、补充和 HTTP Retry-After2；非法配置拒绝 | `TestM6dTimestampWindow*`、`TestConfiguredOpenRate*`、`TestM6dConfiguredRate*`、`TestOpenAPIRateConfig*` |
| R42 限流重放/编码 | 429保留认证nonce且不消耗token，补充token后相同nonce401、新nonce200；UTF-8非法路径/查询拒绝 | `TestM6dThrottle_preservesNonceAndLeavesTokenUnconsumed`、`TestCanonicalOpen_rejectsInvalidUTF8` |
| R41 错误/演示边界 | 内部错误500且无错误原文，输入错误400；演示存储Zod解析；刷新隐藏密钥、撤销及日志导航 | `TestAPIKeyError*`、真实管理浏览器、web unit/demo e2e |
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

## d82aa82 审阅修复

[独立代码报告](reviews/code-d82aa82.md)明确 REQUEST_CHANGES：设备逗号输入扩大范围、
时间差 int64 溢出、持久化非法/null资源、缺少部署限流配置。代码审阅已落盘后通道中断；
gate 通道因平台内容审核中断，无最终结论，记录为 INCONCLUSIVE。
两者均不算批准。代码报告中的重复响应解析、无用 activation 分支与误导测试也已简化。
时间窗 [red](logs/time-window-red.log)证明原缺陷；最终
[focused green](logs/review-fixes-final-focused.log)使用真实隔离 DB 与 race/shuffle。
浏览器回归新增逗号-only、混合空设备项、null/错误资源和外租户请求，截图重新生成；
旧截图可在 d82aa82 的 Git 快照查看。

## 865e9b2 审阅修复

[代码报告](reviews/code-865e9b2.md)REQUEST_CHANGES，[gate报告](reviews/gate-865e9b2.md)
APPROVE，两者同源865e9b2/web3c1d860；任一拒绝就不能发布。
新修复保留429的nonce记录，同时纠正内部错误500、UTF-8校验、演示存储解析和
演示行为测试；M5参数对象为非阻断建议，本轮保持接口。
[red](logs/865-followup-red.log)实际复现三个失败，[green](logs/865-followup-green.log)
覆盖真实HTTP429/重放及相关DB测试。独立批准记录按快照保存在[ledger](reviews/ledger.md)。

镜像导出配置 digest：`sha256:cf67dc03509da4590ad189b3cc2714331d8779cc3d4b1c3b34bcce32a929b45f`；
本地镜像 manifest list ID：`sha256:9964190ed12da00ce932cb65eea9587d253c3e601b7d9688bcd402ba95ba86cb`。
最终全库 core 竞态测试117.999s；独立真实管理浏览器通过。截图已逐张核验。
窄屏表格使用内部横向滚动，
不代表全部列同时显示。
冻结源码之后仅归档日志和更新 pending 文档；审阅候选中的产品代码必须与上述
测试源码一致，最终完成登记也只允许修改证据和状态文档。
