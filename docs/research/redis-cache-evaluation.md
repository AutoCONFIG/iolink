# Redis 类缓存调研

日期：2026-10-07  
状态：**仅调研，未引入依赖、容器或运行时配置**

## 结论

Redis 适合承载短 TTL、可丢失或可重建的数据，但不应成为业务最终状态的唯一来源。当前不把 Redis 加入生产 Compose，也不把它作为 M6d 或其他正在进行阶段的前置依赖。

首个可行试点是共享限流桶和微信 access token 缓存。nonce 防重放暂时继续使用 PostgreSQL：现有实现把 Key 状态、签名校验、nonce 原子写入和限流更新放在同一个事务边界内，已经覆盖并发、重启和撤销语义。把 nonce 移到 Redis 会增加双写、重启持久化和故障时 fail-closed 的新门槛，当前收益不足以抵消复杂度。

## 当前基线

| 数据 | 现在的位置 | 是否权威 | Redis 结论 |
|---|---|---:|---|
| API Key、secret 密文、scope、资源范围、撤销状态 | PostgreSQL | 是 | 不缓存为权威状态 |
| `/open/v1` nonce 防重放 | `api_key_nonces` PostgreSQL 表 | 是 | 暂不迁移 |
| `/open/v1` 每 Key 令牌桶 | `api_keys.rate_tokens` 与 `rate_last_refill` | 是 | 可在后续试点中改为共享 Redis 原子桶 |
| License、租户/成员授权、业务状态、报警最终数据 | PostgreSQL | 是 | 不放 Redis |
| 微信 access token | 进程内互斥保护的短缓存 | 否，可重建 | 适合共享 Redis，secret 不入 Redis |
| 登录验证码、一次性链接、短期会话状态 | 当前没有 Redis 实现 | 否，可过期 | 适合短 TTL Redis |
| 列表或统计读缓存 | 当前直接读 PostgreSQL | 否，可失效 | 后续按租户和授权版本评估 |

## 候选用途和边界

### 1. 共享限流桶

多进程部署时，Redis 可以让每个 API Key 共用一个令牌桶。实现必须是一个原子命令序列（Lua 脚本或 Redis 事务），输入由服务端计算的当前时间、补充速率、burst 和消耗量组成，不能由多个 `GET`/`SET` 调用拼出非原子逻辑。

建议的逻辑键：

```text
iolink:ratelimit:v1:{key_id}
```

值至少需要保存剩余 token 和上次补充时间，空闲键设置略大于一个窗口的 TTL。Redis 不可用时，签名 API 必须 fail closed 并返回可识别的服务不可用错误；不能静默切回每进程内存桶，否则多实例下会绕过额度。

### 2. 微信 access token

这是最适合先试点的缓存：token 过期后可重新向微信获取，Redis 丢失不会改变业务授权。缓存值只包含 access token 和过期时间，不能写入 AppSecret、请求 URL 或完整响应。刷新应使用短锁或 singleflight，提前留出安全余量（例如供应商过期时间减 60 秒），并在供应商返回 token 失效时删除旧值后只重试一次。

Redis 不可用时，若本进程仍有未过期 token，可以继续使用内存值；没有可用 token 则明确返回通知重试错误。这个降级必须是适配器的显式策略，不得被通用缓存层隐式吞掉。

### 3. 验证码、一次性链接和短期会话

这类数据适合使用随机不可预测的键、严格 TTL、一次性消费和原子 `GET+DEL`。Redis 只保存哈希或不可逆摘要，避免把可直接登录的明文凭据写入日志、备份或监控。消费失败、过期和重复消费都应返回同一安全类别，不能泄漏键是否存在。

### 4. 读缓存

池塘、设备和统计列表可以在确认授权过滤后缓存，但键必须包含 tenant、资源范围和授权版本。例如：

```text
iolink:read:v1:{tenant_id}:{authorization_version}:{resource}:{query_digest}
```

写入业务数据、撤权、资源转移和 License 变化都要使相关版本失效。缓存命中不能绕过 PostgreSQL 的最终资源归属检查；无法证明失效范围时宁可不缓存。

### 5. Worker lease 和分布式锁

Redis 锁只适合短期、可恢复的互斥，不应代替 outbox、命令状态或 PostgreSQL lease。若后续使用，必须有 owner token、续租、过期恢复和 fencing token；否则网络分区后旧 worker 可能继续执行外部副作用。当前通知和持久任务已有数据库租约语义，暂不替换。

## 推荐架构

不要在领域层引入通用 `Cache` 接口。按能力定义窄 port，并由 composition root 选择实现：

- `RateLimiter`：原子消费并返回剩余 token/重试时间。
- `EphemeralStore`：带 TTL 的取值、写入、一次性消费和删除，限定于验证码、短期 token 等临时数据。
- `AccessTokenStore`：只表达微信 token 的读取、写入、失效，不暴露任意 Redis 命令。

Redis adapter 只放在 provider 层，应用层不得依赖 Redis SDK。配置应显式区分 `disabled`、`optional` 和 `required`：安全相关的共享限流配置为 `required` 时连接失败必须阻止启动或让对应入口 fail closed；普通读缓存为 `optional` 时只能在明确记录降级指标后回源 PostgreSQL。

## 部署建议（未来试点）

当前 `deploy/docker-compose.yaml` 和 `deploy/docker-compose.dev.yaml` 保持不变。试点通过独立的开发覆盖文件或 Compose profile 加入 Redis，避免测试服务器在没有配置时意外启动新依赖。Redis 服务只加入内部网络，不发布宿主机端口；使用 ACL 的专用用户、随机密码、独立数据卷和受限命令集。是否启用 AOF、备份和 TLS 要在目标部署拓扑冻结后决定，不能用默认开放配置直接上线。

## 必须补齐的验证

引入任何 Redis adapter 前，至少要有以下证据，且与阶段代码绑定到同一快照：

1. 单元测试：原子限流边界、TTL、时钟跳变、重复消费、锁丢失和错误映射。
2. 真实 Redis 集成：多客户端并发、两个应用实例共享同一桶、重启、数据过期和网络断开。
3. 安全故障：Redis 不可用时共享限流和一次性凭据不会放行；读缓存故障只回源到授权后的数据库查询。
4. 运行验证：内存/连接池上限、慢命令、命中率、拒绝数、连接错误和脱敏日志；不得记录 secret、token 或完整业务 payload。
5. 恢复验证：清空或恢复 Redis 后，PostgreSQL 中的 License、Key、授权、报警和业务状态不受影响；临时数据按设计重新生成或过期。

## 后续顺序

1. 不阻塞 M6d：继续用 PostgreSQL 完成当前开放平台验收和双审。
2. 单独做 Redis adapter spike，只覆盖微信 token 或非安全读缓存，不改生产 Compose。
3. 在有真实多实例需求后，再实现共享限流 port，并完成故障/重启/并发证据。
4. 最后再评估 nonce、worker lease 和读缓存的生产化；每项单独冻结契约、测试和回滚方案。

本记录不表示 Redis 已支持、已部署或已通过任何 Rxx 验收。
