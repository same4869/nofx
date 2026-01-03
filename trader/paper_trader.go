package trader

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPaperInitialBalance = 10000.0
	defaultPaperFeeBps         = 5.0
	defaultPaperSlippageBps    = 2.0
)

type paperProtection struct {
	StopLoss   float64 `json:"stop_loss"`
	TakeProfit float64 `json:"take_profit"`
}

// PaperTrader simulates a perp-like trading account locally (paper trading).
// It does NOT place any real exchange orders.
type PaperTrader struct {
	st              *store.Store
	traderID        string
	exchangeID      string
	exchangeType    string
	primaryTimeframe string

	initialBalance float64
	feeBps         float64
	slippageBps    float64

	apiClient *market.APIClient

	// protectionsKey: SYMBOL:POSITION_SIDE (e.g., "BTCUSDT:LONG")
	protections     map[string]paperProtection
	protectionsMu   sync.RWMutex
	protectionsKey  string
	lastTriggerScan map[string]int64 // SYMBOL:POSITION_SIDE -> last scanned bar closeTime ms
}

func NewPaperTrader(st *store.Store, traderID, exchangeID string, initialBalance float64, primaryTimeframe string) (*PaperTrader, error) {
	tf := strings.TrimSpace(primaryTimeframe)
	if tf == "" {
		tf = "1h"
	}
	tf, err := market.NormalizeTimeframe(tf)
	if err != nil {
		return nil, err
	}
	if initialBalance <= 0 {
		initialBalance = defaultPaperInitialBalance
	}

	pt := &PaperTrader{
		st:               st,
		traderID:         traderID,
		exchangeID:       exchangeID,
		exchangeType:     "paper",
		primaryTimeframe: tf,
		initialBalance:   initialBalance,
		feeBps:           defaultPaperFeeBps,
		slippageBps:      defaultPaperSlippageBps,
		apiClient:        market.NewAPIClient(),
		protections:      make(map[string]paperProtection),
		protectionsKey:   fmt.Sprintf("paper_protection:%s:%s", traderID, exchangeID),
		lastTriggerScan:  make(map[string]int64),
	}

	pt.loadProtections()
	return pt, nil
}

func (t *PaperTrader) loadProtections() {
	if t.st == nil {
		return
	}
	raw, err := t.st.GetSystemConfig(t.protectionsKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return
	}
	var m map[string]paperProtection
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		logger.Infof("⚠️ paper: failed to parse protections: %v", err)
		return
	}
	t.protectionsMu.Lock()
	t.protections = m
	t.protectionsMu.Unlock()
}

func (t *PaperTrader) saveProtections() {
	if t.st == nil {
		return
	}
	t.protectionsMu.RLock()
	b, err := json.Marshal(t.protections)
	t.protectionsMu.RUnlock()
	if err != nil {
		return
	}
	_ = t.st.SetSystemConfig(t.protectionsKey, string(b))
}

func (t *PaperTrader) GetBalance() (map[string]interface{}, error) {
	// If no DB (e.g., tests), return a simple static balance.
	if t.st == nil {
		return map[string]interface{}{
			"totalWalletBalance":     t.initialBalance,
			"availableBalance":       t.initialBalance,
			"totalUnrealizedProfit":  0.0,
			"totalEquity":            t.initialBalance,
			"wallet_balance":         t.initialBalance,
			"available_balance":      t.initialBalance,
			"unrealized_profit":      0.0,
			"total_equity":           t.initialBalance,
			"balance":                t.initialBalance,
		}, nil
	}

	positions, err := t.getOpenPositions()
	if err != nil {
		return nil, err
	}

	// Fees: include both OPEN and CLOSED positions' accumulated fees (OPEN contains entry fee).
	var totalFees float64
	_ = t.st.DB().QueryRow(`
		SELECT COALESCE(SUM(fee), 0)
		FROM trader_positions
		WHERE trader_id = ? AND exchange_id = ? AND exchange_type = ?
	`, t.traderID, t.exchangeID, t.exchangeType).Scan(&totalFees)

	// Realized PnL: sum closed positions realized PnL (fees are accounted separately).
	var realizedPnL float64
	_ = t.st.DB().QueryRow(`
		SELECT COALESCE(SUM(realized_pnl), 0)
		FROM trader_positions
		WHERE trader_id = ? AND exchange_id = ? AND exchange_type = ? AND status = 'CLOSED'
	`, t.traderID, t.exchangeID, t.exchangeType).Scan(&realizedPnL)

	priceMap, unrealized, marginUsed, err := t.computeMarkPricesAndPnL(positions)
	if err != nil {
		return nil, err
	}
	_ = priceMap // kept for debugging future use

	walletBalance := t.initialBalance + realizedPnL - totalFees
	totalEquity := walletBalance + unrealized
	available := walletBalance - marginUsed
	if available < 0 {
		available = 0
	}

	return map[string]interface{}{
		"totalWalletBalance":    walletBalance,
		"availableBalance":      available,
		"totalUnrealizedProfit": unrealized,
		"totalEquity":           totalEquity,
	}, nil
}

func (t *PaperTrader) GetPositions() ([]map[string]interface{}, error) {
	if t.st == nil {
		return []map[string]interface{}{}, nil
	}

	positions, err := t.getOpenPositions()
	if err != nil {
		return nil, err
	}

	priceMap, unrealized, _, err := t.computeMarkPricesAndPnL(positions)
	if err != nil {
		return nil, err
	}
	_ = unrealized

	out := make([]map[string]interface{}, 0, len(positions))
	for _, p := range positions {
		mark := priceMap[p.Symbol]
		qty := p.Quantity
		sideLower := "long"
		positionAmt := qty
		if strings.ToUpper(p.Side) == "SHORT" {
			sideLower = "short"
			positionAmt = -qty
		}

		unRealized := 0.0
		if strings.ToUpper(p.Side) == "LONG" {
			unRealized = (mark - p.EntryPrice) * qty
		} else {
			unRealized = (p.EntryPrice - mark) * qty
		}

		out = append(out, map[string]interface{}{
			"symbol":            p.Symbol,
			"side":              sideLower,
			"entryPrice":        p.EntryPrice,
			"markPrice":         mark,
			"positionAmt":       positionAmt,
			"leverage":          float64(p.Leverage),
			"unRealizedProfit":  unRealized,
			"liquidationPrice":  paperComputeLiquidation(p.EntryPrice, p.Leverage, strings.ToLower(sideLower)),
		})
	}

	return out, nil
}

func (t *PaperTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.submitOrder(symbol, "open_long", "BUY", "LONG", quantity, leverage, "ai_decision")
}

func (t *PaperTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.submitOrder(symbol, "open_short", "SELL", "SHORT", quantity, leverage, "ai_decision")
}

func (t *PaperTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	q, lev, err := t.getCloseQuantityAndLeverage(symbol, "LONG", quantity)
	if err != nil {
		return nil, err
	}
	return t.submitOrder(symbol, "close_long", "SELL", "LONG", q, lev, "ai_decision")
}

func (t *PaperTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	q, lev, err := t.getCloseQuantityAndLeverage(symbol, "SHORT", quantity)
	if err != nil {
		return nil, err
	}
	return t.submitOrder(symbol, "close_short", "BUY", "SHORT", q, lev, "ai_decision")
}

func (t *PaperTrader) SetLeverage(symbol string, leverage int) error { return nil }
func (t *PaperTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }

func (t *PaperTrader) GetMarketPrice(symbol string) (float64, error) {
	symbol = market.Normalize(symbol)
	return t.apiClient.GetCurrentPrice(symbol)
}

func (t *PaperTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	symbol = market.Normalize(symbol)
	side := strings.ToUpper(positionSide)
	if side != "LONG" && side != "SHORT" {
		return fmt.Errorf("invalid positionSide: %s", positionSide)
	}

	key := symbol + ":" + side
	t.protectionsMu.Lock()
	p := t.protections[key]
	p.StopLoss = stopPrice
	t.protections[key] = p
	t.protectionsMu.Unlock()
	t.saveProtections()
	return nil
}

func (t *PaperTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	symbol = market.Normalize(symbol)
	side := strings.ToUpper(positionSide)
	if side != "LONG" && side != "SHORT" {
		return fmt.Errorf("invalid positionSide: %s", positionSide)
	}

	key := symbol + ":" + side
	t.protectionsMu.Lock()
	p := t.protections[key]
	p.TakeProfit = takeProfitPrice
	t.protections[key] = p
	t.protectionsMu.Unlock()
	t.saveProtections()
	return nil
}

func (t *PaperTrader) CancelStopLossOrders(symbol string) error {
	symbol = market.Normalize(symbol)
	t.protectionsMu.Lock()
	for _, side := range []string{"LONG", "SHORT"} {
		k := symbol + ":" + side
		if p, ok := t.protections[k]; ok {
			p.StopLoss = 0
			t.protections[k] = p
		}
	}
	t.protectionsMu.Unlock()
	t.saveProtections()
	return nil
}

func (t *PaperTrader) CancelTakeProfitOrders(symbol string) error {
	symbol = market.Normalize(symbol)
	t.protectionsMu.Lock()
	for _, side := range []string{"LONG", "SHORT"} {
		k := symbol + ":" + side
		if p, ok := t.protections[k]; ok {
			p.TakeProfit = 0
			t.protections[k] = p
		}
	}
	t.protectionsMu.Unlock()
	t.saveProtections()
	return nil
}

func (t *PaperTrader) CancelAllOrders(symbol string) error {
	if t.st == nil {
		return nil
	}
	symbol = market.Normalize(symbol)
	_, err := t.st.DB().Exec(`
		UPDATE trader_orders
		SET status = 'CANCELED', updated_at = ?
		WHERE trader_id = ? AND exchange_id = ? AND exchange_type = ? AND symbol = ? AND status = 'NEW'
	`, time.Now().UTC().Format(time.RFC3339), t.traderID, t.exchangeID, t.exchangeType, symbol)
	return err
}

func (t *PaperTrader) CancelStopOrders(symbol string) error {
	_ = t.CancelStopLossOrders(symbol)
	_ = t.CancelTakeProfitOrders(symbol)
	return nil
}

func (t *PaperTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// Simple fixed precision for paper mode.
	return fmt.Sprintf("%.6f", quantity), nil
}

func (t *PaperTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	if t.st == nil {
		return map[string]interface{}{
			"status":      "NEW",
			"avgPrice":    0.0,
			"executedQty": 0.0,
			"commission":  0.0,
		}, nil
	}

	order, err := t.st.Order().GetOrderByExchangeID(t.exchangeID, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return map[string]interface{}{
		"status":      order.Status,
		"avgPrice":    order.AvgFillPrice,
		"executedQty": order.FilledQuantity,
		"commission":  order.Commission,
	}, nil
}

func (t *PaperTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	if t.st == nil {
		return nil, nil
	}
	rows, err := t.st.DB().Query(`
		SELECT symbol, side, entry_price, exit_price, quantity, realized_pnl, fee, leverage, entry_time, exit_time, exit_order_id, close_reason, exchange_position_id
		FROM trader_positions
		WHERE trader_id = ? AND exchange_id = ? AND exchange_type = ? AND status = 'CLOSED' AND exit_time >= ?
		ORDER BY exit_time DESC
		LIMIT ?
	`, t.traderID, t.exchangeID, t.exchangeType, startTime.UTC().Format(time.RFC3339), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ClosedPnLRecord
	for rows.Next() {
		var symbol, side, exitOrderID, closeReason, exchangePosID string
		var entryPrice, exitPrice, quantity, realizedPnL, fee float64
		var leverage int
		var entryTimeStr, exitTimeStr string
		if err := rows.Scan(&symbol, &side, &entryPrice, &exitPrice, &quantity, &realizedPnL, &fee, &leverage, &entryTimeStr, &exitTimeStr, &exitOrderID, &closeReason, &exchangePosID); err != nil {
			continue
		}
		entryTime, _ := time.Parse(time.RFC3339, entryTimeStr)
		exitTime, _ := time.Parse(time.RFC3339, exitTimeStr)

		sideLower := "long"
		if strings.ToUpper(side) == "SHORT" {
			sideLower = "short"
		}

		out = append(out, ClosedPnLRecord{
			Symbol:       symbol,
			Side:         sideLower,
			EntryPrice:   entryPrice,
			ExitPrice:    exitPrice,
			Quantity:     quantity,
			RealizedPnL:  realizedPnL,
			Fee:          fee,
			Leverage:     leverage,
			EntryTime:    entryTime,
			ExitTime:     exitTime,
			OrderID:      exitOrderID,
			CloseType:    closeReason,
			ExchangeID:   exchangePosID,
		})
	}
	return out, nil
}

func (t *PaperTrader) getCloseQuantityAndLeverage(symbol string, positionSide string, quantity float64) (float64, int, error) {
	if t.st == nil {
		return 0, 1, fmt.Errorf("paper trader store is nil")
	}
	symbol = market.Normalize(symbol)
	positionSide = strings.ToUpper(positionSide)
	pos, err := t.st.Position().GetOpenPositionBySymbol(t.traderID, symbol, positionSide)
	if err != nil {
		return 0, 1, err
	}
	if pos == nil {
		return 0, 1, fmt.Errorf("no open %s position for %s", positionSide, symbol)
	}
	q := quantity
	if q <= 0 {
		q = pos.Quantity
	}
	if q <= 0 {
		return 0, 1, fmt.Errorf("invalid close quantity")
	}
	return q, pos.Leverage, nil
}

func (t *PaperTrader) submitOrder(symbol, action, side, positionSide string, quantity float64, leverage int, reason string) (map[string]interface{}, error) {
	if t.st == nil {
		// Still return an orderId to satisfy AutoTrader, but nothing is persisted.
		return map[string]interface{}{"orderId": time.Now().UnixNano()}, nil
	}

	symbol = market.Normalize(symbol)
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if leverage <= 0 {
		leverage = 1
	}

	openMs, err := t.computeNextPrimaryOpenTime(symbol)
	if err != nil {
		return nil, err
	}

	// Snapshot current price for record (actual fill is OPN).
	curPrice, _ := t.apiClient.GetCurrentPrice(symbol)

	orderIDInt := time.Now().UnixNano()
	orderID := strconv.FormatInt(orderIDInt, 10)
	clientOrderID := fmt.Sprintf("paper_opn:%d;reason=%s", openMs, reason)

	order := &store.TraderOrder{
		TraderID:        t.traderID,
		ExchangeID:      t.exchangeID,
		ExchangeType:    t.exchangeType,
		ExchangeOrderID: orderID,
		ClientOrderID:   clientOrderID,
		Symbol:          symbol,
		Side:            side,
		PositionSide:    positionSide,
		Type:            "MARKET",
		TimeInForce:     "GTC",
		Quantity:        quantity,
		Price:           curPrice,
		Status:          "NEW",
		FilledQuantity:  0,
		AvgFillPrice:    0,
		Commission:      0,
		CommissionAsset: "USDT",
		Leverage:        leverage,
		ReduceOnly:      strings.HasPrefix(action, "close_"),
		ClosePosition:   strings.HasPrefix(action, "close_"),
		OrderAction:     action,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := t.st.Order().CreateOrder(order); err != nil {
		return nil, err
	}
	return map[string]interface{}{"orderId": orderIDInt}, nil
}

func (t *PaperTrader) getOpenPositions() ([]*store.TraderPosition, error) {
	if t.st == nil {
		return nil, nil
	}
	all, err := t.st.Position().GetOpenPositions(t.traderID)
	if err != nil {
		return nil, err
	}
	out := make([]*store.TraderPosition, 0, len(all))
	for _, p := range all {
		if p.ExchangeID == t.exchangeID && p.ExchangeType == t.exchangeType {
			out = append(out, p)
		}
	}
	return out, nil
}

func (t *PaperTrader) computeMarkPricesAndPnL(positions []*store.TraderPosition) (map[string]float64, float64, float64, error) {
	priceMap := make(map[string]float64)
	unrealized := 0.0
	marginUsed := 0.0

	for _, p := range positions {
		if _, ok := priceMap[p.Symbol]; !ok {
			price, err := t.apiClient.GetCurrentPrice(p.Symbol)
			if err != nil {
				return nil, 0, 0, err
			}
			priceMap[p.Symbol] = price
		}
		mark := priceMap[p.Symbol]
		qty := p.Quantity

		if strings.ToUpper(p.Side) == "LONG" {
			unrealized += (mark - p.EntryPrice) * qty
		} else {
			unrealized += (p.EntryPrice - mark) * qty
		}

		lev := p.Leverage
		if lev <= 0 {
			lev = 1
		}
		marginUsed += (qty * mark) / float64(lev)
	}

	return priceMap, unrealized, marginUsed, nil
}

func (t *PaperTrader) computeNextPrimaryOpenTime(symbol string) (int64, error) {
	nowMs := time.Now().UnixMilli()
	klines, err := t.apiClient.GetKlines(symbol, t.primaryTimeframe, 3)
	if err != nil {
		return 0, err
	}
	if len(klines) == 0 {
		return 0, fmt.Errorf("no klines")
	}

	// Pick the most recent fully-closed bar (closeTime < now).
	var lastClosed *market.Kline
	for i := len(klines) - 1; i >= 0; i-- {
		if klines[i].CloseTime < nowMs {
			k := klines[i]
			lastClosed = &k
			break
		}
	}
	if lastClosed == nil {
		// Fallback: use the earliest one we got.
		k := klines[0]
		lastClosed = &k
	}
	return lastClosed.CloseTime + 1, nil
}

func paperComputeLiquidation(entry float64, leverage int, side string) float64 {
	if leverage <= 0 {
		return 0
	}
	lev := float64(leverage)
	if side == "long" {
		return entry * (1.0 - 1.0/lev)
	}
	return entry * (1.0 + 1.0/lev)
}

func parsePaperClientOrder(clientOrderID string) (openMs int64, reason string) {
	reason = "ai_decision"
	parts := strings.Split(clientOrderID, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "paper_opn:") {
			v := strings.TrimPrefix(p, "paper_opn:")
			if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
				openMs = ts
			}
		}
		if strings.HasPrefix(p, "reason=") {
			reason = strings.TrimPrefix(p, "reason=")
		}
	}
	return openMs, reason
}
