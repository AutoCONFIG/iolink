# M7a 设计兼容性修订

日期：2026-10-11。范围：TODO 11a / R43设计准入；不代表视频功能或真实设备验收。
最终设计候选：`f796fcc8a99aef1213429f6ab63cf9a7d3f17586`。

## 发现与修订

- 原设计 `97bbe588bd82e6024522eddead7daa1a8f2dfd3b` 的第二位独立审阅者
  [拒绝](reviews/original-code-rejection.md)：固定ZLM会跟随RTSP重定向、允许外部control、
  内部自动重连，并在多个日志等级输出源URI/凭据。修订明确需要版本管理的安全构建，
  在协议发送边界约束origin、关闭内部retry、移除敏感日志；实际补丁和运行验证仍待媒体实施。
- RTSP修订候选 `657d665b85aba6d346a7f15e588cabe73f118dc0` 获
  [代码拒绝](reviews/rtsp-candidate-code-rejection.md)及
  [门审拒绝](reviews/rtsp-candidate-gate-rejection.md)：固定ZLM每路同时绑定RTP与相邻RTCP，
  正常HLS段还含日期/小时目录。已将20路端口改为20对，将段映射改为受限完整相对路径。
- 端口修订 `cc93e28a4b42c2efd018e6500f95099458dad60d` 尚未修复HLS路径；
  不沿用该快照的任何批准作为最终候选准入。

固定上游源码：ZLMediaKit `46220e6a866592c140d719ca2981bd2276344f5e`。
依据与源码永久链接见[设计](../../../design/M7a-video.md)。

## 实际验证

环境：Linux x86_64，Go 1.26.8；本任务专属Docker容器
`iolink-m7a-foundation-20261011-93c1`，PostgreSQL16.15 / TimescaleDB2.30.2。
本地测试镜像ID：`sha256:6f139d56042989bd35f50ba5986e492cd32e35cc6778c50be2659add298dc09f`。
只创建/删除本轮随机数据库，没有操作其他服务容器或实际用户数据。

| 已执行命令/场景 | 实际结果 |
|---|---|
| `make verify-contracts` | PASS，6契约，96操作/568合成fixture；不代表live handler通过 |
| `.venv/contracts/bin/python -m pytest scripts/test_video_contracts.py scripts/test_m6a_contracts.py -q` | PASS，22测试 |
| 设置 `timescaledb.restoring=on`，依次001..012、修订后的video-proposal.sql及原proposal-check.sql | PASS，来源、租户/农场/通道/session FK、TTL、hash、状态、回滚 |
| 同库运行 [port-pairs-check.sql](port-pairs-check.sql) | PASS，20对可占用；奇数、越界、重复活动RTP被拒绝 |
| 同库运行 [hls-path-check.sql](hls-path-check.sql) | PASS，正常ZLM目录段可写；遍历、外部、编码、query/fragment、错误布局被拒绝 |
| 结束后删除自己的随机数据库 | PASS |
| 修改前基线 `go test -count=1 ./internal/migrate ./internal/persistence`，提供隔离IOLINK_TEST_PG_DSN | PASS，两包真实PG执行，没有把skip当通过 |
| `git diff --check` | PASS |

SQL通过 `docker exec -i <本任务容器> psql -X -q -v ON_ERROR_STOP=1 -U <测试用户> -d <随机库>`
输入文件执行。结果包含：

```text
valid sources, tenant/farm/channel/session FKs, source exclusivity, TTL, revocation, hash, RTP, registration PASS
fixture transaction rollback PASS
twenty disjoint RTP/RTCP pairs; odd, out of range and duplicate ports rejected PASS
native ZLM dated TS path accepted; traversal, external, encoded and malformed paths rejected PASS
```

最终候选由两位未参与编辑的独立审阅者分别明确 APPROVE：
[代码审查](reviews/final-code-review.md)、[门审](reviews/final-gate-review.md)，
均绑定 `f796fcc8a99aef1213429f6ab63cf9a7d3f17586`，无阻断。
批准仅允许视频关闭下开始领域/加密/013存储基础，不代替该实现的后续测试和双审。
真实ZLM媒体/安全构建、摄像机、GB模拟及实机、HTTPS播放和微信真机尚未验收；
外部设备、微信资质和现场TLS/NAT/ACL为 `external_blocked`。
