# 文档地图

> 想了解项目,按下面的顺序读。任何文档与代码不一致时,以代码 + 本地图为准并回报。

## 现役文档(评审/开发用)

| 文档 | 内容 | 读者 |
|---|---|---|
| [PLAN.md](PLAN.md) | **计划主文档**:已定决策、里程碑 M0-M5、分工、升级触发器 | 全员,先读这个 |
| [PLAN-DETAILS.md](PLAN-DETAILS.md) | **计划细节**:三段式架构与商业云职责映射、需求追溯矩阵、API/数据库/MQTT/前端/报警/安全全部设计、各里程碑验收清单 | 全员评审;负责人重点读自己模块的节 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 协作规范(⚠️ 其中跨仓版本流程将随 M2 并仓改版,以 PLAN.md §3 为准) | 模块负责人 |
| [api/openapi.yaml](api/openapi.yaml) | 小程序 API 契约(/api/v1,权威) | appapi 负责人、小程序开发 |
| [mqtt-spec.md](mqtt-spec.md) | 设备接入规范(topic/载荷/取值范围,权威) | access 负责人、硬件方 |
| [schema.sql](schema.sql) | 数据库 DDL(PG+TimescaleDB) | core 负责人 |

## 留档(历史决策依据,不再更新)

| 文档 | 内容 |
|---|---|
| [baseline-spike-report.md](baseline-spike-report.md) | ThingsPanel 基座实测报告(2026-09-16)——路线 C 决策的依据 |
| [archive/proposal-baseline-final.md](archive/proposal-baseline-final.md) | 基座路线提案 v1/v2(已否决,决策记录见 PLAN.md §0) |

## 决策一句话

**彻底原生开发**:三段式(传感器含终端 → 本服务器 iolinkd 自研独立进程 → 小程序),
单程序单仓交付;商业云(IoTDA/FunctionGraph 等)只保留职责映射,不引入其机制;
ThingsPanel 仅作功能设计参照。
