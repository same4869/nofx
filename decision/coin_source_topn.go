package decision

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	binanceFutures24hURL = "https://fapi.binance.com/fapi/v1/ticker/24hr"
)

type binance24hTicker struct {
	Symbol      string `json:"symbol"`
	QuoteVolume string `json:"quoteVolume"`
}

type topnUniverseState struct {
	updatedAt time.Time
	symbols   []string
}

var (
	topnMu sync.Mutex

	topnLastFetch time.Time
	topnRank      map[string]int     // symbol -> 1-based rank by quoteVolume
	topnSorted     []string          // sorted desc by quoteVolume
	topnState      topnUniverseState // with hysteresis/dwell applied
)

func defaultTopNParams(n, refreshMins, hysteresisExtra, minDwellMins int) (int, time.Duration, int, time.Duration) {
	if n <= 0 {
		n = 30
	}
	if refreshMins <= 0 {
		refreshMins = 30
	}
	if hysteresisExtra <= 0 {
		hysteresisExtra = 15
	}
	if minDwellMins <= 0 {
		minDwellMins = 360
	}
	return n, time.Duration(refreshMins) * time.Minute, hysteresisExtra, time.Duration(minDwellMins) * time.Minute
}

// getBinanceTopNUniverse returns Binance USDT perpetual symbols ranked by 24h quoteVolume (rolling 24h),
// then applies hysteresis + min-dwell to reduce churn.
func getBinanceTopNUniverse(n, refreshMins, hysteresisExtra, minDwellMins int) ([]string, error) {
	n, refreshEvery, hysteresisExtra, minDwell := defaultTopNParams(n, refreshMins, hysteresisExtra, minDwellMins)

	topnMu.Lock()
	defer topnMu.Unlock()

	// Refresh ranking if stale.
	if topnRank == nil || time.Since(topnLastFetch) >= refreshEvery {
		if err := fetchBinance24hRankingLocked(); err != nil {
			// If we have any cached state, degrade gracefully.
			if len(topnState.symbols) > 0 {
				logger.Infof("⚠️ topn: fetch failed, using previous universe (%d): %v", len(topnState.symbols), err)
				return append([]string{}, topnState.symbols...), nil
			}
			return nil, err
		}
	}

	// Apply min-dwell: keep existing universe unchanged if still within dwell window.
	if len(topnState.symbols) > 0 && time.Since(topnState.updatedAt) < minDwell {
		return append([]string{}, topnState.symbols...), nil
	}

	// Build next universe with hysteresis.
	next := make([]string, 0, n)
	seen := make(map[string]bool)

	// Keep existing symbols if still within N+hysteresisExtra.
	keepRank := n + hysteresisExtra
	for _, sym := range topnState.symbols {
		if r, ok := topnRank[sym]; ok && r > 0 && r <= keepRank {
			next = append(next, sym)
			seen[sym] = true
		}
	}

	// Fill remaining from TopN.
	for _, sym := range topnSorted {
		if len(next) >= n {
			break
		}
		if seen[sym] {
			continue
		}
		next = append(next, sym)
		seen[sym] = true
	}

	// Force include BTC/ETH (as per personal defaults).
	for _, must := range []string{"BTCUSDT", "ETHUSDT"} {
		if seen[must] {
			continue
		}
		if len(next) < n {
			next = append(next, must)
			seen[must] = true
			continue
		}
		// Replace last element if full.
		if len(next) > 0 {
			next[len(next)-1] = must
		}
		seen[must] = true
	}

	// Normalize symbols for consistency.
	for i := range next {
		next[i] = market.Normalize(next[i])
	}

	topnState.symbols = next
	topnState.updatedAt = time.Now()
	return append([]string{}, next...), nil
}

func fetchBinance24hRankingLocked() error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(binanceFutures24hURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binance 24hr api status %d: %s", resp.StatusCode, string(body))
	}

	var raw []binance24hTicker
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}

	// Build list with parsed quoteVolume.
	type item struct {
		sym   string
		qv    float64
	}
	items := make([]item, 0, len(raw))
	for _, t := range raw {
		sym := strings.ToUpper(strings.TrimSpace(t.Symbol))
		if !strings.HasSuffix(sym, "USDT") {
			continue
		}
		// Filter out delivery contracts like BTCUSDT_250627
		if strings.Contains(sym, "_") {
			continue
		}
		qv, err := strconv.ParseFloat(t.QuoteVolume, 64)
		if err != nil {
			continue
		}
		if qv <= 0 {
			continue
		}
		items = append(items, item{sym: sym, qv: qv})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].qv > items[j].qv
	})

	topnRank = make(map[string]int, len(items))
	topnSorted = make([]string, 0, len(items))
	for i, it := range items {
		if _, exists := topnRank[it.sym]; exists {
			continue
		}
		topnRank[it.sym] = i + 1
		topnSorted = append(topnSorted, it.sym)
	}
	topnLastFetch = time.Now()
	return nil
}

