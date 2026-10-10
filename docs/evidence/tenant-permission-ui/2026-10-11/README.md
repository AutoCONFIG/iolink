# 租户业务操作权限前端修复

## 范围与候选

- 需求：R23/R24 的现有页面交互、R37.a 的角色读写一致性；截图中业务用户点击新增养殖场后收到 `tenant action forbidden`。
- 根基线：`0d2168d6b980ba0a3f67f418e2d4afa77d8ec7aa`，前端基线：`7bd9523abff5494806d0971fecb6ecae6ec67cce`。
- 冻结前端源码：`f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350`。本轮没有修改后端权限、数据库或 API 契约。
- 候选状态：验证通过，两位未参与编辑的独立审阅者均明确 APPROVE 同一冻结前端 SHA；发布目标为 v0.0.18。

## 根因与修复

`internal/authorization/policy.go` 仅允许 owner/admin 创建养殖场、池塘、设备和规则，member/viewer/support 仅可读取这些资源。前端原先对所有角色展示创建与设备配置入口，导致可见操作与后端授权不一致。截图及错误不足以区分现场账号的具体只读角色，不记录未经证实的现场角色。

共享 `tenantPermissions.ts` 对未知/空角色默认拒绝写入。养殖场、池塘、设备、报警规则隐藏管理入口并显示只读提示，相关写方法再次检查角色；产品绑定禁用输入并防止回车发送无权限请求。业务读取保留。报警确认按后端矩阵保留 owner/admin/member/support 能力，viewer 隐藏单条、批量与选择列。后端仍是最终授权边界。

## 环境与命令

Linux amd64，Node v24.21.0，npm 11.19.0，Go 1.26.8。权限回归使用独立 Vite 端口 5188、VITE_DEMO_MODE=false、真实 Chrome；仅 HTTP 响应采用合成角色与资源 fixture。已有页面回归使用 VITE_DEMO_MODE=true 和端口 5173。没有使用现场密码、JWT、用户数据或生产数据库。

| 场景 | 命令 | 实际结果 | 证据 |
|---|---|---|---|
| 旧版复现 | 在前端基线临时 worktree 复制新测试和配置，`npx playwright test --config playwright.permissions.config.ts --grep 'viewer reads'` | 如预期失败：新增养殖场按钮期望 0，实际 1 | [baseline-regression.log](baseline-regression.log) |
| 权限单测 | `cd web && npm test -- --run` | 10 文件 / 67 测试通过；owner/admin 可管理，member/viewer/support/空角色拒绝；报警确认角色矩阵通过 | [unit.log](unit.log) |
| 类型与生产构建 | `cd web && VITE_APP_VERSION=v0.0.18 npm run build` | vue-tsc 与 Vite 成功；已有主包大小和第三方 PURE 注释警告保留 | [build.log](build.log) |
| 角色与错误路径 | `cd web && npx playwright test --config playwright.permissions.config.ts` | 10 场景通过；3 个只读角色无管理请求、可读详情/遥测；owner/admin 保留入口与服务器拒绝反馈；5 角色报警确认矩阵 | [permissions.log](permissions.log)，[截图目录](browser/) |
| 原有页面回归 | `cd web && VITE_DEMO_MODE=true npx playwright test e2e/admin-pages.spec.ts` | 5 场景通过；桌面/窄屏、池塘表单、设备注册、报警确认、改密、产品发布绑定、组织角色、过期登录 | [admin-pages.log](admin-pages.log) |
| 后端现有单测 | `go test ./internal/adminapi ./internal/authorization` | 两包通过（缓存命中）；未宣称真实数据库集成通过 | [backend.log](backend.log) |
| 补丁检查 | `git -C web diff --check` | 通过 | 提交前工具记录 |

用户此前明确浏览器视觉验收自行执行，本轮自动浏览器证明权限交互与既有流程，不代表现场部署或全量视觉验收。纯前端修复无需新的数据库变更验收；未运行真实 PostgreSQL/Timescale 集成，不将 skipped 计为通过。现场部署由自动更新机制负责，本轮不更改服务器配置。

## 自查

权限辅助模块只负责前端角色能力，不替代服务端资源归属检查；复用既有 auth session、Element Plus 和设计令牌，无新增依赖、API 或持久化状态。没有新增 any、类型忽略、非空断言或日志字段。改动文件大多低于 200 非空行；既有 ProductsView 为 217 行警告区，仅新增权限保护，本轮不扩展产品布局；进一步功能应拆分。旧版回归失败与新版本通过证明测试能发现原缺陷。

## 双审与发布

两位独立审阅者针对上述同一前端 SHA 和根后端基线审阅，均明确 APPROVE：

| 审阅者 | 结论 | 冻结前端 SHA | 收据 |
|---|---|---|---|
| permission_code_review（未参与编辑） | APPROVE / CLEAR / 无阻断 | `f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350` | [代码审阅](code-review.md) |
| permission_gate_review（未参与编辑） | APPROVE / 无阻断 | `f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350` | [门审](gate-review.md) |

代码审阅独立执行 67 单测、vue-tsc 和 diff 检查；门审独立执行 67 单测、vue-tsc、10 个关闭 demo 的 Chrome 权限场景、生产构建，以及 `go test -count=1 ./internal/authorization ./internal/adminapi`（非缓存通过）。两位均复核源码 SHA 未变，且检查权限辅助函数和所有写调用路径。

发布以根仓库 tag v0.0.18 引用该前端 SHA；根仓库本轮变化仅为子模块指针、此证据包和 IMPLEMENTED 补充状态。GitHub CI 仍只由新 tag 触发，main 不触发。当前证据不宣称服务器自动更新完成；现场角色、浏览器视觉和部署结果仍由用户验收。

主 TODO 13/20 软件项已核验、7 项尚待完成的总量不因本轮维护修复改变；当前后续功能位置仍为 TODO 11a/M7a 视频阶段。
