package ws

import (
	"crypto/hmac"
	"crypto/sha512"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/shopspring/decimal"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var GateLastPriceMap sync.Map
var GateMarketInfoMap sync.Map

type Ticker struct {
	Contract              string `json:"contract"`
	Last                  string `json:"last"`
	ChangePercentage      string `json:"change_percentage"`
	TotalSize             string `json:"total_size"`
	Volume24H             string `json:"volume_24h"`
	Volume24HBase         string `json:"volume_24h_base"`
	Volume24HQuote        string `json:"volume_24h_quote"`
	Volume24HSettle       string `json:"volume_24h_settle"`
	MarkPrice             string `json:"mark_price"`
	FundingRate           string `json:"funding_rate"`
	FundingRateIndicative string `json:"funding_rate_indicative"`
	IndexPrice            string `json:"index_price"`
	QuantoBaseRate        string `json:"quanto_base_rate"`
	Low24H                string `json:"low_24h"`
	High24H               string `json:"high_24h"`
}

type Msg struct {
	Time    int64    `json:"time"`
	Channel string   `json:"channel"`
	Event   string   `json:"event"`
	Payload []string `json:"payload"`
	Auth    *Auth    `json:"auth"`
}

type Auth struct {
	Method string `json:"method"`
	KEY    string `json:"KEY"`
	SIGN   string `json:"SIGN"`
}

const (
	Key    = "YOUR_API_KEY"
	Secret = "YOUR_API_SECRETY"
)

func sign(channel, event string, t int64) string {
	message := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, t)
	h2 := hmac.New(sha512.New, []byte(Secret))
	io.WriteString(h2, message)
	return hex.EncodeToString(h2.Sum(nil))
}

func (msg *Msg) sign() {
	signStr := sign(msg.Channel, msg.Event, msg.Time)
	msg.Auth = &Auth{
		Method: "api_key",
		KEY:    Key,
		SIGN:   signStr,
	}
}

func (msg *Msg) send(c *websocket.Conn) error {
	msgByte, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, msgByte)
}

func NewMsg(channel, event string, t int64, payload []string) *Msg {
	return &Msg{
		Time:    t,
		Channel: channel,
		Event:   event,
		Payload: payload,
	}
}

func GateTicker() {
	u := url.URL{Scheme: "wss", Host: "fx-ws.gateio.ws", Path: "/v4/ws/usdt"}
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	c.SetPingHandler(nil)

	// read msg
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				c.Close()
				panic(err)
			}
			data, err := simplejson.NewJson(message)
			if err != nil {
				continue
			}
			channel, _ := data.Get("channel").String()
			if channel != "futures.tickers" {
				continue
			}
			event, _ := data.Get("event").String()
			if event != "update" {
				continue
			}
			resultList, _ := data.Get("result").Array()
			for _, result := range resultList {
				resultMap := result.(map[string]interface{})
				market := resultMap["contract"]
				lastPrice := resultMap["last"]
				price := lastPrice.(string)
				priceD, _ := decimal.NewFromString(price)
				GateLastPriceMap.Store(market.(string), priceD)
			}
			//tickerList := make([]*Ticker, 0)
			//err = json.Unmarshal([]byte(result), &tickerList)
			//if err != nil {
			//	continue
			//}
			//fmt.Println(123)
		}
	}()

	t := time.Now().Unix()
	//pingMsg := NewMsg("futures.ping", "", t, []string{})
	//err = pingMsg.send(c)
	//if err != nil {
	//	panic(err)
	//}

	//
	//// subscribe order book
	//orderBookMsg := NewMsg("futures.order_book", "subscribe", t, []string{"BTC_USDT"})
	//err = orderBookMsg.send(c)
	//if err != nil {
	//	panic(err)
	//}
	//
	//// subscribe positions
	//positionsMsg := NewMsg("futures.positions", "subscribe", t, []string{"USERID", "BTC_USDT"})
	//positionsMsg.sign()
	//err = positionsMsg.send(c)
	//if err != nil {
	//	panic(err)
	//}
	marketInfoList, err := getGateMarketInfo()
	if len(marketInfoList) <= 0 || err != nil {
		return
	}
	marketNameList := make([]string, 0, len(marketInfoList))
	for _, m := range marketInfoList {
		marketNameList = append(marketNameList, m.Name)
		GateMarketInfoMap.Store(m.Name, m.QuantoMultiplier)
	}
	tickerMsg := NewMsg("futures.tickers", "subscribe", t, marketNameList)
	tickerMsg.sign()
	err = tickerMsg.send(c)
	if err != nil {
		panic(err)
	}

	select {}
}

func GetGateMarketQuantoMultiplier(market string) decimal.Decimal {
	quantoMultiplier, ok := GateMarketInfoMap.Load(market)
	if !ok {
		return decimal.Zero
	}
	quantoMultiplierD, _ := decimal.NewFromString(quantoMultiplier.(string))
	return quantoMultiplierD
}
