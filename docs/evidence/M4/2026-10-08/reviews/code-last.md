# M4 独立代码审阅：最终冻结候选

结论：**APPROVE**，仅覆盖用户指定的接口演示软件范围。

- codeQualityStatus: WATCH
- recommendation: APPROVE
- blockers: []
- snapshot: `2b66812fd0d6e64609c1bc3fccbccccf71a4f99d`
- web-mini tree: `f4d43a4db2ca7d7e7fd94e5cafad2fe1d19d06eb`
- reviewer: `/root/m4_code_last`，未参与业务源码编辑
- date: 2026-10-08 Asia/Shanghai
- requested reportPath: `docs/evidence/M4/2026-10-08/reviews/code-last.md`
- fallback reportPath: `.omo/evidence/m4-code-review.md`

## 范围与审阅输入

本次按用户收窄后的接口演示范围审查：六页导航、登录及资源接口调用、Zod 响应解析、真实本地 HTTP fixture、报警加载与确认、订阅三种模拟结果、39 项测试。真实微信登录、订阅弹窗、资质、HTTPS 部署和真机继续为 `external_blocked`；本批准不登记 R25–R27 外部验收通过，也不代表原计划的完整生产小程序已交付。

先读取 `docs/README.md`、`docs/CONTRIBUTING.md`，核对 PLAN、PLAN-DETAILS、ACCEPTANCE 的 M4/R25–R27 条目、FRONTEND-HANDOVER 与 IMPLEMENTED；响应格式另对照 `docs/api/openapi.yaml` 及实际 appapi 序列化代码。IMPLEMENTED 尚未登记本次候选通过，符合双审前状态。

差异范围为 M4 前的 `d9844c3` 至冻结提交，包含三次候选/修复提交。完整差异保存在 `docs/evidence/M4/2026-10-08/reviews/code-last-snapshot.diff`。审查的源码与配置包括 `web-mini/App.vue`、`src/ApiDemo.vue`、`src/api.ts`、`src/response-schemas.ts`、`src/types.ts`、`src/domain/alarms.ts`、`src/pages/AlarmsPage.vue`、`src/pages/RealtimePage.vue`、三份测试、`vite.config.ts`、依赖清单及阶段证据。生成的 dist 通过重建输出和源码一致性核对；没有把压缩产物当作源码审阅替代。

Notepad：交接未提供且在 `.omo` 中未找到本次 M4 notepad；本报告记录审阅笔记。`omo-agent-toolkit ulw-loop status --json` 返回 `ULW_LOOP_PLAN_MISSING`，所以使用 `.omo/evidence/m4-code-review.md` 后备路径，并在指定的 code-last.md 保存同一报告；原始状态结果见 `code-last-ulw-status.json`。

## 独立验证与已查阅证据

| 检查 | 结果与证据 |
|---|---|
| 本审阅者重跑 `npm test --prefix web-mini` | PASS，3 files / 39 tests，实际输出 `reviews/code-last-tests.log` |
| 本审阅者重跑 `npm run typecheck --prefix web-mini` | PASS，实际输出 `reviews/code-last-typecheck.log` |
| 原生 Node fetch 的真实 HTTP 检查 | PASS：登录 JSON Content-Type、登录不附资源 Bearer、受保护请求转发 Bearer、POST 确认及空 204；输出 `reviews/code-last-native-boundary.log` |
| 真实后端响应与当前 Zod schema 对接 | 从 `logs/live-backend.log` 提取 31 个成功响应，逐个调用本快照 schema.safeParse，全部通过；包括 ponds、alarms、v1 latest/history、v2 model/latest；输出同上 |
| 冻结快照一致性 | `git diff --exit-code 2b66812 -- web-mini` 返回 0，所有当前小程序源码和 dist 与冻结提交一致 |
| 本轮构建、audit、外部软件替代、manifest、契约 | 直接读取 `logs/frontend-contracts.log`：各命令 EXIT 0；构建输出与提交 dist 文件名一致；0 vulnerabilities；67 operations / 334 synthetic fixtures；外部状态仍 external_blocked |
| PostgreSQL/Timescale 和资源权限回归 | 直接读取并检查 `logs/live-backend.log` 的实际输出：Timescale 2.30.0；归属/撤权/历史/确认等测试 PASS，0 SKIP / 0 FAIL。此项由父会话执行，本审阅者没有独立重跑数据库测试；不将后端用例等同于浏览器与真实数据库的整链路 E2E |
| 浏览器操作证据 | 读取 `browser/actions.json`：六页导航、空 code、订阅模拟同意/拒绝/取消；报警页实际显示“请求失败”，没有并列“当前没有报警”。截图已保存为 `browser/login.jpg`、`browser/alarms.jpg`；本代码审阅不认证真机或窄屏视觉验收 |
| 模块大小 | 改动范围的源码和测试均低于 250 纯 LOC，最高 102；没有超大模块问题 |

上表证据路径除特别说明外，均相对于 `docs/evidence/M4/2026-10-08/`。本审阅运行环境为 Linux、Node v24.21.0、npm 11.19.0。初始交接只有 PASS 表和会话截图说明；本轮已经补齐可读取的原始文件，本报告以这些文件和独立运行结果为依据。

## Findings

### CRITICAL

无。

### HIGH

无。没有发现当前演示范围内的认证绕过、跨资源成功、错误后伪造确认成功、响应类型不兼容或构建阻断。

### MEDIUM

**M1 — HTTP fixture 的自行实现 fetch 与平台语义不一致。**

位置：`web-mini/tests/api.test.ts:30`、`:33`、`:36`。

`nodeFetch` 忽略 init.signal，并且对 204 使用 `new Response(Buffer.concat(chunks), { status: 204 })`。原生 WHATWG Response 禁止 204 携带非 null body，即使 Buffer 长度为零；happy-dom 的实现接受了它。本审阅已经用原生 Response 重现这项差异，见 `code-last-native-boundary.log`。因此这份 fixture 能证明本地 socket、路径及部分 JSON 行为，不能完整证明原生 fetch 的空响应或取消行为。该适配器还通过 `response.headers as Record<string, string>` 绕过 Node headers 的真实类型。

建议将这份纯 API 测试置于 Node 环境，直接使用平台 fetch 和原生 Response，移除自行实现的 fetch 与类型断言。此项按测试复杂度与可信度问题记 MEDIUM；独立的原生 fetch 检查已确认当前生产代码正确处理空 204 和认证请求头，故不升级为当前候选的 HIGH 阻断。

**M2 — 旧的页面状态配置断言不能证明实际页面状态覆盖。**

位置：`web-mini/tests/domain.test.ts:15`、`:16`、`:17`；相关配置 `web-mini/src/page-contract.ts:5`。

该测试检查静态配置中的 loading/empty/offline 文案和 actions.length 为真；App.vue 没有使用这些文案驱动实际渲染。删掉某页的实际加载或离线处理仍不会使这部分断言失败。它验证的是声明的存在，不能作为“每页状态已覆盖”的行为证据。这段测试来自本轮前的基线，不能把其问题归为本次新增实现缺陷。

建议后续删去这部分无行为约束的真值断言，依靠实际组件状态与可观察交互验证。当前报警页的加载、空态、错误恢复及确认失败已有真正的组件测试；接口演示批准不依赖上述旧断言，故本项不阻断。

### LOW

**L1 — Page 分支缺少编译期穷尽检查。**

位置：`web-mini/src/ApiDemo.vue:33`、`:40`、`:42`。

最后的 else 默认调用 history。当前 Exclude<Page, 'alarms'> 的五种输入全部正确覆盖；但扩展 Page 时，新增页会默默进入历史接口分支。按 programming 视角，使用穷尽 switch 可让新增变体成为类型检查错误。当前范围不新增页面，所以此项为 LOW。

## 必须运行的技能视角检查

已显式加载并应用：

- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.19/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.19/skills/programming/SKILL.md`
- programming 的 TypeScript README、data-modeling、error-handling 与 logging 参考。

`remove-ai-slops`：对测试和生产代码完成过拟合、删除梯、重复防御及抽象检查。未发现 deletion-only 测试、仅证明请求删除已发生的测试、自然语言 prompt 断言或从被测结果计算期望值的自证测试。静态路由 ID 检查对应真实消费的六页注册表，不按 prose/slop 误判；M2 的配置真值断言违反行为测试视角。M1 的手写 fetch 增加了可以由平台直接承担的测试复杂度。

`programming`：响应以 unknown 进入 HTTP 边界，在 ApiClient 解析为类型，原始 API 错误文本不显示给用户，JSON 空 204 与结构错误分别处理；这些解析是本目标需要的边界行为，不是无需求的数据抽取、归一化或业务内部重复校验。生产代码本轮移除了 RealtimePage 的响应类型断言，没有新增 any、ts-ignore 或非空断言。M1 的测试类型断言和 L1 的非穷尽 Page 分支不符合该技能的严格视角；现有 `domain.test.ts:90` 的 `as never` 也仍是旧测试的类型逃逸，未计作新引入问题。其余新增测试能因路径、请求头、schema、轮询或确认状态的真实回归而失败。

## 最终结论

**APPROVE / WATCH，blockers=[]。** 本冻结快照可以作为接口演示软件候选批准；两个 MEDIUM 测试质量项与一个 LOW 穷尽性项保留为后续改进。没有 CRITICAL 或 HIGH。真实微信/订阅/真机继续 external_blocked，浏览器与本地 HTTP fixture 不构成这些外部验收证据。未修改业务源码、用户的 CI 改动或 `.tmp/`。
