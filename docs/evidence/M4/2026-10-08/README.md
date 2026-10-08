# M4 小程序演示候选证据

日期：2026-10-08 Asia/Shanghai

## 范围

本候选只覆盖可运行的软件演示边界：六页导航、接口调用入口、API 请求/响应边界、报警列表加载/空态/错误态、报警确认按钮、订阅同意/拒绝/取消演示。真实 `wx.login`、code2session、微信订阅弹窗、HTTPS、资质、真机和真实后端数据仍为 `external_blocked`。

## 源码范围

- `web-mini/src/api.ts`：`WaterLatest` 类型、HTTP JSON/错误边界、401/403/404 映射、204 空响应。
- `web-mini/src/response-schemas.ts`：登录、池塘、实时、历史、报警和物模型响应的运行时结构解析。
- `web-mini/src/ApiDemo.vue`：六页共用的最小接口调用入口、返回 JSON 展示和轮询演示。
- `web-mini/src/types.ts`：水质影子和报警契约类型。
- `web-mini/src/pages/AlarmsPage.vue`：报警加载、确认、空态/错误态、订阅演示。
- `web-mini/src/domain/alarms.ts`：报警级别、订阅结果和未确认筛选规则。
- `web-mini/tests/domain.test.ts`：API 正向/错误边界及报警计数测试。
- `web-mini/tests/api.test.ts`：真实本地 HTTP fixture、状态码、结构错误、网络故障、空响应和非法输入。
- `web-mini/tests/pages.test.ts`：报警加载/确认/失败/订阅和六页接口演示交互。
- `web-mini/DESIGN.md`：演示页面令牌和状态约定。

审阅修复：候选初审发现登录 JSON 缺少 `Content-Type` 且错误态同时显示空态；已在 `createFetchRequest` 自动补充 JSON 请求头，并让报警空态只在无错误时显示。测试新增请求头断言。

## 自动检查

环境：Node.js/npm，Linux；依赖来自已提交 `web-mini/package-lock.json`。

| 命令 | 结果 |
|---|---|
| `npm test --prefix web-mini` | PASS，3 个测试文件，39 tests |
| `npm run typecheck --prefix web-mini` | PASS |
| `npm run build --prefix web-mini` | PASS，Vite 生产构建完成 |
| `npm audit --prefix web-mini --omit=optional` | PASS，0 vulnerabilities |
| `python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software` | PASS，`software=passed`，`external=external_blocked` |
| `python3 scripts/check_architecture_manifests.py --all` | PASS，55 requirements |
| `make verify-contracts` | PASS，67 operations / 334 fixtures |
| `git diff --check` | PASS |

## 浏览器 QA

Vite 本地服务：`npm run dev --prefix web-mini -- --host 127.0.0.1`；Chrome 通过 Codex in-app browser 访问 `http://127.0.0.1:5173/`。

1. 初始页面显示“微信登录”、接口演示表单和六个导航按钮；空 code 点击“调用接口”显示校验错误。
2. 首页、池塘、实时水质、历史曲线页均显示对应的接口输入/调用入口；实时页保留 30 秒轮询并在后台隐藏时暂停。
3. 点击“报警中心”后显示报警标题、刷新按钮、订阅区域、空态和错误提示；没有后端登录令牌时安全显示“资源不存在或无权访问”，未伪造报警数据。
4. 点击“模拟同意”后显示“已同意订阅。拒绝或取消不会影响报警中心”。同一页面提供模拟拒绝、模拟取消入口。
5. 截图：本轮浏览器截图已在审阅会话中记录；页面在默认桌面视口无横向溢出，窄屏 CSS 将标题和报警项改为纵向布局。

## 外部验收

`R25` 真 code2session、`R26` 真机六页链路、`R27` 真微信订阅弹窗均为 `external_blocked`：当前没有 AppID/Secret、HTTPS 域名、微信资质或测试设备。软件演示通过不替代这些外部证据。

## 审阅门槛

该候选需由两位未参与编辑的独立审阅者针对同一源码快照明确批准，之后才能把 TODO 8/M4 软件演示记为完成并发布标签。
