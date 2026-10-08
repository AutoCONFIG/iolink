# 首启配置验证

日期：2026-10-08 Asia/Shanghai。

范围：空库自动建表、应用启动、首启初始化 API、重复初始化拒绝；不包含浏览器流程验收，浏览器由部署方自行验证。

环境：本地 Docker 隔离 Compose，PostgreSQL 16 / TimescaleDB 2.30.0，应用镜像由当前工作树现场构建；HTTP `127.0.0.1:18092`。

结果：

- `make verify`：PASS；Go build/vet/race/shuffle，包括真实隔离 Timescale 测试。
- `npm test --prefix web`：PASS，6 files / 50 tests。
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
