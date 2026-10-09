# 产品与物模型角色权限修复

日期：2026-10-09。范围：修复普通租户成员读取产品/物模型时错误返回403的问题。
写操作仍由租户策略限制为 owner/admin；本次不改变平台管理员、租户创建、License 或数据模型。

## 根因

线上账号 `user` 是 `authority=USER`、租户 `2` 的 active `member`。授权策略允许
member/viewer/support 读取 `products`，但 `internal/adminapi/products.go` 的两个 GET
handler 又重复要求 owner/admin，造成规则冲突。服务器日志重复记录
`GET /user/v1/products`、`tenant_id=2`、`actor_id=4`、`status=403`。

## 修改

- 删除产品列表和物模型列表 GET handler 的额外管理员判断，交由 `tenantRequired` 的
  resource/action 策略执行读取授权。
- 创建产品、创建模型、发布模型、设备模型绑定仍走 products/devices write policy，
  member/viewer/support 继续返回 403。
- Web 产品页仅对 owner/admin 显示创建、建模、发布和设备绑定控件；其他角色可以查看目录和模型。
- 修复 console SSR 测试中未完成的 `getPlatformStats` mock，避免无关的随机超时。

## 验证

| 命令 | 结果 |
|---|---|
| `go test -race -shuffle=on ./internal/adminapi -count=1` | PASS；包含 member GET 200、写操作 403 回归 |
| `go test -race -shuffle=on ./internal/adminapi ./cmd/iolinkd -count=1` | PASS |
| `npm test -- --run` | PASS，8 files / 58 tests |
| `npm run typecheck` | PASS |
| `npm run build` | PASS |

线上只读核验：`/readyz` 返回 `ready`，Compose 中 `db` healthy、`iolink` healthy；
`/user/v1/farms`、`/user/v1/ponds`、`/user/v1/stats` 等现有读取接口返回200。
线上镜像尚未替换，待根仓库发布新 tag 后再执行部署和真实 member API 验证。

## 发布

Web 子仓库提交 `03b4286` 已推送 `github/main`。根仓库提交和镜像 tag 需绑定后端修复、
前端子模块指针及本证据；现有 `v0.0.14` 保持不可变。
