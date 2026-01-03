# ROADMAP：个人本地长期运行 + 自动模拟盘（paper trading）

目标：把 NOFX 改造成「一个人本地长期运行、自动模拟盘交易、可复盘/可观测、后续可平滑接入 Binance/Aster 实盘」的系统。

## 指导原则（保证未来能持续 `git pull` 上游）

1. **最小侵入**：尽量新增文件/新增 exchange_type，不重写核心逻辑。
2. **复用现有链路**：尽量复用 `AutoTrader -> decision -> store -> web` 的整条链路。
3. **paper 与 live 的差异“收敛在执行器”**：未来接实盘时，尽量做到“只换 exchange_type/密钥，不改策略/风控/日志结构”。
4. **默认安全 & 单人模式**：减少公网暴露面、禁用遥测、限制注册与用户数。

## 里程碑（Milestones）

### M0：本地长期跑起来（单人模式 + Docker）
- 可用 Docker `restart: unless-stopped` 长期运行
- 单用户可登录并配置策略/AI 模型
- 遥测默认关闭（或可一键关闭）
- 健康检查/日志/pprof 可用

### M1：paper 合约模拟（next_open 成交）跑通端到端链路
- 新增 `paper` 交易所类型（exchange_type=paper）
- `AutoTrader` 可选择 paper 执行器并自动交易（不触发真实交易所下单）
- 订单/成交/仓位/PnL 走现有 DB 表结构展示在 UI
- 支持 SL/TP 触发（基于 K 线 next_open / bar 内逻辑定义）

### M2：Universe（Binance TOP N）与数据一致性收敛
- 候选币种来源从外部 http 服务收敛为 Binance 官方数据（或本地缓存）
- 回测/模拟盘/看板使用同一套 K 线来源与时间框架配置（1h/4h/1d）

### M3：硬风控闸门 + 可解释性
- 下单前强制风控（账户级/币种级/风险金额级）
- 关键失败回滚（例如 SL/TP 设置失败则撤单/平仓，paper & future live 均一致）
- 决策可解释性从“长文本”转为结构化摘要 + 可复盘快照

### M4：稳定性与可观测（Soak Test）
- 24h/72h 长跑：CPU/内存/日志/DB 增长可控
- 自检与降级策略（行情失败/AI 失败连续 N 次自动进入 wait）

## 阶段划分与依赖

- Phase 00：`todo/phases/phase-00-single-user-docker.md`（支撑所有后续）
- Phase 01：`todo/phases/phase-01-paper-perp-next-open.md`（核心功能）
- Phase 02：`todo/phases/phase-02-universe-topn-data-alignment.md`（严谨性提升）
- Phase 03：`todo/phases/phase-03-risk-gates-explainability.md`（为未来实盘做准备）
- Phase 04：`todo/phases/phase-04-soak-observability.md`（长期运行质量）

## 当前待你补充确认的关键参数（会影响实现细节）

这些在对应 phase 文件里也会出现，但建议你提前想好：
- TOP N 的 `N`（已确认：30）
- TOP N 的筛选口径（已确认：Binance **USDT 永续** 24h 成交额 TopN，且强制包含 BTC/ETH）
- paper 的费率/滑点默认值（已确认先固定：fee=5bps, slippage=2bps）
- `next_open`（也可简称 **OPN**）的定义：使用哪一根K线？（建议：决策时点=当前 primary bar close；成交价=下一根 primary bar 的 open，且必须等“下一根K线已生成”后才能撮合，避免未来函数）
- “滑动”的定义（建议）：
  - **Universe 滑动**：24h 指标使用 Binance 自带的 rolling 24h 口径，按固定间隔（如 30min）刷新，并加入“粘性/滞回”降低币池抖动
  - **K线滑动窗口**：每个 symbol/timeframe 只维护最近 N 根（如 1h/4h 各 800～2000 根、1d 400～800 根），cycle 时增量补齐，不做全量回拉

## 已确认的默认参数（Personal Defaults）

- Universe：Binance USDT Perp 24h 成交额 Top30（强制包含 BTCUSDT/ETHUSDT）
- 成交：`next_open` / OPN（按策略 primary timeframe 的下一根开盘价成交）
- 费率与滑点：`fee_bps=5`，`slippage_bps=2`（后续可升级为动态）
