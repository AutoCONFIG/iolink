# 既有扩展 HTTP 与共享认证：独立代码审阅

日期：2026-10-14。审阅者：`/root/latest_code_review`，未参与本次实现、测试或契约编辑。

## 结论与精确范围

- `codeQualityStatus`: **WATCH**
- `recommendation`: **APPROVE**
- `blockers`: **[]**
- 审阅候选 tree：`e88a723196d503455a6b9d3c0d4d58dd085248eb`
- 基线：`d4d7e02`（v0.0.37）
- 独立执行路径：`/tmp/iolink-extended-http-20261014`

APPROVE 仅针对该候选内的既有平台/组织/成员 11 操作、App 身份/v2 5 操作、产品/模型 7 操作，以及共享认证故障分类、两个显式空数值过滤值、组织名 Unicode 长度修复。整个 96 操作的真实矩阵、R02 和外部验收不因本报告完成。

已逐项核对候选相对基线的生产代码、契约、测试及过程证据。独立按 Git blob 算法核验导出，**1,383 个 blob 全部一致，无缺失或摘要差异**；`web` 是复用的 gitlink，未把构建生成文件当成受审生产改动。工作树中的 AGENTS、playback、后续 setup/APIKey/Open 草稿及验证器未入选修改均不属于本次审阅。没有修改实现、测试、导出或索引；只写此报告。

`ulw-loop status --json` 返回 `ULW_LOOP_PLAN_MISSING`，故使用 `.omo/evidence/extended-http-code-review.md` 回退位置。任务未另给 notepad；使用候选 README、基线差异和实际运行结果记录范围。

## 分级发现

### CRITICAL

无。

### HIGH

无。

### MEDIUM

未发现本次增量引入的无用测试、无需求的生产解析、权限绕过、部分事务写入或实质契约偏差。

### LOW

1. **既有大文件仍有维护负担，非本批引入的模块膨胀。** `internal/adminapi/handlers.go:1`（1,019 pure LOC）、`internal/appapi/server.go:1`（344）、`internal/core/admin_store.go:1`（1,217）及 `internal/adminapi/server_test.go:1`（872）超过技能文件尺寸建议。它们在基线已超限；本次新增认证和输入边界放入独立小文件，新文件均不超过 200 pure LOC。`auth.go` 242、`products.go` 240、`tenant_admin.go` 224 处于警戒区。该观察使质量状态为 WATCH，不构成本次限定修复的阻断；后续实质扩展这些大文件时应按责任拆分，不能把本报告当作其架构完成证明。

## 正确性、安全与业务检查

- **认证故障与失效身份分离。** core 将查询 `pgx.ErrNoRows` 转成 `domain.ErrNotFound` 或 `domain.ErrInactiveTenant`，真实 SQL/连接错误仍原样通过安全错误分类器。admin/app 新 helper 对失效身份保持 401，对真正 provider 故障返回 500，并 `Abort` 中止后续 handler。版本不匹配、失效 membership 和角色不匹配仍拒绝，不借错误分类放宽权限。
- **摄像头错误形状保留。** 两侧 auth helper 根据 `camera_route` 使用既有固定 `code/message` 对象，普通接口使用固定 `error` 对象。未知 provider 错误落入 `cameraError` 的 500/internal_error；真实缺失能力不制造错误对象，也不输出底层字符串。
- **平台与业务角色保持隔离。** ADMIN 创建/禁用组织、查看平台目录/统计需平台身份；普通用户通过成员和农场授权访问自己的资源。跨租户、只读写入、旧 JWT 撤销和立即取消成员授权均有真实边界测试。注册仅创建 USER，未给 ADMIN 或新 USER 自动创建业务租户/成员。
- **输入在 HTTP 边界完成解析。** 新成员 DTO 区分显式 false 与遗漏/null；允许 expires_at 的契约空值。产品字段检查精确键、重复、必填和类型，具体模型语义仍由已有 domain/core 处理。App tenant switch 和 v2 telemetry 拒绝重复、未知、尾随及错误类型，原始 telemetry 值只作为既有模型校验的输入，不新增 provider SDK 依赖或万能接口。
- **空值与 Unicode 语义。** 使用参数是否出现区分省略与显式空值；既有 ParseQuery guard 仍先拒绝坏编码。组织名称按 Unicode 字符计数，128 个合法字符可创建，129 被拒绝；UTF-8 和保留名称检查保留。
- **事务/状态完整性。** 实际审计写入故障覆盖组织创建、状态和成员变更；事务回滚同时恢复业务表与审计。设备分配的设备版本、shadow 删除、audit 同事务，触发器注入 audit 失败后整体不变。App 在 telemetry/shadow 写入之后的 alarm_rules 故障也回滚。没有把这些注入案例概括成所有故障时序或未知提交结果已经验收。
- **日志安全。** 使用注入 logger 和 ErrorContext；新增消息、stage 恒定，仅记录安全错误类别，不记录原始 body/header/query/URL、凭据、SQL 或设备明文。产品存档 dump 的 27 条记录只有安全字段，其中 7 条 http_provider 错误分别关联生成的请求 ID，类别均为 `postgres_42P01`。能力缺失 helper 无编造 err。
- **契约增量。** 新状态声明对应实际复现的输入、权限、provider/能力路径；没有放宽既定成功 shape、角色条件或 common error enum。

## 独立运行的验证

全部在精确导出运行，真实 PG 使用本批专属 loopback 33127 DSN，测试自行建立/清理独立数据库；contracts Python 使用主仓已准备的绝对虚拟环境路径。

1. 真实 PG/TCP、race、随机顺序抽样：

   ```bash
   GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated-33127-DSN> \
   IOLINK_CONTRACT_PYTHON=<main-repo>/.venv/contracts/bin/python \
   go test -race -count=1 -shuffle=on -v ./internal/core \
     -run '^(TestIdentityStorageFailuresAreNotReportedAsRevokedCredentials|TestBaselineNumericFiltersRejectExplicitEmptyValues|TestTenantNameLimitCountsUnicodeCharacters|TestPlatformOrganizationDatabaseFailuresAreInternalAndAtomic|TestProductContractHTTPAssignmentAuditFailureRollsBack|TestProductContractHTTPCrossTenantAndMissingTargetsLeaveStateUnchanged|TestExtendedAppContractsRejectExpiredMembershipAndProviderFailures)$'
   ```

   **终端 exit 0**（独立 exec session `75503`，shuffle seed `1791915247287870672`），七个顶层目标实际运行。覆盖 admin/app 六个身份表故障，两个空过滤，Unicode128/129，平台八个真实关系/审计失败，分配 audit 触发器失败，跨租户/不存在/冲突目标，以及 App 五个失效 membership、六个真实 provider 故障。看到真实 TimescaleDB `2.30.2` 环境记录；这些选择目标没有被 SKIP 替代。

2. `go vet ./internal/adminapi ./internal/appapi ./internal/core`：**exit 0**，没有诊断。

3. 输入和摄像头边界 `go test -race -count=1 -shuffle=on -v ./internal/adminapi ./internal/appapi`，选择以下六个函数：

   - `TestProductFieldInputRejectsAmbiguousOrIncompleteFields`
   - `TestProductFieldInputPreservesExplicitFalseAndNullableBounds`
   - `TestExtendedRequestParsingEnforcesExactTypedFields`
   - `TestTelemetryInputRejectsInvalidPropertiesBeforeTheRepository`
   - `TestCameraHTTPAuthKeepsSurfacesSeparate`
   - `TestCameraHTTPErrorsAreSafeContractObjects`

   **终端 exit 0**（session `12596`），两包均 PASS。该命令重放既有摄像头鉴权/错误形状，并非视频运行时验收。

4. `<main-repo>/.venv/contracts/bin/python scripts/check_contracts.py`：**exit 0**，**96 target operations / 603 synthetic request/response fixtures PASS**。这只是静态标准/合成样例检查，不替代真实 HTTP。

## 执行者最终证据核读

证据根目录：`docs/evidence/deployment/2026-10-14-extended-http/`。

- `README.md` 明确 worker 日志来自共享工作树，只作 RED/GREEN 过程，不冒称最终导出证明；也明确本批范围和外部限制。
- 主仓 `snapshot-verify.log` 已终端结束；逐项读取为 33 个测试包，无 `FAIL` 或 `DATA RACE`。root 已确认 exact export 的 `make verify` session `15762` exit 0，包含 build/vet/race。
- 主仓 `snapshot-focused.log` 最后一行为 core `230.035s`、PASS；root 确认 session `20878` exit 0，21 个顶层目标、无 SKIP。没有把未设置 DSN 的测试跳过算通过。
- root 通知 Python session `66278` 的 110 tests 及静态 96/603 退出 0；本报告的静态结果另有独立重放，Python110计作执行者证据而非审阅者自行重放。
- `products/verified-provider-dump.jsonl` 实际解析 27 条 JSONL，字段仅 time/level/msg、route/method/status、duration、tenant/actor、request_id、stage/outcome/err；七个 provider 记录均为安全 SQLSTATE 类别。对应 dump-summary 计数一致。

## programming / remove-ai-slops 视角

**两技能检查已运行。** 已加载/咨询 `programming` 与 `remove-ai-slops` 及 Go 的错误、测试、日志指导；按本次完整增量作生产代码与测试的 overfit/slop 检查。

- 无删除代码即通过、仅断言某字符串移除、提示词/散文钉死或同实现常量自证的测试。固定 error code 属于外部 machine contract，测试通过真实 handler 与独立 OpenAPI enum 校验，可因错误分类或 payload shape 回归而失败。
- 真实 TCP、真实 PostgreSQL 故障、发布/分配后读取、跨租户授权和多表状态摘要提供独立可失败的 oracle；状态检查用于证明被拒绝/失败请求无写入，并非生产 setter 后无需求的再查询。
- 已有测试 fixture 补齐 active/unit/writable/nullable，确保继续进入原先要测试的 provider/revocation 分支；未删除失败断言或减少拒绝范围。fake 的未知 ID 改为真实 domain.NotFound，与 core 语义一致。
- 新解析与 JSON RawMessage 暂存只在不可信 HTTP 边界使用，目的是消除重复/大小写/尾随/必填歧义；没有无需求的数据抓取、全局归一化、数据库再解析或未来 provider 抽象。成员、产品和 telemetry 边界用途不同，保留各自小 helper 比强行共享通用框架更直接。
- 没有新增 broad exception、静默 fallback、未使用 helper、全局 logger 或生产日志中的原始诊断文本。新 helper 的 null/能力检查位于实际可缺少的依赖边界，非静态已保证值的重复防御。
- 文件尺寸视角存在上述**基线遗留**违反建议的模块；未发现本次新增有实质维护/正确性代价的 slop。已明确记录为 LOW/WATCH，不把遗留结构称作已治理。

## 未承诺的验收

APIKey/Open/restore/setup/camera 后续真实矩阵、全部 R02 子项、浏览器交互、真实微信/设备、视频媒体与 playback 运行时、License 交付专项、长期容量及大规模恢复仍由其各自证据/清单推进。共享 auth 改动的代码检查和既有摄像头边界重放不代表这些外部面完成。

**同一候选 e88a723 的限定代码审阅明确 APPROVE；仍需另一位未参与编辑的审阅者针对该候选明确通过，才能闭环此里程碑。**
