package utils

import (
	"move_profit/log"
	"strings"
)

func InArrayString(val string, arr []string) bool {
	if len(arr) <= 0 {
		return false
	}

	for _, v := range arr {
		if v == val {
			return true
		}
	}

	return false
}

func InArray(val int, arr []int) bool {
	if len(arr) <= 0 {
		return false
	}

	for _, v := range arr {
		if v == val {
			return true
		}
	}

	return false
}

// BTCUSDT->BTC_USDT
func Trans2GateMarket(market string) string {
	arr := strings.Split(market, "USDT")
	if len(arr) != 2 {
		log.Log.Errorf("market:%s transMarket err", market)
		return ""
	}
	return arr[0] + "_USDT"
}

// BTC_USDT -> BTCUSDT
func Trans2BinancecMarket(market string) string {
	arr := strings.Split(market, "_USDT")
	if len(arr) != 2 {
		return ""
	}
	return arr[0] + "USDT"
}
