# 运行维护（Operations / Runbook）

## 启动/停止/日志

- 启动：`./start.sh start`（或 `./start.sh start --build`）
- 停止：`./start.sh stop`
- 重启（推荐，原容器原数据）：`./start.sh restart`
- 重建（应用端口绑定/环境变量变更）：`./start.sh recreate`
- 状态：`./start.sh status`
- 日志：`./start.sh logs`（或 `./start.sh logs backend|frontend`）
- 健康检查：`curl http://localhost:8080/api/health`

## 单人模式与注册

- 仅本机访问：`./start.sh single-user-on && ./start.sh recreate`
- 首次注册后建议锁注册入口：`./start.sh lock-registration`

## 数据备份与恢复（SQLite）

- 备份：`./start.sh backup-db`（输出在 `data/backups/`）
- 恢复：`./start.sh restore-db data/backups/<file>`
  - 会先 stop 服务
  - 会先备份当前 DB 到 `data/data.db.pre-restore.*.bak`
  - 然后覆盖 `data/data.db` 并重启

## 观测与性能定位

- pprof：后端默认启用 `:6060`（容器需放行端口时再调整 compose）
- 决策日志：
  - DecisionCard 会展示 `RISK:`（风控快照）
  - DecisionCard 会展示 `METRIC:`（耗时快照）

## 行情数据源（K线）与缓存

- K线来源：用 `.env` 设置 `MARKET_KLINE_SOURCE=auto|binance|coinank`
  - 推荐：`auto`（优先 Binance，失败回退 CoinAnk）
- 生效方式：修改 `.env` 后运行 `./start.sh recreate`
- 验证方式：`./start.sh logs backend` 中会打印 `Market kline source: ...`
- 缓存策略（P1）：K线缓存按“下一根 bar 出现”做刷新（1h/4h/1d 会显著减少重复拉取）

## 异常与降级

当前降级策略（后端）：
- 连续市场数据不可用 >=3：
  - 暂停 10 分钟
  - 2 小时内 Universe 降级为 BTC/ETH（并始终保留当前持仓币种）
- 连续 AI 失败 >=3：
  - 暂停 15 分钟

## 常见问题

1) “更新很慢”  
若 primary 选 1h/4h/1d，OPN 只会在下一根 bar open 才成交，这是设计。  
想快速验收链路：用 Strategy Studio 的 fast preset（1m primary）。

2) “开仓太猛/杠杆太高”

推荐做法（更稳健）：
- 在 Strategy Studio 把 `prompt_variant` 设为 `conservative`
- 风控建议起步值（可按你偏好再收紧）：
  - `max_positions=2`
  - `btc_eth_max_leverage=2~3`，`altcoin_max_leverage=2`
  - `btc_eth_max_position_value_ratio=0.3~0.6`，`altcoin_max_position_value_ratio=0.1~0.25`
  - `max_margin_usage=0.2`
  - `min_confidence=80`（后端硬闸门，低置信开仓会被拒绝）
