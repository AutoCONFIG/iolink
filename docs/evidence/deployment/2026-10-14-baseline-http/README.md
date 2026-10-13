# 既有35操作真实HTTP契约与故障修复

2026-10-14，基线 `4fffc4b`（v0.0.36）。本轮只验证原管理25+用户10操作的当前契约及相关输入、权限、存储错误路径；当前全部96操作及R02整体尚未完成验收。

## 已复现与修复

- scoped farm owner SQL参数类型导致合法创建500；显式bigint绑定。
- 管理写请求接受未知/重复/大小写字段、尾随JSON；严格单对象解码及DTO范围校验，拒绝无写入。登录亦在外部交换/查询前拒绝非法对象。
- 池塘更新忽略farm_id；同事务锁定授权池塘并验证原farm_id，不支持跨farm迁移。创建池塘补齐既有Pond响应中的状态、设备数和latest。
- 用户登录只返回公开id/nickname；设备列表返回当前DevicePage，真实SQL分页/总数、名称、状态/停用和资源归属过滤。
- 零/负ID、分页溢出等400；不存在资源404；真实provider失败500/internal_error及既有安全关联诊断。登录查询与旧密码升级失败亦不签发token。
- enabled:false以前被CreateRule SQL忽略；现在明确存入，测试验证停用期间不报警，启用后正常报警。原直接调用CreateRule的测试fixture明确Enabled:true，保留原业务断言。
- 文档补声明既有角色403，删除重复404键，保留权限规则。新增schema检查器通过stdin接收真实响应，输出仅schema关键词/位置，禁止泄露实际数据；拒绝非有限数字与has_body矛盾。

## 方法与环境

真实TCP httptest服务器、生产handler/core、专属Timescale PG16.15/2.30.2（容器`iolink-http-contract-pg-20261013`、网络`iolink-http-contract-20261013`、卷`iolink-http-contract-data-20261013`均label `iolink.task=http-contract-20261013`；127.0.0.1:33127）。每个测试自行建/删独立数据库，测试DSN由环境传入、不归档口令。实际微信交换以受控测试身份替代，报警经实际ingestion生成；不是外部微信或License交付验收。

命令：`GIN_MODE=release IOLINK_TEST_PG_DSN=<专属DSN> IOLINK_CONTRACT_PYTHON=<contracts venv Python> go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestBaseline'`；静态契约`make verify-contracts`，全库`make verify`，Python验证器/门禁测试用contracts venv运行。

before文件均是已执行失败的记录；admin-login-before中的故障fixture token已替换为`<test-token-redacted>`，保留原HTTP状态和失败判断。早期admin fixture缺少permission policy的运行不作为产品缺陷证据。`boundary-typecheck.log`明确严格类型检查12项诊断未通过（动态SDK/旧resolve类型及必备assert_never不可达分支），没有屏蔽；Python29测试、基础Ruff及no-excuse通过不等于严格类型通过。

本候选排除用户AGENTS和未发布playback草稿；app server只选登录及共享身份错误处理hunk。待最终源导出全库结果和同快照独立双审后再记完成。浏览器由用户验收，硬件/真实微信/License交付/100GiB恢复/长期容量及暂缓新功能不计本次通过。

## 首次审阅与修订

初始树 `dffad35ac6a5adc871621929c5a2bea22d9e724e` 的真实源导出全库33包build/vet/race通过（终端91217 exit0）、focus22顶层无SKIP（60451 exit0）、Python110（75708 exit0）、96操作/584合成fixture通过。但门审明确拒绝：Common responses描述的固定错误码未被原字符串schema约束，旧文字被错误接收。该组 `snapshot-*.log` 是修订前有效软件结果，不是最终发布批准。

修订将共同响应的既定错误码写成各响应的enum（generic Error和License专用错误保持其范围），规范原接口的400/401/403/404/409文字；`error-code-before.log` 真实失败后修复。审阅还指出非法查询转义被URL.Query丢弃、以及微信上游故障被当401/非2xx可能被当成功；`query-before.log`、`app-query-before.log`、`wechat-before.log` 记录实际失败。现在显式检查ParseQuery；只有已识别无效凭证为401，非2xx/解码/网络/缺openid/服务端失败为502/upstream_error，无用户写入，安全诊断请求ID一致。受控HTTP微信提供者不是真实微信验收。

首次导出未带未改web子模块的dist，导致embed/一次focus编译失败；`snapshot-*-prerequisite-failed.log` 保留。补入web gitlink `39299dfb0d7771690117a2765fb766a9b715f554` 的当前未改dist（index SHA256 `a2cf487d2b7cd9c2486ae55e386d12f519fbfaa99fb1dab4c21a1817dd2ee632`）后重跑通过；一次Python选择器文件名拼错也保留为selector-failed，不计通过。worker unit日志来自共享树（含原playback草稿）只作过程回归，最终源证据以修订后导出命令及双审为准。
