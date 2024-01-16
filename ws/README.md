# ws 使用教程

```go
var pub, pri <-chan []byte
server, err := ws.NewWsService(context.Background(), logger, &ws.ConnConf{
    ApiUrl:                   "https://fapi.binance.com",
    URL:                      "wss://fstream.binance.com/ws",
    Key:                      "key",
    Secret:                   "secret",
    IsOpenPrivacyWs:          true,
    IsOpenPublicWs:           true,
    PublicChanLen:            5000,
    PrivacyChanLen:           5000,
    ListenKeyRefreshInterval: "58m50s",
})
if err != nil {
    logger.Fatal(err)
}

initChan := make(chan struct{})
go server.Start(initChan)

select {
case <-initChan:
case <-time.After(time.Second * 10):
    logger.Fatal("ws service init timeout")
}

// 获取输出 chan
pub, err = server.GetPublicMsgChan()
pri, err = server.GetPrivacyMsgChan()

err = server.WriteSubscribeMsg(ws.SubscribeMsgRequest{
    Method: "SUBSCRIBE",
    Params: []interface{}{"btcusdt@miniTicker", "btcusdt@depth100ms"},
})
if err != nil {
    logger.Warnf("failed to write subscribe msg:%s", err.Error())
}

for {
    select {
    case m1 := <-pub:
        fmt.Println("===pub", string(m1))
    case m2 := <-pri:
        fmt.Println("---pri", string(m2))
    }
}
```