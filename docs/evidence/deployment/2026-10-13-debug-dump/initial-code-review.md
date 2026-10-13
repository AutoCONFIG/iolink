# Debug dump candidate independent code review

结论：**APPROVE** 本次有限范围的 debug dump 候选；`codeQualityStatus: WATCH`，`recommendation: APPROVE`，`blockers: []`。未发现 CRITICAL/HIGH。本结论接受下述非阻断诊断/测试问题及已核实的既有测试失败，不把全库检查写成通过。

## 固定范围和输入

- Candidate tree：`4bdc2d5a8fc688c13a309f64611c73691e054483`。
- Base：`ec1fb049f2316a2e5ec8c3e8509d797adf002334`。
- 只读源码：`/tmp/iolink-debugdump-review-4bdc2d5`。已对全部 43 个改动路径执行 `git hash-object` 并与候选 blob 比较：无不一致。没有审查原工作树的 M7a 草稿；没有参与本次源码编辑，没有修改候选源码。
- Goal：既有 HTTP/MQTT/core/数据库、License 准入、通知/微信诊断可关联；debug 文件日志与 console 级别分开；版本目录；启动和默认每 6h 清理严格早于 6h 的记录；安全失败可观察；外部业务语义和 API/MQTT 保持。
- 已读取 `docs/README.md`、`docs/CONTRIBUTING.md`，查阅 R04/相关设计要求，审查本次完整差异和部署说明。`docs/IMPLEMENTED.md` 将 S2a 保持为正在进行，没有伪报完成。
- `omo-agent-toolkit ulw-loop status --json` 返回 `ULW_LOOP_PLAN_MISSING`，所以报告使用 `.omo/evidence/<goal>-code-review.md` 回退位置。任务没有提供 notepad 路径，`.omo/notepads` 不存在；该行政输入缺失未替代源码和证据检查。

## Findings

### CRITICAL

无。

### HIGH

无。

### MEDIUM

1. **记录克隆测试无法发现它命名的共享存储回归。** `internal/observability/context_test.go:43`，尤其 `:57`。`slog.Record` 按值传入 `Handle`；追加属性不会改变原值里的 slice 长度，所以只断言原值 `NumAttrs()==8` 并不能证明共享底层数组没有被改写。我用 Go `-overlay` 仅移除 `internal/observability/context.go:31` 的 `record = record.Clone()`，保持候选文件不变，然后运行 `TestContextHandlerDoesNotMutateSharedRecord`：仍 PASS。证据：`.omo/evidence/debug-dump-code-review-artifacts/clone-mutation.log`。该测试对所宣称的克隆安全性提供虚假信心，应删除或改成能观察副本属性污染/共享记录并发的行为断言。当前生产实现保留 Clone，其他并发日志用例通过，因此不是本次运行正确性阻断项。

2. **通知领取事务缺少投递关联 ID。** `internal/notifications/worker.go:117` 在调用 `Store.Claim`、随后查询队列深度和检查领取结果之后，才在 `:126` 建立本次 ID。因此通过共享 pgx tracer 输出的 Claim 查询无法与后续 sender/Finish 查询按 `request_id` 连接；Claim 失败路径也没有此次尝试的 ID。`internal/persistence/notifications.go:21` 的真实 Claim 含 Begin、多次 SQL 和 Commit，故不是假想的下游调用。现有 `worker_debug_test.go` 的 Claim fake 丢弃 context，只检查 Finish 的 ID，未覆盖这一缺口。可复现：给 Claim seam 记录 `observability.RequestID(ctx)`，用无上游 ID 的后台 context 调用 RunOnce；Claim 为空而 Finish 非空。可在 Claim 前创建关联 context，或在诊断文档中明确领取 SQL 的边界。成功领取后的 claim 结果、发送、微信和 Finish 已正确关联，业务领取/重试语义没有改变，故为非阻断诊断缺口。

### LOW

1. **通知结果落库失败在关联日志中使用 INFO。** `internal/notifications/worker.go:169`。console 为 warn/error 时，这条带 request_id 的失败详情不进入 console；外层 `:192` 的 warning 没有本次尝试 ID。debug 文件仍完整保留此记录，readiness/业务错误没有被伪报为成功，因此不阻断。建议失败记录用 warn/error 中与其恢复语义一致的级别，并保留关联 ID。

## Skills perspective check

**已执行。** 在判断维护性及测试相关性前完整读取 `omo:remove-ai-slops/SKILL.md` 和 `omo:programming/SKILL.md`，另读取 Go README、concurrency、testing 和 logging 参考。

- 两个视角均发现上述 M1：仅数属性的测试没有对它宣称的共享记录安全性提供可失败的保护。该项按 reviewer 规则记 MEDIUM，没有把虚假信心泛化成 HIGH。
- 本次其他新增测试观察实际 JSONL、权限、保留边界、失败 readiness、并发生命周期、真实本地 HTTP 和真实 PostgreSQL 事务结果。日志本身是本次产品行为，固定 stage/operation 和安全字段断言属于机器输出契约。未发现 prompt/prose pin、仅验证代码删除的测试、从实现常量推导期待值的自证测试。
- 记录时间 JSON 解析和文件名解析是保留策略所需的文件边界处理；配置 duration 解析、日志字段类型/枚举限制是相应边界校验。没有发现目标不需要的 production payload/SQL 提取、解析或规范化，没有新 domain `any` 逃逸。slog 参数的 `[]any` 是框架调用形状。
- `programming` 的日志级别视角发现 L1。文件常存 debug 是用户明确要求，覆盖 skill 对生产环境 debug 的通用默认建议。
- 既有尺寸债务仍在：`cmd/iolinkd/main.go` 396→397、`internal/access/access.go` 340→350、`internal/appapi/server.go` 321→321 pure LOC，超过 skills 的 250 门槛。记录为已有结构限制，本次窄小新增职责没有引入新大型模块；不要求把整个既有模块重构混入日志范围。新增 core 集成测试 243、sender 222 在警戒带；其余新实现模块在 200 以下。

## 独立验证与证据

环境：2026-10-13 Asia/Shanghai，`go version go1.26.8 linux/amd64`；专属 disposable Timescale 实例 `127.0.0.1:33047`，设置 `IOLINK_TEST_PG_DSN`。testdb helper 对各场景创建/删除自己的随机命名数据库，没有使用业务数据。

1. 独立执行 `GIN_MODE=release go test -race -shuffle=on -count=1 ./internal/observability ./internal/access ./internal/operations ./internal/wechat ./internal/appapi ./internal/notifications ./internal/platform ./internal/setupapi`，设置上述 DSN：8 包通过，exit 0。证据：`.omo/evidence/debug-dump-code-review-artifacts/focused-race.log`。该次补充了早期 diagnostics log 中因未设置 DSN 而 skipped 的 setup 集成，不把原 skipped 当作通过。
2. 独立执行 `GIN_MODE=release go test -race -shuffle=on -count=1 -v ./internal/core -run 'TestDebugDump|TestHandleEventValidationDump'`，设置上述 DSN 和独立 JSONL 证据目录：全部通过，exit 0，无 SKIP。真实 MQTT→core→PG success/duplicate/database_failure 及 HTTP→缺失 License verifier 拒绝通过；失败场景 sensor_data、telemetry、device_shadows、ingest_messages 均无部分写入。证据：`.omo/evidence/debug-dump-code-review-artifacts/chain-race.log` 及 `chain/*.jsonl`。
3. 亲自读取上述 JSONL，确认同一个 ingest/request ID 跨所需阶段，数据库 span 有 start/terminal，SQLSTATE 以 `postgres_P0001` 表示，禁止的原始值不在日志；测试同时验证原始 SQL、payload、设备名称、请求 query/body/header 等 fixture 不泄漏。
4. `git diff --check BASE TREE`：exit 0。Docker final stage 的 ARG→ENV 与现有 CI `IOLINK_VERSION=${{ github.ref_name }}` 连通；CI 仍由 tags 触发，没有工作流范围变化。
5. 已亲自读取父会话生成的 `docs/evidence/deployment/2026-10-13-debug-dump/{candidate-verify.log,contracts.log,diagnostics-race.log,chain-race.log}`，以实际输出为准。contracts：96 operations、568 synthetic fixtures PASS，仅代表静态契约检查。

## 基线失败与可接受限制

固定候选的 `make verify` **失败**，不能记为 PASS。日志显示 build/vet 通过，只有 `internal/gbcatalog` 的 11 项用例失败，其余包通过。`catalog_test.go:88` 使用固定 `2026-10-11` 时钟，fake Store 的 `:34` 又以真实 `time.Now()` 检查 context deadline，当前日期下必然过期。已执行 `git diff --exit-code BASE TREE -- internal/gbcatalog`：无差异；基线和候选该目录 tree 均为 `698a2e25094c26063062133f40d8b31945cec820`。这不是本次回归；**同意它作为本次日志候选的已知限制，不要求合入无关草稿**。其修复/全库全绿仍需单独记录，不能由本次批准暗示已解决。

真实微信服务、设备、License 交付专项没有在本轮验证，仍是范围外/`external_blocked`；本地 HTTP provider 不替代这些验收。浏览器自验按用户指定执行，本次没有 UI 变更。没有声明 M7a/M8 或完整外部验收完成。

## 阻断项与残余风险

`blockers: []`。本次源码使用同一 Sink 锁协调写入、保留、备份上限和 Close，并通过 sync.Once 加入关闭和等待；损坏超长行在不部分重写的情况下安全失败，原始错误/路径不进诊断。保留只处理一层版本目录下认可名称的 regular 文件，符号链接和无关文件不处理。仍需遵守文档明确的单日志根目录单常驻进程约束；未验证多个进程共同写同一目录。同步清理持有写入锁，部署目录累积较大时存在暂停日志写入的成本，未做容量 benchmark，不据此声称容量关卡通过。

报告路径：`.omo/evidence/debug-dump-4bdc2d5-code-review.md`。
