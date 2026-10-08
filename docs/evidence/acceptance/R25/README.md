# R25 微信登录

软件替代验收：`python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software`，候选快照通过小程序测试、类型检查和生产构建。

外部状态：`external_blocked`。真实 `wx.login`/code2session 需要项目 AppID、Secret、HTTPS 和真机，软件适配器测试不计作真实微信通过。
