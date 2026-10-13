# S2a debug dump 增强

日期：2026-10-13。范围：用户要求的现有功能诊断增强，不涉及 M7a/M8 新业务功能。

## 行为与边界

- Debug 及以上脱敏 JSONL 写入 `{IOLINK_LOG_DIR}/{IOLINK_VERSION}/iolinkd.jsonl`，默认版本 `dev`；发布镜像内置 tag。
- stderr 遵循 `IOLINK_LOG_LEVEL`。HTTP 使用服务器请求 ID；MQTT 每次发布使用独立 ingest ID；数据库查询使用 span ID；通知发送、微信 token/订阅、code2session 记录固定操作、状态、耗时和错误类别。
- 不记录 body、header、query、URL、SQL/参数、密码、JWT、完整 payload 或客户端关联 ID。设备标识 hash 化。
- 默认启动清理，然后每6小时删除当前与旧版本日志中严格早于当前时刻6小时的记录。配置可调整两项时长；大小轮转仍独立生效。无有效时间戳/损坏行保守保留，符号链接、无关文件、历史根目录日志不处理；详见 DEPLOY。
- 写入或清理失败降低 readiness；一处日志根目录由单个常驻进程拥有。CLI 仅 stderr。

## 验证状态

固定候选 Git tree：`4bdc2d5a8fc688c13a309f64611c73691e054483`，基线 `ec1fb049f2316a2e5ec8c3e8509d797adf002334`。从 index tree 导出到 `/tmp/iolink-debugdump-review-4bdc2d5` 验证，排除了原工作树的无关改动；`source-tree.txt` 为 Dockerfile、cmd、deploy、internal 路径的292项 blob 清单。Web 没有修改，复用子模块 `39299df` 已构建的 dist。Go `go1.26.8 linux/amd64`，PostgreSQL 16.15，Timescale 2.30.2。

下列命令从固定导出执行（diagnostics 首轮在相同源码的共享目录执行，未设置 DSN 的 setup 集成跳过不算通过；全库命令另设专属 DSN）。`IOLINK_TEST_PG_DSN` 指向下述 disposable 数据库，不在命令记录中展示口令。

| 验证项 | 实际命令/操作 | 结果与产物 |
|---|---|---|
| 正常/重复/数据库失败 MQTT→core→PG，四表原子性，关联ID及SQL脱敏 | `go test -race -shuffle=on -count=1 -v ./internal/core ./internal/appapi ./internal/wechat ./internal/notifications -run 'TestDebugDump\|TestWechatLogin\|TestWorkerCorrelatesAttempt\|TestSenderTrace'`，已设 DSN 和 `IOLINK_TEST_DUMP_EVIDENCE_DIR` | PASS；`chain-race.log`、`chain/*.jsonl`，实际 TCP MQTT 和本地 HTTP provider |
| HTTP→License verifier 缺失→数据库诊断 | 同上 `TestDebugDumpHTTPRegisterDeviceLicenseRejectionCorrelation` | PASS；无新增设备，同一 server request ID，明确 license/verifier/rejected |
| 通知投递、微信 token/cache/订阅与登录，错误/超时 | 同上相关三个包 | PASS；sent/provider rejected/finish failed、HTTP失败、解码失败、超时均有记录且无测试密钥泄漏 |
| 版本目录、逐行时效、启动/定时/并发清理、轮转、安全字段、HTTP关联 | `go test -race -shuffle=on ./internal/observability ./internal/operations ./internal/access ./internal/setupapi -count=1 -v` | 专项 PASS；`diagnostics-race.log`，setup 数据库用例 SKIP，已由全库及独立审阅者的 DSN 测试补验 |
| 实际服务启动、空库状态、readiness、版本目录和输出级别 | 固定候选 `go build ./cmd/iolinkd`；设置测试 DSN、监听 loopback、`IOLINK_VERSION=v0.0.31`、info console；运行 `serve` 后 curl `/setup/v1/status`、`/readyz` | 均200；`live-http.txt`、`live-dump.jsonl`、`live-stderr.log`、`live-check.json`。file 含 DEBUG，slog stderr 不含 DEBUG；setup 请求的数据库记录使用响应头 request ID。进程已 SIGTERM 结束 |
| 契约 | `make docs-tools && make verify-contracts` | PASS；`docs-tools.log`、`contracts.log`：96 operations / 568 synthetic fixtures；不替代全部 live handler 契约验收 |
| Compose | production/offline `docker compose -f ... config --quiet`，提供合成密码/secret及离线镜像变量 | PASS，默认6h环境可解析；无部署服务器变更 |
| 全库 build/vet/race | 已设 DSN 的 `make verify` | build/vet PASS；race 中本次相关包与其余包 PASS，只有既有 `internal/gbcatalog` 的11个固定日期测试失败，不能称全库全绿；`candidate-verify.log` |
| 独立双审 | 两位未参与编辑者审查同一最终候选 tree `2bd5fd8` | 两位 APPROVE，blockers=[]；`code-review.md`、`gate-review.md` |

早期共享工作树 `make verify` 与子代理写新测试重叠，因临时未使用 import 失败，保留 `working-verify.log`，不算通过。最终检查使用冻结导出避免写入重叠。首次 live console 解析器误将 Gin 非 JSON 启动提示作为 JSON，修正为仅解析 slog JSON 行后成功；没有更改应用来规避检查。

### 既有全库失败

`internal/gbcatalog/catalog_test.go:88` 将 fake clock 固定为 `2026-10-11T02:00:00Z`，但 coordinator 的 store context 使用真实时钟 deadline，2026-10-13 运行导致 `context deadline exceeded`，下游断言共11项失败。`git diff ec1fb04 4bdc2d5 -- internal/gbcatalog` 无输出，该包不属于本次修改。原工作树已有一行 `time.Now().UTC()` 修改，未混入此发布。此限制交由独立审阅者核对；本轮不宣称 `make verify` 整体通过。没有删除、跳过或弱化这些用例。

## 审阅后最终候选

第一轮审阅指出三个非阻断问题，已进一步修复：通知 ID 提前到 Claim 前，Claim 失败与 Finish 失败使用 WARN 且保持同一关联 ID；记录克隆测试增加调用方记录重用检查，能观察共享空间污染。最终固定 tree：`2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b`，导出 `/tmp/iolink-debugdump-review-2bd5fd8`。相对上一候选仅修改 worker 及其测试、context 测试，共3文件；`final-source-tree.txt` 是最终同范围 blob 清单。

`review-fixes-race.log` 为两个修改包的 race PASS。`clone-mutation-fails.log` 使用 Go overlay 仅移除生产代码的 Clone，修正后的测试明确 FAIL（捕获 `!BUG`），证明可捕获目标回归；生产代码保留 Clone，原树测试 PASS。最终固定导出复跑 observability/notifications/wechat/persistence 的真实 DSN race 见 `final-delta-race.log`，4包51项顶层用例通过，无 SKIP。最终 `make build && go vet ./...` exit 0。初轮全库/真实链路结果适用于未变动部分，既有 gbcatalog 失败仍保留。

## 完成与独立审阅收据

实现 commit **`a7335a231174850d9b223dfadd21547de04555af`** 的 tree 精确为 **`2bd5fd8d97b1a2c101992efb0fe326e1ee3af36b`**，与两位独立审阅者批准的一致。后续文档提交只归档证据与更新状态，不修改该源码。发布版本 `v0.0.31`。

| 独立审阅者 | 同一最终tree结论 | 收据 |
|---|---|---|
| `/root/dump_final_code_review` | APPROVE，WATCH仅指既有结构/全库限制，blockers=[] | [代码审阅](code-review.md) |
| `/root/dump_final_gate_review` | APPROVE，blockers=[]，已核对源码、实测及代码审阅证据 | [门审](gate-review.md) |

初轮报告保存在 `initial-code-review.md`、`initial-gate-review.md`；其引用的本地 `.omo/evidence/debug-dump-code-review-artifacts/` 与 `.omo/evidence/debug-dump-4bdc2d5-gate-artifacts/` 分别归档为 `initial-code-artifacts/`、`initial-gate-artifacts/`。报告中的 `candidate.patch` 已无损压缩归档为 `candidate.patch.gz`，只记录初轮所选源文件差异，不是最终完整diff；最终差异可由实现commit与基线比较。最终两位审阅者的关键独立命令及实际输出已嵌入各自报告。原始日志保留输出空白，`live-http.txt` 保留 HTTP 的 CRLF，以便对照原始观察。

测试/审阅结束后核对专属标签，已删除 `iolink-debugdump-pg-20261013` 及其匿名测试卷，见 `cleanup.log`；实际服务进程与两份临时导出目录也已清理。没有操作其他项目容器或服务器业务数据。

专属数据库：`iolink-debugdump-pg-20261013`，标签 `iolink.task=debugdump-20261013`，Timescale `latest-pg16`，镜像 digest `sha256:6f139d56042989bd35f50ba5986e492cd32e35cc6778c50be2659add298dc09f`。测试 helper 为各用例创建和销毁隔离数据库。凭据为本次 disposable 测试值，不使用测试服务器业务数据。

现场浏览器由用户验收；真实微信 provider 和硬件未在本轮验证；License交付专项按用户要求延后。已有 M7a 回放草稿、AGENTS 用户修改与 gbcatalog 旧测试修改不属于本次发布。
