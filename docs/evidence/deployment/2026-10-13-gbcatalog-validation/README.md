# GB catalog 日期测试修复与现有功能回归

日期：2026-10-13。基线 `9c5dba929cf705c0cbee307876b5c300d127ba37`。

本轮处理 S2a 全库验证发现的固定日期测试失败，只调整测试夹具的初始时间；业务代码、超时语义和原有断言保持一致。完整检查与同快照独立双审均通过，发布版本 v0.0.32。

旧夹具固定为 `2026-10-11T02:00:00Z`，但生产代码传给 `context.WithDeadline` 的截止时间由真实时钟执行。固定日期过去后，目录刷新尚未调用存储便超时。夹具改为运行时的 UTC 时间作为起点，后续仍由 fake clock 的 `Advance` 驱动过期场景。

测试专属容器为 `iolink-validation-pg-20261013`，标签 `iolink.task=gbcatalog-20261013`；不使用测试服务器业务数据库。原有 M7a 播放草稿与 AGENTS 修改不在本次发布范围内。

## 冻结候选与环境

候选 tree：`52c1266384b5cdc6a77ef6ef8e454ac9f882dae5`。相对基线只有 `internal/gbcatalog/catalog_test.go:88` 一行变更。`source-tree.txt` 是该包的 Git blob 清单。导出目录 `/tmp/iolink-gbcatalog-20261013/review` 复用未修改的 web 子模块 `39299df` 已构建 dist。

Go `go1.26.8 linux/amd64`，PostgreSQL 16.15，Timescale 2.30.2；环境取证见 `environment.log`。测试 DSN 指向本轮专属容器的 loopback 端口，helper 创建并删除逐用例数据库，不使用业务数据。

## 已执行验证

| 场景 | 命令 | 预期 / 结果 | 证据 |
|---|---|---|---|
| 旧夹具重现 | `go test -race -shuffle=on -count=1 -v -overlay /tmp/iolink-gbcatalog-20261013/overlay.json ./internal/gbcatalog` | overlay 仅替换为 HEAD 的旧测试文件；exit 1，11项失败 | `before-race.log` |
| 当前夹具，正常/重复/冲突/过期/取消/并发/存储错误 | `go test -race -shuffle=on -count=1 -v ./internal/gbcatalog` | exit 0，11项全部通过；未删除、跳过或弱化断言 | `after-race.log` |
| 契约依赖及静态契约 | 冻结导出内 `make docs-tools && make verify-contracts` | exit 0，96 operations / 568 synthetic fixtures 通过；不等于全部 live handler 验收 | `docs-tools.log`、`contracts.log` |
| 全库 build / vet / race / 实际数据库集成 | 冻结导出内设置专属 `IOLINK_TEST_PG_DSN` 后执行 `make verify` | exit 0，全库通过；不把浏览器 opt-in、外部微信/硬件或 License交付专项算作通过 | `candidate-verify.log` |
| 目录实际存储、原子替换、过期与迁移 | 冻结导出内设置同一专属 DSN 后 `go test -race -shuffle=on -count=1 -v ./internal/gbcatalogpg` | exit 0，无 SKIP；审阅提出全库非verbose记录限制后补充明确数据库执行取证 | `catalog-pg-race.log`、`invocation.md` |

第一条 full verify 在导出完成前误启动，未找到 `scripts/embed-frontend.sh`；保留 `export-order-error.log`，不计通过。导出完成后重新执行完整检查。

`make verify` 实际调用 `go build ./...`、`go vet ./...`、`go test -race -shuffle=on ./... -count=1`。数据库 DSN 已设置，`internal/testdb` 的缺 DSN 跳过分支未使用。`TestM6dBrowser_realManagementFlow` 的 opt-in 环境没有启用，浏览器继续由用户验收；真实微信、真实硬件与 License交付专项仍未在本轮验证。

## v0.0.31 远端产物

[GitHub CI 38344069527](https://github.com/AutoCONFIG/iolink/actions/runs/38344069527) 已成功，触发 ref 为 `v0.0.31`，源码 `9c5dba929cf705c0cbee307876b5c300d127ba37`。原始摘要见 `v0.0.31-ci.json`、`v0.0.31-jobs.json`。镜像构建推送与本地测试分别记录；CI 中 skipped 的测试 job 不算通过。CI配置仍只由 tag push 触发。

## 完成收据与清理

实现提交 `69a7631` 的 tree 精确为 `52c1266384b5cdc6a77ef6ef8e454ac9f882dae5`，与两个批准收据一致。后续文档提交仅归档证据与更新状态。全库有测试的33个包 PASS，目录夹具11项 PASS，目录 PostgreSQL 存储7项 PASS；原失败日志保留。

| 独立审阅者 | 同一tree结论 | 收据 |
|---|---|---|
| `/root/gbcatalog_release_code_review` | APPROVE，blockers=[]，WATCH为既有文件大小与真实时钟耦合的低风险观察 | [代码审阅](code-review.md)、[独立前后执行](reviewer-checks.log) |
| `/root/gbcatalog_release_gate_review` | APPROVE，blockers=[]，独立核对1084个导出blob与前后结果 | [门审](gate-review.md) |

审阅结束后核对专属标签，已删除 `iolink-validation-pg-20261013` 及其匿名卷 `a09589b125844eab9762baeacc0bd03c8181a6186c4853fa5afbc33b49889888`，见 `cleanup.log`。两位审阅者均已结束，临时导出与overlay随后删除。其他项目容器、服务器与原有草稿均保留。

现有测试仍受真实10秒context deadline约束，极长的进程停顿可能导致超时；这是原有边界，审阅列为非阻断观察。没有新增业务功能或宣称 M7a/M8/R55完成。
