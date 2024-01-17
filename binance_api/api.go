package binance_api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/levigross/grequests"
	"github.com/shopspring/decimal"
	"io"
	"move_profit/utils"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ApikeyInvalidError = errors.New("invalid apikey")
	apikeyInvalidCode  = -2015
	apikeyInvalidCode2 = -2014
)

type switchLeverageResp struct {
	Leverage         int    `json:"leverage"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	Symbol           string `json:"symbol"`
}

type errMsgResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type fapiTimeStampResp struct {
	ServerTime int64 `json:"serverTime"`
}

type fapiV2OpenOrderResp struct {
	AvgPrice                string `json:"avgPrice"`
	ClientOrderId           string `json:"clientOrderId"`
	CumQuote                string `json:"cumQuote"`
	ExecutedQty             string `json:"executedQty"`
	OrderId                 int    `json:"orderId"`
	OrigQty                 string `json:"origQty"`
	OrigType                string `json:"origType"`
	Price                   string `json:"price"`
	ReduceOnly              bool   `json:"reduceOnly"`
	Side                    string `json:"side"`
	PositionSide            string `json:"positionSide"`
	Status                  string `json:"status"`
	StopPrice               string `json:"stopPrice"`
	ClosePosition           bool   `json:"closePosition"`
	Symbol                  string `json:"symbol"`
	Time                    int64  `json:"time"`
	TimeInForce             string `json:"timeInForce"`
	Type                    string `json:"type"`
	ActivatePrice           string `json:"activatePrice"`
	PriceRate               string `json:"priceRate"`
	UpdateTime              int64  `json:"updateTime"`
	WorkingType             string `json:"workingType"`
	PriceProtect            bool   `json:"priceProtect"`
	PriceMatch              string `json:"priceMatch"`
	SelfTradePreventionMode string `json:"selfTradePreventionMode"`
	GoodTillDate            int    `json:"goodTillDate"`
}

type fapiV2AccountResp struct {
	FeeTier                     int                      `json:"feeTier"`
	CanTrade                    bool                     `json:"canTrade"`
	CanDeposit                  bool                     `json:"canDeposit"`
	CanWithdraw                 bool                     `json:"canWithdraw"`
	UpdateTime                  int                      `json:"updateTime"`
	MultiAssetsMargin           bool                     `json:"multiAssetsMargin"`
	TradeGroupId                int                      `json:"tradeGroupId"`
	TotalInitialMargin          string                   `json:"totalInitialMargin"`
	TotalMaintMargin            string                   `json:"totalMaintMargin"`
	TotalWalletBalance          string                   `json:"totalWalletBalance"`
	TotalUnrealizedProfit       string                   `json:"totalUnrealizedProfit"`
	TotalMarginBalance          string                   `json:"totalMarginBalance"`
	TotalPositionInitialMargin  string                   `json:"totalPositionInitialMargin"`
	TotalOpenOrderInitialMargin string                   `json:"totalOpenOrderInitialMargin"`
	TotalCrossWalletBalance     string                   `json:"totalCrossWalletBalance"`
	TotalCrossUnPnl             string                   `json:"totalCrossUnPnl"`
	AvailableBalance            string                   `json:"availableBalance"`
	MaxWithdrawAmount           string                   `json:"maxWithdrawAmount"`
	Assets                      []*fapiV2AccountAsset    `json:"assets"`
	Positions                   []*fapiV2AccountPosition `json:"positions"`
}

type fapiV2AccountAsset struct {
	Asset                  string `json:"asset"`
	WalletBalance          string `json:"walletBalance"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	MarginBalance          string `json:"marginBalance"`
	MaintMargin            string `json:"maintMargin"`
	InitialMargin          string `json:"initialMargin"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	CrossWalletBalance     string `json:"crossWalletBalance"`
	CrossUnPnl             string `json:"crossUnPnl"`
	AvailableBalance       string `json:"availableBalance"`
	MaxWithdrawAmount      string `json:"maxWithdrawAmount"`
	MarginAvailable        bool   `json:"marginAvailable"`
	UpdateTime             int64  `json:"updateTime"`
}

type fapiV2AccountPosition struct {
	Symbol                 string `json:"symbol"`
	InitialMargin          string `json:"initialMargin"`
	MaintMargin            string `json:"maintMargin"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	Leverage               string `json:"leverage"`
	Isolated               bool   `json:"isolated"`
	EntryPrice             string `json:"entryPrice"`
	MaxNotional            string `json:"maxNotional"`
	BidNotional            string `json:"bidNotional"`
	AskNotional            string `json:"askNotional"`
	PositionSide           string `json:"positionSide"`
	PositionAmt            string `json:"positionAmt"`
	UpdateTime             int    `json:"updateTime"`
}
type apiOrderReq struct {
	Symbol       string `json:"symbol"`       // 交易对
	Side         string `json:"side"`         //买卖方向 SELL, BUY
	PositionSide string `json:"positionSide"` //持仓方向 BOTH、LONG、SHORT
	//"MARKET",  // 市价单
	//"STOP", // 止损单
	//"STOP_MARKET", // 止损市价单
	//"TAKE_PROFIT", // 止盈单
	//"TAKE_PROFIT_MARKET", // 止盈暑市价单
	//"TRAILING_STOP_MARKET" // 跟踪止损市价单
	OrderType  string          `json:"type"`
	ReduceOnly string          `json:"reduceOnly"` //true, false; 非双开模式下默认false；双开模式下不接受此参数； 使用closePosition不支持此参数。
	Quantity   decimal.Decimal `json:"quantity"`
	Price      decimal.Decimal `json:"price"`
	MyOrderId  string          `json:"newClientOrderId"` // 用户自定义的订单号
	//true, false；触发后全部平仓，仅支持STOP_MARKET和TAKE_PROFIT_MARKET；不与quantity合用；自带只平仓效果，不与reduceOnly 合用
	IsClosePosition string `json:"closePosition"`
	// 有效方式
	//"GTC", // 成交为止, 一直有效
	//"IOC", // 无法立即成交(吃单)的部分就撤销
	//"FOK", // 无法全部立即成交就撤销
	//"GTX" // 无法成为挂单方就撤销
	TimeInForce string `json:"timeInForce"`
	Timestamp   int64  `json:"timestamp"`
	RecvWindow  int64  `json:"recvWindow"`
}

/*
*

	{
	    "clientOrderId": "testOrder", // 用户自定义的订单号
	    "cumQty": "0",
	    "cumQuote": "0", // 成交金额
	    "executedQty": "0", // 成交量
	    "orderId": 22542179, // 系统订单号
	    "avgPrice": "0.00000",  // 平均成交价
	    "origQty": "10", // 原始委托数量
	    "price": "0", // 委托价格
	    "reduceOnly": false, // 仅减仓
	    "side": "SELL", // 买卖方向
	    "positionSide": "SHORT", // 持仓方向
	    "status": "NEW", // 订单状态
	    "stopPrice": "0", // 触发价，对`TRAILING_STOP_MARKET`无效
	    "closePosition": false,   // 是否条件全平仓
	    "symbol": "BTCUSDT", // 交易对
	    "timeInForce": "GTD", // 有效方法
	    "type": "TRAILING_STOP_MARKET", // 订单类型
	    "origType": "TRAILING_STOP_MARKET",  // 触发前订单类型
	    "activatePrice": "9020", // 跟踪止损激活价格, 仅`TRAILING_STOP_MARKET` 订单返回此字段
	    "priceRate": "0.3", // 跟踪止损回调比例, 仅`TRAILING_STOP_MARKET` 订单返回此字段
	    "updateTime": 1566818724722, // 更新时间
	    "workingType": "CONTRACT_PRICE", // 条件价格触发类型
	    "priceProtect": false,            // 是否开启条件单触发保护
	    "priceMatch": "NONE",              //盘口价格下单模式
	    "selfTradePreventionMode": "NONE", //订单自成交保护模式
	    "goodTillDate": 1693207680000      //订单TIF为GTD时的自动取消时间
	}
*/
type apiOrderRsp struct {
	ClientOrderId           string `json:"clientOrderId"`
	CumQty                  string `json:"cumQty"`
	CumQuote                string `json:"cumQuote"`
	ExecutedQty             string `json:"executedQty"`
	OrderId                 int    `json:"orderId"`
	AvgPrice                string `json:"avgPrice"`
	OrigQty                 string `json:"origQty"`
	Price                   string `json:"price"`
	ReduceOnly              bool   `json:"reduceOnly"`
	Side                    string `json:"side"`
	PositionSide            string `json:"positionSide"`
	Status                  string `json:"status"`
	StopPrice               string `json:"stopPrice"`
	ClosePosition           bool   `json:"closePosition"`
	Symbol                  string `json:"symbol"`
	TimeInForce             string `json:"timeInForce"`
	Type                    string `json:"type"`
	OrigType                string `json:"origType"`
	ActivatePrice           string `json:"activatePrice"`
	PriceRate               string `json:"priceRate"`
	UpdateTime              int64  `json:"updateTime"`
	WorkingType             string `json:"workingType"`
	PriceProtect            bool   `json:"priceProtect"`
	PriceMatch              string `json:"priceMatch"`
	SelfTradePreventionMode string `json:"selfTradePreventionMode"`
	GoodTillDate            int64  `json:"goodTillDate"`
}

//"timeInForce": [ // 有效方式
//"GTC", // 成交为止, 一直有效
//"IOC", // 无法立即成交(吃单)的部分就撤销
//"FOK", // 无法全部立即成交就撤销
//"GTX" // 无法成为挂单方就撤销
//],

type apiV3TickerPriceResp struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type fapiV2BalanceResp struct {
	AccountAlias       string `json:"accountAlias"`
	Asset              string `json:"asset"`
	Balance            string `json:"balance"`
	CrossWalletBalance string `json:"crossWalletBalance"`
	CrossUnPnl         string `json:"crossUnPnl"`
	AvailableBalance   string `json:"availableBalance"`
	MaxWithdrawAmount  string `json:"maxWithdrawAmount"`
	MarginAvailable    bool   `json:"marginAvailable"`
	UpdateTime         int64  `json:"updateTime"`
}

type binance struct {
	fapiEndpoint string
	apiEndpoint  string
	key          string
	secret       string
}

var BinanceApiClient *binance

func InitBinanceApi(apiKey, apiSecret string) {
	BinanceApiClient = &binance{
		fapiEndpoint: "https://fapi.binance.com", // U本位合约
		apiEndpoint:  "https://api.binance.com",  // 现货/杠杆/币安宝/矿池
		key:          apiKey,
		secret:       apiSecret,
	}
}

// GetFAPIv2Balances U本位合约-账户余额 https://binance-docs.github.io/apidocs/futures/cn/#user_data-7
//
//	{
//	       "accountAlias": "SgsR",    // 账户唯一识别码
//	       "asset": "USDT",        // 资产
//	       "balance": "122607.35137903",   // 总余额
//	       "crossWalletBalance": "23.72469206", // 全仓余额
//	       "crossUnPnl": "0.00000000"  // 全仓持仓未实现盈亏
//	       "availableBalance": "23.72469206",       // 下单可用余额
//	       "maxWithdrawAmount": "23.72469206",     // 最大可转出余额
//	       "marginAvailable": true,    // 是否可用作联合保证金
//	       "updateTime": 1617939110373
//	   }
func (b *binance) GetFAPIv2Balances(key, secret string) ([]*fapiV2BalanceResp, error) {
	values := url.Values{}
	values.Set("timestamp", b.timestampMilli())

	api := fmt.Sprintf("%s/fapi/v2/balance?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(secret, values))

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err := json.Unmarshal(body, &res)
		if err != nil {
			return nil, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return nil, ApikeyInvalidError
		}
		return nil, fmt.Errorf("%s", res.Msg)
	}
	var res []*fapiV2BalanceResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return res, nil
	}
}

func (b *binance) GetFAPIv2BalanceByAsset(key, secret string, assets []string) (map[string]string, error) {
	resMap := make(map[string]string)

	balances, err := b.GetFAPIv2Balances(key, secret)
	if err != nil {
		return resMap, err
	}
	for _, asset := range assets {
		for _, balance := range balances {
			if strings.ToUpper(asset) == strings.ToUpper(balance.Asset) {
				resMap[asset] = balance.Balance
			}
		}
	}
	return resMap, nil
}

// GetApiV3TickerPrice 获取交易对最新价格 https://binance-docs.github.io/apidocs/spot/cn/#24hr
// [
//
//	{
//	  "symbol": "LTCBTC",
//	  "price": "4.00000200"
//	},
//
// ]
func (b *binance) GetApiV3TickerPrice(symbols ...string) ([]*apiV3TickerPriceResp, error) {
	api := fmt.Sprintf("%s/api/v3/ticker/price", b.apiEndpoint)

	if len(symbols) > 0 {
		b, _ := json.Marshal(symbols)
		api = fmt.Sprintf("%s?symbols=%s", api, string(b))
	}

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err := json.Unmarshal(body, &res)
		if err != nil {
			return nil, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return nil, ApikeyInvalidError
		}
		return nil, fmt.Errorf("%s", res.Msg)
	}

	var res []*apiV3TickerPriceResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return res, nil
	}
}

/*
*
双向持仓  开多
双向持仓  开空
双向持仓  平多
双向持仓  平空
双向持仓多加仓
双向持仓空加仓
双向持仓  减空
双向持仓  减多
双向持仓  多转空
双向持仓  空转多
单向持仓开多
单向持仓开空
单向持仓多加仓
单向持仓空加仓
单向持仓多减仓
单向持仓空减仓
单向持仓多平仓
单向持仓空平仓
*/

// 双向持仓，市价开多
//
//	func OpenOrder() {
//		//市价开多
//		values := url.Values{}
//		values.Set("symbol", "BTCUSDT")
//		values.Set("side", "BUY")
//		values.Set("positionSide", "LONG")
//		values.Set("type", "MARKET")
//		values.Set("quantity", decimal.NewFromFloat(0.003).String())
//		//values.Set("price", decimal.NewFromFloat(0).String())
//		values.Set("newClientOrderId", "123")
//		values.Set("closePosition", "false")
//		//values.Set("timeInForce", "IOC")
//		values.Set("timestamp", b.timestampMilli())
//	}

func (b *binance) Order(market string, size string, side string) (*apiOrderRsp, error) {
	//市价开多
	values := url.Values{}
	values.Set("symbol", trans2BinancecMarket(market)) //BTCUSDT
	values.Set("side", side)                           //BUY SELL
	values.Set("type", "MARKET")
	values.Set("quantity", size)
	values.Set("timestamp", b.timestampMilli())
	api := fmt.Sprintf("%s/fapi/v1/order?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(b.secret, values))

	req, err := http.NewRequest(http.MethodPost, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", b.key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err := json.Unmarshal(body, &res)
		if err != nil {
			return nil, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return nil, ApikeyInvalidError
		}
		return nil, fmt.Errorf("%s", res.Msg)
	}

	var res *apiOrderRsp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return res, nil
	}
}

func (b *binance) GetBinanceTimeStamp() (int64, error) {
	api := fmt.Sprintf("%s/fapi/v1/time", b.fapiEndpoint)

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err := json.Unmarshal(body, &res)
		if err != nil {
			return 0, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		return 0, fmt.Errorf("%s", res.Msg)
	}
	timeStampResp := new(fapiTimeStampResp)
	err = json.Unmarshal(body, &timeStampResp)
	if err != nil {
		return 0, err
	}
	return timeStampResp.ServerTime, nil
}

// 切换杠杆模式
func (b *binance) switchLeverageType(leverageType string) error {
	values := url.Values{}
	serverTimeStamp := time.Now().UnixMilli()
	values.Set("symbol", "BTCUSDT")
	values.Set("marginType", leverageType) //保证金模式 ISOLATED(逐仓), CROSSED(全仓)
	values.Set("timestamp", fmt.Sprintf("%d", serverTimeStamp))

	binanceStamp, _ := b.GetBinanceTimeStamp()
	if binanceStamp > 0 {
		diff := binanceStamp - serverTimeStamp
		if diff < 0 {
			diff = -diff
		}
		if diff >= 5000 {
			values.Set("recvWindow", fmt.Sprintf("%d", diff))
		}
	}

	api := fmt.Sprintf("%s/fapi/v1/marginType?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(b.secret, values))

	req, err := http.NewRequest(http.MethodPost, api, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", b.key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err = json.Unmarshal(body, &res)
		if err != nil {
			return fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return ApikeyInvalidError
		}
		return fmt.Errorf("%s", res.Msg)
	}
	return nil
}

// 切换杠杆
func (b *binance) switchLeverage(leverage int) (*switchLeverageResp, error) {
	if leverage < 1 || leverage > 125 {
		return nil, fmt.Errorf("leverage over limit")
	}
	values := url.Values{}
	serverTimeStamp := time.Now().UnixMilli()
	values.Set("symbol", "BTCUSDT")
	values.Set("leverage", "BUY")
	values.Set("timestamp", fmt.Sprintf("%d", serverTimeStamp))

	binanceStamp, _ := b.GetBinanceTimeStamp()
	if binanceStamp > 0 {
		diff := binanceStamp - serverTimeStamp
		if diff < 0 {
			diff = -diff
		}
		if diff >= 5000 {
			values.Set("recvWindow", fmt.Sprintf("%d", diff))
		}
	}

	api := fmt.Sprintf("%s/fapi/v1/leverage?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(b.secret, values))

	req, err := http.NewRequest(http.MethodPost, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", b.key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err = json.Unmarshal(body, &res)
		if err != nil {
			return nil, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return nil, ApikeyInvalidError
		}
		return nil, fmt.Errorf("%s", res.Msg)
	}

	var res *switchLeverageResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return res, nil
	}
}

func (b *binance) GetTickerMapBySymbols(symbols []string, pair string) (map[string]string, error) {
	m := make(map[string]string)

	symbolPairs := make([]string, 0)
	for _, value := range symbols {
		symbolPairs = append(symbolPairs, value+pair)
	}

	tickers, err := b.GetApiV3TickerPrice(symbolPairs...)
	if err != nil {
		return m, err
	}

	for _, symbol := range symbols {
		for _, ticker := range tickers {
			if symbol == strings.ReplaceAll(ticker.Symbol, pair, "") {
				m[symbol] = ticker.Price
			}
		}
	}

	return m, nil
}

func (b *binance) GetFapiV2Account(key, secret string) (*fapiV2AccountResp, error) {
	values := url.Values{}
	values.Set("timestamp", b.timestampMilli())

	api := fmt.Sprintf("%s/fapi/v2/account?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(secret, values))

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err := json.Unmarshal(body, &res)
		if err != nil {
			return nil, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return nil, ApikeyInvalidError
		}
		return nil, fmt.Errorf("%s", res.Msg)
	}

	var res fapiV2AccountResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return &res, nil
	}
}

func (b *binance) GetFapiV2OpenOrders(key, secret string) ([]*fapiV2OpenOrderResp, error) {
	values := url.Values{}
	values.Set("timestamp", b.timestampMilli())

	api := fmt.Sprintf("%s/fapi/v1/openOrders?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(secret, values))

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		var res errMsgResp
		err := json.Unmarshal(body, &res)
		if err != nil {
			return nil, fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return nil, ApikeyInvalidError
		}
		return nil, fmt.Errorf("%s", res.Msg)
	}

	var res []*fapiV2OpenOrderResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return res, nil
	}
}

// makeSignature 添加签名
func (b *binance) makeSignature(secret string, values url.Values) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(values.Encode()))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (b *binance) timestampMilli() string {
	return fmt.Sprintf("%d", time.Now().UnixMilli())
}

//
//// 获取合约仓位
//func (b *binance) ListPositions(uid int64, key, secret string) ([]gateapi.Position, error) {
//	rsp, err := b.GetFapiV2Account(key, secret)
//	if err != nil {
//		return nil, err
//	}
//	pos := []gateapi.Position{}
//	for _, v := range rsp.Positions {
//		if !strings.HasSuffix(v.Symbol, "USDT") {
//			continue
//		}
//		sizeD, _ := decimal.NewFromString(v.PositionAmt)
//		market := model.TransBinanceMarket(v.Symbol)
//		marketInfo, ok := GetMarketInfo(market, model.EngineTypeFuturesUsdt)
//		if !ok {
//			continue
//		}
//		if marketInfo.QuantoMultiplier.IsZero() {
//			continue
//		}
//		size := sizeD.Div(marketInfo.QuantoMultiplier).IntPart()
//		one := gateapi.Position{
//			User:          uid,
//			Contract:      model.TransBinanceMarket(v.Symbol),
//			Size:          size,
//			Margin:        v.InitialMargin,
//			EntryPrice:    v.EntryPrice,
//			InitialMargin: v.InitialMargin,
//			UnrealisedPnl: v.UnrealizedProfit,
//			Mode:          model.TransBinanceMode(v.PositionSide),
//			UpdateTime:    int64(v.UpdateTime),
//		}
//		if v.Isolated {
//			one.Leverage = v.Leverage
//			one.CrossLeverageLimit = "0"
//		} else {
//			one.Leverage = "0"
//			one.CrossLeverageLimit = v.Leverage
//		}
//		pos = append(pos, one)
//	}
//	return pos, nil
//}

//func (b *binance) ListFuturesAccounts(key, secret string) (gateapi.FuturesAccount, error) {
//	rsp, err := b.GetFapiV2Account(key, secret)
//	if err != nil {
//		return gateapi.FuturesAccount{}, err
//	}
//	account := gateapi.FuturesAccount{
//		Total:                 rsp.TotalWalletBalance,
//		UnrealisedPnl:         rsp.TotalUnrealizedProfit,
//		PositionMargin:        rsp.TotalMaintMargin,
//		OrderMargin:           rsp.TotalOpenOrderInitialMargin,
//		Available:             rsp.AvailableBalance,
//		Currency:              "USDT",
//		PositionInitialMargin: rsp.TotalPositionInitialMargin,
//	}
//
//	return account, nil
//}

// BTC_USDT -> BTCUSDT
func trans2BinancecMarket(market string) string {
	arr := strings.Split(market, "_USDT")
	if len(arr) != 2 {
		return ""
	}
	return arr[0] + "USDT"
}

func GetListenKey(host, apiKey, apiSecret string) (listenKey string, err error) {
	hash := hmac.New(sha512.New, []byte(apiSecret))
	hash.Write([]byte(fmt.Sprintf("timestamp=%d", time.Now().UnixMilli())))

	requestBody := map[string]interface{}{
		"timestamp": time.Now().UnixMilli(),
		"signature": hex.EncodeToString(hash.Sum(nil)),
	}

	requestOptions := &grequests.RequestOptions{
		JSON:    requestBody,
		Headers: map[string]string{"X-MBX-APIKEY": apiKey},
	}

	// 发送HTTP POST请求以获取listenKey
	url := host + "/fapi/v1/listenKey"
	resp, err := grequests.Post(url, requestOptions)
	if err != nil {
		return
	}

	// 检查响应状态码
	if resp.StatusCode != 200 {
		err = fmt.Errorf("GetListenKey HTTP request failed with status code: %d, msg: %s", resp.StatusCode, string(resp.Bytes()))
		return
	}

	// 解析JSON响应以获取listenKey
	var response map[string]interface{}
	if err = resp.JSON(&response); err != nil {
		err = fmt.Errorf("GetListenKey error decoding JSON response:%v", err)
		return
	}

	listenKey, ok := response["listenKey"].(string)
	if !ok {
		err = fmt.Errorf("GetListenKey failed to extract listenKey from the response")
		return
	}

	return listenKey, nil
}

func RefreshListenKey(host, apiKey string) (err error) {
	requestOptions := &grequests.RequestOptions{
		Headers: map[string]string{"X-MBX-APIKEY": apiKey},
	}

	// 发送HTTP PUT 请求以延长 listenKey 时间
	url := host + "/fapi/v1/listenKey"
	resp, err := grequests.Put(url, requestOptions)
	if err != nil {
		return
	}

	// 检查响应状态码
	if resp.StatusCode != 200 {
		err = fmt.Errorf("RefreshListenKey HTTP request failed with status code: %d, msg: %s", resp.StatusCode, string(resp.Bytes()))
		return
	}

	return nil
}
