# M7a 领域、加密与存储基础

范围：TODO 11a第一实现增量，R43基础；不代表摄像机API或播放验收。
设计准入快照 `f796fcc8a99aef1213429f6ab63cf9a7d3f17586` 已获两份独立批准，
见[设计修订与双审](../2026-10-11-design/README.md)。

## 当前候选范围

- 来源语法与GB标识、播放时间和codec领域值，窄CredentialCipher port。
- 独立HKDF标签、AES-256-GCM随机nonce、tenant/entity/version/purpose AAD绑定。
- 013生产schema：摄像机、GB设备/目录/通道、媒体stream/session/segment及完整复合FK。
- 既有池塘删除遇到视频历史FK返回409，逻辑停用也不级联删除历史。

实现源码为 `50e72e5040f18061fb3df52be8437b6186e6c36b`；
后续测试修订 `2d93000` 仅将既有设备分页测试对齐当前契约。
`bf740ae` 补强超长密文测试，使用认证有效的4097字节明文外部密文；生产 cipher 未变。
本增量正在完成全仓库验证和实现双审，未将 M7a 整阶段记为完成。
没有视频开关配置、摄像机HTTP/媒体provider/worker/GB runtime/播放器。
SSRF允许列表/DNS钉住、License视频入口、实时授权、媒体token/撤权须随下一增量验证。
外部摄像机、微信真机及现场TLS/ACL/NAT仍为 `external_blocked`。

## 环境与已执行检查

2026-10-11，Go 1.26.8 linux/amd64；专属容器
`iolink-m7a-foundation-20261011-93c1`，仅回环端口 32904。
PostgreSQL 16.15、TimescaleDB 2.30.2，镜像标签 `latest-pg16`，实际 image ID
`sha256:6f139d56042989bd35f50ba5986e492cd32e35cc6778c50be2659add298dc09f`。
真实数据库命令设置 `IOLINK_TEST_PG_DSN` 指向该隔离实例，凭据不归档；
测试为每例创建、清理随机数据库，未设置 DSN 的 skipped 不作为通过。
Web 未修改，子模块保持 `f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350`。

| 场景 | 实际命令/预期 | 观察结果 | 证据 |
|---|---|---|---|
| 静态契约及视频回归 | `make docs-tools`、`make verify-contracts`；pytest video/m6a 两文件 | PASS；96 操作/568 样例，22 pytest | [契约](contracts.log)、[回归](contract-tests.log)、[工具](docs-tools.log) |
| 来源、播放 TTL/codec、凭据边界 | `go test -race -shuffle=on -count=1 -v ./internal/domain ./internal/videocredential -run TestVideo` | PASS；URI 正反例、序列化拒绝、300秒边界、未知终态拒绝、独立 HKDF/AES 双向互操作、Unicode/4096字节上限、错误 key/AAD/tag、截断及取消 | [主会话实测](domain-crypto.log)、[执行报告](domain-crypto-report.md) |
| 超长密文测试能发现长度守卫缺失 | `go test -race -count=1 -v ./internal/videocredential -run TestVideoCredentialRejectsOversizedEnvelope`；再用 Go overlay 仅移除 Open 长度上限 | 正常守卫 PASS；移除守卫后预期 FAIL：`oversized authenticated ciphertext accepted`。独立 AES-GCM 确认测试输入认证有效，无生产源码修改 | [正常](cipher-boundary-green.log)、[守卫缺失](cipher-boundary-without-guard.log)、[当时overlay](cipher-boundary-overlay.json)、[overlay源码](cipher-without-guard.go.txt) |
| 013存储约束、失败原子回滚、密文落库 | 设置隔离 DSN 后 `go test -race -shuffle=on -count=1 -v ./internal/migrate ./internal/core -run TestM7aVideo` | PASS，无 skips；真实迁移执行三组 SQL 正反例，跨 tenant/farm/source FK、TTL/hash、20组RTP/RTCP、HLS布局、唯一性和回滚 | [PG/HTTP实测](pg-http.log)、[执行报告](storage-lifecycle-summary.md) |
| 既有资源删除 HTTP 回归 | 上述命令启动实际 `UserRoutes` httptest server | PASS；active/disabled camera及历史返回409，空塘204，越权/缺失404、member/viewer403、无登录401；拒绝删除保留资源和session | [PG/HTTP实测](pg-http.log) |
| 既有 M6b 分页测试不匹配 | 首次隔离 DSN `make verify` | FAIL；设备列表当前为分页对象，旧测试按数组解码导致member/viewer/support失败；保留原失败，修正解码并增加分页总数/默认值校验 | [原失败](verify-before-pagination-fix.log) |
| 全仓库 build/vet/race/非缓存测试 | 隔离 DSN + `GIN_MODE=release make verify` | PASS，退出0；所有Go包build/vet/race通过，core真实数据库测试181.309秒 | [全库输出](verify.log) |
| 补强测试后的全仓库复核 | 同一隔离 DSN `make verify` | PASS，退出0；全部build/vet/race，core真实数据库测试127.933秒 | [复核输出](verify-after-boundary-test.log) |

执行者报告中的 `.omo/` 路径是当时的本地记录；可随 Git 分发的报告、环境和专项日志已归档到本目录。
团队四条工作线均已交付，归档状态见 [team-archive.json](team-archive.json)。
[摄像机 API 接入点](camera-api-seams.md)及[安全 ZLM 构建接入点](rtsp-build-seams.md)
是后续调研，不表示这些能力已经实现。

## 双审

全仓库检查已通过，实现双审针对同一冻结快照执行中；设计批准不能替代实现批准。
独立 code/gate 审阅者均不得参与本次编辑。
