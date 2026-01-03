package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"strconv"
	"strings"
	"time"
)

type paperPendingOrder struct {
	dbID          int64
	orderID       string
	clientOrderID string
	symbol        string
	positionSide  string
	action        string
	qty           float64
	leverage      int
}

// StartOrderSync starts background order fill + position update + SL/TP trigger checks for paper trading.
func (t *PaperTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	// Keep parameters for API consistency; PaperTrader already has them.
	if st != nil && t.st == nil {
		t.st = st
	}
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.syncOnce(); err != nil {
				logger.Infof("⚠️  paper sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 Paper order sync started (interval: %v)", interval)
}

func (t *PaperTrader) syncOnce() error {
	if t.st == nil {
		return nil
	}

	// 1) Evaluate SL/TP triggers (may enqueue close orders)
	if err := t.scanStopTakeTriggers(); err != nil {
		logger.Infof("⚠️  paper SL/TP scan failed: %v", err)
	}

	// 2) Fill due NEW orders on OPN
	return t.fillDueOrders()
}

func (t *PaperTrader) fillDueOrders() error {
	now := time.Now().UTC()
	nowMs := now.UnixMilli()

	rows, err := t.st.DB().Query(`
		SELECT id, exchange_order_id, client_order_id, symbol, position_side, order_action, quantity, leverage, status
		FROM trader_orders
		WHERE trader_id = ? AND exchange_id = ? AND exchange_type = ? AND status = 'NEW'
		ORDER BY created_at ASC
	`, t.traderID, t.exchangeID, t.exchangeType)
	if err != nil {
		return err
	}
	defer rows.Close()

	var list []paperPendingOrder
	for rows.Next() {
		var p paperPendingOrder
		var status string
		if err := rows.Scan(&p.dbID, &p.orderID, &p.clientOrderID, &p.symbol, &p.positionSide, &p.action, &p.qty, &p.leverage, &status); err != nil {
			continue
		}
		list = append(list, p)
	}

	for _, o := range list {
		targetOpenMs, reason := parsePaperClientOrder(o.clientOrderID)
		if targetOpenMs <= 0 || nowMs < targetOpenMs {
			continue
		}

		openPrice, err := t.getOpenPriceAt(o.symbol, targetOpenMs)
		if err != nil {
			continue // keep pending
		}
		execPrice := t.applySlippage(openPrice, o.positionSide, strings.HasPrefix(o.action, "open_"))

		notional := execPrice * o.qty
		fee := notional * (t.feeBps / 10000.0)

		// Update order to FILLED
		if err := t.st.Order().UpdateOrderStatus(o.dbID, "FILLED", o.qty, execPrice, fee); err != nil {
			return err
		}

		// Create fill (deterministic trade id per order)
		fill := &store.TraderFill{
			TraderID:        t.traderID,
			ExchangeID:      t.exchangeID,
			ExchangeType:    t.exchangeType,
			OrderID:         o.dbID,
			ExchangeOrderID: o.orderID,
			ExchangeTradeID: o.orderID + ":paper_fill",
			Symbol:          o.symbol,
			Side:            t.orderSideForAction(o.action),
			Price:           execPrice,
			Quantity:        o.qty,
			QuoteQuantity:   notional,
			Commission:      fee,
			CommissionAsset: "USDT",
			RealizedPnL:     0,
			IsMaker:         false,
			CreatedAt:       now,
		}

		if strings.HasPrefix(o.action, "close_") {
			entryPrice, _ := t.getOpenPositionEntry(o.symbol, o.positionSide)
			if entryPrice > 0 {
				if o.positionSide == "LONG" {
					fill.RealizedPnL = (execPrice - entryPrice) * o.qty
				} else {
					fill.RealizedPnL = (entryPrice - execPrice) * o.qty
				}
				fill.RealizedPnL = math.Round(fill.RealizedPnL*100) / 100
			}
		}

		_ = t.st.Order().CreateFill(fill)

		// Update position records
		if err := t.applyFillToPositions(o, execPrice, fee, reason, time.UnixMilli(targetOpenMs)); err != nil {
			return err
		}
	}

	return nil
}

func (t *PaperTrader) applyFillToPositions(o paperPendingOrder, execPrice float64, fee float64, reason string, fillTime time.Time) error {
	posStore := t.st.Position()
	action := o.action

	if strings.HasPrefix(action, "open_") {
		// Refuse to open if already has an open position of same side.
		existing, err := posStore.GetOpenPositionBySymbol(t.traderID, o.symbol, o.positionSide)
		if err != nil {
			return err
		}
		if existing != nil {
			return nil
		}

		pos := &store.TraderPosition{
			TraderID:           t.traderID,
			ExchangeID:         t.exchangeID,
			ExchangeType:       t.exchangeType,
			ExchangePositionID: "paper_" + o.orderID,
			Symbol:             o.symbol,
			Side:               o.positionSide,
			Quantity:           o.qty,
			EntryQuantity:      o.qty,
			EntryPrice:         execPrice,
			EntryOrderID:       o.orderID,
			EntryTime:          fillTime,
			Leverage:           o.leverage,
			Status:             "OPEN",
			Source:             "system",
			Fee:                fee, // entry fee
		}
		return posStore.CreateOpenPosition(pos)
	}

	if strings.HasPrefix(action, "close_") {
		pos, err := posStore.GetOpenPositionBySymbol(t.traderID, o.symbol, o.positionSide)
		if err != nil {
			return err
		}
		if pos == nil {
			return nil
		}

		realized := 0.0
		if o.positionSide == "LONG" {
			realized = (execPrice - pos.EntryPrice) * o.qty
		} else {
			realized = (pos.EntryPrice - execPrice) * o.qty
		}
		realized = math.Round(realized*100) / 100

		totalFee := pos.Fee + fee

		closeReason := reason
		if closeReason == "" {
			closeReason = "ai_decision"
		}
		// Normalize to expected close_reason labels
		switch closeReason {
		case "stop_loss", "take_profit", "ai_decision", "manual":
		default:
			closeReason = "ai_decision"
		}

		// When closing, also clear stored protections (avoid re-trigger).
		_ = t.CancelStopOrders(o.symbol)
		return posStore.ClosePositionWithAccurateData(pos.ID, execPrice, o.orderID, fillTime, realized, totalFee, closeReason)
	}

	return nil
}

func (t *PaperTrader) scanStopTakeTriggers() error {
	positions, err := t.getOpenPositions()
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	// Build a set of currently pending close orders to avoid duplicates.
	pendingClose := make(map[string]bool) // SYMBOL:POSITION_SIDE
	rows, err := t.st.DB().Query(`
		SELECT symbol, position_side, order_action
		FROM trader_orders
		WHERE trader_id = ? AND exchange_id = ? AND exchange_type = ? AND status = 'NEW'
	`, t.traderID, t.exchangeID, t.exchangeType)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sym, side, action string
			if err := rows.Scan(&sym, &side, &action); err == nil {
				if strings.HasPrefix(action, "close_") {
					pendingClose[sym+":"+side] = true
				}
			}
		}
	}

	nowMs := time.Now().UnixMilli()

	for _, p := range positions {
		key := p.Symbol + ":" + strings.ToUpper(p.Side)
		if pendingClose[key] {
			continue
		}

		t.protectionsMu.RLock()
		prot, ok := t.protections[key]
		t.protectionsMu.RUnlock()
		if !ok || (prot.StopLoss <= 0 && prot.TakeProfit <= 0) {
			continue
		}

		// Fetch last 2-3 bars and only scan each completed bar once.
		klines, err := t.apiClient.GetKlines(p.Symbol, t.primaryTimeframe, 3)
		if err != nil || len(klines) == 0 {
			continue
		}

		// Find latest completed bar.
		var lastClosedIdx = -1
		for i := len(klines) - 1; i >= 0; i-- {
			if klines[i].CloseTime < nowMs {
				lastClosedIdx = i
				break
			}
		}
		if lastClosedIdx < 0 {
			continue
		}
		bar := klines[lastClosedIdx]

		// Only scan a given completed bar once per position.
		if last, ok := t.lastTriggerScan[key]; ok && last == bar.CloseTime {
			continue
		}
		t.lastTriggerScan[key] = bar.CloseTime

		triggered := ""
		side := strings.ToUpper(p.Side)

		// Conservative tie-break: stop_loss first if both hit in same bar.
		if prot.StopLoss > 0 {
			if side == "LONG" && bar.Low <= prot.StopLoss {
				triggered = "stop_loss"
			}
			if side == "SHORT" && bar.High >= prot.StopLoss {
				triggered = "stop_loss"
			}
		}
		if triggered == "" && prot.TakeProfit > 0 {
			if side == "LONG" && bar.High >= prot.TakeProfit {
				triggered = "take_profit"
			}
			if side == "SHORT" && bar.Low <= prot.TakeProfit {
				triggered = "take_profit"
			}
		}
		if triggered == "" {
			continue
		}

		// Enqueue a close order that will fill at the next bar open (OPN).
		closeAction := "close_long"
		sideStr := "SELL"
		posSide := "LONG"
		if side == "SHORT" {
			closeAction = "close_short"
			sideStr = "BUY"
			posSide = "SHORT"
		}

		openMs := bar.CloseTime + 1
		orderID := strconv.FormatInt(time.Now().UnixNano(), 10)
		clientOrderID := fmt.Sprintf("paper_opn:%d;reason=%s", openMs, triggered)

		order := &store.TraderOrder{
			TraderID:        t.traderID,
			ExchangeID:      t.exchangeID,
			ExchangeType:    t.exchangeType,
			ExchangeOrderID: orderID,
			ClientOrderID:   clientOrderID,
			Symbol:          p.Symbol,
			Side:            sideStr,
			PositionSide:    posSide,
			Type:            "MARKET",
			TimeInForce:     "GTC",
			Quantity:        p.Quantity,
			Price:           0,
			Status:          "NEW",
			Leverage:        p.Leverage,
			ReduceOnly:      true,
			ClosePosition:   true,
			OrderAction:     closeAction,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		if err := t.st.Order().CreateOrder(order); err == nil {
			logger.Infof("🛡 paper: queued %s for %s %s (OPN=%d)", triggered, p.Symbol, posSide, openMs)
		}
	}

	return nil
}

func (t *PaperTrader) getOpenPriceAt(symbol string, openMs int64) (float64, error) {
	// Try from recent klines first.
	klines, err := t.apiClient.GetKlines(symbol, t.primaryTimeframe, 5)
	if err == nil {
		for _, k := range klines {
			if k.OpenTime == openMs {
				return k.Open, nil
			}
		}
	}

	// Fallback: query a tight range around openMs.
	tfDur, err := market.TFDuration(t.primaryTimeframe)
	if err != nil {
		return 0, err
	}
	start := time.UnixMilli(openMs).UTC()
	end := start.Add(tfDur)
	rangeK, err := market.GetKlinesRange(symbol, t.primaryTimeframe, start, end)
	if err != nil {
		return 0, err
	}
	for _, k := range rangeK {
		if k.OpenTime == openMs {
			return k.Open, nil
		}
	}
	return 0, fmt.Errorf("open price not available at %d", openMs)
}

func (t *PaperTrader) applySlippage(price float64, positionSide string, isOpen bool) float64 {
	if t.slippageBps <= 0 {
		return price
	}
	rate := t.slippageBps / 10000.0
	adjust := 1.0
	if strings.ToUpper(positionSide) == "LONG" {
		if isOpen {
			adjust += rate
		} else {
			adjust -= rate
		}
	} else {
		if isOpen {
			adjust -= rate
		} else {
			adjust += rate
		}
	}
	return price * adjust
}

func (t *PaperTrader) orderSideForAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

func (t *PaperTrader) getOpenPositionEntry(symbol string, positionSide string) (float64, error) {
	pos, err := t.st.Position().GetOpenPositionBySymbol(t.traderID, symbol, positionSide)
	if err != nil || pos == nil {
		return 0, err
	}
	return pos.EntryPrice, nil
}
