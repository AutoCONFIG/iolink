# M6c 设计冻结：License、首启和离线交付

## 状态和边界

`license_state` 持久化当前导入的原始 payload、签名和摘要；解析后的字段仅为缓存，授权决策每次使用已验签原文。`license_clock` 只保留最大已见 UTC 时间和 `clock_error` 状态。运行时公钥来自 `IOLINK_LICENSE_PUBLIC_KEY_FILE`（PEM），对应 `IOLINK_LICENSE_KEY_ID`。两项必须一起配置，缺失时仍能启动基础监测与管理恢复，但导入/新增/恢复/可选动作拒绝；配置非 RSA 或不足 2048 位的密钥则启动失败。签发私钥不进入仓库、镜像、运行包或数据库。

状态为 `missing`、`invalid`、`instance_mismatch`、`not_before`、`expired`、`clock_error`、`valid` 或 `permanent`。`expired` 与 `clock_error` 保留既有基础采集、查询和报警；设备新增/恢复以及 video、openapi、automation、reports 功能拒绝。无效导入事务回滚，不替换上一份有效授权。

## 数据和事务

DDL 提案为 `internal/migrate/sql/011_license.sql`（设计通过后才作为运行迁移）。`deployment_config` 保存安装实例 ID；首次迁移使用 PostgreSQL 16 内置 `gen_random_uuid()`，备份恢复保留 ID。单行 `license_state` 是实例额度锁，所有增加设备数的操作先锁该行，再锁设备行/计数；导入也锁同一行。按 `disabled_at IS NULL` 跨租户计数；网关和子设备使用同一设备表计数模型。停用释放额度，恢复重新检查额度。无证书或验签失败均拒绝设备准入；不得以尚未配置 guard 绕过。

新增纯业务包 `internal/license` 只依赖标准库。应用编排通过窄小的 `license.Runtime`/`license.DeviceAdmission` 能力消费授权，不引用 pgx；PostgreSQL adapter 使用数据库事务协调 License、配额和业务写入。现有 core 的设备 SQL 属于已有 adapter seam，迁移只在此 seam 接入事务额度检查，后续拆分不扩展此次范围。

设备注册/恢复时，授权检查在当前事务的同一 License 行锁下重验原文签名、实例、时钟、时间和配额；不允许“先检查再另事务注册”。失败回滚设备、影子、审计及 outbox 写入。License 成功导入和 `license.imported` 审计原子提交，记录 payload SHA-256、key_id 和额度；无效尝试单独记录 `license.import_rejected`（安全 reason 枚举 + 原文件 SHA-256），不记原文或签名。授权相关读取审计仅记录摘要，不记签发私钥或上传内容。

时钟观察使用独立、短事务锁定 `license_clock`，持久化 `max(old,now)`；业务失败不丢失已观察高水位。正常进程启动及每次授权检查观察时钟，回拨严格大于 5 分钟时锁存 `clock_error`，重启/换 License 不清除。恢复仅允许受保护的本地 `iolinkd license reconcile-clock`，当前时间必须达到旧高水位，写审计后清除；无需支持把高水位回退。主机 root 能改库/程序，不宣称抵抗 root 篡改。

## HTTP 契约

管理端 `GET /admin/v1/license` 返回状态、实例 ID、摘要、额度和 feature 列表；`POST /admin/v1/license` 只接受 `{payload_b64,signature_b64}`，成功 204，字段/签名/实例/未生效/过期无效 400，clock_error 为 409，缺少验证能力为 503，当前有效证书不因失败上传改变。两操作均只允许实时有效平台管理员，普通租户主体 403；平台 token 版本在事务内重验。支持证书重导入，摘要相同仍为 204、无重复业务副作用。最大上传 64KiB，严格拒绝额外字段、重复 JSON 键、尾随对象、无效 UTF-8、空 features/null、缺省必填字段；RSA-PSS salt 长度为 SHA-256 的 32 字节。契约详见 `docs/api/license-openapi.yaml`。

设备恢复新增 `POST /admin/v1/devices/{device_no}/restore`：按当前租户管理员授权，成功 204，跨租户 404，无权限 403，非法或状态不允许 409，License/配额拒绝 403（稳定码 `license_required` / `device_quota_exceeded`），服务能力缺失 503。恢复成功使设备 offline、last_seen_at 清空、session_version 增加，旧 session/旧 shadow 失效；重复恢复不占第二份额度。停用重复幂等。后续所有可选 HTTP/MQTT/worker 执行入口调用同一 feature gate；M6c 只用 fake future executor 验证每个 feature，真实 future 入口仍为 R39.b 待各阶段实现。

状态是证书状态，`overage` 单独表示 max(used-limit,0)，有效超额显示 `overage` 状态但保留已授权 features。失效仍可展示已验签的到期时间和额度，不能用展示缓存授权。`features` 固定数组，missing 时为 []；时间字段 missing 时 null。所有错误响应形状为 `{error:稳定错误码}`，不返回解析器/密码/上传原文错误。

## 首启和离线包

首启为本地分步 CLI 向导，避免暴露未认证的远程安装入口：①连接测试、显式 migrate up；② `setup status` 显示 deployment_id；③ `setup init PLATFORM_USER TENANT_USER` 从受保护 stdin JSON 接收两个自设强密码及默认租户名，在同事务创建平台管理员、独立 USER 租户 owner、默认租户 membership 及审计（系统租户仍保留用于系统审计）；④ `license import` 从 stdin 导入签发方对该 ID 的 License；⑤ `setup status` 显示每步状态。重复初始化拒绝，失败不留下部分账户或 membership。密码 12..256 UTF-8 字节，至少含字母及非字母且不全相同，不公开默认口令；旧 `admin init/reset-password` 本地恢复入口保留。平台管理员默认无业务读取权限，租户 owner 只在默认租户管理业务。

后台“系统授权”页：平台主体显示实例 ID、状态、签发/生效/到期时间、使用/上限/超额、features、摘要和文件导入；使用平台权限真实 403 呈现无权状态。上传不显示原始证书，成功重拉状态；覆盖加载、missing/empty、错误/重试、登录过期、签名拒绝保留旧显示与额度拒绝。设备页增加停用/恢复动作与额度错误提示，菜单隐藏不代替服务端 gate。

离线包构建要求 caller 显式给出本地已拉取的 application/Timescale/反代镜像；解析 docker image inspect 锁定 ID/digest，`docker save` 后附 image manifest，生成 `pull_policy: never` 的独立 offline Compose。包括 x86_64 核心二进制、已构建前端、迁移、数据库/反代配置、可选流媒体配置（未实现前只可标 optional_unimplemented，不能承诺可运行视频）、源码 revision/version、SBOM/依赖和许可证清单、安装/卸载说明、上报 smoke 工具、SHA-256 清单。与用户线上追踪 latest-pg16 的 Compose 分开。

安装脚本先严格校验文件清单、拒绝缺少/损坏/路径越界，再 docker load，版本和依赖检查；自动生成本地随机数据库密码/JWT key（只写 0600 .env），提示输入自设管理员和 owner 密码且不回显，完成迁移/初始化/License 导入/健康与模拟上报。没有签发方 License 时停在授权步骤，记录外部阻塞。卸载仅停容器，默认保留数据；删数据需要显式参数。

软件验收使用本任务专属隔离 TimescaleDB，完整断网安装采用独立 Docker network + `pull_policy: never` 验证不访问 registry；干净实机 30 分钟、实际签发/续期仍需外部交付证据，不把本地隔离 Docker 验证写成已满足外部实机。ARM64 未验收。

## 权限矩阵

| 操作 | 平台管理员 | 租户管理员 | 普通成员 |
|---|---:|---:|---:|
| 查询/导入 License | 是 | 否 | 否 |
| 新增或恢复设备 | 通过租户授权且受额度守卫 | 是且受额度守卫 | 否 |
| 基础采集/查询/报警 | 依资源授权 | 是 | 按既有 RBAC |
| 可选 feature | 需 License feature 且资源授权 | 需 License feature 且资源授权 | 需 License feature 且资源授权 |

## 验收映射

- R38：签名状态表、伪造/缺失/未生效/过期/错实例、回拨和无效导入不替换。
- R39.a：并发注册不超额，停用释放，恢复重验，降额保留存量。
- R40：离线 x86_64 Docker/Compose 安装校验、首启、迁移、版本和依赖清单。
