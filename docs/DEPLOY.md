# 部署、迁移与恢复

2026-09-19，M0迁移/安全首启已实现并在隔离环境验收。项目为绿地重建，没有正式生产部署或生产数据需要保留；以下旧schema升级和旧二进制回退只作为隔离非生产fixture/中断安装恢复演练，不承诺已部署客户数据升级兼容。空库安装、迁移checksum/锁/事务安全、失败与中断恢复、备份恢复仍是强制验收。以下明确区分可运行入口和M5/M6c目标；TLS、备份恢复、容量和离线安装尚未验收，不能声称生产就绪。旧docs/schema.sql仅用于历史基线，不是安装入口。

## 配套服务与版本

核心iolinkd（含构建后的管理前端）、PG16+TimescaleDB、TLS反代；M7另有流媒体。测试服务器应用镜像使用 `latest`，数据库使用 `timescale/timescaledb:latest-pg16`，两者在执行 `docker compose up` 时拉取，数据库保持 PG16 主版本。镜像更新仍按下节停应用、迁移、启动流程执行；验收记录实际拉取的 digest 和扩展版本，以便复现。Go/Node/前端依赖由锁文件固定。起始验收机4vCPU/8GiB/SSD、Linux x86_64、Docker Engine+Compose v2，记录精确版本；ARM64另验。

配置包含HTTP/MQTT监听地址、PG_DSN、SECRET_KEY（至少32字节随机）、微信三项和模板字段映射、每设备上报周期/离线倍数、日志级别；生产缺关键配置拒绝启动。凭据用受限权限文件或secret注入，日志/备份清单脱敏。首次安装由本地CLI设置管理员，旧默认密码必须变更后才能开放业务入口。

## M0 已实现的安装入口

先构建 `go build -o /tmp/iolinkd ./cmd/iolinkd`。本地数据库使用下节的开发 Compose 覆盖文件发布 loopback 端口；配置 `deploy/.env` 后可用 `docker compose -p iolink-dev -f deploy/docker-compose.yaml -f deploy/docker-compose.dev.yaml up -d --wait db`。提供指向本地数据库的 `IOLINK_PG_DSN` 和至少32字节随机 `IOLINK_SECRET_KEY` 后运行：

```bash
/tmp/iolinkd migrate status
/tmp/iolinkd migrate up
/tmp/iolinkd admin init operator < /run/secrets/iolink-admin-password
/tmp/iolinkd serve
```

口令文件须事先以受限权限创建，内容12–256字节、不含空白；文件路径为部署者提供的实际受限文件。CLI不从argv或环境读取管理员口令，也拒绝终端直接回显输入。初始化仅允许库内尚无管理员时执行，不安装公开默认口令。`migrate up` 可重复运行；`serve` 仅检查版本，不自动迁移。

`migrate adopt-legacy` 仅用于隔离的非生产旧schema fixture/恢复演练：它只接管与仓库遗留DDL指纹、Timescale时序表和13个月保留策略匹配的八表库；出现未知表或漂移即拒绝，不能绕过检查硬写版本。fixture 需先快照/备份，演练应核对数据保留与失败恢复。该入口不构成已部署客户数据升级支持。旧公开默认密码被强制门禁阻止，需运行 `admin reset-password admin < /run/secrets/iolink-admin-password` 后才能启动。新版本/checksum不兼容也拒绝启动。当前M0仅递增token_version，既有HTTP token的撤销校验还在M2待实现；本地重置需先停服务并轮换根密钥，再启动以使既有令牌失效。

## 测试服务器 Compose

复制 `deploy/docker-compose.yaml` 和 `deploy/.env.example` 到服务器同一目录即可，无需克隆源码。文件内统一管理 iolinkd 与 PostgreSQL 16/TimescaleDB；MQTT 内嵌于 iolinkd，当前没有 Redis 依赖。下面命令都在该目录运行。

```bash
cp .env.example .env
chmod 600 .env
openssl rand -hex 32
openssl rand -hex 32
```

把两次生成的值分别填入 `.env` 的 `IOLINK_PG_PASSWORD` 和 `IOLINK_SECRET_KEY`。PG 口令必须使用 URL 安全字符（上述 hex 符合要求），因为 Compose 将其直接嵌入 PostgreSQL URL。准备受限权限的管理员口令文件 `/run/secrets/iolink-admin-password`，内容12–256字节、不含空白。支持 `IOLINK_REPORT_INTERVAL=60|300`（默认60）和 `IOLINK_OFFLINE_GRACE=1..10`（默认3）；微信功能需要同时配置 `IOLINK_WX_APPID`、`IOLINK_WX_SECRET`、`IOLINK_WX_TEMPLATE_ID`。

首次启动：

```bash
docker compose pull
docker compose up -d --wait db
docker compose run --rm --pull never --no-deps iolinkd migrate up
docker compose run --rm --pull never --no-deps -T iolinkd admin init operator < /run/secrets/iolink-admin-password
docker compose up -d --pull never --wait iolinkd
curl -f http://127.0.0.1:8080/readyz
```

`IOLINKD_IMAGE` 默认 `ghcr.io/autoconfig/iolink:latest`，`pull_policy: always` 在执行 `up` 时检查拉取镜像，不会在后台定时升级。需要更新时先备份数据库，显式停应用并迁移，再启动：

```bash
docker compose pull
docker compose stop iolinkd
docker compose run --rm --pull never --no-deps iolinkd migrate up
docker compose up -d --pull never --wait iolinkd
```

数据库保存在项目的 `pgdata` 命名卷中，`docker compose down` 保留数据；`down -v` 会删除数据。内部 JSON 诊断日志保存在同目录的 `logs/iolinkd.jsonl`，容器内限制为20MB×5个备份，可通过 `IOLINK_LOG_LEVEL=debug|info|warn|error` 调整级别；日志目录和 Docker stdout/stderr 都有大小上限。默认仅监听 `127.0.0.1:8080/1883`，数据库不公开端口。测试机需从其他主机直接访问时，配置 `IOLINK_HTTP_BIND=0.0.0.0` / `IOLINK_MQTT_BIND=0.0.0.0` 并配置防火墙；公网部署按下节反代 TLS。

`latest` 适合测试机追踪最新发布；正式发布和可复现验收应把 `IOLINKD_IMAGE` 固定为 tag 或 digest。该 Compose 不代表 TLS、容量、离线安装或 M6b 完整阶段验收已通过。

## 从本地源码构建

在仓库根目录先执行 `git submodule update --init web`，复制 `deploy/.env.example` 为 `deploy/.env` 并填写随机配置。开发文件覆盖镜像拉取规则，使用当前源码构建 Go 与管理前端，Docker cache 复用依赖和编译缓存，无需手工生成 `web/dist`。

```bash
docker compose -p iolink-dev -f deploy/docker-compose.yaml -f deploy/docker-compose.dev.yaml build iolinkd
docker compose -p iolink-dev -f deploy/docker-compose.yaml -f deploy/docker-compose.dev.yaml up -d --wait db
docker compose -p iolink-dev -f deploy/docker-compose.yaml -f deploy/docker-compose.dev.yaml run --rm --pull never --no-deps iolinkd migrate up
docker compose -p iolink-dev -f deploy/docker-compose.yaml -f deploy/docker-compose.dev.yaml run --rm --pull never --no-deps -T iolinkd admin init operator < /run/secrets/iolink-admin-password
docker compose -p iolink-dev -f deploy/docker-compose.yaml -f deploy/docker-compose.dev.yaml up -d --build --wait iolinkd
```

源码修改后再次执行最后一条命令即可重新构建。开发 DB 额外发布 `127.0.0.1:${IOLINK_DEV_PG_PORT:-5432}` 供宿主 Go 调试；该覆盖文件不要用于公网部署。`make dev` 只启动开发 DB，再用宿主 `go run` 迁移和服务，需要单独导出本地 `IOLINK_PG_DSN`、根密钥并已初始化管理员；它不会自动读取 `deploy/.env` 给 Go 进程。

## M6c 离线包

离线包由操作者明确提供已经存在于本机的应用和 TimescaleDB 镜像构建，脚本会检查 `linux/amd64`、保存镜像 manifest 和 digest、复制迁移/Compose/前端/依赖清单，并生成 SPDX SBOM、许可证清单和 SHA-256 清单。应用镜像是前后端一体的 `iolinkd`，其中也带有 `/app/mqtt-sim` 供隔离验收使用；`web-mini` 不进入常驻镜像。

```bash
docker build -t local/iolinkd:build .
IOLINKD_IMAGE=local/iolinkd:build \
IOLINK_DB_IMAGE=timescale/timescaledb:latest-pg16 \
  sh scripts/build-offline-bundle.sh dist/iolink-offline
```

把整个 `dist/iolink-offline` 目录复制到目标机。安装前准备仅本地可读的 setup JSON，例如：

```json
{"platform_username":"platform","platform_password":"Platform-pass1!","tenant_username":"tenant","tenant_password":"Tenant-pass2@","tenant_name":"Default tenant"}
```

然后执行：

```bash
chmod 600 setup-input.json
IOLINK_SETUP_INPUT=$PWD/setup-input.json ./install.sh
```

脚本会先校验所有 SHA-256，再 `docker load`、启动数据库、运行 `migrate up`、执行同一事务内的 `setup init`、显示 `setup status`，导入 License 后才启动应用并检查 `/healthz` 与 `/readyz`。签发方提供 License 时，把 envelope 保存到受限文件并同时设置 `IOLINK_LICENSE_INPUT`；缺少 License 时安装在授权步骤停止，不会启动未完成首启的服务。`./uninstall.sh` 默认只停止并移除容器，保留数据库卷；删除数据必须显式执行 Compose 的 `down -v`。

离线 Compose 使用 `pull_policy: never` 和内部网络；`deploy/reverse-proxy.optional.yaml` 与 `deploy/streaming.optional.yaml` 目前明确标记为 `optional_unimplemented`，不能当作已交付的视频或反代服务。

## M1 迁移fixture注意

003迁移新增pond快照、逐字段时间、采样ID窗口和报警唯一约束。旧遥测没有塘快照，只能按仍存在的设备当前塘回填；设备已删除的孤立旧样本保留NULL，不伪造归属，未来授权查询必须拒绝不可归属记录。旧影子各字段时间只能以原影子ts回填，不能恢复旧版本未保留的逐字段历史。

若旧schema fixture已有同设备/同塘/同指标的重复未确认报警，或存在非法规则，003会整体失败回滚。演练中需逐项核对并显式修复后重试，不会静默删除报警、自动确认或修改阈值。已提交001/002的fixture升级失败时保留原版本；空库一次迁移失败则全部回滚。此演练验证迁移安全，不承诺生产数据升级兼容。

M1已新增通知outbox持久记录；可靠发送、重试、发送状态与微信真机仍属M5。当前兼容的best-effort发送发生在提交之后，不可把pending行当作送达证据。

## M5/M6c 发布与恢复目标

1. 校验发布包manifest/hash和依赖版本，准备隔离数据库、应用角色、持久卷、配置。
2. 起数据库并执行migrate up/status；如需覆盖迁移兼容路径，只能使用上节隔离旧schema fixture。
3. 设置管理员；M6b仅在非生产旧库fixture中回填默认租户并生成 membership，M6c提供离线安装脚本；部署反代证书并验证readiness（数据库、迁移版本、broker就绪）。
4. 注册测试设备→MQTT上报→查询→触发/确认报警，保存完整证据；仅healthz=ok不足以验收。
5. 发布/恢复演练前停新写/排空worker并做一致备份，记录旧镜像与schema版本；迁移失败或进程中断时停止服务并保留错误，不把部分迁移当成功。
6. schema不兼容时禁止直接回退旧二进制；只能在非生产fixture上先恢复已验证备份，再回退旧镜像并核对数据。该演练验证安全恢复路径，不构成已部署客户二进制回退或升级兼容承诺。记录维护窗口，不能声称零停机。

## TLS与端口

验收基线选择宿主机Nginx终结TLS，应用容器仅绑定127.0.0.1:8080与127.0.0.1:1883，DB不发布宿主端口。公网仅443/8883，M7媒体另列端口清单；不把内部metrics公开为业务接口。

以下为目标完整块结构示例，需要替换域名/证书，确认Nginx构建包含stream模块；不宣称当前Compose已经按此配置：

```nginx
# 顶层，和http同级
stream {
    server {
        listen 8883 ssl;
        ssl_certificate /etc/nginx/certs/fullchain.pem;
        ssl_certificate_key /etc/nginx/certs/privkey.pem;
        proxy_pass 127.0.0.1:1883;
    }
}
http {
    server {
        listen 443 ssl;
        server_name iolink.example.com;
        ssl_certificate /etc/nginx/certs/fullchain.pem;
        ssl_certificate_key /etc/nginx/certs/privkey.pem;
        location = /metrics { deny all; }
        location / { proxy_pass http://127.0.0.1:8080; }
    }
}
```

该片段需与Nginx的events等主配置合并后`nginx -t`检查，已有http/stream时只合入内部server，禁止重复外层块。设备和浏览器均验证服务端证书；自签仅用于测试且客户端明确信任，不用跳过验证代替TLS验收。

## 一致备份与隔离恢复

目标选择整库逻辑备份，不沿用未经验证的业务/时序分拆。每日备份保留14天，异机副本与校验和，凭据单独保管；RPO<=24小时，100GiB验收数据集目标RTO<=2小时。备份失败告警且不删除最后一份成功备份。

在有权限访问的源数据库使用`pg_dump -Fc`保存整个数据库；同时记录PG/Timescale精确版本、数据库角色/权限清单、schema_migrations版本与各表验证摘要。使用相同版本的目标环境恢复，应用不连接目标库直到验收完成。以下为恢复流程规格，实际连接参数由运维提供：

1. 在隔离目标创建角色和空数据库，安装与源一致的Timescale扩展。
2. 调用`timescaledb_pre_restore()`准备目标。
3. 用`pg_restore`恢复custom格式全库备份；不用并行`-j`，出现未解释错误即失败。
4. 调用`timescaledb_post_restore()`恢复正常运行；前一步失败时保留隔离库用于诊断，不对外开放。
5. 核对业务和遥测行数、min/max时间、抽样值/摘要、影子、报警、索引、序列、迁移版本和retention任务；再跑采集/查询/报警回归。

Timescale全库恢复准备/收尾及非并行限制参考[官方逻辑备份文档](https://github.com/timescale/Tiger-Data-Docs/blob/main/src/content/docs/deploy/self-hosted/backup-and-restore/logical-backup.mdx)，查看日期2026-09-19。实际版本差异必须在恢复演练中验证。

`deploy/backup.sh`使用操作者提供的PG环境变量生成单个custom格式全库备份和sha256校验文件，校验文件只记录备份文件名，便于异机复制。`deploy/restore.sh`要求`IOLINK_RESTORE_TARGET=isolated`且目标数据库名匹配`iolink_restore_*`，在隔离目标校验后恢复并检查迁移表。两个脚本不记录DSN或口令，也不依赖固定Compose容器名。恢复演练仍需按本流程核对业务/时序摘要；未完成真实演练前不能称R31通过。无硬件或微信凭据不阻止恢复测试，但完整外部链路仍单列未验。

## 可观察性与关卡

内部诊断日志输出 JSON 行，常驻服务写入 `deploy/logs/iolinkd.jsonl`，同时输出到 stderr。
使用复制到服务器的文件时，日志目录是该 Compose 文件旁的 `logs/`。
默认 `IOLINK_LOG_LEVEL=info`；排查内部采集/通知处理可临时设置 `debug` 并重建应用容器。
`IOLINK_LOG_MAX_MB` 为每个文件的MB上限（1–1024，默认20），
`IOLINK_LOG_BACKUPS` 为轮转备份数量（1–20，默认5），默认总预算约120MB。
备份由轮转库以时间戳命名；不要让其他进程或宿主 logrotate 同时轮转这些文件。
迁移和管理员 CLI 仅输出 stderr，避免与常驻服务争用同一日志文件。

日志文件权限为0600，Docker 默认以 root 写入，宿主读取使用 `sudo tail -f logs/iolinkd.jsonl`；
不应将日志目录作为公开静态文件目录。每个 API 请求返回服务器生成的 `X-Request-ID`，
日志记录路由模板、状态码、耗时和已验证的租户/用户ID；不记录请求 body、header、query
或原始动态路径。设备标识使用稳定 hash 关联，SDK packet/payload、密码、JWT、微信凭据
以及原始数据库错误值均不进入日志。错误记录保留安全类别和 PostgreSQL SQLSTATE。
未知错误记录类型，结合路由、处理阶段和指标定位，不输出原始错误文本中的数据。

日志目录无法创建/打开时拒绝启动；运行中写入失败会在 stderr 报告并使 `/readyz` 返回503，
修复磁盘或目录后重启服务。只读目录、非法级别/轮转范围也会拒绝启动。
日志功能不替代通知 outbox、业务审计表或数据库备份。

存活/就绪分开；就绪失败不接新业务；指标包括在线数、接入/持久化失败、遥测、报警、通知成功/失败、队列长度和任务延迟。日志可定位tenant/device/event而不含secret。SIGTERM先撤销就绪、停止MQTT接入、关闭HTTP新请求，再在10秒上限内排空通知；重复状态事件不导致在线数负值。

R30验证端口隔离与TLS，R31恢复数据，R32发布/恢复安全，R33容量和磁盘预算，R40离线安装。每次演练保存输入版本、命令、耗时、退出码和核对结果，不能只保存“已提交脚本”。
