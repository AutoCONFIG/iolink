# 协作规范(单仓)

> 2026-09-16 起:原跨仓(模块仓+contracts 版本依赖)流程已废止,代码全部在主仓。

## 分支与 MR

- `main` 为受保护主干;开发走 feature 分支(`feat/xxx`、`fix/xxx`)→ MR → 负责人 review → 合并
- 提交信息:动词开头,英文或中文均可,如 `feat(adminapi): pond CRUD`
- 接口变更(/api/v1、/admin/v1)= 契约变更:先改 docs/ 下对应文档,随同一 MR 提交

## 代码所有权(按 package,评审归属)

| Package | Owner |
|---|---|
| internal/access、cmd/mqtt-sim | 设备接入线 |
| internal/core、internal/adminapi | 主程 |
| internal/appapi + 小程序 | 小程序线 |
| web/admin | 管理前端线 |

## 开发环境(一次性)

```bash
git clone https://git.hyhy.fun/rsplab/iolink.git
# 推送凭据:首次 push 输入一次账号密码/token(git credential store 自动保存)
# 跑起来:
make dev
```

## 测试纪律

- internal/* 每个包必须有测试;appapi 类接口测试用 fake(参考 internal/appapi/server_test.go),不连数据库
- MR 前本地:`make verify`(build+vet+test)
