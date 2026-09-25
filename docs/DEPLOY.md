# 部署、迁移与恢复

2026-09-19，M0迁移/安全首启已实现并在隔离环境验收。以下明确区分可运行入口和M5/M6c目标；TLS、备份恢复、容量和离线安装尚未验收，不能声称生产就绪。旧docs/schema.sql仅用于历史基线，不是安装入口。

## 配套服务与版本

核心iolinkd（含构建后的管理前端）、PG16+TimescaleDB、TLS反代；M7另有流媒体。发布包固定镜像版本和digest，Go/Node/前端依赖锁文件及扩展版本写manifest；不能用latest作为可复现证据。起始验收机4vCPU/8GiB/SSD、Linux x86_64、Docker Engine+Compose v2，记录精确版本；ARM64另验。

配置包含HTTP/MQTT监听地址、PG_DSN、SECRET_KEY（至少32字节随机）、微信三项和模板字段映射、每设备上报周期/离线倍数、日志级别；生产缺关键配置拒绝启动。凭据用受限权限文件或secret注入，日志/备份清单脱敏。首次安装由本地CLI设置管理员，旧默认密码必须变更后才能开放业务入口。

## M0 已实现的安装入口

先构建 `go build -o /tmp/iolinkd ./cmd/iolinkd`。在隔离开发环境可用 `docker compose -p iolink-dev -f deploy/docker-compose.yml up -d --wait` 起数据库；开发Compose仅本机监听，固定开发口令不用于生产。提供 `IOLINK_PG_DSN` 和至少32字节随机 `IOLINK_SECRET_KEY` 后运行：

```bash
/tmp/iolinkd migrate status
/tmp/iolinkd migrate up
/tmp/iolinkd admin init operator < /run/secrets/iolink-admin-password
/tmp/iolinkd serve
```

口令文件须事先以受限权限创建，内容12–256字节、不含空白；文件路径为部署者提供的实际受限文件。CLI不从argv或环境读取管理员口令，也拒绝终端直接回显输入。初始化仅允许库内尚无管理员时执行，不安装公开默认口令。`migrate up` 可重复运行；`serve` 仅检查版本，不自动迁移。

已有旧库必须先停旧应用、备份，再执行 `migrate adopt-legacy`。该命令只接管与仓库遗留DDL指纹、Timescale时序表和13个月保留策略匹配的八表旧库；出现未知表或漂移即拒绝，不能绕过检查硬写版本。旧公开默认密码被强制门禁阻止，需运行 `admin reset-password admin < /run/secrets/iolink-admin-password` 后才能启动。新版本/checksum不兼容也拒绝启动。当前M0仅递增token_version，既有HTTP token的撤销校验还在M2待实现；本地重置需先停服务并轮换根密钥，再启动以使既有令牌失效。

生产Compose须在 `deploy/.env` 受限文件配置强随机PG口令、根密钥，支持 `IOLINK_REPORT_INTERVAL=60|300`（默认60）和 `IOLINK_OFFLINE_GRACE=1..10`（默认3）。PG口令使用URL安全字符，例如随机hex，避免嵌入DSN产生URL歧义；任意口令须正确URL编码连接串。迁移和管理员命令在起常驻应用之前运行：

```bash
docker build -t iolinkd:latest .
docker compose -p iolink-prod -f deploy/docker-compose.prod.yml up -d --wait db
docker compose -p iolink-prod -f deploy/docker-compose.prod.yml run --rm --no-deps iolinkd migrate up
docker compose -p iolink-prod -f deploy/docker-compose.prod.yml run --rm --no-deps -T iolinkd admin init operator < /run/secrets/iolink-admin-password
docker compose -p iolink-prod -f deploy/docker-compose.prod.yml up -d iolinkd
```

上述容器流程提供已实现命令的使用方式；M0实际验收使用本机二进制+隔离Timescale容器，不冒称已完成生产容器/TLS验收。正式发布须使用固定发布标签/digest取代本地构建的latest标签。

## M1 迁移兼容注意

003迁移新增pond快照、逐字段时间、采样ID窗口和报警唯一约束。旧遥测没有塘快照，只能按仍存在的设备当前塘回填；设备已删除的孤立旧样本保留NULL，不伪造归属，未来授权查询必须拒绝不可归属记录。旧影子各字段时间只能以原影子ts回填，不能恢复旧版本未保留的逐字段历史。

若旧库已有同设备/同塘/同指标的重复未确认报警，或存在非法规则，003会整体失败回滚。需管理员逐项核对并显式修复后重试，不会静默删除报警、自动确认或修改阈值。已提交001/002的旧版本升级失败时保留原版本；空库一次迁移失败则全部回滚。

M1已新增通知outbox持久记录；可靠发送、重试、发送状态与微信真机仍属M5。当前兼容的best-effort发送发生在提交之后，不可把pending行当作送达证据。

## M5/M6c 升级与发布目标

1. 校验发布包manifest/hash和依赖版本，准备隔离数据库、应用角色、持久卷、配置。
2. 起数据库并执行migrate up/status，旧库按上节显式接管。
3. 设置管理员；M6b迁移默认租户，M6c提供离线安装脚本；部署反代证书并验证readiness（数据库、迁移版本、broker就绪）。
4. 注册测试设备→MQTT上报→查询→触发/确认报警，保存完整证据；仅healthz=ok不足以验收。
5. 升级前停新写/排空worker并做一致备份，记录旧镜像与schema版本；迁移失败停止上线并保留错误，不把部分升级当成功。
6. 向后兼容迁移可回旧二进制；破坏性迁移只能恢复已验证备份与旧镜像再回退。记录维护窗口，不能声称零停机。

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

旧`deploy/backup.sh`固定容器名并分拆导出，仅列为待替换实现；未通过本流程前不能称备份可恢复。无硬件或微信凭据不阻止恢复测试，但完整外部链路仍单列未验。

## 可观察性与关卡

存活/就绪分开；就绪失败不接新业务；指标包括在线数、接入/持久化失败、遥测、报警、通知成功/失败、队列长度和任务延迟。日志可定位tenant/device/event而不含secret。SIGTERM按设计排空有界，重复状态事件不导致在线数负值。

R30验证端口隔离与TLS，R31恢复数据，R32升级/回退，R33容量和磁盘预算，R40离线安装。每次演练保存输入版本、命令、耗时、退出码和核对结果，不能只保存“已提交脚本”。
