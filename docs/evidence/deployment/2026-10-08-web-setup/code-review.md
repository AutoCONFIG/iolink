# 首启初始化最终代码审阅 — APPROVE

- codeQualityStatus: WATCH
- recommendation: APPROVE
- requestedVerdict: APPROVE
- reportPath: .omo/evidence/first-run-setup-code-review-final.md
- blockers: []
- 日期：2026-10-08，Asia/Shanghai
- 同一冻结候选：根 `360bc8742ce9392c9cfdcd25f295050540f5fee7`，web `b340caedd038d1c80504aeffec8de7927b2871ba`。

## 审阅范围及输入

批准本轮空库自动迁移、Web 首个管理员初始化、业务 HTTP 门禁、认证/Origin/输入/并发/失败回滚/日志脱敏的代码范围。没有沿用 M4 或原候选的批准；原候选 `7d238d9` / web `2647569` 由本 reviewer 明确 REJECT，收据为 `.omo/evidence/first-run-setup-code-review.md`。

依据已读的 docs/README.md 权威关系、docs/CONTRIBUTING.md、PLAN/PLAN-DETAILS、ACCEPTANCE R03/R04、DEPLOY、setup-openapi 和 IMPLEMENTED S4，独立核对完整原始 diff 与修正 diff。根总体起点 `c85a0db`，web 起点 `66c5262`。完整快照和身份收据：`.omo/evidence/first-run-setup-review-final/identity.json`、`snapshot.diff`、`web-snapshot.diff`。

本轮主要文件：cmd/iolinkd/main.go；internal/domain/bootstrap.go；internal/platform/bootstrap.go / web_bootstrap.go；internal/setupapi/server.go / server_test.go；docs/api/setup-openapi.yaml、验收/部署/状态/证据文档及 scripts/check_contracts.py；web 的 src/api/setup.ts、src/router/index.ts、src/views/SetupView.vue、src/styles/main.css、tests/setup-api.test.ts、vite.config.ts。

ulw-loop status 返回 ULW_LOOP_PLAN_MISSING，因此使用 `.omo/evidence` 回退收据路径，并按调用方要求单独保留 final 文件。未提供 notepad，仓库没有 `.omo/notepads`；探针、输出和 diff 在上述审阅证据目录中。工作树源文件未修改，只有审阅 artifact 和临时测试 fixture/overlay；复审结束时两仓 HEAD 与冻结值一致，根仅有既存未跟踪 `.tmp/`。

## 原阻断项复审

| 原项 | 独立复审结果 |
|---|---|
| H1：Vite 缺 setup 代理，非 demo 导航被带往初始化失败页 | CLOSED。web/vite.config.ts:11 已装配 /setup/v1；实际按当前配置启动 Vite，通过它访问临时 localhost:8080 HTTP fixture，得到200 application/json 与 required=true，fixture确实收到同一路径。 |
| M1：Origin 实现和契约不一致/非 Origin 结构被接受 | CLOSED。契约明确同 Host（含端口）的 http/https，用于 TLS 终止反代；仍须安装密钥，无 cookie 认证和 CORS。server.go:82 拒绝 userinfo/path/query/fragment；真实数据库新增允许 HTTPS 反代和拒绝错误 host/port/附带内容用例全部通过。 |
| M2：结果未知可再次盲 POST | CLOSED。web/src/api/setup.ts:33 每次写入前读 status；:40 网络或5xx后再查状态。已完成则返回成功，由页面进入登录；查询不可用时抛安全错误，下一次调用在状态查询失败前不会发 POST。新增 lost-response / unavailable-status 用例区分此行为且独立通过。 |
| M3：非法 JSON 的 oversize fixture 无法验证大小限制 | CLOSED。server_test.go:95 改为4097空格加合法凭据。正常 suite通过；独立 Go overlay 删除 MaxBytesReader 后，该用例确实失败（实际204、预期400），不再是假覆盖。 |

## Findings by severity

### CRITICAL

无。

### HIGH

无。

### MEDIUM

**M1 — 继承的 composition root 超长，继续存在维护风险；本轮非阻断。**

`cmd/iolinkd/main.go:60`，按技能纯 LOC 规则，整体起点373、当前386。该文件同时承载命令分派/输入、数据库准备、服务装配/生命周期以及 HTTP 挂载/SPA；没有 SIZE_OK 注释。违反两个技能的模块大小门槛，后续可按责任拆开。当前增量和调用关系已完整检查，没有发现它导致本轮初始化的正确性、权限或回滚失败；不据此强行扩大本轮为全 composition root 重构。

### LOW

- `internal/platform/bootstrap.go:16` 注释仍称调用只来自本地 CLI / stdin；现在也服务 WebBootstrap。注释应同步实际入口。
- `web/tests/setup-api.test.ts:31` 的精确 JSON body 字符串对属性顺序有维护耦合。该测试整体仍验证真实 wire 路径和凭据只用于初始化的行为；可以改用解析后的字段断言。新增 GET/POST 计数用于断言状态确认和禁止重复写入的明确协议动作，有实际回归价值。
- 原 Docker 原始输出有尾随空白，旧 commit 全 diff whitespace 检查非绿色；README 已明确 NOTE，没有声称全部原始日志通过空白检查。无生产源码空白缺陷。

## 独立运行及证据

| 检查 | 结果 | 原始证据 |
|---|---|---|
| IOLINK_TEST_PG_DSN=<isolated> GIN_MODE=release go test -race -shuffle=on -count=1 -v ./internal/setupapi ./internal/platform ./cmd/iolinkd | PASS；真实专属 PostgreSQL16/Timescale2.30.0，随机隔离 DB 实际执行，无 DB SKIP | `.omo/evidence/first-run-setup-review-final/backend.log` |
| npm test --prefix web && npm run typecheck --prefix web | PASS；6 files / 52 tests，vue-tsc退出0 | 同目录 `web.log` |
| make verify-contracts；architecture manifests --all；go vet ./internal/setupapi ./internal/platform ./cmd/iolinkd | PASS；69 operations/343 synthetic fixtures、55 requirements、vet无输出 | 同目录 `static-checks.log` |
| 实际当前 Vite 配置，临时后端 localhost:8080，GET /setup/v1/status | PASS；200 application/json，required=true，backend记录该请求，两个 fixture服务已关闭 | 同目录 `vite-proxy-probe.json` |
| 仅删除生产 body size limit 的 Go overlay，再运行 oversize 测试 | EXPECTED FAIL；实际204使测试失败，证明新增用例能发现关键限制回归 | 同目录 `body-limit-mutation.log`、`body-limit-overlay.json`、`server-without-body-limit.go` |

原候选的 internal/migrate 全包 race/shuffle 已由本 reviewer 独立通过，见 `.omo/evidence/first-run-setup-review/backend.log`；本次该迁移执行器/SQL与 main 装配没有进一步变化。没有为复审再次无目的运行全库。

独立检查执行方目录 `docs/evidence/deployment/2026-10-08-web-setup/` 的 README、backend.log、full-verify.log、contracts.log、architecture.log、Docker build/start、status-before/init-status/status-after/readyz，以及 review-fixes-backend.log、review-fixes-web.txt。提供的 frontend build 摘要退出0、1817 modules，保留 Zod annotation 和大 chunk 警告；本 reviewer 独立重跑测试/类型检查和真实 Vite proxy，以验证具体修复，不将摘要当成零警告或浏览器验收。原 Docker build/start 输出属于原功能候选，作为不变启动链的补充证据，未伪称最终镜像 digest 在本 reviewer 环境重建并验收。

## 正确性、架构与日志核对

- 常驻入口监听前执行 migrate.Up；事务锁、checksum/history/dirty/newer DB 拒绝条件保留，失败即返回。
- WebBootstrap 通过已有 BootstrapAdmin 创建管理员；Web、CLI 使用同一 advisory lock，账户和 audit 同事务，外部副作用未混入。真实并发仅一次成功、重复拒绝、audit失败回滚、DB不可用fail closed皆通过。
- admin/open/app v1和v2装配都用实时 setup.Protect；未初始化/状态未知时拒绝，初始化成功后不需重启就开放登录。没有引入状态缓存或静默降级。
- 窄小 store capability seam 与 provider SQL 分离；新增 domain 只有 typed sentinel errors，无 provider SDK或未来 provider虚假支持。使用已有初始化事务，没有复制业务写入逻辑。
- 输入解析处于 HTTP/API 响应边界，UnknownFields、媒体类型、EOF和请求体上限服务明确目标；新状态GET由不确定结果处理需要，没有额外抽取/规范化层或插件抽象。
- composition root 注入 observability slog；setup 使用 RequestLogging 的 route template和服务端request ID。日志消息常量、err由安全类别/SQLSTATE过滤，不记 body/header/query/原始 URL/密码/密钥。页面没有新增敏感浏览器存储。

## Skill perspective check

**已运行。** 本会话明确加载并咨询 `remove-ai-slops/SKILL.md` 与 `programming/SKILL.md`，路径均位于 `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.19/skills/`，并读 Go/TypeScript 参考。对完整新增生产代码和测试及修正 diff 再做独立过拟合/slop审阅。

没有 deletion-only、仅验证要求删除的测试、自然语言 prompt pins、自算输出期望值或独立常量镜像测试。原 oversize 假覆盖已修复并做变异验证。真实 DB 与 real wire 测试支持测试层次；两个新增未知结果 unit 用例确实能因行为回归失败，spy只隔离 HTTP错误/状态边界。没有新 untyped domain escape hatch、无需求数据抽取/解析/规范化或无测试价值的抽象。复杂 Origin 谓词中的每项都是约定边界条件，未借此引入多余框架。

**剩余 diff 仍违反两个技能的模块大小观点（MEDIUM M1）**，精确 JSON 顺序是 LOW 维护耦合；未发现剩余造成本轮正确性/回归的 skill violation。按角色要求只记录，没有实现清理。

## 验收限制与最终结论

`APPROVE`，`blockers: []`。批准冻结的本轮初始化代码与协议修复，保留上述非阻断维护项，所以 codeQualityStatus 为 WATCH。

浏览器完整流程/视觉验收明确由用户承担，此报告没有浏览器 PASS，也不宣称 R04 或 M0–M8 所有验收完成。外部微信/硬件、客户离线首启、公网TLS、远程发布/CI/镜像状态不属于本次代码审阅的通过结论。已检查门审 final 收据的同一根/web身份，但本结论来自上述独立 diff/测试/探针，不继承其批准。
