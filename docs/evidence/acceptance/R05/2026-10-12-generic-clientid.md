# R05 变更：MQTT 接入改为通用 ClientID 形式（2026-10-12）

## 需求来源

用户要求支持通用 MQTT 接入（任意 Client ID + Username/Password），不再要求 ClientID=username=device_no。身份仍完全由 username(device_no)+password(secret) 决定，安全语义不变。

## 契约变更（已同步权威文档）

- `docs/mqtt-spec.md` M1：ClientID 为任意唯一传输会话键（1–128 字节 UTF-8，不含通配符），不参与鉴权；必须清理会话（3.1.1 CleanSession=1；MQTT 5 Clean Start=1 且 Session Expiry Interval=0）；同一 device_no 新合法连接按 username 替换并踢除旧连接（Session Taken Over）。
- `docs/PLAN-DETAILS.md` §3 与 `docs/ACCEPTANCE.md` R05 同步更新（含删除旧"三元组"残留句）。

安全动机：持久会话以 ClientID 为键，其他设备可用自己的合法凭据配他人 ClientID 恢复会话，继承其订阅与未确认 QoS1 下行消息（ResendInflightMessages 不经过 ACL）；强制清理会话（含 v5 Clean Start=0 拒绝）后 ClientID 冲突仅为标准接管，无跨设备状态泄露。

## 实现范围

- `internal/access/auth.go`：新增 `ValidClientID`（非空、≤128 字节、UTF-8、无 `+`/`#`）。
- `internal/access/broker.go` `OnConnectAuthenticate`：移除 `cl.ID != username` 拒绝；校验 ClientID 语法；强制清理会话（v3 Clean=1；v5 CleanStart=1 且 Session Expiry Interval=0，堵死 Clean Start=0 的 inheritClientSession 会话继承）；认证与 DeviceRevision fail-closed 不变；WillFlag 仍拒绝。
- `internal/access/access.go`：`connected()` 对同 device_no 旧客户端异步踢除（`retireClient`：先发 Session Taken Over DISCONNECT，写阻塞超 5s 则 `Stop()` 强制关 TCP，防止 keepalive=0 停滞对端导致 goroutine/fd 泄漏）。
- 模拟器 `cmd/mqtt-sim`、`cmd/mqtt-once` 沿用 ClientID=device_no，在新规则下仍有效（向后兼容，实测通过）。
- 管理端 `web/src/views/DevicesView.vue`（子模块 iolink-webui 提交 a84e8f7）：注册弹窗展示完整接入参数（Username=device_no、Password=secret、ClientID 任意唯一、清理会话、端口 1883/8883）。

## 独立双审（第一轮：两位独立审阅者均判 FAIL，问题已全部修复；第二轮复核：两位均 PASS）

- 安全审阅者 P1（PoC 实证）：v5 Clean Start=0 + 同 ClientID 可继承他人订阅与未确认下行，绕过 ACL → 已改为 v5 强制 CleanStart=1（复核含 PoC 复跑：CONNACK 134 拒绝，继承路径不可达）。
- 集成审阅者 P1（真实 PG 实证）：`internal/core/mqtt_integration_test.go` 残留旧契约断言（ClientID≠username 拒绝）→ 已重写为"generic ClientID 可连 + 同设备旧连接被踢除"断言（两位审阅者各自用隔离容器独立复现 PASS）。
- P2：踢除 goroutine 无限阻塞风险（已加 5s 超时强制关连接）；单测矩阵缺空 ClientID / v5 显式 expiry=0 / v5 CleanStart=0 项（已补）；PLAN-DETAILS 残留"三元组"句（已删）；旧连接被替换与下行投递之间的瞬态窗口（异步踢除固有，数据面受 isCurrent ACL 保护，接受并记录）。

## 验证命令与结果（工作树：main @ 本地，2026-10-12）

- `go test ./internal/access/` → ok 0.004s
  - `TestConnectAuthGenericClientID`：任意 ClientID（含 `mqtt_20842687-727`、中文）接受；空/含分隔符 username、通配符/超长/非 UTF-8/空 ClientID、v3 持久会话、v5 CleanStart=0（含带零 expiry 变体）、v5 Session Expiry>0、WillFlag、错 secret、revision 不可用均拒绝；v5 CleanStart=1+显式 expiry=0 接受。
  - `TestAuthenticatedLifecycleWatchdogAndTakeover`（1m/5m）：watchdog、心跳恢复、替换会话、重复断开幂等保持。
- 真实 PostgreSQL/Timescale 集成（隔离容器 `timescale/timescaledb:latest-pg16`，`IOLINK_TEST_PG_DSN=postgres://iolink:***@127.0.0.1:15432/iolink?sslmode=disable`，用后已删除容器）：
  - `go test ./internal/core/ -run TestRealMQTTAuthenticationACLAndPersistence -v -count=1` → PASS 1.97s（遗嘱 v4/v5 拒绝、错 secret 拒绝、generic-1 ClientID 连接成功且同设备旧连接收到断开、ACL/伪造 topic 拒绝、QoS1 上报入库+报警、message_id 去重、MQTT5 裸包上报、停用设备拒绝）。
  - `go test ./internal/... -count=1`（带 DSN）→ 含测试的包全部 ok，0 失败 0 意外 skip（core 139s、camerapg 66.8s、license 66.8s 等真实链路均执行；两位审阅者各自独立复现亦全部 ok）。
- `make verify`（无 DSN）→ 通过（34 ok，无 FAIL）。
- `cd web && npm run build` → 构建成功。

## 部署与端到端验证（rsplab.hyhy.fun，2026-10-12）

- git.hyhy.fun 当日宕机（Cloudflare 522/521），tag v0.0.29 暂无法推送、CI 镜像暂不可用（external_blocked，恢复后补推并切回 ghcr 正式镜像）。作为替代，本地以 `git archive` 导出提交 713306a 干净快照（不含工作树其他在途修改）+ web 子模块 a84e8f7，`docker build` 出 `ghcr.io/autoconfig/iolink:r05-test` 并 `docker load` 到测试服务器部署（compose override 屏蔽 watchtower，验证期临时措施）。
- 部署后 `/readyz` ok，随后以真实 MQTT 客户端（paho，MQTT 3.1.1，QoS1，clean session）在服务器本机 127.0.0.1:1183 验证：
  1. 临时设备 dev-r05e2e01（数据库直建，sha256(secret_hash)，用后即删）；
  2. 任意 ClientID `mqtt_r05e2e-727` + username=device_no + password=secret → CONNACK rc=0；
  3. devices.status 落库为 online；
  4. 错误密码 → CONNACK rc=5 拒绝；
  5. 第二连接（另一任意 ClientID `mqtt_r05e2e-second`）建立后，旧连接立即收到断开（Session Taken Over 替换）；
  6. 断开后状态转 offline；
  7. 临时设备行已删除，无残留。
- 状态：deployed & verified（最终态）。origin git.hyhy.fun 当日宕机（Cloudflare 521/522，external_blocked），tag 已通过同步的 GitHub 远端（AutoCONFIG/iolink）推送，GitHub Actions CI 产出 `ghcr.io/autoconfig/iolink:v0.0.29` 与 `latest`；测试环境已 `docker compose pull && up -d` 切回正式镜像并二轮冒烟复验（同上 7 步全部通过），watchtower 屏蔽已移除、本地构建镜像与 /root/iolink-src-r05 源码目录已删除。origin 的补推在后台持续重试。
