# Phase 03：硬风控闸门 + 可解释性（为未来实盘做准备）

目标：把风险控制从“提示词建议”升级为“代码硬闸门”，并把决策/执行过程变成可复盘的结构化数据。

## 你需要确认（Before we start）

1) 风控优先级：是否“宁可少交易，也不允许超规则开仓”？（推荐：是）  
2) 硬风控阈值初版建议（你可改）：
   - `MaxMarginUsage`: 30%（保守）
   - `DailyLossLimit`: 2%～5%
   - `MaxDrawdown`: 10%～20%
3) SL/TP：是否要求“开仓后必须成功设置保护，否则立即回滚平仓”？（推荐：是）

## 任务清单（Tasks）

### P03-1：账户级硬风控
- 最大保证金占用
- 日内亏损熔断（进入冷静期）
- 总回撤熔断（停止开新仓/必要时强平）

### P03-2：币种级/风险金额级硬风控
- 单币最大风险金额（risk_usd 上限）
- 单币种最大集中度（notional/ equity）

### P03-3：关键失败回滚
- paper：SL/TP 写入失败/逻辑异常必须回滚
- future live：未来接入实盘时保证同样语义

### P03-4：结构化可解释性
- 存储：
  - 输入特征快照（关键指标/价格变化/候选币评分）
  - 决策摘要（理由要点、触发信号、风险点）
  - 风控裁剪结果（哪些被 cap/被拒绝、原因）
- 前端展示：以“摘要 + 关键数值”替代长文本

## 验收方式（Verification）

- 人为设置很激进策略，验证会被硬风控拦截并记录原因
- 人为模拟连续亏损，验证触发熔断并进入冷静期
- SL/TP 失败路径能回滚且不留下“裸仓”

## 当前实现（What’s landed）

### 1) 风控参数位置（Strategy → Risk Control）

已新增/支持这些字段（全部是后端代码硬闸门，0=关闭）：
- `max_margin_usage`：最大保证金使用率（比例，默认新策略 0.3=30%）
- `daily_loss_limit`：日内亏损熔断（比例）
- `max_drawdown`：最大回撤熔断（比例）
- `stop_cooldown_minutes`：熔断后冷静期（分钟）
- `max_risk_usd`：单笔最大风险金额（USDT，按 stop-loss 距离推导）
- `require_protection`：要求 SL/TP 设置成功，否则回滚（best-effort）

前端已在 Strategy Studio 的 Risk Control 区域暴露对应配置项。

### 2) 熔断状态可恢复（重启不丢）

风险状态会写入 `system_config`（Key：`risk_state:<trader_id>:<exchange_id>`），包含：
- 日内起始净值、日内已实现盈亏
- 历史峰值净值（用于回撤熔断）
- 熔断冷静期 `stop_until` + 触发原因

### 3) 可解释性落库（后端）

每个 cycle 会在 `decision_records.execution_log` 追加一条结构化快照：
- 前缀：`RISK:`
- 内容：JSON（包含 equity、margin_used_pct、daily_loss_pct、drawdown_pct、limits、stop_reason 等）

开仓时如果发生风控裁剪（例如 MaxMarginUsage/MaxRiskUSD cap），会在对应 action 的 `reasoning` 里追加简短标记，方便回放定位。

### 4) 可解释性展示（前端）

`web/src/components/DecisionCard.tsx` 会：
- 自动解析最新一条 `execution_log` 中的 `RISK:` JSON
- 用结构化卡片展示：Equity / MarginUsed / DailyLoss / Drawdown / StopUntil / Reason
- raw execution log 默认过滤 `RISK:`（可切换为 raw 查看完整日志）

### 5) P03-2：币种级硬闸门细化

开仓执行时新增两类硬约束：
- **symbol 集中度**：同一 `symbol` 的“已占用名义价值 + 本次开仓名义价值”不得超过 `equity × ratio`（BTC/ETH 与 Alt 使用各自 ratio）
- **risk_usd**：当启用 `max_risk_usd` 时，要求 AI 输出 `risk_usd>0` 且不得超限；同时仍会按 stop-loss 距离计算的 implied risk 做二次裁剪
