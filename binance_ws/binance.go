package binance_ws

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/shopspring/decimal"
	ws "move_profit/gate_ws"
	"move_profit/log"
	"strings"
	"sync"
	"time"
)

var binanceLastPriceMap sync.Map
var count2Taker = 0

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
		price, _ := decimal.NewFromString(binancePrice)
		if !price.IsPositive() {
			return
		}
		market := trans2GateMarket(binanceMarket)
		binanceLastPriceMap.Store(market, price)
		gatePrice, ok := ws.GateLastPriceMap.Load(market)
		if !ok {
			continue
		}
		gatePriceD := gatePrice.(decimal.Decimal)
		diff := price.Sub(gatePriceD).Abs()
		diffRate := diff.Div(price)
		bothTaker, _ := decimal.NewFromString("0.001")
		fee := bothTaker.Mul(decimal.NewFromInt(2))
		msg := fmt.Sprintf("市场:%s gate市价:%+v binance市价:%+v 价差:%+v 价差比例:%+v%s", market, gatePriceD, price, diff, diffRate.Mul(decimal.NewFromInt(100)).Truncate(5), "%")
		if diffRate.LessThan(fee) {
			continue
		}
		count2Taker++
		log.Log.Infof("%s ,count:%d", msg, count2Taker)
	}
}

// BTCUSDT->BTC_USDT
func trans2GateMarket(market string) string {
	arr := strings.Split(market, "USDT")
	if len(arr) != 2 {
		log.Log.Errorf("market:%s transMarket err", market)
		return ""
	}
	return arr[0] + "_USDT"
}
