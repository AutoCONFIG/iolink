# 当前实现状态与证据

## TODO 总览（2026-10-08 核对）

当前推进位置：**TODO 8 / M4 接口演示已完成软件核验**。按用户范围，小程序只展示接口调用及返回结果，正式小程序由专人开发。
源码 `2b66812fd0d6e64609c1bc3fccbccccf71a4f99d` 已获两位独立审阅者同快照 APPROVE；[证据](evidence/M4/2026-10-08/README.md)覆盖 39 项测试、真实数据库后端和浏览器演示。发布快照 `036523d` 亦获同两位审阅者批准，已推送 `main` 与 `v0.0.11`，tag CI 镜像构建发布成功；[发布收据](evidence/M4/2026-10-08/release.md)。
**主清单 20 项：13 项已完成已核验，剩 7 项（M7a、M7b、M8a–M8d、R55）**；补充对账与外部验收另列。
M6d 已发布 `v0.0.10`，源码 `81c842e6903814cee187e67a699485ef6f934148`、web `66c52624f6d97c9dd7fe29b05b64b3db39c4a72d` 的阶段证据保留。
外部微信、硬件、生产 License 和客户安装仍按 `external_blocked` 记录。

本节是对已有证据的状态整理，不是重新运行验收。下方旧表、计划勾选和历史证据
中的待审措辞存在滞后；有冲突时使用本节列出的范围和最终收据定位，不将旧勾选
当成完成依据。历史软件通过只覆盖其被审快照，不认证后续修改或全部外部链路。

五类状态：

- **已完成已核验**：对应软件范围有执行证据及两位独立批准；外部范围另列。
- **已完成待核验**：实现和已有测试可查，但该项真实环境验收或收据核对未完成。
- **正在进行**：实现或验收仍在推进，不能登记完成。
- **待完成已调研**：已有针对具体缺口的调查/审阅记录，仍需实现或闭环。
- **待完成未调研**：有需求设计，但未找到该阶段的专项技术调研、契约冻结与执行证据。

### 主清单

| 位置 | 内容 | 状态 | 范围、缺口及定位 |
|---|---|---|---|
| TODO 1 | 项目指南、范围冻结、provider/验收映射 | 已完成已核验 | 指南与静态检查范围；[证据](evidence/rebuild/foundation-guidance.txt)，收据 `.omo/evidence/todo1-review-e-code-review.md`、`todo1-gate-review-c.md`；不代表 R01/R02 全部发布验收通过 |
| TODO 2 | 领域规则、窄 ports、依赖边界与不变量 | 已完成已核验 | [证据](evidence/rebuild/contracts.txt)，收据 `.omo/evidence/todo2-review-f-code-review.md`、`todo2-gate-review-e.md` |
| TODO 3 / M0 | PostgreSQL/Timescale 底座、迁移、事务、审计 | 已完成已核验 | 历史最终软件候选双审批准；[证据](evidence/rebuild/postgres-timescale.txt)，收据 `.omo/evidence/todo3-final-code-review-v3.md`、`todo3-final-gate-review-v3.md`；旧计划未勾选须对账 |
| TODO 4 / M1 | MQTT 身份/ACL、采集、影子、历史、报警与 outbox | 已完成已核验 | 真实 DB/MQTT 软件链；[证据](evidence/rebuild/mqtt-ingestion.txt)，双审 `.omo/evidence/todo4-independent-{code,gate}-review.md`，同 manifest `d0dc8a9c…`；真实硬件另验 |
| TODO 5 / M2 | 登录、归属转移、资源 API 与授权 | 已完成已核验 | [证据](evidence/rebuild/ownership-api.txt)，双审 `.omo/evidence/todo5-independent-{code,gate}-review.md`，同 manifest `c4a1c98e…`；微信真实用户流程另验 |
| TODO 6 | 通知事务意图、持久 outbox、重试/租约与微信适配器 | 已完成已核验 | 软件范围；[证据](evidence/rebuild/notifications.txt)，双审 `.omo/evidence/todo6-independent-{code,gate}-review.md`，同 manifest `ef5bf5cf…`；微信实发另验 |
| TODO 7 / M3 | 管理后台、生产构建与集成前端 | 已完成已核验 | 被审版本浏览器范围；[证据](evidence/rebuild/admin-frontend.txt)，双审 `.omo/evidence/todo7-independent-{code,gate}-review.md`，同 manifest `ed8887f0…`；后续新增页各阶段另验 |
| **TODO 8 / M4** | **小程序六页最小接口演示** | **已完成已核验** | 用户收窄为接口演示；源码 `2b66812`，39 测试/类型检查/构建、真实 PG16/Timescale 后端测试和浏览器导航/失败/订阅演示通过；[代码审阅](evidence/M4/2026-10-08/reviews/code-last.md)与[门审](evidence/M4/2026-10-08/reviews/gate-last.md)同快照 APPROVE；[证据](evidence/M4/2026-10-08/README.md)；真实微信及真机仍 `external_blocked`，不代表生产小程序完整交付 |
| TODO 9 / M5 | 健康、指标、停机、备份恢复、TLS/容量/CI 基础 | 已完成已核验 | 软件范围；[证据](evidence/rebuild/m5-operations.txt)、[gate](evidence/rebuild/gates/M5.json)，批准 `.omo/evidence/todo9-independent-code-review.md`、`todo9-final-gate-review.md`；公网 TLS、大规模演练及远端 CI另列 |
| TODO 10a / M6a | 产品/物模型版本、通用遥测、兼容投影 | 已完成已核验 | R34–R35；[R34](evidence/acceptance/R34/README.md)、[R35](evidence/acceptance/R35/README.md)明确最终工作树双审通过 |
| TODO 10b / M6b | 多租户、成员/RBAC、撤权与当前资源隔离 | 已完成已核验 | R36.a/R37.a；同候选 `0335438` 双审；[最终证据](evidence/acceptance/R37/2026-10-04-version-revocation/README.md)、[gate](evidence/rebuild/gates/M6b.json)；未来资源随其阶段另验 |
| TODO 10c / M6c | License、配额/功能守卫、离线包与安装恢复 | 已完成已核验 | R38/R39.a/R40 软件范围；`8ad5505` 同快照双审、发布 v0.0.9；[证据](evidence/M6c/2026-10-05/README.md)；实际签发方/客户安装及 R39.b另列 |
| **TODO 10d / M6d** | **开放 Key、签名、重放、限流与后台** | **已完成已核验** | R41–R42；源码 `81c842e`、web `66c5262`；真实 Timescale、竞态/乱序、契约、前端、浏览器、Docker 及 429/UTF-8/500 回归通过；代码与 gate 双审 APPROVE；[最新证据](evidence/M6d/2026-10-07/README.md) |
| TODO 11a / M7a | RTSP/GB28181、媒体接入、播放鉴权及双端播放 | 待完成未调研 | R43；已有 [M7需求](EXTENSIONS.md)，未找到专项媒体接入/部署契约与阶段执行证据；需摄像机/GB参数/端口/微信真机 |
| TODO 11b / M7b | 坐标/腾讯地图、双端定位与只读大屏 | 待完成未调研 | R44–R45；有设计，尚无阶段实施调研证据；需权限过滤、WGS84/GCJ-02对照、断网/无Key状态、20屏刷新性能 |
| TODO 12a / M8a | 持久命令、回执、TTL、取消、幂等及周期调整 | 待完成未调研 | R46；有状态设计，待冻结命令接口和 worker/固件能力契约；模拟与真实硬件分别验收 |
| TODO 12b / M8b | HTTP签名上报、TCP网关桥接、Modbus TCP/RTU、子设备 | 待完成未调研 | R47–R49；有范围设计，待设备/寄存器/桥接专项调查与协议契约；写操作依赖 M8a |
| TODO 12c / M8c | HTTP/MQTT转发、cron、场景联动与调试台 | 待完成未调研 | R50–R53；有重试/租约/时区/防环设计，待专项执行方案与实现；依赖 M8a/M8b |
| TODO 12d / M8d | CSV/XLSX异步导出、日报、下载鉴权及清理 | 待完成未调研 | R54；有100万行/公式安全/5分钟下载/7天清理需求，待导出实现与SQL对账设计；日报依赖 M8c |
| TODO 13 / R55 | 同版本全量验收、缺陷关闭、手册与交付包审计 | 待完成未调研 | 有验收清单，尚无最终发布候选审计；前置所有软件及适用外部验收。F1–F4见下表 |

“未调研”指本次未找到阶段专项调研证据；不表示没有需求设计。M7/M8详细内容仍以
[EXTENSIONS](EXTENSIONS.md)及[ACCEPTANCE](ACCEPTANCE.md)为准，不在此改变范围。

### 补充 TODO 与遗留验收

| 位置 | 内容 | 状态 | 下一步与依据 |
|---|---|---|---|
| S1 | 服务器/开发两份 Compose、latest-pg16 | 已完成已核验 | 用户免双审；隔离构建/首启/重启通过；[证据](evidence/deployment/2026-10-03-compose/) |
| S2 | JSON日志转储、轮转、关联ID、脱敏及健康状态 | 已完成已核验 | 双审都批准 `1bc74b8`，验证 source `e1b78e1`；[证据](evidence/deployment/2026-10-03-logging/README.md)、`code-review.md`及 `.omo/evidence/logging-gate-review.md`；下方旧“待双审”已过时 |
| S4 | 空库自动建表、Web 首启初始化和业务 API 门禁 | 已完成待核验 | web `b340cae` 已推送远端；真实隔离 Timescale、并发/回滚/非法输入、Go 全库、契约、前端测试/类型检查/构建及 Docker 首启通过；审阅问题已修订，同新候选双审待完成；浏览器由用户验收；[证据](evidence/deployment/2026-10-08-web-setup/README.md) |
| V1 | 历史 TODO、Rxx状态及批准快照收据对账 | 已完成待核验 | 实现/历史批准已存在，文档同步尚未完整核对：TODO 3旧计划未勾、R36/R37及旧 Dxx逐项核对；TODO 8历史拒绝已由 `2b66812` 新同快照双审闭环；不得直接批量改 passed |
| V2 | 微信真实登录、订阅、通知及用户归属流程 | 已完成待核验 | 仅指已有服务端/适配器及接口演示软件；真实凭据、资质、用户/真机为 external_blocked；TODO 8软件演示完成不代替真实微信验收 |
| V3 | 实际签发方License导入/续期与客户离线安装 | 已完成待核验 | 已有工具/软件链；真实签发方、干净断网x86_64客户主机30分钟安装及ARM64证据待提供 |
| V4 | 公网TLS/网络暴露、100GiB恢复、发布中断、长时容量 | 已完成待核验 | 已有脚本/模板/软件烟测；真实主机、证书、恢复介质及24小时500设备/90天fixture演练为 external_blocked |
| V5 | 现有采集固件的实际值对照与断线恢复 | 已完成待核验 | 已有MQTT/DB软件证据；真实设备验收 external_blocked，后续控制/网关硬件功能还需开发 |
| V6 | 远端CI运行与当前发布版本产物核对 | 已完成待核验 | CI `56cbd91` 仅由新 tag push 触发镜像发布，main/PR 不触发；当前标签远端结果须另行取证，不能把本地检查当远端CI通过 |
| X1 | R36.b/R37.b/R39.b 后续资源权限、执行撤权和License复验 | 待完成已调研 | 跨阶段规则已明确；Key/播放/命令/jobs/下载的真实入口按 M6d–M8d分别实现验收 |
| S3 | Redis 类缓存引入调研 | 待完成已调研 | 仅形成候选用途、窄 port、故障语义和验证门槛；当前未引入 Redis 依赖、容器或生产配置；[调研记录](research/redis-cache-evaluation.md)；不阻塞 M6d |
| D12/D13/D15/D16/D17 | 平台owner边界、遥测角色、支持到期时区/精度、租户管理员资源范围 | 已完成已核验 | 已纳入 M6b `0335438` 同候选批准；[gate](evidence/rebuild/gates/M6b.json)明确列出 fixed_defects；旧计划未勾需同步 |
| D14 | M6b证据归档与同快照批准绑定 | 已完成待核验 | gate和批准文件已存在，但manifest及旧表仍不同步；纳入 V1，完成对账后关闭 |
| D01–D11 | 历史构建/迁移/API/报警/通知/运维缺陷登记 | 已完成待核验 | 多项已有阶段修复与批准；逐项核对最终关闭证据，适用外部部分保留阻塞；不沿用旧表来推断当前代码缺陷 |
| F1 | 全量范围、Rxx归属、provider与证据追溯审计 | 待完成未调研 | TODO 13最终发布快照审计，现有manifest检查不代替该轮核验 |
| F2 | 最终架构/代码质量、安全与依赖边界审计 | 待完成未调研 | TODO 13，待全部功能完成后冻结候选执行 |
| F3 | 最终完整软件/真实链路/前端及适用外部QA | 待完成未调研 | TODO 13，必须重新绑定最终版本，不能拼接不同版本的passed |
| F4 | 最终范围、发布产物、手册与恢复流程一致性审计 | 待完成未调研 | TODO 13，和F1–F3汇总至R55；适用外部未通过不能称全部完成 |

推荐下一步：完成 V1 历史证据对账，再进入 M7a。M4 接口演示与 M6d 软件阶段已完成；微信、硬件、生产 License 和客户安装仍按 `external_blocked` 保留。

附加部署里程碑：统一服务器 `deploy/docker-compose.yaml` 与源码构建覆盖
`deploy/docker-compose.dev.yaml` 已通过隔离容器构建、首启及重启验证，并发布至 v0.0.6；用户明确免除该配置变更的双审。
数据库追踪 `timescale/timescaledb:latest-pg16`，启动时拉取。证据见 [Compose](evidence/deployment/2026-10-03-compose/)。

附加日志里程碑：JSON 文件持久化、轮转、内部采集/通知事件、HTTP 请求关联和脱敏已实现，
固定源码 `e1b78e12d9dea3c1f60412fce1bcdaa0adb33b0e` 的真实隔离 DB/MQTT/HTTP/重启链路及完整检查通过，待双审。
证据见 [日志转储](evidence/deployment/2026-10-03-logging/)。此项不改变 M6b 及 TLS/容量阶段状态。

M6c License 与离线交付软件验收完成：源码 `8ad550552d56f061222cd1ff53226000003c703e`、web `0aa7771acf0ef6322c9ac1e4c9839616646bb675` 已通过相关全库、真实 Timescale 竞态、契约、浏览器与离线链路检查，并获两位独立审阅者明确批准。实际签发方、客户干净断网主机 30 分钟安装及 R39.b 仍未验收。证据见 [M6c License 交付](evidence/m6c-license-delivery.md)。

重建 TODO 10：M6a、M6b、M6c 软件子阶段已闭环，下一项 M6d（R41–R42 开放平台）。TODO 10 整体仍未完成；TODO 11–13 及适用外部验收仍待推进。

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
| R25–R27 | 软件接口演示通过；外部阻塞 | `2b66812` 六页接口演示、响应校验、报警确认和订阅模拟，同快照双审通过 | 用户范围仅接口演示；真实登录/订阅/六页真机链路需微信资质与设备，模拟不是实发证据 |
| R28 | 软件通过，外部阻塞 | durable outbox/worker、WeChat HTTP adapter、重试/lease/归属重验及失败状态 | 真微信凭据、授权和设备回执缺失，外部实发为 `external_blocked` |
| R29–R33 | 软件通过，适用外部阻塞 | health/readiness、metrics、10秒有界排空、固定监听/TLS反代模板、整库custom备份恢复、容量烟测与CI | 真实TLS主机、100GiB恢复、发布中断和24小时500设备/90天容量演练为 `external_blocked` |
| R34–R35 | 软件验收通过 | M6a 产品模型、版本发布/分配、通用遥测、兼容投影、fixture 回填 | 外部不适用；证据见 docs/evidence/acceptance/R34/ 与 R35/ |
| R36–R37 | R36.a/R37.a 软件验收完成 | 租户上下文、成员/RBAC、Casbin策略、资源隔离、权限撤销及组织后台；事务写入校验实时权限，遥测/报警校验权限版本，不完整报警上下文拒绝；source `ef6510ffa5e43fd9796352ae324e09dfc3820c68` 的完整检查、真实 Timescale HTTP 正反向回归与生产授权器 fake executor 验证通过；D12/D13/D15/D16/D17 已整合；最终候选 `03354381b5954087d82d0d3f807cad9993b94b8d` 已获两位独立审阅者明确批准 | R36.b/R37.b 后续资源按对应阶段验收；外部输入仍按 `external_blocked` 记录；证据见 [R36](evidence/acceptance/R36/) 与 [最终验证](evidence/acceptance/R37/2026-10-04-version-revocation/) |
| R38–R40 | R38/R39.a/R40 软件验收完成 | signed-only License、离线签发工具、管理 API、011 迁移、License-first 配额事务与安全错误、共享 feature guard、可恢复离线安装；源码 `8ad5505` 同快照双审通过 | R39.b 实际可选入口、实际签发方、干净客户主机30分钟安装、ARM64仍未验收；保留较早 `333fc42` 离线镜像来源说明；证据见 [M6c](evidence/m6c-license-delivery.md) |
| R41–R42 | 已实现待验收 | 开放 Key、HMAC、nonce、共享限流、scope/资源守卫及管理 UI；[最新证据](evidence/M6d/2026-10-07/README.md) | 同冻结候选重验及双审未闭环，不能登记完成 |
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
| D13 | R36/R37 | 已修复：viewer/member/support 遥测写入返回 403，owner/admin 保持授权写入；拒绝请求无部分状态。原始复现与修复证据见 [D13](evidence/acceptance/R37/2026-10-01-v2-permission-fix/)；阶段门待最终快照双审 |
| D15 | R37 | 支持到期时间在 Asia/Shanghai、UTC、America/New_York 环境下显示、编辑、保存和重载保持同一瞬间；已由 web `483cb2cecc4a368fdf6fc3406fa682207b514aea` 修复并有三场景 Playwright 证据，R36/R37 阶段门仍待新快照双审 |
| D16 | R37 | 已修复：未编辑授权保存保留原始秒与小数秒；web `e84092326d9979e066c611e2be8eb6b9736e09f3`；证据见 [D16](evidence/rebuild/M6b-D16/)；阶段门待最终快照双审 |
| D17 | R36/R37 | 已修复：本租户 owner/admin 无农场分配仍可访问全部业务资源和批量确认报警，其他角色保持农场范围；实时授权和跨租户拒绝无部分写入；批量确认路径补丁与回归测试随最终候选提交；证据见 [D17](evidence/acceptance/R37/2026-10-03-d17-app-scope/)；阶段门待最终快照双审 |

此表是审查已发现问题的起点，不声称已完成逐行代码审计；实施时新的问题继续编号，不用更改需求来掩盖缺陷。
