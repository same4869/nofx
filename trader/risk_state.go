package trader

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"time"
)

// riskState is persisted to system_config so risk gates survive restarts.
// It is intentionally small and forward-compatible.
type riskState struct {
	Version int `json:"version"`

	TraderID   string `json:"trader_id"`
	ExchangeID string `json:"exchange_id"`

	// DailyDate is the UTC date for the daily-loss circuit, formatted as YYYY-MM-DD.
	DailyDate        string  `json:"daily_date"`
	DailyStartEquity float64 `json:"daily_start_equity"`
	DailyRealizedPnL float64 `json:"daily_realized_pnl"`

	// EquityPeak is the peak equity observed since state creation (used for drawdown circuit).
	EquityPeak float64 `json:"equity_peak"`

	// StopUntil is RFC3339 in UTC; when in the future, trading is paused.
	StopUntil  string `json:"stop_until"`
	StopReason string `json:"stop_reason"`

	UpdatedAt string `json:"updated_at"`
}

func riskStateKey(traderID, exchangeID string) string {
	// ExchangeID is included to avoid collisions when the same trader_id is reused.
	return fmt.Sprintf("risk_state:%s:%s", traderID, exchangeID)
}

func (at *AutoTrader) loadRiskState() {
	if at == nil || at.store == nil {
		return
	}
	key := riskStateKey(at.id, at.exchangeID)
	raw, err := at.store.GetSystemConfig(key)
	if err != nil || raw == "" {
		return
	}

	var rs riskState
	if err := json.Unmarshal([]byte(raw), &rs); err != nil {
		logger.Infof("⚠️ [%s] Failed to parse risk state, ignoring: %v", at.name, err)
		return
	}

	at.riskStateMu.Lock()
	at.riskState = rs
	at.riskStateMu.Unlock()

	// Restore stopUntil if valid.
	if rs.StopUntil != "" {
		if t, err := time.Parse(time.RFC3339, rs.StopUntil); err == nil {
			at.stopUntil = t
		}
	}
}

func (at *AutoTrader) saveRiskState() {
	if at == nil || at.store == nil {
		return
	}
	key := riskStateKey(at.id, at.exchangeID)

	at.riskStateMu.RLock()
	rs := at.riskState
	at.riskStateMu.RUnlock()

	rs.Version = 1
	rs.TraderID = at.id
	rs.ExchangeID = at.exchangeID
	rs.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	b, err := json.Marshal(rs)
	if err != nil {
		return
	}
	_ = at.store.SetSystemConfig(key, string(b))
}

func utcDayString(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
