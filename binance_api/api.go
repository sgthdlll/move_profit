package binance_api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	sdk "github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/levigross/grequests"
	"io"
	"move_profit/utils"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var binanceMarketInfoMap sync.Map

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

type MsgResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type fapiTimeStampResp struct {
	ServerTime int64 `json:"serverTime"`
}

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

	result, _ := BinanceApiClient.GetMarketInfo()

	for _, contract := range result.Symbols {
		binanceMarketInfoMap.Store(utils.Trans2GateMarket(contract.Symbol), contract)
	}

}

func GetMarketInfo(market string) (futures.Symbol, bool) {
	val, ok := binanceMarketInfoMap.Load(market)
	if !ok {
		return futures.Symbol{}, false
	}
	return val.(futures.Symbol), true
}

func (b *binance) GetMarketInfo() (*futures.ExchangeInfo, error) {
	return sdk.NewFuturesClient(b.key, b.secret).NewExchangeInfoService().Do(context.Background(), futures.WithRecvWindow(10000))
}

func (b *binance) Order(market string, size string, side string) (*apiOrderRsp, error) {
	//市价开多
	values := url.Values{}
	values.Set("symbol", utils.Trans2BinancecMarket(market)) //BTCUSDT
	values.Set("side", side)                                 //BUY SELL
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
		var res MsgResp
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
		var res MsgResp
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

// 切换持仓模式
func (b *binance) SwitchPositionMode() error {
	values := url.Values{}
	serverTimeStamp := time.Now().UnixMilli()
	values.Set("dualSidePosition", "false")
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

	api := fmt.Sprintf("%s/fapi/v1/positionSide/dual?%s&signature=%s", b.fapiEndpoint, values.Encode(), b.makeSignature(b.secret, values))

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
		var res MsgResp
		err = json.Unmarshal(body, &res)
		if err != nil {
			return fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return ApikeyInvalidError
		}
		return fmt.Errorf("%s", res.Msg)
	}
	var res MsgResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return fmt.Errorf("resp code not 200 resp:%+v", resp)
	}
	return nil
}

// 切换杠杆模式
func (b *binance) SwitchLeverageType(leverageType string) error {
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
		var res MsgResp
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

func (b *binance) SwitchMarginMode(market string) error {

	values := url.Values{}
	serverTimeStamp := time.Now().UnixMilli()
	values.Set("symbol", utils.Trans2BinancecMarket(market))
	values.Set("marginType", "CROSSED")
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
		var res MsgResp
		err = json.Unmarshal(body, &res)
		if err != nil {
			return fmt.Errorf("resp code not 200 resp:%+v", resp)
		}
		if res.Code == apikeyInvalidCode {
			return ApikeyInvalidError
		}
		return fmt.Errorf("%s", res.Msg)
	}

	var res *MsgResp
	err = json.Unmarshal(body, &res)
	if err != nil {
		return fmt.Errorf("parse body err %+v err:%+v", resp, err)
	} else {
		return nil
	}
}

// 切换杠杆
func (b *binance) SwitchLeverage(market string, leverage int) (*switchLeverageResp, error) {
	if leverage < 1 || leverage > 125 {
		return nil, fmt.Errorf("leverage over limit")
	}
	values := url.Values{}
	serverTimeStamp := time.Now().UnixMilli()
	values.Set("symbol", utils.Trans2BinancecMarket(market))
	values.Set("leverage", fmt.Sprintf("%d", leverage))
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
		var res MsgResp
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

// makeSignature 添加签名
func (b *binance) makeSignature(secret string, values url.Values) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(values.Encode()))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (b *binance) timestampMilli() string {
	return fmt.Sprintf("%d", time.Now().UnixMilli())
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
