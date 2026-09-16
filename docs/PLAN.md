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
- [ ] 并仓:iolink-access → internal/access;iolink-appapi → internal/appapi;contracts → internal/domain(三仓存档 tag v0.2.0)
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

## 2. 分工(并仓后按 package)

| Package/端 | 内容 | 负责人 |
|---|---|---|
| internal/access + cmd/mqtt-sim | 设备接入层与联调工具 | 负责人 A(原 access) |
| internal/core | 管道/报警/存储 | 主仓 |
| internal/appapi + 小程序 | /api/v1 与小程序 | 负责人 B(原 appapi)|
| internal/adminapi + web/admin | 管理后台 | 主仓 + 前端 |
| 硬件固件 | Modbus 采集→MQTT(按 mqtt-spec) | 硬件方 |

并行原则不变:契约先行,各 package 独立可测(fake/模拟器)。

## 3. 协作与流程变化

- 并仓后跨仓 go get / tag 流程取消;**接口评审 = 主仓 MR(internal/domain 或 /admin/v1 契约)**
- CONTRIBUTING.md 将随 M2 并仓同步改版(环境配置一节保留:netrc/GOPRIVATE 用于拉取 upstream 参照与 CI)
- 硬件接入规范 mqtt-spec.md 不变(M1 已按它验收)

## 4. 规模假设与升级触发条件

- 设计容量:<500 设备、每分钟上报(≈14 万行/天),单实例余量充足
- 升级触发:设备>500 / 设备远程控制 / 多租户商业化 / 复杂协议矩阵 → 优先方案:保持单程序扩容(资源加配);再不够,按 contracts 边界拆独立进程,设备与小程序零改动
