# M7a 视频接入设计

范围：TODO 11a / R43，附带 R36.b/R37.b/R39.b。这是实施前设计，当前没有视频生产实现，
设计批准不等于播放验收通过。API 见 [video-openapi](../api/video-openapi.yaml)，
DDL 提案见 [video-proposal.sql](video-proposal.sql)，部署见 [M7a部署](../deploy/M7a-video.md)。

## 业务边界与页面

- 普通业务账号在 `/user/v1` 管理摄像机；平台 ADMIN 无摄像机详情/播放权限，不自动创建租户、
  用户或摄像机。平台看板仅显示平台聚合状态。许可证仍绑定部署实例，video feature 授权并非个人许可证。
- camera → pond → farm → tenant；创建/调塘时服务端推导 farm/tenant，拒绝请求中的 tenant_id/farm_id。
  owner/admin 可配置本租户 camera、GB 设备与目录绑定；member/viewer 可看所属或获授权农场的视频。
  support 在现有合法且未到期支持上下文内只读/播放；Web user surface 仍拒绝 ADMIN，
  不扩大已有支持身份规则。所有身份均逐请求复核当前资源归属、成员状态和到期。
- 增加“视频”业务菜单：列表（名称、池塘、来源类型、在线/错误状态）、配置弹窗、目录绑定、
  播放面板。只返回来源类型与 GB 标识，不回传 RTSP URI、内网地址、账号、密码或 digest。
  修改来源需重新提交完整来源配置；RTSP 凭据可省略表示匿名，不能表示保留旧密码。
  凭据表单不预填、不持久化，失败后清空。列表为空是空态，无授权组织提示等待管理员授权。
- GB 设备配置仅 owner/admin 可见；其他角色无设备目录权限（目录包含未分配通道）。
  mini 仅列摄像机、详情、申请/查询/停止自己播放会话，展示接口和状态；真机播放另验。
- UI 明确区分 pending、ready、offline、failed、expired、revoked；
  未授权、License 禁用、provider 未配置、源不可达、编解码不支持分别提示错误。
  H.264/AAC + HTTPS HLS 为基线；无音频可播放，其他音视频轨不声明支持，不启动隐式转码。
  不包含录像、回放、PTZ、级联。

## 接口与错误

API 完整路径写在契约内，独立的业务 JWT 与 mini uid JWT 在各自 surface 验证；
普通请求须现有租户上下文，缺少为 403，过期/错误 token 为 401。
摄像机不存在、跨租户、未获农场访问权统一 404；当前租户角色不具备操作能力为 403。
列表按 camera.id 升序，limit 1–100（默认 50），after_id 正整数；不输出未授权总数。
通道列表按20位 channel_id 字典序，使用 after_channel_id/next_after_channel_id 而非数字行ID。
平台 ADMIN 到 user surface 为 403。所有错误为常量 code/message，无源 URI/凭据/provider 原文。

| 操作 | 语义 |
|---|---|
| camera create/PUT | 同事务校验池塘归属/来源绑定、写配置/审计，PUT source_version+1，并撤销旧会话和写 stop jobs |
| camera DELETE | 幂等停用（保留审计），撤销会话，写 stop jobs；详情及列表不返回停用记录 |
| GB device create/PUT | device_id 全部署唯一，人工登记租户，凭据加密；PUT 撤销注册/会话并写停止意图 |
| GB DELETE | 有启用 camera 绑定时 409；否则停用注册与通道，不物理删除 |
| GB catalog POST | 202，返回持久 job_id；同设备未完成刷新复用 job，不无限追加，重复响应不伪报新目录 |
| GB channels GET | 当前租户设备通道列表，包含发现状态，不能自动创建摄像机或改变 tenant |
| playback POST | 无请求体；事务内验权及 License，创建最长 300 秒 session 和持久启动 job，201 返回 grant |
| session GET/DELETE | 仅创建者可查/停止，跨用户 404；DELETE 已停止仍 204，写幂等停止意图 |
| media manifest/segment GET | 只接受该 session 专用签名 token；pending 为 425 + Retry-After:1，终态不返回媒体 |

媒体 token 认证失败为 401；已撤销/成员撤权为 403，未知媒体对象为 404。
License 缺少/无 video/过期拒绝新配置、目录刷新、播放申请/启动（403）；
读取已存在的脱敏元数据、停止/删除仍可用。License 公钥、密钥或 provider 不可用为 503。
源启动失败不会同步伪报 ready：session 进入 failed，GET 暴露安全 error_code。
所有请求限制 JSON 16KiB、单对象、拒绝未知字段，URI ≤2048、密码 ≤128，SIP/XML 限制另见下节。

## 模块与事务

- domain：来源类型、合法状态迁移、摄像机/会话类型、5分钟上限、可播放编解码规则。
- application：ProvisionCamera、RefreshCatalog、RequestPlayback、ReadMedia、StopPlayback、
  ProcessMediaJob；依赖窄 ports（CameraStore、SessionStore、MediaIngest、GBSignaling、
  CredentialCipher），不依赖 pgx/ZLM/sipgo SDK。
- adapters：PG 事务/租约实现、ZLM REST/media 实现、sipgo 传输与 GB XML/SDP/dialog 实现。
  composition root 注入 provider、时钟、cipher、logger、授权及 License 能力；默认视频不开启，
  无完整能力时 fail closed。不是通用插件框架，不独立拆微服务。
- 创建播放事务：共享锁读取活跃 tenant/member/user/farm 授权行及 camera source_version，
  插入 session、唯一 camera/version stream、start job 与 audit，同事务提交。
  source 修改、撤权、删除与授权读使用同一锁顺序（tenant→user→membership→farm→camera→session）；
  farm 转移/删除和成员变更的已有写入口必须接入 session 撤销或 live 判定，不依靠 JWT 缓存。
- 外部拉流/INVITE 均在事务外。worker claim 使用 jobs 的 lease_owner/leased_until，
  更新结果 CAS 校验租约、source_version 和会话有效性；执行前再验 current 权限及 video License。
  worker 不持有数据库锁等待媒体网络。外部成功后 DB 回写失败属于 unknown，不直接重放。
- start 幂等身份 camera/version；ZLM 固定 stream ID（不可枚举随机值持久化）查询实际状态，
  RTSP unknown 先 getMediaList 查询再补建；GB unknown 复用持久 Call-ID/CSeq/dialog，
  先停止旧接收/对话再发新 INVITE，不并行重复点播。资源失效回写后安排补偿 stop。
- 多 session 共享单路媒体，仅最后活跃 session 停止源；过期扫描间隔 ≤1秒，
  start/stop 同 stream 行序列化，停止前再查活跃 session，防止 stop 清掉新申请。
  session 到期严格由 now ≥ expires_at 判定，扫描延迟不延长 token。
  下游失败退避 1/2/4/8/16/30秒，最多6次且不超过 session 到期；最终 failed。
  SIP 注册恢复触发当前有效 session 的重连 job，不恢复撤销/过期 session。
- jobs payload 只存实体 ID、source_version、action；不存 URL/密码/token。
  lease 30秒、每10秒续租；失租即取消外部调用并执行上面的 reconciliation。
  仅单实例 iolinkd SIP runtime（目录/对话状态持久化，nonce 可内存，重启重新挑战）；
  不承诺多实例 SIP 或内存 nonce 的分布式一致性。

## 播放网关与撤权

grant 含 session_id、expires_at 和 HTTPS manifest_url（含 bearer path token，仅此响应返回）。
token 使用独立派生密钥与 audience `iolink-media-v1`，HS256 固定算法；
claims 包含 session UUID、tenant/camera/user、source_version、iat/exp、随机 jti；
拒绝其他 JWT、算法、超300秒、未来 iat、claim 与 DB 不符、token hash 不匹配。
DB session 保存 SHA256(token)，不保存原 token；刷新必须新建 session，不延长旧 session。
前端只保存在播放组件内存；不进入 localStorage、埋点、错误报告或浏览器日志。

每个 manifest/segment 请求检查签名及活跃 DB session，并实时复核 user token_version、
tenant/member permission_version、active/expiry、farm owner/member、camera source_version。
DB 不可用为503，绝不使用过期授权缓存。License 到期后已播放 session 可存活至原300秒到期，
停止/撤权随时生效。媒体开始播放后每1秒检查当前授权；失败或撤权中断在途传输，
因此撤销后的新请求立即拒绝，在途最多1秒（已下载到播放器的内容不可追回）。

manifest 由网关获取并重写所有 URI 为同源带 token 的 gateway 路径；不回传 ZLM 主机。
仅支持有限 live HLS manifest（≤256KiB）及 H264/AAC TS 段（≤8MiB），关闭 master、
外部 URI、KEY、MAP、byterange 和 fMP4；非支持标签/绝对路径/转义遍历/重定向一律502。
网关分配随机 segment_id，持久映射到固定 stream 下规范段名，绑定 session/source_version，
禁止把任意客户端 segment_id 当文件路径。段超时10秒，manifest 3秒，
授权失效主动 cancel。每用户≤5 session、每 camera≤20 session，超限429，Retry-After:1。
映射在源 HLS 窗口结束或 session 到期后清理；负载测试验证并发与上限而非无限缓存。

所有 media 响应 Cache-Control:no-store、Referrer-Policy:no-referrer，
网关返回 Allow-Origin 仅配置同源（不带跨站凭据）；边缘代理不缓存、不记录原媒体 path/query。
ZLM HLS、API、RTSP/RTMP 不向用户公开；仅依赖 on_play/on_http_access 的缓存鉴权不足以撤权。

## 来源凭据与 SSRF

RTSP URI 仅 rtsp scheme、无 userinfo/query/fragment；允许编码 path，但解码后拒绝控制字符，
反斜线及 `..` 段；端口明确为554或运维允许列表。host/CIDR 必须显式配置，
空列表拒绝。解析全部 A/AAAA，所有目标都在允许范围才可拉取；拒绝 metadata、
unspecified、multicast、loopback、link-local（即使误列入允许项）。内网可按明确 CIDR 授权。
连接传 ZLM 已验证且钉住的 numeric IP，每次重连重新检验；不把原 hostname 交由 ZLM 自行解析。
RTSP 使用 TCP interleaved 传输，禁止来源 redirect 和额外 SDP 外部控制 URL；
adapter 若无法保证，启动拒绝该来源。ZLM 进程 egress ACL 是第二层强制边界。
GB peer/IP 同样按 allowlist，Contact/SDP 不能指定任意外网/metadata；RTP 仅接受登记 peer，
每次注册不能通过 Contact 任意变更既有 tenant/device 身份。

凭据 AES-256-GCM，root→独立 `iolink-camera-credential-v1` KDF，随机 nonce、
tenant+camera/device+source_version 为 AAD。错误 key/AAD/tag 必须失败，不存明文 fallback。
GB digest 的 HA1/密码同样按独立 device AAD 加密；备份恢复需要同 root，不支持未设计的透明轮换。
禁止 adapter SDK 输出请求 URI/digest；日志遵循 observability allowlist，
仅 operation/status、安全错误类别、服务器请求ID和 camera/device hash；不记录 SDP/XML/媒体 token。

## GB28181-2016 最小完整链路

人工登记20位 device ID、密码、预期允许 peer 网段；device ID 全部署唯一（冲突409），
通道20位 ID 仅在所属 device 内唯一。未知 device REGISTER 拒绝，无自动租户/设备创建。
sipgo 仅提供 SIP transport/parse，不把它的单注册 demo 当 GB 实现。

1. REGISTER：验证 request URI/From/To/device username，challenge 用 crypto nonce，
   绑定 device+来源 IP+realm，TTL60秒。MD5 Digest 是 GB2016 兼容基线，
   验证 method/URI/realm/nonce、恒时比较、nonce 单次使用；允许无 qop 的2016设备，
   有 qop 仅 auth 且检查 cnonce/nc。合法 SIP transaction 重发相同响应，
   不生成重复注册。REGISTER Expires 0 注销，合法有效期60–3600秒。
2. 只接受登记 peer 的 MESSAGE，认证 registration/dialog 对应身份（IP/来源端口/传输/
   From/To/DeviceID均匹配；TCP绑定已登记连接）。UDP peer 必须部署来源ACL防伪造；
   不能仅凭XML DeviceID区分同NAT后的设备，同peer多个device禁止注册，返回冲突。
   Keepalive 解析 DeviceID/Status，默认周期60秒、3次未到离线（或注册先过期）；
   SIP response 200 不等于业务 ready。SIP 消息≤64KiB，XML 解码≤64KiB、
   通道总数≤1000，禁 DTD/实体、限制深度16；SN/device ID/分页总数必须一致。
3. Catalog 查询持久 SN，设备回复多个目录页暂存同 SN；10秒截止、
   去重/汇总 SumNum 完整才事务替换目录。超时不覆盖旧目录，标 stale，
   新目录缺失通道使对应 camera offline 并撤销 session，不静默继续旧点播。
4. 点播：openRtpServer 先分配独占 UDP 端口与SSRC，再 INVITE SDP，
   只支持 UDP RTP/AVP PS；offer 含 recvonly、PS/90000、y=SSRC、
   正确公网/局域网 receiver 地址。200 校验 SDP/SSRC/peer 后 ACK，
   只有 ZLM 已提供兼容 tracks 和可取 HLS 后 ready。4xx/超时/BYE/失保活→offline/failed、
   关闭 RTP 与持久 retry/stop；取消与200并发仍 ACK 后 BYE，不能遗留对话。
5. 重启/重连：租约回收查询媒体，旧 dialog 尽力 BYE，关闭旧 RTP，再挑战注册与目录；
   仅重启仍有效的会话，注册离线/不足 codec 不伪报播放成功。

不支持的 TCP RTP、H265、G711 转码明确 reported unsupported，不声明 GB2016 全能力。
兼容承诺是本项目要求的注册/保活/目录/UDP PS 实时链路，实物 camera 必须单独提供证据。

## 数据设计与校验

提案不放入 internal/migrate/sql；双审批准后才生成013生产迁移。
ponds 新增 (id,farm_id) 唯一键；camera 同时 FK (farm_id,tenant_id) 和 (pond_id,farm_id)，
不用为 ponds 增加重复 tenant 列。GB 设备/通道 camera FK 含 tenant_id，防跨租户绑定。
源配置版本、session 过期上限、GB/RTSP互斥、状态与 token hash 用数据库约束兜底；
来源 URI/cipher JSON 仅 adapter内部，授权/URL/crypto 规则仍在应用边界，不声称 SQL 能代替。
删除采用逻辑停用，农场/池塘有 camera 历史时物理删除返回409，先停止/清理引用；
不得扩大级联删除审计。DDL、两个租户 fixture、错误 FK/来源/TTL/状态、事务回滚需真实PG核验。

## 实施顺序与验收矩阵

1. 契约/DDL/部署与错误设计双审同快照；静态schema与真实PG提案验证归档。
2. domain/ports、PG存储、crypto、camera/user/mini API 与授权/License。阶段仍未完成。
3. RTSP/ZLM、媒体 session worker、HLS gateway：真实 ZLM +合成 H264/AAC源集成。
4. SIP/GB 注册/保活/目录/实时点播 +模拟设备UDP PS，异常/重启/并发；对接真camera。
5. Web页面/API demo、最终软件双审；R43真实摄像机/微信验收仍需外部输入。

| 组 | 必须验证的正向与负向 | 层 |
|---|---|---|
| 边界 | 全角色、两租户/两用户、调塘、ADMIN拒绝、缺tenant、非法JSON、列表过滤 | U/I/HTTP |
| crypto/SSRF | 正确解密、错误key/AAD、无明文；CIDR/DNS混合地址/rebinding/metadata/redirect/SDP | U/I |
| DB/worker | 原子job、回滚、并发重复start、租约失效、unknown reconcile、stop/newsession竞态 | PG/I |
| token | 300秒边界/篡改/audience/claim/hash、撤权/停租户/调塘/密码撤销、DB失效与在途取消 | U/I/media |
| License | 申请/worker真实video入口拒绝、已有session边界、停止可用、public key缺失503 | I |
| RTSP/HLS | H264/AAC tracks、合法段重写、非法manifest/路径、源断线重连、ZLM断线/重启 | ZLM/I |
| SIP/GB | digest/replay/transaction重发、未知ID、越权peer、XML界限、完整目录/超时、INVITE取消/BYE | UDP/I |
| 外部 | 真RTSP与GB各一路、HTTPS浏览器、微信真机资质与播放、凭据不出前端 | W/X/H |

当前外部输入：camera地址/账号/允许网段、GB平台与设备ID/realm/peer/公网NAT、TLS域名证书、
微信类目/AppID/真机。缺少则 external_blocked，不能用模拟源冒充真实摄像机或小程序通过。

## 固定研究来源

ZLM源码固定至 `46220e6a866592c140d719ca2981bd2276344f5e`；
sipgo固定至 `03cdf8e07c69e96719816d70f13a44a105e52d5a`。后续运行镜像必须记录digest，
版本行为不匹配时复审 adapter，不自动假定 main兼容。

- [ZLM配置](https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/conf/config.ini)：
  默认API密钥公开、多个协议监听默认启用，必须覆盖。
- [ZLM API](https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/server/WebApi.cpp)：
  addStreamProxy/delStreamProxy/getMediaList/openRtpServer/closeRtpServer。
- [ZLM hook](https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/server/WebHook.cpp)：
  on_http_access 带目录/秒数缓存，不能单独保障实时撤权。
- [sipgo注册demo](https://github.com/emiago/sipgo/blob/03cdf8e07c69e96719816d70f13a44a105e52d5a/example/register/server/main.go)：
  仅单注册示范，不能复用共享nonce/用户名日志。
