# MQTT 接入规范(iolink-access 契约)

> 版本 v0.1 · 状态:草案,评审后冻结 · 硬件/固件方按本文实现

## 1. 接入方式

- 协议:MQTT 3.1.1 / 5.0,内嵌于 iolink 服务(mochi-mqtt),对外端口 `:1883`(可 TLS)
- ClientID / 用户名 / 密码 = 设备三元组,由平台预注册(设备管理页/SQL 导入),一机一密
- 非法三元组拒绝连接

## 2. Topic 规划

| 方向 | Topic | 说明 |
|---|---|---|
| 上行 | `iolink/up/{device_no}/properties` | 属性上报(水质数据+电量),QoS1,周期 60s(可配置 300s) |
| 上行 | `iolink/up/{device_no}/ack` | 命令回执,QoS1 |
| 下行 | `iolink/down/{device_no}/cmd` | 平台下发命令(预留),QoS1 |

设备只允许 publish 自己的 `iolink/up/{自己的device_no}/#`,订阅自己的下行 topic,由 access 侧做 ACL 校验。

## 3. 属性上报载荷(properties)

```json
{
  "temperature": 27.5,
  "dissolved_oxygen": 6.8,
  "ph": 7.9,
  "turbidity": 12.5,
  "salinity": 28.6,
  "battery": 95
}
```

- 字段均可选(传感器按需上报),值为 JSON number;字段含义与单位:

| 字段 | 单位 | 合理范围 |
|---|---|---|
| temperature | ℃ | 0 ~ 50 |
| dissolved_oxygen | mg/L | 0 ~ 20 |
| ph | pH | 0 ~ 14 |
| turbidity | NTU | 0 ~ 1000 |
| salinity | ppt | 0 ~ 50 |
| battery | % | 0 ~ 100 |

- 超范围值 access 直接丢弃该字段并记日志(防脏数据入库)

## 4. 在线/离线判定

- MQTT 连接建立 → online;断开或 3 × 上报周期无消息 → offline
- offline 由 access 侧心跳检测判定,设备无需发送离线包

## 5. 命令与回执(预留,本期可不实现)

下发:
```json
{"id":"cmd-20260915-001","name":"set_interval","params":{"seconds":"60"}}
```
回执:
```json
{"id":"cmd-20260915-001","ok":true}
```

## 6. 联调路径

1. 先用 MQTT 模拟器(mosquitto_pub / mqttx CLI)按本规范发数据
2. access 负责人联调通过后,固件再对接真实传感器(Modbus 采集 → 组 JSON → publish)
