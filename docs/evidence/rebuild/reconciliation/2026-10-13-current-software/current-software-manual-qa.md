# manualQa：当前源的有界运维软件

源树 `6ed49166e940861740b2c9a1fee4166c0523e2ca`。以下 PASS 均指对应局部软件行为，不是整个需求完成状态。

## surfaceEvidence

| scenario id | criterion reference | surface | exact invocation | verdict | artifactRefs |
|---|---|---|---|---|---|
| OPS-01 | R03/R04 CLI 子范围 | 构建二进制 CLI + 真 PG | `iolinkd migrate up` 两次；`iolinkd migrate status`；`iolinkd admin init ops-qa`，口令 stdin | PASS：16 applied/clean，重复成功 | A01/A02 |
| OPS-02 | R29 健康/就绪 | 实际 serve 进程 HTTP | `curl -i --max-time 8 -sS http://127.0.0.1:45841/healthz` 和 `/readyz` | PASS：200/200 | A03 |
| OPS-03 | R29 指标可用子范围 | 实际 HTTP | 同 curl 调用 `/metrics` | PASS：200，Go/process 指标文本存在；业务计数不在本项内 | A04 |
| OPS-04 | R33 工具烟测子范围 | 已有 capacity CLI + 真 HTTP | `python3 scripts/capacity_smoke.py --url http://127.0.0.1:45841/healthz --clients 20 --requests 100` | PASS：100 成功、0 错误、p95 0.01552 秒 | A05 |
| OPS-05 | R29 关闭子范围 | OS 信号 + 进程等待 | Python `server.terminate()` 发 SIGTERM；`server.wait(timeout=15)` | PASS：exit0，0.00317秒 | A06 |
| OPS-06 | R31 微型一致恢复 | 原样 shell 脚本 + PG/Timescale 工具 | README 记录的 `docker exec … bash …/backup.sh` 和 `restore.sh`；`psql -X -v ON_ERROR_STOP=1 -U iolink -d <source/restore> -At` stdin=`compare.sql` | PASS：12 张非空表逐行摘要一致，内置迁移/索引/旧 retention 汇总一致 | A07/A08/A09 |
| OPS-07 | R03/R04/R28/R29 现有回归 | 冻结导出 Go race + 真数据库 | `go test -race -shuffle=on -v -count=1 ./internal/migrate ./internal/persistence ./internal/platform ./internal/operations ./internal/notifications`；真实专属 DSN 仅环境传入 | PASS：5包、44顶层测试，无 SKIP | A10 |

## adversarialCases

| scenario id | criterion reference | adversarial class | expected behavior | verdict | artifactRefs |
|---|---|---|---|---|---|
| OPS-A01 | R29 | 脏迁移依赖 | `psql` stdin `UPDATE schema_migrations SET dirty=true WHERE version=16;` 后 ready503、health200；清除 dirty 后 ready200 | PASS：实际 curl 符合 | A11 |
| OPS-A02 | R29 | 数据库断连 | `ALTER DATABASE iolink_ops_20261013 ALLOW_CONNECTIONS false;` 并 `pg_terminate_backend` 仅该库会话；health/ready503 | PASS：依赖失败拒绝就绪，未继续返回200 | A12 |
| OPS-A03 | R29 | 断连后恢复 | 允许连接 true，按真实 curl 记录有界探测；随后 health/ready200 | PASS：约0.426秒恢复，保留中间503 | A13 |
| OPS-A04 | R31 | 非隔离库目标 | restore.sh `PGDATABASE=iolink_ops_20261013 IOLINK_RESTORE_TARGET=isolated` | PASS：exit2 明确拒绝 | A14 |
| OPS-A05 | R31 | 非隔离授权标记 | restore.sh `PGDATABASE=iolink_restore_ops_20261013 IOLINK_RESTORE_TARGET=production` | PASS：exit2 明确拒绝 | A15 |
| OPS-A06 | R31 | 损坏 dump | `docker exec … truncate -s 1 <backup-file>`，随后 restore.sh 保持原 checksum sidecar | PASS：exit2 checksum mismatch；恢复库12表未变 | A16 |
| OPS-A07 | 当前 CLI/HTTP 范围 | 浏览器交互 | not_applicable：本次未修改 UI，用户负责浏览器验收；CLI/HTTP 证据不能替代它 | not_applicable | — |

## artifactRefs

所有路径相对当前证据目录，文件均为本次实际调用输出；A01 源/环境绑定由根代理同时核对导出身份。

| id | kind | description | path |
|---|---|---|---|
| A01 | environment | 工具/provider/容器归属与二进制 SHA | environment.log；surface-summary.json |
| A02 | CLI transcript | applied/clean 16版、重复迁移、初始化 | migrate-before.log；migrate-up.log；migrate-repeat.log；migrate-after.log；admin-init.log |
| A03 | HTTP transcript | 正常 health/ready | health-ready.log；ready-ready.log |
| A04 | HTTP transcript | 原始指标响应 | metrics.log |
| A05 | CLI JSON | 小规模健康探测 | capacity-smoke.log |
| A06 | process observation | 信号后实际等待结果 | shutdown.json；server.log |
| A07 | SQL fixture | 非空事务数据与实际创建输出 | fixture.sql；fixture-create.log |
| A08 | shell transcript | 原样备份恢复输出和内置摘要 | backup.log；restore.log；backup.meta；backup.summary；backup.sha256 |
| A09 | SQL/data transcript | 十二表完整行有序 JSON 摘要 | compare.sql；source-rows.log；restored-rows.log |
| A10 | Go transcript | race+真数据库回归 | focused-race.log |
| A11 | SQL/HTTP transcript | dirty/recover 状态响应 | dirty-set.log；ready-dirty.log；health-dirty.log；dirty-clear.log；ready-recovered.log |
| A12 | SQL/HTTP transcript | 断连与拒绝就绪 | db-connections-deny.log；health-db-unavailable.log；ready-db-unavailable.log |
| A13 | SQL/HTTP transcript | 恢复及保留过渡503 | db-connections-recover.log；db-recovery-probes.log；health-db-recovered.log；ready-db-recovered.log |
| A14 | shell transcript | 命名隔离拒绝 | restore-nonisolated.log |
| A15 | shell transcript | 标记隔离拒绝 | restore-flag.log |
| A16 | shell/SQL transcript | 损坏拒绝及库未变化 | tamper.log；restore-checksum.log；restore-after-reject.log |

未执行真实微信发送、公网 TLS、100GiB 恢复或24小时设备负载；这些需求未由本矩阵给出 PASS。
