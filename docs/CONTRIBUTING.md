# 协作与兼容规范(给模块负责人)

> 主仓(iolink)= contracts 契约 + core + 组装;子仓 access/appapi = 模块。
> **核心问题:模块负责人怎么和主项目保持兼容?—— 通过 contracts 的版本化,而不是通过"随时同步主仓代码"。**

## 1. 兼容机制:契约版本化

负责人**不需要关心主仓的日常演进**(core 重构、部署调整都与你无关)。你唯一的兼容义务:

> **你的模块永远只 import `git.hyhy.fun/rsplab/iolink/contracts`,并始终针对某个已发布的 tag 开发(当前 `v0.0.1`)。**

- contracts 只增不改:新增接口/字段 = 兼容;删除/改签名 = 破坏性,见 §3
- 你本地 `go get git.hyhy.fun/rsplab/iolink/contracts@<tag>` 固定依赖,测试全用 fake(参考 `appapi/server_test.go`)
- 所以:主仓哪怕天天改,你的模块照常编译、测试、发布——**兼容是编译期 + 契约评审保证的,不是靠人肉对齐**

## 2. 环境准备(一次性)

```bash
# go get 私有契约需要两步:
go env -w GOPRIVATE=git.hyhy.fun/*
# ~/.netrc(600权限):
#   machine git.hyhy.fun
#   login <你的GitLab用户名>
#   password <你的token或密码>
```

## 3. 需要改契约时(唯一需要主仓配合的场景)

1. 在主仓对 `contracts/` 提 MR,说明为什么现有接口不够
2. 主仓负责人评审、合并、打新 tag(`contracts/v0.0.2`,破坏性变更升 `v0.1.0` 并在 CHANGELOG 说明迁移)
3. 你在自己仓 `go get ...@v0.0.2`,适配,发布模块新 tag
4. 主仓 `git submodule update --remote` + 集成测试通过后升级 submodule 指针

**禁止**:在模块仓里自己 fork contracts、绕过接口直连数据库/对方模块——CI 的 `make verify` 和 MR 评审会拦。

## 4. 日常节奏

| 谁 | 频率 | 动作 |
|---|---|---|
| 模块负责人 | 每次发版 | 自己仓:MR → merge → 打 tag;在主仓开一个"升级指针"的轻量 MR |
| 主仓负责人 | 集成时 | `git submodule update --remote`,跑集成测试,合入指针升级 |
| 全员 | 契约变更时 | 走 §3 流程 |

## 5. CI 保障

- 模块仓 CI:只测自己 + 契约(版本依赖),与主仓其他部分零耦合
- 主仓 CI:`make verify` 检查 submodule 指针干净且指向已打 tag 的提交,防止集成漂移
