# 既有扩展 HTTP：独立最终门审

日期：2026-10-14。审阅者 `/root/latest_gate_review`，未参与本批实现、测试、契约或执行者证据编辑。本次只写此报告。

## 结论

- `recommendation`: **APPROVE**
- `codeQualityStatus`: **WATCH**
- `blockers`: **[]**
- 候选 tree：`e88a723196d503455a6b9d3c0d4d58dd085248eb`
- 基线：`d4d7e02`（v0.0.37）
- 冻结导出：`/tmp/iolink-extended-http-20261014`
- 另一位独立审阅者报告：`.omo/evidence/extended-http-code-review.md`，同一 tree 明确 APPROVE / WATCH、无阻断。

批准仅限平台/组织/成员 11 项、App 身份/v2 5 项、产品/模型 7 项的本次真实测试及修复，以及共享认证故障分类、两个显式空数值过滤值、组织名 Unicode 长度边界。全部 96 项真实矩阵、R02、V1d、外部验收及整个项目仍未由本报告完成。执行者须以本候选保存最终源码绑定、测试结果和双审记录后登记该小里程碑。

未发现 ulw-loop plan（状态为 `ULW_LOOP_PLAN_MISSING`），按规则使用本回退报告位置。未提供独立 notepad，依据权威文档、候选 README、差异、执行记录和实际重放核验。

## originalIntent / desiredOutcome / userOutcomeReview

用户要求现阶段优先测试和完善已有功能，平台管理员只管理平台，用户管理自己的物联网业务；不自动创建业务租户。每批须真实验证正反路径、记录安全诊断并由两位未参与编辑的审阅者针对同一快照通过，然后自主推进下一项。浏览器由用户测试，真实设备/微信和 License 交付专项不能以本地模拟替代。

本批期望用户看到：既有平台组织与成员操作可用且可撤权，用户切换组织、读取模型和提交遥测遵循当前契约，产品草稿/发布/分配保持固定设备版本；坏输入、跨组织和存储故障得到正确状态而不留下部分状态，内部日志安全关联到服务器请求 ID。

冻结源码与实际 HTTP/PG 结果支持上述限定结果。首次初始化及注册没有新增业务组织/成员；平台和业务权限保持分离。没有把过程中的 fixture 修正、合成检查或受控微信身份称为真实外部验收。

## 成功标准逐项检查

| 标准 | 直接核验与结果 |
| --- | --- |
| C1 同一源码与本批 23 项真实契约操作 | 独立逐 blob 比较导出与候选：1,383 个 blob 全匹配。核对工作流及实际调用 helper 得到 11+5+7 个不同操作；真实 TCP、真实 PG 响应再经当前 OpenAPI 校验。静态 96/603 单列为合成检查。 |
| C2 不可信输入严格拒绝且无写入 | 成员显式 false 与遗漏/null 区分，nullable expires_at 保留；产品字段和 App telemetry 拒绝重复、未知、大小写歧义、错误类型、尾随 JSON。非正 ID、坏编码与显式空过滤值拒绝；检查业务表/审计/遥测/shadow/alarm/outbox 状态不变。 |
| C3 认证授权、归属及版本语义 | 平台管理员与 USER 分离；无自动业务租户。实际 signed token 撤销、成员撤权、viewer 写 403、跨租户资源 404、失效成员 401。设备模型版本不随新模型发布移动，不可读字段不泄漏。 |
| C4 故障原子性 | 平台关系与 audit 故障、产品分配末端 audit 触发器故障、App telemetry/shadow 后 alarm_rules 故障均真实注入；状态摘要确认回滚。未据此声称所有提交未知结果或全部故障时序已验收。 |
| C5 安全诊断与能力不足 | 注入 logger、常量消息与 Context 日志；实际 provider 错误保留安全 SQLSTATE 和请求 ID，不写 body/query/header/原始 URL、SQL、凭据或设备明文。能力缺失返回安全对象且不制造 err。 |
| C6 共享修复 | admin/app 六个真实身份存储故障为 500，noRows/失效/版本撤销仍为 401，并 Abort 防止继续执行业务；两个显式空数值 filter 为 400；合法 128 Unicode 字符组织名创建 201、129 拒绝 400 且无写入。摄像头既有错误对象形状保留。 |
| C7 回归、双审及证据准确性 | 独立选定真实 PG/race 命令退出 0；执行者同冻结源完整 make verify 已终端退出 0，33 测试包，focused 21 顶层且无 SKIP；110 Python 和 96/603 静态退出 0。同树代码审阅明确通过，且具备技能与 overfit 检查。 |

## 独立验证

在精确导出执行：

```bash
GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated-33127-DSN> \
IOLINK_CONTRACT_PYTHON=<main-repo>/.venv/contracts/bin/python \
go test -race -shuffle=on -count=1 -v ./internal/core \
  -run '^Test(PlatformOrganization|PlatformFarmMemberContract|ExtendedAppContracts|ProductContractHTTP|IdentityStorageFailures|BaselineNumericFilters|TenantNameLimit)'
```

- 独立 exec session `82142` 已终端 **exit 0**，core `208.368s`，shuffle seed `1791914949665607955`。所选源码包含 21 顶层目标，覆盖本批正向操作及命名反例。工具输出约 39,965 tokens 被截断，自有可见输出保留 18 顶层 PASS；不把截断片段冒称完整 21 项归档。完整计数依据执行者冻结源 focused 日志另行核对，而非伪造审阅者完整录屏。
- 独立 `<contracts-venv>/bin/python scripts/check_contracts.py`：exit 0，96 操作/603 合成 fixture。
- 独立 gofumpt 检查全部 35 个改动 Go 文件：exit 0，无未格式化文件；`git diff --check d4d7e02 e88a723196d503455a6b9d3c0d4d58dd085248eb`：exit 0。
- 独立验证全部 1,383 个 tracked blob 与导出对应内容一致。web 是复用 gitlink，构建产物未冒称新增受审生产源码。
- 独立解析 `products/verified-provider-dump.jsonl`：27 条 JSONL，7 条 ERROR/http_provider，7 个不同请求 ID，7 条 err 全为 `postgres_42P01`；与 dump-summary 一致。

真实测试使用专属 loopback 33127 PostgreSQL 16.15 / TimescaleDB 2.30.2，各测试独立创建/删除数据库。未新增或清理执行者容器/卷/网络；资源收尾由执行者核对归属后完成。

## 已核读的执行者与同伴证据

证据根 `docs/evidence/deployment/2026-10-14-extended-http/`：

- `README.md`：范围、环境、过程证据与外部限制准确；仍写待最终双审，由执行者在闭环时更新，不能把本报告当作已发布证明。
- `snapshot-verify.log`：核读最终 33 个 ok 测试包、无 FAIL/DATA RACE，core `419.417s`；root 确认 exact export 的 session `15762` 终端 exit 0，包含 build/vet/race。审阅者未独立重跑全库。
- `snapshot-focused.log`：最终 PASS/core `230.035s`；session `20878` 终端 exit 0，完整 21 个顶层、无 SKIP，与独立选择命令一致。
- session `66278`：执行者报告并归档 110 Python 与静态 96/603 exit 0。Python110计为执行者证据；本审阅者静态结果另有独立重放。
- `platform/`、`app/`、`products/`：核读命名的正向、strict 输入、权限、关系故障、能力不足、晚期 audit/alarm 故障及状态摘要；只将 worker 共享工作树日志当 RED/GREEN 过程证据，最终源码结论绑定冻结导出。
- `identity-provider-before.log`、`empty-filter-before.log`、`tenant-unicode-before.log`：旧错误与本候选回归对应，不把旧失败日志计入新源通过结果。
- `.omo/evidence/extended-http-code-review.md`：逐项读完，同树 APPROVE/WATCH；其 1,383 blob、七组独立真实 PG/race、六项输入/摄像头边界、vet、静态重放，以及完整 root 结果核读均明确标识独立与执行者来源。

生产及测试检查路径包括 adminapi/auth.go、auth_errors.go、handlers.go、membership_input.go、organization_errors.go、platform_users.go、product_input.go、products.go、registration.go、tenant_admin.go；appapi/auth_errors.go、extended_input.go、server.go、telemetry_v2.go；core/admin_store.go、platform_admin.go、users.go；平台/产品/App HTTP 和边界测试、identity_provider_contract_test.go、baseline_empty_query_contract_test.go、tenant_name_contract_test.go；admin OpenAPI 本次实际缺失状态声明。完整变更以基线→候选 diff 为准，未审 working tree 后续草稿。

## programming / remove-ai-slops 直接检查

已咨询两技能及 Go 错误/测试指导，直接对完整生产增量与测试做 overfit/slop 检查；同伴报告也明确覆盖相同视角。没有用同伴结论替代直接检查。

- 真实 HTTP、独立 OpenAPI enum/shape、真实 PG 状态和注入 SQL/触发器故障能因业务回归失败；不是删除即通过、字符串移除测试或实现常量互相自证。
- 多表无写入/回滚断言与明确原子性标准相关，不是没有用户价值的生产再查询。补齐 active/unit/writable/nullable 等 fixture 保留原先要验证的 provider/权限分支，未放宽断言。
- 新解析器位于实际不可信 HTTP 边界，避免重复/未知/必填歧义，模型业务规则继续由原 domain/core 处理。未引入无需求 provider 抽象、全局归一化、SDK 依赖、静默 fallback 或广泛异常吞掉。
- logger 由 composition root 注入，错误仅安全类型；缺少能力日志不编造错误。输入 helper 小且责任明确；未发现本批造成实质维护负担的多余抽取或 tautological 测试。
- 既有 handlers/admin_store/server_test 等大文件仍超过技能尺寸建议，非本批引入，作为 LOW/WATCH 维护观察；本批新 helper 均小。没有成功标准要求本批完成全部既有结构治理，故不阻断。

## 精确证据缺口与未批准范围

无本批限定成功标准的阻断缺口。独立 verbose 输出截断限制如上；完整执行者 focused 记录与独立终端退出码共同支持结论，不能称为完整自有持久 transcript。

本报告不认证全部 96 HTTP 真实面、R02/V1d 全部矩阵、APIKey/Open/restore/setup 后续面、摄像头视频运行时、GB/media/playback、新 M7/M8、浏览器交互、真实微信/设备、License 交付专项、长期容量/100GiB恢复。摄像头既有错误对象回归只覆盖共享认证形状。现有 Python strict 的 12 诊断保持未通过，不计作 strict gate PASS，且本批没有新增 Python 源码来掩盖它。

AGENTS、playback 和下一批 Setup/spec6 草稿未入候选；这些工作树变化不得随本批误发布。外部缺凭据/硬件部分继续保留 external_blocked。两位独立审阅通过的是同一 e88a723 的上述限定增量，不是“全部项目完成”。
