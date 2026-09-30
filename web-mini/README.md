# IoLink 小程序边界（M4）

本目录冻结 P01–P06 六页和小程序 API/状态语义，供 uni-app 页面层接入。`src/pages.ts` 是页面清单；`src/domain` 包含登录过期、无归属、离线、历史查询、三倍周期过期和订阅决策规则；`src/api.ts` 只依赖 `/api/v1` 目标契约。

当前交付为可独立测试的软件边界，真实 `wx.login`、code2session、订阅弹窗和真机渲染需要项目方微信资质、HTTPS 域名和设备，记录为 `external_blocked`，不得把 mock 当成真机通过。

```bash
npm ci
npm test
npm run typecheck
```
