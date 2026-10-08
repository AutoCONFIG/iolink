# 首启平台与用户看板边界证据

日期：2026-10-08。当前工作树实现首启只创建平台管理员；业务用户自行注册，平台管理员手工创建组织并分配普通组织角色。平台管理员只读取平台汇总，业务用户看板按当前组织和资源权限读取。

## 验证命令

- `make verify`：Go build、vet、race、shuffle 全仓通过，日志见 `go-verify-final.log`。
- `make verify-contracts`：5 份 OpenAPI 共 74 个目标操作、366 个合成 fixture 通过，见 `contracts-final.log`。
- `npm test --prefix web`：8 个测试文件、58 个测试通过，见 `web-tests-final.log`。
- `npm run build --prefix web`：Vite 构建通过；保留已有 chunk 大小提示，见 `web-build-final.log`。
- `docker build -t local/iolink-platform-console:smoke .`：应用镜像构建通过，见 `docker-build.log`。
- Docker smoke：专用 Timescale 容器 `iolink-todo9-pg`、临时数据库、应用容器；覆盖自动迁移、初始化、注册、组织创建、用户授权、平台/用户 session、未授权拒绝、嵌入前端和重启持久化，见 `docker-smoke.log`。

## 审阅修订与最终回归

初次审阅发现新组织尚无成员时接口返回 `null`，违反空集合 `[]` 契约。先在真实 HTTP 入组测试中添加创建组织后、首次赋权前的断言，`empty-members-before.log` 记录失败；随后将成员查询的输出初始化为空数组。

- `IOLINK_TEST_PG_DSN=<隔离 Timescale> go test -race -shuffle=on ./internal/core ./internal/adminapi ./internal/platform -count=1`：三包 PASS，无数据库跳过；见 `empty-members-after.log`。
- `GIN_MODE=release IOLINK_TEST_PG_DSN=<隔离 Timescale> go test -race -shuffle=on ./internal/core -run 'TestCLISetupCreatesOnlyPlatformAdmin|TestPlatformConsoleOnboardingAndIsolation' -count=1 -v`：PASS；见 `final-onboarding.log`。CLI 正向断言同时检查指定 ADMIN 账号及密码校验成功，没有业务租户、USER 或成员关系。
- 最终前端 `npm test --prefix web`、`npm run build --prefix web`：58 测试、vue-tsc 和 Vite 构建 PASS；平台成员编辑只显示可授权的普通角色，不显示无法保存的 support 选项。

数据库来自专用容器 `iolink-todo9-pg`，映射本地 `55439`；测试通过 `internal/testdb` 创建和删除随机数据库。DSN 只传入测试进程环境，不写入日志。Docker 构建和 smoke 是修订前候选的现场运行证据，后续空成员修复由上述真实 SQL/HTTP 回归覆盖；原 smoke 日志仅保留汇总，不将其当成逐步 HTTP/SQL 捕获或浏览器验收。

## 范围边界

该证据不代表真实微信、硬件、生产签发方 License、公网 TLS 或浏览器视觉验收通过。浏览器由用户自行验收。License 当前仍为实例级能力，未将其描述成租户证书能力。

## 同源码双审与发布

根实现提交 `2c8a6a91e7ab03f8c575011ecac0259ddcb60560`，web 提交 `1327ff1cbb09b2b78aee1c9173b44081368d9d9c`。两位未参与实现编辑的审阅者独立明确 APPROVE，均无阻断：[代码审阅](code-review.md)、[最终门审](gate-review.md)。[初次拒绝](gate-review-initial.md)保留追溯，不作为完成依据。后续归档只改证据和状态，不修改批准源码、测试、API 契约或 web gitlink。

代码审阅保留一项 MEDIUM 非阻断限制：新组织创建 handler 尚未严格拒绝额外字段和尾随 JSON；权限仍有效，未发现授权绕过，已登记 `docs/IMPLEMENTED.md` D18。合成契约通过不等于全部实时 handler 严格校验通过。

web `main` 已推送；根发布标签拟为 `v0.0.13`，远端 CI 结果另行取证。GitHub workflow 仅监听 tag push，推送 main 不触发。
