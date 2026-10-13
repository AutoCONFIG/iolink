# 既有23扩展HTTP操作与认证/输入边界

基线v0.0.37 / `d4d7e02`；2026-10-14。范围：平台/组织/成员11操作，普通用户身份/v2的5操作，产品/模型7操作。它们已经实现，当前只修复与验证已有行为；原35接口的共享认证与两个空过滤值亦纳入。整个96操作、R02及外部阶段不因本批通过而完成。

## 已执行的过程证据

- platform/：真实TCP/PG11正向、首次仅有平台ADMIN且无业务租户、注册USER不自动获租户、撤权后原JWT立即失效；先复现非法JSON写入/非正ID错类/真实SQL被401掩盖/漏400403500声明，修复后6顶层race/shuffle、无SKIP通过。8真实关系故障/8缺少可选能力路径检查实际状态、无部分写入、同请求ID安全SQLSTATE/能力诊断。初始一个未挂载user平台路径属于fixture错误，corrected记录去除该错误，不列产品缺陷。
- app/：5当前契约操作，57真实TCP响应；有效published模型、固定device版本/可读字段、跨租户/失效成员/模型隐藏及空集合。尾随遥测JSON以前实际提交数据库；现在整请求严格解析并拒绝，包含重复top/nested字段、类型/未知/尾随及malformed query。6SQL故障含telemetry+shadow之后alarm_rules失败，整个业务状态回滚。真实微信交换采用受控身份，不是外部微信验收。
- products/：7正向操作、产品/模型strict非法输入零写入、外国资源/未发布模型/冲突、缺凭据/只读角色/旧token撤销，7真实SQL失败安全诊断，audit失败原子回滚，发布/分配幂等、固定device model version、实际generic repo过滤不可读字段。首轮fixture多语句prepared参数和viewer缺farm grant分别纠正，不把它们当产品缺陷。
- identity-provider-before.log：真实users/tenants/tenant_memberships关系缺失，admin/app共6身份认证面先错误401；现在仅未知/失效身份401，实际存储错误500。NoRows在core seam转换domain sentinel；HTTP abort后不继续业务handler，摄像头保留code/message形状。
- empty-filter-before.log：两个可选数值filter显式空值先返回200；现在400。
- tenant-unicode-before.log：合法128个Unicode字符的组织名先被当128字节拒绝；改按字符计数、129字符仍拒绝，不留下写入。

以上worker日志来自共享工作树，后续发布必须用排除playback/AGENTS草稿的冻结源码导出重跑并同快照独立双审，不能以过程日志替代最终身份绑定。受控Fixture口令/签名材料仅在内存/环境；响应不进入验证器argv或磁盘，失败诊断不输出token/body。缺少能力只记真实缺失，不制造err。

## 环境、命令与限制

专属Timescale PG16.15/2.30.2，127.0.0.1:33127；容器`iolink-http-contract-pg-20261013`、网络`iolink-http-contract-20261013`、卷`iolink-http-contract-data-20261013`均label `iolink.task=http-contract-20261013`。每测试独立建/删数据库。使用 `GIN_MODE=release IOLINK_TEST_PG_DSN=<专属DSN> IOLINK_CONTRACT_PYTHON=<主仓contracts venv绝对路径> go test -race -shuffle=on -count=1 -v ./internal/core`，各日志选择器限定其目标函数；全库以最终`make verify`为准，静态`make verify-contracts`为合成检查。

待最终源回归与独立双审，不记完成。APIKey/Open/restore/初始化与camera已实现面将继续；新GB/media/playback、真实设备/微信、License交付专项、浏览器、长期容量/大恢复仍单列，不冒称通过。
