# 新会话项目指南

## 权威资料

先读 [`docs/README.md`](docs/README.md) 获取文档地图，再按其权威关系执行：用户范围 → `docs/PLAN.md` 与设计附件（`docs/PLAN-DETAILS.md`、`docs/EXTENSIONS.md`）→ 版本契约（`docs/api/`、`docs/mqtt-spec.md`）→ `docs/ACCEPTANCE.md` 验收。`docs/IMPLEMENTED.md` 是唯一实施状态记录；代码是实现证据，不覆盖需求。也须遵循 [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md)。

## 范围与兼容

这是绿地重建：没有正式生产部署或生产数据需要保留，可重设内部包与数据库 schema；不承诺生产数据升级或旧内部 API 兼容。必须保持 M0–M8、R01–R55，以及外部 API、MQTT 和业务语义不变；按验收阶段推进，不把文档通过或旧代码当成已完成。

## 架构与 provider

采用模块化单体：domain 定义业务规则，application 编排用例，依赖由领域/应用指向窄小、按能力划分的 ports；provider adapters 实现 ports，只有 composition root 选择和组装具体实现。领域和应用不得依赖 provider SDK；避免万能接口、微服务化及无需求的插件抽象。

首发参考 provider 仅为 PostgreSQL 16 + TimescaleDB、内嵌 MQTT、内置 HTTP API、微信适配器。MySQL、其他时序库及 QQ、Telegram、WhatsApp 是未来集成，当前并未实现；不得暗示已经支持。所需能力缺失时启动/操作必须 fail closed，不可静默降级或伪报成功。

## 安全与数据完整性

在操作边界执行认证、授权及资源归属校验，拒绝未授权和跨用户/租户访问。相关数据库变更使用明确事务边界；业务状态与 outbox 意图须原子提交，外部副作用在事务外执行，并显式处理重试、重复和未知结果。错误、能力不足或校验失败不得留下部分状态。

## 诊断日志

使用 composition root 注入的 slog logger；常驻服务由 internal/observability 统一输出脱敏 JSON。
消息保持常量，动态数据使用字段；HTTP 只记录路由模板和服务器生成的请求 ID，禁止记录
body、header、query、原始 URL、密码、JWT、MQTT/微信 secret 或完整遥测 payload。
新增字段须检查 observability 的安全字段规则；设备标识使用 hash，错误仅保留安全类别/SQLSTATE。
迁移与管理员 CLI 仅写 stderr，不与常驻服务争用轮转文件。

## 验证

根据改动运行相关测试与契约检查；常用仓库检查：`make verify`、`make docs-tools`、`make verify-contracts`。真实 PostgreSQL/Timescale 集成测试使用隔离的 `IOLINK_TEST_PG_DSN`；未设置时跳过不算通过。只在具备相应凭据、设备或外部服务证据时报告外部验收通过；否则标为 `external_blocked`。

每个阶段都必须覆盖其需求的正向与错误路径、契约检查、集成测试和可用的真实链路，并把命令、输出和环境写入证据文件；缺少凭据、设备或外部服务时标为 `external_blocked`，不得把 skipped 或计划命令写成 passed。只有两位未参与该次编辑的独立审阅者针对同一工作树或发布快照明确通过，才可把阶段记为完成。完整需求细节与状态仍以 `docs/` 权威文档为准，本文件只给全局门槛，不复制 Rxx 台账。

## 自主推进与里程碑

已有任务清单时，按照权威文档中的清单顺序和依赖自主执行，持续完成开发、验证、修复与独立双审，不在任务之间暂停等待用户重复确认。每个小功能里程碑闭环后，合并到当前路径的 `main`，打 tag 并 push；报告 `docs/IMPLEMENTED.md` 中完成的位置，然后直接开始下一项。CI 仅由新 tag 触发。尽可能使用子代理并行推进可并行的工作，明确文件归属，避免互相覆盖。

用户明确要求暂停、取消或改变范围时遵循最新指令。确实缺少凭据、设备或外部服务时记录 `external_blocked`，继续不依赖该条件的任务；不得把未完成工作记为完成，也不得把已有授权的常规操作重新交给用户确认。
