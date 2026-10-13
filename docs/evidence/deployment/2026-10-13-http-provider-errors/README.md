# 既有 HTTP 500 响应与诊断安全

2026-10-13开始、2026-10-14继续；基线 `9c63186`。本组是R02.c真实HTTP矩阵的故障响应子步骤，不是全部35操作或96扩展契约验收。

管理及用户既有handler中的原始provider错误和自定义500文字改为契约 `{"error":"internal_error"}`。实际错误通过注入logger、常量消息/阶段http_provider、服务器请求ID记录安全类别；缺少能力只记真实缺失，不伪造error。既有4xx/License错误映射和latest专用诊断保留；未改数据库、前端或新业务功能。

- `admin-before.log` / `app-before.log`：37与18个真实认证TCP HTTP故障场景先失败，展示旧响应含虚构凭据URI/非契约文字，或已正确500缺少内部诊断。所有URI/凭据标记均为故障fixture，非实际凭据。
- `admin-after.log` / `app-after.log`：同55场景race通过；响应只有internal_error，dump为单条同请求ID的ERROR及安全错误类别，原始URI、请求标记、密码及JWT不出现。
- `real-pg-http.log`：真实专属Timescale中的farms/alarms/alarm_rules逐个临时不可查询；两个已认证HTTP接口共7个读取面均500/internal_error，实际postgres_42P01与响应请求ID关联，表名/口令/JWT不进dump。每个表恢复，测试数据库由testdb清理。

专属容器 `iolink-http-contract-pg-20261013`，label `iolink.task=http-contract-20261013`；专属网络 `iolink-http-contract-20261013`、卷 `iolink-http-contract-data-20261013` 同label，端口127.0.0.1:33127，Timescale latest-pg16（PG16.15/TS2.30.2）。测试DSN只从环境传入，不归档凭据。相关测试和独立审阅结束后核对归属清理。

冻结发布快照排除原playback草稿和用户AGENTS修改；app server.go只选择两个login错误处理hunk。后续全库、契约及同快照独立双审尚待完成。浏览器由用户验收，真实微信/硬件/License交付与后续M7/M8功能不计本组通过。
