# 计划细节(需求追溯 + 各层详细设计)

> 本文是 [PLAN.md](PLAN.md) 的细节附件,文档总入口见 [README.md](README.md)。原则:**每一项设计都能追溯到需求文档原文;需求文档没有的(如 OTA)不进范围。**
> 架构总纲(三段式):**传感器(含监测终端)→ 本服务器(iolinkd,自研独立进程)→ 小程序**。
> 需求文档第一章的「商业云」段是初版快速实现路径,现役方案以本服务器为唯一中枢,见追溯矩阵第一行。
> 状态:与 PLAN v2 同步评审。

## 〇、原商业云组件 → 本服务器职责映射(只对齐职责,不复刻机制)

需求文档原方案里的每个商业云组件,在 iolinkd 中都有职责上的承担者。**注意:这是"职责归属表",不是"要实现的机制清单"——**

- ❌ 不实现云函数运行时/FaaS 调度/函数注册发现/动态加载——服务场景不同:本服务器是
  固定领域的单体应用,handler 集合编译期确定(就是 core 包里的普通 Go 方法),没有
  "第三方上传函数"的需求
- ✅ 只保证:原方案每个云组件的**职责**在 iolinkd 里有明确的归属模块,不因去掉商业云而丢功能:

| 原商业云组件(需求文档) | 原职责 | iolinkd 等价实现 | 状态 |
|---|---|---|---|
| **IoTDA** 设备接入 | 产品/设备管理、MQTT 接入、设备影子、状态 | access 层:内嵌 broker + 三元组鉴权 + ACL;设备影子表 device_shadows | ✅ M1 |
| **IoTDA 规则引擎** 数据转发 | 属性上报转发到云函数 | access→core 进程内 channel(比云转发少一跳网络+序列化) | ✅ M1 |
| **云函数 iot_data_handler** | 校验/识别设备/标准化/入库 | core.storeReading + access 取值白名单 | ✅ M1 |
| **云函数 alarm_handler** | 阈值判断→报警记录→通知 | core.alarmEngine(alarm_rules 按池塘)+ alarms 表;通知→Notifier | ✅ M1(通知 M5) |
| **云函数 device_status_handler** | 设备状态维护 | access 看门狗(在线判定)+ core.storeStatus + 影子表 | ✅ M1(影子 M2) |
| **云函数 api_handler** | 小程序业务 API | appapi(/api/v1)进程内直调 core,无 HTTP 中转 | ✅ M1 |
| **API Gateway** | 给小程序提供 HTTPS 入口 | Gin 对外端口(同进程) | ✅ M1 |
| **SMN/通知** | 短信/推送 | Notifier 接口:实现=微信订阅消息(政策敏感,独立适配器) | M5 |
| **OBS** | 备份/归档 | pg_dump 定时(M5 评估是否需要对象存储) | M5 |
| **ECS(远期)** | Go 业务系统 | 本服务器即"未来的 ECS 业务系统"——原方案升级路径=部署形态切换,代码不变 | 内建 |

> 关键等价性:原方案 FunctionGraph 是"无运行时"的托管函数,本项目**不还原其机制**,
> 只还原其职责——4 个 handler(iot_data_handler / alarm_handler / device_status_handler /
> api_handler)就是 core/appapi 包里的普通 Go 方法,编译期写死、随二进制发布,
> 调用方式从"云触发"变为"进程内方法调用"。将来若真出现"用户自定义数据处理逻辑"的需求
> (商业化、多租户),再评估是否引入插件化脚本机制(参照 ThingsPanel 的做法),现在不做。

## 一、需求追溯矩阵(需求文档 → 落点 → 里程碑)

| 需求文档章节 | 原文要点 | 本方案落点 | 里程碑 |
|---|---|---|---|
| 一 总体架构 | 最初:传感器→终端→MQTT→**商业云**(IoTDA+FunctionGraph)→小程序,为快速实现借商用运行时;<br>**现行目标:自研独立服务器进程,结构收敛为三段式——传感器(含终端)→ 本服务器(iolinkd)→ 小程序** | 本服务器=access(接入)+core(业务)+appapi(用户API) 单进程,商业云的全部职责等价内化,无外部平台依赖 | 已实现(M1) |
| 二 服务选型 | IoTDA+FG+APIG+DB+OBS+SMN | IoTDA→内嵌broker;FG→core;APIG→Gin;OBS→暂无(归档M5评估);SMN→微信订阅消息(M5) | M1/M5 |
| 三.1 产品模型 | WaterQuality/DeviceStatus 两个服务 | 物模型固定实现:wire.Report(属性)+ 设备影子(状态);多产品扩展留 products 表设计 | 已实现 |
| 三.2 数据格式 | JSON 字段+每分钟/5分钟 | mqtt-spec §3 字段表;上报间隔服务端可配(默认60s) | 已实现 |
| 四 云函数1 | 校验/识别/标准化/入库 | core.storeReading+访问层取值范围校验 | 已实现 |
| 五 云函数2 | 阈值判断→报警→记录→通知;阈值按池塘 | alarm_rules(按池塘)+alarmEngine+alarms 表 | 已实现(通知M5) |
| 六 两级报警 | 报警中心 + 微信订阅消息 | 一级:/api/v1/alarms(已实现);二级:M5 Notifier→微信接口(政策风险隔离) | M5 |
| 七 小程序6页面 | 登录/首页/池塘/实时/历史/报警 | uniapp 6 页面,逐一对应 /api/v1 接口(见§2.1) | M4 |
| 八 访问方式 | 小程序只打业务 API,严禁直连平台 | 架构不变量;IoTDA API 不存在了,设备面/用户面物理分离 | 已实现 |
| 九 实施步骤1-8 | 数据模型/产品/硬件/转发/函数/库/API/小程序 | 数据模型=schema;产品=物模型;硬件=mqtt-spec+modbus;转发/函数=core;库/接口/小程序=M2-M4 | 分解完成 |
| 十 增加 ECS | 以后业务层替换,设备端不动 | 设备只认 MQTT 规范;小程序只认 /api/v1;core 可拆独立进程 | 触发器见 PLAN§4 |
| 十五 三原则 | 设备面/用户面分离+稳定API | 同上三条,写入 CONTRIBUTING 不变量 | 已实现 |

## 二、API 详细设计

### 2.1 小程序 `/api/v1`(需求文档第八节 8 个接口全覆盖)

统一响应:`{"code":0,"message":"ok","data":...}`;错误码:0 成功,1xxx 参数,2xxx 认证,3xxx 业务,5xxx 内部。鉴权:`Authorization: Bearer <JWT>`,claims={uid,exp};JWT 有效期 7d。

| # | 接口 | 方法/路径 | 请求要点 | 响应要点 | 需求出处 |
|---|---|---|---|---|---|
| 1 | 登录 | POST /auth/login | {code}(wx.login) | {token, expires_in, user} | 页面①/步骤7 |
| 2 | 池塘列表 | GET /ponds | — | [{pond_id,pond_name,status,device_count,latest}] | /api/pond/list |
| 3 | 设备列表 | GET /devices?pond_id= | 可按池塘过滤 | [{device_no,pond_id,model,status,last_seen_at}] | /api/device/list |
| 4 | 设备详情 | GET /devices/{device_no} | — | 设备+latest | /api/device/detail |
| 5 | 最新水质 | GET /water/latest?device_no= | — | {ts,temperature,dissolved_oxygen,ph,turbidity,salinity} | /api/data/latest |
| 6 | 历史 | GET /water/history?device_no=&metric=&range=today\|7d\|30d | 降采样≤200点 | {metric,unit,points[]} | /api/data/history |
| 7 | 报警列表 | GET /alarms?limit=&only_unconfirmed= | — | [{id,device_no,pond_id,metric,current_value,threshold,level,message,confirmed_at}] | /api/alarm/list |
| 8 | 确认报警 | POST /alarms/{id}/confirm | — | 204 | /api/alarm/confirm |
| 9 | 首页汇总 | GET /stats/summary | — | {online_devices,offline_devices,alarm_devices,ponds[]} | 页面② |

### 2.2 管理后台 `/admin/v1`(M2 新增;需求:运营者要能配池塘/设备/规则)

统一:`Authorization: Bearer <JWT>`,claims={aid},独立密钥派生(admin_jwt_key = HMAC(SecretKey,"admin"));账号存 users 表 authority='ADMIN'(原型期单管理员,种子 SQL 建)。

| 接口 | 方法/路径 | 说明 |
|---|---|---|
| 管理员登录 | POST /admin/v1/login | {username,password} → admin JWT |
| 养殖场 | GET/POST/PUT/DELETE /admin/v1/farms | 原型期单农场,接口完整保留 |
| 池塘 | GET/POST/PUT/DELETE /admin/v1/ponds | 删除需无设备绑定 |
| 设备注册 | POST /admin/v1/devices | {name,pond_id,model} → 服务端生成 device_no+secret,**secret 仅此一次返回**(sha256 入库) |
| 设备管理 | GET /devices、GET /devices/{no}、DELETE | 列表含在线状态/最新值;删除须先解绑 |
| 报警规则 | GET/POST/PUT/DELETE /admin/v1/alarm-rules | {pond_id,metric,min,max,level};metric∈物模型枚举 |
| 报警管理 | GET /alarms、POST /alarms/{id}/confirm、POST /alarms/batch-confirm | 管理员视角(全租户) |
| 系统统计 | GET /admin/v1/stats | 设备总数/在线率/报警趋势(给总览页) |

## 三、数据库详细设计(最终 DDL 基线)

### 3.1 业务表

```sql
users(id PK, open_id VARCHAR(64) UK, nickname, phone, password_hash NULL,   -- password_hash: 管理员用
      authority VARCHAR(16) DEFAULT 'USER',                                 -- USER|ADMIN
      created_at)

farms(id PK, owner_id→users, name, location, created_at)
ponds(id PK, farm_id→farms, name, area_mu NUMERIC(10,2), created_at)

devices(id PK, pond_id→ponds, device_no VARCHAR(64) UK,
        secret_hash VARCHAR(64),                       -- sha256(secret), 注册时一次生成
        model, status VARCHAR(8) online|offline,
        last_seen_at, created_at)

device_shadows(device_no PK, last JSONB,               -- 物模型全字段最新值(含 battery/signal)
               ts, signal INT)                          -- M2 新增: /water/latest 读这里

alarm_rules(id PK, pond_id→ponds, metric, min_value, max_value,
            level, enabled DEFAULT TRUE, UK(pond_id,metric))

alarms(id PK, device_no, pond_id→ponds, metric, current_value,
       threshold, level, message, confirmed_at NULL, created_at)
       idx(pond_id, created_at DESC); partial idx(confirmed_at) WHERE confirmed_at IS NULL
```

### 3.2 时序表(TimescaleDB)

```sql
sensor_data(ts, device_no, temperature, dissolved_oxygen, ph,
            turbidity, salinity, battery, signal INT)      -- M2 补 signal 列
  hypertable(ts); retention 13个月; idx(device_no, ts DESC)

物模型字段白名单(access 层强制, 越界丢弃该字段):
  temperature 0~50℃ | dissolved_oxygen 0~20mg/L | ph 0~14
  turbidity 0~1000NTU | salinity 0~50ppt | battery 0~100% | signal -120~0dBm
```

### 3.3 数据生命周期

| 数据 | 保留 | 依据 |
|---|---|---|
| sensor_data 原始 | 13 个月(自动 retention) | 历史曲线最长 30 天 + 年度趋势 |
| alarms | 永久(业务记录) | 报警追溯 |
| device_shadows | 永久覆盖写 | 最新值 |

## 四、MQTT 接入细节(变化点标注)

- Topic:`iolink/up/{device_no}/properties`(QoS1)、`iolink/up/{device_no}/ack`、下行预留 `iolink/down/{device_no}/cmd`
- 认证:username=device_no,password=secret;错误即断;ACL 锁死自己前缀
- 载荷:字段全部可选(按传感器 presence 上报),**M2 新增 `signal`(int,RSSI)**
- 在线判定:连接即 online;断开或 3×interval 无消息 → offline(看门狗);interval 服务端配置
- 心跳即上报,无独立心跳包(省电)
- 命令下行(预留,M5 后按需):下发 {id,name,params},回执 {id,ok,error}

## 五、主程序装配细节(cmd/iolinkd)

```text
启动顺序: config → PG pool → migrations 检查 → core → access.Serve()(goroutine)
        → access.Run(看门狗, goroutine) → appapi(/api/v1) → adminapi(/admin/v1)
        → 静态资源(/ → web/admin/dist)
退出: SIGINT/SIGTERM → 停 API → broker.Close() → pool.Close()   [5s 超时]
配置项(IOLINK_*): HTTP_ADDR(:8080) MQTT_ADDR(:1883) PG_DSN SECRET_KEY
                 WX_APPID/WX_SECRET REPORT_INTERVAL(1m) OFFLINE_GRACE(3)
Goroutine 模型: 每消息一个事件, core.HandleEvent 同步执行(µs级);报警判定进程内;
               无内部队列(M5 订阅消息若慢, Notifier 内部异步化)
```

## 六、前端细节

### 6.1 管理后台(web/admin,Vue3+TS)

| 页面 | 路由 | 组件要点 |
|---|---|---|
| 登录 | /login | 用户名+密码 → /admin/v1/login |
| 总览 | /dashboard | 池塘状态墙(卡片:🟢🟡🔴+四参数) + /stats 统计卡 |
| 池塘 | /ponds | 表格+弹窗 CRUD;删除校验无设备 |
| 设备 | /devices | 列表(状态/最新值);注册弹窗:**secret 只显示一次**;解绑/删除 |
| 规则 | /alarms/rules | 规则表(池塘×指标);metric 下拉=物模型;min/max 至少一个 |
| 报警中心 | /alarms | 列表+筛选(级别/状态);确认/批量确认;历史 |
| 系统 | /system | 管理员改密(M5);预留租户入口 |

### 6.2 小程序(uniapp)

| 页面 | 数据接口 | 组件要点 |
|---|---|---|
| 登录 | /auth/login | wx.login→code;静默续登(refresh 不做,过期重登) |
| 首页 | /stats/summary | 统计卡 + 池塘状态墙(点击进详情) |
| 池塘列表 | /ponds | 状态色条 + 四参数速览 |
| 实时监测 | /water/latest | 每指标卡片;下拉刷新;30s 轮询 |
| 历史曲线 | /water/history | uCharts 折线;今天/7天/30天 切换 |
| 报警中心 | /alarms + confirm | 列表(级别色);点击确认;订阅消息授权引导(M5) |

## 七、报警细节

- 评估时机:每条 properties 事件入库后同步评估(µs 级)
- 规则:每池塘×每指标一条(uk);min/max 至少一端;命中即报警
- 去重:同设备同指标存在未确认报警 → 不重复生成(防风暴);确认后可再触发
- 级别:critical(min 破坏,如 DO 下限)/ warning(max 破坏)——溶解氧低是致死项,用 critical
- 两级通知:①报警中心(已实现) ②微信订阅消息(M5):alarm 触发 → Notifier(异步) → 微信 `subscribeMessage.send`;小程序端引导授权订阅
- 需求文档"池塘A DO=4.0 / 池塘B DO=5.0"场景 = alarm_rules 两行,已支持

## 八、安全与非功能

| 项 | 设计 |
|---|---|
| 设备密钥 | 注册时生成 32 字节随机 → 明文仅返回一次,库存 sha256 |
| JWT | HS256;app/admin 双 audience 独立派生密钥;设备侧不涉及 |
| MQTT | 原型期 TCP+认证(局域网/内网穿透场景);公网 4G 部署时前置 TLS(nginx/caddy 终结,M5) |
| SQL 注入 | 全参数化(pgx) |
| 水平越权 | appapi 全部查询按 uid→farm→pond 过滤 |
| 容量 | 14 万行/天;单实例;预留拆分路径 |
| 监控 | /healthz(已)+ /metrics(PG/设备/报警计数,M5) |
| 备份 | 每日 pg_dump 业务表 + monthly 归档时序表(M5) |

## 九、里程碑验收清单(全部对应需求)

| 里程碑 | 验收 |
|---|---|
| M2 | adminapi 全接口 curl 通过;注册设备→mqtt-sim 用新密钥上报→影子/曲线正确;规则 CRUD 生效(改阈值立即影响报警) |
| M3 | 浏览器全流程:登录→建池塘→注册设备→配规则→看到模拟器数据与报警→确认 |
| M4 | 真机:微信登录→看池塘→实时值→30 天曲线→报警确认 |
| M5 | 报警触发→微信收到订阅消息;备份可恢复;/metrics 有数 |
