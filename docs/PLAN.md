# IoLink 开发计划与路线图(v2 · 彻底原生开发路线)

> 文档入口:[docs/README.md](README.md)(地图) · 细节附件:[PLAN-DETAILS.md](PLAN-DETAILS.md)(需求追溯+各层设计+验收清单)

> v2 修订于 2026-09-16。**路线决策已确认:彻底原生开发(路线 C)**——所有功能原生自研,
> 单程序交付;ThingsPanel(upstream/thingspanel/)仅作功能设计参照,不进运行时。
> 评审依据:docs/proposal-baseline-final.md(路线对比)+ docs/baseline-spike-report.md(基座实测,留档)。

## 0. 已定架构决策(不再重复讨论)

| 决策点 | 结论 |
|---|---|
| 架构形态 | **三段式:传感器(含终端)→ 本服务器(iolinkd 自研独立进程)→ 小程序**;单程序=本服务器一个二进制(内嵌 broker + 业务核心 + 双 API + go:embed 前端) |
| 仓库拓扑 | **单仓**:access/appapi 模块仓并回主仓(边界=Go package);contracts 并回 internal/domain |
| 功能实现 | 全部原生自研(接入/管道/报警/双端 API/管理后台/小程序);替代原「商业云快速实现」路径,不嵌第三方平台 |
| 数据库 | PostgreSQL 16 + TimescaleDB 单实例:业务表 + sensor_data 时序宽表 + device_shadows 影子表 |
| 前端 | 管理后台 Vue3(soybean-admin/vben 模板 + Element Plus);小程序 uniapp |
| 升级触发器 | OTA / 真多租户 / 复杂协议矩阵 任一出现 → 重开基座评估;/api/v1 契约不变 |

## 1. 里程碑

### M0 — 契约与骨架 ✅(2026-09-16)
- [x] 契约三件套(openapi / mqtt-spec / schema)+ contracts 模块 + core 骨架 + TimescaleDB 建表

### M1 — 端到端数据链路 ✅(2026-09-16)
- [x] 内嵌 mochi-mqtt broker、三元组鉴权(sha256)、topic ACL、离线看门狗
- [x] 管道落库 + 池塘阈值报警引擎(去重)+ 全部 appapi 接口 + 微信 code2session 客户端
- [x] 单进程组装(iolinkd);实测:模拟器→落库→低DO→critical报警→报警中心→确认,全链路绿
- 期间成果:thingspanel spike 实测(基座评估,报告留档);基座决策:不采用,转参照

### M2 — 并仓 + 管理后台 API(当前)
- [x] **并仓(2026-09-16 完成)**:
  - [x] 代码搬运:internal/access、internal/appapi、internal/{domain,event,wire}
  - [x] 回归:build/vet/test 全绿;iolinkd 单二进制冒烟通过(broker+API)
  - [x] 完整历史备份:~/iolink-repo-backups/*.bundle(GitLab 删除前的保险)
  - [x] CONTRIBUTING.md 已改为单仓口径
  - [ ] GitLab 上删除 iolink-access / iolink-appapi(需所有者操作:项目 Settings→General→Advanced→Delete;或先 Archive)
- [ ] internal/adminapi:/admin/v1 —— 管理员登录、池塘 CRUD+绑设备、设备注册(生成 device_no+secret)、报警规则 CRUD、报警管理(列表/确认/批量)
- [ ] 设备影子表 device_shadows(上报 UPSERT,/water/latest 改读影子)
- [ ] 配套:docker-compose 加 admin 路由说明;openapi 拆为 app 与 admin 两份
- 验收:curl 全套 /admin/v1;mqtt-sim→影子→latest 正确
- 负责:主仓(adminapi)+ access 负责人(并仓/搬运与回归测试)

### M3 — 管理后台前端(Vue3)
- [ ] web/admin 脚手架:soybean-admin 或 vben 模板(拍板项3)+ Element Plus
- [ ] 页面:登录 / 总览(池塘状态墙+统计) / 池塘管理 / 设备管理(注册发密钥) / 报警规则 / 报警中心
- [ ] go:embed 打进 iolinkd,单二进制交付;CI 加前端 build
- 验收:浏览器完成"建池塘→注册设备→配规则"全流程
- 负责:前端(新)+ 主仓(adminapi 配合)

### M4 — 微信小程序
- [ ] uniapp 工程(参照 thingspanel/app 的工程化),6 页面:登录/首页/池塘/实时/历史/报警
- [ ] 真实微信 code2session 联调(IOLINK_WX_* 凭据)
- [ ] uCharts 历史曲线;报警中心+确认
- 验收:真机预览全流程走通
- 负责:appapi 负责人 + 小程序开发

### M5 — 通知与生产化 ✅ 后端部分(2026-09-16;订阅消息实发需微信凭据联调)
- [x] Notifier:core.AlarmNotifier 接口 + WeChatNotifier(订阅消息,配置驱动,未配置自动跳过)
- [x] /metrics(prometheus):iolink_devices_online / telemetry_total / alarms_total / notifications_total
- [x] 池塘聚合字段:status(最重未确认报警)+ latest(影子最新值)—— appapi /ponds 与 adminapi /ponds
- [x] 部署定型:Dockerfile(多阶段)+ deploy/docker-compose.prod.yml + deploy/backup.sh(业务/时序分离,留14天)+ docs/DEPLOY.md(TLS/备份/监控/升级)
- [x] 前端嵌入:web/admin/dist 占位页 + go:embed + SPA fallback(iolinkd 单二进制含后台)
- [ ] 微信订阅消息真机实发(需小程序凭据+模板,随 M4 联调)
- 负责:主仓

## 2. 分工(按交付物划分,三人协同参考)

> 原则:剩余工作按「交付物」切四条并行线,每条线有独立的验收物;线间只靠契约(openapi / admin-api 契约 / mqtt-spec)耦合,契约先行冻结即可全并行。
> 与原 access/appapi 两仓分工无关——那两仓已并回主仓(2026-09-16),原负责人按下表重新认领。

| 线 | 交付物 | 内容 | 技能 | 建议人选 | 工期估计 |
|---|---|---|---|---|---|
| **L1 后端线** | 可部署的 iolinkd | M2:并仓(机械)、/admin/v1 全套、设备影子表;M5:订阅消息 Notifier、备份/metrics、部署定型 | Go(主) | 主程(你) | 4~5 天 |
| **L2 管理前端线** | web/admin | Vue3 模板脚手架 + 6 页面(登录/总览/池塘/设备/规则/报警),go:embed 联动 | Vue3/TS | 前端 | 1.5~2 周 |
| **L3 小程序线** | uniapp 小程序 | 6 页面 + 微信登录联调 + uCharts 曲线 | uniapp/Vue | 前端(可兼)或第三人 | 1.5~2 周 |
| **L4 硬件线** | 固件 | Modbus 采集 → 按 mqtt-spec 上报(先模拟器后真机) | 嵌入式 | 硬件方 | 与 L1 联调 |

**协同节奏**(关键路径 = L2/L3 前端,所以契约最优先):
1. L1 第 1 天先冻结 `/admin/v1` 契约(照 openapi.yaml 的风格补 admin 部分),L2/L3 立即用 mock 开工
2. 三线每日集成:L1 合代码,L2/L3 对真实接口联调
3. 人数弹性:只有 2 人 → L2+L3 合并给同一人(都是 Vue 系);>3 人 → L1 拆出"并仓机械活"给新人练手

**按 package 的代码所有权**(并仓后 MR 评审归属):
internal/access、cmd/mqtt-sim → L1 兼(原 A 可认领);internal/core、internal/adminapi → L1;
internal/appapi → L3;web/admin → L2。

### M6 — 商业化前置(License/多产品/OpenAPI;多租户随订阅制启动)

> 商业模式决策(2026-09-17):**买断制 + 订阅制双行**(详见 FEATURE-MATRIX §二·五)。
> 买断制先行(私有化部署形态已就绪,水产行业偏好数据自持);订阅制后行(客户量起来再启动)。

**买断制开卖前置(先行):**
- [ ] **License 授权机制**:license 模块(RSA 签名授权文件:有效期/设备数上限/功能开关),启动与运行时校验,超限行为(拒绝新设备注册+告警日志);admin 后台授权信息页
- [ ] 私有化交付物定型:离线安装包(镜像+compose+安装脚本),含 schema 初始化与首启向导
- [ ] 多产品/物模型抽象:products + product_models 表,物模型字段由固定水质五参扩展为产品级定义(现有水质终端迁移为首个产品)
- [ ] OpenAPI 开放平台:第三方应用 API-Key 签发/签名/限流,开放接口文档(买断制客户对接政企平台同样需要)

**订阅制启动项(后行,客户量起来再实施):**
- [ ] 多租户:tenants 表,users/farms 挂 tenant_id;全部查询按租户隔离;admin 后台租户管理
- [ ] 细粒度 RBAC:casbin(角色×资源×动作),管理后台角色管理页

- 验收:①License 过期/超设备数时行为正确;②离线包在干净机器 30 分钟内部署完成;③第三方持 API-Key 调开放接口
- 负责:L1 后端 + 前端(授权信息页)

### M7 — 行业标配(视频/地图/大屏)
- [ ] 视频接入:GB28181/RTSP → 流媒体网关(ZLMediaKit),设备关联摄像头,后台/小程序播放
- [ ] 设备地图:GIS 定位(设备/池塘经纬度),地图页(后台+小程序)
- [ ] 数据大屏:聚合只读 API + 大屏前端(布局参照 thingsvis 思路)
- 验收:地图看到设备分布;大屏实时刷新;摄像头可点开播放
- 负责:前端为主 + L1(接口/信令)

### M8 — 增强能力(数据面与协议面补全)
- [ ] 数据转发:规则引擎 out(HTTP/MQTT 推送,按规则配置)
- [ ] HTTP 协议接入:HTTP 上报端点(签名鉴权,复用物模型白名单)
- [ ] Modbus 平台侧:平台 Modbus 主站直连 DTU(视硬件方案,与固件线对齐后启动)
- [ ] 定时任务:用户可配 cron(联动触发,如定时下发命令)
- [ ] 场景联动:条件→动作编排(触发条件复用告警条件模型)
- [ ] 网关+子设备接入:两级拓扑(gateway→sub-devices),数据归因子设备
- [ ] 设备调试台:Web 下发命令/查看原始报文
- [ ] 数据报表:历史导出 CSV/Excel + 周期日报
- 验收:逐项对应蜂鸟清单功能位,FEATURE-MATRIX.md 状态同步
- 负责:L1 + 前端(按项拆分)

## 3. 协作与流程变化

- 并仓后跨仓 go get / tag 流程取消;**接口评审 = 主仓 MR(internal/domain 或 /admin/v1 契约)**
- CONTRIBUTING.md 将随 M2 并仓同步改版(环境配置一节保留:netrc/GOPRIVATE 用于拉取 upstream 参照与 CI)
- 硬件接入规范 mqtt-spec.md 不变(M1 已按它验收)

## 4. 规模假设与升级触发条件

- 设计容量:<500 设备、每分钟上报(≈14 万行/天),单实例余量充足
- 升级触发:设备>500 / 设备远程控制 / 多租户商业化 / 复杂协议矩阵 → 优先方案:保持单程序扩容(资源加配);再不够,按 contracts 边界拆独立进程,设备与小程序零改动
