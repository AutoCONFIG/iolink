# IoLink 智慧水产监测平台

传感器/终端 → iolinkd → 管理后台与微信小程序。核心采用 Go 单仓、内嵌 MQTT、PostgreSQL 16 + TimescaleDB；管理前端目标为 Vue3/TypeScript/Element Plus，小程序为 uniapp。

**当前按已通过双审的基线分阶段实施，不能按旧文档认定项目全部完成。** 已有接入、存储、报警和双端 API 代码；管理前端仍是占位页，小程序未交付；M0/M1已补迁移、安全首启、接入和数据事务；归属/隔离等问题仍待M2修复。现状及证据见 [IMPLEMENTED](docs/IMPLEMENTED.md)。

M0–M8 全部纳入，按阶段完成和验收。核心业务以单二进制交付；数据库、TLS反代及 M7 流媒体是独立配套服务，小程序单独构建发布。ThingsPanel 等 upstream 项目只作参考，不计作本项目实现。

## 文档入口

- [文档地图](docs/README.md)：权威关系、阅读顺序。
- [实施计划](docs/PLAN.md)：范围、架构、阶段依赖与关卡。
- [逐项验收台账](docs/ACCEPTANCE.md)：R01–R55，成功/失败/权限/外部验证。
- [基础设计](docs/PLAN-DETAILS.md) / [扩展设计](docs/EXTENSIONS.md)：M0–M8目标语义。
- [管理API](docs/api/admin-openapi.yaml) / [小程序API](docs/api/openapi.yaml) / [MQTT](docs/mqtt-spec.md)：待实现的目标契约，不是当前代码的行为保证。
- [前端交接](docs/FRONTEND-HANDOVER.md) / [部署目标](docs/DEPLOY.md) / [协作规范](docs/CONTRIBUTING.md)。
- [初次双审记录](docs/PLAN-REVIEW-2026-09-19.md)：原始计划未通过的证据。

## 仓库

`cmd/iolinkd` 组装服务；`internal/access` 接入；`internal/core` 数据与报警；`internal/appapi`、`internal/adminapi` 双API；`internal/domain` 共享模型；`internal/platform` 配置；`web/admin/dist` 当前为占位页；`deploy` 为待整改部署脚本。

## 当前可做的检查

```bash
make verify
make docs-tools
make verify-contracts
```

接口测试需要本机监听权限。M0已修复统一验证入口，并新增真实Timescale迁移测试、CLI/真实MQTT冒烟脚本；证据见 [M0验收记录](docs/evidence/2026-09-19/M0/README.md)。真实数据库测试需要设置 `IOLINK_TEST_PG_DSN` 到隔离实例（测试角色须有创建数据库权限），未设置时明确跳过，不能计作通过。GitLab CI已配置，远程流水线尚未执行。M1软件链路已通过修复后的双审，见 [M1验收记录](docs/evidence/2026-09-19/M1/README.md)。首次安装命令见 [部署说明](docs/DEPLOY.md)。TLS、恢复、容量和完整业务功能仍待对应阶段验收。
