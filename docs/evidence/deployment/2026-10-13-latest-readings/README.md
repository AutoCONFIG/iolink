# 既有最新读数完整性与存储故障响应

日期：2026-10-13，基线 `0cf0b27`。范围R10/R17/R22既有池塘状态墙、设备详情和water latest；新视频/网关/任务功能不在本次范围。实现提交 `e1c7b50` / tree `8edf585a14ea1b532aab9e692e05cda96067658e` 已获两位未参与编辑者同快照APPROVE，发布v0.0.35。

管理侧池塘聚合按池塘筛选并遍历全部200条分页，设备数和最新shadow取自同一设备集合；去除全局第一页截断。两侧只将缺失shadow与不适用的通用产品视为无水质数据；真实存储失败返回500且不输出部分陈旧结果。water latest原400 unsupported_product保留。核心Latest将无行映射为domain.ErrNotFound，其他provider错误不降为成功。失败写入注入logger，按现有dump关联/脱敏规则处理。

环境：Go1.26.8、Linux amd64。专属容器 `iolink-latest-pg-20261013`，归属label `iolink.task=latest-readings-20261013`；Timescale镜像digest `6f139d56042989bd35f50ba5986e492cd32e35cc6778c50be2659add298dc09f`，PostgreSQL16.15 / TimescaleDB2.30.2，端口127.0.0.1:33126。测试通过IOLINK_TEST_PG_DSN自动建/删独立数据库；凭据不归档。匿名卷 `b62b5f0acd704ad15292b2b326a0a64ef035b05be8b1d49fdf340a3bdf98fe7d` 在验证和审阅结束后清理。

## 已观察结果

- `admin-before.log`：原handler对201设备只返回200台且最新值null；2个provider故障读取仍200。新增测试先复现失败。
- `app-before.log`：原handler10个失败子场景，包括数据库故障/超时返回200及混合塘返回陈旧部分读数。
- `admin-after.log`：对应管理handler race测试通过。
- `app-after.log`：30个真实认证HTTP子场景race通过，包含正向、缺失shadow、通用产品、故障/超时、混合塘、空塘及安全关联错误日志；存储是消费者fake，不冒称PG。
- `real-pg-http.log`：`GIN_MODE=release IOLINK_TEST_PG_DSN=<专属> go test -race -shuffle=on -count=1 -v ./internal/core -run TestLatestReadingHTTPWithLargePondAndDatabaseFailure` exit0，无SKIP。真实核心入库生成第201台最新shadow，两个认证HTTP状态墙一致为201设备；空塘为0/null；缺失shadow可识别。临时重命名shadow表造成真实PG错误，六个latest HTTP读取面均500，认证仍正常。

## 最终冻结与双审

初tree `6ed49166e940861740b2c9a1fee4166c0523e2ca` 全库build/vet/race通过，但独立两审发现管理侧新增500错误码不符合目标契约，均拒绝（两个before-error-fix报告）。修订只将两处响应及对应断言改为约定 `internal_error`；真实故障仍500，安全细节在日志。

最终tree `8edf585a14ea1b532aab9e692e05cda96067658e` 导出 `/tmp/iolink-latest-r2-20261013`，不含旧playback路由/测试草稿或用户AGENTS修改。Go构建嵌入既有web生产dist（web源 `39299dfb0d7771690117a2765fb766a9b715f554`），未改前端源码。

- `snapshot-final-verify.log`：最终导出 `GIN_MODE=release IOLINK_TEST_PG_DSN=<专属> make verify` 终端exit0，build/vet/race通过，33个有测试包；PG用例具备真实DSN，非跳过验收。
- `final-focused.log`：修订后两侧HTTP、管理跨页及真实PG/six故障面race回归exit0；`contracts.log`为96 operations / 568 synthetic fixtures通过，不代替所有live handler契约。
- [最终代码审阅](code-review.md)、[最终门审](gate-review.md)：两位未编辑者对同tree明确APPROVE，各自复跑真实PG/HTTP与race；门审核对1173快照blob及最终全库完成记录。

旧 `snapshot-verify.log` 对应6ed初树，保留为历史；最终批准依赖8edf结果。浏览器交互由用户验收；真实微信、硬件、License交付及客户环境运维验收不在这组软件结果内。专属容器和导出在最终V1补证审阅结束后由根代理核对归属并清理。
