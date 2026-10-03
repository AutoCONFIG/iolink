# 当前实现状态与证据

附加部署里程碑：统一服务器 `deploy/docker-compose.yaml` 与源码构建覆盖
`deploy/docker-compose.dev.yaml` 已通过隔离容器构建、首启及重启验证，待同快照双审和发布。
证据见 [Compose](evidence/deployment/2026-10-03-compose/)。此项不改变 M6b 及 TLS/容量阶段状态；内部日志转储另行开发。

基线代码：b71095bbb6b228caed74adf47c48ca4c6b07c92b，检查日期2026-09-19。文档双审通过后已实施M0及M1软件部分；旧文档的“全部完成/全绿”不构成本轮验收。需求细项见 ACCEPTANCE.md。

重建 Todo 4 的实现与验证候选已完成，待双审门禁收据写入后登记完成；新增应用层 normalized event 边界与真实 ingestion 回归。命令、环境、负面路径和证据见 [Todo 4证据](evidence/rebuild/mqtt-ingestion.txt)。该证据只覆盖软件/provider 链路，硬件和外部 provider 仍按 `external_blocked` 处理。

| 需求范围 | 当前状态 | 可确认的代码/产物 | 已知差异或验证缺口 |
|---|---|---|---|
| R01 | 实现待验收 | make verify统一入口、空构建缓存验证、GitLab CI配置 | 本地验证通过；远程CI未执行，不能登记全部验收通过 |
| R02 | 部分实现 | R02.a验收通过：标准、引用及159个合成fixture | R02.b已通过真实MQTT3/5软件集成；R02.c全部真实HTTP契约仍待M2验 |
| R03–R04 | 验收通过 | 版本/checksum/锁/事务迁移；显式旧库接管；CLI安全首启；配置校验与注入 | 限M0需求；逐设备周期、HTTP改密撤销在M2，生产发布仍未完成 |
| R05–R08 | 验收通过 | M1软件：恒时鉴权、身份匹配/停用拒绝、Will拒绝、精确字段、会话接管和看门狗 | 真实broker/MQTT3与5、慢设备隔离及60/300周期边界通过；未声称真实硬件通过 |
| R09、R11–R13 | 验收通过 | M1软件：遥测/影子/报警/outbox同事务，塘快照、24h采样ID去重、history边界与单位、并发报警唯一 | 真实DB/MQTT/HTTP历史测试及故障回滚通过；M2资源授权、M5通知可靠发送另验 |
| R10 | 部分实现 | R10.a验收通过：逐字段合并和时间、旧影子塘失效、周期输出 | R10.b调塘API及双端完整latest契约待M2 |
| R14 | 部分实现 | 管理登录和改密单测 | 首启与Argon2id已补；旧口令登录升级、token撤销未完成；全部负面用例未通过 |
| R15 | 未实现 | 微信用户建档已存在 | 用户分配/解除接口不存在，正常前后台归属链不闭环 |
| R16–R19 | 部分实现 | 管理CRUD/注册/规则代码 | 目标详情/调塘/软删除、校验和历史保留尚需实现验证 |
| R20 | 部分实现 | 部分列表按owner过滤 | 池塘/设备详情、latest/history、确认报警缺用户归属检查；不得称数据隔离完成 |
| R21–R22 | 部分实现 | 报警/统计/状态墙代码 | 筛选、去重统计、全量报警等级有差异；管理Telemetry依赖已补并通过单条MQTT实测，完整聚合契约仍待验 |
| R23 | 已实现（Todo 7 双审待登记） | web/dist 已构建并嵌入 internal/web/dist | 浏览器覆盖登录、总览、农场/池塘、设备、规则、报警、设置及错误/空/过期状态；证据见 docs/evidence/rebuild/admin-frontend.txt |
| R24 | 已实现（Todo 7 双审待登记） | 真实 Vite dist 已 embed，SPA fallback 与 API JSON 404 代码及浏览器构建验收 | Playwright、Go compile/vet、契约和 manifest 检查通过；admin/app handler IPv6 httptest 受沙箱阻断 |
| R25–R27 | 外部阻塞 | code2session客户端代码；小程序未实现 | 软件子项部分实现/未实现；需微信资质/真机，fake不是实发证据 |
| R28 | 软件通过，外部阻塞 | durable outbox/worker、WeChat HTTP adapter、重试/lease/归属重验及失败状态 | 真微信凭据、授权和设备回执缺失，外部实发为 `external_blocked` |
| R29–R33 | 软件通过，适用外部阻塞 | health/readiness、metrics、10秒有界排空、固定监听/TLS反代模板、整库custom备份恢复、容量烟测与CI | 真实TLS主机、100GiB恢复、发布中断和24小时500设备/90天容量演练为 `external_blocked` |
| R34–R35 | 软件验收通过 | M6a 产品模型、版本发布/分配、通用遥测、兼容投影、fixture 回填 | 外部不适用；证据见 docs/evidence/acceptance/R34/ 与 R35/ |
| R36–R37 | 已修复，待新快照双审 | 租户上下文、成员/RBAC、Casbin角色-资源-动作策略、资源归属过滤、权限版本撤销、组织与成员后台页；D15（支持到期时间显示/保存的时区一致性）已在 source `62c0ecab0e5aa32fc0ad7227a7d8d4ff45ca1b9a`、web `483cb2cecc4a368fdf6fc3406fa682207b514aea` 修复，并由三时区 Playwright 证据覆盖；旧审批因平台管理员legacy-owner缺陷及后续源变更作废，仍待同一最终快照的新双审 | R36.b/R37.b 的 Key、播放、命令、任务、报表等后续资源按对应阶段验收；D15 修复不构成 R36/R37 通过；硬件、公网MQTT和生产迁移仍按 `external_blocked` 记录；证据见 [R36](evidence/acceptance/R36/) 与 [R37](evidence/acceptance/R37/) |
| R38–R42 | 未实现 | 文档设计 | License、离线包、开放平台待实施 |
| R43–R45 | 未实现 | 文档设计 | 视频/地图/大屏待实施；真实验收所需外部输入未就绪 |
| R46–R54 | 未实现 | MQTT命令结构预留/调试CLI | 命令生命周期、HTTP/Modbus/网关/转发/调度/联动/Web调试/报表待实施 |
| R55 | 未实现 | 无全量发布验收包 | 必须逐项通过后再判定 |

## 已执行检查

M0完整命令、版本、日志及双审结论见 [M0证据](evidence/2026-09-19/M0/README.md)。M1的软件验收与双审见 [M1证据](evidence/2026-09-19/M1/README.md)，两者是不同时间的工作树快照。

- `make verify`：build/vet/全部Go包测试通过；独立工作树副本、空Go构建缓存、提供真实数据库连接的重复验证见日志。下载模块缓存复用，不声称离线全新机器已验收。
- 真实Timescale测试通过：空库、幂等、旧数据保留、结构漂移拒绝、checksum/版本异常、故意失败回滚、四并发迁移，以及真实core遥测signal写入。
- 实际二进制首启门禁、管理员初始化、HTTP登录、真实MQTT→数据库→管理latest、重启持久化及SIGTERM通过。
- OpenAPI两份35操作/159合成fixture通过，不能代替真实handler验收。
- 两位独立审阅员均明确通过M0代码；B首轮发现生产周期配置漏传，修复后复审通过。
- M1真实数据库100并发、影子合并/时序、24h去重/过期、旧塘快照、故障整体回滚、历史1/200点上限及真实MQTT3/5链路通过；access/core竞态检测通过。M1两审首轮发现缺陷，修复后两审均明确通过。
- GitHub/GitLab远程CI未由本地会话触发；本地CI等价命令、专属Timescale容器备份恢复和20客户端容量烟测已通过。真微信、真实TLS主机、100GiB恢复、24小时容量、硬件和视频仍为 `external_blocked`。覆盖率未取得有效数字，不编造覆盖率。

## 实施缺陷登记（待修）

| 缺陷ID | 关联需求 | 修复目标 |
|---|---|---|
| D01 | R01 | 代码已修：统一验证与CI已建；远程CI待执行 |
| D02 | R03/R09 | 已关闭M0 signal/迁移缺陷；M1事务/影子已验 |
| D03 | R15 | 用户与农场分配/解除闭环 |
| D04 | R20/R36 | 所有资源操作授权，不只过滤列表 |
| D05 | R10/R22 | 已注入管理Telemetry；M1影子存储已验；完整API/统计语义待M2 |
| D06 | R11/R21 | M1 unit/max_points已修并实测；M2报警筛选仍待修 |
| D07 | R12/R13/R19 | M1双阈值、并发去重、事务已修；M2规则完整CRUD/权限另验 |
| D08 | R08/R29 | M1状态幂等/重启/在线数和M5健康/就绪/有界排空已修并验证 |
| D09 | R14/R04 | M0首启/密码存储已修；M2登录迁移及token撤销待修 |
| D10 | R28 | 通知持久化、失败识别、接收人和软件重试/lease已验证；真机仍阻塞 |
| D11 | R30–R32 | TLS反代边界、custom整库备份恢复、迁移失败/回退文档和软件演练已完成；真实部署演练仍阻塞 |
| D15 | R37 | 支持到期时间在 Asia/Shanghai、UTC、America/New_York 环境下显示、编辑、保存和重载保持同一瞬间；已由 web `483cb2cecc4a368fdf6fc3406fa682207b514aea` 修复并有三场景 Playwright 证据，R36/R37 阶段门仍待新快照双审 |

此表是审查已发现问题的起点，不声称已完成逐行代码审计；实施时新的问题继续编号，不用更改需求来掩盖缺陷。
