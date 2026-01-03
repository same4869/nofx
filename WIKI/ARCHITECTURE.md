# 架构与关键语义（Architecture）

## 端到端链路

1) Strategy（策略配置）  
`store.StrategyConfig`（前端 Strategy Studio 编辑）定义：
- 候选币：static / topn
- 指标：多周期 `selected_timeframes` + `primary_timeframe` + count
- 风控：RiskControl（含硬闸门字段）

2) Context（交易上下文）  
`AutoTrader.buildTradingContext()`：
- 拉 balance/positions
- 获取 candidate coins
- 拉多周期 market data（`market.GetWithTimeframes`）
- 拼接成 `decision.Context`

3) AI 决策  
`decision.GetFullDecisionWithStrategy()`：
- 构建 system prompt / user prompt
- 调 AI（MCP client）
- 解析为 decisions（open/close/hold/wait）

4) 执行与落库  
`AutoTrader.execute*WithRecord()`：
- 先做硬风控闸门（cap / reject）
- 走 `Trader` 接口下单
- 订单/成交/仓位写入 store（paper 为本地撮合）

5) 复盘与看板  
- 后端：`decision_records`、`trader_orders`、`trader_fills`、`trader_positions`、`trader_equity_snapshots`
- 前端：DecisionCard（展示 RISK/METRIC）、订单列表、持仓、净值曲线、图表叠加订单标记

## OPN（next_open）语义（必须保持）

- 订单创建时点：只使用已收盘的 K 线做决策输入  
- 撮合成交价：下一根 primary bar 的 open  
- 禁止未来函数：必须等“下一根 K 线实际出现”后才能撮合  

## Paper 与 Live 的差异边界（设计原则）

- paper 与 live 的差异应“收敛在执行器（Trader 实现）”，策略/风控/日志结构保持一致。  
- 未来接入实盘时，尽量做到：只换 exchange_type 与密钥，不改策略配置语义、不改风控语义。  

## 风控与可解释性语义

- 风控闸门优先级：宁可少交易，也不允许越线开仓。  
- `RISK:`：每个 cycle 的账户级风险快照（用于复盘与 UI）。  
- `METRIC:`：每个 cycle 的性能/耗时快照（用于长跑与定位瓶颈）。  

