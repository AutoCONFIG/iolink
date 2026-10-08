# 首启、平台管理和用户看板：独立代码审阅

日期：2026-10-09（Asia/Shanghai）。审阅者：`/root/setup_code_review`，未参与实现编辑；仅创建本审阅报告。

## 结论

- `codeQualityStatus`: **WATCH**
- `recommendation`: **APPROVE**
- `blockers`: **[]**
- 明确批准当前实现源码；无 CRITICAL/HIGH 阻断。下列 MEDIUM 是非阻断契约差异，本结论不宣称全部目标 HTTP 契约或外部验收通过。

## 审阅快照与范围

- 根实现提交：`2c8a6a91e7ab03f8c575011ecac0259ddcb60560`。
- 根比较基准：`febbfc5722b8aeba4bc76ddfec6a0cdf0f6b06fe`。
- Web 实现提交：`1327ff1cbb09b2b78aee1c9173b44081368d9d9c`，Web 工作树干净。
- Web 比较基准：`b340caedd038d1c80504aeffec8de7927b2871ba`。
- 最终改动源码 manifest SHA-256：`23082062a2b01fd16681db76eda25f9a7b218683788da5b89b0e4d034dcb5dd0`。计算方法：根相对上述基准的改动路径和新增源码，加 Web 相对其基准的改动路径；仅保留 `cmd/`、`internal/`、`scripts/`、`web/src/`、`web/tests/`；路径排序，对每文件生成 `sha256 + 两个空格 + 路径 + LF`，再对全部行计算 SHA-256。
- 关键最终文件摘要：
  - `internal/core/tenant_members.go`: `fc40e9827d6aa7c25542b3aabddaa1d9065520041b10e3d85b00022eb4bd9df0`
  - `internal/core/platform_console_test.go`: `18427dacc9178df58708cafaa5f6d6f592da8c2ab4bb17fcb2fefdc59944a8eb`
  - `web/src/views/TenantsView.vue`: `f1a25a9610c126645dc0fd040d4a5edffd93a584085d672f2caa3d3b9867777d`
- `docs/IMPLEMENTED.md` 和本次证据归档仍有未提交元数据；本批准绑定上述源码，不以元数据状态代替功能证据。
- 输入 notepad：未提供；`.omo/notepads/` 不存在。已检查项目权威地图和贡献规范。`omo-agent-toolkit ulw-loop status --json` 返回 `ULW_LOOP_PLAN_MISSING`，故使用规范要求的 fallback 报告位置。

成功条件：首启只创建平台 ADMIN；USER 自行注册；平台管理员手工创建组织并授予普通角色；平台汇总和租户业务看板分开；认证、资源归属、撤权和事务回滚正确。

核对范围包括 setup、注册、会话/JWT、平台用户目录/汇总、租户成员与状态管理、业务路由守卫、相关测试、OpenAPI、部署/需求修订、离线安装脚本，以及 Web API、auth store、router、导航、双看板、注册、待授权和组织成员页面。未把根 gitlink diff 当成 Web 源码 diff；直接审查上述 Web 基准到实现提交的全部相关源码。

## 当前发现（按严重度）

### CRITICAL

无。

### HIGH

无。

### MEDIUM

**M1 — 新组织创建接口尚未执行声明的严格 JSON 字段契约。**

- 位置：`internal/adminapi/tenant_admin.go:76`；契约：`docs/api/admin-openapi.yaml` 的 `POST /tenants` 请求对象声明 `additionalProperties: false`。
- Handler 使用 Gin 默认 `ShouldBindJSON`。仓库未启用 Gin 全局 unknown-field 拒绝；其默认解码只读取首个 JSON 值。因此已认证平台管理员发送 `{"name":"example","unexpected":true}`，或在合法对象后附加第二个 JSON 值，会走创建路径，而不是因请求形状非法返回 400。此结论来自实际解码器调用和仓库配置检查；未把未执行的恶意请求写成 passed 测试。
- 影响：新接口与声明的请求校验契约不一致；现有合成契约 fixture 不能发现此 handler 差异。DTO 只读取 `name`，权限校验仍然生效，故未发现授权绕过或跨租户写入，不升为 HIGH。
- 建议：沿用注册接口的严格单对象 JSON 解码，并为实际 handler 增加一个额外字段/尾随对象拒绝用例。

### LOW

无新增可执行发现。

## 已解决并复核的问题

1. 新组织无成员时曾返回 `null`，与集合契约和 Web Zod array 边界不符。最终 `internal/core/tenant_members.go:26` 初始化空 slice；`internal/core/platform_console_test.go:123` 在赋权前通过真实 HTTP 断言 `[]`。已看到 `empty-members-before.log` 的真实失败，并独立重跑最终用例通过。
2. 平台管理员成员编辑曾暴露 `support` 选项，但服务明确拒绝平台授予支持身份。最终 `web/src/views/TenantsView.vue:117` 只给平台提供四个普通角色，租户管理员仍保留支持角色。
3. CLI 成功测试曾只检查被移除的自动业务创建。最终 `internal/core/platform_console_test.go:188` 增加指定用户名的 ADMIN 数量和真实密码摘要校验；该测试不会在 SetupInit 不创建管理员却返回 nil 时通过。独立复跑最终版本通过。

## 技能视角与测试有效性

**已运行技能视角检查。** 已读取 `remove-ai-slops/SKILL.md`、`programming/SKILL.md` 和 Go/TypeScript 指南，逐项审查新增生产路径和测试，而非仅采用执行者结论。

- 最终测试未发现仅断言源码删除、脆弱 prompt 文案、实现常量镜像或自证式测试。CLI 原先的纯负向成功测试已补正向身份/密码断言；HTTP 空数组测试锁定机器消费契约；事务失败测试通过真实数据库 trigger 注入审计失败并验证持久化结果。
- Web SSR 测试检查实际渲染的两种看板结构，有相关性；它不执行 onMounted 数据请求，也不证明浏览器交互。此覆盖边界没有被当成视觉验收或完整前端链路通过。
- 新 API 的 Zod 解码发生在 HTTP 响应边界，注册 JSON 解码发生在请求边界；没有为任务不需要的数据引入提取、解析或归一化。新增 ports 按注册、平台目录、平台汇总等能力划分，无万能接口或 provider SDK 进入领域层。
- 新逻辑保留了真实存储和权限校验；未发现新增无类型逃生口、无需求的抽象或无效测试。M1 是 `programming` 边界校验视角的非阻断缺口；未发现新增 `remove-ai-slops` 过拟合/冗余复杂度违规。
- 250 pure LOC 检查仍发现既有 `internal/adminapi/handlers.go`（963）、`server_test.go`（681）和 `internal/core/m2_integration_test.go`（353）的基线规模债务。前两个模块由本次拆分缩小或基本不变，第三个仅增加必要 fixture。未发现新模块超限；没有证据表明这些既有规模问题导致本目标的维护或运行失败，不作为本次阻断。

## 验证与证据

环境：Linux amd64；Go `go1.26.8`；Node `v24.21.0`；专用 Docker 容器 `iolink-todo9-pg`，实际 PostgreSQL `16.15` / TimescaleDB `2.30.0`。`IOLINK_TEST_PG_DSN` 在运行测试的命令中设置，凭据未写入报告。`testdb.New` 为每个相关集成测试创建/清理隔离数据库。

### 独立执行

1. 设置上述专用测试 DSN 和 `GIN_MODE=release` 后：

   ```text
   go test -race -shuffle=on -count=1 -v ./internal/core ./internal/adminapi ./internal/platform ./cmd/iolinkd -run 'TestPlatformConsole|TestCLISetup|TestM6bStoredPlatform|TestM6bEnsurePlatform|TestM6bTenantMembership|TestPlatformAdmin|TestTenantAdmin|TestSetupCLI|TestSetupInput'
   ok internal/core       6.665s
   ok internal/adminapi   2.351s
   ok internal/platform   1.021s
   ok cmd/iolinkd         3.534s
   ```

   实际运行了新入组/CLI setup、既存 ADMIN 普通角色拒绝、租户成员版本撤销、平台/租户授权边界、审计失败回滚测试，输出无 SKIP。

2. 在最终 CLI 正向断言和最终 Web 源码确定后再次运行同一专用 DSN：

   ```text
   go test -race -shuffle=on -count=1 -v ./internal/core -run 'TestPlatformConsoleOnboardingAndIsolation|TestCLISetupCreatesOnlyPlatformAdmin'
   -test.shuffle 1791475195591189665
   --- PASS: TestPlatformConsoleOnboardingAndIsolation (1.82s)
   --- PASS: TestCLISetupCreatesOnlyPlatformAdmin (0.67s)
   PASS
   ok git.hyhy.fun/rsplab/iolink/internal/core 3.571s
   ```

3. `npm test --prefix web`：8 个测试文件、58 项通过；`npm run typecheck --prefix web`：`vue-tsc -b` exit 0。
4. `.venv/contracts/bin/python scripts/check_contracts.py`：5 份 OpenAPI，74 操作 / 366 合成 request/response fixtures 通过。命令明确说明实时 handler 验证属于 R02.c，本报告保持此限制。
5. 根和 Web `git diff --check`：exit 0。

### 已检查的执行者证据

路径前缀：`docs/evidence/deployment/2026-10-08-platform-console/`。

- `focused.log`：真实集成用例 PASS，包含撤权与 setup 审计失败路径。
- `go-verify-final.log`：build/vet/race/shuffle 全仓命令通过；该日志未记录 DSN，不能单独认定其中数据库测试运行，故本审阅补充上述独立 DSN 验证。
- `empty-members-before.log`：真实 HTTP 发现空成员为 null 而 FAIL；`empty-members-after.log`：core/adminapi/platform 全包复跑 PASS。
- `final-onboarding.log`：最终入组和 CLI 正向/负向测试 PASS，无 SKIP。
- `contracts-final.log`：合成契约检查通过。
- `web-tests-final.log`：最终 Web 提交内容 58 项测试通过；`web-build-final.log`：包含 vue-tsc 的最终 Vite build 通过，保留既有 chunk size 提示。
- `docker-smoke.log` 只有 smoke 总结，没有逐步命令/响应记录；本审阅未独立重做该 Docker smoke，不以这两行总结证明浏览器双看板或外部链路通过。
- 早期 `go-verify.log` 的 app token 失败已有后续 focused/final 证据，未把已修复的历史失败列为当前问题。

## 正确性核对

- Web 初始化和 CLI setup 均只创建 ADMIN；注册只创建 USER，不授予成员身份；新组织没有隐式 owner。
- 平台 JWT 校验用户 authority/token version，并通过 platform actor 进行存储内授权。未分配组织的 USER 可以查看账号会话，业务路由 fail closed。
- 平台组织管理只授予 USER 的普通角色；ADMIN 普通租户成员和平台自行授予 support 均拒绝。租户管理员目标组织必须匹配当前 JWT 组织。
- 成员版本/角色、停用组织和密码 token version 在受保护操作重新校验；相关写入采用事务并审计，审计失败回滚用户、组织或角色变更。
- 平台看板调用汇总 API；租户看板通过 tenantRequired 和存储资源过滤，不共享全平台业务明细。未发现新增敏感日志、SQL 动态拼接注入或未经授权的外部副作用。

浏览器视觉、真实微信/硬件/生产 License/公网 TLS 仍未在本代码审阅中验收。批准不代替第二位独立审阅者、真实外部验收或发布授权。
