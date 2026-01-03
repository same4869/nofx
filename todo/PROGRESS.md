# 进度表（Progress）

说明：每完成一项就在对应框里打勾，并在“记录”里补充日期/备注。

## Phase 00：单人模式 + Docker 长跑
- [x] P00-1：写清单人部署推荐 `.env`（MAX_USERS、REGISTRATION_ENABLED、EXPERIENCE_IMPROVEMENT）
- [x] P00-2：给出“首次注册后自动锁定注册”的操作流程（不改代码版）
- [x] P00-3：给出健康检查/日志/pprof 的本地验证命令

记录：
- 日期：2026-01-03
- 备注：compose 支持 `NOFX_BIND_IP=127.0.0.1` 仅本机访问，`./start.sh single-user-on|off|lock-registration` 一键配置，并在 Phase00 文档补齐操作与验收步骤。

## Phase 01：paper 合约模拟（next_open）
- [x] P01-1：新增 `paper` exchange_type（DB/接口/前端选择项）
- [x] P01-2：实现 `PaperTrader`（实现 `trader.Trader` 接口）
- [x] P01-3：实现 `next_open` / OPN 撮合（避免未来函数）与固定费率/滑点
- [x] P01-4：实现 SL/TP 触发逻辑（并落库 close_reason）
- [x] P01-5：UI 可查看订单/仓位/净值曲线，且不会触发真实交易所调用（订单表已加到主界面）

记录：
- 日期：2026-01-03
- 备注：新增订单列表 UI（App 左栏 “订单记录”）；K 线图已支持显示 FILLED 订单标记；净值曲线在 ChartTabs 的 Equity 页签可查看。建议你用 paper exchange 跑一遍并确认无真实下单请求。

## Phase 02：Universe TOP N + 数据一致性
- [x] P02-1：实现 Binance TOP N 候选币种拉取与缓存（可配置 N + 粘性/滞回）
- [x] P02-2：策略配置支持选择 coin_source=topn（或在 paper 侧强制使用）
- [x] P02-3：统一 1h/4h/1d 时间框架配置、指标 lookback 与滑动窗口策略（按 TF 推荐 lookback 拉取更多历史，输出仍可控）

记录：
- 日期：2026-01-03
- 备注：已新增 `coin_source.source_type=topn`（Binance fapi rolling 24h `quoteVolume` TopN + 30min 刷新 + TopN+X 滞回 + min-dwell），并将 `market.GetWithTimeframes` 改为按 TF 推荐 lookback（1h=1200/4h=800/1d=500）拉取更多历史，避免高周期指标偏差。

## Phase 03：硬风控 + 可解释性
- [x] P03-1：账户级硬风控（MaxMarginUsage / DailyLoss / MaxDrawdown 熔断 + 冷静期可恢复）
- [x] P03-2：币种级与风险金额级硬风控（symbol 集中度 + `risk_usd` + `max_risk_usd`）
- [x] P03-3：关键失败回滚策略（`require_protection=true` 时 SL/TP 失败回滚，best-effort）
- [x] P03-4：结构化可解释性落库与前端展示（前端 DecisionCard 解析 `RISK:` 并展示风控快照）

记录：
- 日期：2026-01-03
- 备注：P03-2 新增：开仓时按“同一 symbol 的已占用名义价值”进行集中度裁剪（沿用 BTC/ETH vs Alt 不同比例），并在启用 `max_risk_usd` 时强制 `risk_usd` 非空且不得超限；P03-4 新增：前端 DecisionCard 以结构化卡片展示 `RISK:` 快照，raw log 默认过滤掉 `RISK:`（可切换 raw）。

## Phase 04：长跑与可观测（Soak Test）
- [x] P04-1：24h soak：cycle 级耗时/成功率采集（`execution_log` 写 `METRIC:` JSON）
- [x] P04-2：异常降级：行情失败/AI 失败连续 N 次进入 wait（并触发短暂停；行情失败会降级币池为 BTC/ETH）
- [x] P04-3：备份与恢复（`./start.sh backup-db` / `./start.sh restore-db <file>`）

记录：
- 日期：2026-01-03
- 备注：前端 DecisionCard 已展示 `METRIC:` 卡片；后端新增 `ErrMarketDataUnavailable` 作为降级触发信号；`start.sh` 增加 DB 备份/恢复命令。
