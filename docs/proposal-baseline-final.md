# 最终方案提案:ThingsPanel 基座 + 水产业务 BFF

> 状态:**待评审(未实施)** · 依据:docs/baseline-spike-report.md(全部链路实测通过)
> 本文替代原「纯自研三模块」方案;若否决基座路线,原方案(PLAN.md)依然完整有效。

## 一、目标架构

### 形态A(推荐):单程序 —— 一个二进制,进程内直连

```
┌─ 养殖现场 ─────────────────────────────────┐
│  传感器 → RS485/Modbus → 水质监测终端        │
└────────┼───────────────────────────────────┘
         │ MQTT(devices/telemetry)
         ▼
┌─ iolinkd 单进程(一个二进制) ────────────────┐
│  ① 内嵌 gmqtt broker(thingspanel 插件)     │
│  ② 内嵌 ThingsPanel Application             │
│     NewApplication(WithConfigFile,WithRedis)│
│     →Start()/Wait()/Shutdown()              │
│     其 MQTT adapter 经 loopback:1883        │
│  ③ 水产 BFF(现 appapi,进程内直调服务层,     │
│     不走 HTTP)                              │
└────────┬───────────────────────────────────┘
         │
   PostgreSQL + Redis(数据库,任何架构都在进程外)
         ▲
   Vue3 管理后台(平台自带,直连 9999 亦可关停)
         ▲
   微信小程序(uniapp)→ BFF /api/v1
```

**可行性已验证**(源码实查):
- 后端即库:`internal/app.NewApplication(options...) → Start()/Shutdown()/Wait()`,
  Option 有 `WithConfigFile/WithRedis/DB`;还暴露 `GetUplinkBus()/GetDownlinkBus()`
- broker 即库:gmqtt 的 `plugin/thingspanel` 插件使 broker 与后端共库共 Redis,
  作为依赖嵌入同一进程,后端 adapter 走 loopback:1883(同机回环,µs 级)
- BFF 不经 open API HTTP:直接 import 平台 service 层单调用,零序列化

**进程间开销的诚实评估**:数据路径 设备→broker→adapter 为同进程 loopback TCP(µs 级);
原多进程形态的 BFF→平台 HTTP 跳数(毫秒级)在单程序中归零。
在 <100 设备、每分钟一条的量级下,单程序的收益更多是**部署与运维简单**(一个二进制),
性能本身两种形态都绰绰有余——但单程序同时把"未来量级上来"的路径也留直了。

### 形态B(备选):多进程独立部署

```
设备 → gmqtt broker(独立) → ThingsPanel 后端(独立, 9999)
                                        │ open API
                              水产 BFF(独立进程) → 小程序
```
适合:需要水平扩展、平台与业务独立发布、或用官方容器镜像零改动的场景。
进程间仅 loopback 通信,在本项目量级下性能差异可忽略;主要差别是部署与版本管理粒度。

**建议:默认形态A,保留形态B作为规模扩展出口**(两者代码几乎相同,只差装配方式)。

## 二、设计原则兑现情况(原三条不变)

| 原则 | 兑现方式 |
|---|---|
| 1. 设备只面向接入层 | 设备只认 ThingsPanel MQTT 规范;平台换/升级与设备无关 |
| 2. 小程序只访问业务 API | 小程序只见 BFF 的 /api/v1;ThingsPanel 对小程序不可见 |
| 3. 稳定 API | BFF 契约保持原方案定义不变,后端实现从自研 core 换成 ThingsPanel API |

## 三、现有资产处置

| 资产 | 处置 | 说明 |
|---|---|---|
| iolink-access 仓 | **退役存档**(tag v0.2.0) | 接入层由 ThingsPanel 接管;其 ACL/看门狗设计可留作评审参照 |
| 主仓 core(管道/报警) | **退役存档** | 场景联动覆盖,且"单类设备统一规则"优于按池塘逐条配置 |
| 主仓 contracts | **保留** | BFF 与未来扩展仍用它定义 /api/v1 领域接口 |
| appapi 仓 | **改造为 BFF**(核心工作) | 保留:JWT/路由/openapi 对齐;替换:Deps 实现→ThingsPanel open API client;新增:微信登录对接、池塘聚合 |
| cmd/mqtt-sim、tp-spike-pub | 改造 | 换 ThingsPanel 的 topic/payload 格式,做联调工具 |
| iolinkd(主仓 cmd) | **保留并升级** | 变成单程序宿主:内嵌 gmqtt + 内嵌平台 Application + BFF |
| docs/mqtt-spec.md | **重写** | 换成 ThingsPanel 的 topic/payload(base64 values 等坑位写清) |

## 四、实施步骤(评审通过后执行,预计节奏)

| 步骤 | 内容 | 产出 |
|---|---|---|
| S1 部署定型 | spike 环境标准化(docker-compose 整合,标准端口),部署文档 | 平台可一键起 |
| S2 BFF 改造 | appapi 接 ThingsPanel open API;微信登录接真实凭据 | BFF 全接口可测 |
| S3 硬件规范 | 重写 mqtt-spec(新 topic/payload);modbus-protocol-plugin 评估接入 | 硬件可开工 |
| S4 小程序 | 基于 uniapp app 二开 6 页面(登录/首页/池塘/实时/历史/报警) | MVP 可演示 |
| S5 通知闭环 | 报警→微信订阅消息(BFF 调微信接口,通知组配置 webhook 触发) | M4 目标 |
| S6 商业化预留 | 多租户(平台自带)、池塘分权(BFF)、数据归档策略 | 可扩展 |

## 五、风险与对策

| 风险 | 等级 | 对策 |
|---|---|---|
| 上游跟进成本(它活跃,我们二开) | 中 | 二开代码集中隔离(自有 module/独立前端页面);定期 rebase;锁版本不盲追 |
| 平台 API 语义坑(base64 values、动作代码"30"、默认禁用) | 已趟平 | 报告有完整清单;封装在 BFF 的 client 一层,业务代码不接触原始 API |
| 池塘领域不是平台原生 | 中 | 设备分组映射(一池塘=一分组);BFF 持有池塘→设备映射表 |
| 无微信登录 | 已确认 | BFF 自建(与原方案相同),不依赖平台 |
| 许可证合规 | 低 | Apache-2.0 允许闭源二开;保留 LICENSE 与修改声明 |

## 六、对比:为什么不继续纯自研

| 维度 | 纯自研 | 基座(本提案) |
|---|---|---|
| 已完成度 | M1 端到端 ✅ | spike 全链路 ✅ |
| 管理后台 | **需要从零建 Vue 前端** | 现成 Vue3 后台 |
| 多租户/OTA/大屏 | 全部自建 | 平台自带 |
| 协议扩展(Modbus等) | 自己写插件框架 | modbus 插件现成 |
| 代码可控性 | 100% | 核心业务(BFF)100%;平台二开可控 |
| 长期维护 | 全自担 | 平台层靠上游,业务层自有 |

## 七、待拍板清单

1. **是否确认基座路线**(本文核心)
2. 部署形态:单机 docker-compose(原型期)还是云上(哪朵云)
3. 小程序是否基于 thingspanel/app(uniapp)二开,还是全新写
4. 管理后台:直接用平台原版,还是水产定制视图(建议先用原版)
