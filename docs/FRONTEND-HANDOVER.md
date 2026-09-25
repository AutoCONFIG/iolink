# 管理前端和小程序交接基线

2026-09-19，目标规格。当前管理前端仅占位页，小程序未交付。先读PLAN、PLAN-DETAILS、ACCEPTANCE；两份OpenAPI是目标契约，必须在M2契约测试通过后才能声称后端联调就绪。

## 管理前端

固定Vue3 + TypeScript + Vite + Element Plus，自建脚手架。源码在独立仓库 https://github.com/AutoCONFIG/iolink-webui （git.hyhy.fun/rsplab/iolink-webui 为内部镜像），以 git 子模块形式挂载在 web/（子模块仓库根 = web/，`npm run build` 产物输出到其 dist/，即 web/dist）。主仓库构建时由 scripts/embed-frontend.sh 把 web/dist 镜像到 internal/web/dist 打入 iolinkd 单二进制（子模块没有 dist 时自动生成兜底占位页）；主仓库 clone 后需执行 `git submodule update --init` 或 `make bootstrap`。下表页面ID是唯一范围，不再用“6+1/6+2”描述。

| 页面ID | 页面/建议路由 | 接口与验收要点 |
|---|---|---|
| A01 | 登录 /login | POST /admin/v1/login；401回登录，首启/强制改密另有引导 |
| A02 | 总览 /dashboard | GET /stats + /ponds；critical/warning/normal及latest；无数据和过期属性明确显示 |
| A03 | 农场和用户分配 /farms | farms CRUD、users检索、PUT farms/{id}/owner；未分配提示；转交/解除二次确认并说明历史权限变化 |
| A04 | 池塘 /ponds | ponds CRUD；有任何设备（含停用）/历史/规则/报警引用409时显示具体原因，不提示直接删历史 |
| A05 | 设备 /devices | 注册（可选60/300秒周期）、列表、详情、调塘、停用；secret一次性展示与复制，关闭后不再请求明文；调塘提示历史保留原塘 |
| A06 | 规则 /alarms/rules | alarm-rules CRUD；metric/上下限/level/enabled；min<max及至少一端；level由规则选择 |
| A07 | 报警 /alarms | 列表level/only_unconfirmed/分页，单条/批量确认；批量失败整批不更新 |
| A08 | 系统 /system | POST /password；更改后清理旧token重新登录；版本、依赖与通知状态 |

A03与A04是独立业务页面；后续M6增加产品/模型、组织/成员/角色、License/首启、开放Key管理，M7增加视频/地图/大屏，M8增加协议/网关/命令/转发/调度/联动/调试/报表页面，对应R34–R54。不能以完成A01–A08就宣称M0–M8所有UI完成。

## 接口与状态

- 管理前缀/admin/v1，小程序/api/v1；成功JSON不包code/data，失败{error}，空列表[]，时间UTC RFC3339，UI按Asia/Shanghai显示。
- 管理集合limit/offset，稳定排序，过滤再分页；无效参数400。认证无效401，资源不存在/跨范围404；M6已认证动作不足403。无数据latest为null，不能当成0。
- latest按属性合并，timestamps给逐字段时间，report_interval给设备周期；超过3倍周期标过期，即使包ts较新也不能把旧字段显示为新值。
- 表单按目标OpenAPI校验；响应契约检查失败视为联调缺陷，不在前端静默改名猜字段。
- 管理鉴权token仅保存在运行时内存，页面刷新重新登录；不在日志/URL保存token。小程序使用平台安全存储并在登出/过期清除。
- Vue开发用Vite代理/admin和/api到本机后端；生产同源，管理路由避免与/admin/v1冲突；不为开发直接放开生产CORS。

## 小程序

页面P01登录、P02首页、P03池塘、P04实时、P05历史、P06报警。微信登录先建档，未分配显示联系管理员；由A03分配后才出现数据。实时30秒轮询、切后台暂停，回前台刷新；历史范围today/7d/30d，max_points<=200、展示unit；报警支持确认和订阅引导，拒绝订阅仍可查看。

M7增加定位/视频入口，真实微信播放资质和地图Key按EXTENSIONS外部输入表验收；PC mock不能替代真机结果。

## 联调与构建关卡

1. M0/M2先修复旧schema和契约差异，建立隔离数据库及管理员/微信用户A/B测试样本；不依赖旧文档不存在的dev-001种子数据。
2. 页面mock直接按目标schema生成；对真实后端完成A01–A08和P01–P06，每一步记录网络响应与页面证据。
3. `注册设备→mqtt-sim使用新secret→看属性→触发阈值→确认报警→转交农场→验证A失权B可见`作为共同验收。
4. 前端构建锁定依赖，dist来自源码，不提交凭据；CI先构建前端再go:embed。刷新非API路由能显示页面，未知API返回JSON404而不是SPA HTML。
5. 整体通过R23–R27后记录证据；当前make dev/verify均不是已经完成整改的保证，先看IMPLEMENTED当前限制。
