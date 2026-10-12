# R05 变更：MQTT 接入对齐 ThingsPanel 签发式凭据（2026-10-12）

## 需求来源

用户要求"严格对齐 ThingsPanel 的方式"：注册时平台直接签发可复制粘贴的接入三元组（建议 Client ID + 生成的 Username + Password），Username 不再使用裸设备编号；且后续提交推送 GitHub 远端而非 git.hyhy.fun。

## 契约变更（已同步权威文档）

- `docs/mqtt-spec.md` M1：接入凭据为 ThingsPanel 式签发三元组——Username=注册时签发的接入用户名（`iolink-<device_no>-<6hex>`，devices.mqtt_username 明文存储、控制台可随时查看），Password=注册时返回的 secret（32 字节随机、64hex，仅显示一次），ClientID 任意唯一（注册弹窗给出建议值）。broker 按 username 反查设备（devices.mqtt_username），解析出的 device_no 仍是 topic 与 ACL 键。清理会话、同 device_no 单活动会话、踢除旧连接等语义不变（沿用同日 v0.0.29 语义）。
- `docs/PLAN-DETAILS.md` §3、`docs/ACCEPTANCE.md` R05 同步更新；`docs/schema.sql` 基线加入 mqtt_username 列。

## 实现范围

- 迁移 `internal/migrate/sql/016_device_mqtt_username.sql`：加列 + 唯一约束 + 存量设备回填（回填用户名可在控制台查看）。
- `internal/core`：RegisterDevice 签发 username（唯一冲突重试）；`Resolve(username, password)`（按 mqtt_username 反查设备 + secret 恒时比对 + 停用/租户停用拒绝）替换 `Authenticate(deviceNo, secret)`；设备列表/详情 SELECT 透出 mqtt_username。
- `internal/access`：`Authenticator` 接口改为 `Resolve(username, password) (deviceNo, ok)`；`Server.deviceOf` 维护活跃连接→device_no 映射（会话建立时二次 Resolve 后写入，断开时删除）；OnACLCheck/OnPublish/isCurrent 全部基于映射中的 device_no；连接认证、清理会话强制、Will 拒绝、DeviceRevision fail-closed、retireClient 踢除逻辑保持 v0.0.29 语义。
- `internal/adminapi`：注册响应新增 `mqtt_username` 与建议 `client_id`（crypto/rand 生成 `mqtt_<8数字>-<3数字>`）；列表/详情响应透出 `mqtt_username`。
- web（子模块 iolink-webui）：注册弹窗改为 ThingsPanel 式三元组展示（Client ID/Username/Password 三行 + 独立复制按钮 + topic 说明），设备详情抽屉新增"MQTT 接入"区块（Username 可随时查看/复制）。
- `cmd/mqtt-sim`、`cmd/mqtt-once`：新增 `-username` 必填参数（-device 仍用于 topic）；`scripts/vdevice.py`：`--username` 必填、`--device` 可选（从 username 解析设备号），Client ID 默认自动生成。

## 独立双审（第一轮：安全视角 PASS、契约测试视角 FAIL→已修复）

- 契约审阅 P1-1：Go 签发后缀误为 12 hex（6 字节），与 docs/016 迁移/vdevice/demo 的 6 hex 声明不一致 → 已统一为 3 字节（6 hex 字符）。
- 契约审阅 P1-2：`docs/api/admin-openapi.yaml` 版本契约未同步（注册响应与 Device/DeviceDetail 为 additionalProperties:false 且缺少 mqtt_username/client_id）→ 已补全三处 schema 并通过 YAML 解析校验。
- 契约审阅 P2：注册响应测试补齐 mqtt_username/client_id 断言；真实 broker 集成补"未知 username 拒绝"用例；IMPLEMENTED.md 台账补记。
- 安全审阅 P2：connected() 二次 Resolve 失败时主动 DisconnectClient 消除"已 CONNACK 但全 ACL 拒绝"的僵尸连接（已修）；016 回填格式与签发格式统一（已修）；digits() 错误显式传播为 500（已修）。安全审阅确认：锁序单向（ss.mu→s.mu）、deviceOf 按客户端指针键控无新旧混淆、Resolve 租户隔离与 UNIQUE 兜底、fail-closed 全链路成立。

## 验证命令与结果（工作树：main @ 本地，2026-10-12）

- `go build ./... && go vet ./...` → 通过。
- `make verify` → 34 ok，无 FAIL。
- 真实 PostgreSQL/Timescale 集成（隔离容器 `timescale/timescaledb:latest-pg16` @127.0.0.1:15433/15434，`IOLINK_TEST_PG_DSN`，用后已删）：
  - `go test ./internal/core/ -run TestRealMQTTAuthenticationACLAndPersistence -v -count=1` → PASS 1.27s / 修复后 1.15s（真实 broker 上以签发型 username 连接：遗嘱 v4/v5 拒绝、错 secret 拒绝、未知 username 拒绝、generic ClientID 可连、同设备新会话踢除旧连接、ACL/伪造 topic 拒绝、QoS1 入库+报警、message_id 去重、MQTT5 裸包、停用拒绝）。
  - `go test ./internal/... -count=1`（带 DSN）→ 32 个含测试包全部 ok（core 121s、camerapg 50s 等真实链路执行），0 失败。
- `cd web && npm run build` → 成功；`python3 -m py_compile scripts/vdevice.py` → 通过。
- `docs/api/admin-openapi.yaml` → python yaml.safe_load 解析通过。

## 第二轮复核（两位审阅者均 PASS）与收尾

- 契约审阅复核：OpenAPI 三处 schema 与 handler 实际响应键集合严格一致（required==properties==handler，check_contracts.py 96 操作/568 fixture PASS）；016 回填实测形状正确；新增断言/用例与实现吻合。
- 收尾修复：cmd/mqtt-sim、cmd/mqtt-once 帮助文本示例改为真实签发形状（iolink-dev-abc12345-abcdef）；新增 TestResolveFailureAtEstablishmentFailsClosed（僵尸连接分支 fail-closed 单测，-race 通过）。
- 记录为后续项：016 存量库回填分支的仓库内 DSN 用例（本轮由两位审阅者真实 PG 探针独立验证通过，行为正确）。

## 部署与端到端验证（rsplab.hyhy.fun，2026-10-12，最终态）

- tag v0.0.30 推 GitHub（AutoCONFIG/iolink），GitHub Actions CI 产出 `ghcr.io/autoconfig/iolink:v0.0.30` 与 `latest`；测试环境 `docker compose pull && up -d` 完成部署，`/readyz` ok。迁移 016 在存量库自动应用：现有设备 dev-752a4ede 回填签发用户名 `iolink-dev-752a4ede-c95ac9`（6hex 形状正确）。
- 从本机走外部路径（rsplab.hyhy.fun:1183，MQTT 3.1.1，paho）验证：
  1. 临时设备 dev-vnode04 签发 username `iolink-dev-vnode04-e2e01`（产品 water-quality）；
  2. 旧形式 username=device_no + secret → **CONNECT 被拒**（严格签发型生效）；
  3. 签发型 username + 任意 ClientID → **连接成功**；
  4. QoS1 发布 `iolink/up/dev-vnode04/properties` → PUBACK 确认；
  5. sensor_data 入库 1 行、设备状态 online；
  6. 临时设备已删除，无残留。
- 状态：deployed & verified。用户侧切换指引：MQTTX Username 改为设备详情页"MQTT 接入"区块显示的签发用户名（dev-752a4ede 为 `iolink-dev-752a4ede-c95ac9`），Password 不变。
