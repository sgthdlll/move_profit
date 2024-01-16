package main

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/op/go-logging"
	"github.com/shopspring/decimal"
	"os"
	"os/signal"
	"quant/binance_api"
	"quant/log"
	"quant/ws"
	"strings"
	"sync"
	"syscall"
	"time"
)

type TEngineType string

const (
	EngineTypeSpot        TEngineType = "spot"
	EngineTypeFuturesUsdt TEngineType = "futures_usdt"
	EngineTypeFuturesUsd  TEngineType = "futures_usd"
	EngineTypeFuturesBtc  TEngineType = "futures_btc"
)

var Log, ErrLog *logging.Logger
var quitChan chan struct{}
var wg sync.WaitGroup
var binanceLastPriceMap sync.Map
var count2Taker = 0
var countTakerAndMaker = 0
var count2Maker = 0

func initLog() {
	Log = log.New("./logs/move_profit.log", "DEBUG")
	ErrLog = log.New("./logs/copy_trading_err.log", "DEBUG")
}

type binanceTransHistoryResp struct {
	Rows  []*binanceTransHistory `json:"rows"`
	Total int                    `json:"total"`
}

type binanceTransHistory struct {
	Asset     string `json:"asset"`
	TranId    int    `json:"tranId"`
	Amount    string `json:"amount"`
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status"`
}

func main() {
	initLog()
	binance_api.InitBinanceApi("02rw4kB2Lla22hGzFEkD77Cxnm55ogQYeZk5hthXmfRUM2NuyVYBRMCRcL6tb0nd", "arMz2bClKB0F3nekZc8JNIw2YBZ1ONpxfaOhKRJyMPceyLBEZcawauYXc9kNwJz5")
	//processPushMsg("", "")
	binance_api.BinanceApiClient.Order("BTC_USDT", "0.01", "BUY")
	////signalQuit()
	//AsyncProcessBinancePubChan()
	//go ws.GateTicker()
	//select {}
}

func signalQuit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		Log.Infof("notify get a signal %s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			close(quitChan)
			wg.Wait()
			return
		case syscall.SIGHUP:
		default:
			close(quitChan)
			wg.Wait()
			return
		}
	}
}

func processPushMsg(apiKey, secretKey string) {
	server, err := ws.NewWsService(quitChan, Log, &ws.ConnConf{
		UserId:                   123,
		ApiUrl:                   "https://fapi.binance.com",
		URL:                      "wss://fstream.binance.com/ws",
		Key:                      apiKey,
		Secret:                   secretKey,
		IsOpenPrivacyWs:          true,
		IsOpenPublicWs:           false,
		PublicChanLen:            5000,
		PrivacyChanLen:           5000,
		ListenKeyRefreshInterval: "58m50s",
	})
	if err != nil {
		Log.Errorf("init binance ws err:%+v", err)
		return
	}

	initChan := make(chan struct{})
	go func(server *ws.WsService, initChan chan struct{}) {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				Log.Errorf("handler err:%+v", err)
				return
			}

			server.Start(initChan)
		}()
	}(server, initChan)

	select {
	case <-initChan:
	case <-time.After(time.Second * 60):
		Log.Error("ws service init timeout")
		return
	}
	var pri <-chan []byte
	// 获取输出 chan
	pri, err = server.GetPrivacyMsgChan()
	if err != nil {
		Log.Errorf("GetPrivacyMsgChan err:%+v", err)
		return
	}
	var restartChan chan int
	restartChan, err = server.GetRestartMsgChan()
	if err != nil {
		Log.Errorf("GetRestartMsgChan err:%+v", err)
		return
	}

	for {
		select {
		case <-quitChan:
			Log.Infof("binance ws push closed user_id:%d", 123)
			return
		case msgBytes := <-pri:
			Log.Infof("binance [ProcessBinancePositionChange]userId:%d,msg:%s", 123, string(msgBytes))
			//e.processBinancePriMsg(msgBytes)
		case <-restartChan:
			Log.Infof("binance restartFixBinanceLossPositonSignal user_id:%d", 123)
			//第一次启动、断线重连需要拉取一次仓位信息，生成一次仓位信号
			//e.restartFixBinanceLossPositonSignal()
		}
	}
}

func AsyncProcessBinancePubChan() {
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				Log.Errorf("handler err:%+v", r)
				return
			}
		}()

		processBinancePubChan()
	}()
}

func processBinancePubChan() {
	server, err := ws.NewWsService(quitChan, Log, &ws.ConnConf{
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
	wg.Add(1)
	go func(server *ws.WsService, initChan chan struct{}) {
		defer func() {
			wg.Done()
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

	err = server.WriteSubscribeMsg(ws.SubscribeMsgRequest{
		Method: "SUBSCRIBE",
		Params: []interface{}{"!ticker@arr"},
	})
	if err != nil {
		return
	}

	for {
		select {
		case <-quitChan:
			Log.Infof("binance pub chan closed")
			return
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
		Log.Errorf("binance pase msg:[%s] err:[%+v]", string(msgBytes), err)
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
			return
		}
		gatePriceD := gatePrice.(decimal.Decimal)
		diff := price.Sub(gatePriceD).Abs()
		diffRate := diff.Div(price)
		bothTaker, _ := decimal.NewFromString("0.00091")
		takerAndMaker, _ := decimal.NewFromString("0.00065")
		bothMaker, _ := decimal.NewFromString("0.00033")
		msg := fmt.Sprintf("市场:%s gate市价:%+v binance市价:%+v 价差:%+v 价差比例:%+v%s", market, gatePriceD, price, diff, diffRate.Mul(decimal.NewFromInt(100)).Truncate(5), "%")
		if diffRate.GreaterThanOrEqual(bothTaker) {
			count2Taker++
			Log.Infof("both taker:%s ,count:%d", msg, count2Taker)
		} else if diffRate.GreaterThanOrEqual(takerAndMaker) {
			countTakerAndMaker++
			//Log.Infof("taker & maker:%s,count:%d", msg, countTakerAndMaker)
		} else if diffRate.GreaterThanOrEqual(bothMaker) {
			count2Maker++
			//Log.Infof("both maker:%s,count:%d", msg, count2Maker)
		}
	}
}

func getMarketKey(market string, engineType TEngineType) string {
	return fmt.Sprintf("%s_%s", market, engineType)
}

// BTCUSDT->BTC_USDT
func trans2GateMarket(market string) string {
	arr := strings.Split(market, "USDT")
	if len(arr) != 2 {
		Log.Errorf("market:%s transMarket err", market)
		return ""
	}
	return arr[0] + "_USDT"
}
