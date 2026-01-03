# AI Handoff（快速接手指南）

## 一句话定位

把 NOFX 从“开源通用 AI 交易平台”改造成“单人本地长期运行 + 自动模拟盘（paper）闭环”，并为未来接入实盘保留一致的风控/日志/数据语义。

## 用户偏好（不可随意更改）

- 市场：加密货币（现货+合约，当前优先合约式模拟盘）  
- 优先交易所：ASTER、Binance（实盘后续再接）  
- 主分析周期：1h / 4h / 1d  
- 成交假设：`next_open`（OPN：下一根 primary bar open 成交）  
- Universe：Binance USDT 永续 rolling 24h `quoteVolume` Top30（强制包含 BTC/ETH）  
- 部署：Docker，本地长期跑；优先单人/本机访问

## 关键改动清单（What changed）

### 1) Paper（模拟盘）执行器

- 新增 `exchange_type=paper`，后端/前端均可创建 paper exchange（不需要密钥）。
- `PaperTrader` 实现 `trader.Trader` 接口：下单→落库→OPN 撮合→写 fills→更新 positions。
- SL/TP：以“bar 内 high/low 触发 + OPN 平仓单”方式实现，close_reason 落库。

关键文件：
- `trader/paper_trader.go`
- `trader/paper_order_sync.go`
- `trader/auto_trader.go`

### 2) Universe：TopN（可复现）

- 新增 `coin_source.source_type=topn`：从 Binance Futures 24hr ticker 拉 `quoteVolume` 排名。
- 带“滞回/最短驻留”减少币池抖动，失败回退到上一轮缓存。

关键文件：
- `decision/coin_source_topn.go`
- `store/strategy.go`（配置字段）
- `web/src/components/strategy/CoinSourceEditor.tsx`

### 3) 硬风控（代码闸门）+ 可解释性

- 账户级：MaxMarginUsage、日内亏损熔断、最大回撤熔断、冷静期（可重启恢复）。
- 币种/单笔：symbol 集中度、`risk_usd` 与 `max_risk_usd`、SL/TP 必须成功否则回滚（best-effort）。
- 可解释：每 cycle 写 `RISK:` JSON 快照；每 cycle 写 `METRIC:` 性能快照；前端展示成卡片。

关键文件：
- `trader/risk_state.go`（risk_state 持久化到 system_config）
- `trader/risk_gates.go`（RISK 快照 + 熔断逻辑）
- `trader/cycle_metrics.go`（METRIC 快照）
- `web/src/components/DecisionCard.tsx`（解析并展示 RISK/METRIC）

### 4) 单人模式 + 运维脚本增强

- `NOFX_BIND_IP` 支持只绑定 `127.0.0.1`；`start.sh` 一键开/关单人模式、锁注册。
- 增加 DB 备份/恢复：`./start.sh backup-db` / `./start.sh restore-db <file>`

关键文件：
- `start.sh`
- `docker-compose.yml`
- `docker-compose.prod.yml`

### 5) UI 补齐链路可见性

- Strategy Studio：TopN 配置、风控配置、preset（快速验收/1h4h1d）
- 主界面增加订单列表（验证 paper 订单/成交链路）

关键文件：
- `web/src/components/OrderHistory.tsx`
- `web/src/App.tsx`
- `web/src/pages/StrategyStudioPage.tsx`

## 运行与验证（最短路径）

1) 启动：`./start.sh start --build`
2) 单人模式（推荐）：`./start.sh single-user-on && ./start.sh recreate`
3) UI 配置：
   - 创建 paper exchange
   - 创建 strategy（用 preset：1h/4h/1d + Top30）
   - 创建 trader 绑定 paper exchange + strategy，Start
4) 验证：
   - DecisionCard 中看到 `RISK:` 与 `METRIC:` 卡片
   - 主界面看到订单列表；K 线图看到 FILLED 订单标记
   - 无真实交易所下单（paper 不会调用真实下单接口；行情请求属于正常）

## AI 提供方：LinkAI（体验一致性）

项目已内置 `provider=linkai`（OpenAI 兼容）作为一等公民，前端会自动预填 LinkAI BaseURL，减少手误。

配置路径（UI）：
1) `AI Traders` → `Models` → `Add AI Model` → 选择 `LinkAI`
2) 填写：
   - `API Key`：你的 LinkAI Key（如需要 `app_code`，常见格式为 `APIKEY-APP_CODE`）
   - `Custom Base URL`：默认已填 `https://api.link-ai.tech/v1`（一般无需改）
   - `Custom Model Name`：默认已填 `deepseek-chat`，可在 LinkAI 控制台/文档里替换为你想用的模型
3) 创建/编辑 Trader 时把 `AI Model` 选成 `LinkAI`

## 未来迭代建议（如果要继续）

优先级建议：
1) K 线数据源已支持收敛到 Binance 官方：用 `MARKET_KLINE_SOURCE=auto|binance`（更稳定/可复现）  
2) 为 paper 增加 spot 模拟（与 perp 分开）  
3) 实盘接入做成“仅替换执行器”的开关（保持风控与日志一致）  
