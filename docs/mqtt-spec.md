# MQTT目标契约 v1-plan（M1、M8）

2026-09-19；目标规格，当前代码需按R05–R10/R46/R49复核，不声称已冻结到运行版本。M1部分与M8新增部分明确分开。

## M1 认证与Topic

MQTT 3.1.1/5.0；局域测试1883，生产TLS8883。ClientID=username=device_no，password为注册时返回的secret（目标32字节随机、64hex）；无效凭据/不匹配/停用拒绝。同一device_no只允许一个活动会话，新合法连接替换旧连接且状态变化幂等。

| 方向 | Topic | 权限/用途 |
|---|---|---|
| 上行 | iolink/up/{device_no}/properties | 仅该设备、QoS1、retain=false |
| 上行 | iolink/up/{device_no}/ack | 仅该设备回执；M1只校验记录，M8处理状态 |
| 下行 | iolink/down/{device_no}/cmd | 仅该设备可订；M8命令服务发布 |

不允许设备订阅#、+或其他设备的topic；其他上行类型拒绝，不把“自身前缀”理解为任意路径。认证成功才报告online，ACL先于业务处理。M1不接受CONNECT遗嘱（WillFlag），防止遗嘱绕过普通发布ACL；设备离线由服务端记录。

## M1 属性和状态

```json
{"message_id":"sample-0001","temperature":27.5,"dissolved_oxygen":6.8,"ph":7.9,"turbidity":12.5,"salinity":28.6,"battery":95,"signal":-65}
```

| 字段 | 类型 | 单位/范围 |
|---|---|---|
| temperature | number | ℃，0..50 |
| dissolved_oxygen | number | mg/L，0..20 |
| ph | number | 无，0..14 |
| turbidity | number | NTU，0..1000 |
| salinity | number | ppt，0..50 |
| battery | number | %，0..100 |
| signal | integer | dBm，-120..0 |
| message_id | 可选string，1..128字符 | 设备内唯一采样ID，支持方去重；旧固件不提供时至少一次 |

属性可选，JSON null按未提供处理。错误类型/坏JSON/超过64KiB整包拒绝；超范围仅丢该字段，其他有效字段保留；未知属性丢弃并计数。全无有效属性不写遥测/影子，不重置数据freshness。采样ts在M1由服务端接收时刻赋UTC时间。QoS1传输确认不等于数据库持久化保证，数据库异常必须可观察，固件的重试语义按集成测试记录。当前broker对非法MQTT3发布可能断开连接或不返回PUBACK；设备需重新连接，携带相同message_id重试时24小时内不会重复持久化。

message_id去重键(device_no,message_id)，保留24小时；重复上报不重复写遥测/报警，超过窗口允许作为新样本。影子按属性合并且每字段记录时间，packet ts新不代表所有字段都新。

注册时可选report_interval=60或300秒，缺省继承全局IOLINK_REPORT_INTERVAL；IOLINK_OFFLINE_GRACE默认3。固件使用同一周期，M8成功set_interval回执后同步服务端配置，回执不确定时保留待核实状态；连接在线，断开或3×周期未见有效心跳离线；同一离线状态不重复发转换，恢复上报转在线。有效已认证properties（包括没有有效传感值）仅更新连接last_seen，数据freshness仍只由有效字段更新。服务重启清理旧会话在线状态。

## M8 命令与回执（本次全量范围内，依赖M8a实现）

```json
{"id":"cmd-0001","name":"set_interval","params":{"seconds":60},"expires_at":"2026-09-19T10:01:00Z"}
```

```json
{"id":"cmd-0001","ok":true,"result":{"seconds":60}}
```

失败回执为 `{id,ok:false,error:string}`。params/result为物模型定义的JSON对象，不限定字符串值；id由平台生成，不得更换ID来重试同一物理动作。设备校验expires_at，缓存已执行ID/结果24小时，重复命令回相同结果；不支持去重的固件禁止自动重试物理写入。平台仅接受该设备自己的命令ID回执；TTL、重试、取消、迟到回执规则见EXTENSIONS的M8a。

## M8 网关扩展

上行 `iolink/up/{gateway_no}/sub/{sub_no}/properties` 或 `/ack`；下行 `iolink/down/{gateway_no}/sub/{sub_no}/cmd`。仅登记该子设备且同租户的网关可发布/订阅具体topic；不开放通配符。属性/命令格式复用模型和上述结构，平台将数据归属到sub_no而非网关。网关注册/解除/停用后ACL与会话同步更新，跨网关/租户伪造必须拒绝。

## 验收

R05–R10先用真实broker+MQTT客户端+隔离数据库测，R46/R49用模拟固件测状态和去重，真固件/DTU回执再独立验收。输入不足记外部阻塞，不能用命令行演示替代固件交付。
