# R25 微信登录

软件演示验收：源码 `2b66812`；`python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software` 通过。39 项测试、类型检查和生产构建通过；JSON 登录、响应结构解析、错误映射和 token 隐藏已验证。[命令、日志与同快照双审](../../M4/2026-10-08/README.md)。

外部状态：`external_blocked`。真实 `wx.login`/code2session 需要项目 AppID、Secret、HTTPS 和真机，软件适配器测试不计作真实微信通过。
