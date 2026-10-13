# Debug dump 固定候选独立门审

recommendation: **APPROVE**

blockers: []

日期：2026-10-13，Asia/Shanghai。审阅角色：第二位未参与源码编辑的独立只读门审。只新增本报告和验证输出，不修改候选源码。

候选 Git tree：`4bdc2d5a8fc688c13a309f64611c73691e054483`；base：`ec1fb049f2316a2e5ec8c3e8509d797adf002334`。源码导出：`/tmp/iolink-debugdump-review-4bdc2d5`。本结论仅覆盖这个 tree；不自动批准后续修订、原工作树混合草稿、发布后的状态更新或其他里程碑。

`omo-agent-toolkit ulw-loop status --json` 实际返回 `ULW_LOOP_PLAN_MISSING`，没有 currentAttemptDir，所以按要求采用 `.omo/evidence/debug-dump-4bdc2d5-gate-review.md` 回退路径。

## originalIntent

用户原文由任务主代理补充：

> 此外debug dump部分，要覆盖多项记录，这样下游的请求可以转储到这个日志，内部的处理逻辑等细节也可以记录下来，后续可以方便的定位问题。

> 此外其他部分也要落到debug dump里面而且要有时间轮回清理机制（比如6小时，那么就6小时清理一次截至当前6小时之前的数据），此外要有版本号的文件夹划分，例如/debugdump-root/{version}(例如v0.0.3）/这个版本内的日志。

本轮只验证并增强已有功能诊断。要求 HTTP、MQTT、数据库、License 准入、通知和微信下游可定位问题；版本目录和时间清理须实际可用。仓库 AGENTS 的日志脱敏、composition root 注入、CLI stderr、失败可观察及数据完整性约束继续生效。浏览器由用户验收；真实微信凭据/硬件和 License 交付专项延后；CI 仅 tag。用户没有要求持久保存从 MQTT 到异步 outbox 的上游关联 ID。

## desiredOutcome

运维人员能在 `{root}/{version}/iolinkd.jsonl` 获取多条脱敏诊断：HTTP 请求内关联业务及数据库、单次 MQTT 发布内关联验证及事务、单次通知尝试内关联微信调用和结果落库。文件始终保留 debug 及以上，stderr 独立按配置过滤。服务启动及之后默认每 6 小时清除严格早于清理时刻 6 小时的记录，包括旧版本和轮转文件。转储或清理失败应可观察，readiness 降级，不把业务失败写成成功，不泄漏原始请求、SQL、凭据、payload 或明文设备标识。

## 成功标准与 userOutcomeReview

以下 C 编号是为本次门审给任务中的明确要求建立的引用标签，不扩充需求。

| criterion | 任务中的要求 | 审核结果与直接证据 |
|---|---|---|
| C1 | 多阶段/下游脱敏记录能定位现有功能的问题 | PASS。独立复跑真实 TCP MQTT→core→PG 成功/重复/数据库失败及 HTTP→License 拒绝；`gate-artifacts/focused-race.log` 中 `TestDebugDump*`。亲读 `chain/*.jsonl`，同 ID 的 ingress、validation、database start/terminal、committed/duplicate/rejected 均可连接。HTTP 登录实际 Routes→本地微信 HTTP 的成功/拒绝，以及 worker→本地微信→Finish 的成功/拒绝/落库失败复跑通过。 |
| C2 | 根目录按版本划分，dump 和 console 级别独立 | PASS。`observability.New` 把经过路径校验的 Version 加入 root；New 使用 debug dump handler 和独立 console handler；独立 `TestDumpRetainsDebugAndConsoleUsesConfiguredLevelWithGroupedAttrs` 通过。实际服务产物 `live-dump.jsonl` 含 DEBUG，`live-stderr.log` 的 slog JSON 无 DEBUG，`live-http.txt` 中两个入口都 200。Docker final ARG→ENV 与现有 tag CI build arg 连通。 |
| C3 | 默认每 6h 清理严格早于当前 6h 的记录 | PASS。默认两项 duration 都是 6h；启动清理和 ticker 已装配。独立测试覆盖逐条 timestamp、纳秒边界、误导 mtime、当前/旧版本、backup、空旧 dump、启动及 timer/Close。`retention.go` 保留相等时间与无有效 timestamp 的行；只处理认可名称的一层版本 regular 文件。 |
| C4 | 转储/清理错误可观察、ready 降级；CLI 仅 stderr | PASS。Sink write/short write、console failure、cleanup failure 均设置 failed；stderr 使用固定失败消息；`cmd/iolinkd/main.go:337` 的 Ready 检查 Healthy。独立 `TestDumpFailureFallsBackToStderrEvenBelowConsoleLevel`、cleanup fail-closed、Close/write 并发、Health fail-closed 通过；code reviewer 的完整 observability/CLI 证据亦已读取。CLI 的非 serving 分支仅 Console；QueryTracer 没有打开另一个文件 sink。 |
| C5 | 不记录原始请求/SQL/凭据/完整 payload/明文设备 ID | PASS。直接核对完整 diff 和 safeAttr；新增日志消息常量，动态字段类型或来源受限；QueryTracer 忽略 StartData SQL/Args，错误仅 safe class/SQLSTATE。亲读 live/chain JSONL，真实注入 SQL 错误只得到 `postgres_P0001`。独立 HTTP/MQTT/微信 adversarial fixture 检查通过，客户端 ID/header/query/body、SQL 和测试密钥未泄漏；device 被 hash。 |
| C6 | 保持业务、API/MQTT 与原子性；不扩展到其他里程碑 | PASS。本次无契约、迁移、domain 规则或 web gitlink 变更；实际失败采集四张表都无部分记录，去重只有一份。基线与候选 diff-check 通过。contracts.log 的 96 operations/568 synthetic fixtures 实际 PASS，按静态契约限度解读。新状态行保持“正在进行”，没有把真实微信、硬件、License交付或 M7a/M8 宣称完成。 |
| C7 | 验证证据真实、跳过不算通过；同快照独立双审 | PASS 作为本次有限范围候选审核。两个审阅者均未参与编辑，第一份 code-review 明确 APPROVE 同 tree。本门审独立复跑相关 8 包、核验 JSONL 和基线失败，不把 make verify 整体写成通过。外部验证保持未验收。 |

整体 userOutcomeReview：固定候选满足用户要求的本轮诊断结果，可以实际按操作关联定位下游成功与失败，版本及清理机制可运行。下面的 NOTE 会改善诊断与测试质量，但没有证据证明本轮明确成功标准失败。因此不将它们升级为 blockers。

## 独立复核动作

环境：`go version go1.26.8 linux/amd64`。设置 `IOLINK_TEST_PG_DSN` 指向专属 disposable Timescale 的 `127.0.0.1:33047/iolink`；密码不写入报告。core/setup testdb helper 为各用例建删随机隔离数据库，没有使用业务数据。

从固定导出执行：

```text
go test -race -shuffle=on -count=1 -v ./internal/observability ./internal/operations ./internal/access ./internal/setupapi ./internal/core ./internal/appapi ./internal/wechat ./internal/notifications -run 'TestDebugDump|TestWechatLogin|TestWorkerCorrelatesAttempt|TestSenderTrace|Test.*Retention|Test.*Cleanup|Test.*Prun|Test.*Backup|Test.*Dump|Test.*Close|Test.*Context|TestPublishEmitsCorrelatedSafeRecords|TestRejectedPublishStillHasCorrelation|TestRequestLogsCorrelateWithoutUntrustedContent|TestDurationEnvironment|TestReopenedWriter|TestDerivedLogger'
```

exit 0，8 包 PASS，无 SKIP。regex 也选中了一些现有 Context 权限回归，实际通过。完整输出：`.omo/evidence/debug-dump-4bdc2d5-gate-artifacts/focused-race.log`。

直接使用 `git ls-tree -rz TREE` 和 `git hash-object` 验证导出全部 **1047 个 blob**，无缺失、无变化；gitlink 不作为文件 blob。这比只核对 executor 的 source-tree 文本更强。`git diff --check BASE TREE` exit 0；`git diff BASE TREE -- internal/gbcatalog` 长度为 0。机器收据：`gate-artifacts/identity.json`。

独立解析现场产物：`live-dump.jsonl` 531 条 JSON，`live-stderr.log` 10 条 slog JSON；setup 响应 `X-Request-ID=6FONWU26YRGTCKN4VSKQ2VYWPK` 在 dump 中连接 HTTP start、database start/terminal、HTTP status 200。`live-check.json` 的 linked_records 与实际 dump 逐项相同，不以其 PASS 文案代替检查。现场 stdout 的 Gin 非 JSON 启动文本按格式识别，没有当作 slog JSON。没有自行重复启动服务；该项依据已读取的实际 HTTP/文件输出，真实业务 HTTP/TCP 操作由上述测试独立重现。

## remove-ai-slops / programming 直接审查

已加载并咨询两个 SKILL.md，以及 programming 的 Go README 和 logging 参考；按只读门审规则做直接 pass，没有启动清理或新增代理。

| overfit/slop 类别 | 直接检查结果 |
|---|---|
| 过量或无用测试 | 主要新增用例分别针对时间、并发、故障、脱敏和真实链路，覆盖不同失败类，有必要。只有下述 Clone 断言没有实现它宣称的保护。 |
| deletion-only / 仅验证删除 | 未发现。retention 的内容缺失断言与 boundary/recent/malformed 内容保留一起验证用户要求的记录保留行为，并非测试代码删除。 |
| tautological / implementation-mirroring | `TestContextHandlerDoesNotMutateSharedRecord` 的 NumAttrs 断言无法区分是否 Clone；已独立用仅移除 Clone 的 overlay 重现仍 PASS。其余对 stage/operation/JSONL 的断言针对本轮机器输出契约；expected 来自 fixture 和目标 outcome，不从受测输出自推。 |
| 不必要生产提取/解析/归一化 | 未发现 payload、SQL、请求内容提取。JSON time 读取是逐条保留策略所需；backup filename 解析防止扫无关文件；duration 解析属于配置边界。 |
| 无需求抽象/死代码/重复 | tracing value 与 handler 为多个实际调用/派生 logger 提供 seam；沿用 slog 与 lumberjack，没有新依赖或未来 provider 抽象。若干保留/配置 helpers 是文件/资源边界职责，不要求将其压回单个巨大函数。 |
| typing/errors/resources/concurrency | ctx 延续至 DB 和 HTTP，外部副作用/事务边界未变；logging 字段 `[]any` 是 slog adapter 调用形状；safe errors 采用 errors.Is/As 和安全类别。Close sync.Once、shared sink lock、ticker join、temporary file error cleanup 已检查并 race 通过。 |
| logging/size | 通知 Finish failure 的 INFO 层级是 NOTE；始终文件 DEBUG 是用户要求，覆盖技能默认建议。main/access/server 已有 >250 pure LOC 债务仍在；新诊断生产模块均 <200，core 新测试243、sender222处在警戒带。没有借本轮窄范围要求整仓重构。 |

### NOTE N1：Clone 测试提供虚假信心

`internal/observability/context_test.go:43` 只检查按值传入的原 Record.NumAttrs；这不能观察副本追加对共享底层数组的污染。实际生产代码 `context.go:31` 仍正确调用 Clone，故没有运行故障证据，不违反本轮成功标准。

我先读取 code reviewer 的 overlay.json，并逐行 diff 确认 overlay 唯一变化是移除 `record = record.Clone()`，再独立执行：

```text
go test -race -count=1 -overlay /tmp/iolink-review-clone-mutant-_vugvyhn/overlay.json ./internal/observability -run '^TestContextHandlerDoesNotMutateSharedRecord$' -v
```

实际仍 PASS。完整输出：`gate-artifacts/clone-mutation.log`。建议用记录重用/副本污染可观察断言替换该测试；候选源码在复现期间没有被编辑。

### NOTE N2：通知 Claim SQL 无本次投递 ID

`internal/notifications/worker.go:117` 在真实 Claim 和 QueueDepth 后才于 `:126` 创建 ID。因此 Claim SQL 与后续成功领取、发送、微信、Finish 的关联范围不同；本次后半链路已通过。原文未要求跨异步上游关联或领取事务每条 SQL 必须同 ID，不能以此拒绝 C1。建议在领取前生成 attempt context，并记录领取失败。

### NOTE N3：通知 Finish failure 的 INFO 层级

`internal/notifications/worker.go:169` 的 correlated failure 在 warn console 下不可见；外层 warning 又缺本次 ID。文件 dump 仍保存失败详情，未伪报业务成功，因此不违反 C4。建议使用与恢复语义一致的 WarnContext。

## Code review 覆盖复核

已亲读 `.omo/evidence/debug-dump-4bdc2d5-code-review.md`，并实际查看其 referenced `focused-race.log`、`chain-race.log`、`clone-mutation.log` 和 chain JSONL。该报告明确审查两个技能、测试无用/overfit、删除验证、自证、production parsing/normalization、日志与尺寸；结论为 APPROVE、blockers 空。报告 coverage 与本门审直接 pass 相符；没有用其结论代替直接检查。

## 既有 make verify 失败的判断

`candidate-verify.log` 显示 build/vet PASS，race 只有 `internal/gbcatalog` 的 11 项失败，其他包 PASS。独立 `go test -race -count=1 -v ./internal/gbcatalog` 再次复现同 11 项失败：`catalog_test.go:88` 的 fake time 为 2026-10-11，而 fake store `:35` 对 deadline 使用真实 time.Now；`validation.go:15` 使用 context.WithDeadline，当前日期下已过期。完整输出 `gate-artifacts/gbcatalog-baseline-failure.log`。

该目录 BASE→TREE diff 为 0，本次不更改其时钟、数据库或目录协调器逻辑。用户要求是当前 debug dump 诊断发布，没有要求这轮修复所有既有目录测试。相关范围独立 race、实际 PG/TCP/HTTP、静态契约均有通过证据，故**此既有缺陷不阻断本次日志候选**。必须保留 `make verify` 整体失败记录，不能借门审宣称全库全绿或该缺陷已解决。

## checked artifact paths

- 权威/范围：`docs/README.md`、`docs/CONTRIBUTING.md`；相关 `docs/PLAN.md`、`docs/PLAN-DETAILS.md`、`docs/EXTENSIONS.md`、`docs/ACCEPTANCE.md` 条目；`docs/IMPLEMENTED.md`、`docs/DEPLOY.md`。
- 会话/notepad 替代输入：`.omo/evidence/zcode-session-context-2026-10-13.md`。`.omo/notepads` 不存在；没有现成独立 notepad，已从原文补充及完整源码/产物完成核对。
- 固定 diff：BASE→TREE 全部43路径，包括 Dockerfile、compose、cmd main、access、appapi、core、event、notifications、observability、operations、platform、setupapi、wechat 及相关测试；另看 `.github/workflows/ci.yml` 版本注入/trigger。
- Executor 证据目录：`docs/evidence/deployment/2026-10-13-debug-dump/README.md`、`source-tree.txt`、`candidate.patch`、`candidate-verify.log`、`diagnostics-race.log`、`chain-race.log`、`contracts.log`、`docs-tools.log`、`live-http.txt`、`live-check.json`、`live-dump.jsonl`、`live-stderr.log`、`chain/TestDebugDumpHTTPRegisterDeviceLicenseRejectionCorrelation.jsonl` 和 success/duplicate/database_failure 三份 MQTT chain JSONL。
- 独立第一审：`.omo/evidence/debug-dump-4bdc2d5-code-review.md`、`.omo/evidence/debug-dump-code-review-artifacts/{focused-race.log,chain-race.log,clone-mutation.log,chain/*.jsonl}`。
- 本审独立产物：`.omo/evidence/debug-dump-4bdc2d5-gate-artifacts/{identity.json,focused-race.log,gbcatalog-baseline-failure.log,clone-mutation.log}`。

## exact evidence gaps

- 没有真实微信服务、设备/硬件及实际 License交付证据；已由用户明确延后，不能从本地 HTTP 和临时测试值推断通过。浏览器由用户验收，本轮无 UI diff。
- 没有全库全绿的输出；有完整失败输出及独立复现，不缺失败记录，不声称通过。
- Executor 早期 diagnostics 中实际 **6 个** setup DB tests SKIP；README 当时写“两项”不准确。完整 DSN `candidate-verify.log` 及 code reviewer 的 setup 包真实 race PASS 补足验证；不把那些 SKIP 自身当作 PASS。
- source-tree.txt 是292条所选源码清单，不是全树1048条 entry；门审已直接校验全部1047 blobs，无导出缺失。建议明确清单筛选边界。
- 缺现成 notepad 文件，没有缺少本轮行为证明。没有提供多进程共同写同一 root 的容量或一致性证据，DEPLOY 明确单常驻进程；这不是本轮成功标准。
- Claim SQL 关联和 Clone test 的有效保护不足按上述 NOTE 如实记录；其他实测行为足以支持这次有限范围批准。

最终结论：**APPROVE 固定 tree 4bdc2d5a8fc688c13a309f64611c73691e054483**。无与本轮明确成功标准相绑定的阻断项。后续修订需对新 tree 审查；本结论不等于完成发布、远端 CI 或全部外部验收。
