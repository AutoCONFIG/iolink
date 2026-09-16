# IoLink 开发计划与路线图

> 制定于 2026-09-16,基于第一阶段架构决策:自研单服务替代「华为云 IoTDA + FunctionGraph」,
> 保持设备端(MQTT)与小程序(/api/v1)契约不变,保留向 ECS 部署形态升级的路径。

## 0. 已定架构决策(不再重复讨论)

| 决策点 | 结论 |
|---|---|
| 架构形态 | Go 模块化单体:access(设备接入)+ core(业务)+ appapi(小程序API)单进程 |
| 仓库拓扑 | 主仓 = contracts + core + 组装;access/appapi 独立仓 + git submodule 绑定 |
| 共享契约 | `git.hyhy.fun/rsplab/iolink/contracts` 嵌套模块,版本化 tag,只增不改 |
| 数据库 | PostgreSQL + TimescaleDB(时序宽表 sensor_data),单实例 |
| MQTT | 先内嵌(规划 mochi-mqtt),接口抽象可换外部 EMQX |
| 协作 | 契约版本化兼容,见 docs/CONTRIBUTING.md |
| 升级路线 | 单机 compose → 100~500 台观察 → 商业化时 IoTDA→消息队列→ECS 微服务(设备/小程序零改动) |

## 1. 里程碑

### M0 — 契约与骨架 ✅(2026-09-16 完成)
- [x] 契约三件套:openapi.yaml / mqtt-spec.md / schema.sql
- [x] contracts 模块(domain/event/wire)+ core 骨架(管道/报警/仓库)+ cmd/iolinkd
- [x] access/appapi 两仓 scaffold + submodule 绑定 + 全部打 tag
- [x] TimescaleDB 启动建表,/healthz 端到端验证
- [x] CONTRIBUTING 协作规范

### M1 — 端到端数据链路 ✅(2026-09-16 完成)
- [x] access:内嵌 mochi-mqtt broker,设备三元组鉴权(sha256)+ topic ACL + 防伪造校验
- [x] 模拟器:cmd/mqtt-sim(周期上报)+ cmd/mqtt-once(单发,报警触发用)
- [x] cmd/iolinkd 全量组装(access+core+appapi 单进程);core 结构化实现 Authenticator/UserStore
- [x] appapi:真实微信 code2session 客户端(配置驱动)+ /stats/summary 首页汇总
- [x] 验收通过:模拟器→MQTT→sensor_data 落库→低DO触发 critical 报警→报警中心 API→确认消除
- 负责:access 负责人 + 主仓

### M2 — 报警闭环(报警引擎已在 M1 提前打通)
- [ ] alarm_rules 数据导入(池塘阈值配置页可后置,先 SQL)
- [ ] 报警产生 → 报警中心列表/确认接口联调
- [ ] 池塘状态聚合(normal/warning/critical)
- 验收:模拟低 DO → 报警中心可见 → 确认后消失
- 负责:core(主仓)+ appapi 负责人

### M3 — 小程序联调
- [ ] appapi:接入真实微信 code2session
- [ ] 小程序 6 页面:登录/首页/池塘列表/实时监测/历史曲线/报警中心
- [ ] 与 appapi 按 openapi.yaml 联调
- 负责:appapi 负责人 + 小程序开发

### M4 — 通知与运维加固
- [ ] 微信订阅消息适配器(Notifier 接口,关注平台政策)
- [ ] 数据保留/归档策略、备份、基础监控
- [ ] 部署文档(生产 compose / systemd)
- 负责:主仓

## 2. 工作包分工

| WP | 内容 | 负责人 | 状态 |
|---|---|---|---|
| WP0 | 架构/契约/骨架 | 主仓 | ✅ M0 |
| WP1 | 设备接入 access | 模块负责人 A | M1 |
| WP2 | 业务核心 core | 主仓 | M1-M2 |
| WP3 | 应用 API appapi | 模块负责人 B | M2-M3 |
| WP4 | 微信小程序 | 小程序开发 | M3 |
| WP5 | 硬件固件(采集→Modbus→MQTT) | 硬件方 | M1(先模拟器) |

并行原则:WP1/WP3/WP4/WP5 互不阻塞——契约(mock/模拟器)先行。

## 3. 规模假设与升级触发条件

- 当前设计容量:<500 设备(每分钟上报),14 万行/天,单实例余量充足
- 触发升级的信号:FunctionGraph→(未来)ECS 迁移点 = 设备数 >500、需设备远程控制、多租户/复杂权限、统计分析需求
- 升级动作:access 或 appapi 从单体拆为独立进程(contracts 保证接口稳定),设备端与小程序不改
