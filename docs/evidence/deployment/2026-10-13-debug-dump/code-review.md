# Debug dump 最终候选独立代码审查

```text
codeQualityStatus: WATCH
recommendation: APPROVE
reportPath: .omo/evidence/debug-dump-2bd5fd8-code-review.md
blockers: []
```

**明确批准最终 Git tree `2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b` 的本次 debug dump 变更。** 无 CRITICAL/HIGH 阻断项。WATCH 表示下文已核实的既有全库失败与结构债务；不表示本次目标未达成，也不把全库检查写成通过。

日期：2026-10-13，Asia/Shanghai。角色：未参与源码编辑的独立只读代码审阅者 `/root/dump_final_code_review`。只新增本报告；没有修改、回滚候选源码，没有派生子代理。此结论不自动覆盖后续修订、原工作树中的 M7a 草稿、发布收据或状态更新。

## 目标、标准与固定输入

- Base：`ec1fb049f2316a2e5ec8c3e8509d797adf002334`；前次候选：`4bdc2d5a8fc688c13a309f64611c73691e054483`。
- 固定源码：`/tmp/iolink-debugdump-review-2bd5fd8`。以 `git ls-tree -r -z --full-tree TREE` 读取对象清单，独立计算导出文件的 Git blob SHA-1（符号链接使用链接文本），验证 **1047 个 blob 全部一致，无缺失、无差异**。web gitlink 为 `39299dfb0d7771690117a2765fb766a9b715f554`。
- Goal：现有 HTTP/MQTT/core/数据库、License 准入、通知与微信下游有多条安全关联诊断；debug 文件与 console 级别独立；`logs/{version}`；启动及默认每 6h 按记录时间清除严格早于 6h 的日志；失败可观察；业务/API/MQTT 与事务语义不变。
- 成功标准：关联贯穿本次操作；不记录原始请求、URL、SQL/参数、密钥、完整 payload 或明文设备标识；文件/清理失败 fail closed 或降低 readiness；保留相等边界、未知时间记录并避开 symlink/无关文件；真实链路、错误路径和同快照审查证据可信。
- 权威资料：已先读 `docs/README.md`、`docs/CONTRIBUTING.md`，查阅 PLAN/设计附件及 ACCEPTANCE 的 R04/R55、DEPLOY 和 IMPLEMENTED 的相关条目。S2a 在候选中仍为“正在进行”，没有提前宣称阶段或外部验收完成。
- 已亲读前次 `.omo/evidence/debug-dump-4bdc2d5-{code,gate}-review.md`、部署证据 README，并检查引用的代码及产物，没有把前审结论当作本审证明。
- `omo-agent-toolkit ulw-loop status --json` 实际返回 `ULW_LOOP_PLAN_MISSING`，无 `currentAttemptDir`，故使用约定回退报告路径。任务未提供现成 notepad，`.omo/notepads` 不存在；已读取 `.omo/evidence/zcode-session-context-2026-10-13.md` 作为背景输入，目标仍以任务原需求为准。

## 完整差异与最终 delta

独立查看 BASE→TREE 的全部 43 个改动路径（2373 additions / 130 deletions），包括 Dockerfile、main、两份 Compose、DEPLOY/IMPLEMENTED；access/broker、appapi/server/wechat、core/service/pipeline/generic_ingestion/license_runtime、event、notifications/worker；observability 的 config/context/database/dump_handler/logging/redaction/retention/backups；operations、platform、setupapi、wechat/sender/tracing 和相应测试。没有契约、migration、domain 业务规则或 web gitlink 的变化。`git diff --check BASE TREE` exit 0。

前候选→最终候选逐行核对，确实只有：

1. `internal/notifications/worker.go`：`:117` 在 Claim 前生成投递 ID，同一 context 进入 Claim、QueueDepth、Send，WithoutCancel 保留 ID 至 Finish；`:124` Claim error 用 WARN，`:174` Finish error 用 WARN。返回值、lease、重试、未知结果、Finish 时序没有改变。真实 `internal/persistence/notifications.go:21` 的 Begin/Exec/QueryRow/Commit 都接收同一 ctx；QueryTracer 忽略 SQL/Args，只输出安全错误类别。
2. `internal/notifications/worker_debug_test.go`：Claim fake 记录 context ID，与 Finish ID 比较；provider success/rejection、Finish failure 都检查同 ID 与敏感 fixture 缺失；`:123` 的 Claim failure 用 warn console，检查没有执行 Finish、错误身份与返回语义保持、安全 WARN 可见。
3. `internal/observability/context_test.go:43`：调用方重用原 record 后实际读取第二条 JSONL，检查 outcome 保留且无 request_id/`!BUG` 污染；替代只数原 record 属性的无效断言。生产 `context.go:31` 仍保留 Clone。

三个前审问题均已关闭，不再保留前树的 Clone 覆盖缺口、Claim 关联缺口或 Finish 失败 INFO 问题。

## Findings by severity

### CRITICAL

无。

### HIGH

无。

### MEDIUM

无本次变更遗留项。没有发现无用测试或目标不需要的生产复杂性。

### LOW

既有结构债务：`cmd/iolinkd/main.go:1` pure LOC 396→397，`internal/access/access.go:1` 340→350，`internal/appapi/server.go:1` 321→321，仍超过两个技能的 250 行视角。本次新增日志模块没有形成新的超限文件；这些既有模块的全面拆分应单列，按 programming 的现有非严格项目规则，不将无关结构重构混入本次日志范围。新 core 集成测试 243、wechat sender 222 位于技能警戒带。此项没有证明本次正确性或可维护性失效，不阻断本轮。

## remove-ai-slops / programming perspective check

**已运行。** 在判断维护性和测试相关性之前，完整加载两个 `SKILL.md`，另咨询 programming 的 Go README、testing 与 logging 参考。按只读角色应用审查标准，没有执行修改型清理流程。

| 检查项 | 本审直接结论 |
|---|---|
| deletion-only / 仅验证请求删除 | 未发现。retention 测试同时验证旧记录移除与边界、新记录、损坏行及无关文件保留，是用户要求的实际文件行为。 |
| tautological / implementation-mirroring | 新 Clone 用例现在能区分移除 Clone 的回归，已独立 mutation 验证。Claim ID 与日志 ID 从各 seam 的可观察 context 获取，并非从输出计算期待输出。其他测试断言机器 JSONL 与 fixture 结果。 |
| prompt/prose pin / 脆弱文本测试 | 未发现。消息与 stage/operation/outcome 是本轮日志产物；安全 fixture 缺失断言针对泄漏风险，没有固定自然语言说明文案的测试。 |
| needless extraction/parsing/normalization | 未发现。逐行 JSON timestamp 解析属于保留文件边界；backup 名称解析限定可清理文件；duration 解析属于配置边界。没有新 SQL、payload 或请求内容提取。 |
| typing / abstraction / errors | 没有新 domain any 逃逸；`[]any` 是 slog adapter 形状。trace/context handler 支持实际多个调用点及派生 logger；没有未来 provider/插件抽象、新依赖或无用途包装。Claim/Finish 返回原错误；安全分类使用 errors.Is/As。 |
| concurrency/resources | Sink 锁协调写入、清理、备份上限与 Close；sync.Once 防重复关闭并等待清理 goroutine；临时文件错误清理保留原文件。synctest 的 time.Sleep 是受控虚拟时间，非真实等待。 |
| logging / skill violations | 项目注入 slog 与 safeAttr 规则优先；始终文件 DEBUG 是用户要求，覆盖技能默认生产级别建议。新增错误日志是稳定消息、安全字段、同 ID。上方 LOW 明确记录既有尺寸违规；除此没有发现本 diff 违反两个技能视角的遗留项。 |

## 独立执行的最终快照验证

环境：`go version go1.26.8 linux/amd64`。从最终导出执行：

```text
go test -race -shuffle=on -count=1 -v ./internal/observability ./internal/notifications -run 'TestContextHandler|TestWorkerCorrelatesAttempt|TestWorkerClaimFailure'
```

exit 0；`TestContextHandlerDoesNotMutateSharedRecord`、`TestWorkerClaimFailureIsCorrelatedAndSafe`，以及 worker sent/provider_rejected/persistence_failed 全部 PASS。observability 1.017s、notifications 1.030s。该次是 context/HTTP fake 层的窄回归，没有把它宣称为 PostgreSQL 验证。完整命令和必要输出在本报告保存：

```text
--- PASS: TestContextHandlerDoesNotMutateSharedRecord (0.00s)
--- PASS: TestWorkerClaimFailureIsCorrelatedAndSafe (0.00s)
--- PASS: TestWorkerCorrelatesAttemptAndDownstreamWithoutSensitiveData (0.01s)
    --- PASS: .../sent (0.00s)
    --- PASS: .../provider_rejected (0.00s)
    --- PASS: .../persistence_failed (0.00s)
PASS / exit 0
```

另独立读取 `/tmp/iolink-review-clone-mutant-_vugvyhn/context.go`，与最终 `context.go` 比较，确认唯一变化是移除 `record = record.Clone()`。以 shell process substitution 提供仅把最终 context.go 映射到该 mutant 的 Go overlay（未改候选/原 overlay），执行：

```text
go test -race -count=1 -overlay <(python3 - <<'PY'
import json
print(json.dumps({'Replace': {'/tmp/iolink-debugdump-review-2bd5fd8/internal/observability/context.go': '/tmp/iolink-review-clone-mutant-_vugvyhn/context.go'}}))
PY
) ./internal/observability -run '^TestContextHandlerDoesNotMutateSharedRecord$' -v
```

实际 exit 1，预期 FAIL，独立复现产物：

```text
context_test.go:68: caller record polluted by prior context:
{"level":"INFO","msg":"","count":0,"count":1,"count":2,"count":3,"count":4,"count":5,"count":6,"count":7,"!BUG":"redacted","outcome":"next"}
--- FAIL: TestContextHandlerDoesNotMutateSharedRecord (0.00s)
FAIL .../internal/observability 0.010s
```

证明该测试能发现它命名的共享存储回归；没有制造生产修复或弱化测试。

## 亲自核验的证据与结果边界

证据根目录：`docs/evidence/deployment/2026-10-13-debug-dump/`。

- `final-delta-race.log`：observability/notifications/wechat/persistence 四包全部 `ok`；51 个顶层 PASS，无 FAIL/SKIP。README 指向最终导出和专属 DSN；真实存储用例未跳过。`review-fixes-race.log` 两包 PASS，无 FAIL/SKIP。`clone-mutation-fails.log` 与本审独立 FAIL 一致。
- `chain-race.log`：core/appapi/wechat/notifications 四包 PASS，无 SKIP。已读真实链路测试，MQTT 用真实 TCP broker、PG 四表结果校验，覆盖成功、去重、触发器 SQL 失败的事务回滚；HTTP middleware 调用真实 core RegisterDevice 验证缺失 verifier 拒绝；登录另通过实际 Routes 调用本地 HTTP provider。未把测试路由 fixture 当作完整 OpenAPI handler 验收。
- 亲自解析 `chain/*.jsonl` 四份文件：21/75/117/83 条记录；所需 HTTP/License、MQTT ingress/core/dedup/persistence/数据库 start-terminal 能按对应 ID 连接，数据库失败仅 `postgres_P0001`，未出现 SQL/body/query/payload/凭据等原始字段。直接核对测试 fixture 的敏感内容缺失断言与生产 safeAttr。
- `live-http.txt` 两次实际服务请求 200；亲自解析 `live-dump.jsonl` 531 条与 `live-stderr.log` 10 条 slog JSON：file 有 521 条 DEBUG，console DEBUG 为 0；receipt 的 request ID 对应 4 条真实 HTTP/DB 记录，与 `live-check.json` 逐项相同。该 live 为前树未变装配的输出，不伪称本审再次部署服务。
- `contracts.log` 实际 96 operations / 568 synthetic fixtures PASS；已读 `docs-tools.log`。这仅是静态契约验证。CI 和 Docker 版本注入仍对应 tag，CI workflow 本次无改动。
- `diagnostics-race.log` 的 setup 数据库用例实际 **6 个 SKIP**，没有把它们当作 PASS；前树全库 DSN 输出及前审 setup 独立 DSN 测试提供补验。

## 既有全库失败、外部限制与残余风险

`candidate-verify.log` 实际 `make verify` **失败**：build/vet 通过，race 仅 `internal/gbcatalog` 的 11 项失败，其余包通过。直接读冻结源码 `catalog_test.go:88` 的固定 2026-10-11 时钟、`:35` 的真实 time.Now deadline 比较，以及 `validation.go:15` 的 context.WithDeadline，确认当前日期导致到期失败。独立 `git diff --exit-code BASE TREE -- internal/gbcatalog` 无差异。前审已独立复现相同缺陷；没有删除、跳过或降低这些测试，本审按范围不重复全库检查。此既有失败不阻断 debug dump 候选，仍不得宣称全库全绿或 gbcatalog 缺陷关闭。

真实微信服务、设备/硬件、License 实际交付没有本轮凭据与现场证据，属于用户延后的外部验收范围；本地 HTTP 与临时 fixture 不替代该验收。浏览器按用户安排自验，本次无 UI diff。单日志根目录单常驻进程约束已写 DEPLOY；未验证多进程共同写 root。清理持锁扫描会暂停日志写入，未做容量 benchmark，不据此称容量关卡通过。

**最终结论：APPROVE 最终 tree `2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b`；blockers 为 `[]`。**
