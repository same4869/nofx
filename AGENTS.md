# AGENTS.md（本仓库的 AI 协作规则）

本文件用于让新的 AI/协作者快速对齐“个人版 NOFX”的目标、边界与迭代规则。作用域：整个仓库。

## 必读（接手顺序）

1) `WIKI/AI-HANDOFF.md`：我们改了什么、为什么改、下一步怎么跑/怎么验收  
2) `WIKI/DEV-RULES.md`：迭代规则（如何改、改哪里、如何保持可 `git pull`）  
3) `WIKI/TODO-RULES.md`：`todo/` 的维护规则与进度更新方式  

## 项目目标（不要偏离）

- 单人本地长期运行（Docker），默认安全（尽量仅本机访问）。
- 先模拟盘（paper）跑通全链路：策略→数据→AI→执行→落库→看板/复盘。
- 未来可平滑接入实盘（Binance/Aster），但不要让 paper 触发真实下单。

## 关键语义（不可破坏）

- OPN / `next_open`：成交价=下一根 primary bar 的 open，且必须等该 bar 实际出现后撮合（避免未来函数）。
- 硬风控必须“后端代码强制执行”，不依赖 prompt。
- 可解释性与观测：
  - `decision_records.execution_log` 中 `RISK:` 为风控快照 JSON
  - `decision_records.execution_log` 中 `METRIC:` 为 cycle 指标 JSON

## 迭代原则（最重要）

- **最小侵入以便跟上游**：优先新增文件/新增配置项/新增分支逻辑，避免大改核心流程。
- **差异收敛到执行器**：paper vs live 的差异尽量收敛在 `Trader` 实现，不要分叉策略/风控语义。
- **改动要可验收**：新增功能必须给出可重复的验收方式（命令/步骤/预期现象）。

## 文档与进度维护（必须同步）

- “长期有效知识/规则”写进 `WIKI/`。
- “阶段任务/进度打勾/验收步骤”写进 `todo/`：
  - 每完成一个子任务：更新 `todo/PROGRESS.md` + 对应 `todo/phases/phase-*.md` 的 “What’s landed”。

## 本地验证（优先用 Docker）

- 构建/启动：`./start.sh start --build`
- 健康检查：`curl -fsS http://localhost:8080/api/health`
- 需要格式化 Go 代码但本机无 go 工具链时：可用 `docker run golang:1.22 gofmt ...`

