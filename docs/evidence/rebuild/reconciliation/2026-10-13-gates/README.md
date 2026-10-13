# V1b / D14 只读阶段门

日期：2026-10-13，基线 `de499f4`。范围是已有验收工具修复，不是重新验收业务代码。候选及独立双审见后续收据；双审前不计完成。

## 行为

`check_architecture_manifests.py --gate` 只输出评估JSON，不创建或改写历史gate文件。整体门仍要求所有父需求的软件状态passed；已有收据错误、阻塞或仅覆盖子项时拒绝整体通过。保留历史整体收据的两种证据形式：逐需求路径字典，或仓库内非空证据包文件路径（M6d）；子项收据仍要求逐需求字典及完整绑定。

M6b由acceptance-map的结构化 `gate_parts` 明确评估R36.a/R37.a。必须有complete收据、匹配的source/web验证记录、全部成功且非空的本地命令产物、两份不同文件的显式APPROVE及同candidate字段。缺失、畸形、失败、错配、拒绝、重复文件别名及越界路径均失败。原M6b候选/source/web与审阅字段保留，只补记录已批准的 `.a` 范围。

这证明本地记录一致性；不能证明报告真实作者、密码学来源或当前运行时验收。历史缺少候选绑定的TODO3/M5不因本修复补成通过；R36/R37整体与未来 `.b` 保持not_run。

## 已执行证据

环境：Linux；专用Python环境 `.venv/contracts/bin/python`（3.14.6）、Pydantic2.13.4、pytest9.1.1，Ruff0.15.1。本轮不改变Go运行时/数据库/前端，未创建容器；浏览器由用户验收。

- `before-cli.log`：旧实现运行新增收据保护断言，3 failed / 28 deselected，证明成功路径创建及覆盖收据。
- `gate-tests.log`：初轮73门禁/清单回归通过。
- `all-python-tests.log`：最终 `python -m pytest -q scripts`，107通过；包括profile接线、缺失收据CLI、整体门与子项边界、拒绝和错配、实际临时文件及别名、收据字节不变及原M6d证据包收据CLI回归。
- `native-cli.log`：实际仓库 `--all` 与 `--gate M5/M6a/M6b/M6c/M6d` exit0；JSON明确 `.a` 和历史candidate/source/web；前后全部gate JSON与acceptance-map SHA256相同。
- `contracts.log`：`make verify-contracts` exit0，96 target operations / 568 synthetic fixtures。不是实际业务handler验收。
- `typecheck.log`：`uvx basedpyright --pythonpath .venv/contracts/bin/python --level error scripts/architecture_gate.py` exit0；首次检查发现Pydantic字段覆盖类型问题，已用共用基类消除。
- `ruff.log`、`no-excuse.log`：新模块及其两测试文件的基础Ruff规则和编程技能静态规则通过；不声称全库Ruff ALL或旧无类型checker全量类型检查通过。

初候选tree `48fba0f` 被两审发现M6d字符串证据包解析回归并拒绝；已补格式支持及真实CLI回归，最终冻结快照复跑、审阅及发布信息随后归档。V1仍含历史绑定和其他已有功能核验，外部微信、硬件、License交付、TLS/容量/恢复保留external_blocked，M7a/M8新功能按用户范围暂缓。
