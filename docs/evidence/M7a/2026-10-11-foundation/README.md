# M7a 领域、加密与存储基础

范围：TODO 11a第一实现增量，R43基础；不代表摄像机API或播放验收。
设计准入快照 `f796fcc8a99aef1213429f6ab63cf9a7d3f17586` 已获两份独立批准，
见[设计修订与双审](../2026-10-11-design/README.md)。

## 当前候选范围

- 来源语法与GB标识、播放时间和codec领域值，窄CredentialCipher port。
- 独立HKDF标签、AES-256-GCM随机nonce、tenant/entity/version/purpose AAD绑定。
- 013生产schema：摄像机、GB设备/目录/通道、媒体stream/session/segment及完整复合FK。
- 既有池塘删除遇到视频历史FK返回409，逻辑停用也不级联删除历史。

本增量尚在实施和验证中；测试/双审收据将在实际执行后补齐。
没有视频开关配置、摄像机HTTP/媒体provider/worker/GB runtime/播放器。
SSRF允许列表/DNS钉住、License视频入口、实时授权、媒体token/撤权须随下一增量验证。
外部摄像机、微信真机及现场TLS/ACL/NAT仍为 `external_blocked`。
