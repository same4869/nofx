package market

import (
	"os"
	"strings"
	"sync"
)

type KlineSource string

const (
	KlineSourceAuto   KlineSource = "auto"   // prefer Binance, fall back to CoinAnk
	KlineSourceBinance KlineSource = "binance"
	KlineSourceCoinAnk KlineSource = "coinank"
)

var (
	klineSourceOnce sync.Once
	klineSource     KlineSource
)

func MarketKlineSource() KlineSource {
	klineSourceOnce.Do(func() {
		v := strings.TrimSpace(strings.ToLower(os.Getenv("MARKET_KLINE_SOURCE")))
		switch v {
		case string(KlineSourceBinance):
			klineSource = KlineSourceBinance
		case string(KlineSourceCoinAnk):
			klineSource = KlineSourceCoinAnk
		default:
			klineSource = KlineSourceAuto
		}
	})
	return klineSource
}

