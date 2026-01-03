# 迭代规则（Dev Rules）

目标：让后续改动既能满足“个人系统”，又尽量不阻碍 `git pull` 上游更新。

## 总原则

1) 最小侵入：优先新增文件/新增配置字段/新增分支，不大改核心流程。  
2) 差异收敛：paper vs live 的差异尽量收敛在 `Trader` 实现。  
3) 可复盘优先：任何“自动化行为”要能落库并解释（RISK/METRIC/close_reason）。  
4) 失败可降级：行情/AI/外部依赖失败要有退路（缓存、缩小币池、进入 wait）。  

## 代码组织约定（新增功能放哪里）

- `trader/`：执行器与交易生命周期（paper/live、撮合、风控闸门、指标采集）。  
- `decision/`：策略引擎、币池来源、prompt 构建、AI 输出解析。  
- `store/`：数据结构与表结构（尽量向后兼容、迁移用 ALTER）。  
- `market/`：行情/K线拉取与缓存（尽量稳定、可复现）。  
- `web/src/`：看板/配置 UI（避免把业务逻辑塞进 UI）。  

## 日志/可解释性约定

- `decision_records.execution_log`：
  - `RISK:` 前缀：账户级风控快照 JSON
  - `METRIC:` 前缀：cycle 指标 JSON
  - 其他字符串：人类可读日志

UI 解析规则：
- 默认过滤 `RISK:`/`METRIC:` 的 raw log，避免刷屏；可切换 raw 查看全部。

## 风控约定

- 所有“硬风控”必须由后端执行，不依赖提示词。  
- 触发熔断/冷静期必须可重启恢复（用 `system_config` 持久化状态）。  
- 开仓保护（SL/TP）若要求强制成功：失败需回滚（best-effort）。  

## 与上游同步（推荐流程）

1) 先 `git pull origin main`（或你实际分支）  
2) 解决冲突时优先保留：
   - 新增文件（paper、risk_*、topn 等）
   - 小范围改动点（compose/start.sh/UI 小组件）
3) 如冲突触及核心文件（`trader/auto_trader.go`、`decision/engine.go`）：
   - 优先保证接口/语义不变（OPN、RISK/METRIC、硬风控闸门）
   - 再考虑合并上游新功能

