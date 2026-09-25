# M1 软件链路实施与双审

日期：2026-09-19。需求基线：2026-09-19修订版ACCEPTANCE，R02.b/R05–R13（R10仅a）；工作树未提交，快照见manifest.json，历史M0快照不改写。环境沿用专用tmpfs Timescale测试容器，版本见[environment.json](../M0/environment.json)：Go1.26.8、PG16.15、Timescale2.30.0。所有测试只创建并清理自有临时数据库。

## 逐项结果

| 需求 | 结果 | 证据与边界 |
|---|---|---|
| R02.b、R05–R06 | 软件验收通过 | 实际MQTT3.1.1和5连接；合法/错secret/ClientID不匹配/停用；新会话替换旧会话；自身发布/订阅，跨设备/通配符/伪造子设备拒绝；恶意跨设备retained Will在CONNECT阶段拒绝；失败认证不产生online |
| R07 | 软件验收通过 | 七字段逐一上下界和越界、null/空对象/类型/坏JSON/超64KiB、未知精确字段与大小写覆盖反例；全无有效值不写遥测；实际MQTT验证未知字段不覆盖合法值、超大包不入库 |
| R08 | 软件验收通过 | 确定性时钟验证60/300秒×3边界、超时一次离线、有效空heartbeat恢复、重复断开不重复；慢设备不阻塞其他设备和Run取消；实际连接接管及核心重启在线数归零；周期全局继承/设备覆盖 |
| R09 | 软件验收通过 | 真实遥测/塘快照；24h message_id窗口重复、过期及无ID至少一次；注入outbox INSERT失败，遥测/影子/报警/outbox/去重键全回滚，移除故障后可重试 |
| R10.a | 软件验收通过 | 部分属性合并、逐字段时间、迟到样本不覆盖新值；换塘后旧影子不可读，新样本不混入旧塘值；report_interval输出；R10.b调塘API和完整双端响应仍待M2 |
| R11 | 软件验收通过 | 真实1440行历史数据聚合1/200点上限、升序/空[]/注入指标拒绝；HTTP+真实DB验证unit/空历史/非法metric/range/max_points；today按上海业务日边界。该HTTP测试显式注入测试身份交换器，不是真微信验收，M2历史资源授权仍待完成 |
| R12–R13 | 软件验收通过 | min/max实际命中端、等于不报警、非法倒置阈值拒绝；100并发只有一未确认报警和outbox；A塘旧报警不阻止B塘新报警，确认幂等后可再次触发；真实MQTT→阈值报警亦通过 |

## 命令和记录

- [verify.txt](verify.txt)：`IOLINK_TEST_PG_DSN=<隔离实例URL> make verify`，build/vet/全部Go测试通过，数据库测试未skip。
- [cases.txt](cases.txt)：相同隔离变量下 `go test ./internal/access ./internal/core ./internal/appapi ./internal/adminapi ./internal/migrate -count=1 -v`，逐用例记录。
- [database-and-protocol.txt](database-and-protocol.txt)：最后补齐真实HTTP历史和MQTT阈值链路后的 `go test ./internal/core -count=1 -v`；与fake API测试区分。
- [race.txt](race.txt)：相同隔离变量下 `go test -race ./internal/access ./internal/core -count=1`，通过。
- [cli-regression.txt](cli-regression.txt)：重建实际iolinkd后再跑scripts/smoke_m0.py，迁移/首启/真实MQTT/管理latest/重启/SIGTERM回归通过。

预期/实际均一致；故障用例预期拒绝或整体回滚而不是退出零。MQTT3越权发布可由broker直接断开，负面用例结束后重新建立连接，不把断线混淆为后续合法报文失败。没有真实硬件、微信、TLS、远程CI或容量演练的通过结论。

## 双审过程

审阅员 `/root/plan_review_a` 与 `/root/plan_review_b`，均未编辑本阶段业务代码。

- A第一轮不通过：全局锁跨数据库、未知属性大小写绕过、密钥SQL非恒时比较。
- B第一轮不通过：CONNECT遗嘱绕过发布ACL、全局锁、未知属性；另提醒notifier装配顺序。
- 修复：按设备串行且全局索引锁不跨I/O；周期缓存/最多16看门狗worker；精确白名单后解码；拒绝Will；摘要格式校验+恒时比较；监听前装配notifier。全部增加针对性测试。
- A第二轮明确“通过，无剩余阻断项”；B第二轮明确“通过，未发现剩余阻断项”。两审阅读真实测试证据；A独立运行access单测，两审均核对diff格式。真实DB/MQTT由主执行者执行，不冒称两审各自重复实测。

M1软件通过不表示全项目完成。M2仍需修复归属/全资源授权、登录升级/令牌撤销、设备调塘停用API及35个HTTP目标操作；通知outbox在本阶段只保证与报警原子落库，发送重试/状态/真实送达在M5；M3–M8其余功能仍按既定阶段实施。

## 最终快照与清理

最终追加：解码前拒绝原始非UTF-8字节，防止标准库替换非法采样ID；见[final-json-regression.txt](final-json-regression.txt)。A/B再次明确通过这一追加，两位均核对83文件清单全部匹配；A另独立重跑access测试通过。最终manifest文件本身SHA256为 `8fba8cca79ab322f73cc1d786a7285ace5cd843b1304004a44d5fe344e35d41d`（对磁盘JSON文件字节计算，区别于重序列化JSON摘要）。

验收结束后已核对测试容器标签与tmpfs挂载，仅删除本轮自建 `iolink-m0-audit-20260919`，既有业务容器未修改。复跑时需重建隔离实例并更新测试DSN；日志没有依赖仍运行的测试服务。
