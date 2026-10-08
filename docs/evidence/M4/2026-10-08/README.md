# M4 小程序演示候选证据

日期：2026-10-08 Asia/Shanghai

## 范围

本候选只覆盖可运行的软件演示边界：六页导航、API 请求边界、报警列表加载/空态/错误态、报警确认按钮、订阅同意/拒绝/取消演示。真实 `wx.login`、code2session、微信订阅弹窗、HTTPS、资质、真机和真实后端数据仍为 `external_blocked`。

## 源码范围

- `web-mini/src/api.ts`：`WaterLatest` 类型、HTTP JSON/错误边界、401/403/404 映射、204 空响应。
- `web-mini/src/types.ts`：水质影子和报警契约类型。
- `web-mini/src/pages/AlarmsPage.vue`：报警加载、确认、空态/错误态、订阅演示。
- `web-mini/src/domain/alarms.ts`：报警级别、订阅结果和未确认筛选规则。
- `web-mini/tests/domain.test.ts`：API 正向/错误边界及报警计数测试。
- `web-mini/DESIGN.md`：演示页面令牌和状态约定。

## 自动检查

环境：Node.js/npm，Linux；依赖来自已提交 `web-mini/package-lock.json`。

| 命令 | 结果 |
|---|---|
| `npm test --prefix web-mini` | PASS，1 个测试文件，15 tests |
| `npm run typecheck --prefix web-mini` | PASS |
| `npm run build --prefix web-mini` | PASS，Vite 生产构建完成 |
| `python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software` | PASS，`software=passed`，`external=external_blocked` |
| `python3 scripts/check_architecture_manifests.py --all` | PASS，55 requirements |
| `make verify-contracts` | PASS，67 operations / 334 fixtures |
| `git diff --check` | PASS |

## 浏览器 QA

Vite 本地服务：`npm run dev --prefix web-mini -- --host 127.0.0.1`；Chrome 通过 Codex in-app browser 访问 `http://127.0.0.1:5173/`。

1. 初始页面显示“微信登录”和六个导航按钮。
2. 点击“报警中心”后显示报警标题、刷新按钮、订阅区域、空态和错误提示；没有后端登录令牌时安全显示“资源不存在或无权访问”，未伪造报警数据。
3. 点击“模拟同意”后显示“已同意订阅。拒绝或取消不会影响报警中心”。同一页面提供模拟拒绝、模拟取消入口。
4. 截图：本轮浏览器截图已在审阅会话中记录；页面在默认桌面视口无横向溢出，窄屏 CSS 将标题和报警项改为纵向布局。

## 外部验收

`R25` 真 code2session、`R26` 真机六页链路、`R27` 真微信订阅弹窗均为 `external_blocked`：当前没有 AppID/Secret、HTTPS 域名、微信资质或测试设备。软件演示通过不替代这些外部证据。

## 审阅门槛

该候选需由两位未参与编辑的独立审阅者针对同一源码快照明确批准，之后才能把 TODO 8/M4 软件演示记为完成并发布标签。
