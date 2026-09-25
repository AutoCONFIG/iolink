# M0 实施、双审与验收记录

执行日期：2026-09-19。需求基线：docs/ACCEPTANCE.md（2026-09-19修订版，需求语义未改变）；原始提交 b71095bbb6b228caed74adf47c48ca4c6b07c92b。当前实施工作树未提交，文件摘要见 manifest.json。manifest包含当前代码、测试、构建配置和说明，不包含自身及本证据目录，避免自引用；此前文档审查清单保留为历史快照。

| 需求 | 结论 | 已执行与限制 |
|---|---|---|
| R01 | 实现待验收 | 独立工作树副本、空GOCACHE下build/vet/全部Go测试（含真实数据库）通过；依赖下载缓存复用。GitLab CI已采用同一make verify，远程流水线未执行，不声称新机/离线部署通过 |
| R02.a | 验收通过 | 2份OpenAPI正式标准/本地引用/35操作159合成请求响应fixture通过；不覆盖R02.b/c真实协议/HTTP全矩阵 |
| R03 | 验收通过 | 空库逐版、重复、旧库保历史、结构指纹漂移拒绝、checksum/缺版本/未来版本拒绝、失败事务回滚、四并发迁移通过；真实core与MQTT写signal均通过 |
| R04 | 验收通过 | 缺secret/非法配置拒绝；首启无默认管理员、旧默认密码门禁、本地stdin初始化、长输入/重复初始化拒绝、随机盐Argon2id；配置单测及Compose300/4映射通过；实测日志无所用口令/根密钥/设备secret |

## 可重放命令和日志

- [clean-verify.txt](clean-verify.txt)：将git已跟踪及未忽略新增文件复制到临时空目录，设置全新GOCACHE及隔离 `IOLINK_TEST_PG_DSN`，运行 `make verify`；退出0。目录用完已清理，未冒称未提交工作树是git正式检出版本。
- [database-tests.txt](database-tests.txt)：`IOLINK_TEST_PG_DSN=<隔离实例URL> go test ./internal/migrate -count=1 -v`；退出0，所有数据库测试实际执行、无skip。测试仅创建并清理自己命名的数据库；角色须有CREATEDB权限。
- [cli-smoke.txt](cli-smoke.txt)：`go build -o /tmp/iolink-m0 ./cmd/iolinkd`、`go build -o /tmp/iolink-mqtt-once ./cmd/mqtt-once` 后，提供 `IOLINK_TEST_CONTAINER` 和 `IOLINK_TEST_PG_DSN`，运行 `python3 scripts/smoke_m0.py`；退出0，临时测试库自动删除。
- [contracts.txt](contracts.txt)：固定依赖环境运行 `python scripts/check_contracts.py`；退出0。依赖版本见 scripts/requirements-docs.txt。
- [environment.json](environment.json)：本机Go/Python/Docker/Compose/内核及PG16.15、Timescale2.30.0精确版本；含使用非敏感输入渲染生产Compose所得300/4实值。镜像digest固定，数据库为本轮专用tmpfs测试容器，与已有业务容器分离。

CLI场景输入：随机生成管理员口令与根密钥、单设备temperature=26/signal=-65。预期与实际：未迁移/无管理员启动非零退出，迁移和初始化后登录成功；真实MQTT写入1条对应遥测，管理pond latest=26；重启后同值；SIGTERM退出0；敏感测试值未出现在进程日志。该最小链路不验证全部目标HTTP响应形状，尚未修复的M1/M2问题继续保留。

## 独立双审

审阅员为 `/root/plan_review_a`、`/root/plan_review_b`，均未编辑本次业务代码。

1. A首轮通过，建议stdin拒绝超长截断前缀；已修并增加真实CLI反例。
2. B首轮不通过：生产Compose漏传report_interval/offline_grace；已补environment，实际渲染300/4通过。
3. 修复后A明确“通过，无剩余阻断项”；B明确“代码审查通过，无剩余阻断项”。A独立重复平台/迁移单测但未提供DB环境；真实DB和CLI证据由主执行者提供，不能重复算独立实测。

D01实现已补、远程CI待验；D02中M0缺signal/迁移问题关闭；D05只关闭依赖注入部分；D09只关闭安全首启/新密码存储部分。HTTP旧哈希升级、令牌撤销与归属权限仍在M2；TLS、备份恢复、微信真机、硬件、容量以及M6–M8未因此通过。M0代码双审通过不等于全项目交付完成。
