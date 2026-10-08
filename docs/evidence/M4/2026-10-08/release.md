# M4 接口演示发布收据

日期：2026-10-08 Asia/Shanghai。

发布快照：`036523d3078ff54f4f83f3256e7d4f0b9c9e066b`；源码快照：`2b66812fd0d6e64609c1bc3fccbccccf71a4f99d`。

两位未参与编辑的审阅者对最终发布快照只读复核并明确批准，回复保存在本会话：

- `/root/m4_code_last`：APPROVE / WATCH，blockers=[]；确认业务源码/测试/dist 不变，13/20、外部阻塞、日志命令及 tag-only CI 一致。
- `/root/m4_gate_last`：APPROVE，blockers=[]；确认同一发布快照及上述范围；复跑架构与契约检查通过。

源码审阅完整报告见 [代码审阅](reviews/code-last.md)和[门审](reviews/gate-last.md)。归档原件的空格及末尾空行是 NOTE，不宣称整个提交差异 `--check` 通过。

实际发布命令：`git push --atomic github main refs/tags/v0.0.11`，exit 0。输出确认 `main d9844c3..036523d` 和新 tag `v0.0.11`。

`git ls-remote --heads --tags github main v0.0.11 'v0.0.11^{}'`，exit 0，确认：

```text
036523d3078ff54f4f83f3256e7d4f0b9c9e066b refs/heads/main
70e486714a042ca5cb03948263d2d3450353df19 refs/tags/v0.0.11
036523d3078ff54f4f83f3256e7d4f0b9c9e066b refs/tags/v0.0.11^{}
```

通过 GitHub REST API 只读查询 `/repos/AutoCONFIG/iolink/actions/runs/37779362699`：head_branch=`v0.0.11`，head_sha=`036523d…`，event=`push`，最终 status=`completed`，conclusion=`success`。[CI 运行](https://github.com/AutoCONFIG/iolink/actions/runs/37779362699)。镜像构建与推送 job `build-image` 成功；`build` / `integration` 按现有 job 条件 skipped，这不算远端测试通过。本地测试证据仍见 M4 README。

同次发布的 Actions runs 查询仅出现新 tag 运行，没有 `036523d` 的 main 运行；CI 配置只有 push.tags=["*"]。本收据后续提交到 main 仅记录事实，不改变发布 tag 指向。

R25–R27 真实微信、订阅及真机仍为 `external_blocked`。当前只核验 GHCR 发布 job 成功，不声称测试服务器已拉取此镜像或已启动成功。
