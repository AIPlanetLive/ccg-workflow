> **Archive status:** 该 plan 的实现与验证已完成；此归档副本随文档 closure commit 生效，执行过程产物 `state.md` 与 `journal.md` 按协议保留在归档之外。
> **Current results:** 参见 [README — Installation](../../../README.md#installation)、[README.zh-CN — 安装](../../../README.zh-CN.md#安装) 与 [CHANGELOG — Unreleased](../../../CHANGELOG.md#unreleased)。

# codeagent-wrapper Ubuntu 源码构建与跨平台交付

> ⚠️ **Long-task mode** — 本 plan 处于长任务模式
> - 进度状态：`./state.md`
> - 决策日志：`./journal.md`
> - 协议详情：`~/.claude/references/long-task-protocol.md`
>
> 实施时（含 compact 之后）必须先读 state.md 和 journal.md 再决定下一步动作。
> 声称任务完成前必须实际跑本 plan 的 verify 步骤并贴出输出。

## 输入与目标

用户要求修改 `/Users/lindong/research/ccg-workflow/` 中的 `codeagent-wrapper`，使其可在 Ubuntu 主机（验收机为 `ssh dgx0023`）使用，并进一步明确权威交付链：`~/research/ccg-workflow/` 是唯一源码源；在 Mac 上从更新后的源码同时编译 macOS 与 Ubuntu binary；编译产物 check in 到 `~/research/ai-agent-config`，由该仓配置树在两类宿主上消费。

最终产物不是外部 release downloader，也不要求 Ubuntu 自行安装 Go/Node。使用者在安装/同步 `ai-agent-config` 后，继续调用稳定入口 `~/.claude/bin/codeagent-wrapper`；该入口在 Mac arm64 上执行 Darwin binary，在 `dgx0023` 的 Ubuntu x86_64 上执行 Linux binary，且非交互 SSH 的默认 `PATH` 不含 `~/.local/bin` 时仍能找到 Codex/Claude backend。

本计划是唯一实施入口，没有上游 spec。用户已确认 scope 为 wrapper 独立交付，rigor 为 `(A0,V1)`。

## 当前状态与基线事实

- `ccg-workflow` 当前为 `main@7f7b689`，工作树除本计划外 clean。`codeagent-wrapper/build-all.sh` 与 CI 已能交叉编译 `linux/amd64`、`linux/arm64`，发布 binary 为静态 ELF。
- 2026-07-29 在 `dgx0023` 实测：Ubuntu 22.04、`x86_64`、Go 1.18.1、无 Node 命令；Codex 0.145.0 与 Claude 2.1.220 位于 `/home/lindong/.local/bin`，非交互 SSH 默认 `PATH` 不含该目录。
- 同日把现有 Linux 发布 binary 放到 `dgx0023:/tmp`，显式补 `PATH="$HOME/.local/bin:$PATH"` 后真实 Codex 调用返回 `ubuntu-ok`。因此 Linux runtime 与 backend 协议成立，缺口是 backend executable discovery 与跨平台持久交付入口。
- Mac 本地 `ai-agent-config` 主 worktree 有大量用户既有 dirty changes，不能作为本任务写入面。`dgx0023` 上存在用户已完成且 clean 的 `71dba3e feat(install): support Linux hosts alongside macOS`；该 commit 已只读 fetch，并在 `/Users/lindong/research/ai-agent-config-codeagent-ubuntu` 建立独立分支 `codex/codeagent-wrapper-ubuntu`，作为本任务唯一 `ai-agent-config` 写入面。
- `ai-agent-config@71dba3e` 已声明 macOS 为参考平台、Ubuntu 已验证，并集中维护平台差异；它仍只跟踪 `claude/bin/codeagent-wrapper` 这一份 Mach-O arm64 binary。根 installer 会把整个 `claude/` 链接到 `~/.claude`，因此同一路径要跨 OS 必须由仓内稳定入口选择同目录 platform binary，不能安装时原地改写 tracked 文件。
- `ccg-workflow` 项目规则要求任何 Go 改动同步 bump `codeagent-wrapper/main.go` 与 `src/utils/installer.ts` 的 wrapper version。

## 范围与边界

### 包含

- `ccg-workflow` Go runtime：backend 裸命令不在当前 `PATH` 时，回退查找用户级 `~/.local/bin`；保持既有 cwd、参数、信号与错误语义。
- `ccg-workflow` 版本契约：同步 bump Go wrapper version 与 TypeScript `EXPECTED_BINARY_VERSION`，更新中英文 README/CHANGELOG 中与 Linux backend discovery 相关的用户说明。
- Mac 构建：从同一最终源码快照生成 Darwin arm64 与 Linux amd64 binary；两份 binary 的 `--version` 必须一致，Linux artifact 必须是静态 ELF。
- `ai-agent-config` 持久载体：扩展 `lib/install-platform.sh` 为 wrapper dispatcher 的 OS/architecture 唯一规范化权威，把现有稳定命令路径改为只消费规范化 token 的最小 shell dispatcher，并跟踪 Darwin arm64、Linux amd64 两份 binary；unsupported platform 明确失败。其他 owning component installer 的既有平台判断不在本任务范围。
- `ai-agent-config` 测试与文档：验证 dispatcher 选择/转发和 unsupported path；更新 README 平台表与 CHANGELOG。
- 真实验收：Mac 入口跑 Darwin binary；`dgx0023` 用本次 Linux binary、默认非交互 SSH `PATH` 完成真实 Codex 调用。
- 两个仓库分别经过 review gate 与 provenance-safe local commit；`ai-agent-config` commit 留在隔离分支，未经许可不整合回本地 `main`。最终 commit ids 只记录在 commit 外的交付报告，避免 Git 自引用。

### 不包含

- 不依赖或修改 `fengshao1227/ccg-workflow` 的可移动 `preset`，不把任何外部 release 当本次交付源。
- 不让 Ubuntu 构建源码，不安装/升级远端 Go 或 Node，不改 backend 凭据。
- 本轮只承诺当前 reference/target 组合：Darwin arm64 与 Linux amd64；Darwin amd64、Linux arm64、Windows 保留源码可构建能力，但不 check in 到 `ai-agent-config` dispatcher。
- 不覆盖 `dgx0023` 的正式 `~/.claude/bin`；真实验收使用任务专用临时目录与 state dir。
- 未经用户显式许可不整合 `ai-agent-config` 分支，不 push 任一仓库。`ccg-workflow` 的 Go 源码 push 与现有 CI 自动删除/重建 `preset` release、同步 R2 是一个不可拆分的外部动作，必须作为整体征权；本计划的 binary 交付不依赖它。

## Rigor 与执行边界

| 项目 | 结论 | 理由 |
| --- | --- | --- |
| R | light → A0 | 两仓改动均为可回滚本地 Git 变更；远端验收不覆盖正式安装。 |
| G | standard → V1 | wrapper/dispatcher 是反复调用的用户工具，回归会影响真实 agent 工作流。 |
| 默认向量 | `(A0,V1)`，label `standard` | 用户已确认。 |
| Phase override | 无 | 无不可逆外部动作或零容忍数据风险。 |

V1 要求每个改行为 unit 都有针对性验证并经单 reviewer。push、release、覆盖远端正式路径不在授权范围内。

并发隔离：开工检查确认 `ccg-workflow` 只有当前 `main` worktree，且没有 active-work 登记；本 plan 声明该仓由本 session 独占，源码单元直接在当前 worktree 实施。`ai-agent-config` 已使用独立 worktree 与任务分支隔离；若上述输入快照在执行中失效，立即停止相关写入并重新划分边界。

## 用户视角验收（L2）

以下均由 agent 独立完成，无需人工 gate：

1. 在任务专用 consumer 前缀下创建 `.claude -> ai-agent-config/claude`（与正式 installer 的链接拓扑同构），但不 export 或覆盖进程真实 `HOME`；从该临时前缀的 `.claude/bin/codeagent-wrapper --version` 调用稳定入口，Mac arm64 实际选择 Darwin arm64 artifact，输出与源码 version 一致。
2. 在 `dgx0023` 构造同样的临时 consumer 前缀 symlink 拓扑、保留真实 `HOME` 供 backend discovery/auth 使用，并从该显式路径调用稳定入口；不补 `PATH`，Linux amd64 实际选择 Linux amd64 artifact，真实 Codex 调用的标准输出去除 wrapper banner 后得到 `ubuntu-ok`。
3. dispatcher 对未交付的 OS/architecture 组合非零退出，stderr 明确列出检测到的平台与支持组合；参数与退出码原样转发给 platform binary。
4. 从最终 `ccg-workflow` source commit 生成的两份 checked-in binary 在上述真实 symlink consumer path 上均可用。

内部验证另行覆盖：Codex、Claude、Gemini 三个 backend 共用 resolver；`PATH` hit/miss、不可执行 candidate、logical backend cwd；两份 binary 的 source commit、version、file type/linkage；Go/TypeScript version invariant；Go 全量与 race tests；dispatcher 与 hook tests。

## 设计与实施步骤（L3）

### 工作单元边界

执行分为三个有序单元：① `ccg-workflow` 源码/测试/用户文档形成 clean source commit；② 只从该 commit 构建两份 binary，并与 `ai-agent-config` 的集中平台 token、dispatcher、测试、文档一起形成交付 commit；③ 全部验收后归档 plan，形成 docs-only closure commit。最终 commit ids 只在 commit 外的交付报告汇总。

### 1. 工具链 preflight 与 RED

- 按用户 2026-07-29 的执行决定，不依赖本机 OrbStack：通过 `ssh dgx0023` 的 Docker daemon 使用固定 Go 1.21+ container toolchain跑 Go RED/GREEN；源码从 Mac working tree同步到任务专用远端 mirror，Mac working tree仍是唯一源码源。
- 在 Mac 准备任务隔离、按官方 SHA-256 校验的当前稳定 standalone Go toolchain，仅用于从最终 clean source commit交叉编译两平台 binary，不改系统 Go。当前固定为 Go 1.26.5；旧 Go 1.21.13 在当前 macOS 生成的 Mach-O 缺 `LC_UUID`、会被 dyld拒绝，不能作为 Mac artifact builder。远端 Docker或 Mac 隔离 build toolchain任一路不可用且无法修复时走 Stop Gate。
- 安装 `ccg-workflow` 的 pnpm dependencies（当前无 `node_modules`），先跑既有 targeted test 确认 setup GREEN。
- 在 `codeagent-wrapper` 新增 command resolution 测试：`PATH` hit、`PATH` miss + 三个 backend 在 `~/.local/bin` 可执行、candidate 不可执行、全部 miss，以及绝对 command path 不改变 Codex cwd 分支。新增跨 Go/TypeScript version 一致性测试，并同步更新现有硬编码 `5.9.0` 的 Go 断言。先运行相关 target，保留由目标缺口造成的 RED。
- 在 `ai-agent-config` worktree 新增 platform token + dispatcher test contract，fake `uname` 只进入 `lib/install-platform.sh`，验证规范化 Darwin arm64/Linux amd64 token、argv/exit forwarding、临时 consumer 前缀下的真实 symlink 拓扑与 unsupported combination；在替换入口前证明当前单 Mach-O 结构不满足 Linux case。

内部 verify：RED 必须由缺失行为造成，不接受缺依赖、语法错误或 test setup failure。

### 2. 实现 `ccg-workflow` backend discovery

- 增加单一、最窄的 executable resolver：先走 `exec.LookPath`；仅 miss 时检查 `os.UserHomeDir()/.local/bin/<backend>` 是否为非目录且任一 execute bit 已设置；仍 miss 则返回原命令。
- 在创建 command runner 前解析 executable；working-directory 判断使用 logical backend name 而非解析后的绝对路径。
- 同步 bump `codeagent-wrapper/main.go` 与 `src/utils/installer.ts` 的版本，保持项目版本不变量。

内部 verify：新增 targeted tests RED→GREEN；Go/TypeScript version invariant test通过；`go test ./...` 与 race test通过；不存在宽泛 HOME 扫描或 silent fallback。

### 3. 先冻结并提交 `ccg-workflow` source unit

- 同步 `ccg-workflow` 的 README/README.zh-CN/CHANGELOG 后，对 source unit 运行 V1 标准单 reviewer gate；finding 修复后由原 reviewer定向复核。
- 任何 reviewer remediation 都使之前的测试证据失效；重跑受影响 tests，直到最终 source unit clean。
- 按 `create-commit` stage 仅 source/tests/version/docs owned paths并创建 clean source commit。构建输入权威为该 commit，不用 dirty diff；记录其 tree id。

内部 verify：README 与 README.zh-CN 对 Linux backend discovery 的用户语义一致；commit 后工作树除 active plan assets 外无 source unit 变化；从该 commit checkout 运行 targeted/full tests 仍 GREEN。

### 4. 从 source commit 编译两平台 binary

- 在 Mac 用同一任务隔离 standalone Go invocation、`CGO_ENABLED=0` 和一致 ldflags，从已提交的 source commit 分别构建 `GOOS=darwin GOARCH=arm64`、`GOOS=linux GOARCH=amd64`。
- 构建产物直接落到 `ai-agent-config` 隔离 worktree 的 platform-suffixed tracked paths；不把唯一交付副本放系统临时目录。
- 对两份文件运行 `file`、`--version` 与 SHA-256，记录 source commit/tree；发现 version/architecture/linkage 不一致即失败。

内部 verify：build command 与 source commit可复现；binary 更新发生在计划声明的两个 owned path。此后 source commit 不再修改；若必须修源码，废弃两份 artifact、回到第 3 步形成新 source commit并重建。

### 5. 实现 `ai-agent-config` 集中平台 token 与稳定 dispatcher

- 把现有 tracked Mach-O 从稳定入口迁移为 `codeagent-wrapper-darwin-arm64`，再由本次最终构建覆盖更新；新增 `codeagent-wrapper-linux-amd64`。
- 扩展 `lib/install-platform.sh`，集中规范化 wrapper dispatcher 消费的 `INSTALL_OS` 与 `INSTALL_ARCH`；dispatcher 自身不得调用 `uname` 或形成第二套平台映射。其他 owning component installer 的既有平台判断保持不变。
- 用 `apply_patch` 新建 mode `0755` 的文本 dispatcher `claude/bin/codeagent-wrapper`：解析自身真实目录并 source repo 的 `lib/install-platform.sh`，只消费规范化 token选择相邻 artifact，用 `exec` 转发全部参数；unsupported 组合明确报错。
- 不在 dispatcher 中改 PATH；backend discovery 属 Go wrapper 权威语义，避免 shell 与 Go 两份行为漂移。

内部 verify：shell syntax、dispatcher unit tests、Mac `--version` 实跑；`codeagent-stdin-guard` 对稳定入口的识别不回归。

### 6. 同步 `ai-agent-config` 用户文档

- `ai-agent-config`：基于 `71dba3e` 已有 Linux 平台文档，在 README 平台表加入 codeagent-wrapper 的双 artifact/稳定入口行为；CHANGELOG 记录 Ubuntu wrapper 可用。无需改 architecture/UX contract。

内部 verify：只改目标自然段，每个自然段一行；文档准确描述稳定入口与双 artifact 行为。

### 7. 本地与真实 Ubuntu 验收

- 在 Mac 的任务专用 consumer 前缀下创建 `.claude` symlink 到 `ai-agent-config` worktree 的 `claude/`，保留真实 `HOME`，通过该显式 symlink path 运行 dispatcher 与完整相关测试。
- 将 `ai-agent-config` worktree 中 dispatcher、`lib/install-platform.sh` 与 Linux binary复制到 `dgx0023` 的任务专用 repo mirror；在任务专用 consumer 前缀下构造 `.claude` symlink，保留真实 `HOME` 供 backend discovery/auth 使用，并设置 `CODEAGENT_STATE_DIR` 到同一临时范围，避免失败 terminal record写入正式 HOME。
- 保持远端默认非交互 SSH `PATH`，通过 symlink stable entry运行真实 Codex task并校验业务输出；另用 fake backend覆盖 Claude/Gemini resolver，不消耗不必要的真实模型调用。
- 验收后核对远端正式 `~/.claude/bin` 未变化；任务临时资产可由系统自然回收，不执行 `rm`。

内部 verify：记录 OS/arch、dispatcher选择、binary `file`/`--version`、命令退出码与 `ubuntu-ok`；不读取或回显凭据。

### 8. `ai-agent-config` gate、commit 与文档收尾

- 对 `ai-agent-config` platform/dispatcher/binaries/tests/docs diff执行 V1 标准单 reviewer gate；finding 修复若改变 dispatcher、platform token或binary，失效对应证据并重跑 build/digest/dispatcher/真实 Ubuntu E2E，再由原 reviewer复核。
- 按 `create-commit` stage 仅本任务 owned paths并创建 `ai-agent-config` local commit；不得卷入 Mac 主 worktree既有 dirty changes。
- 完成两仓验收后，按 docs organization/sync-docs recipe 初始化或更新 `ccg-workflow/docs/CLAUDE.md` 索引，把 plan 归档到 `docs/plans/20260729-codeagent-wrapper-ubuntu/plan.md`；归档导航头只指向稳定 README/CHANGELOG 结果入口，不写 commit ids。归档副本验证并完成 closure commit 后，移除活动目录中重复的 `plan.md` 与 `baseline.patch`；按 long-task protocol 保留 `state.md`/`journal.md` 作为执行记录，并在其头部指向归档 plan。该 docs-only diff单独过对应 review gate并形成 closure commit。
- `ai-agent-config` commit 保留在 `codex/codeagent-wrapper-ubuntu`，汇报相对 `main@8c2436e` 的前置用户 commit `71dba3e` 与本任务 commit，说明能否 fast-forward；征得显式许可后才整合到本地 main。
- `ai-agent-config` push单独征得许可。`ccg-workflow` push的征权 packet必须明确它会自动重建 GitHub `preset` release并同步 R2；用户授权的是这一整体动作。

## 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| 两份 binary 不是同一源码 | 先形成 clean source commit；同一构建 invocation只从该 commit构建并比较 embedded version；最终交付报告记录 provenance。 |
| shell dispatcher 改变信号/退出码 | 最后一跳必须 `exec`；测试 argv、退出码与 signal-transparent 结构，不增加中间常驻进程。 |
| absolute backend path 改变 Codex cwd | cwd 分支改用 logical backend name并加 regression test。 |
| 远端验收污染正式 state | binary、workdir 与 `CODEAGENT_STATE_DIR` 都指向任务临时目录；验收后只读核对正式路径未变化。 |
| `ai-agent-config` 本地主 worktree dirty且落后远端用户 commit | 只在基于 `71dba3e` 的独立 worktree写入；不触碰或整合 main，直到用户授权。 |
| wrapper dispatcher 出现第二套平台判断 | dispatcher 不调用 `uname`，只消费 `lib/install-platform.sh` 提供的 `INSTALL_OS`/`INSTALL_ARCH`；其他 owning component installer 不在本任务范围。 |
| review 修复使已构建 artifact 过期 | source unit先 review+commit再 build；后续输入变化显式废弃 artifacts与跨平台证据并回到构建步骤。 |
| Go 测试或 Mac 构建工具链不可用 | Go tests固定走 `dgx0023` 的 Go 1.21.13 Docker，以覆盖 module 下界；最终 build固定走 Mac 任务隔离 Go 1.26.5，以满足当前 dyld。任一路失败先修根因，两路证据不得互相冒充。 |

## 用户决策 gate

实施阶段全自动执行。只有以下动作暂停征权：整合 `ai-agent-config` 分支回本地 main、push `ai-agent-config`、push `ccg-workflow` 并触发自动 preset/R2 发布这一整体动作、覆盖 `dgx0023` 正式 wrapper，或发现必须新增本计划未声明的平台 artifact。

## Defaulted Decisions

| 决策 | 默认 | 理由 |
| --- | --- | --- |
| 输出策略 | 单 plan | 用户已明确唯一源码与交付链。 |
| check-in 矩阵 | Darwin arm64 + Linux amd64 | 精确覆盖当前 Mac 与 `dgx0023`；不为未验证宿主增加 binary 负担。 |
| 稳定入口 | 中央 platform token + shell dispatcher + platform-suffixed binaries | 整个 `claude/` 被链接到 HOME；dispatcher复用既有平台权威且不改写 tracked 文件。 |
| backend fallback | 仅 `~/.local/bin` | 直接覆盖 Ubuntu 的标准用户安装位置，避免不可控的全 HOME 扫描。 |
| 工作单元 | 两仓强耦合单元 | source、binary 与 dispatcher 任一单独落地都会形成不可用中间态。 |

## 关键引用

| 路径 | 用途 |
| --- | --- |
| `/Users/lindong/research/ccg-workflow/codeagent-wrapper/backend.go` | backend command 契约 |
| `/Users/lindong/research/ccg-workflow/codeagent-wrapper/executor.go` | command runner 与 cwd 行为 |
| `/Users/lindong/research/ccg-workflow/codeagent-wrapper/main.go` | embedded wrapper version |
| `/Users/lindong/research/ccg-workflow/src/utils/installer.ts` | expected wrapper version契约 |
| `/Users/lindong/research/ccg-workflow/codeagent-wrapper/build-all.sh` | 既有跨平台构建矩阵 |
| `/Users/lindong/research/ai-agent-config-codeagent-ubuntu/claude/bin/codeagent-wrapper` | 当前 Mac-only binary，将变为稳定 dispatcher |
| `/Users/lindong/research/ai-agent-config-codeagent-ubuntu/README.md` | 跨平台用户入口 |
| `/Users/lindong/research/ai-agent-config-codeagent-ubuntu/CHANGELOG.md` | 用户可感知变更记录 |
