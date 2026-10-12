#!/usr/bin/env python3
"""iolink 虚拟设备节点 — 标准 MQTT 3.1.1/5.0 客户端模拟真实硬件。

接入参数与任何标准 MQTT 客户端一致（ThingsPanel/MQTTX 等通用配置均可）：
  Client ID  = 任意唯一值（注册弹窗会给出建议值；默认自动生成 vdev-xxxx）
  Username   = 注册时签发的接入用户名（iolink-<设备号>-xxxxxx，弹窗可直接复制）
  Password   = 注册时签发的 Secret（仅显示一次，64 位 hex）
  Clean Session = 必须开启；不设置遗嘱（Will）
  上报 topic: iolink/up/{device_no}/properties  (QoS1, retain=false)
  回执 topic: iolink/up/{device_no}/ack
  命令 topic: iolink/down/{device_no}/cmd       (自动订阅，device_no 从
               username 中解析，也可用 --device 显式指定)

用法示例：
  python3 vdevice.py --host rsplab.hyhy.fun --port 1183 \
      --username iolink-dev-xxxxxxxx-abcdef --secret <64hex> --interval 60

依赖：pip install paho-mqtt
"""

import argparse
import json
import math
import random
import signal
import sys
import threading
import time
import uuid

try:
    import paho.mqtt.client as mqtt
except ImportError:
    sys.exit("需要 paho-mqtt：pip install paho-mqtt")

VALID_INTERVALS = (60, 300)


def log(msg):
    print(time.strftime("[%H:%M:%S] ") + msg, flush=True)


class VirtualDevice:
    def __init__(self, args):
        self.args = args
        self.device = args.device or self._device_from_username(args.username)
        self.interval = args.interval
        self.interval_changed = threading.Event()
        self.stop = threading.Event()
        self.executed = {}  # command id -> (result dict, ts)，模拟固件 24h 去重缓存
        self.t0 = time.time()
        self.battery = round(random.uniform(88, 100), 1)
        self.connected = threading.Event()
        try:
            self.client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2,
                                      client_id=args.client_id, clean_session=True,
                                      protocol=mqtt.MQTTv5 if args.mqtt5 else mqtt.MQTTv311)
        except AttributeError:
            self.client = mqtt.Client(client_id=args.client_id, clean_session=True,
                                      protocol=mqtt.MQTTv5 if args.mqtt5 else mqtt.MQTTv311)
        self.client.username_pw_set(args.username, args.secret)
        self.client.on_connect = self._on_connect
        self.client.on_disconnect = self._on_disconnect
        self.client.on_message = self._on_command
        self.client.reconnect_delay_set(1, 30)

    @staticmethod
    def _device_from_username(username):
        # 签发格式 iolink-<device_no>-<6hex>；解析失败则要求显式 --device
        parts = username.rsplit("-", 1)
        if username.startswith("iolink-dev-") and len(parts) == 2 and len(parts[1]) == 6:
            return parts[0][len("iolink-"):]
        raise SystemExit("无法从 username 解析设备编号，请用 --device dev-xxxxxxxx 显式指定")

    # ---- MQTT 回调（兼容 paho 1.x / 2.x 的参数个数差异）----

    def _on_connect(self, *args):
        rc = args[3]
        failed = getattr(rc, "is_failure", None)
        if failed is None:
            failed = rc != 0
        if failed:
            if not self.stop.is_set():
                log(f"连接被拒绝 rc={rc}（认证失败：检查设备编号/Secret 是否正确、设备是否被停用）")
                self.stop.set()
            return
        log(f"已连接 broker（client_id={self.args.client_id}，MQTT "
            f"{'5.0' if self.args.mqtt5 else '3.1.1'}，清理会话）")
        self.client.subscribe(f"iolink/down/{self.device}/cmd", qos=1)
        log(f"已订阅命令 topic iolink/down/{self.device}/cmd")
        self.connected.set()

    def _on_disconnect(self, *_args):
        if not self.stop.is_set():
            log("连接断开，将自动重连")

    def _on_command(self, _client, _userdata, msg):
        try:
            cmd = json.loads(msg.payload)
            cid, name = cmd["id"], cmd.get("name", "")
        except (ValueError, KeyError):
            log(f"收到无法解析的命令：{msg.payload[:80]!r}")
            return
        params = cmd.get("params") or {}
        if cid in self.executed:  # 平台重发同一物理动作：回相同结果，不重复执行
            self._publish_ack(cid, self.executed[cid][0])
            log(f"命令 {name}({cid[:8]}…) 为重复指令，回缓存结果")
            return
        if name == "set_interval" and str(params.get("interval")) in map(str, VALID_INTERVALS):
            self.interval = int(params["interval"])
            result = {"ok": True}
            log(f"命令 set_interval：上报周期调整为 {self.interval}s")
        else:
            result = {"ok": False, "error": "unsupported_action"}
            log(f"命令 {name}({cid[:8]}…) 不支持，回执失败")
        self.executed[cid] = (result, time.time())
        for old in [k for k, (_, ts) in self.executed.items() if time.time() - ts > 86400]:
            del self.executed[old]
        self._publish_ack(cid, result)
        if name == "set_interval":
            self.interval_changed.set()  # 让上报循环立即按新周期工作

    def _publish_ack(self, cid, result):
        payload = {"id": cid, **result}
        self.client.publish(f"iolink/up/{self.device}/ack",
                            json.dumps(payload, ensure_ascii=False), qos=1, retain=False)
        log(f"已回执 ack {json.dumps(payload, ensure_ascii=False)}")

    # ---- 上报 ----

    def reading(self):
        """生成带昼夜正弦 + 噪声的水质六字段读数。"""
        phase = (time.time() - self.t0) / 1800 * math.pi
        self.battery = max(5.0, self.battery - random.uniform(0.0, 0.02))
        signal = random.randint(-85, -45)
        return {
            "message_id": uuid.uuid4().hex,
            "temperature": round(24 + 3 * math.sin(phase) + random.uniform(-0.4, 0.4), 2),
            "dissolved_oxygen": round(7.5 + 1.2 * math.cos(phase) + random.uniform(-0.3, 0.3), 2),
            "ph": round(7.8 + 0.4 * math.sin(phase / 2) + random.uniform(-0.1, 0.1), 2),
            "turbidity": round(max(0, 12 + 6 * math.sin(phase / 3) + random.uniform(-2, 2)), 1),
            "salinity": round(28.5 + random.uniform(-0.3, 0.3), 2),
            "battery": round(self.battery, 1),
            "signal": signal,
        }

    def publish_once(self):
        if not self.connected.wait(15):
            log("等待连接超时，退出")
            self.stop.set()
            return False
        payload = self.reading()
        topic = f"iolink/up/{self.device}/properties"
        info = self.client.publish(topic, json.dumps(payload, ensure_ascii=False), qos=1, retain=False)
        info.wait_for_publish(timeout=10)
        log(f"已上报（broker 已确认 PUBACK）{json.dumps(payload, ensure_ascii=False)}")
        return True

    def reporting_loop(self):
        while not self.stop.is_set():
            if not self.publish_once():
                return
            if self.args.once:
                return
            # 分片等待，保证 set_interval 与 Ctrl-C 都能即时生效
            waited = 0.0
            while waited < self.interval and not self.stop.is_set():
                if self.interval_changed.is_set():
                    self.interval_changed.clear()
                    break
                time.sleep(0.5)
                waited += 0.5

    def run(self):
        self.client.connect(self.args.host, self.args.port, keepalive=30)
        self.client.loop_start()
        try:
            self.reporting_loop()
        except KeyboardInterrupt:
            pass
        finally:
            self.stop.set()
            self.client.disconnect()
            self.client.loop_stop()
            log("已断开连接")


def main():
    p = argparse.ArgumentParser(description="iolink 标准 MQTT 虚拟设备节点")
    p.add_argument("--host", required=True, help="broker 地址，例如 rsplab.hyhy.fun")
    p.add_argument("--port", type=int, default=1883, help="broker 端口（测试环境 1183）")
    p.add_argument("--username", required=True, help="注册时签发的 MQTT 用户名（iolink-<设备号>-xxxxxx）")
    p.add_argument("--secret", required=True, help="注册时签发的 Secret（MQTT Password，64 位 hex）")
    p.add_argument("--device", default=None, help="设备编号 dev-xxxxxxxx（默认从 username 解析）")
    p.add_argument("--client-id", default=f"vdev-{uuid.uuid4().hex[:8]}",
                   help="MQTT Client ID，任意唯一值（默认自动生成）")
    p.add_argument("--interval", type=int, default=60, choices=VALID_INTERVALS,
                   help="上报周期秒数（默认 60）")
    p.add_argument("--mqtt5", action="store_true", help="使用 MQTT 5.0（默认 3.1.1）")
    p.add_argument("--once", action="store_true", help="上报一次后退出（联调用）")
    args = p.parse_args()
    VirtualDevice(args).run()


if __name__ == "__main__":
    main()
