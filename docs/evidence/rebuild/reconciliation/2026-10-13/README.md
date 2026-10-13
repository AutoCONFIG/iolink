# V1 首批历史证据对账

日期：2026-10-13；起点 `732851c`（v0.0.33）。本轮对账、归档及修正旧状态引用，不重新认证所有现有功能，不修改业务源码、外部契约或需求范围。首批文档核对已完成，同冻结快照独立双审通过；V1整体仍在进行。

## 证据身份

| 阶段 | 可确认的历史身份 | 两份收据及范围 | 剩余限制 |
|---|---|---|---|
| TODO 3 | 代码审阅记录 internal/cmd 摘要 `601a1183ed47e5d31f9cea6c879bc0eea480b40474b0fb3910e567ca2fd030ef` | 本目录 `historical/todo3-code.md`、`todo3-gate.md` 均批准修复后的冻结共享工作树；迁移/事务/归属软件范围 | 门审没有显式重复该摘要，不能补造缺失的 Git commit/manifest；旧计划未勾选仍是历史记录，当前状态由 IMPLEMENTED 维护 |
| TODO 5 | manifest `c4a1c98e459d5d135bdf0121efd0e5b022ad9ee5944a0149d20ebd9f8a5fc43e` | `historical/todo5-code.md`、`todo5-gate.md` 明确同摘要 APPROVE，R14–R22软件范围；实现证据 `../../ownership-api.txt` | 当时真实DB记录含执行摘要，审阅环境有Docker/IPv6限制；不补称本轮重跑 |
| TODO 6 | manifest `ef5bf5cff614339c255d2846a7631af042a45417bce16186f2a02c3cdd2ab6e2` | `historical/todo6-code.md`、`todo6-gate.md` 同摘要 APPROVE；持久通知软件；实现证据 `../../notifications.txt` | 真微信订阅/实发仍 external_blocked |
| TODO 9 / M5 | 两份收据都称 shared working tree | `historical/todo9-code.md`、`todo9-gate.md` APPROVE软件范围；`../../gates/M5.json` 与 `../../m5-operations.txt` | 没有明确候选摘要/commit，不能补称已验证同一持久快照；TLS、100GiB、发布中断和24h容量仍 external_blocked |
| M6b / D14 | 执行source `ef6510ffa5e43fd9796352ae324e09dfc3820c68`；candidate `03354381b5954087d82d0d3f807cad9993b94b8d`；web `e84092326d9979e066c611e2be8eb6b9736e09f3` | `../../gates/M6b.json` 与 `../../../acceptance/R37/2026-10-04-version-revocation/` 的 README、verification.json、两份批准收据一致，明确R36.a/R37.a | 不认证R36.b/R37.b未来资源，整体需求manifest继续not_run |

归档的历史报告来自本机 `.omo/evidence/`，副本按文件内容保存；`historical-files.sha256` 可对照。报告内 `.omo` 路径表示当时位置，不表示原始日志全已归档在本目录。历史测试的实际结果及环境限制仍以原报告与对应 docs/evidence 为准。

## D01–D11 逐项核对

| 缺陷 | 后续软件证据 | 本轮结论及限制 |
|---|---|---|
| D01 / R01 | M0 clean-cache验证及CI配置；v0.0.32 GitHub tag镜像job115315675773 SUCCESS，见 `../../../deployment/2026-10-13-tenant-json/v0.0.32-ci-jobs.json` | 旧“远程CI未执行”已过时；当前远端只发布镜像，build/integration为SKIP不能当测试通过；R01最终新环境/全量验收仍待R55 |
| D02 / R03/R09 | `../../../2026-09-19/M0/README.md`、M1 README/manifest：migration/signal/事务/回滚 | 软件修复与历史双审已记录；M1 manifest摘要 `8fba8cca79ab322f73cc1d786a7285ace5cd843b1304004a44d5fe344e35d41d`，不是Git commit |
| D03 / R15 | TODO5同摘要收据，用户/农场分配/解除 | 软件关闭证据存在；真实微信用户流程另验 |
| D04 / R20/R36 | TODO5操作归属及M6b `.a` 当前资源隔离 | 软件证据存在；未来资源归其阶段，不称完整R36已验收 |
| D05 / R10/R22 | M1影子/依赖注入及TODO5 latest/API/统计 | 旧“完整API/统计待M2”已被后续软件证据覆盖；不重写M0当时只修部分的历史记录 |
| D06 / R11/R21 | M1历史units/max_points及TODO5报警筛选分页 | 旧“报警筛选待修”不表示当前未实现；具体边界见原收据 |
| D07 / R12/R13/R19 | M1双阈值/并发唯一/事务，TODO5规则生命周期/权限 | 已有软件证据；不称全部现版本API重新验收 |
| D08 / R08/R29 | M1生命周期/重启/在线数及M5健康/排空 | 软件记录存在；M5候选源绑定缺口保留 |
| D09 / R14/R04 | M0首启/口令存储及TODO5登录/改密/token撤销 | 旧“登录迁移/token撤销待修”已过时；后续首启流程变更按S4/S5证据单独追溯 |
| D10 / R28 | TODO6同摘要通知收据及M5软件记录 | 软件与真实微信验收分列，实发仍 external_blocked |
| D11 / R30–R32 | M5软件备份恢复、版本/迁移拒绝、TLS模板 | M5候选源绑定缺口及真实主机/恢复规模/中断演练保留，不伪造关闭证据 |

## 门禁范围限制

实际读取 acceptance-map：R36/R37 的整体 `software_status=not_run`。这些需求含未来 `.b`；已有M6b收据只批准 `.a`。本轮新增scope_note指向该历史收据，保留整体not_run，避免把未来资源改成passed。

`scripts/check_architecture_manifests.py --gate M6b` 目前只检查整体R36/R37状态，不能独立表达 `.a` 阶段完成；它实际返回exit1。该结果不是通过。脚本在成功分支还会重写历史gate文件，输出只包含gate/requirements/evidence，不能代替带candidate和双审绑定的历史收据。此工具语义需要后续修正，D14/V1不在本轮整体关闭。

## 已执行检查与本轮边界

本轮修改对象是台账和历史报告索引。没有创建数据库容器或运行浏览器；没有改动业务源码。

| 本轮实际检查 | 结果 | 输出 |
|---|---|---|
| `.venv/contracts/bin/python scripts/check_architecture_manifests.py --all` | exit0，55需求及引用结构通过；不等于所有需求验收通过 | `manifests.log` |
| `make verify-contracts` | exit0，96 operations / 568合成fixtures；不代替live handler验收 | `contracts.log` |
| `sha256sum -c historical-files.sha256` | exit0，8个归档副本摘要一致；另与本机8个原报告字节相等 | `archive-check.log` |
| 5份当前修改Markdown本地引用检查 | exit0，92个引用存在，missing=[] | `links.log` |
| `scripts/check_architecture_manifests.py --gate M6b` | exit1，R36/R37整体not_run；未覆盖历史gate元数据，不计通过 | `m6b-coarse-gate.log` |
| v0.0.33 GitHub tag CI 38410342151 | 镜像job115340679974 SUCCESS；build/integration SKIP，不计测试通过 | `v0.0.33-ci-jobs.json` |

M6b执行source和candidate的 `git diff --name-only ef6510f 0335438` 仅含docs路径，证实两个收据描述的“文档/证据差异”；M1 manifest文件实际SHA256与上表一致。不把历史日志或计划命令列为本轮运行通过。

## 本轮批准与归档收据

提交 `5dedd0f7fe7b00791e921d9b081f9013732a671c` 的 tree 精确为 `751f8ff4c3e255d4122b87bae68ffad7d65ca90d`，与两位未参与编辑的独立审阅者批准的一致。后续提交仅记录审批与更新V1a状态，不变更核对结论或扩大历史验收范围。

| 本轮独立审阅 | 同一文档tree结论 | 收据 |
|---|---|---|
| `/root/v1a_code_review` | APPROVE，CLEAR，blockers=[]；1143个快照文件、8个原文归档、92引用及静态检查核对 | [代码审阅](code-review.md) |
| `/root/v1a_gate_review` | APPROVE，blockers=[]；范围、历史身份、失败门禁结果及静态/契约复跑核对 | [门审](gate-review.md) |

独立审阅的实际命令与结果包含在报告中。审批仅覆盖V1a文档对账，不把V1/D14、完整R36/R37、后续业务阶段或外部验收标为完成。本轮没有新增数据库容器/常驻进程，审阅结束后临时导出 `/tmp/iolink-v1-audit-20261013` 已清理；原有M7a草稿保留。

V1剩余：TODO3/M5源绑定补证、整体需求与阶段门的子项语义核验、其余Rxx历史表逐项对账。M7a/M8新业务继续暂缓；真实微信/硬件、License交付、TLS/容量/安装等保持原外部验收状态。
