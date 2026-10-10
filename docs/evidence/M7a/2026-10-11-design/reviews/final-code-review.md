# M7a 最终设计独立代码质量与准入审查

日期：2026-10-11（Asia/Shanghai）。审查角色：独立只读代码质量审阅者 `/root/m7a_final2_design_code`。

**结论：APPROVE。**

```json
{
  "sourceCommit": "f796fcc8a99aef1213429f6ab63cf9a7d3f17586",
  "sourceTree": "3d8334edaffdd0ec7f49380c1d1e237c652ed035",
  "codeQualityStatus": "CLEAR",
  "recommendation": "APPROVE",
  "reportPath": ".omo/evidence/m7a-final2-design-code-20261011-code-review.md",
  "blockers": []
}
```

本次批准是 **M7a 设计准入**，当前实施授权限于 **视频保持关闭的领域值、加密 adapter 与 013 存储 schema 基础**。安全 ZLM 构建尚未产出，媒体、GB、视频 HTTP/worker 和播放器均未通过运行验收；后续须完成设计列出的对应门槛。此结论不代表 R43、R36.b/R37.b/R39.b 视频入口完成，不代表可以开启视频或交付真实摄像机/微信播放。

## 固定范围与独立性

- 使用独立 detached worktree `/tmp/iolink-m7a-final2-code-20261011`，HEAD 完整 SHA 为上面的 `sourceCommit`。
- 先读 AGENTS.md 与 docs/README.md，再按权威关系核对 PLAN、PLAN-DETAILS、EXTENSIONS、ACCEPTANCE、CONTRIBUTING 和 IMPLEMENTED；IMPLEMENTED 仍将 M7a 记为正在进行、R43–R45 未实现。
- 审查最终文件：`docs/design/M7a-video.md`、`docs/design/video-proposal.sql`、`docs/api/video-openapi.yaml`、`docs/deploy/M7a-video.md`、`scripts/check_contracts.py`、`scripts/test_video_contracts.py` 和三个 SQL 验证文件。
- 对照原始 M7a 提交 `97bbe588bd82e6024522eddead7daa1a8f2dfd3b`、安全构建修订 `657d665b85aba6d346a7f15e588cabe73f118dc0`、端口修订 `cc93e28a4b42c2efd018e6500f95099458dad60d`、HLS 修订 `f796fcc8a99aef1213429f6ab63cf9a7d3f17586`。中间产品、设备和权限 UI 提交不属于本审查授权；完整 M7a 文件 diff 另存证。
- 未读取其他审阅者报告。全部判断来自自己的文件、上游源码读取及验证；父任务的进度消息不作为批准证据。
- 未修改 tracked 文件，未实现修复，未委派子代理，仅写审查 artifacts。`git diff --exit-code HEAD -- docs scripts internal deploy` 退出 0。
- `omo-agent-toolkit ulw-loop status --json` 返回 `ULW_LOOP_PLAN_MISSING`，故使用上述 fallback report path。另按任务要求交付同内容 `.omo/evidence/m7a-final2-design-code-20261011.md`。

证据根目录：`.omo/evidence/m7a-final2-design-code-20261011-artifacts/`。文件摘要与环境见 [manifest.json](m7a-final2-design-code-20261011-artifacts/manifest.json)，完整 diff 见 [full-m7a.diff](m7a-final2-design-code-20261011-artifacts/full-m7a.diff)，notepad 见 [notepad.md](m7a-final2-design-code-20261011-artifacts/notepad.md)。

## Findings by severity

| Severity | Findings | 必须先修复的阻断 |
|---|---|---|
| CRITICAL | 无 | 无 |
| HIGH | 无 | 无 |
| MEDIUM | 无 | 无 |
| LOW | 无 | 无 |

未发现阻止本次限定基础实现的设计矛盾、数据库错误或测试虚假成功。下面的未完成门槛是设计明确要求的后续工作，不能改记为已通过。

## 设计核对

### 权限、License 与凭据

`docs/design/M7a-video.md:9`–20、28–51、62–68、84–95 明确 USER/mini/media surface、ADMIN 拒绝、租户与农场归属、角色能力、会话创建者以及逐操作实时授权。source 更新、停用、调塘、农场转移和成员撤权均要求事务撤销或 live 判定；源 URI、密码、peer 地址不进入前端响应。目标 OpenAPI 的 Camera/GBDevice/PlaybackSession 为明确的脱敏输出形状，额外字段拒绝；mini 无 provisioning 路由，媒体采用专用 path token。

License 是实例授权：配置、目录、会话申请和 worker 启动检查 video；存量元数据读取、停止和删除仍可用。过期后的存量播放最多延续原 300 秒，不能刷新延长。共享登录 JWT 不能作为媒体 token；HS256/audience/claim/hash/version 检查、DB 故障 503、在途最多 1 秒撤权、no-store/no-referrer 与日志禁止原路径均已规定。AES-GCM 的独立 KDF、随机 nonce、tenant/entity/version AAD、错误 key/tag/AAD 拒绝和无明文 fallback 已明确。

这些均为拟实现要求；本轮静态契约检查不能证明真实 HTTP/worker/媒体权限已经存在。对应两租户、多角色、两用户、License 和撤权验证仍须随实际入口执行。

### 原子 jobs、未知结果与重启

`docs/design/M7a-video.md:62`–80 将 session、唯一 camera/version stream、start job、audit 放在同一事务；已有 jobs 表在 005 迁移中具备 tenant、idempotency_key、lease_owner/leased_until。网络副作用在事务外执行，结果回写须 CAS 租约和当前 source/session 权限，失租或回写失败进入 reconciliation。固定 stream 身份、共享一路媒体、最后 session 停源、start/stop 序列化及新申请竞态已纳入必测矩阵。

固定 ZLM WebApi 的 `addStreamProxy` 通过 tuple key 管理 proxy；`getMediaList` 是已注册媒体查询，不能把尚在连接的 proxy 缺席直接判为不存在。实际 adapter 实施时须保留 unknown 状态、正确处理重复 key/在连接的 proxy；不得将查询空结果升级为确定失败或伪报 ready。当前没有该 adapter，本轮不把这项运行行为记为通过。

### RTSP 安全修订与可实施性

独立取得并读取上游 SHA `46220e6a866592c140d719ca2981bd2276344f5e` 的 RtpServer、HlsMakerImp、RtspPlayer、PlayerProxy，并补查 WebApi/config；原文归档在 artifacts/zlm，文件 SHA256 在原文可重新计算。

| 源码证据 | 实际行为 | 最终设计覆盖 |
|---|---|---|
| RtspPlayer.cpp:183–200 | 301/302 读取 Location 后调用 play | M7a-video.md:126 对所有 3xx 拒绝，且允许网段内 redirect 也拒绝 |
| RtspPlayer.cpp:209–265、283–301 | Content-Base 和 session/track control 进入后续请求 | M7a-video.md:127–131 在保存与最终 send 边界约束 pinned numeric origin，禁 userinfo/query/fragment/编码遍历及异常补发 |
| RtspPlayer.cpp:718–786 | 在 request 序列化时按目的 URI 加 Digest/Basic 认证；Trace 输出 URI | 同一发送边界必须在生成/发送带认证报文之前拒绝不合法 URI，安全构建移除原文日志 |
| RtspPlayer.cpp:98、851；PlayerProxy.cpp:205、276、333 | 输出 URL/user/password 或原始异常 | M7a-video.md:132–133 与 deploy/M7a-video.md:36–37 要求从源码移除各等级敏感日志，禁止只调高日志等级 |
| PlayerProxy.cpp:207、237、262；WebApi.cpp:1348 | 默认 retry_count=-1，失败及断流自动重连 | M7a-video.md:134–135 显式 retry_count=0，全部重连交由 worker 重验 DNS/权限/License |

修订指定了实际可改的源码边界，未把未打补丁的固定原版当安全 provider。补丁、构建配方、补丁 SHA256、最终 image digest、私有能力探测，以及同一 digest 的正常源/恶意服务/全部日志等级/禁自动重连实测均是媒体启用前置条件（设计:137–141、210；部署:31–37、60–64）。缺失或不匹配时 enabled 视频启动失败，基础里程碑保持关闭。egress ACL 作为第二层，不替代协议校验。

本轮没有构建或运行安全镜像；这些门槛全部 **pending**，不是 passed。

### GB、端口对与 HLS

GB 的人工登记、全局 device ID、peer 绑定、Digest nonce/replay/SIP transaction、Expires 注销、保活、XML 体积/实体/深度、完整分页目录原子替换、INVITE/ACK/BYE/取消竞态、重启与 codec ready 判断已写入 `M7a-video.md:151`–183。UDP 来源 ACL 是部署前提，TCP RTP/H265/G711 转码没有被暗示为已支持。

固定 `RtpServer.cpp:137`–156 显式绑定 RTP `local_port` 和 RTCP `local_port+1`。最终设计按 30000..30038 的 20 个偶数 RTP 端口分配、防火墙/NAT 30000..30039；DDL:91 的偶数/范围检查与 :103 的 active unique index 能排除相邻 RTP/RTCP 重叠。provider 确认关闭后再归还端口对仍属实际 worker 必测。

固定 `HlsMakerImp.cpp:156`–192 生成 `YYYY-MM-DD/HH/MM-SS_index.ts`；最终设计保留完整受限相对路径，DDL:133 布局约束与 128 字符上限和源码一致。DDL 是布局兜底，应用仍须严格解析有效日期、时间和 index；不得把正则接受的字符串当完整日期证明。segment_id 是 session 内随机映射，不透传 client/provider 路径；master、外部 URI、KEY/MAP/byterange/fMP4 和重定向都拒绝。WebApi 的 proxy MediaTuple params 为空，与无 query 的基线布局相容。

### DDL 与现有 schema

提案没有放入 internal/migrate/sql，现有迁移仍止于 012。camera 的 `(farm_id,tenant_id)` 与 `(pond_id,farm_id)`、GB device/channel 的含 tenant FK、session/stream 的 camera/tenant/version 复合 FK 保持归属一致。source 互斥、cipher/hash 下限、状态、revoked_at、5 分钟上限与 RTP 分配原子字段检查均可在真实 PG 上建立。逻辑停用与历史引用阻止农场/池塘物理删除，没有新增业务审计级联删除。

## 独立验证与原始输出

| 验证 | 实际命令/环境 | 结果与 artifact |
|---|---|---|
| 标准契约 | 在 detached worktree 执行 `make verify-contracts DOCS_PYTHON=/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python` | 退出 0；6 契约、96 操作、568 样例：[contracts.log](m7a-final2-design-code-20261011-artifacts/contracts.log) |
| 22 pytest | 同 pinned Python 执行 `-m pytest scripts/test_video_contracts.py scripts/test_m6a_contracts.py -v -p no:cacheprovider` | 退出 0，22 passed（18 视频、4 物模型）：[pytest-22.log](m7a-final2-design-code-20261011-artifacts/pytest-22.log) |
| 真实提案 | `docker exec -i iolink-m7a-foundation-20261011-93c1 psql -X -v ON_ERROR_STOP=1 -U iolink_test -d iolink_m7a_final2_code_64a9bcbcb375`；每个迁移先 SET timescaledb.restoring=on，顺序 001..012 + proposal | PG 16.15 / Timescale 2.30.2；全部退出 0：[postgres.log](m7a-final2-design-code-20261011-artifacts/postgres.log) |
| SQL 正负路径 | 同随机库执行 proposal-check.sql、port-pairs-check.sql、hls-path-check.sql | 来源正例、10 类失败与回滚，20 对端口、奇数/越界/冲突拒绝，原生 HLS 路径与不安全路径拒绝全部 PASS；同 postgres.log |
| 清理 | 只对本审阅者随机库执行 DROP DATABASE | 退出 0；未停止/删除共用专属容器；同 postgres.log |
| 只读边界 | `git diff --exit-code HEAD -- docs scripts internal deploy` | 退出 0：[tracked-diff-check.log](m7a-final2-design-code-20261011-artifacts/tracked-diff-check.log) |
| 格式 | scoped `git diff ... --check` | 退出 0，无空白错误 |

本次不运行全 Go/Web suite：M7a diff 没有视频生产代码、没有可运行 UI 或 Compose；契约、DDL 与指定上游源码是本设计关卡的相应验证。数据库没有 skip，没有把模拟输入称为真实摄像机。真实媒体安全构建、GB 模拟器/实物、HTTPS 浏览器、微信真机、现场 NAT/ACL 仍 pending 或 external_blocked。

## remove-ai-slops / programming 视角

**检查已运行。** 已读取 `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`、`programming/SKILL.md` 及 `programming/references/python/README.md`。此处使用只读审查规则，不执行两项技能的代码清理动作。

- 新增测试调用实际目标 OpenAPI 的 WriteValidator/Validator，包含来源正例、互斥/缺字段/非法 scheme/凭据泄漏/越权字段等负例；它们能够因机器消费的 schema 回归而失败。
- mini/media 路由测试检查机器消费的路径、security 与错误响应，不检查自然语言说明。fixture 测试以输入提供的 example 和 oneOf 为已知期望，真实 validator 仍验证样例；没有从被测输出推导预期。
- 未发现删除专用、只证明移除、tautological、仅镜像实现常量的无用测试。SQL 中回滚后的查询是事务不留状态这一需求的测试结果，不是生产删除/设置后的冗余确认。
- 新增 parsing 仅用于契约工具和必要的不可信协议/URI 边界设计；没有新增不受目标要求的生产数据抽取、归一化、宽异常、`Any` escape hatch 或插件抽象。
- 两个 scoped Python 文件纯 LOC 分别 111 / 81，未触发 250 行上限。现有 fixture/resolve 保持既有未完整类型化的工具签名；本次复用工具，未因此引入生产未经解析数据流。没有为样式迁移扩大范围。

**本次 M7a diff 未发现这两项视角要求拒绝的测试或生产复杂度违反项；没有 skill-perspective blocker。**

## 批准后保留的门槛

1. 基础实现保持视频关闭；领域/crypto/013 须按实际行为取得新的单元和真实数据库证据及独立复审。
2. 媒体实现先交付固定安全补丁、配方、hash/digest 与能力探测；未完成恶意 RTSP、敏感日志和重连测试不能开启视频。
3. HTTP/worker/媒体按角色、租户、来源版本、License、unknown/租约/停止竞态复验；GB 按目录、dialog、peer、端口释放和重启实测。
4. R43 最终完成仍要求真实 RTSP 与 GB 各一路、浏览器与微信真机，并保持 external_blocked 的诚实状态。

**最终 verdict：APPROVE；codeQualityStatus=CLEAR；blockers=[]。**
