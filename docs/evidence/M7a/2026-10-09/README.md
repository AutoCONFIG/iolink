# M7a 契约设计验证

日期：2026-10-09。范围：R43设计与R36.b/R37.b/R39.b入口要求，**不是视频功能验收**。
实现基线 root e7be77d / web 953335e；本候选包含专项设计、22操作OpenAPI、
DDL提案、部署/页面/负面场景矩阵和静态校验器请求/响应方向修正。
双审快照将由本证据所在提交绑定，审阅报告归档于同目录 reviews/。
尚未新增013迁移或任何视频生产handler/provider，也未修改可运行Compose。

## 已执行

| 命令/环境 | 实际结果 |
|---|---|
| `make verify-contracts`，既有 .venv/contracts 锁定依赖 | PASS，6个契约，96操作/568合成请求响应fixture |
| `.venv/contracts/bin/python -m pytest scripts/test_video_contracts.py scripts/test_m6a_contracts.py -q` | PASS，22测试（18视频/4原物模型） |
| 专用Docker `iolink-todo9-pg`，创建随机 `iolink_m7a_design_<uuid>` 数据库 | PG16.15 / Timescale2.30.0，未修改容器已有数据库 |
| 先设置 timescaledb.restoring=on、顺序执行001..012，再执行 `docs/design/video-proposal.sql` | PASS，提案可在当前schema上创建 |
| 执行 [proposal-check.sql](proposal-check.sql) | PASS，RTSP/GB正例，10类FK/互斥/TTL/状态错误和fixture事务回滚 |
| finally DROP本轮随机数据库 | 成功；无真实摄像机或生产数据参与 |

首次运行负面fixture发现通用OAS30Validator跳过writeOnly必填字段，已切换请求为
OAS30WriteValidator、响应为OAS30ReadValidator；缺RTSP URI和缺密码现在真实拒绝。
来源oneOf用互斥kind枚举表达，不用解析器脱离文档后无法定位的discriminator隐式引用。
example只用于生成测试数据，仍必须通过完整schema验证，不跳过pattern/oneOf校验。

## 数据库复现

在专用Timescale容器内，用容器配置的POSTGRES_USER创建新的随机数据库。
依次通过 `docker exec -i iolink-todo9-pg psql -X -q -v ON_ERROR_STOP=1 -U <test-user> -d <random-db>`
输入 `SET timescaledb.restoring=on;`、按文件名排序的 `internal/migrate/sql/*.sql`、
`docs/design/video-proposal.sql`、本目录 `proposal-check.sql`。
任何非零退出即失败，finally连接postgres删除自己的随机数据库。密码/DSN不写日志。

本次实际输出：

```text
DDL proposal after 001..012 PASS
valid sources, tenant/farm/channel/session FKs, source exclusivity, TTL, revocation, hash, RTP, registration PASS
fixture transaction rollback PASS
```

## 未执行/外部缺口

- 摄像机、ZLM真实媒体、GB模拟/实物、HTTP视频worker和网关：未实现，不能报passed。
- 浏览器视觉由用户验收；微信资质/真机、RTSP/GB设备、公网TLS/NAT/ACL：external_blocked。
- 双审尚待结论，设计批准后才能写生产实现，TODO11a仍正在进行，主清单完成数仍13/20。
