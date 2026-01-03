# Phase 02：Universe（Binance TOP N）+ 数据一致性收敛

目标：让“模拟盘/回测/复盘”更严谨、更可复现：币池来源、K线来源、时间框架与指标假设尽量统一，避免依赖外部不稳定服务。

## 你需要确认（Before we start）

1) TOP N 的 `N`：已确认 30  
2) TOP N 口径：建议取 **Binance USDT 永续 24h 成交额 TopN**  
3) 过滤规则：
   - 是否强制包含 BTC/ETH（推荐：是）
   - 是否过滤极小币种/刚上线币（建议：是）

## 口径与“滑动”建议（你这次已授权我来定）

### TOPN 口径（建议落地为可复现规则）

- 数据源：Binance Futures `24hr` ticker（rolling 24h）  
- 指标：优先用 `quoteVolume` 作为“24h 成交额（USDT）”，只筛 `USDT` 永续合约（如 `BTCUSDT` 这类）  
- 排序：按 `quoteVolume` 降序取 Top30，并 **force include** `BTCUSDT/ETHUSDT`

### Universe “滑动”（建议：滚动 24h + 粘性/滞回）

- 刷新频率：每 `30min`（可配置）刷新一次 TopN（rolling 24h 本身就是滑动窗口）
- 粘性/滞回（降低币池抖动，建议作为默认行为）：
  - 初始选 Top30
  - 后续刷新时：对“已在币池的币”，只要排名仍在 Top45（=N+15）就保留；对新币只从 Top30 中补齐
  - 可选：设置最短驻留时间（例如 6h）避免频繁换池

## 任务清单（Tasks）

### P02-1：实现 Binance TOP N 拉取与缓存
- 数据来源必须可验证（尽量 Binance 官方 REST）
- 加本地缓存（例如 5～30 分钟），避免每个 cycle 都打 API
- 建议增加：失败降级（保留上一轮缓存 + 记录原因），并把“本轮币池快照”落库（便于复盘）

实现状态（当前代码）：已落地 “官方 REST + 30min 刷新 + TopN+X 滞回 + min-dwell” 的进程内缓存；重启后会重新拉取并重新计算币池（后续如需要可再把币池快照持久化到 DB）。

### P02-2：策略 coin_source 扩展（可选，但建议）
- 在 `StrategyConfig.CoinSource` 增加 `source_type=topn`（或在 paper trader 内部实现 topn 并覆盖候选币）
- 支持配置 N 与过滤条件

实现状态（当前代码）：已支持 `source_type=topn`，并在前端策略编辑器加入配置项（N/刷新/滞回/最短驻留）。

### P02-3：时间框架与指标 lookback 标准化
- 统一用 `["1h","4h","1d"]`
- lookback（建议）：
  - 1h：>= 1200 根（约 50 天，足够多数指标稳定）
  - 4h：>= 800 根（约 133 天）
  - 1d：>= 500 根（约 1.4 年）
- 让指标计算对齐（EMA/MACD/RSI/ATR/BOLL）并保证历史足够长

实现状态（当前代码）：已增强 market 多周期拉取会优先按 `primary_count` 拉取更多根（上限 1500）并带缓存；推荐你在“实盘节奏（1h/4h/1d）”与“快速验收（1m primary）”之间切换来验证链路与性能。

### P02-4：K线来源统一（决策/看板/OPN 一致性）

- 用一个环境变量统一策略输入与看板K线的来源，减少“看见的K线”和“决策/撮合用的K线”不一致
- 默认推荐 Binance（fapi）以更可复现；允许在网络故障时回退

实现状态（当前代码）：已新增 `MARKET_KLINE_SOURCE=auto|binance|coinank`：
- `auto`：优先 Binance，失败回退 CoinAnk
- `binance`：只用 Binance（推荐用于 paper OPN/backtest 一致性）
- `coinank`：只用 CoinAnk（不推荐用于严谨回测/OPN）

### K线“滑动窗口”（建议：只保留最近 N 根 + 增量补齐）

- 目标：让“长期运行”不会随着时间推移无限增长内存/DB 负担，同时避免每 cycle 全量回拉。  
- 建议策略：
  - 每个 `symbol/timeframe` 维护一个固定长度 ring buffer（或在现有缓存结构上做裁剪）
  - 每次 cycle 只拉取“last_seen_ts 之后”的新K线，缺口补齐；并把超出窗口的旧K线丢弃
  - 窗口长度用上面的 lookback 作为下限（实际可多 10% 作为缓冲）

## 验收方式（Verification）

- 启动后观察日志/DB：候选币列表稳定，且能解释为何入选
- 改 N/过滤条件后候选币变化符合预期
- 断网/接口失败时：系统能降级（例如回退到上一轮缓存或静态列表）
