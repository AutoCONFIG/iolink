# IoLink 已实现功能总表

> 更新:2026-09-16 · 依据代码实查(非计划口径)。所有 ✅ 项均通过单元测试和/或端到端实测。
> 未实现项的排期见 [FEATURE-MATRIX.md](FEATURE-MATRIX.md)(M6-M8)。

## 一、设备接入(internal/access)—— 全部完成

| 功能 | 说明 | 验证方式 |
|---|---|---|
| 内嵌 MQTT Broker | mochi-mqtt 嵌入 iolinkd,端口 :1883,单进程无外部 broker | M1 端到端实测 |
| 设备三元组鉴权 | username=device_no + password=secret(sha256 存储),错误拒绝 | 单测+实测(错误密钥被拒) |
| Topic ACL | 设备只能发布 `iolink/up/{自己的}/#`,只能订阅自己的下行 topic | 单测 TestCheckACL |
| 伪造检测 | publish 的 device_no 与连接身份不符即丢弃并告警日志 | 代码路径 |
| 遗嘱/离线看门狗 | 断开事件 + 3×上报周期无消息判离线 | 单测 TestStatusChange |
| 上报字段白名单 | 七字段物理范围校验(温度 0-50℃ 等),越界丢弃字段不拒整包 | 单测 TestRangeValidation |
| 事件标准化 | 原始 payload → 领域事件(进程内,零序列化) | 单测 TestHandleReportNormalizes |

## 二、数据与存储(internal/core)—— 全部完成

| 功能 | 说明 | 验证方式 |
|---|---|---|
| 遥测入库 | PostgreSQL + TimescaleDB 宽表 sensor_data,retention 13 个月 | M1 实测落库 |
| 设备影子 | device_shadows:每设备最新物模型值 UPSERT,实时查询不打大表 | M2 实测(影子读写) |
| 在线状态维护 | 连接/断开/看门狗 → devices.status + last_seen_at | M1 实测 |
| 领域模型 | User→Farm→Pond→Device 链 + 双库 schema(schema.sql) | DDL 实测 |

## 三、报警引擎(internal/core)—— 全部完成

| 功能 | 说明 | 验证方式 |
|---|---|---|
| 按池塘阈值规则 | alarm_rules 表:池塘×指标一条,min/max 至少一端 | M1 实测(DO<4.0 触发) |
| 报警分级 | critical(如 DO 下限)/warning(上限) | M1 实测 |
| 报警去重 | 同设备同指标存在未确认报警不重复生成(防风暴) | M1 实测 |
| 确认流转 | 确认后可再触发;单条+批量确认接口 | M1 实测(204) |
| 通知派发 | 报警→异步解析池塘绑定用户→下发,不阻塞采集路径 | 代码路径(真机实发等微信凭据) |
| 微信订阅消息 | WeChatNotifier:token 缓存、模板消息,配置驱动(未配置自动跳过) | 代码完成,实发待 M4 |

## 四、小程序 API(/api/v1,internal/appapi)—— 全部完成

| 接口 | 对应小程序页面 | 验证方式 |
|---|---|---|
| POST /auth/login(微信 code→JWT) | ① 登录 | 单测 |
| GET /ponds(**含 status 状态墙 + latest 最新值聚合**) | ③ 池塘列表 / ② 首页 | 实测聚合字段 |
| GET /devices、GET /devices/{no} | 设备状态 | 单测 |
| GET /water/latest(**读影子**) | ④ 实时监测 | 实测 |
| GET /water/history(today/7d/30d,自动降采样) | ⑤ 历史曲线 | 实测(6 点返回) |
| GET /alarms + POST /alarms/{id}/confirm | ⑥ 报警中心 | 实测 |
| GET /stats/summary(在线/离线/报警计数) | ② 首页 | 实测 |
| 微信 code2session 客户端(配置驱动) | 登录后端 | 代码完成,真机联调 M4 |
| 用户级数据隔离(uid→farm→pond) | 安全 | 架构保证 |

## 四·五、多用户系统(internal/core users.go + 全链路隔离)—— 全部完成

| 功能 | 说明 | 验证方式 |
|---|---|---|
| 微信用户自动建档 | 首次登录 openid 幂等入库(EnsureUser),无需注册流程 | 单测(fake)+实现 |
| 多用户数据隔离 | 全部查询按 uid→farm→pond 过滤,用户只见自己的池塘/设备/报警 | 架构强制(repos 全带 owner_id 条件) |
| 管理员独立体系 | users.authority=ADMIN,独立登录(/admin/v1/login)与 JWT 派生密钥 | 单测+实测 |
| 多端账号并存 | 同表双形态:openid(微信)+ username/password(管理员) | DDL |

## 五、管理后台 API(/admin/v1,internal/adminapi)—— 全部完成

| 接口组 | 能力 | 验证方式 |
|---|---|---|
| POST /login | 管理员 JWT(独立派生密钥,与小程序 token 不互通) | 单测+实测 |
| farms CRUD | 养殖场管理,删除有池塘时 409 | 实现+错误路径 |
| ponds CRUD | 池塘管理,删除有设备时 409;**列表含 status+latest 聚合** | 实现+单测 |
| POST /devices | 设备注册:生成 device_no + 32 位 secret(**明文仅返回一次**),库存 sha256 | 单测+实测 |
| GET/DELETE /devices | 设备列表(在线状态)/删除 | 实现 |
| alarm-rules CRUD | 规则管理:metric 白名单校验、min/max 至少一端 | 单测(3 断言) |
| /alarms 管理 | 全量列表/确认/批量确认 | 实现+单测 |
| POST /password | 管理员改密(旧密码校验,新密码≥8 位) | 单测 |
| GET /stats | 总览统计(设备总数/在线/离线/未确认报警) | 单测+实测 |

## 六、运维与交付(cmd/、deploy/、web/)—— 全部完成

| 功能 | 说明 | 验证方式 |
|---|---|---|
| 单二进制交付 | iolinkd = broker + core + 双 API + SPA + metrics,一个文件 | 构建冒烟 |
| 管理后台嵌入 | web/admin/dist go:embed + SPA fallback(占位页可换前端产物) | 实测 curl / |
| /metrics | prometheus:devices_online / telemetry_total / alarms_total / notifications_total | 实测端点 |
| /healthz | 存活 + DB 连通 | 实测 |
| Dockerfile | 多阶段构建,CGO=0 | 已提交 |
| 生产 compose | restart 策略、健康检查、env 注入(deploy/docker-compose.prod.yml) | 已提交 |
| 备份脚本 | 业务/时序分离备份,保留 14 天(deploy/backup.sh + crontab 示例) | 已提交 |
| 部署文档 | TLS 反代(含 MQTT 8883)/备份恢复/微信启用/升级(docs/DEPLOY.md) | 已提交 |
| 联调工具 | cmd/mqtt-sim(周期上报)、cmd/mqtt-once(单发触发报警) | M1/M2 实测 |

## 七、待完成(均有归属)

| 项 | 归属 | 阻塞点 |
|---|---|---|
| 管理后台前端 6+1 页面 | L2 前端工程师 | 无(契约+交接文档已备) |
| 小程序 6 页面 + 微信登录真机 | L3 小程序 | 无(/api/v1 就绪) |
| 微信订阅消息真机实发 | M4 联调 | 小程序凭据(AppID/模板) |
| 硬件固件(Modbus→MQTT) | L4 硬件方 | 无(规范已冻结) |
| M6 商业化前置(多产品/多租户/OpenAPI 平台) | 未排期启动 | 见 FEATURE-MATRIX.md |
| M7 行业标配(视频/地图/大屏) | 未排期启动 | 同上 |
| M8 增强能力(转发/HTTP/Modbus/定时/联动/网关/调试台/报表) | 未排期启动 | 同上 |
