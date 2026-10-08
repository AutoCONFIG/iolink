# M4 小程序演示候选证据

日期：2026-10-08 Asia/Shanghai

## 范围

按用户最新范围，本候选只覆盖接口演示：六页导航、接口调用入口、API 请求/响应边界、报警列表加载/空态/错误态、报警确认按钮、订阅同意/拒绝/取消演示。正式小程序由专人开发。真实 `wx.login`、code2session、微信订阅弹窗、HTTPS、资质和真机仍为 `external_blocked`。本地真实数据库后端测试另行通过，不等于真机端到端验收。

审阅源码：`2b66812fd0d6e64609c1bc3fccbccccf71a4f99d`，tree `1bd83aabd5ea4ef617cbf5975dea270de67d92ca`。之后的归档提交只更新使用说明、证据、状态和 CI 的 tag 触发配置。

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

逐条命令及输出：[frontend-contracts.log](logs/frontend-contracts.log)。该日志的 `git diff --check` 只检查当时工作树。提交差异 `git diff 8c6eb8b 2b66812 --check` 在生成 JS 的 11、12、20 行报告尾随空格，作为非阻断备注保留；手写源码差异检查通过。

归档后的 manifest/契约和源码一致性复核见 [archive-checks.log](logs/archive-checks.log)。审阅差异原件 `reviews/code-last-snapshot.diff` 保留原始内容，包括生成文件中的空格，不能用归档原件声称提交差异无空格问题。

## 真实后端验证

环境：专属 Docker 容器 `iolink-todo9-pg`，`127.0.0.1:55439`，PostgreSQL 16 / TimescaleDB 2.30.0。DSN 从容器配置在内存中读取，不记录值。

```text
GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated; set, value omitted> go test -race -shuffle=on -count=1 -v ./internal/core ./internal/appapi -run 'Test(M2OwnershipLifecycleAndTokenRevocation|HistoryAndAlarmFiltersUseSnapshotOwnership|AppResourceHTTP|LoginAndListPonds|WaterLatest|AuthRequired|TenantRoleConfirmationMatrix)'
```

结果：exit 0，两包 PASS，无 SKIP。覆盖资源归属/转交、历史与报警过滤、确认角色矩阵、跨租户 404、撤权 401 和拒绝操作无部分状态。命令、环境及输出见 [live-backend.log](logs/live-backend.log)。独立代码审阅另外以原生 Node fetch 验证 JSON 登录、Bearer 和空 204，并用当前 Zod schema 解析 31 个实际后端成功响应，见 [native-boundary.log](reviews/code-last-native-boundary.log)。

## 浏览器 QA

Vite 本地服务：`npm run dev --prefix web-mini -- --host 127.0.0.1`；Chrome 通过 Codex in-app browser 访问 `http://127.0.0.1:5173/`。

1. 初始页面显示“微信登录”、接口演示表单和六个导航按钮；空 code 点击“调用接口”显示校验错误。
2. 首页、池塘、实时水质、历史曲线页均显示对应的接口输入/调用入口；30 秒轮询、后台暂停、前台刷新和卸载释放由组件测试核验。
3. 点击“报警中心”后显示报警标题、刷新按钮及订阅区域。本次浏览器运行没有可用后端，显示“请求失败”，没有同时显示成功空态或伪造报警数据。
4. 分别点击“模拟同意”“模拟拒绝”“模拟取消”均显示相应结果，并保留报警中心和导航。
5. [操作记录](browser/actions.json)、[登录截图](browser/login.jpg)、[报警截图](browser/alarms.jpg)。本轮浏览器只核验导航、输入校验和失败/订阅演示；成功请求由真实 HTTP fixture、组件测试和后端测试分别核验，不声称浏览器到真实微信或后端的成功链路已经通过。

## 外部验收

`R25` 真 code2session、`R26` 真机六页链路、`R27` 真微信订阅弹窗均为 `external_blocked`：当前没有 AppID/Secret、HTTPS 域名、微信资质或测试设备。软件演示通过不替代这些外部证据。

## 审阅门槛

两位未参与源码编辑的独立审阅者针对同一 `2b66812` 快照批准接口演示范围：

- 代码审阅：[/root/m4_code_last](reviews/code-last.md)，APPROVE / WATCH，无阻断项。
- 门审：[/root/m4_gate_last](reviews/gate-last.md)，APPROVE，无阻断项。

旧 `code-review.md` 和 `gate-review.md` 对应 `2bf4205`，仅保留审阅历史，不作为本候选批准。生成文件空格、测试 fixture 的 204 构造和旧页面元数据断言的覆盖局限按审阅报告保留，不计作外部验收。
