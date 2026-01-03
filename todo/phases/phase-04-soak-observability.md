# Phase 04：长跑（Soak Test）+ 可观测与自愈

目标：在你电脑上跑 24h/72h，系统稳定、性能可控、失败可降级、自愈可恢复。

## 你需要确认（Before we start）

1) 你希望的“长期运行形态”：
   - 电脑常开？
   - 每天固定时段运行？
2) 你希望的告警方式（可选）：Telegram / 本地通知 / 仅日志

## 任务清单（Tasks）

### P04-1：Soak 指标采集
- 每个 cycle：行情耗时、AI 耗时、执行耗时、DB 耗时、错误率
- 进程级：CPU、内存、goroutine（pprof + 轻量日志）

### P04-2：异常降级策略
- 行情失败连续 N 次：回退到缓存/减少币池/进入 wait
- AI 失败连续 N 次：进入 wait + 延长周期

### P04-3：备份与恢复
- `data/data.db` 定期备份
- 提供一键恢复流程（替换 DB + 重启）

## 验收方式（Verification）

- 24h 运行后：
  - 没有明显内存泄漏（RSS 稳定或可解释增长）
  - DB 文件增长可控（或有归档策略）
  - 服务自动重启后能恢复状态（trader 配置、历史曲线可见）

## 当前实现（What’s landed）

### 1) Cycle 指标采集（P04-1）

- 后端每个 cycle 会在 `decision_records.execution_log` 追加一条结构化指标：
  - 前缀：`METRIC:`
  - 内容：JSON（build_context_ms / ai_decision_ms / execute_ms / total_ms / candidate_coins / market_data_ok / failures 等）
- 前端 `web/src/components/DecisionCard.tsx` 会把 `METRIC:` 解析成 “Cycle Metrics” 卡片展示。

### 2) 异常降级（P04-2）

- 当连续 3 次出现“市场数据不可用”（`decision.ErrMarketDataUnavailable`）：
  - 暂停 10 分钟
  - 启用 2 小时降级币池：仅保留 BTC/ETH（并始终保留当前持仓的币种，确保风控/平仓可工作）
- 当连续 3 次 AI 失败：
  - 暂停 15 分钟

### 3) 备份与恢复（P04-3）

新增 `start.sh` 命令：
- `./start.sh backup-db`：把 `data/data.db` 备份到 `data/backups/`
- `./start.sh restore-db <backup_file>`：停止服务 → 先备份当前 DB → 用备份覆盖 → 重启服务

### 4) 进程级观测（pprof）

后端已默认启用 pprof（端口 `:6060`），可用于长期运行时定位 CPU/内存热点。
