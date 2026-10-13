# D18 组织创建请求校验

日期：2026-10-13，基线 `d0bef4a`。范围为现有平台组织创建接口的严格单对象 JSON 校验；验证与最终同快照独立双审通过，发布版本 v0.0.33。

契约 `docs/api/admin-openapi.yaml` 的 `POST /tenants` 要求仅有 `name` 的对象；历史审阅登记 D18：Gin 默认绑定忽略额外字段及尾随 JSON。沿用现有平台初始化、真实 HTTP、Timescale、事务审计与授权隔离场景验证修复，不扩展新业务功能。

## 候选与环境

初轮源码候选 tree `386c7d65af727270c879fd0ab7073ad68f5ecaa6`，仅修改 `internal/adminapi/tenant_admin.go` 与 `internal/core/platform_console_test.go`（含gofumpt导入分组）。导出 `/tmp/iolink-d18-20261013/review` 验证，排除共享工作区的 AGENTS/M7a草稿。Web gitlink `39299df` 未改变，复用已有dist并执行 `make embed-front`。

Go1.26.8、PostgreSQL16.15、Timescale2.30.2；专属容器 `iolink-d18-pg-20261013`，标签 `iolink.task=d18-20261013`，DSN非空且显式指向loopback33123；密码为本轮disposable值，不归档。环境见 `environment.log`，两个源码blob见 `source-tree.txt`。

## 已执行验证

| 场景 | 实际命令/操作 | 结果 | 证据 |
|---|---|---|---|
| 原代码拒绝额外字段回归 | red tree `dddaf3065da9f66b270658697bdfd8faf06d8b68`（仅新增测试）中，设专属DSN运行 `go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'` | 预期400，实际201，exit1，确认旧handler写入路径 | `before-race.log` |
| 初轮修复后实际HTTP/数据库组织流程 | 同一命令在初轮候选导出运行，设同一DSN | exit0；9种错误输入拒绝且组织/审计均无写入；正常创建允许尾随空白，保留403/409/事务回滚/资源隔离回归 | `after-race.log` |
| 原生服务curl | 候选 `go build -o /tmp/iolink-d18-20261013/iolinkd ./cmd/iolinkd`；独立默认测试DB，loopback33124/33125，`serve`；首启状态required=true，然后CLI仅初始化平台管理员并登录；curl POST `/admin/v1/tenants` | 额外字段400，尾随对象400，合法对象201；SQL核对仅accepted-org和一条tenant.created审计；服务已SIGINT结束exit0 | `live-unknown.http`、`live-trailing.http`、`live-valid.http`、`live-db.txt`、`live-stderr.log` |
| 静态契约 | 导出内 `make docs-tools && make verify-contracts` | exit0；96 operations / 568 synthetic fixtures，不代替全部live handler验收 | `docs-tools.log`、`contracts.log` |
| 全库build/vet/race与实际数据库集成 | 导出内显式设置专属DSN运行 `make verify` | exit0，33个有测试包通过；不把浏览器opt-in跳过算作通过 | `candidate-verify.log` |

9种错误输入包括额外字段、尾随对象、尾随scalar、尾随垃圾、大小写字段名、name:null、数组、null及空对象。沿用应用对name的长度/空白/保留名校验及事务语义；仅此入口变更，其他JSON接口没有改动。

首次red包命令未执行 `make embed-front`，报缺少 `internal/web/dist`，保留 `red-embed-setup-error.log`，不计通过；补嵌入后成功取得上述实际201失败。没有删除或弱化旧断言。

全库 `make verify` 的调用显式设置本轮DSN，日志为 `candidate-verify.log`，已返回exit0。非verbose输出不逐项枚举SKIP；上表verbose组织集成与原生服务证明本次数据库链路。浏览器由用户验收；真实微信/硬件和License交付专项未执行。

## 发布环境调整

用户明确要求停用 `git.hyhy.fun`，已执行 `git remote remove origin`；当前只保留 `github`，main上游为github/main。Go模块路径没有修改。

v0.0.32 [tag CI 38403143144](https://github.com/AutoCONFIG/iolink/actions/runs/38403143144) 已SUCCESS，源码d0bef4a；这是前一版本发布证据，不替代本候选验证。当前CI仍仅由tag push触发。

## 审阅后最终候选

初轮代码审阅发现非阻断的回归测试缺口：额外字段用boolean时，即使删除 `len(req)!=1` 校验，类型校验仍会拒绝请求。因此把同一个测试输入改为字符串额外字段，直接观察关闭未知字段的规则，没有新增重复测试。

最终 tree `8e856090c601c4900b9b6a60b68363868ff1f319`，导出 `/tmp/iolink-d18-20261013/final`。相对初轮只有测试输入一行变更，生产blob不变；`final-source-tree.txt`记录两个文件blob。

| 最终补证 | 实际结果 | 证据 |
|---|---|---|
| 最终用例+旧Gin handler overlay，同一原测试命令（设DSN） | exit1，实际201而预期400，证明最终字符串用例捕获原缺陷 | `final-old-handler-race.log` |
| 最终用例+仅移除len guard的overlay（其余生产代码不变） | exit1，实际201而预期400，证明闭合对象校验有被直接覆盖 | `closed-key-mutation.log` |
| 最终源码 `make build && go vet ./...`，随后同一实际HTTP/PG owner race场景 | exit0；9错误输入、无写入与原正常/权限/冲突/回滚场景通过 | `final-race.log`；build/vet结果见工具执行记录 |

完整33包race、静态契约与原生curl实测来自初轮快照；最终仅测试输入修改，生产代码一致，受影响场景已复跑。最终双审需绑定 `8e85609`，不能沿用初轮批准。

## 完成与双审收据

实现提交 `eed35d50092c60283503412a1de9a0f2681a7abd` 的tree精确为 `8e856090c601c4900b9b6a60b68363868ff1f319`，与最终两个独立批准收据一致。后续文档提交仅归档证据与更新状态。当前D18已闭环，不改变M7a/M8或外部验收状态。

| 未参与编辑的独立审阅者 | 最终同tree结论 | 收据 |
|---|---|---|
| `/root/d18_release_code_review` | APPROVE，CLEAR，blockers=[]；覆盖缺口已验证关闭 | [代码审阅](code-review.md)、[独立HTTP/PG执行](reviewer-race.log)、[快照核对](reviewer-identity.log) |
| `/root/d18_release_gate_review` | APPROVE，blockers=[]；独立HTTP/PG执行与1101个blob核对通过 | [门审](gate-review.md) |

初轮收据为 `initial-code-review.md` / `initial-gate-review.md`，对应初轮386c7d6tree，不计作最终批准。较早两个最终审阅代理因上游503/no_route_available中断，没有批准结果；本表为重新执行后的完整收据。

核对专属标签后，已删除 `iolink-d18-pg-20261013` 与匿名卷 `98a2b9e285e42a5527e2bacf7eed2e3dead8e198a3ee10b4bd9b68d0f0f66600`，见 `cleanup.log`；三份临时导出、overlay、binary与测试dump根目录均已删除。原生服务早已SIGINT结束，其他项目资源及共享工作区既有草稿未处理。

`live-*.http` 保留原始HTTP CRLF，Go输出及独立报告按取证归档；源码与状态文档另执行空白检查。诊断日志没有归档口令、JWT或原请求body。
