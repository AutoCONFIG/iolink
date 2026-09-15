# upstream/ — 开源物联网平台参考项目

收集的开源 IoT 平台源码,供 IoLink 设计与开发参考。均为上游项目的克隆,不直接修改;更新方式:进入对应目录 `git pull`。

| 项目 | 语言/技术栈 | 许可证 | 定位 | 选型理由 |
|---|---|---|---|---|
| [hummingbird](https://github.com/winc-link/hummingbird) | Go | Apache-2.0 | 超轻量单体平台 | SQLite/LevelDB 存储,内存占用极低;轻量级方案的代表参考。⚠️ 上游约 2025-05 起停止维护 |
| [magistrala](https://github.com/absmach/magistrala) | Go 微服务 | Apache-2.0 | 云原生 IoT 框架 | 前身 Mainflux;消息中间件 FluxMQ + 身份/权限/设备供给,微服务架构代表 |
| [sagooiot](https://github.com/sagoo-cloud/sagooiot) | Go (GoFrame) + Vue3 | LGPL-3.0 | 企业级全功能平台 | 设备管理、物模型、规则引擎、告警等标配功能齐全,前端完整 |
| [thingspanel](https://github.com/ThingsPanel/thingspanel-backend-community) | Go (Gin) + Vue3 | Apache-2.0 | **插件化/组件化平台** | 与本项目插件化方向直接对口,见下方说明 |

## ThingsPanel 插件体系速查(重点参考)

ThingsPanel 是四个项目中唯一以插件化为核心架构的,代码位置(相对 `upstream/thingspanel/`):

- **四类插件入口**(API 层 `internal/api/`,服务层 `internal/service/`):
  - 协议接入插件 `protocol_plugin.go` + `protocol_plugin/` — 多协议设备接入,平台通过 HTTP 向插件回查设备配置(`GetDeviceConfig`)
  - 服务插件 `service_plugin.go` — `service_type`: 1-接入协议 / 2-接入服务
  - 可视化插件 `vis_plugin.go` — 大屏/仪表盘扩展
  - 通知插件 `protocol_plugin/notify_plugin.go`
- **插件市场**: `market_*.go`(bundle 目录、安装、发布),含市场客户端 `market_client.go`
- **设备/产品模板**: `device_template*.go`、`device_config.go` — 物模型 + 插件组合成可复用模板
- 前端(单独仓库): https://github.com/ThingsPanel/thingspanel-frontend-community

## 候选但未收录

- [联犀 unitedrhino/things](https://github.com/unitedrhino/things) — Go 微服务 SaaS 平台,多租户做得好,但 AGPL-3.0
- [PandaX](https://github.com/PandaXGO/PandaX) — 低代码方向,AGPL-3.0 且 2025-01 后停更
- [iot-master](https://github.com/god-jason/iot-master) — 工业数采(Modbus/PLC)见长,如需工业协议参考可加
- FastBee / JetLinks — 国内活跃但为 Java,与本项目 Go 技术栈不符
