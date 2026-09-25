# 基础能力设计基线（M0–M5）

版本 2026-09-19，目标设计，当前差异见 IMPLEMENTED.md。需求 ID 和验收见 ACCEPTANCE.md；M6–M8 见 EXTENSIONS.md。历史原始需求附件缺失，本版不以无法核实的原文作为证据。

## 1. API、身份和归属

目标契约为 api/openapi.yaml 与 api/admin-openapi.yaml，属于待实现的约定，不代表路由当前行为。成功返回业务 JSON 或 204，失败 `{error: string}`，snake_case，UTC RFC3339；空数组返回 []，缺少最新值返回 null。字段 required 和 nullable 以契约为准，兼容变更不得悄悄删除字段。

微信 code2session 取得 openid 后幂等创建 USER。管理员账号通过独立用户名/口令登录；用户 token 和管理 token 使用不同派生密钥、不同身份 claim；拒绝错误签名、过期或缺失 exp 的 token。生产不得使用默认密钥或可接受任意 code 的 stub。管理员口令迁移到带盐慢哈希（Argon2id），首启要求自设管理员密码；旧 SHA256 账号成功登录后升级哈希并强制改初始密码，密码/口令哈希不出响应。旧密码变更后撤销该管理员已有 token（token_version）。

基础归属选择“管理员分配已注册微信用户”：

1. 微信用户先登录建档，首页未分配时显示空状态和联系管理员，不自动获取其他农场。
2. 后台用户检索只返回 id/nickname，不暴露 openid/password_hash。项目方核实微信用户身份后，管理员选定用户。
3. 创建农场可给 owner_id；省略/null 表示待分配，不默认当前管理员。PUT /farms/{id}/owner 原子更换或解除 owner；仅管理员可操作，目标必须是 USER。
4. 更换 owner 后，旧用户立即不能读/确认该农场任何数据，新 owner 获得该农场的历史及当前数据；请求在数据库授权事务中校验，不信任客户端 owner。
5. 通知接收人为发送时仍有效的 owner，未分配或解除则暂停发送；不向 internal-admin 发微信。已有待发送通知重验归属，避免转交后的历史通知发给旧 owner。
6. 操作记录操作者、原/新 owner、时间和资源 ID，不写密钥。M6 增加 tenant/membership，不改变基础“农场 owner 决定小程序数据范围”的语义。

所有详情、列表、最新、历史、报警确认必须携带 uid 对 farm/pond 的授权条件，先授权再读数据/修改，跨用户统一 404。未认证/坏 token 为 401；管理员权限不足为 403。详情、PUT和确认操作资源不存在返回404；DELETE目标已不存在幂等返回204，禁止成功空更新掩盖错误。新用户 A/B 的交叉读取与写入为 R20 必测。

## 2. 数据与迁移

目标表：users、farms、ponds、devices、device_shadows、alarm_rules、alarms、sensor_data、schema_migrations、audit_events。现有 docs/schema.sql 只是遗留初始化文件（目前缺 signal 等），不可作为生产升级程序；M0 以版本迁移替换入口。

- farms.owner_id 允许 NULL；device 当前 pond_id 必填，device_no 不可复用；增加 name、disabled_at，不做无归属设备库存。
- 遥测新增 signal INT 和 pond_id 快照；报警已经有 pond_id，影子也增加 pond_id。历史归属按采集时 pond_id 保留，农场 owner 变化对该农场历史一起生效。
- 调塘必须先断开设备并撤销现有会话，在同一事务更新 pond_id，清空最新影子，重连后按新塘归属写入；调塘前历史保留旧塘，不能通过 device_no 绕过历史所属塘授权。
- 删除设备是停用：拒绝新连接、断开会话、禁用命令和采集，保留历史、报警与审计；列表默认隐藏停用设备，include_disabled 可查询。池塘只要仍被任何设备记录（含停用）、历史、规则或报警引用就返回409；M0–M5不提供硬删/归档来绕过此限制，禁止静默级联删数据。
- 旧数据迁移以已有 device→pond 补快照，记录该回填依据及已知历史不可还原；先备份再迁移。缺少关联的孤立历史保留隔离归档，禁止猜测其他用户归属，管理员显式处理后才开放。
- 每条遥测和影子更新在同一事务；报警持久化与通知 outbox 同事务写入，通知发送在事务外。并发报警去重由部分唯一约束 `(device_no,pond_id,metric) WHERE confirmed_at IS NULL` 保证，冲突按已存在处理。
- 影子采用属性合并更新，未上报字段保留其值与各自时间；缺失和 JSON null 不等于清空。整包全无有效属性不入库且不更新数据 freshness，但有效连接心跳可更新 last_seen。
- 时间序列按 ts 建 hypertable、设备/塘/时间索引、13 个月 retention；报警/审计保留。当前 latest 接口的 ts 指最近有效上报时间，字段 freshness 必返 timestamps 对象和 report_interval（60/300秒），旧客户端忽略新字段；UI 对超过 3×interval 的字段标为过期，不能装成实时值。

迁移执行器使用数据库锁、版本和 checksum，已应用迁移不可改写；事务可执行的 DDL 原子执行，失败回滚该版，禁止带 dirty schema 启动服务。应用启动要求迁移版本匹配，不自动在多实例中争抢执行未知迁移。旧版二进制与新 schema 不兼容时禁止直接回退，执行已演练的全库恢复再回退。运行命令及恢复门槛见 DEPLOY.md。

## 3. 接入和状态

设备鉴权采用 device_no/secret；ClientID 必须等于 device_no，不允许一套凭据多个在线会话。secret 由 CSPRNG 生成 32 字节、64 字符 hex，仅注册/显式轮换返回一次；数据库存摘要，连接比对恒时。当前三元组实现有差异，按 R05 复核。

设备 MQTT 3.1.1/5.0，QoS1，上报和回执 topic 及范围见 mqtt-spec.md。认证成功后才 online；仅允许自身 properties/ack 发布、自身 cmd 订阅。拒绝跨设备 topic、通配符订阅、伪造设备名和超大包（64 KiB）。数据库写失败不得伪称入库成功；接入层须记录错误和重试策略，QoS1 传输确认不等于业务持久化承诺。基础采集接受至少一次语义，固件携带 message_id 时在设备窗口内去重；无 ID 的旧固件允许重复遥测但报警仍唯一。

全局 IOLINK_REPORT_INTERVAL 默认60秒（仅60/300），IOLINK_OFFLINE_GRACE 默认3。注册请求可选 report_interval；缺省时继承当时全局值并写入设备，详情始终返回。M1测试从设备存储加载，M2注册接口配置；M0只验全局配置。M0–M5不提供注册后单独修改周期的HTTP入口；M8通过set_interval命令成功回执后原子更新服务端周期，未确认期间按新旧周期较大值判离线，失败/过期保持原配置。固件注册时需按相同周期配置。看门狗到期仅产生一次 offline 转换，重新有效上报恢复 online；断开立即 offline；进程重启先将旧会话状态置离线，按新连接重建。在线 gauge 从当前状态集合计算，不对重复事件盲目增减。

## 4. 查询、报警与通知

- 历史范围 today/7d/30d，today 按 Asia/Shanghai 业务日边界转 UTC 查询，数据库与传输均 UTC。max_points 为 1..200，依据窗口长度选聚合桶并再次限制点数；每个点 {ts,value}，返回 metric、unit、points，单位来自模型，非法范围/指标为 400。
- 池塘 status 取所有未确认报警最高等级，不能只看最近 200/500 条。stats.alarm_devices 是未确认报警的不同设备数，不是报警条数；池塘 summary 保留 critical，不能统一降成 warning。
- 规则采用配置的 level，不按上界/下界自动分级。min/max 至少一端且都有限数；两端都有时 min < max；严格小于 min 或大于 max 才触发，等于边界不触发。报警 threshold/message 使用实际命中的那一端。
- 同设备、同发生池塘、同指标未确认不重建；调塘不自动确认旧塘报警，新塘同指标独立产生报警，旧塘owner仍可处理旧报警；确认幂等，确认后仍超限的下一次上报可再触发。批量确认事务执行，任一不存在/越权则整体失败，不部分成功；结果 confirmed 表示本次实际新确认数量。
- 通知 outbox 保存报警 ID、状态、attempts、next_at 和接收规则，后台 worker 有界并发；瞬时网络错误指数退避最多 5 次（1/2/4/8/16 分钟），永久错误记录失败并暴露运维状态，不无限循环。发送前检查当前 owner/订阅条件；微信超时后的投递结果可能未知，不承诺外部 exactly-once。
- 模板字段映射配置化（示例 thing1/number2/time3），验证配置与实际模板一致；token 缓存线程安全，过期刷新一次后重试。HTTP 非2xx、解码错误、errcode 非0均不得计成功。
- 无微信凭据时应用可运行，但通知明确 disabled；没有发送证据不能通过 R28 外部验收。小程序引导订阅授权，拒绝授权仍可使用报警中心。

## 5. 前端、运维及验收边界

后台页面与流程见 FRONTEND-HANDOVER.md；小程序六页为登录、首页、池塘、实时、历史、报警，M7 在导航增加地图/视频，M6/M8 功能主要由管理后台配置。小程序过期 token 回登录，断网/无归属/无数据提供可恢复状态。

运维要求配置校验、HTTP 超时、优雅关闭（先拒绝新请求→停止接入→等待已开始事务/worker 上限 10s→关连接）、只读健康/就绪端点、业务指标。端口、TLS、备份恢复与负载验收见 DEPLOY.md、R29–R33。所有这些都是目标，已有实现必须重新验证。

## 6. 契约冻结关卡

M0 校验两份目标 OpenAPI 的结构、引用及每个路径的请求/响应；M2 用真实 handler 响应做契约回归。M6–M8 每个子阶段进入编码前，按 EXTENSIONS.md 将其详细路径、请求/响应、错误和模型版本补入扩展 OpenAPI，再独立评审；不把尚未生成的扩展契约写成“已冻结”。阶段前设计细化属于本计划的明确交付，不能跳过也不要求所有阶段同时编码。
