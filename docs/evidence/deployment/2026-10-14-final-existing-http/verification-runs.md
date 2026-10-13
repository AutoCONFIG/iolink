# 精确源码回归与超时记录

2026-10-14。最终候选 `bbfb29c41c31549899655750e171ef2c25a6d0eb`；Git archive导出 `/tmp/iolink-final-existing-http-20261014-v2`。未改测试断言、默认超时、业务源码或fixture以绕过全库失败。

| 树/命令 | 实际终端结果 | 证据及解释 |
|---|---|---|
| 4a59 / make verify / session59598 | exit2，core600.127s超时 | snapshot-verify.log；默认10分钟整包上限，当时camera子测试仅1秒在迁移中。该树还存在独立确认的能力日志阻断，不计发布通过 |
| 4a59 / focused / session17950 | exit0，core189.370s | snapshot-focused.log；仅旧树专项通过，不代替新树或全库门槛 |
| bbfb / 并行make verify / session77523 | exit2，core600.104s超时 | snapshot2-verify.log；不同M6bAlarmHTTP子测试仅1秒。结果保留失败，不计通过 |
| bbfb / focused / session80051 | exit0，core264.801s，20顶层/无SKIP | snapshot2-focused.log；本批命名的正向、权限、输入、失败及事务场景 |
| bbfb / Python、静态、ruff / session69233 | exit0 | snapshot2-python.log：114通过；snapshot2-contracts.log：96操作/621合成fixture；snapshot2-ruff.log：检查/格式通过 |
| bbfb / 单独make verify / session56771 | exit0，33测试包、core471.529s | snapshot2-serial-verify.log；同一导出、同一默认make verify和真实DSN，其他执行者暂缓PG长任务。build/vet/race通过 |

每次真实数据库运行均设置专属 `IOLINK_TEST_PG_DSN`，每测试独立数据库。默认 `go test -race -shuffle=on ./... -count=1` 的10分钟上限保持原样；单独运行指协调外部执行者不再同时进行长PG回归，Go包本身仍按默认调度。

两次超时发生于不同正在推进的短子测试，不能据此声称有单项业务断言错误，也不能自动宣称不存在其他问题。同一bbfb摄像头PG包在并行运行255.700s、单独运行59.881s；串行完整471.529s低于默认上限。这些观测支持并行资源负载的解释，不等同长期容量或全局性能验收。之后发布只使用最终bbfb完整通过和同树双审，旧失败不覆盖或隐藏。

没有调试器、临时生产端点、数据库性能开关、retry/quarantine或延长sleep。专属容器/卷/网络与导出在对应审阅结束后核对归属清理。Python已有严格类型12项诊断仍未通过，不把ruff或114功能测试写成严格类型通过。
