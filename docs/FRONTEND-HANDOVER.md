# 管理后台前端 · 开发交接文档

> 给前端工程师的自足上手文档。读完本文即可开工,不需要读后端代码。
> 你的工作对应 PLAN.md 的 **L2 线(管理前端)**;契约如有变动会在此文档与 admin-openapi.yaml 同步更新。

## 1. 你要做什么

为智慧水产监测平台开发 **Web 管理后台**(Vue3),部署形态是编译后嵌入后端二进制
(`go:embed`),生产环境与后端**同源同端口**,无需处理 CORS。

技术栈(已定,不纠结):**Vue3 + TypeScript + Vite + Element Plus**,
推荐从 [soybean-admin](https://github.com/soybeanjs/soybean-admin) 或 vben 模板起步
(布局/菜单/登录页/请求封装开箱即得),只写业务页面。

## 2. 页面清单(6+1 个)

| 页面 | 路由建议 | 核心接口 | 要点 |
|---|---|---|---|
| 登录 | /login | `POST /admin/v1/login` | 用户名+密码;token 存 localStorage,请求头 `Authorization: Bearer <token>` |
| 总览 | /dashboard | `GET /stats` + `GET /api` 数据 | 统计卡(devices_total/online/offline/open_alarms)+ 池塘状态墙(每池塘一卡:🟢🟡🔴 + 四参数) |
| 池塘管理 | /ponds | `/farms` `/ponds` CRUD | 删除有设备绑定的池塘会 409,前端提示先移除设备 |
| 设备管理 | /devices | `/devices` | **注册成功弹窗展示 secret,明确提示"仅显示一次"**(关掉就再也拿不到) |
| 报警规则 | /alarms/rules | `/alarm-rules` CRUD | metric 下拉固定五项;min/max 至少填一个;level: critical(红)/warning(黄) |
| 报警中心 | /alarms | `/alarms` + confirm/batch-confirm | 列表按级别标色;单条确认 + 批量确认 |

池塘状态色规则(与小程序一致):有 critical 未确认报警=🔴;只有 warning=🟡;否则 🟢。
(当前 `/stats` 只有计数,状态墙数据可先用 `GET /ponds` + 前端组合,M2 收尾会加聚合字段——见 §6 待办)

## 3. 接口文档(权威)

**[api/admin-openapi.yaml](api/admin-openapi.yaml)** —— OpenAPI 3.0,全部端点/参数/响应/错误码。
常用端点速查:

```
POST /admin/v1/login                     {username, password} → {token, expires_in}
GET  /admin/v1/stats                     → {devices_total, online, offline, open_alarms}
GET/POST /admin/v1/farms                 养殖场
GET/POST /admin/v1/ponds                 池塘 {farm_id, name, area_mu}
GET/POST /admin/v1/devices               注册: {pond_id, model} → {device_no, secret, ...}
GET/POST/PUT/DELETE /admin/v1/alarm-rules 规则 {pond_id, metric, min_value?, max_value?, level}
GET  /admin/v1/alarms?limit=             报警列表
POST /admin/v1/alarms/{id}/confirm       确认 → 204
POST /admin/v1/alarms/batch-confirm      {ids:[...]} → {confirmed: n}
```

## 4. 接口约定(重要,与常见后端风格略有差异)

- **REST 原生风格**:成功 = 2xx + 业务 JSON(无 `{code,message,data}` 包裹);失败 = 4xx/5xx + `{"error":"原因"}`
- **时间格式**:RFC3339,如 `"2026-09-16T11:08:14.943963+08:00"`
- **字段命名**:snake_case(`device_no`、`min_value`、`area_mu`)
- **鉴权失败**一律 401 → 前端统一拦截跳登录
- **删除冲突**:池塘删除 409(有设备);前端捕获并提示
- 枚举值:status=`online|offline`;level=`critical|warning`;metric 五项白名单
  `temperature | dissolved_oxygen | ph | turbidity | salinity`(单位见 §6)

## 5. 本地开发环境

后端一条命令起(含数据库):

```bash
git clone https://git.hyhy.fun/rsplab/iolink.git && cd iolink
make dev        # 起 PostgreSQL/TimescaleDB + iolinkd(:8080)
# 首次启动自带种子数据: 管理员 admin/admin123、示范农场/池塘、模拟设备 dev-001
```

前端 dev server 与后端跨端口,**用 Vite 代理**解决(后端不做 CORS,生产同源也不需要):

```ts
// vite.config.ts
server: { proxy: { '/admin': 'http://localhost:8080', '/api': 'http://localhost:8080' } }
```

模拟数据自造:让后端同学给你一条注册好的设备,或自己调 `POST /devices` 注册后,
用主仓的模拟器打数据:`go run ./cmd/mqtt-sim -device <device_no> -secret <secret> -interval 5s`
(需本机 1883 端口空闲;`make dev` 起的 iolinkd 自带 broker)。

演示种子账号:`admin / admin123`(本地开发;生产会强制修改)。

## 6. 已知边界与近期变动(防踩坑)

1. `GET /ponds` 暂不带状态/latest 聚合字段——总览状态墙先前端组合,M2 收尾后端会补 `status/latest` 字段(字段名会提前在群里同步)
2. 设备 `signal`(RSSI)已在影子中,设备详情后续会暴露;先不用
3. metric 单位对照:temperature ℃ / dissolved_oxygen mg/L / ph 无 / turbidity NTU / salinity ppt —— 图表 y 轴标注用
4. 管理后台路由前缀 `/admin/v1`,与小程序 `/api/v1` 完全独立,token 不互通

## 7. 验收标准(= M3 完成)

浏览器从零走通:**登录 → 建养殖场/池塘 → 注册设备(拿到 secret)→ 配一条阈值规则 →
看到模拟器数据出现在设备列表 → 人为触发报警(sensor_data 低于阈值)→ 报警中心出现并确认**。

## 8. 协作方式

- 仓库:`git.hyhy.fun/rsplab/iolink`,前端代码放 `web/admin/`,feature 分支 → MR
- 契约问题先提 MR 改 `docs/api/admin-openapi.yaml`(后端 review 后同步实现),不要先写代码后补文档
- 后端接口有 bug/缺失 → GitLab issue 或直接找主程
