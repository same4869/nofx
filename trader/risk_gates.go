package trader

import (
	"encoding/json"
	"fmt"
	"nofx/store"
	"time"
)

type accountRiskSnapshot struct {
	Type string `json:"type"`

	TraderID   string `json:"trader_id"`
	ExchangeID string `json:"exchange_id"`
	TimeUTC    string `json:"time_utc"`

	Equity        float64 `json:"equity"`
	MarginUsedPct float64 `json:"margin_used_pct"`

	DailyDate        string  `json:"daily_date"`
	DailyStartEquity float64 `json:"daily_start_equity"`
	DailyLossPct     float64 `json:"daily_loss_pct"`
	DailyRealizedPnL float64 `json:"daily_realized_pnl"`

	EquityPeak   float64 `json:"equity_peak"`
	DrawdownPct  float64 `json:"drawdown_pct"`
	StopUntilUTC string  `json:"stop_until_utc,omitempty"`
	StopReason   string  `json:"stop_reason,omitempty"`

	Limits struct {
		MaxMarginUsage  float64 `json:"max_margin_usage"`
		DailyLossLimit  float64 `json:"daily_loss_limit"`
		MaxDrawdown     float64 `json:"max_drawdown"`
		CooldownMinutes int     `json:"cooldown_minutes"`
	} `json:"limits"`
}

func (at *AutoTrader) updateAndEnforceAccountRisk(record *store.DecisionRecord, equity float64, marginUsedPct float64) (bool, string) {
	if at == nil || at.config.StrategyConfig == nil {
		return false, ""
	}

	rc := at.config.StrategyConfig.RiskControl

	maxMarginUsage := rc.MaxMarginUsage
	if maxMarginUsage <= 0 {
		maxMarginUsage = 0.9
	}

	dailyLossLimit := rc.DailyLossLimit
	maxDrawdown := rc.MaxDrawdown
	cooldownMins := rc.StopCooldownMinutes
	if cooldownMins <= 0 {
		cooldownMins = 360
	}

	now := time.Now().UTC()
	today := utcDayString(now)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	at.riskStateMu.Lock()
	rs := at.riskState

	if rs.DailyDate != today || rs.DailyStartEquity <= 0 {
		rs.DailyDate = today
		rs.DailyStartEquity = equity
		rs.DailyRealizedPnL = 0
	}

	// Update realized daily PnL from DB (restart-safe).
	if at.store != nil {
		if pnl, err := at.store.Position().SumRealizedPnLSince(at.id, at.exchangeID, dayStart); err == nil {
			rs.DailyRealizedPnL = pnl
		}
	}

	if rs.EquityPeak <= 0 {
		rs.EquityPeak = equity
	}
	if equity > rs.EquityPeak {
		rs.EquityPeak = equity
	}

	// Compute drawdown and daily loss.
	drawdownPct := 0.0
	if rs.EquityPeak > 0 {
		drawdownPct = (rs.EquityPeak - equity) / rs.EquityPeak
		if drawdownPct < 0 {
			drawdownPct = 0
		}
	}
	dailyLossPct := 0.0
	if rs.DailyStartEquity > 0 {
		dailyLossPct = (rs.DailyStartEquity - equity) / rs.DailyStartEquity
	}

	// Circuit break checks (disabled when limit <= 0).
	triggered := false
	reason := ""
	if dailyLossLimit > 0 && dailyLossPct >= dailyLossLimit {
		triggered = true
		reason = fmt.Sprintf("daily_loss_limit breached: %.2f%% >= %.2f%%", dailyLossPct*100, dailyLossLimit*100)
	}
	if maxDrawdown > 0 && drawdownPct >= maxDrawdown {
		triggered = true
		if reason != "" {
			reason += "; "
		}
		reason += fmt.Sprintf("max_drawdown breached: %.2f%% >= %.2f%%", drawdownPct*100, maxDrawdown*100)
	}

	// Optional: hard pause when margin usage exceeds limit (limit is CODE ENFORCED on opens as well).
	if maxMarginUsage > 0 && marginUsedPct/100.0 > maxMarginUsage {
		triggered = true
		if reason != "" {
			reason += "; "
		}
		reason += fmt.Sprintf("max_margin_usage breached: %.2f%% > %.2f%%", marginUsedPct, maxMarginUsage*100)
	}

	if triggered {
		until := now.Add(time.Duration(cooldownMins) * time.Minute)
		if !at.stopUntil.IsZero() && at.stopUntil.After(until) {
			until = at.stopUntil
		}
		at.stopUntil = until
		rs.StopUntil = until.Format(time.RFC3339)
		rs.StopReason = reason
	}

	// Mirror daily PnL to legacy fields used by APIs.
	at.dailyPnL = rs.DailyRealizedPnL
	at.lastResetTime = dayStart

	at.riskState = rs
	at.riskStateMu.Unlock()

	// Persist only when we have a store.
	at.saveRiskState()

	// Record an explainable snapshot to execution_log.
	if record != nil {
		snap := accountRiskSnapshot{
			Type:             "risk_snapshot",
			TraderID:         at.id,
			ExchangeID:       at.exchangeID,
			TimeUTC:          now.Format(time.RFC3339),
			Equity:           equity,
			MarginUsedPct:    marginUsedPct,
			DailyDate:        today,
			DailyStartEquity: rs.DailyStartEquity,
			DailyLossPct:     dailyLossPct,
			DailyRealizedPnL: rs.DailyRealizedPnL,
			EquityPeak:       rs.EquityPeak,
			DrawdownPct:      drawdownPct,
		}
		snap.Limits.MaxMarginUsage = maxMarginUsage
		snap.Limits.DailyLossLimit = dailyLossLimit
		snap.Limits.MaxDrawdown = maxDrawdown
		snap.Limits.CooldownMinutes = cooldownMins
		if triggered {
			snap.StopUntilUTC = at.stopUntil.UTC().Format(time.RFC3339)
			snap.StopReason = reason
		}
		if b, err := json.Marshal(snap); err == nil {
			record.ExecutionLog = append(record.ExecutionLog, "RISK:"+string(b))
		}
	}

	if triggered {
		return true, reason
	}
	return false, ""
}
