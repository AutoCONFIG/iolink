# M6c 固定源码与软件验证

日期：2026-10-05（Asia/Shanghai）。当前为候选，尚未取得最终快照双审。

## 快照与环境

- 运行源码：M6c runtime `333fc427a4cd9c49715193176993c1e1827dedc6`；当前发布快照 `ec89218`（完整 SHA 在 Git 记录中）。
- 测试配置候选：`67199df37f2979bfa8130d9917271ffc5c624340`，随后 `ec89218` 只修复 License-first 锁顺序与 restore 安全错误映射。
- web：`0aa7771acf0ef6322c9ac1e4c9839616646bb675`；运行构建使用 `0f3869c0c813bcb906b05eb52c9983e4b148db4f`，二者差异仅为默认 Playwright 配置排除专用生产模式测试，产品源码相同。
- 本索引提交另加入 M6c 架构门禁支持、测试、manifest 修正和证据；不改变运行代码。
- Linux amd64，Go 1.26.8、Node 24.21.0、npm 11.19.0、Docker 29.8.2、Compose 5.6.0。
- 隔离测试数据库 `iolink-m6c-pg-20261004`：TimescaleDB 2.30.2 / PostgreSQL 16.15，本机端口 33778。真实集成命令设置 `IOLINK_TEST_PG_DSN`；凭据不写入证据。
- 应用镜像 `local/iolinkd:m6c-333fc42`，完整 ID 与 OCI revision 见 [image identity](m6c-image-identity.txt)；包版本和两个镜像 ID 见 [manifest](offline-manifest.json)。Compose ps 的 config image 标签不是运行镜像的实际 ID。

## 检查矩阵

| 需求/场景 | 已执行命令或操作 | 结果与保留产物 |
|---|---|---|
| Go 全库、签名解析与密钥边界 | `TMPDIR="$PWD/.tmp" make verify` | PASS，[原始日志](m6c-verify-final.log)；最终快照复跑 PASS，[ec89218 日志](m6c-verify-ec89218.log)。未设置 DSN 的集成跳过不计通过，真实 DB 由下一行覆盖 |
| R38/R39.a/首启真实事务与并发 | 设置隔离 DSN 后 `go test -race -shuffle=on ./cmd/iolinkd ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/platform ./internal/wechat -count=1` | 原始七包 PASS；锁顺序/错误映射修复后的最终快照复跑 PASS，[日志](m6c-integration-ec89218.log) |
| signed-only 011 schema 回归 | 设置隔离 DSN 后运行 License/admission/restore/setup focused tests | PASS，[日志](m6c-focused-new-schema.log)；命令选择见测试源码，完整包级运行以上一行为准 |
| 版本契约 | `make docs-tools`、`make verify-contracts` | PASS，[依赖日志](m6c-docs-tools.log)、[契约日志](m6c-contracts.log)：57 操作、284 合成 fixture，非全部真实接口验收 |
| 前端类型与业务回归 | `npm test --prefix web`、`npm run build --prefix web` | 28 测试 PASS，[测试](m6c-web-tests.log)、[构建](m6c-web-build.log)；现有 bundle size / Zod 注释警告保留 |
| License UI 正向/错误/权限/状态 | `cd web && npx playwright test --config playwright.m6c.config.ts` | 11 场景 PASS，[日志](m6c-browser.log)、[39 张截图](browser/)；375/768/1440 宽度、无溢出。使用生产预览与 wire fixture，不能冒充真实数据库联调 |
| 既有页面回归 | `cd web && npx playwright test` | 16 场景 PASS，[日志](m6c-web-existing-e2e.log)，默认 demo 配置与 M6c 实 API 模式分开 |
| 集成镜像 | `docker build --label org.opencontainers.image.revision=333fc427a4cd9c49715193176993c1e1827dedc6 -t local/iolinkd:m6c-333fc42 .` | PASS，[日志](m6c-docker-final.log)，同时包含后端与编译后的 web |
| 架构脚本 | `python3 scripts/check_architecture_manifests.py --all`、`python3 -m pytest scripts/test_check_architecture_manifests.py -q` | manifest PASS，30 测试 PASS，[最终测试日志](architecture-tests-ec89218.log)；M6c gate 当前预期阻塞，[门禁日志](architecture-gate.log)，等待双审后登记 |

## 实际离线链路

`IOLINKD_IMAGE=local/iolinkd:m6c-333fc42 IOLINK_DB_IMAGE=timescale/timescaledb:latest-pg16 sh scripts/build-offline-bundle.sh .tmp/m6c-offline-333fc42`。
包内执行 `install.sh`，Compose project `iolink_m6c_resume_333fc42`，独立数据卷和 `Internal=true` 网络。
临时测试私钥置于包外，仅挂载公钥；测试身份和 License 不复用生产。

| 场景 | 实际结果 | 产物 |
|---|---|---|
| 新卷、管理员/租户首启但没有 License | 退出 3，停在服务启动前；不是安装成功 | [stop](m6c-offline-stop.log)、[before](m6c-offline-status-before.json) |
| 同一卷供应实例绑定的测试 License，不再提供 setup 输入 | 安装成功，health/readiness 200；相同 deployment_id，无重复初始化 | [resume](m6c-offline-resume.log)、[after](m6c-offline-status-after.json)、[audit](m6c-offline-audit.txt) |
| 内网断外网、运行包不带签发器 | 外网探测失败；mqtt-sim 存在、license-sign 不存在 | [补充验证](offline-followup.log) |
| MQTT 模拟上报 | 5 条 sensor_data 落库 | [补充验证](offline-followup.log)；初次 QA 的 fixture 错用了不存在的列，修正为 report_interval 后完成，不把初次失败脚本记为整体 PASS |
| 真实集成镜像浏览器登录、读取 License | permanent、offline-fixture、使用 1/3；owned Chrome 访问隔离容器 bridge IP，未解除网络隔离 | [操作记录](m6c-live-ui.log)、[截图](m6c-live-ui.png) |
| 当前包修改 VERSION 后执行 installer | 退出 1，校验失败，在 docker load/Compose 之前拒绝；测试后恢复文件 | [篡改日志](offline-tamper.log) |

包校验清单：[SHA256SUMS](offline-SHA256SUMS)。[SPDX](sbom.spdx.json) 枚举 106 个 Go 与 194 个 npm 依赖，含构建依赖。
[dependency notices](dependency-notices.txt) 收集可取得的根 LICENSE/COPYING/NOTICE，缺失来源逐项标明；NOASSERTION 不代表法律认定。
OS/数据库内部软件未做完整许可证扫描，镜像由 manifest 标识；这不是完整法律合规审查。

## 未通过范围

- 实际签发方授权包、干净客户断网 x86_64 主机 30 分钟首启：`external_blocked`。隔离容器安装仅为软件链路证据。
- ARM64 未构建或验收；当前离线包限定 linux/amd64。
- 真微信、实体设备及后续可选功能入口未验收。R39.a 为当前配额和共享生产 guard + fake executor；R39.b 随真实功能阶段验证。
- 最终两位独立审阅者须针对同一快照明确批准后，才能把 M6c 软件阶段登记完成。
