# 当前软件运维链路核验（V1 / TODO3 / M5 的有界补证）

2026-10-13：从冻结源树 `6ed49166e940861740b2c9a1fee4166c0523e2ca` 导出的 `/tmp/iolink-latest-20261013` 构建并实际执行。web 源提交 `39299dfb0d7771690117a2765fb766a9b715f554`，使用根代理准备的 dist/embed；本次没有修改产品源代码。历史 TODO3/M5 缺失的候选摘要仍是历史缺口，本次结果只绑定当前软件源树，不补造旧收据。

## 已执行与边界

- `go build -o /tmp/iolink-current-ops-20261013/iolinkd ./cmd/iolinkd`，随后实际 CLI 迁移两次、查询 16 版均 applied 且非 dirty、stdin 初始化管理员。不是浏览器首启体验验收。
- `curl -i --max-time 8 -sS http://127.0.0.1:45841/healthz`、`/readyz`、`/metrics` 均 200。指标检验仅为 Prometheus 文本可读取和 Go/process 指标存在，不声称遥测/报警/通知计数已逐一核对。
- `python3 scripts/capacity_smoke.py --url http://127.0.0.1:45841/healthz --clients 20 --requests 100`：100/100 成功，错误率 0，p95 0.01552 秒。仅健康端点小样本，不能替代 R33 的 500 设备 / 24 小时 / 90 天历史查询容量验收。
- 对专属数据库设置 migration 16 dirty：ready 503，health 200；恢复 dirty=false 后 ready 200。禁止数据库连接并终止该数据库会话后 health/ready 均 503；重新允许连接后实测若干失效连接响应仍 503，约 0.426 秒恢复 200。[原始响应](db-recovery-probes.log)保留了过渡失败。
- 真实 `SIGTERM` 后 0.00317 秒退出，exit=0；已提交 SQL fixture 随后仍可备份、比对。[退出记录](shutdown.json)。这不是高负载排空或未知外部副作用演练。
- 使用容器中 PG16 工具执行原样 `deploy/backup.sh` 和 `deploy/restore.sh`，在空隔离库恢复成功。源与恢复库 12 张表的行数及完整行内容有序 JSON 摘要一致：farms/ponds/devices/alarms 各 1，sensor_data 2，device_shadows/telemetry/products/product_models/jobs/outbox_events/notification_outbox 各 1。[SQL](compare.sql)、[源](source-rows.log)、[恢复](restored-rows.log)。这是 SQL 直接种入的微型 fixture，不表示 MQTT 上报或实际通知已由本次进程驱动。
- 脚本自身还核对 16 条迁移记录、90 个 public 索引和旧 sensor_data retention job 1 个。[元数据](backup.meta)、[内置汇总](backup.summary)。内置汇总尚未覆盖新增 generic telemetry、产品、jobs/outbox 等表，本次用独立 SQL 补验这些表，不据此声称脚本已扩展汇总。
- 非 `iolink_restore_*` 目标、非 isolated 标记、损坏 dump 校验和分别 exit=2。拒绝后恢复库 12 表摘要不变。[拒绝后](restore-after-reject.log)。备份二进制包含临时管理员哈希，未提交；已连同容器临时目录清理。
- 冻结导出运行 `go test -race -shuffle=on -v -count=1 ./internal/migrate ./internal/persistence ./internal/platform ./internal/operations ./internal/notifications`，配置真实专属 `IOLINK_TEST_PG_DSN`：5 包通过，44 个顶层测试通过，无 SKIP/FAIL。[日志](focused-race.log)。包括迁移失败回滚/checksum/并发、产品租户隔离、配置脱敏、健康依赖失败、通知 lease/重验/重试/unknown。微信仅受控 HTTP/fake 软件测试，未实际发送微信消息。

## 环境、调用和复现

[environment.log](environment.log)：Go 1.26.8、Python 3.14.6、curl 8.21.0、PG 16.15 / Timescale 2.30.2。构建二进制 SHA256 `09bb98f7e613433997779808fbf923877574bbc0c0f10389f7bf5f0ef3035cbb`。

容器 `iolink-latest-pg-20261013`，标签 `iolink.task=latest-readings-20261013`，回环地址端口 33126。QA 只创建 `iolink_ops_20261013` / `iolink_restore_ops_20261013`；Go 测试由仓库 testdb helper 创建和删除自身数据库。PG 密码通过环境变量注入，不在日志、命令记录或文档中公开。

完整实际命令及断言保存在 [原始脚本记录](run_ops_qa.capture.txt)，成功调用输出是 [qa-run.log](qa-run.log)。这是本次运行脚本的原始字节记录，固定输出指向本归档目录，不能直接执行重放。复验须另建明确归属的数据库、容器临时目录和独立输出目录，重建并核对源树/二进制身份、重新采集environment；按下列命令与场景矩阵逐步执行并另存结果，不覆盖本目录。PG口令仅从环境提供，不记录。原错误日志中 `.py` 路径为当时名字，原文保留；文本记录与原脚本字节一致。

备份调用：`docker exec -e PGPASSWORD -e PGHOST=127.0.0.1 -e PGUSER=iolink -e PGDATABASE=iolink_ops_20261013 -e BACKUP_DIR=/tmp/iolink-current-ops-20261013/backups iolink-latest-pg-20261013 bash /tmp/iolink-current-ops-20261013/backup.sh`。恢复使用同样 provider 环境，改 `PGDATABASE=iolink_restore_ops_20261013`，加入 `BACKUP_FILE=<backup.log 中记录的绝对路径>`、`IOLINK_RESTORE_TARGET=isolated`，执行 `restore.sh`。[备份](backup.log)和[恢复](restore.log)保留实际输出和 exit code。

初次 QA 脚本把目标库已有 Timescale extension 当成不存在，命令 `CREATE EXTENSION timescaledb` 失败；保留 [初次日志](qa-first-attempt.log)、[错误](restore-extension-first-attempt.log)，改为 `IF NOT EXISTS` 后运行通过。另一次脚本把允许连接后的第一请求必须 200 当作条件，实际仍有失效连接 503；保留 [该次日志](qa-recovery-first-attempt.log)、[第一响应](health-recovery-first-request.log)，改为记录每次真实响应的有界恢复探测。本次结果采用最后完整成功运行，未把前两次失败算通过。

## 资源和验收状态

两 QA 数据库已强制删除，容器内 `/tmp/iolink-current-ops-20261013` 已删除：[数据库清理](cleanup-iolink_ops_20261013.log)、[恢复库清理](cleanup-iolink_restore_ops_20261013.log)、[临时目录清理](cleanup-container-tmp.log)。共享容器由根代理保留用于独立审阅；QA 不停止它。主机二进制/日志临时目录也已清理；[最终确认](cleanup-final.log)证明两 QA 库数量为0、容器内及主机临时路径都不存在。

本补证不是阶段独立审阅的替代。最终同快照双审由根代理另行绑定，审阅结果未到时不可据此宣布整个 V1 / M5 / R01–R55 完成。公网 TLS、真实微信/硬件、100GiB 恢复 RTO、RPO 调度、发布中断与备份回退、24 小时设备容量仍缺外部或专门演练证据。浏览器体验由用户测试，本次未运行浏览器。

[manualQa 场景矩阵](current-software-manual-qa.md)列出每项实际表面、精确调用和证据引用。

## 根代理补充：最终源码与状态核对

上面实际二进制和运维演练来源是6ed树，不冒称它是后来的发布二进制。6ed至最终最新读数实现树 `8edf585a14ea1b532aab9e692e05cda96067658e` 只改管理侧两个遥测失败响应字符串及对应测试断言；迁移、persistence、platform、health、main、notifications、backup/restore及运维调用链源码均未改。最终实现提交 `e1c7b50`，发布v0.0.35；最终源码的完整真实Timescale build/vet/race重新执行通过，33有测试包：[最终全库](../../../deployment/2026-10-13-latest-readings/snapshot-final-verify.log)。最新读数的独立双审不代替本次运维补证/台账审阅。

[55行范围核对](rxx-state-review.md)登记当前历史范围及其局限。R01完整无缓存/CI证据不足、R02.c未齐、R39.b未验，将这三行整体not_run；R03/R04将旧planned描述替换为真实记录；标准契约、迁移和配置provider-operation只提升已实测子范围。外部阻塞和未来功能没有改为通过。

`state-regressions.log`为更新状态后Python107测试通过；`state-cli.log`记录真实CLI全清单通过、M0因R01/M6c因R39整体not_run而明确阻塞、M6b仅.a及M5软件仍通过，所有历史gate收据字节保持不变。阻塞是纠正整体状态后的预期结果，不是把未完成范围绕过成passed。

本轮独立双审候选从已发布v0.0.35源码建立，只修改台账和证据；两份明确批准后才登记V1有界对账完成。历史TODO3/M5摘要不可恢复的限制保留，本次提供新日期、明确源树的软件结果，不追改旧报告。后续记录以IMPLEMENTED最新V1行和最终审阅收据为准。
