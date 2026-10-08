# 首启配置验证

日期：2026-10-08 Asia/Shanghai。

范围：空库自动建表、应用启动、首启初始化 API、重复初始化拒绝；不包含浏览器流程验收，浏览器由部署方自行验证。

环境：本地 Docker 隔离 Compose，PostgreSQL 16 / TimescaleDB 2.30.0，应用镜像由当前工作树现场构建；HTTP `127.0.0.1:18092`。

结果：

- `make verify`：PASS；Go build/vet/race/shuffle，包括真实隔离 Timescale 测试。
- `npm test --prefix web`：PASS，修订后 6 files / 52 tests。
- `npm run typecheck --prefix web`：PASS。
- `npm run build --prefix web`：PASS。
- `make verify-contracts`：PASS，69 operations / 343 fixtures；包括 `docs/api/setup-openapi.yaml`。
- `python3 scripts/check_architecture_manifests.py --all`：PASS，55 requirements。
- Docker 首启：数据库健康，应用健康；启动自动执行 12 个迁移，无 CLI migrate 步骤。
- `GET /setup/v1/status` 初始返回 `{"required":true}`。
- `POST /setup/v1/initialize` 使用隔离安装密钥和测试凭据返回 HTTP 204；凭据未写入证据。
- 初始化后 status 返回 `{"required":false}`，`GET /readyz` 返回 `ready`。
- 重复初始化返回 HTTP 409；并发、非法输入、审计失败回滚、数据库不可用 fail closed 由 `internal/setupapi/server_test.go` 覆盖。

原始日志：`backend.log`、`full-verify.log`、`contracts.log`、`architecture.log`、`docker-build-final.log`、`docker-start-final.log`、`status-before.json`、`init-status.txt`、`status-after.json`、`readyz.txt`。日志不包含密钥、密码或 DSN 值。

## 审阅修订与针对性重验

初始候选根 `7d238d9` / web `2647569` 被独立审阅指出：Vite 缺少初始化 API 代理、网络结果未知时未先查状态、Origin 契约未明确 TLS 终止策略，以及原超大 body 测试输入本身不是合法 JSON。

修订后，Vite 转发 `/setup/v1`；每次初始化写入前检查状态，网络或服务器错误后再次查状态，确认已完成则进入登录，状态不可用时下一次提交不会直接写入。Origin 校验明确为同 Host（含端口）的 http/https，兼容 HTTPS 反代到 HTTP，仍须安装密钥、不使用 cookie 认证且不开放 CORS；userinfo/path/query/fragment 拒绝。超大 body 改为合法 JSON 加前导空白，能区分 body 限制是否生效。

2026-10-08 修订后执行：

- `IOLINK_TEST_PG_DSN=<隔离 Timescale> GIN_MODE=release go test -race -shuffle=on -count=1 -v ./internal/setupapi ./internal/platform ./cmd/iolinkd`：PASS，三包退出 0，未跳过 DB；[原始输出](review-fixes-backend.log)。使用专属 `iolink-todo9-pg`，测试自行创建与删除随机数据库。
- web 目录 `npm test && npm run typecheck && npm run build`：PASS，52 tests，类型检查和构建退出 0；[输出摘要](review-fixes-web.txt)。Zod 注释及既有大 chunk 提示保留，不表示零警告。
- 实际启动 Vite 与临时 HTTP backend（仅 `/setup/v1/status` fixture），经开发服务器 GET 返回 HTTP 200 `application/json` 和 `required=true`：PASS；用于确认代理不再返回 SPA HTML，不属于浏览器验收。
- `make verify-contracts`：PASS，69 operations / 343 fixtures；`python3 scripts/check_architecture_manifests.py --all`：PASS，55 requirements。
- 原始 Docker 日志保留尾部空白，提交空白检查为 NOTE；不宣称归档原始日志通过 `git show --check`。
