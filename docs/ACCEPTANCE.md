# 需求与验收台账

2026-09-19 重建基线。范围：用户本次确认的 M0–M8；需求来源类别 BASE 为原项目监测需求转述（原稿缺失）、EXT 为既有商业化路线、AUDIT 为本次审查发现的必要质量要求。各行状态和证据以 IMPLEMENTED.md 为准；表中测试是验收规格，不能把列出的用例本身当作已执行结果。

## 状态与证据规则

实现状态仅用：未实现、部分实现、实现待验收、验收通过、外部阻塞。外部阻塞行另记软件子项状态；“验收通过”必须同时满足适用的软件和真实环境验证。文档双审通过只使需求基线可用，不改变实现状态。

每项证据记录必须包含：ID/需求版本、提交或工作树SHA256、环境/依赖精确版本、测试用例与数据、执行命令、预期/实际、日志或截图路径、执行日期、缺陷ID、审查人。缺少证据不得勾选。证据存 docs/evidence/<日期>/<ID>/，尚未创建的目录不能作为已验收引用。

环境代号：U=单元/HTTP fake；D=隔离PG16+Timescale真实数据库；I=运行iolinkd+真实MQTT客户端；W=浏览器；X=真实微信；H=实际终端/DTU/摄像机；O=隔离部署恢复机。所有需持久化的用例必须D验证，mock通过不能替代D/I。

## 分阶段子项（避免前置关卡依赖未来功能）

以下主需求跨阶段累计验收，必须按子项存证；前一阶段只通过指定子项，不能把未来部分勾完成。主需求整体仍为部分实现/实现待验收，直到全部子项通过。

- R02.a（M0）：文档标准/引用/合成请求响应fixture和校验框架；R02.b（M1）：真实MQTT契约；R02.c（M2）：35个目标HTTP操作真实handler回归。
- R10.a（M1）：存储层影子合并/失效及读取；R10.b（M2）：调塘API与双端latest端到端。
- R36.a/R37.a（M6b）：租户/RBAC底座、当时已存在的用户/农场/池塘/设备/产品/模型/遥测/报警/审计资源；用fake执行器验证上下文传递，不能声称真实未来job已通过。
- R36.b/R37.b：未来资源随各自阶段必测同一租户与角色矩阵：Key在M6d（R41–R42）、视频/地图/大屏在M7（R43–R45）、命令在M8a（R46）、协议/网关在M8b（R47–R49）、jobs/转发/联动/调试在M8c（R50–R53）、报表在M8d（R54）。
- R39.a（M6c）：核心设备配额、授权状态和各feature守卫单元测试（future executor使用fake）；R39.b：各可选功能真实HTTP/worker入口随M6d/M7/M8对应Rxx复验，未实现入口不记通过。
- 所有扩展Rxx的退出条件包括适用的R36.b/R37.b/R39.b，R55核对这些子项全部齐全。不得为M6b/M6c通过而提前验收未来资源。

共用负面关卡 N：未认证401；资源越权404、已认证动作无权限403；输入非法400；冲突409；失败不产生部分不可见写入；停用/撤权在下一次操作生效；重启后已提交数据保持。Rxx引用N表示该项所有适用操作都必须测，不是任意抽一例。UI同时验证空数据/加载/失败可重试/会话过期；异步项同时验证超时/重复/重启。

## M0–M2：基线、链路和接口

| ID/来源 | 阶段与需求 | 成功与边界验收（全部满足） | 环境；实现落点 |
|---|---|---|---|
| R01 AUDIT | M0 可复现构建测试 | 干净检出运行build/vet/tests通过；make verify无已删除目录；CI同命令；无缓存也可重现 | U/O；Makefile、CI |
| R02 AUDIT | M0/M1/M2 分层契约验证 | M0仅R02.a静态标准/引用/合成fixture；M1完成R02.b真实MQTT字段/ACL回归；M2完成R02.c全部目标handler请求/响应回归；无空响应定义 | U；docs/api、mqtt-spec |
| R03 AUDIT | M0 空库/旧库迁移 | 空库逐版迁移后正常上报；旧schema保数据升级；重复执行无副作用；checksum改变/并发迁移/故意失败均按设计拒绝或回滚 | D/O；internal/migrate、遗留schema |
| R04 AUDIT | M0 首启和配置 | 缺生产secret/非法配置拒绝；首启无公开默认密码；env中全局默认上报周期/离线阈值生效（逐设备注册由M2验收）；用户输入不入敏感日志 | U/I/O；platform、cmd/iolinkd |
| R05 BASE | M1 设备认证 | 合法凭据连接；错secret、ClientID不匹配、已停用设备拒绝；第二会话按唯一身份规则替换并保持正确状态；注册secret仅一次返回 | U/D/I；access、core/deviceauth |
| R06 BASE | M1 Topic ACL/伪造 | 自己properties/ack可发、cmd可订；跨设备/通配符/伪造子设备/非法topic拒绝；认证前不产生online | U/I；access/broker/auth |
| R07 BASE | M1 物模型字段 | 七字段正常/边界；越界丢字段并可观察；坏JSON/类型/64KiB超限拒绝；全无有效字段不写数据；signal -120..0 | U/D/I；wire、access、pipeline |
| R08 BASE | M1 状态看门狗 | 60/300秒配置；连接在线、断开离线、3×interval超时一次离线；重新上报恢复；重复断开/服务重启在线数不为负且与库一致 | U/D/I；access、core/metrics |
| R09 BASE | M1 遥测持久化 | 上报字段/时间/塘快照落库正确；故障注入时遥测与影子同事务；有message_id去重、无ID至少一次语义明确 | D/I；core/pipeline |
| R10 BASE | M1 设备影子 | 部分字段上报保留其他字段及各自时间；设备/池塘latest读影子；M1存储层失效、M2调塘API清空分别验收；timestamps和report_interval满足契约 | D/I；core/repos、admin_store |
| R11 BASE | M1 历史查询 | today/7d/30d按业务日；max_points=1/200均不超限；unit正确；无数据[]；非法range/metric400；输入不能SQL注入 | U/D/I；appapi、core/repos |
| R12 BASE | M1 阈值与分级 | min/max分别及同时设定；等于不触发；缺字段跳过；不合法上下限拒绝；threshold/message为实际命中端；level遵循规则 | U/D/I；core/alarm、adminapi |
| R13 BASE | M1 去重和确认 | 同设备/同塘/同指标并发100次仍仅一未确认；A塘旧报警未确认不阻止B塘新报警；确认幂等；确认后再超限再生成；事务失败不留孤立通知 | D/I；alarm、schema约束/outbox |
| R14 BASE | M2 管理登录/改密 | 正确登录、错误口令401、app/admin token互斥；过期/无exp拒绝；慢哈希迁移；改密后旧token失效；响应无哈希/secret | U/D/I；adminapi、platform |
| R15 AUDIT | M2 用户分配闭环 | 微信新用户A建档→管理员检索/建场分配→A看到池塘→B不可见→转交B后A立即不可见；解除后无人可见且暂停通知 | D/I（fake微信仅软件）/X；users、farms owner契约 |
| R16 BASE | M2 农场/池塘管理 | 创建/修改/合法删除；未知对象404；有任何设备（含停用）/历史/规则/报警引用409；DELETE已不存在204；不会静默删历史 | U/D/I；adminapi/core |
| R17 BASE | M2 设备注册/详情 | 唯一device_no、64hex随机secret、摘要库存；注册需合法pond；可选report_interval=60/300、缺省继承全局且详情返回；设备详情含latest；列表空[]且限制offset/limit；N | U/D/I；admin_store、API |
| R18 AUDIT | M2 设备调塘/停用 | 旧连接撤销、原子调塘、影子清空；旧历史按旧塘权限；A塘未确认报警调塘后B同指标独立报警，A/B各自确认；停用拒绝连接/控制、保留历史；include_disabled可见；N | D/I；device生命周期 |
| R19 BASE | M2 规则管理 | CRUD和enabled；每塘/指标唯一；非法metric/level/无阈值/min>=max拒绝；更新立即影响下次采集；N | U/D/I；adminapi、alarm_rules |
| R20 AUDIT | M2 所有用户资源隔离 | A/B对池塘详情/设备详情/latest/history/确认报警逐一互访均404且未改库；列表过滤正确；转交/调塘后仍按当前授权和历史快照；新旧塘报警分别只能各自owner读/确认 | D/I；appapi、所有repository |
| R21 BASE | M2 报警中心 | limit/offset/level/only_unconfirmed生效；单确认204；批确认全部成功或任一失败整体不变；不存在404；N | U/D/I；双API |
| R22 BASE | M2 统计/状态墙 | critical优先；历史旧的未确认报警不被200/500条截断；alarm_devices按设备去重；停用不计在线设备；两端latest一致 | D/I；API聚合 |

## M3–M5：界面与基础交付

| ID/来源 | 阶段与需求 | 成功与边界验收 | 环境；实现落点 |
|---|---|---|---|
| R23 BASE | M3 管理后台全页 | 按FRONTEND-HANDOVER页面ID逐页走登录→用户分配/建场池塘→设备→配规则→看数据/报警→确认→改密；错误/空态完整；密钥仅一次弹窗 | W/I；web/admin（待建） |
| R24 BASE | M3 嵌入构建 | 源码锁依赖构建dist再embed；路由刷新可用；API不存在返回JSON404不回HTML；生产无占位页；CI前端构建 | U/W/O；web/embed、Dockerfile、CI |
| R25 BASE | M4 微信登录 | 真code2session、无效/过期code失败；首次幂等建档；重复登录同uid；断网/拒绝授权可恢复；无归属首页引导 | U/X/D；uniapp、appapi |
| R26 BASE | M4 六业务页 | 登录/首页/池塘/实时/历史/报警逐页真机走通；实时30秒轮询、旧字段标过期；曲线单位/时间/点数正确；转交后不可越权 | X/I；小程序（待建） |
| R27 BASE | M4 订阅授权 | 用户明确触发订阅引导；同意/拒绝/取消各有处理，拒绝不阻止报警中心；续登不伪造已授权 | U/X；小程序 |
| R28 BASE | M5 报警通知 | A归属池塘报警→A真微信收到正确内容/数值/时间；B不收；转交重验接收人；无配置disabled；HTTP/解码/微信errcode失败不计成功；重试/重启/去重有记录 | U/D/I/X；notify/outbox |
| R29 BASE | M5 健康/指标/关闭 | health反映DB连通，就绪检查迁移/依赖；online重启后正确，telemetry/alarm/通知成功计数可核对；SIGTERM有界关闭，不丢已提交数据 | D/I/O；main/metrics |
| R30 BASE | M5 TLS部署 | 新机仅443/8883及授权视频端口开放，DB/内部API不可公网直达；HTTP/MQTT证书验证正确，错误证书拒绝；无默认密码 | I/O；deploy |
| R31 AUDIT | M5 一致备份恢复 | 全库备份在隔离空库恢复；业务/时序行数、min/max时间、影子、报警、索引、retention核对；损坏备份明确失败；RPO<=24h，100GiB验收集RTO<=2h | D/O；backup/restore脚本（待修/建） |
| R32 AUDIT | M5 升级/回退 | 旧版本数据升级保留；故意失败停止服务/保持旧库或恢复备份；二进制回退遵守schema兼容；文档每命令可重放 | D/O；迁移和发布工具 |
| R33 AUDIT | M5 容量 | 按PLAN的500设备/24h/20查询客户端/90天历史；查询p95<2s、错误<0.1%、无未解释数据丢失；13个月磁盘预算附测量依据 | I/D/O；负载工具（待建） |

## M6–M8：扩展交付

| ID/来源 | 阶段与需求 | 成功与边界验收 | 环境；实现落点 |
|---|---|---|---|
| R34 EXT | M6a 产品/模型 | 发布/版本/设备分配后台可用；第二产品数值与枚举采集→影子→查询→报警；非法类型/单位变更拒绝；模型版本历史稳定 | U/D/I/W；products/models（待建） |
| R35 EXT | M6a 旧水质兼容 | 旧固件与v1接口全回归；旧数据幂等回填计数与样本一致；迁移中断可恢复；动态API不能绕过指标白名单 | D/I；通用telemetry与兼容投影（待建） |
| R36 EXT | M6b 多租户 | M6b先验R36.a现有资源互访拒绝；R36.b的Key/播放/命令/jobs/报表在所属阶段验证；默认租户迁移无丢失；一用户多组织切换检查membership；停租户阻断接入/任务 | D/I/W；tenant/membership（待建） |
| R37 EXT | M6b RBAC | EXTENSIONS权限表逐角色读写；R37.a撤权旧token及fake执行器失效；R37.b真实待执行job/播放/下载在所属阶段验证；平台管理员默认不可读租户数据；支持授权有期限与审计 | U/D/I/W；权限策略（待建） |
| R38 EXT | M6c License | 有效/永久/伪造/缺失/未生效/过期/时钟回拨/错实例逐状态表；无效导入不替换有效证书；签发私钥不进运行包 | U/D/I/W；授权/离线签发（待建） |
| R39 EXT | M6c 配额/功能 | R39.a并发注册不超设备数（网关/子设备用配额模型fixture）；真实网关/子设备计数随R49复验；停用释放、恢复重验；过期维持存量基础采集报警，控制/开放等按表拒绝；R39.b实际可选HTTP/worker随各功能阶段复验 | D/I；配额与feature守卫（待建） |
| R40 EXT | M6c 离线交付 | 已装Docker的干净断网x86_64机器30分钟首启；自设管理员、默认租户、授权、迁移、上报成功；校验和失败拒绝；版本/许可证/依赖清单齐全 | O/W/I；离线包/向导（待建） |
| R41 EXT | M6d Key管理 | 签发仅一次显示、加密库存、轮换/撤销即时生效、scope和资源范围；未授权Key跨租户404；日志无secret | D/I/W；开放平台（待建） |
| R42 EXT | M6d 签名/限流 | 固定签名向量通过，改body/过时/nonce重放拒绝；60次/分burst10超限429+Retry-After；重启与并发nonce一致；合法业务请求成功 | U/D/I；open/v1（待建） |
| R43 EXT | M7a 视频 | RTSP与GB28181-2016各一路注册/拉流/保活/重连；H264/AAC浏览器和微信真机播放；5分钟令牌过期/撤权拒绝；源凭据不出前端 | U/I/W/X/H；camera/媒体信令（待建） |
| R44 EXT | M7b 地图 | 后台和小程序定位/点选；WGS84存储与GCJ02显示对照；权限过滤、无坐标/无Key/断网明确；两租户无泄漏 | W/X/I；地图（待建） |
| R45 EXT | M7b 大屏 | 数据与库一致、10秒刷新、断网显示更新时间；20并发p95刷新<2s；只读token不能写且可吊销 | W/D/I；大屏（待建） |
| R46 EXT | M8a 命令 | queued/sent/成功/失败/过期/取消全状态；重复提交同命令、重复回执不重复效果；离线/超时/重启/迟到回执；未知结果不盲重试物理动作；N | U/D/I/H；commands（待建） |
| R47 EXT | M8b HTTP与TCP桥接 | HTTP签名/模型校验→同一存储报警链；旧secret显式轮换；裸TCP样例经网关转MQTT且按规范归属；篡改/重放/错误桥接拒绝 | U/D/I/H；ingest/网关固件（待建） |
| R48 EXT | M8b Modbus | TCP和RTU各样例；端序/scale/寄存器对照实际值，断线/重试不造新值；写操作经过命令权限且未知结果可见 | U/I/H；Modbus适配（待建） |
| R49 EXT | M8b 网关子设备 | 注册拓扑→两子设备独立数据/影子/报警；跨网关/租户伪造拒绝；断开状态传播、单设备过期独立；下行正确归因 | D/I/H；gateway/subdevice（待建） |
| R50 EXT | M8c 数据转发 | HTTP和MQTT各正确收包；event_id重复可去重；失败指数重试后失败队列、重启接管/人工重放；地址限制和凭据脱敏 | U/D/I/W；forward/outbox（待建） |
| R51 EXT | M8c 场景联动 | AND/OR、持续时长/冷却、命令和转发；直接/间接环拒绝、最大4跳；禁用不启动新动作；重启不重复建立job；N | U/D/I/W；automation（待建） |
| R52 EXT | M8c 定时 | cron每分钟/时区正确；重复worker同scheduled_at仅一个job；停机仅补最近且<=5分钟；禁用/修改生效；授权撤销不执行 | U/D/I/W；scheduler（待建） |
| R53 EXT | M8c 调试台 | 查看自己的脱敏报文/命令/回执、24h保留与64KiB限制；跨租户/只读角色不可下发；无绕过命令服务的publish口 | W/I/D；debug（待建） |
| R54 EXT | M8d 报表 | CSV/XLSX及日报与SQL计数/值/时间边界一致；缺失/负数/公式文本正确；>100万行拒绝；异步重试无重复、下载越权/过期拒绝、7天清理 | U/D/I/W；reports（待建） |
| R55 AUDIT | 全量交付 | R01–R54软件和适用外部证据齐全、缺陷关闭；两独立审阅员对相同发布版本逐项验收通过；安装/运维/用户手册与产物一致 | 全环境；发布清单（待建） |

## 每阶段契约细化交付模板

M6–M8每项进入编码前增加其OpenAPI和迁移提案，至少含路由/字段/单位/nullable、角色资源权限、状态转换、失败/重试、前端页面和外部依赖。由该项Rxx和EXTENSIONS固定语义检查，不得通过省略失败用例降低要求。该关卡通过只表示可开始该项实现，Rxx仍为未验收。
