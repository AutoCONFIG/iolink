# 文档地图与使用规则

2026-09-19 重建实施基线。用户确认 M0–M8 全部纳入，按阶段实施。需求基线已双审通过，正在分阶段实施；文档通过不等于实现通过。实施记录见 [M0证据](evidence/2026-09-19/M0/README.md)、[M1证据](evidence/2026-09-19/M1/README.md)。

权威关系：用户范围 → PLAN/设计附件 → 版本契约 → ACCEPTANCE验收。代码是实现证据，不能覆盖需求；文档冲突必须修订并复审。状态仅由 IMPLEMENTED维护。

| 文件 | 用途/阅读顺序 |
|---|---|
| [PLAN.md](PLAN.md) | 先读：范围、阶段、依赖、质量目标 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | 每项R01–R55的验收与证据要求 |
| [IMPLEMENTED.md](IMPLEMENTED.md) | 当前状态、已执行检查、已知差异；不继承旧勾选 |
| [PLAN-DETAILS.md](PLAN-DETAILS.md) | M0–M5身份、数据、报警、接口和迁移目标 |
| [EXTENSIONS.md](EXTENSIONS.md) | M6–M8产品模型/权限/授权/视频/协议/任务，外部输入清单 |
| [api/admin-openapi.yaml](api/admin-openapi.yaml)、[api/openapi.yaml](api/openapi.yaml) | M0–M5目标契约，已标待实现；扩展契约在各阶段入口补齐后复核 |
| [mqtt-spec.md](mqtt-spec.md) | 基础MQTT及命令、网关阶段目标 |
| [schema.sql](schema.sql) | 遗留初始化脚本，有已登记缺陷；不是新目标DDL或升级入口 |
| [FRONTEND-HANDOVER.md](FRONTEND-HANDOVER.md) | 页面ID、交互与联调关卡 |
| [DEPLOY.md](DEPLOY.md) | 安装、TLS、备份恢复的目标流程及当前限制 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 契约先行、测试与证据规范 |
| [FEATURE-MATRIX.md](FEATURE-MATRIX.md)、[FEATURE-CHECKLIST.md](FEATURE-CHECKLIST.md) | 功能到Rxx的速查，不重复维护完成计数 |
| [PLAN-REVIEW-FINAL.md](PLAN-REVIEW-FINAL.md) | 当前修订版双审过程与结论 |
| [PLAN-REVIEW-2026-09-19.md](PLAN-REVIEW-2026-09-19.md) | 初次双审未通过记录，行号对应原始commit，不随改文档重写历史 |

原始需求附件、baseline-spike-report.md、proposal-baseline-final.md 未在仓库文档中找到，不设置假链接或当作已存在证据。历史方案可从Git历史寻找，当前基线以用户本次指令和上述重建文档为准。upstream/README 是参考材料，其旧基座候选描述不属于本项目当前决策。
