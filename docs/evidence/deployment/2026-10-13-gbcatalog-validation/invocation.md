# 执行取证补充

全库命令从冻结导出执行，工具调用直接在 `make verify` 前设置非空 `IOLINK_TEST_PG_DSN`，目标为 `127.0.0.1:33057/iolink`，本轮专属容器；不记录口令。该调用返回 exit 0，日志为 `candidate-verify.log`。专属容器及版本见 `environment.log`。

全库日志没有 `-v`，不能单凭该文件逐项列出 SKIP；因此不将浏览器 opt-in 或外部链路称为通过。审阅指出这个取证粒度限制后，另从同一冻结导出对 `internal/gbcatalogpg` 使用同一专属 DSN 执行：

```bash
go test -race -shuffle=on -count=1 -v ./internal/gbcatalogpg
```

结果记录于 `catalog-pg-race.log`；该包使用 `internal/testdb.New`，只有非空 DSN 才能实际建库、迁移和执行存储测试。此补证不替代全库原始结果。
