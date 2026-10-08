# IoLink 小程序演示设计约定

## 研究记录

- 继承现有 `web/DESIGN.md` 的运营界面基线：深蓝文字、青绿色主操作、冷灰分隔线。
- 本阶段只补齐 M4 演示状态和 API 边界，不扩展生产级小程序视觉系统；六页页面继续使用原生 Vue 页面和系统字体。

## 令牌与组件

- 颜色令牌：`--mini-ink`、`--mini-muted`、`--mini-line`、`--mini-accent`、`--mini-warning`、`--mini-danger`。
- 间距以 4px 为基础，报警列表使用 8px 间隔，页面内边距为 16/20px。
- `alarm-page` 是页面容器；`alarm-item` 承载严重、预警、已确认三种状态；`subscription` 承载请求、演示结果和不可用提示。
- 所有状态同时显示文字；按钮使用原生 `button`，错误使用 `role="alert"`，加载使用 `role="status"`。

## 已接受债务

- 浏览器演示不会伪装真实微信弹窗；真实 `wx.login`、code2session、订阅授权、真机和 HTTPS 仍记录为 `external_blocked`。
