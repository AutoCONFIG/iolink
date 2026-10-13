# 既有APIKey/Open、restore、初始化与摄像头核验

基线v0.0.38后 `3635fd6`；2026-10-14。范围为APIKey5、签名Open读取5、设备restore1、初始化2、摄像头7，共20个不同物理操作。restore在admin/license两份契约重复声明，分别校验但只计一个行为。仅核验现有实现，不引入新的GB/media/playback运行时，不启动用户暂缓的License交付专项或浏览器验收。

## 过程证据

- setup/input-before.log：真实PG/TCP重复或错大小写字段实际返回204并创建管理员；补严格解析后拒绝400且零身份写入。
- setup/utf8-before.log：真实无效UTF8输入实际创建管理员，补边界UTF8检查后再回归。shared-edit-build.log是并行编辑时camera fixture暂态未闭合导致的构建失败，不计为产品缺陷或通过证据。
- 初始化正常首启仅一个ADMIN、无业务租户/成员，成功204、之后status.required=false，重复初始化409。输入、来源、安装密钥、媒体类型错误和超限拒绝；真实users/audit故障503且无管理员残留。观测logger dump验证服务器请求ID关联、SQLSTATE安全类别、不包含安装密钥/口令/用户明文或SQL关系名。
- setup/final.log：Go race/shuffle核心3专项及既有bootstrap回归通过；core19.132s、setupapi6.069s，终端exit0。保留初始化并发/审计回滚旧断言。
- camera/：真实PG/TCP当前7操作，101次实际响应video OAS检查、5顶层/53子测试、无SKIP，core39.915s终端exit0。读/停用/角色/归属/无身份或到期/严格查询输入/配置不可用无写入；6真实SQL故障的归一化安全错误类别和服务器requestID，停用末端故障回滚。首次diagnostic-before仅缺关联诊断，补两HTTP边界helper后通过；原始SQLSTATE由生产PG QueryTracer另行记录，没有为跨层细节改provider错误语义。adminapi/appapi race/shuffle回归通过。受控本地License只到达原有video功能守卫；Availability保持nil，POST/PUT实际503而非成功。
- key-open/：本批11操作和旧非浏览器M6d14顶层回归，真实PG/race/shuffle终端exit0、core83.409s，无SKIP；admin/open两个HTTP包回归及96/621合成检查通过。先复现非法字段写入、错误机器码、真实SQL401和未声明500/503；修复后输入零写入、真实SQLSTATE/请求关联、vault或缺少管理依赖503无虚构err、签名/nonce/重放/429/资源隔离、rotate/revoke和末端audit回滚通过。restore影子删除失败在device更新之后仍完全回滚并返回实际500，两份schema同时检查。早期restore-before的audit注入没有触达该业务路径，是fixture假设错误；restore-shadow/actual-status才证明末端存储故障，不把该假设当产品缺陷。
- 初始候选 `4a59e953f9ad7a336babe45d90d28258fdcc072a` 的旧snapshot日志与两审前核验单列：自查及独立审阅发现nil camera capability诊断带人为err，新增真实两端capability-before先失败。修复为常量能力诊断且无err；capability-after重跑camera全6顶层/两端真实503及原正反路径通过。SQL故障断言检查非空安全类别和非空请求ID，不再绑定Go运行时具体类名。旧候选的通过回归不计为新候选最终通过。

## 环境与边界

专属容器 `iolink-http-contract-pg-20261013`、网络 `iolink-http-contract-20261013`、卷 `iolink-http-contract-data-20261013`，均label `iolink.task=http-contract-20261013`，PG16.15/Timescale2.30.2，loopback33127。测试各自创建和删除数据库。真实响应仅经stdin传入固定本地六份OpenAPI验证器，不归档body或将敏感响应写入argv。

命令使用 `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated-DSN> IOLINK_CONTRACT_PYTHON=<main-repo>/.venv/contracts/bin/python go test -race -shuffle=on -count=1` 配合各日志选择器。

## 最终闭环

实现提交 `95025f0` / tree `bbfb29c41c31549899655750e171ef2c25a6d0eb`；两位未参与编辑者独立确认1,437 tracked blobs匹配冻结导出，并明确APPROVE/WATCH：[代码审阅](reviews/final/code-review.md)、[门审](reviews/final/gate-review.md)。首轮4a59拒绝收据保留在reviews/initial，不计为通过。AGENTS、playback及后续APIKey名称计数修复没有进入本候选。

精确源码完整build/vet/race：session56771终端exit0、33测试包/core471.529s；本批20顶层专项无SKIP、core264.801s；Python114、静态96/621与ruff检查通过。命令、旧并行超时失败和最终独立完整成功均见[回归记录](verification-runs.md)，不延长默认超时或隐藏失败。仅关闭本批命名矩阵，不代表穷尽全部输入/故障组合、R02或外部验收。发布v0.0.39，主软件范围计数不增加。

当前契约操作范围：原35、本批前23、本批20，共78个不同目标操作；restore两份schema重复校验，计为79个spec-operation。相对于96个契约操作，2个License操作按用户安排留到专项、15个GB/media/playback操作未挂载或未发布另列。摄像头2个配置写操作核验的是当前503拒绝，未宣称创建/替换成功或真实视频可用。

专属容器/卷/网络在紧接着的APIKey名称边界核验后已统一核对归属并清理：[清理收据](../2026-10-14-api-key-name/cleanup.log)。后续真实128字符名称边界独立发布v0.0.40，不把本候选未测试的边界写成v0.0.39已通过。

摄像头生产装配缺少Availability，配置写入只可证明503/fail closed/no-write；受控本地签名License仅为到达已实现接口的依赖，不是交付License专项通过。真实视频/微信/设备、浏览器、长期容量及大恢复继续保留原有外部限制。
