package ws

import (
	"context"
	"encoding/json"
	"fmt"
	gateapi "github.com/gateio/gateapi-go/v6"
	"io"
	"net/http"
	"quant/utils"
	"time"
)

type MarketInfo struct {
	Name string `json:"name"`
}

func getGateMarketInfo() []string {
	api := "https://api.gateio.ws/api/v4/futures/usdt/contracts"

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return []string{}
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []string{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}
	}

	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		return []string{}
	}
	var marketInfoList = make([]*MarketInfo, 0)
	err = json.Unmarshal(body, &marketInfoList)
	if err != nil {
		return []string{}
	}
	marketList := make([]string, 0, len(marketInfoList))
	for _, info := range marketInfoList {
		marketList = append(marketList, info.Name)
	}
	return marketList
}

func getGateApiClient() *gateapi.APIClient {
	cfg := gateapi.NewConfiguration()
	//    cfg.AddDefaultHeader(kHeadUserIdKey, strconv.FormatInt(int64(userId), 10))
	cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	client := gateapi.NewAPIClient(cfg)
	//client.ChangeBasePath(config.ConfigNode.Apiv4Excore)
	return client
}

func PlaceExchagneOrder(market string, size int) {
	reqOrder := gateapi.FuturesOrder{
		Contract: market,
		Price:    "0",
		Size:     int64(size),
		Tif:      "ioc",
	}
	client := getGateApiClient()
	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    "f6f3cc50911e00ae4ff6a5dbb1913a5e",
		Secret: "8ea94f2850ad01d1da565e5c2ef6369e9667cb18209f676120857d5bc8f42c34",
	})
	orderResponse, _, err := client.FuturesApi.CreateFuturesOrder(ctx, "usdt", reqOrder)
	fmt.Println(orderResponse)
	if err != nil {
		return
	}
	return
	//respOrder, rsp, err := client.FuturesApi.CreateFuturesOrder(e.getContext(order.SubUserId), order.Settle, reqOrder)
}
