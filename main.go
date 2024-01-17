package main

import (
	"move_profit/binance_api"
	"move_profit/binance_ws"
	"move_profit/gate_api"
	"move_profit/gate_ws"
	"move_profit/log"
)

func main() {
	log.InitLog()
	binance_api.InitBinanceApi("02rw4kB2Lla22hGzFEkD77Cxnm55ogQYeZk5hthXmfRUM2NuyVYBRMCRcL6tb0nd", "arMz2bClKB0F3nekZc8JNIw2YBZ1ONpxfaOhKRJyMPceyLBEZcawauYXc9kNwJz5")
	binance_api.BinanceApiClient.SwitchPositionMode()
	gate_api.SwitchPositionMode()

	gate_api.InitGateClient()
	binance_ws.AsyncProcessBinancePubChan()

	go gate_ws.GateTicker()
	select {}

	//quantoMultiplier := ws.GetGateMarketQuantoMultiplier("BTC_USDT")
	//if quantoMultiplier.IsZero() {
	//	return
	//}
	//size, _ := decimal.NewFromString("0.01")
	//binanceSize := size.String()
	//gateSize := size.Div(quantoMultiplier).IntPart()
	//pre1 设置持仓模式为单向持仓
	//设置binance持仓模式为单向持仓

	//pre2 设置小币种为5倍杠杆
	//pre3 设置大币种为10倍杠杆

	//1检测到价差大于双倍taker手续费
	//2暂定用120U计算下单数量
	//3低价交易所挂多单，高价交易所挂卖单
	//4当交易所没有价差的时候，双腿平仓
	//fmt.Println(binanceSize)
	//fmt.Println(gateSize)
	//gate_api.PlaceExchagneOrder("BTC_USDT", 1)

	////processPushMsg("", "")
	//binance_api.BinanceApiClient.Order("BTC_USDT", "0.01", "SELL")
	////signalQuit()
	//AsyncProcessBinancePubChan()

	//select {}
}
