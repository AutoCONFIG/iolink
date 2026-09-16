# ThingsPanel 基座验证报告(spike)

> 验证日期:2026-09-16 · 方式:本地全组件实装 + API 实测
> 结论:**基座路线技术可行,全部关键链路实测通过;小程序侧需自建 BFF(即现有 appapi)。**

## 一、验证环境

四组件全部本地起跑(非标准端口,与 iolinkd 开发环境共存):

| 组件 | 端口 | 说明 |
|---|---|---|
| thingspanel 后端 | 9999 | 源码编译(436 文件,一次编译通过) |
| thingspanel-gmqtt | 1883 | MQTT broker,后端以 plugin 模式对接,自动完成订阅注册 |
| PostgreSQL(库 ThingsPanel) | 5433 | 20 个 sql 迁移顺序导入,74 张表 |
| Redis | 6380 | 缓存/心跳事件 |

## 二、实测通过的关键链路

### 1. 设备接入 → 遥测落库 ✅
- 平台建产品(水质监测终端)+ 设备(dev-001,voucher=access token)
- MQTT 认证:username = voucher(一机一 token)
- Topic `devices/telemetry`,**payload 格式(重要,给硬件方)**:
  ```json
  {"device_id":"<平台设备UUID>","values":"<base64(JSON)>"
  ```
  ⚠️ 两个坑:`values` 是 `[]byte` → 必须 base64;`device_id` 必须显式携带(voucher 不自动映射)
- 实测:`temperature 27.5 / dissolved_oxygen 6.8 / ph 7.9` 全部落库 ✅

### 2. 场景联动阈值告警 ✅(调试中发现 4 个坑,均已趟平)
配置链:`notification_group` → `alarm/config` → `scene_automations`(触发条件 DO<4.0)
- 坑1:创建 API **默认 enabled=N**,必须再调 `POST /scene_automations/switch/:id`
- 坑2:告警动作 `action_type` 是**字符串代码 "30"**(AUTOMATE_ACTION_TYPE_ALARM),不是 "ALARM"
- 坑3:告警配置 ID 放 **`action_target`** 字段(不是 action_param)
- 坑4:动作字段语义不直观(文档少,靠读源码 automate_telemetry.go)
- 实测:DO=2.9 触发 → `alarm_history` 生成 ✅ → `GET /alarm/info/history` 可查 ✅
- 引擎日志确认条件评估:`[dissolved_oxygen]: 2.9 < 4.0 → true`

### 3. 小程序 6 页面 API 覆盖核查

| 页面 | ThingsPanel API | 结果 |
|---|---|---|
| 登录 | `/api/v1/login` | ⚠️ 平台账号体系,**无微信登录,需自建** |
| 首页统计 | `/alarm/device/counts` + 设备统计接口 | ✅ 可用 |
| 池塘列表 | — | ❌ **无池塘概念**,需设备分组映射或 BFF 自建 |
| 实时监测 | `/telemetry/datas/current/:id` | ✅ 实测通过 |
| 历史曲线 | `/telemetry/datas/history`(秒级时间戳) | ✅ 实测通过 |
| 报警中心 | `/alarm/info/history` | ✅ 实测通过 |

## 三、决策建议

**推荐基座路线,形态为「ThingsPanel 平台 + 自建水产业务 BFF」:**

```
设备 ──MQTT──► ThingsPanel(gmqtt+后端) ──open API──► 水产 BFF(现有 appapi 改造) ──► 小程序
                    │                                      │
                    └ 场景联动/告警/通知组(配置化)          └ 微信登录、池塘领域、权限
```

- **退役**:access 仓(它的 broker+adapter 更完整)、core 的管道/报警(场景联动覆盖,且支持"单类设备统一规则",比按池塘逐条配更强)
- **保留改造**:appapi → 水产 BFF(微信登录 + 池塘/养殖场领域 + 聚合 ThingsPanel API),这是我们真正的差异化业务,本来就要写
- **白得**:Vue3 管理后台、多租户(tenant_id 全模型)、OTA、可视化(thingsvis)、插件化协议(Modbus 插件现成)
- **代价**:跟进上游(它活跃,Apache-2.0 允许闭源 fork);动作/触发等 API 语义不直观(报告中的坑就是证据)

## 四、复现步骤(spike 环境仍在运行)

```bash
# 依赖
docker run -d --name tp-spike-pg -e POSTGRES_PASSWORD=thingspanel2026 -e POSTGRES_DB=ThingsPanel -p 5433:5432 timescale/timescaledb:latest-pg16
docker run -d --name tp-spike-redis -p 6380:6379 redis:7 --requirepass redis
# 迁移: upstream/thingspanel/backend-community/sql/{1..20}.sql 顺序导入
# broker: upstream/thingspanel/thingspanel-gmqtt,配置 cmd/gmqttd/spike.yml(已改端口)
/tmp/gmqttd.bin start -c cmd/gmqttd/spike.yml
# 后端:
/tmp/tp-backend.bin -config configs/conf.spike.yml
# 登录: tenant@tenant.cn / 123456 (x-token 头)
# 打数: go run ./cmd/tp-spike-pub -token water-sim-token-001 -payload '{"device_id":"...","values":"<b64>"}'
# 清理: docker rm -f tp-spike-pg tp-spike-redis; kill $(cat /tmp/tp.pid) $(cat /tmp/gmqtt.pid)
```

## 五、若确认走基座路线的下一步

1. appapi 改造为 BFF:微信登录 + 池塘聚合(open API client)
2. 硬件接入规范改用 ThingsPanel topic/payload(更新 mqtt-spec.md)
3. 前端:thingspanel-frontend-community 二开或直接使用(池塘/水产视图)
4. 与上游建立跟踪机制(定期 rebase 或功能开关隔离二开代码)
