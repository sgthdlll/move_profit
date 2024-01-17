package binance_ws

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/shopspring/decimal"
	"move_profit/binance_api"
	"move_profit/gate_api"
	gate_ws "move_profit/gate_ws"
	"move_profit/log"
	"move_profit/utils"
	"sync"
	"time"
)

var binanceLastPriceMap sync.Map
var count2Taker = 0

var fishingChan = make(chan int)

func AsyncProcessBinancePubChan() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Log.Errorf("handler err:%+v", r)
				return
			}
		}()

		processBinancePubChan()
	}()
}

func processBinancePubChan() {
	server, err := NewWsService(log.Log, &ConnConf{
		ApiUrl:                   "https://fapi.binance.com",
		URL:                      "wss://fstream.binance.com/ws",
		IsOpenPublicWs:           true,
		PublicChanLen:            5000,
		ListenKeyRefreshInterval: "58m50s",
	})
	if err != nil {
		return
	}

	initChan := make(chan struct{})
	go func(server *WsService, initChan chan struct{}) {
		defer func() {
			if r := recover(); r != nil {
				return
			}

			server.Start(initChan)
		}()
	}(server, initChan)

	select {
	case <-initChan:
	case <-time.After(time.Second * 60):
		return
	}
	var pub <-chan []byte
	pub, err = server.GetPublicMsgChan()
	if err != nil {
		return
	}

	err = server.WriteSubscribeMsg(SubscribeMsgRequest{
		Method: "SUBSCRIBE",
		Params: []interface{}{"!ticker@arr"},
	})
	if err != nil {
		return
	}

	for {
		select {
		case msgBytes := <-pub:
			processPubMsg(msgBytes)
		}
	}
}

func processPubMsg(msgBytes []byte) {
	//{"e":"24hrMiniTicker","E":1702530188424,"s":"BTCUSDT","c":"42731.50","o":"40971.90","h":"43517.60","l":"40812.90","v":"360594.073","q":"15202373572.10"}
	/**
	{
	   "e": "24hrMiniTicker",  // 事件类型
	   "E": 123456789,         // 事件时间(毫秒)
	   "s": "BNBUSDT",          // 交易对
	   "c": "0.0025",          // 最新成交价格
	   "o": "0.0010",          // 24小时前开始第一笔成交价格
	   "h": "0.0025",          // 24小时内最高成交价
	   "l": "0.0010",          // 24小时内最低成交价
	   "v": "10000",           // 成交量
	   "q": "18"               // 成交额
	 }
	*/
	data, err := simplejson.NewJson(msgBytes)
	if err != nil {
		log.Log.Errorf("binance pase msg:[%s] err:[%+v]", string(msgBytes), err)
		return
	}
	tickerList, _ := data.Array()
	for i := 0; i < len(tickerList); i++ {
		ticker := data.GetIndex(i)
		binanceMarket, _ := ticker.Get("s").String()
		binancePrice, _ := ticker.Get("c").String()
		binancePriceD, _ := decimal.NewFromString(binancePrice)
		if !binancePriceD.IsPositive() {
			return
		}
		market := utils.Trans2GateMarket(binanceMarket)
		binanceLastPriceMap.Store(market, binancePriceD)
		gatePrice, ok := gate_ws.GateLastPriceMap.Load(market)
		if !ok {
			continue
		}
		gatePriceD := gatePrice.(decimal.Decimal)
		diff := binancePriceD.Sub(gatePriceD).Abs()
		diffRate := diff.Div(binancePriceD)
		bothTaker, _ := decimal.NewFromString("0.001")

		fee := bothTaker.Mul(decimal.NewFromInt(2))
		msg := fmt.Sprintf("市场:%s gate市价:%+v binance市价:%+v 价差:%+v 价差比例:%+v%s", market, gatePriceD, binancePriceD, diff, diffRate.Mul(decimal.NewFromInt(100)).Truncate(5), "%")
		stopDiff, _ := decimal.NewFromString("0.0001")
		if diffRate.LessThan(stopDiff) {
			//出现平仓信号，判断是否有仓位可平仓
			log.Log.Infof("[close position]%s", msg)
		}

		if len(fishingChan) >= 1 {
			continue
		}
		if diffRate.LessThan(fee) {
			continue
		}
		gateMarketInfo, ok := gate_api.GetMarketInfo(market)
		if !ok {
			continue
		}

		quantoMultiplier, _ := decimal.NewFromString(gateMarketInfo.QuantoMultiplier)

		binanceMarketInfo, ok := binance_api.GetMarketInfo(market)
		if !ok {
			continue
		}
		size := decimal.NewFromInt(120).Div(binancePriceD).Truncate(int32(binanceMarketInfo.QuantityPrecision))
		sizeGate := size.Div(quantoMultiplier)
		count2Taker++
		log.Log.Infof("%s ,count:%d", msg, count2Taker)
		fmt.Println(fmt.Sprintf("size:%+v  size_gate:%+v", size, sizeGate))
		if gatePriceD.LessThan(binancePriceD) {
			//gate买单，binance卖单
			_, err = gate_api.PlaceExchagneOrder(market, int(sizeGate.IntPart()))
			if err != nil {
				continue
			}
			_, err = binance_api.BinanceApiClient.Order(market, size.String(), "SELL")
			if err != nil {
				continue
			}
		} else {
			//gate卖单，binance买单
			_, err = gate_api.PlaceExchagneOrder(market, int(sizeGate.Mul(decimal.NewFromInt(-1)).IntPart()))
			if err != nil {
				continue
			}
			_, err = binance_api.BinanceApiClient.Order(market, size.String(), "BUY")
			if err != nil {
				continue
			}
		}
		fishingChan <- 1
	}
}
