# IoLink — 智慧水产监测平台(自研服务)

单服务 IoT 中间层:设备接入(MQTT)+ 业务核心(数据管道/报警)+ 小程序 API,一个进程、一个数据库。
替代第一阶段「华为云 IoTDA + FunctionGraph」的托管方案,同时保持其升级路径(未来 ECS 只替换本服务的部署形态,设备端与小程序契约不变)。

## 架构

```
设备(水质终端, MQTT) ──► [access] ──事件──► [core] ──► PostgreSQL/TimescaleDB
                                             ▲            │
小程序 ──HTTPS /api/v1──► [appapi] ──Repository接口────────┘
```

- **access**(仓库 [iolink-access]) 设备 MQTT 接入、三元组鉴权、物模型校验、在线/离线判定;向 core 发布标准化事件
- **core**(主仓 `internal/core`) 数据校验入库、报警引擎(按池塘阈值)、报警记录;实现 contracts 定义的 Repository
- **appapi**(仓库 [iolink-appapi]) 微信登录、`/api/v1` 业务接口;只依赖 contracts 接口
- **contracts**(主仓 `contracts/` 嵌套 Go 模块) 共享领域模型与接口,唯一共享点,改动需评审

数据量级:<100 设备 × 1min ≈ 14 万行/天,单实例绰绰有余;设计重心在边界清晰与可升级。

## 仓库拓扑

| 仓库 | go module | 职责 | 负责人 |
|---|---|---|---|
| iolink(本仓) | `iolink` | 组装、部署、docs、core、contracts | - |
| [iolink-access] | `iolink/access` | 设备接入 | 独立负责 |
| [iolink-appapi] | `iolink/appapi` | 小程序 API | 独立负责 |

依赖方向(单向,禁止互相 import):

```
iolink-access  ──►  iolink/contracts  ◄──  iolink-core(主仓)
iolink-appapi  ──►  iolink/contracts  +  Repository 接口
```

- 各负责人 **只 clone 自己的仓 + contracts**(`go get iolink/contracts`),日常不碰主仓
- 主仓通过 **git submodule** 固定 access/appapi 的集成版本,`make bootstrap` 一键拉起

## 快速开始

```bash
make bootstrap        # init submodule + go mod resolve
make dev              # docker-compose 起 PostgreSQL/TimescaleDB + 运行 iolinkd
make test             # 全部模块测试
```

## 契约文档(改动须评审)

- `docs/api/openapi.yaml` — 小程序 API(appapi ↔ 小程序)
- `docs/mqtt-spec.md` — 设备接入规范(access ↔ 硬件)
- `docs/schema.sql` — 数据模型
- `contracts/` — Go 接口(代码级契约)

## 升级路线

第一阶段本服务部署于单机 docker-compose;设备永远只连 MQTT,小程序永远只打 `/api/v1`。
未来迁 ECS/微服务:把 access 或 appapi 拆成独立进程(contracts 已保证接口稳定),设备端与小程序零改动。

[iolink-access]: https://git.hyhy.fun/rsplab/iolink-access
[iolink-appapi]: https://git.hyhy.fun/rsplab/iolink-appapi
