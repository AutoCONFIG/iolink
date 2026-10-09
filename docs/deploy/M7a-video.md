# M7a 视频部署附件（设计，尚不可运行）

本附件冻结视频的网络与鉴权边界；`deploy/streaming.optional.yaml` 仍是未实现占位，
不把它当可部署视频配置。生产实现后才提交可运行覆盖文件。
不改变现有服务器 Compose 首启流程。

| 流量 | 端口/协议 | 暴露和限制 |
|---|---|---|
| 浏览器→HTTPS反代→iolinkd | 443/TCP→8080 | Web/API及 /media/v1；TLS必需，同源 |
| camera→iolinkd SIP | 5060/UDP、5060/TCP | 仅授权camera网段/来源IP，禁互联网任意注册 |
| GB camera→ZLM RTP | 30000–30019/UDP | 最多20路同时接收；每路独占，防火墙允许登记peer |
| iolinkd→ZLM API/HLS | 80/TCP | 私有media网络，无宿主机映射，只允许iolinkd |
| ZLM→RTSP camera | 运维授权端口，默认554/TCP | numeric IP + egress允许网段/端口，禁任意出站 |
| RTSP/RTMP/WebRTC/SRT公开入口 | 无 | 关闭未用协议；不得发布554/1935/8000/9000/10000等默认端口 |

GB声明receiver IP由运维明确设置，与上述UDP端口一一NAT映射；不能根据不可信Host/SDP推导。
无合法receiver IP或端口不可用时503/启动失败，不发送不可接收的INVITE。
TCP SIP保留作注册/目录传输，媒体仅UDP PS；SIP默认5060而非HTTPS反代。
Linux主机限制media网络egress，Docker网络本身不是SSRF防火墙。
私有API和RTP不同通路；RTP直接入ZLM不代表允许外部调用API/取HLS。

## 必需配置（待实现字段，当前不能直接放入 .env 使用）

- 视频显式enabled开关、ZLM固定镜像digest、私有API地址、非默认随机API secret文件、
  camera允许CIDR/host/RTSP端口、GB server ID/realm/receiver IP/peer CIDR、SIP监听地址。
- ZLM `protocol.enable_hls=1`，H264/AAC TS，不开启fMP4/录制/转码；
  `rtp_proxy.port_range=30000-30019`，每路显式 openRtpServer指定端口。
  关闭固定10000 listener及未用RTSP/RTMP/WebRTC/SRT，具体字段需用选定镜像实测。
- ZLM API secret、camera密文root、TLS私钥只读挂载，不进入镜像/git/日志。
  iolinkd/media容器非root、最小写卷、日志轮转、无特权/host network。
- Nginx /media/v1 关闭access_log/缓存、禁止query转发、禁止公开ZLM路径，
  只代理到iolinkd，超时不延长token TTL；其他HTTP日志只用路由模板和服务器请求ID。
  前端Referrer-Policy:no-referrer；HLS播放URL绝不放进监控payload。

## 网关健康与失败

iolinkd readyz覆盖启用后的provider能力，缺API secret/allowlist/GB配置或认证连接失败不能ready。
ZLM health通过私有鉴权API测试，不用公开HLS当健康检查。
可选功能关闭时基础采集可运行，视频请求503；开启而必要能力不足则启动失败。
先停止接收新session，再撤销session/持久stop jobs，最多10秒关闭媒体请求，常规停机预算15秒。
异常退出重启后reconcile租约/stream/RTP/dialog，再接收播放请求，不依靠内存作为唯一状态。

## 验收与安装记录

设计阶段：端口/API/未用协议检查；生产阶段：记录镜像digest、有效ZLM配置的脱敏摘要、
docker网络与宿主机ACL、RTSP/GB peer、HTTPS域名、codec以及软件/真机范围。
从外部网络检查ZLM API/HLS/其他listener不可达；合法客户端经网关访问媒体，
篡改/撤销/过期token均拒绝；禁止直接访问ZLM bypass。
没有服务器ACL、公网TLS和真camera证据时仅软件验证，部署验收为external_blocked。
