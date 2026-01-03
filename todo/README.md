# NOFX 个人版（本地长期运行 + 自动模拟盘）改造计划

本目录用于把当前 NOFX 改造成「一个人本地长期运行、自动模拟盘（paper trading）、可回测/可复盘、后续可平滑接入实盘（Binance/Aster）」的版本，同时尽量保持对上游仓库持续 `git pull` 的兼容性。

## 你当前已确认的关键前提

- 不改成 Python（保留 Go/React 主体，便于跟进上游更新）。
- 先做 **paper 合约模拟**（支持 long/short/leverage），现货模拟后置。
- 成交假设优先用 `next_open`（更接近严谨回测）。
- `next_open` 在本计划里也可简称 OPN（Next Bar Open），并要求“等下一根K线出现后再撮合”，避免未来函数。
- 候选币种用 **Binance TOP N**（更可复现），Docker 部署为主。

## 文件结构

- `todo/ROADMAP.md`：总览、里程碑、阶段拆分与依赖关系。
- `todo/PROGRESS.md`：可打勾的进度表（每完成一项在这里标记）。
- `todo/phases/`：每个阶段的详细任务、你需要确认的事项、验收方式。

## 使用方式（建议）

1) 先读 `todo/ROADMAP.md`，确认阶段拆分和优先级是否符合你的预期。  
2) 在开始做某个阶段前，先在对应 `todo/phases/phase-*.md` 里把“你需要确认的点”确认掉。  
3) 每完成一个子任务，在 `todo/PROGRESS.md` 中勾选并记录日期/备注。  
