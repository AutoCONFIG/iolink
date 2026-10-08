# IoLink 小程序接口演示（M4）

本目录提供 P01–P06 六页的最小接口演示，用于检查登录、池塘、实时数据、历史、报警及物模型接口的返回结果。正式小程序由专人开发。`src/api.ts` 使用 `/api/v1` 与 `/api/v2` 契约，并在响应边界校验结构。

从仓库根目录运行，后端地址由 Vite 代理配置：

```bash
npm ci --prefix web-mini
IOLINK_API_TARGET=http://127.0.0.1:8080 npm run dev --prefix web-mini -- --host 127.0.0.1
```

打开终端输出的本地地址。登录页填写有效微信 code；资源页也可填写测试用户的 Bearer Token，输入设备、指标和时间范围后调用接口。成功结果显示 JSON；登录 token 在结果展示中隐藏。报警页支持加载、刷新和确认；订阅同意/拒绝/取消按钮仅演示结果，不调用真实微信弹窗。

`IOLINK_API_TARGET` 默认 `http://127.0.0.1:8080`。这些输入只用于本地演示；真实 `wx.login`、code2session、订阅弹窗和真机渲染需要微信资质、HTTPS 域名及设备，保持 `external_blocked`。

验证命令：`npm test --prefix web-mini`、`npm run typecheck --prefix web-mini`、`npm run build --prefix web-mini`。本轮范围与日志见 [M4 证据](../docs/evidence/M4/2026-10-08/README.md)。
