package trader

import (
	"encoding/json"
	"nofx/store"
	"time"
)

type cycleMetrics struct {
	Type       string `json:"type"`
	TraderID   string `json:"trader_id"`
	Cycle      int    `json:"cycle"`
	TimeUTC    string `json:"time_utc"`
	Exchange   string `json:"exchange"`
	ExchangeID string `json:"exchange_id"`

	BuildContextMs int64 `json:"build_context_ms"`
	AIDecisionMs   int64 `json:"ai_decision_ms"`
	ExecuteMs      int64 `json:"execute_ms"`
	TotalMs        int64 `json:"total_ms"`

	CandidateCoins int `json:"candidate_coins"`
	MarketDataOK   int `json:"market_data_ok"`
	Decisions      int `json:"decisions"`
	Failures       int `json:"failures"`
}

func (at *AutoTrader) appendCycleMetrics(record *store.DecisionRecord, m cycleMetrics) {
	if record == nil {
		return
	}
	m.Type = "cycle_metrics"
	m.TraderID = at.id
	m.Exchange = at.exchange
	m.ExchangeID = at.exchangeID
	m.TimeUTC = time.Now().UTC().Format(time.RFC3339)

	if b, err := json.Marshal(m); err == nil {
		record.ExecutionLog = append(record.ExecutionLog, "METRIC:"+string(b))
	}
}
