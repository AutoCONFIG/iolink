# Debug dump 最终固定候选独立门审

recommendation: **APPROVE**

blockers: []

候选 tree：`2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b`。base：`ec1fb049f2316a2e5ec8c3e8509d797adf002334`。冻结导出：`/tmp/iolink-debugdump-review-2bd5fd8`。日期：2026-10-13，Asia/Shanghai。审阅者未参与源码编辑；只写本报告，未修改源码或派出代理。

`omo-agent-toolkit ulw-loop status --json` 实际返回 `ULW_LOOP_PLAN_MISSING`，没有 currentAttemptDir。本报告使用要求的回退位置 `.omo/evidence/debug-dump-2bd5fd8-gate-review.md`。结论仅适用于该 tree，不批准共享工作树的无关修改，也不等于发布完成或全部外部验收通过。

## originalIntent

任务提供的用户原文要求 debug dump 覆盖多项记录，下游请求和内部处理细节便于定位问题；其他部分也写入 dump；每 6h 清理截至当前 6h 前的数据；以 `/debugdump-root/{version}/` 划分日志。

本次是已有功能诊断增强。仓库要求 composition root 注入 slog、日志脱敏、安全错误类别、设备 hash、业务与 outbox 原子性及外部 API/MQTT 语义保持。任务明确：本轮不要求从持久 outbox 延续上游 ID；不增加新业务功能；CI 仍只由 tag 触发。浏览器由用户验收；真实微信、硬件和 License 交付专项延后。

## desiredOutcome

运维人员在 `{root}/{version}/iolinkd.jsonl` 找到 HTTP、MQTT、core、数据库、License 准入、通知及微信调用的脱敏生命周期记录，按服务器生成的 ID 定位一次处理的成功或失败。文件始终保存 debug 及以上，console 独立按级别过滤。启动和默认每 6h 的清理按记录时间删除严格早于当前时刻 6h 的数据，覆盖当前与旧版本的活动/轮转文件。转储和清理失败可观察且 readiness 降级。

## userOutcomeReview 与成功标准

C1–C7 是本报告给任务中已有要求建立的引用标签，不增加范围。

| criterion | 明确要求 | 核对结果与 evidencePointer |
|---|---|---|
| C1 | 现有多阶段和下游诊断可关联、便于定位问题 | PASS。核对 access→Event→core→pgx tracer、HTTP context→DB/微信、worker→Claim/微信/Finish 的实际调用。读取并独立解析 `docs/evidence/deployment/2026-10-13-debug-dump/chain/*.jsonl`：成功有 committed，重复有 duplicate，事务错误有 failed/rejected；数据库 start/terminal span 全部成对。最终 worker 关键测试独立 race PASS，Claim 与 Finish ID 相同；Claim 和 Finish 错误为同 ID 的 WARN。 |
| C2 | 版本目录和独立 file/console 级别 | PASS。`internal/observability/logging.go` 校验 Version 并加入根目录，dump handler 固定 Debug，console 独立配置。Docker final stage 的 ARG→ENV 与 `.github/workflows/ci.yml` 的 tag build arg 连通。独立解析现场 `live-dump.jsonl` 531 条，其中 521 DEBUG；`live-stderr.log` 10 条 slog JSON，0 DEBUG。`live-http.txt` 的 setup/readiness 均 200，setup 请求 ID 与实际 dump 四条记录吻合。 |
| C3 | 默认每 6h 清理严格早于当前 6h 的记录 | PASS。`config.go` 默认两项 duration 都 6h；`New` 启动清理、`cleanupLoop` ticker 装配；`retention.go` 的 Before 保留相等边界。直接检查逐行 timestamp、旧版本/备份、误导 mtime、坏行保留、symlink/无关文件、启动/定时/关闭/并发测试及 `diagnostics-race.log`、`final-delta-race.log` 的实际结果。 |
| C4 | 转储/清理错误可观察、readiness 降级；CLI 仅 stderr | PASS。Sink 的错误/短写、console 故障、清理故障标记 failed，常量失败消息输出 stderr；`cmd/iolinkd/main.go` Ready 检查 Healthy，只有 serving 分支创建文件 sink。实际错误路径见 `dump_handler_test.go`、`retention_test.go`、`retention_lifecycle_test.go` 及专项 race 输出。最终通知 Claim/Finish 错误不会写成成功。 |
| C5 | 禁止原始请求、URL、SQL/参数、凭据、完整 payload 和明文设备 ID | PASS。完整差异中的新增消息均常量；safeAttr 类型/枚举边界已检查；pgx TraceQueryStart 忽略 SQL/Args，错误只安全类别/SQLSTATE，device hash。直接扫描 live/chain 实际产物，未发现测试敏感 fixture、SQL 或 MQTT topic。数据库注入错误只出现 `postgres_P0001`。最终 Claim error 的 raw fixture 未泄漏。 |
| C6 | 业务/API/MQTT与事务完整性不变，无新里程碑范围 | PASS。API、MQTT、迁移、domain、依赖 manifest、web gitlink、CI workflow 无差异。真实 TCP MQTT/PG 测试实际检查 sensor_data、telemetry、device_shadows、ingest_messages：重复单份、事务错误四表零部分写入。`contracts.log` 为 96 operations / 568 synthetic fixtures PASS，按静态契约限度解读。IMPLEMENTED 保持 S2a 正在进行及外部未验收状态。 |
| C7 | 验证如实记录；不把 skipped 或计划当 passed；完成状态需同快照独立双审 | PASS 作为本次独立候选门审。最终 delta 日志四包真实 DSN race PASS，无 SKIP；本审独立关键测试 PASS，并复现 Clone mutation FAIL。早期六个 setup SKIP 由真实 DSN 全库和独立第一轮 setup race 收据补验，SKIP 本身不当 PASS。全库 gbcatalog 11 项既有失败仍明确记录；阶段尚未在该 tree 记完成。最终代码审阅收据的对账见下节。 |

整体判断：本次候选交付了用户要求的诊断结果与清理机制。三文件修订修复前轮三项 NOTE；没有证据证明任一明确成功标准失败。无 blockers。

## 冻结身份与最终差异

通过 `git ls-tree -rz TREE` 和逐文件 `git hash-object`，独立核对冻结导出全部 **1047 个 blob**：missing `[]`，mismatch `[]`。没有用 executor 的292行筛选 manifest 代替整树校验。

`git diff --check BASE TREE` exit 0。前候选 `4bdc2d5a8fc688c13a309f64611c73691e054483` →最终 tree 只有三文件：

- `internal/notifications/worker.go`：服务器生成的 request ID 前移到 Claim 之前；Claim 错误新增同 ID 的 WARN；Finish 错误 INFO→WARN。
- `internal/notifications/worker_debug_test.go`：检查 Claim/Finish 相同 ID、Finish WARN 和 Claim 失败错误传播、无投递、副作用与脱敏输出。
- `internal/observability/context_test.go`：用调用方记录重用观察共享容量污染，生产 `context.go` 的 Clone 保留。

其他生产代码与前候选相同；前轮真实链路及全库收据可用于未变动部分。未为此次 delta 重复全库测试。

## 本审独立验证

环境：Go `go1.26.8 linux/amd64`。设置 `GIN_MODE=release`；`IOLINK_TEST_PG_DSN` 指向任务专属 disposable Timescale `127.0.0.1:33047/iolink`，不在报告复制口令。以下关键 delta 测试没有依赖该 DSN，未据此宣称它们验证了真实 DB；真实 DB 依据已核对的 final-delta/chain/全库日志。

从最终冻结导出执行：

```text
go test -race -shuffle=on -count=1 -v ./internal/observability ./internal/notifications -run 'TestContextHandlerDoesNotMutateSharedRecord|TestDerivedLoggerPreservesContextAndRedaction|TestWorkerCorrelatesAttemptAndDownstreamWithoutSensitiveData|TestWorkerClaimFailureIsCorrelatedAndSafe'
```

实际 exit 0，无 SKIP，完整 stdout 在本次门审工具记录。本报告保存关键输出：

```text
-test.shuffle 1791863138294598218
TestContextHandlerDoesNotMutateSharedRecord PASS
TestDerivedLoggerPreservesContextAndRedaction/attrs PASS
TestDerivedLoggerPreservesContextAndRedaction/group PASS
ok git.hyhy.fun/rsplab/iolink/internal/observability 1.016s
-test.shuffle 1791863138520738768
TestWorkerCorrelatesAttemptAndDownstreamWithoutSensitiveData/sent PASS
TestWorkerCorrelatesAttemptAndDownstreamWithoutSensitiveData/provider_rejected PASS
TestWorkerCorrelatesAttemptAndDownstreamWithoutSensitiveData/persistence_failed PASS
TestWorkerClaimFailureIsCorrelatedAndSafe PASS
ok git.hyhy.fun/rsplab/iolink/internal/notifications 1.029s
```

独立 mutation：先读取 `/tmp/iolink-debugdump-clone-overlay.json`，diff 确認 `/tmp/iolink-debugdump-clone-mutant.go` 唯一变化为删除 `record = record.Clone()`；overlay 指向共享目录。再分别核对共享 `context.go` 和 `context_test.go` blob 与最终 tree 一致，然后执行：

```text
go test -race -count=1 -v -overlay /tmp/iolink-debugdump-clone-overlay.json ./internal/observability -run '^TestContextHandlerDoesNotMutateSharedRecord$'
```

实际 exit 1，明确失败：

```text
context_test.go:68: caller record polluted by prior context:
{"level":"INFO","msg":"","count":0,"count":1,"count":2,"count":3,"count":4,"count":5,"count":6,"count":7,"!BUG":"redacted","outcome":"next"}
--- FAIL: TestContextHandlerDoesNotMutateSharedRecord
FAIL git.hyhy.fun/rsplab/iolink/internal/observability 0.012s
```

没有写入 mutation 源码或 overlay，使用 executor 已有文件；生产 Clone 未改变。该结果与 `clone-mutation-fails.log` 相符。旧 NumAttrs 测试的虚假信心问题已关闭。

现场产物亦独立重算，未信任 `live-check.json` 的 verdict：531 条 file JSON、10 条 console slog JSON；同响应 ID 的 HTTP start、DB start/terminal、HTTP 200 共4条与 live-check 的 linked_records 逐项相同。MQTT三场景及HTTP License JSONL全部能解析，数据库 span 均有对应 terminal；安全 fixture 扫描无命中。该动作是产物复核，没有再次启动现场二进制。

## remove-ai-slops / programming 直接 pass

已读取两个 SKILL.md、programming Go README、logging、Go concurrency/testing 参考。按只读门审要求审查，不执行清理/重构或新增代理。用户明确文件保留 DEBUG、错误脱敏的要求覆盖技能的一般生产日志默认建议。

| 检查 | 直接结论 |
|---|---|
| excessive/useless tests | 新用例覆盖不同用户行为和失败类：关联、脱敏、时间边界、启动/定时/关闭、轮转、并发、文件/console故障与真实链路。最终三文件测试增量小，Claim failure测试使用错误传播和实际日志输出，不仅检查调用次数。 |
| deletion-only / merely requested removal | 未发现。retention 的“旧内容消失”与边界/新内容/坏行保留同时检查，属于用户保留策略行为，不是仅测试源码删除。 |
| tautological / false confidence | 最终 Clone 测试能区分生产 Clone 有无，已独立 mutation FAIL。其他 expected 来自输入 fixture 和目标 outcome，没有从受测输出反推同一输出。 |
| implementation-mirroring | 日志 stage/operation、安全字段和级别属于这次输出行为。`retention_concurrency_test.go:47` 的 writer identity 辅助断言约束当前资源复用策略；这是低级别维护 NOTE，主断言仍检查400条记录完整、不丢失。它不证明任一成功标准失败。 |
| unnecessary production extraction/parsing/normalization | 未发现请求内容、SQL、payload 的新提取。duration配置解析、backup filename识别、逐行timestamp解析有明确边界用途。`retention.go` cleanup后的 Stat 是恢复 rotation size计数所需，不是删后再确认的冗余查询。 |
| speculative abstraction / dead code / scope drift | context handler、dump handler、tracer各有实际多调用 seam；trace结构服务 token与subscribe两处；继续使用slog/lumberjack/pgx，未增未来provider或插件抽象。 |
| typed boundaries/errors | 没有新增domain any逃逸；slog []any/测试JSON map为adapter形状。错误采用errors.Is/As及安全类别，日志不调用raw error内容。配置与文件边界检查保留。 |
| resources/concurrency | Sink锁覆盖写入/清理/Close，sync.Once关闭并join ticker；temporary文件失败清理；context传至DB与HTTP，WithoutCancel保留完成结果落库上下文。检查已有race/lifecycle收据，无新生命周期缺口。 |
| logging | composition root注入，固定消息+字段；Claim/Finish失败WARN且带attempt ID，前轮层级/关联NOTE已关闭。多个生命周期记录是用户明确要求，不按技能默认一条错误原则删除目标诊断。 |
| size / maintenance | 独立计数：既有main397、access350、appapi/server321 pure LOC仍超250；新observability实现最大185；worker200、core新集成测试243、wechat/sender222。既有尺寸债务是NOTE，不将无关整仓重构列为此日志候选阻断。 |

未配置/未要求新的独立security scanner、nilaway或golangci套件，本审不声称这些工具已通过。仓库实际build/vet/race与契约收据按下节记录。

## Code review 覆盖对账

本审已读前轮 `.omo/evidence/debug-dump-4bdc2d5-code-review.md` 及其引用产物，确认两个技能与overfit/slop各类显式覆盖；它不替代最终tree审查。

已亲读最终 `.omo/evidence/debug-dump-2bd5fd8-code-review.md`：明确 APPROVE 同 tree `2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b`，WATCH，blockers为空；审阅者未参与编辑。报告显式覆盖 remove-ai-slops / programming、过量/无用测试、deletion-only、自证/implementation-mirroring、production提取/解析/归一化、typing、错误、并发资源、日志与尺寸；前三项修订都明确关闭。

该报告的 final-delta、review-fixes、clone-mutation、chain/live、全库与契约引用产物，本审均已实际检查；另diff其引用的 `/tmp/iolink-review-clone-mutant-_vugvyhn/context.go`，也仅移除生产Clone。没有用其结论替代本审直接pass。关于辅助writer identity断言，本审保留低级别实现绑定NOTE，代码审阅者没有判为遗留缺陷；两者均没有证据支持成功标准失败，结论一致。现有两位未参与编辑者明确批准相同最终tree，S2a状态归档仍由任务主代理处理。

## 已核实的全库失败

`candidate-verify.log` 实际为 build/vet PASS，race整体FAIL，仅 `internal/gbcatalog` 的11项测试失败；其余包PASS。前轮独立门审复现输出 `.omo/evidence/debug-dump-4bdc2d5-gate-artifacts/gbcatalog-baseline-failure.log` 已读取。

本审重新核对 BASE→最终TREE 的该目录diff为0且目录tree相同；直接查看 `catalog_test.go:35` 的真实time.Now deadline检查、`:88` 固定2026-10-11 fake clock及 `validation.go:15` 的WithDeadline，当前日期下过期原因成立。没有删除、跳过或弱化这些用例。任务是本轮debug dump，没有要求同时修复全部既有目录测试；该缺陷不构成此次回归或范围失败，故非阻断。**不能称make verify全库绿、不能称该缺陷已修复。**

## checked artifact paths

- 权威和上下文：用户提供AGENTS.md；`docs/README.md`、`docs/CONTRIBUTING.md`，相关 `docs/PLAN.md`、`PLAN-DETAILS.md`、`EXTENSIONS.md`、`ACCEPTANCE.md`要求；`.omo/evidence/zcode-session-context-2026-10-13.md`。没有 `.omo/notepads`，没有另行提供notepad路径；上下文收据作替代输入，未虚构notepad。
- 最终全部43个changed paths的diff与冻结源码：Dockerfile、cmd/iolinkd/main.go、两份deploy compose、docs/DEPLOY.md、docs/IMPLEMENTED.md；internal/access/{access.go,broker.go,broker_debug_test.go}；internal/appapi/{server.go,wechat.go,wechat_debug_test.go}；internal/core/{debug_dump_integration_test.go,generic_ingestion.go,license_runtime.go,pipeline.go,service.go,service_debug_test.go}；internal/event/event.go；internal/notifications/{worker.go,worker_debug_test.go}；internal/observability/{backups.go,config.go,context.go,context_test.go,database.go,dump_handler.go,dump_handler_test.go,logging.go,logging_test.go,redaction.go,retention.go,retention_concurrency_test.go,retention_lifecycle_test.go,retention_test.go,sink_test.go}；internal/operations/{request_logging.go,request_logging_test.go}；internal/platform/platform.go；internal/setupapi/server.go；internal/wechat/{sender.go,tracing.go,tracing_test.go}。另查 `.github/workflows/ci.yml` 与未变更gbcatalog源码。
- Executor目录 `docs/evidence/deployment/2026-10-13-debug-dump/`：README.md（真实QA矩阵）、candidate.patch、source-tree.txt、final-source-tree.txt、candidate-verify.log、diagnostics-race.log、chain-race.log、contracts.log、docs-tools.log、live-http.txt、live-check.json、live-dump.jsonl、live-stderr.log、review-fixes-race.log、clone-mutation-fails.log、final-delta-race.log；chain目录中的HTTP License及MQTT success/duplicate/database_failure四份JSONL。
- 前轮独立收据：`.omo/evidence/debug-dump-4bdc2d5-{code-review,gate-review}.md`；`debug-dump-4bdc2d5-gate-artifacts/gbcatalog-baseline-failure.log`。
- 最终独立代码收据：`.omo/evidence/debug-dump-2bd5fd8-code-review.md`；其引用的 `/tmp/iolink-review-clone-mutant-_vugvyhn/context.go` 已与最终源码diff。
- Mutation实际输入：`/tmp/iolink-debugdump-clone-overlay.json`、`/tmp/iolink-debugdump-clone-mutant.go`；共享context源码/测试的blob已与最终tree核对一致。

## exact evidence gaps

- 没有真实微信服务、硬件、License交付或浏览器的本轮证据；任务明确延后/由用户验收，不从本地HTTP或合成值推断外部通过。仍为external_blocked/未验收。
- 没有全库全绿输出；保留完整失败与前轮独立复现。最终只复跑delta相关验证，不声称重跑全库。
- 早期diagnostics的6个setup SKIP不是PASS；真实DSN全库/前轮独立setup收据补足相关验证。最终delta日志无SKIP。
- 最终source-tree.txt仅292条筛选路径；本审自行校验完整1047 blobs，缺失0、不一致0。
- `candidate.patch` 是前候选的41个源文件路径差异，逐字匹配 `git diff BASE 4bdc2d5 -- Dockerfile cmd deploy internal`；没有两项docs变更和最终三文件delta。它不是最终完整diff。本审直接从Git对象读取最终全部43路径差异，已补足范围核对。
- 没有独立notepad文件；原意、目标、范围和QA证据已经可读，不以行政路径缺失拒绝。
- 没有多进程同根目录写入、容量benchmark或远端tag CI结果；文档要求单常驻进程，本轮候选审核不包含发布后证明。
- 本报告初稿时最终代码审阅文件尚未落盘；现已读取并完成同tree、技能及引用产物覆盖对账，不再有此证据缺口。

本次结论：**APPROVE 最终固定 tree 2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b**。blockers为空；低级别维护NOTE及外部/全库限制如上。
