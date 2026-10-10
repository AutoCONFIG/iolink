# M7a final2 independent design gate review

recommendation: **APPROVE**

blockers: **[]**

Commit: **f796fcc8a99aef1213429f6ab63cf9a7d3f17586**  
Tree: `3d8334edaffdd0ec7f49380c1d1e237c652ed035`  
Reviewer: `/root/m7a_final2_design_gate`, 2026-10-11, Asia/Shanghai  
Review worktree: `/tmp/iolink-m7a-final2-gate-review`, detached, clean at identity check  
Notepad: `.omo/evidence/m7a-final2-design-gate-20261011/notepad.md`

## originalIntent / desiredOutcome

对 M7a 视频实施前设计作独立只读准入审查，核验最终三次修订与安全、授权、事务和部署边界是否可实施。用户需要一份能够据此开始视频关闭状态下领域、加密与存储实现的可靠设计；用户没有要求本次宣称 R43、媒体软件链路或实机播放完成。

目标仍为 docs/EXTENSIONS.md:65–67、docs/ACCEPTANCE.md:82 的 RTSP 与 GB28181-2016、HTTPS HLS、H264/AAC、五分钟令牌、撤权与凭据保护。当前准入范围明确保留媒体安全构建与真实运行门槛；外部摄像机、TLS/NAT/ACL 和微信真机仍为 external_blocked。

## userOutcomeReview

设计准入通过。最终文档明确承认固定原版 ZLM 不符合来源安全及日志要求，并给出源码补丁、构建配方、批准 digest、能力探测、正常与恶意 server 运行证据的前置条件；不能以原版镜像或仅调高日志等级开启媒体。可先保持视频关闭实施领域、加密和数据库基础。R43 与媒体运行均未获本报告批准。

没有发现证据能够证明本次设计准入违反明确成功条件。本文是一个独立审阅人的明确结论；AGENTS.md / CONTRIBUTING.md 要求的同快照两位独立批准尚未齐备，不能把本报告作为双审完成或阶段完成。

## Success criteria and direct findings

以下 D1–D6 是本次审查的索引，逐项对应原任务和权威资料，并非新增产品要求。

| Criterion | 对应要求与直接核验 | 结论 |
|---|---|---|
| D1 设计范围与状态诚实 | docs/design/M7a-video.md:3、:192–215，docs/deploy/M7a-video.md:1–5，docs/IMPLEMENTED.md:40/:118；当前无013迁移、视频handler/provider或可运行视频Compose宣称 | PASS |
| D2 RTSP来源与日志边界可实施 | design:116–150，deploy:25–39/:60–65；numeric IP钉住、每次DNS重验、3xx拒绝、Content-Base及session/track control解析约束、最终send边界检查、无原文日志、安全构建/digest探测、retry_count=0。与固定源码逐一核对，见下文 | PASS for design; runtime pending |
| D3 20路RTP/RTCP容量与释放边界一致 | design:171–176，deploy:11/:16–20/:29–30，proposal:91/:105；20个偶数30000..30038，各保留RTCP+1，防火墙30000..30039。真实PG接受20对并拒绝奇数、超范围和重复活跃端口；源码确实同时绑定相邻端口 | PASS |
| D4 HLS兼容与路径安全可实施 | design:96–108，proposal:133；保留固定 YYYY-MM-DD/HH/MM-SS_index.ts 布局，应用严格日期/时间解析、禁止编码/query/fragment/遍历/客户端provider路径，DDL仅布局兜底；真实PG合法与恶意路径fixture通过，固定源码布局一致 | PASS for design; gateway runtime pending |
| D5 认证授权、数据完整性、unknown与撤权设计 | design:9–17/:31–92/:153–187；业务与mini身份分开、ADMIN拒绝、跨租户/用户404、农场live权限、License真实入口守卫、事务内session/job/audit、锁序与CAS租约、外部调用在事务外、unknown查询/补偿、start/stop序列化、五分钟和在途最多1秒取消。提案真实PG外键/互斥/TTL/hash/状态/回滚核验通过；实施阶段仍须完成相应并发和媒体矩阵 | PASS for design |
| D6 本快照独立复现契约和提案 | make verify-contracts 96 operations/568 synthetic fixtures；指定pytest 22 passed；PG16.15/Timescale2.30.2上顺序001..012+proposal+3个suite全部PASS，自己的随机DB已清理；完整命令、环境及输出见证据目录 | PASS |

## Fixed-source inspection

直接从固定上游 SHA `46220e6a866592c140d719ca2981bd2276344f5e` 下载源码，未沿用他人源码副本。副本与SHA256在 `.omo/evidence/m7a-final2-design-gate-20261011/`。

- `RtspPlayer.cpp:191–199` 明确301/302读取Location并调用play；`:209–238` 接收Content-Base并保存session control；`:286–301` 直接使用track control发SETUP。`Rtsp.cpp:132–145/:382–389` 允许绝对control。故原版不能满足设计要求，新版拒绝与发送边界校验具有明确落点。
- `RtspPlayer.cpp:718–772` 使用目标URI生成认证及请求，`:98/:772/:851` 有URI、账号密码或原始异常日志；`PlayerProxy.cpp:205/:275` 有源URL或异常日志。文档要求各等级移除并对恶意server取运行日志证据，不能以日志级别代替修补。这里只验证原版行为与补丁设计，未声称安全构建已存在。
- `WebApi.cpp:1348` 默认retry_count=-1；`PlayerProxy.cpp:207/:237/:263–270` 初始失败及断线两条路径均由retry_count控制。设置0可以关闭provider自动重拉，把重连交给重新验权/DNS的持久worker；仍须运行证据验证。
- `RtpServer.cpp:137–156` 显式绑定RTP端口和port+1的RTCP端口，验证20对修订必要且正确。provider实际关闭后才可释放端口对是外部资源生命周期所需 reconciliation，不是无意义setter/getter确认。
- `HlsMakerImp.cpp:157–193` 用日期/小时目录及分钟秒/index形成TS，`HlsMaker.cpp:149` 将其写入manifest。保留受限相对路径能够获取原生段。源码还可给段附加query；本设计禁止query，后续adapter须使用不带参数的私有流配置，这属于现有禁止query要求的实现约束。

## Executed verification / manual QA matrix

| 实际表面 | 操作 | 实际结果与证据 |
|---|---|---|
| 独立发布快照 | detached checkout、git status、tree、diff --check HEAD~3 HEAD | clean；tree如上；diff check exit0 |
| 机器消费OpenAPI | make verify-contracts，使用root既有contracts虚拟环境，但cwd和输入均为独立worktree | exit0，96操作/568合成fixture，`contracts.log` |
| 视频/原物模型schema回归 | 指定两个pytest文件 | exit0，22 passed in 0.89s，`pytest.log` |
| PostgreSQL真实提案 | 专用容器、个人随机库、001..012、proposal、proposal-check.sql | 全部exit0；两租户FK/来源/TTL/状态/hash/RTP/注册及fixture回滚PASS，`database.log` |
| PostgreSQL端口对 | port-pairs-check.sql | 20对接受；奇数/超范围/重复拒绝PASS，`database.log` |
| PostgreSQL段路径 | hls-path-check.sql | 原生日期目录TS接受；遍历/外部/编码/非法布局拒绝PASS，`database.log` |
| 第三方实现 | 固定SHA源码直接检查 | 原版行为与修订依据吻合；副本和`source-sha256.log` |
| 视频UI、真实RTSP/GB/微信、补丁构建 | 本次设计尚无运行artifact | 未执行/未实现；不记PASS，外部依赖external_blocked |

容器为 `iolink-m7a-foundation-20261011-93c1`，镜像tag `timescale/timescaledb:latest-pg16`，实际PG16.15/Timescale2.30.2；不把可变tag当生产pin。只创建并删除 `iq_m7a_final2_gate_64bccc801eba4bb4b6bc88675f310338`，未改既有数据库。迁移中的既有Timescale列类型提示不是本修订失败。无需运行Go/UI全套，因为本次五个变更文件均为设计/提案/SQL证据，生产代码未改。

## Direct remove-ai-slops / programming pass

已实际读取两份可用SKILL.md，并直接检查三次修订完整diff、提案DDL、两个新增SQL suite、既有scripts/test_video_contracts.py和scripts/check_contracts.py；没有仅依赖报告覆盖。

- 过量/无用测试、deletion-only、只验证移除、自然语言词句钉死：未发现于本次新增测试。SQL suite驱动真实PG约束，验证能接受正常布局/20对及拒绝恶意值，不是为删掉某段文本而写的测试。
- 反身/镜像测试：新增SQL fixture不是复写正则判定；通过INSERT/UPDATE观察DB行为，未出现“期望值由同一实现计算”的循环证明。反例用明确SQLSTATE接住约束异常；sentinel RAISE EXCEPTION是P0001，不被check_violation/unique_violation吞掉，约束被放宽会真实失败。
- 既有schema测试检验机器消费契约边界及响应不泄密，有行为价值；不能证明HTTP授权、SSRF或媒体运行，证据文字也未把它们当作运行验收。操作数与合成fixture数只作记录，本结论还依靠完整文档、源码与PG路径审查。
- 不必要生产抽取/解析/规范化：本次未引入生产抽象或代码。日期/时间、URI、XML/SDP的解析均在明确不可信边界，确有SSRF/路径/协议需求；没有以通用插件框架扩张范围。domain/application不依赖SDK的窄ports与现有AGENTS一致。
- programming边界/类型/错误/测试责任检查：DDL使用约束维护组合身份、五分钟TTL及状态；外部未知结果有显式unknown与reconcile；错误输出保持安全类别。此次无新增Go/Python/Rust/TS生产代码，类型检查、枚举穷尽及250纯代码行生产模块门槛无新增适用对象。没有为文档文字添加测试，未引入维护负担或伪信心阻断。

## Code-review report coverage and exact evidence gaps

形成独立判断前未读其他审阅结论。父任务随后明确确认：**暂无 f796fcc 同快照另一位code reviewer报告/收据**，另启审阅尚待执行。因此不能确认另一份报告显式覆盖programming与remove-ai-slops及上述过拟合分类；这是精确的收据缺口，不能伪报覆盖。本次直接完整技能检查支持我自己的设计准入判断，不替代第二位独立审阅。根据门审规则，没有把这一流程中的待第二审状态误作本设计内容失败，也没有把单审当双审已完成。

另外尚不存在/未提供：安全补丁与构建配方SHA256、批准镜像digest、实际能力探测、正常源与恶意RTSP server零跳转/外部control/日志/重连运行证据；真实ZLM/GB/媒体gateway与并发授权矩阵；摄像机及微信真机/TLS/ACL运行证据。design:137–143/:199–215、deploy:32–39/:60–65将这些明确列为后续媒体开启或R43验收前置，故当前设计准入不因此失败。不能将本报告用于解除这些门槛。

非阻断NOTE：DDL日期正则只保证布局与数值范围，允许诸如二月31日；文档明确由应用做严格日期解析，真实PG测试也仅证明DDL范围，不证明运行时日期有效性。后续HLS实现必须遵守这一已经写明的要求。

## Checked artifact paths

- 权威与状态：`AGENTS.md`、`docs/README.md`、`docs/CONTRIBUTING.md`；`docs/PLAN.md`、`docs/EXTENSIONS.md`、`docs/ACCEPTANCE.md`、`docs/IMPLEMENTED.md` 的相关M7/R43/权限/License条目。
- 完整设计：`docs/design/M7a-video.md`、`docs/design/video-proposal.sql`、`docs/api/video-openapi.yaml`、`docs/deploy/M7a-video.md`。
- 三次修订文件：上述design/deploy/proposal加 `docs/evidence/M7a/2026-10-11-design/port-pairs-check.sql`、`hls-path-check.sql`；diff范围 `2fc2b6e..f796fcc`。
- 执行器原证据：`docs/evidence/M7a/2026-10-09/README.md`、`proposal-check.sql`；其静态/PG成功由本次独立复现支持，旧PG版本未套用成本次环境。
- 测试/承载schema：`scripts/test_video_contracts.py`、`scripts/test_m6a_contracts.py`、`scripts/check_contracts.py`、`Makefile`、`internal/migrate/sql/001*` 至 `012*`。
- 本审证据目录：`.omo/evidence/m7a-final2-design-gate-20261011/` 中 `commands.md`、`notepad.md`、`contracts.log`、`pytest.log`、`database.log`、`environment.log`、`source-sha256.log`、`RtpServer.cpp`、`HlsMakerImp.cpp`、`HlsMaker.cpp`、`RtspPlayer.cpp`、`Rtsp.cpp`、`PlayerProxy.cpp`、`WebApi.cpp`。
- ulw-loop status实际返回 `ULW_LOOP_PLAN_MISSING`，使用任务指定fallback报告路径。没有编造attemptDir或其他review报告。
