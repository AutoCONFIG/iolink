# IoLink 开发计划与路线图(v2 · 彻底原生开发路线)

> 文档入口:[docs/README.md](README.md)(地图) · 细节附件:[PLAN-DETAILS.md](PLAN-DETAILS.md)(需求追溯+各层设计+验收清单)

> v2 修订于 2026-09-16。**路线决策已确认:彻底原生开发(路线 C)**——所有功能原生自研,
> 单程序交付;ThingsPanel(upstream/thingspanel/)仅作功能设计参照,不进运行时。
> 评审依据:docs/proposal-baseline-final.md(路线对比)+ docs/baseline-spike-report.md(基座实测,留档)。

## 0. 已定架构决策(不再重复讨论)

| 决策点 | 结论 |
|---|---|
| 架构形态 | **三段式:传感器(含终端)→ 本服务器(iolinkd 自研独立进程)→ 小程序**;单程序=本服务器一个二进制(内嵌 broker + 业务核心 + 双 API + go:embed 前端) |
| 仓库拓扑 | **单仓**:access/appapi 模块仓并回主仓(边界=Go package);contracts 并回 internal/domain |
| 功能实现 | 全部原生自研(接入/管道/报警/双端 API/管理后台/小程序);替代原「商业云快速实现」路径,不嵌第三方平台 |
| 数据库 | PostgreSQL 16 + TimescaleDB 单实例:业务表 + sensor_data 时序宽表 + device_shadows 影子表 |
| 前端 | 管理后台 Vue3(soybean-admin/vben 模板 + Element Plus);小程序 uniapp |
| 升级触发器 | OTA / 真多租户 / 复杂协议矩阵 任一出现 → 重开基座评估;/api/v1 契约不变 |

## 1. 里程碑

### M0 — 契约与骨架 ✅(2026-09-16)
- [x] 契约三件套(openapi / mqtt-spec / schema)+ contracts 模块 + core 骨架 + TimescaleDB 建表

### M1 — 端到端数据链路 ✅(2026-09-16)
- [x] 内嵌 mochi-mqtt broker、三元组鉴权(sha256)、topic ACL、离线看门狗
- [x] 管道落库 + 池塘阈值报警引擎(去重)+ 全部 appapi 接口 + 微信 code2session 客户端
- [x] 单进程组装(iolinkd);实测:模拟器→落库→低DO→critical报警→报警中心→确认,全链路绿
- 期间成果:thingspanel spike 实测(基座评估,报告留档);基座决策:不采用,转参照

### M2 — 并仓 + 管理后台 API(当前)
- [ ] **并仓(两模块仓处置,不可遗漏)**:
  - [ ] 代码搬运:iolink-access → internal/access;iolink-appapi → internal/appapi;contracts → internal/domain
  - [ ] 回归:`go build/test ./...` 全绿;iolinkd 装配改回本地包 import;删 go.mod 对两仓的 require/replace
  - [ ] 文档处置:两仓 docs/PLAN.md 有价值内容(模块 backlog)并入主仓 PLAN-DETAILS 对应节
  - [ ] GitLab 仓处置:iolink-access / iolink-appapi 置为 archived(只读留档,tag v0.2.0 可追溯);CONTRIBUTING.md 同步改版(删跨仓流程)
  - [ ] 两仓负责人切换工作方式:clone 主仓,按 package 分工,MR 流程不变
- [ ] internal/adminapi:/admin/v1 —— 管理员登录、池塘 CRUD+绑设备、设备注册(生成 device_no+secret)、报警规则 CRUD、报警管理(列表/确认/批量)
- [ ] 设备影子表 device_shadows(上报 UPSERT,/water/latest 改读影子)
- [ ] 配套:docker-compose 加 admin 路由说明;openapi 拆为 app 与 admin 两份
- 验收:curl 全套 /admin/v1;mqtt-sim→影子→latest 正确
- 负责:主仓(adminapi)+ access 负责人(并仓/搬运与回归测试)

### M3 — 管理后台前端(Vue3)
- [ ] web/admin 脚手架:soybean-admin 或 vben 模板(拍板项3)+ Element Plus
- [ ] 页面:登录 / 总览(池塘状态墙+统计) / 池塘管理 / 设备管理(注册发密钥) / 报警规则 / 报警中心
- [ ] go:embed 打进 iolinkd,单二进制交付;CI 加前端 build
- 验收:浏览器完成"建池塘→注册设备→配规则"全流程
- 负责:前端(新)+ 主仓(adminapi 配合)

### M4 — 微信小程序
- [ ] uniapp 工程(参照 thingspanel/app 的工程化),6 页面:登录/首页/池塘/实时/历史/报警
- [ ] 真实微信 code2session 联调(IOLINK_WX_* 凭据)
- [ ] uCharts 历史曲线;报警中心+确认
- 验收:真机预览全流程走通
- 负责:appapi 负责人 + 小程序开发

### M5 — 通知与生产化
- [ ] 微信订阅消息:core Notifier 接口实现(报警触发→下发),BFF 提供订阅授权接口
- [ ] 备份(pg_dump 每日)、sensor_data retention 已内建、基础 metrics(/metrics)
- [ ] 部署定型:生产 compose / systemd 二选一,部署文档
- 负责:主仓

## 2. 分工(按交付物划分,三人协同参考)

> 原则:剩余工作按「交付物」切四条并行线,每条线有独立的验收物;线间只靠契约(openapi / admin-api 契约 / mqtt-spec)耦合,契约先行冻结即可全并行。
> 与原 access/appapi 两仓分工无关——那两仓 M2 并回后,原负责人按下表重新认领。

| 线 | 交付物 | 内容 | 技能 | 建议人选 | 工期估计 |
|---|---|---|---|---|---|
| **L1 后端线** | 可部署的 iolinkd | M2:并仓(机械)、/admin/v1 全套、设备影子表;M5:订阅消息 Notifier、备份/metrics、部署定型 | Go(主) | 主程(你) | 4~5 天 |
| **L2 管理前端线** | web/admin | Vue3 模板脚手架 + 6 页面(登录/总览/池塘/设备/规则/报警),go:embed 联动 | Vue3/TS | 前端 | 1.5~2 周 |
| **L3 小程序线** | uniapp 小程序 | 6 页面 + 微信登录联调 + uCharts 曲线 | uniapp/Vue | 前端(可兼)或第三人 | 1.5~2 周 |
| **L4 硬件线** | 固件 | Modbus 采集 → 按 mqtt-spec 上报(先模拟器后真机) | 嵌入式 | 硬件方 | 与 L1 联调 |

**协同节奏**(关键路径 = L2/L3 前端,所以契约最优先):
1. L1 第 1 天先冻结 `/admin/v1` 契约(照 openapi.yaml 的风格补 admin 部分),L2/L3 立即用 mock 开工
2. 三线每日集成:L1 合代码,L2/L3 对真实接口联调
3. 人数弹性:只有 2 人 → L2+L3 合并给同一人(都是 Vue 系);>3 人 → L1 拆出"并仓机械活"给新人练手

**按 package 的代码所有权**(并仓后 MR 评审归属):
internal/access、cmd/mqtt-sim → L1 兼(原 A 可认领);internal/core、internal/adminapi → L1;
internal/appapi → L3;web/admin → L2。

## 3. 协作与流程变化

- 并仓后跨仓 go get / tag 流程取消;**接口评审 = 主仓 MR(internal/domain 或 /admin/v1 契约)**
- CONTRIBUTING.md 将随 M2 并仓同步改版(环境配置一节保留:netrc/GOPRIVATE 用于拉取 upstream 参照与 CI)
- 硬件接入规范 mqtt-spec.md 不变(M1 已按它验收)

## 4. 规模假设与升级触发条件

- 设计容量:<500 设备、每分钟上报(≈14 万行/天),单实例余量充足
- 升级触发:设备>500 / 设备远程控制 / 多租户商业化 / 复杂协议矩阵 → 优先方案:保持单程序扩容(资源加配);再不够,按 contracts 边界拆独立进程,设备与小程序零改动
