# Phase 01：paper 合约模拟（next_open / OPN 成交）端到端跑通

目标：新增 paper 交易所执行器，让系统在“不接触真实交易所 API 下单”的情况下，自动跑完整链路并在 UI 中展示订单/仓位/净值曲线/决策记录。

## 你需要确认（Before we start）

1) paper 默认币种范围：是否仅限 Binance USDT 永续？（推荐：是）  
2) 默认参数是否先用固定值（已确认）：
   - 手续费：`fee_bps = 5`
   - 滑点：`slippage_bps = 2`
3) `next_open` 定义：决策发生在当前 bar close，成交在下一根 bar open（推荐：是）
   - 口径说明（建议采用）：**OPN = Next Bar Open**，且必须等“下一根K线已产生”后才允许撮合，避免使用未来数据。

## 设计要点（Design）

- 新增 `exchange_type=paper`，但不影响现有 `binance/aster` 逻辑。
- `PaperTrader` 实现 `trader/interface.go` 的 `Trader` 接口，供 `AutoTrader` 复用。
- 成交模型使用 `next_open` / OPN：需要依赖“primary timeframe 的下一根K线 open 价”。
  - 建议口径：决策 snapshot 只使用**各 timeframe 的已收盘K线**；下单生成 `PENDING` 订单，等到 primary 的下一根K线出现后，用该 K 线的 open 价撮合成交。
  - 这能保证：1h/4h/1d 多周期策略不会因为“高周期未收盘”或“偷看未来”产生偏差。
- 实现建议（已按此落地）：paper 下单先落库为 `NEW`，由后台 `Paper order sync`（默认 15s）负责在 OPN 时刻撮合、写入 fills、更新 positions（避免 AutoTrader 假设“秒级成交”导致的不一致）。

## 任务清单（Tasks）

### P01-1：新增 paper 交易所类型（配置与 UI）
- 后端支持 `exchange_type=paper`（列表/创建/更新）
- 前端 exchange 下拉框可选 paper
- paper exchange 不需要 API key，但仍走加密字段存储（保持结构一致）

### P01-2：实现 `PaperTrader`（合约模拟）
必须实现接口：
- `GetBalance()`：返回 `totalWalletBalance/availableBalance/totalEquity/totalUnrealizedProfit` 等字段（对齐 AutoTrader 使用）
- `GetPositions()`：返回 `symbol/side/entryPrice/markPrice/positionAmt/leverage/unRealizedProfit/liquidationPrice` 等字段（对齐 AutoTrader 构建 ctx）
- `OpenLong/OpenShort/CloseLong/CloseShort`：生成本地 orderId，并按 `next_open` 成交、更新仓位
- `SetStopLoss/SetTakeProfit`：写入仓位保护参数（不发真实条件单）
- `GetOrderStatus`：返回 `FILLED` 等状态（paper 下通常立即可确定）

### P01-3：仓位与订单落库
- 复用现有 `store/order.go` 与 `store/position.go` 的结构（保证 UI 不用大改）
- 记录 close_reason：`ai_decision/stop_loss/take_profit` 等（后续复盘用）

### P01-4：SL/TP 触发检查
- 每个 cycle 或每个 bar 更新时：检查当前价格是否触发 SL/TP
- 触发则自动平仓并落库（生成一个“系统平仓”的 order/fill/position close）

### P01-5：端到端验证（关键）
在 UI 中完成：
- 新建 strategy（1h/4h/1d，TOP N 候选币先用静态代替也可）
- 新建 paper exchange
- 新建 trader（绑定 strategy + AI model + paper exchange），点击 Start
- Dashboard 看到：订单、仓位、PnL、决策记录持续更新
- 通过日志或网络监控确认：没有向 Binance/Aster 下真实订单请求

当前实现补充（已落地）：
- 主界面左栏新增 “订单记录” 表格，可直接查看 `trader_orders`（NEW/FILLED 等）
- K 线图会叠加该 trader 的 FILLED 订单标记（B/S）
- Equity 曲线在 ChartTabs 的 Equity 页签

## 验收方式（Verification）

建议用“可重复的最小验收脚本/手工步骤”：
1) 启动：`./start.sh start`
2) UI 配置：
   - AI 模型（可用你自己的 key 或本地兼容 OpenAI API）
   - paper exchange
   - strategy（先固定 1h/4h/1d，候选币可先手工 10 个）
   - trader（scan interval 建议 >= 5 分钟）
3) 观察 30 分钟：
   - 有决策记录入库
   - 有订单与仓位变化（或持续 wait/hold）
   - 净值曲线有点位
4) SL/TP：构造一个小仓位 + 低阈值 SL/TP，验证能触发并记录 close_reason
   - 提示：若 primary timeframe 选 `1h`，OPN/触发撮合只会在整点附近发生；想快速验收可临时把 primary 调到 `1m`。

## “更新太慢”排查（paper + OPN 的常见原因）

- **OPN 本质**：订单只会在“下一根 primary bar 的 open”才撮合；primary=1h/4h/1d 时，看起来就会很慢（这是设计，不是 bug）。
- **快速验收配置（推荐）**：primary=`1m` + scan interval=`1m` + 静态币池 `BTCUSDT/ETHUSDT`，先把链路跑通再切回 1h/4h/1d。
- **性能优化（已做一项）**：市场 K 线拉取加入了按 timeframe 的缓存（减少每个 cycle 重复拉取导致的等待）。  

## OPN（next_open）撮合细则（建议直接写成实现约束）

1) **信号时点**：`t_signal = primary_bar_close_time`（只用已完成 bar）。  
2) **订单创建**：在 `t_signal` 创建 `PENDING` 订单，记录 `target_fill_time = next_primary_bar_open_time`。  
3) **撮合条件**：当且仅当 market 数据已包含 `target_fill_time` 对应的 bar（即“下一根 bar 已生成”）时，才允许撮合。  
4) **成交价**：`fill_price = next_primary_bar.open`，再应用固定 `slippage_bps` 与 `fee_bps`。  
5) **缺数据处理**：若下一根 bar 长时间不可得（行情源故障/延迟），订单保持 `PENDING`，并触发降级/告警（Phase 04 再增强）。  

## 交付物（Deliverables）

- `paper` 模拟盘能长期运行，并完整走通 NOFX 链路
- 为 Phase 02/03（严谨回测与硬风控）打下可持续迭代基础
